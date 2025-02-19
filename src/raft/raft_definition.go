package raft

import (
	"math/rand"
	"sync"
	"time"

	"6.5840/labrpc"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 3D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 3D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	// 3A
	currentTerm int // 当前任期
	votedFor    int // 投票给谁，一个任期内，节点只能将选票投给某一个节点，所以当节点任期更新时将投票重置为-1
	state       int // 状态
	// 收到 RequestVote RPC 和 AppendEntries RPC都要重置选举超时计时器
	electionTimer   *time.Timer  // 选举超时计时器（每个都有）
	heartbeatTicker *time.Ticker // 心跳定时器（只有leader有）

	// 3B
	logs []LogEntry // 日志
	// 所有机器
	commitIndex int //已知已提交的日志条目的最高索引值（初始值为 0，单调递增）
	lastApplied int //已应用到状态机的日志条目的最高索引值（初始值为 0，单调递增）
	// 当节点发现 commitIndex > lastApplied 时，代表着 commitIndex 和 lastApplied 间的 entries 处于已提交，未应用的状态。因此应将其间的 entries 按序应用至状态机。
	// leader
	nextIndex  []int //针对每个服务器，记录要发送给该服务器的下一个日志条目的索引值（初始化为leader最后一个日志条目的索引值 +1）
	matchIndex []int //针对每个服务器，记录已知已复制到该服务器的日志条目的最高索引值（初始值为 0，单调递增）
	// 不能简单地认为 matchIndex = nextIndex - 1，当有节点宕机时，nextIndex 会大于 matchIndex
	cond         *sync.Cond    // 用于通知其他 goroutine的条件变量
	applyMsg     chan ApplyMsg // 已提交日志需要被应用到状态机里
	addLogSignal chan struct{} // 提醒leader新增log的信号，通过给channel传一个struct{}{}即可唤醒
}

// LogEntry
type LogEntry struct {
	Term int         // 从 leader 接受到日志的任期
	Cmd  interface{} // 待施加至状态机的命令
}

// 状态
const (
	Follower = iota
	Candidate
	Leader
)

const noVote = -1

// 广播心跳时间(broadcastTime)<<选举超时时间
func HeartBeatTimeout() time.Duration {
	return 50 * time.Millisecond
}

// 随机超时选举时间生成（150ms-300ms）
func randomElectionTimeout() time.Duration {
	return time.Duration(150+rand.Int63()%150) * time.Millisecond
}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (3A, 3B).
	Term        int // Candidate 任期
	CandidateId int // Candidate id
	// 3B
	LastLogIndex int // Candidate 最后一条日志的索引，是投票的额外判据
	LastLogTerm  int // Candidate 最后一条日志的任期
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (3A).
	Term int // 此节点任期
	//假如 Candidate 发现 Follower 的任期高于自己，则会放弃 Candidate 身份并更新自己的任期。
	VoteGranted bool // 是否投票给Candidate
}

// AppendEntries RPC（leader下发，实现leader的心跳机制，后续用来日志复制）
type AppendEntriesArgs struct {
	Term     int // leader的任期
	LeaderId int // leader的id
	//Client 可能将请求发送至 Follower 节点，得知 leaderId 后 Follower 可将 Client 的请求重定位至 Leader 节点。
	//因为 Raft 的请求信息必须先经过 Leader 节点，再由 Leader 节点流向其他节点进行同步，信息是单向流动的。在选主过程中，leaderId暂时只有 debug 的作用。
	PrevLogIndex int        // 前一条日志的索引
	PrevLogTerm  int        // 前一条日志的任期
	Entries      []LogEntry // 待复制的日志（若为空则是一次心跳）
	LeaderCommit int        // leader的commitIndex，帮助 Follower 更新自身的 commitIndex
}

// AppendEntries RPC reply
type AppendEntriesReply struct {
	Term    int  // 此节点的任期。假如 Leader 发现 Follower 的任期高于自己，则会放弃 Leader 身份并更新自己的任期。
	Success bool // 此节点是否认同leader的心跳
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{
		peers:     peers,
		persister: persister,
		me:        me,
		dead:      0,

		// 3A
		currentTerm:     0,
		votedFor:        noVote,
		state:           Follower,
		electionTimer:   time.NewTimer(randomElectionTimeout()),
		heartbeatTicker: time.NewTicker(HeartBeatTimeout()),

		// 3B
		logs:         make([]LogEntry, 1), // 索引0的日志置为空，因为需要记录前一个日志信息
		commitIndex:  0,
		lastApplied:  0,
		nextIndex:    make([]int, len(peers)),
		matchIndex:   make([]int, len(peers)),
		applyMsg:     applyCh,
		addLogSignal: make(chan struct{}),
	}
	// 初始化条件变量，控制applier协程和其他协程之间的通信
	rf.cond = sync.NewCond(&rf.mu)
	// 从崩溃前的状态初始化
	rf.readPersist(persister.ReadRaftState())

	go rf.ticker()         // 启动 ticker goroutine 来监听选举超时定时器，开始选举
	go rf.heartbeatEvent() // 启动心跳 goroutine 来监听心跳定时器，发送心跳（日志复制在心跳RPC中实现）
	go rf.AddLogEvent()    // 启动 AddLogEvent goroutine 来监听 addLog 事件，发送日志复制请求
	go rf.applierEvent()   // 启动状态机应用日志的 goroutine

	return rf
}
