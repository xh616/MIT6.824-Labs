package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"time"

	"6.5840/labgob"
	// "6.5840/labrpc"
)

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// var term int
	// var isleader bool
	// Your code here (3A).
	// return term, isleader
	// 读取状态需要加锁
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.state == Leader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (3C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
	defer func() {
		Debug(dPersist, "S%d persisted status{currentTerm:%d,log_len:%d,commitIndex:%d,appliedIndex:%d}",
			rf.me, rf.currentTerm, len(rf.logs)-1, rf.commitIndex, rf.lastApplied)
	}()
	// 创建一个新的缓冲区，用来存储序列化的数据
	w := new(bytes.Buffer)

	// 创建一个 labgob 编码器，用来将数据编码到缓冲区
	e := labgob.NewEncoder(w)

	// 将当前任期、投票对象和日志条目进行编码
	status := &PersistentStatus{
		Logs:              rf.logs,
		CurrentTerm:       rf.currentTerm,
		VotedFor:          rf.votedFor,
		LastIncludedIndex: rf.lastIncludedIndex,
		LastIncludedTerm:  rf.logs[0].Term, //在Snapshot()方法里的0号日志对该值做了记录
	}
	if err := e.Encode(status); err != nil {
		Debug(dError, "persist encode err:%v", err)
		return
	}

	// 将缓冲区中的数据转换为字节切片
	data := w.Bytes()

	// 将序列化的数据和快照保存到持久化存储
	rf.persister.Save(data, rf.snapshot)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if len(data) < 1 { // bootstrap without any state?
		return
	}

	defer func() {
		Debug(dPersist, "after read persist, S%d recover to status{currentTerm:%d,commitIndex:%d,applied:%d,log_len:%d}",
			rf.me, rf.currentTerm, rf.commitIndex, rf.lastApplied, len(rf.logs)-1)
	}()

	// Your code here (3C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
	// 创建一个新的缓冲区，读取传入的字节数据
	r := bytes.NewBuffer(data)

	// 创建一个 labgob 解码器，用来从缓冲区读取数据
	d := labgob.NewDecoder(r)

	// 用于存储解码后的数据
	persistentStatus := &PersistentStatus{}
	if err := d.Decode(persistentStatus); err != nil {
		Debug(dError, "readPersist decode err:%v", err)
		return
	}

	// 如果解码成功，将解码后的值赋给 Raft 服务器的状态
	rf.currentTerm = persistentStatus.CurrentTerm
	rf.votedFor = persistentStatus.CurrentTerm
	rf.logs = persistentStatus.Logs
	// 最新的快照点
	rf.lastIncludedIndex = persistentStatus.LastIncludedIndex
	rf.logs[0].Term = persistentStatus.LastIncludedTerm //使用0号日志存储最新的快照点任期
	// 之前被快照的数据，一定是被提交的，并且已提交的日志一定会被应用到状态机（论文设定）
	rf.commitIndex = persistentStatus.LastIncludedIndex
	rf.lastApplied = persistentStatus.LastIncludedIndex
	// 加载上一次的快照
	rf.snapshot = rf.persister.ReadSnapshot()
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (3D).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dSnap, "after Snapshot, S%d status{currentTerm:%d,commitIndex:%d,applied:%d,snapshotIndex:%d,lastIncludedIndex:%d,log_len:%d}",
			rf.me, rf.currentTerm, rf.commitIndex, rf.lastApplied, index, rf.lastIncludedIndex, len(rf.logs)-1)
	}()
	Debug(dSnap, "before Snapshot, S%d status{currentTerm:%d,commitIndex:%d,applied:%d,snapshotIndex:%d,lastIncludedIndex:%d,log_len:%d}",
		rf.me, rf.currentTerm, rf.commitIndex, rf.lastApplied, index, rf.lastIncludedIndex, len(rf.logs)-1)

	// 快照点不能超过应用点 且 当前快照点得大于上一次快照点
	// 我的应用协程是按照一提交就应用来写的，然后得等日志被应用完才能裁剪，所以这里得要写成commitIndex
	if rf.commitIndex < index || index <= rf.lastIncludedIndex {
		Debug(dSnap, "S%d snapshot fail: rf.lastApplied<index(%v) rf.lastIncludedIndex>=index(%v)",
			rf.me, rf.lastApplied < index, rf.lastIncludedIndex >= index)
		return
	}

	// 丢弃被快照了的日志，同时修改其他状态
	// last: snap{nil,1,2,3} {nil}
	// now:  snap{nil,1,2,3,4,5} {nil,4,5}
	split := rf.logIndex(index) // index在log中的索引
	rf.lastIncludedIndex = index
	// 0号日志暂存上一个LastIncludedTerm，可以少定义一个变量
	rf.logs = append([]LogEntry{{Term: rf.logs[split].Term}}, rf.logs[split+1:]...)
	rf.snapshot = snapshot //时刻存储raft持有的最新快照
	rf.persist()           // 持久化
}

// InstallSnapshot RPC，leader使用InstallSnapshot RPC 来拷贝快照到那些远远落后的机器。
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dSnap, "after InstallSnapshot, S%d status{currentTerm:%d,commitIndex:%d,applied:%d,lastIncludedIndex:%d,log_len:%d}",
			rf.me, rf.currentTerm, rf.commitIndex, rf.lastApplied, rf.lastIncludedIndex, len(rf.logs)-1)
	}()
	Debug(dSnap, "before InstallSnapshot, S%d status{currentTerm:%d,commitIndex:%d,applied:%d,lastIncludedIndex:%d,log_len:%d}",
		rf.me, rf.currentTerm, rf.commitIndex, rf.lastApplied, rf.lastIncludedIndex, len(rf.logs)-1)

	// 1、如果leader的term小于当前term，则返回false
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		return
	}
	// 如果当前节点落后了leader，改任期和投票后，可以接着安装快照
	if args.Term > rf.currentTerm {
		rf.currentTerm, rf.votedFor = args.Term, noVote
	}
	// leader安装快照的过程请求了对等点, 算是一次ping/pong, 可以刷新选举计时器以及重新置为follower
	rf.state = Follower
	rf.electionTimer.Reset(randomElectionTimeout())

	// 5、保存快照文件，删除快照之前的日志
	// leader快照点落后于当前节点的快照点时，无需快照
	if args.LastIncludedIndex <= rf.lastIncludedIndex {
		reply.Term = rf.currentTerm
		return
	}
	// leader快照点落后于当前节点的提交点时，无需快照
	if args.LastIncludedIndex <= rf.commitIndex {
		reply.Term = rf.currentTerm
		return
	}

	defer rf.persist() // 持久化

	// 快照信息
	msg := ApplyMsg{
		SnapshotValid: true,
		Snapshot:      args.Data,
		SnapshotTerm:  args.LastIncludedTerm,
		SnapshotIndex: args.LastIncludedIndex,
	}

	//6、如果现存的日志条目与快照点具有相同的索引值和任期号，则保留其后的日志条目并进行回复
	for i := 1; i < len(rf.logs); i++ {
		if rf.realIndex(i) == args.LastIncludedIndex && rf.logs[0].Term == args.LastIncludedTerm {
			rf.logs = append([]LogEntry{{Term: args.LastIncludedTerm}}, rf.logs[i+1:]...)
			rf.lastIncludedIndex = args.LastIncludedIndex
			rf.commitIndex = args.LastIncludedIndex
			rf.lastApplied = args.LastIncludedIndex
			rf.snapshot = args.Data
			// 使用快照的状态重置状态机，并加载快照中的配置
			go func() {
				// 等待日志复制的日志进入管道过后才能把快照塞入管道
				for rf.replicatePreSnapshot.Load() { //不断加载，直到为false
					time.Sleep(time.Millisecond * 10) //等待10ms后再次检查
				}
				rf.applyMsg <- msg
			}()

			reply.Term = rf.currentTerm
			return
		}
	}

	// 7、丢弃整个日志（因为经过上述检查后，这里整个log都是过期的）
	rf.logs = []LogEntry{{Term: args.LastIncludedTerm}}
	rf.lastIncludedIndex = args.LastIncludedIndex
	rf.commitIndex = args.LastIncludedIndex
	rf.lastApplied = args.LastIncludedIndex
	rf.snapshot = args.Data

	// 8、使用快照的状态重置状态机，并加载快照中的配置。
	go func() {
		// 等待日志复制的日志进入管道过后才能把快照塞入管道
		for rf.replicatePreSnapshot.Load() { //不断加载，直到为false
			time.Sleep(time.Millisecond * 10) //等待10ms后再次检查
		}
		rf.applyMsg <- msg
	}()

	reply.Term = rf.currentTerm
}

func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
}

// example RequestVote RPC handler.
// 获取投票请求
// 当 Candidate 任期小于当前节点任期时，返回 false（放弃 Candidate 身份并更新自己的任期）。
// 如果 votedFor 为 -1（即当前任期内此节点还未投票）或者 votedFor为 candidateId（即当前任期内此节点已经向此 Candidate 投过票），则同意投票；否则拒绝投票。
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dVote, "after called RequestVote, S%d status{votedFor:%d,role:%s,currentTerm:%d}",
			rf.me, rf.votedFor, StateToString(rf.state), rf.currentTerm)
	}()
	Debug(dVote, "before called RequestVote, S%d status{votedFor:%d,role:%s,currentTerm:%d}",
		rf.me, rf.votedFor, StateToString(rf.state), rf.currentTerm)

	// 如果Candidate任期小于我的任期，则不投票
	if args.Term < rf.currentTerm {
		// 告知Candidate我的任期
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	defer rf.persist() // 持久化

	// 如果Candidate任期大于我的任期，则更新我的任期并重置我的投票，并转为Follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = noVote // 无论何时，任期更新则重置投票
		rf.state = Follower
	}

	// 没投票或者已经投票给这个Candidate，并且Candidate的日志至少和我的一样新，则投票
	if (rf.votedFor == noVote || rf.votedFor == args.CandidateId) &&
		rf.isLogUpToDate(args.LastLogTerm, args.LastLogIndex) {
		rf.votedFor = args.CandidateId
		rf.electionTimer.Reset(randomElectionTimeout()) // 重置选举超时器
		Debug(dVote, "S%d vote to S%d", rf.me, args.CandidateId)
		reply.VoteGranted = true
		reply.Term = rf.currentTerm // 告知Candidate我的任期
		return
	}

	// 失败处理
	reply.VoteGranted = false
	reply.Term = rf.currentTerm // 告知Candidate我的任期
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
// 发送投票请求
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
	//这个ok只代表该RPC有没有调用成功，不代表是否给自己投票
	// 如果 ok 为 true，表示 RPC 调用成功，并且收到了响应。
	// 如果 ok 为 false，表示 RPC 调用失败（例如网络问题、目标节点宕机等）。
}

// 当 Leader 任期小于当前节点任期时，返回 false，否则返回 true。
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dLog, "after AppendEntries, S%d status{currentTerm:%d,role:%s,commitIndex:%d,applied:%d,lastIncludedIndex:%d,log_len:%d}",
			rf.me, rf.currentTerm, StateToString(rf.state), rf.commitIndex, rf.lastApplied, rf.lastIncludedIndex, len(rf.logs)-1)
	}()
	Debug(dLog, "before AppendEntries, S%d status{currentTerm:%d,role:%s,commitIndex:%d,applied:%d,lastIncludedIndex:%d,log_len:%d}",
		rf.me, rf.currentTerm, StateToString(rf.state), rf.commitIndex, rf.lastApplied, rf.lastIncludedIndex, len(rf.logs)-1)

	// 如果leader任期小于我的任期，leader任期落后，会变为follower，则不接受该leader
	if rf.currentTerm > args.Term {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	defer rf.persist() // 持久化

	//正常收到心跳就变为Follower并刷新选举超时计时器
	rf.state = Follower
	rf.electionTimer.Reset(randomElectionTimeout())

	// 如果leader任期大于我的任期，则更新我的任期并重置我的投票
	if rf.currentTerm < args.Term {
		rf.currentTerm = args.Term
		rf.votedFor = noVote
	}

	if rf.lastIncludedIndex > args.PrevLogIndex /*Follower的快照点已经超过本次日志复制的点，没有必要接受此日志复制RPC了*/ {
		reply.Term, reply.Success = rf.currentTerm, false
		return
	}

	// 如果我宕机，落后了leader很多日志
	if rf.lastLogIndex() < args.PrevLogIndex { // 真实的最后一条日志的索引
		//这种情况下，该raft实例断网一段时间过后，日志落后。所以直接返回 XLen即可。
		//leader更新nextIndex为XLen即可，表示当前raft实例缺少XLen及后面的日志，leader在下次广播时带上这些日志
		// leader   0{0} 1{101 102 103} 5{104}	PrevLogIndex=3	nextIndex=4
		// follower 0{0} 1{101 102 103} 5{104}  PrevLogIndex=3  nextIndex=4
		// follower 0{0} 1{101} 5 				PrevLogIndex=1  nextIndex=2
		reply.XTerm, reply.XIndex = -1, -1
		reply.XLen = rf.realLogLen() //有快照机制后要返回真实日志长度
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// 该日志在 prevLogIndex 上的任期不能和 prevLogTerm 匹配，说明日志不一致
	// 返回包含自己在冲突位置的任期存储的第一条日志
	index := rf.logIndex(args.PrevLogIndex) //有快照后要用真实index在log中的索引
	if rf.logs[index].Term != args.PrevLogTerm {
		conflictIndex, conflictTerm := -1, rf.logs[index].Term
		for i := args.PrevLogIndex; i > rf.commitIndex; i-- {
			if rf.logs[rf.logIndex(i)].Term != conflictTerm {
				break
			}
			conflictIndex = i
		}
		reply.XTerm, reply.XIndex, reply.XLen = conflictTerm, conflictIndex, rf.realLogLen()
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// 将leader的日志复制到自己的日志中
	for i, entry := range args.Entries {
		index := rf.logIndex(args.PrevLogIndex + i + 1) //有快照后要用真实index在log中的索引
		if index < len(rf.logs) {                       //有日志重叠
			if rf.logs[index].Term != entry.Term { // 看是否发生冲突
				// rf.logs[:index]代表从 rf.logs 切片中获取从开始到 index（不包括 index）的部分
				rf.logs = rf.logs[:index]        // 有冲突，删除冲突的日志和之后的日志
				rf.logs = append(rf.logs, entry) // 追加新的日志
			}
			// 如果没有冲突，就不需要做任何操作
		} else if index == len(rf.logs) { //没有重叠，且刚好在下一个位置
			rf.logs = append(rf.logs, entry)
		}
	}

	// 更新commitIndex，领导者提交日志时，需要等到领导者下一次广播才能让跟随者也跟着提交。
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.lastLogIndex())
		rf.cond.Signal() //通知applier协程应用日志
	}

	reply.Term = rf.currentTerm
	reply.Success = true
}

// 实现leader的心跳机制，后续用来日志复制
func (rf *Raft) SendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// 开始选举
/*
使用 n-1 个协程向其他节点并行地发送 RequestVote 请求。
统计过程中，若发现失去了 Candidate 身份，则停止统计。
若获得票数过半，则成功当选 Leader，广播心跳并进行日志复制。
*/
func (rf *Raft) startElection() { //选举已在锁中，无需继续内部加锁
	rf.votedFor = rf.me
	rf.currentTerm++
	rf.persist() // 有任期更新，需要持久化
	rf.electionTimer.Reset(randomElectionTimeout())
	Debug(dTimer, "S%d start election", rf.me)
	voteGrantedCnt := 1 //先给自己投一票

	// 发送投票请求
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		args := &RequestVoteArgs{
			Term:         rf.currentTerm,
			CandidateId:  rf.me,
			LastLogIndex: rf.lastLogIndex(),
			LastLogTerm:  rf.lastLogTerm(),
		}
		Debug(dVote, "sendRequestVote S%d -> S%d, args{%+v}", rf.me, i, args)
		// 并发发送请求，并处理投票完的结果
		go func(i int) {
			reply := &RequestVoteReply{}
			// 投票RPC未得到响应
			if ok := rf.sendRequestVote(i, args, reply); !ok {
				return
			}
			// 在发完RPC请求后再上锁
			rf.mu.Lock()
			defer rf.mu.Unlock()
			defer func() {
				Debug(dVote, "sendRequestVote S%d's reply, grant %v at term: %d", i, reply.VoteGranted, reply.Term)
			}()
			if rf.currentTerm != args.Term || rf.state != Candidate { // 已经到下一个任期了或者不是Candidate了，提前结束
				return
			}

			// 对方任期大于自己任期，更新自己任期，重置投票，转换为Follower
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.votedFor = noVote
				rf.persist() // 持久化
				rf.state = Follower
			} else if reply.Term == rf.currentTerm && rf.state == Candidate {
				// 如果对方任期等于自己任期，并且自己是Candidate，则收集投票
				if reply.VoteGranted {
					voteGrantedCnt++
				}
				if voteGrantedCnt > len(rf.peers)/2 {
					rf.state = Leader
					// Leader上的易失性状态，选举后可能换leader需要重新初始化
					rf.initializeLeaderEasilyLostState()
					// 成为leader后立马广播心跳，防止选举超时
					rf.broadcastHeartbeat()
				}

			}
		}(i)
	}
}

// 后台心跳协程，监听定时器，如果是leader就广播心跳
func (rf *Raft) heartbeatEvent() {
	for !rf.killed() {
		<-rf.heartbeatTicker.C
		rf.mu.Lock()
		if rf.state == Leader {
			// 广播心跳
			rf.broadcastHeartbeat()
			rf.electionTimer.Reset(randomElectionTimeout()) // leader广播完毕时，也应该把自己的选举超时器刷新一下
		}
		rf.mu.Unlock()
	}
}

// 广播心跳及日志复制（包括快照复制）具体逻辑
func (rf *Raft) broadcastHeartbeat() {
	Debug(dTimer, "S%d start broadcast", rf.me)

	// 并行发送心跳，并更新日志
	for i := range rf.peers {
		if i == rf.me {
			continue
		}

		if rf.nextIndex[i] <= rf.lastIncludedIndex { //下一个要复制的日志在快照里面，发送安装快照RPC
			args := &InstallSnapshotArgs{
				Term:              rf.currentTerm,
				LeaderId:          rf.me,
				LastIncludedIndex: rf.lastIncludedIndex,
				LastIncludedTerm:  rf.logs[0].Term,
				Data:              rf.snapshot,
			}
			Debug(dSnap, `sendInstallSnapshot S%d -> S%d, args{LastIncludedIndex:%d,LastIncludedTerm:%d}`,
				rf.me, i, args.LastIncludedIndex, args.LastIncludedTerm)
			go rf.handleSendInstallSnapshot(i, args)
		} else { //下一个要复制的日志存在于未裁减的log中，发起日志复制RPC
			args := &AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: rf.nextIndex[i] - 1,
				Entries:      make([]LogEntry, 0),
				LeaderCommit: rf.commitIndex,
			}
			// 下一个日志在leader log里，且前一个日志没在快照里，也在leader log里
			if args.PrevLogIndex > rf.lastIncludedIndex &&
				args.PrevLogIndex < rf.lastIncludedIndex+len(rf.logs) {
				args.PrevLogTerm = rf.logs[rf.logIndex(args.PrevLogIndex)].Term
			} else if args.PrevLogIndex == rf.lastIncludedIndex { //下一个日志在leader log里，但上一个日志在快照里，没在leader log里
				args.PrevLogTerm = rf.logs[0].Term
			} else { // 排除极端情况：PrevLogIndex>=rf.lastIncludedIndex+len(rf.log) ,这种情况说明raft实现可能存在问题，因为leader的日志落后于follower了
				panic(fmt.Sprintf(`S%d -> S%d may meet some errors on raft implementation: PrevLogIndex(%d)>={lastIncludedIndex+len(log)(%d)`,
					rf.me, i, args.PrevLogIndex, rf.lastIncludedIndex+len(rf.logs)))
			}

			// 日志复制
			args.Entries = append(args.Entries, rf.logs[rf.logIndex(rf.nextIndex[i]):]...) //有快照后要用真实index在log中的索引
			Debug(dLog, `sendAppendEntries S%d -> S%d, args{PrevLogIndex:%d,PrevLogTerm:%d,LeaderCommit:%d,entries_len:%d}`,
				rf.me, i, args.PrevLogIndex, args.PrevLogTerm, args.LeaderCommit, len(args.Entries))
			go rf.handleSendAppendEntries(i, args)
		}
	}
}

// 处理发送快照的请求
func (rf *Raft) handleSendInstallSnapshot(peer int, args *InstallSnapshotArgs) {
	reply := &InstallSnapshotReply{}
	if ok := rf.sendInstallSnapshot(peer, args, reply); !ok {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dSnap, `sendInstallSnapshot S%d's reply, status{nextIndex:%d,matchIndex:%d,LastIncludedIndex:%d,LastIncludedTerm:%d}`,
			peer, rf.nextIndex[peer], rf.matchIndex[peer], rf.lastIncludedIndex, rf.logs[0].Term)
	}()

	if rf.currentTerm != args.Term || rf.state != Leader { //已经到下一个任期或不是leader，没有必要在进行广播
		return
	}

	// 若对方任期大于自己任期，更新自己任期，转换为Follower
	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.state = Follower
		rf.persist() // 持久化
		return
	}

	// 可能快照和日志复制reply同一时间到达，需要取两者的最大值
	rf.matchIndex[peer] = max(rf.matchIndex[peer], args.LastIncludedIndex)
	rf.nextIndex[peer] = max(rf.nextIndex[peer], args.LastIncludedIndex+1)
}

// 处理日志复制的请求
func (rf *Raft) handleSendAppendEntries(peer int, args *AppendEntriesArgs) {
	reply := &AppendEntriesReply{}
	if ok := rf.SendAppendEntries(peer, args, reply); !ok {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dLog, `sendAppendEntries S%d's reply, nextIndex:%d matchIndex:%d`, peer, rf.nextIndex[peer], rf.matchIndex[peer])
	}()

	if rf.currentTerm != args.Term || rf.state != Leader { //已经到下一个任期或不是leader，没有必要在进行广播
		return
	}
	// 若对方任期大于自己任期，更新自己任期，转换为Follower
	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.state = Follower
		rf.persist() // 持久化
		return
	}
	// 心跳成功或日志复制成功
	if reply.Success {
		// 可能快照和日志复制reply同一时间到达，需要取两者的最大值
		rf.matchIndex[peer] = max(rf.matchIndex[peer], args.PrevLogIndex+len(args.Entries))
		rf.nextIndex[peer] = max(rf.nextIndex[peer], args.PrevLogIndex+len(args.Entries)+1)

		//超过半数节点追加成功，也就是已提交，并且还是leader，那么就可以应用当前任期里的日志到状态机里。
		rf.checkAndCommitLogs()
	} else { //失败，由逐一减小尝试改为快速定位nextIndex
		rf.findNextIndex(peer, reply)
	}
}

// 判断是否能提交日志，并判断是否能进行应用日志到状态机
func (rf *Raft) checkAndCommitLogs() {
	N := rf.commitIndex
	//遍历对等点，找到相同的最大的那个N，即找最大的可以提交的日志索引
	for _N := rf.commitIndex + 1; _N < rf.realLogLen(); _N++ {
		succeedNum := 0
		for p := 0; p < len(rf.peers); p++ {
			if _N <= rf.matchIndex[p] && rf.logs[rf.logIndex(_N)].Term == rf.currentTerm {
				succeedNum++
			}
		}
		if succeedNum > len(rf.peers)/2 {
			N = _N
		}
	}

	if N > rf.commitIndex { //Leader可以应用日志了
		Debug(dLog, `S%d commit to index: %d, and lastIncludedIndex:%d`, rf.me, N, rf.lastIncludedIndex)
		rf.commitIndex = N
		rf.cond.Signal() //通知applierEvent应用日志
	}
}

// 快速定位nextIndex
func (rf *Raft) findNextIndex(peer int, reply *AppendEntriesReply) {
	// Case 3: follower's log is too short:
	// Follower落后leader日志时
	if reply.XTerm == -1 && reply.XIndex == -1 {
		rf.nextIndex[peer] = reply.XLen
		return
	}

	flag := false
	// Case 2: leader has XTerm，即Follower没落后但冲突
	// Follower返回在发现不一致后的当前任期中存储的第一条日志
	// Leader在其日志中搜索第一个条目任期等于conflictTerm的索引，回退后再复制
	for i, entry := range rf.logs {
		if entry.Term == reply.XTerm {
			flag = true
			rf.nextIndex[peer] = rf.realIndex(i) // 回退到冲突的地方，有快照后这里要用真实索引
		}
	}

	// leader doesn't have XTerm，即落后的节点成了leader，直接nextIndex = conflictIndex进行覆盖
	if !flag {
		rf.nextIndex[peer] = reply.XIndex
	}

}

// 应用日志到状态机
func (rf *Raft) applierEvent() {
	for !rf.killed() {
		rf.mu.Lock()
		for rf.commitIndex <= rf.lastApplied { //用for防止虚假唤醒
			rf.cond.Wait() //等待commitIndex变化唤醒该协程
		}
		msgs := make([]ApplyMsg, 0, rf.commitIndex-rf.lastApplied)
		for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
			msgs = append(msgs, ApplyMsg{
				CommandValid: true,
				Command:      rf.logs[rf.logIndex(i)].Cmd, //有快照后要用真实index在log中的索引
				CommandIndex: i,
			})
		}
		rf.mu.Unlock()
		Debug(dLog2, "S%d need apply msg{%+v}", rf.me, msgs)
		for _, msg := range msgs {
			rf.mu.Lock()
			if msg.CommandIndex != rf.lastApplied+1 { //下一个apply的log一定是lastApplied+1，否则就是被快照按照替代了
				rf.mu.Unlock()
				continue
			}
			// 如果执行到这里了，说明下一个msg一定是日志复制，而不是快照安装
			rf.replicatePreSnapshot.Store(true) // 设true让快要执行快照安装的协程要先阻塞等待一会
			rf.mu.Unlock()
			rf.applyMsg <- msg
			rf.replicatePreSnapshot.Store(false) // 解除快照安装协程的等待
			rf.mu.Lock()
			// 每apply一个log便及时更新lastApplied
			rf.lastApplied = max(msg.CommandIndex, rf.lastApplied)
			Debug(dLog2, "S%d lastApplied:%d", rf.me, rf.lastApplied)
			rf.mu.Unlock()
		}
	}
}

// 新增日志goroutine 来监听 addLog 事件，发送日志复制请求
func (rf *Raft) AddLogEvent() {
	for !rf.killed() {
		<-rf.addLogSignal
		rf.mu.Lock()
		if rf.state == Leader {
			// 发心跳进行日志复制
			rf.broadcastHeartbeat()
			// 刷新选举超时计时器
			rf.electionTimer.Reset(randomElectionTimeout())
			// 新增log引起的快速log复制算一次心跳，减少不必要的心跳发送
			rf.heartbeatTicker.Reset(HeartBeatTimeout())
		}
		rf.mu.Unlock()
	}
}

// 选举后，领导人的易失状态需要重新初始化
func (rf *Raft) initializeLeaderEasilyLostState() {
	defer func() {
		Debug(dLeader, "S%d become new leader, nextIndex:%v,matchIndex:%v", rf.me, rf.nextIndex, rf.matchIndex)
	}()

	// for i := 0; i < len(rf.peers); i++ {
	// 	rf.nextIndex[i] = len(rf.logs) //初始值为领导人最后的日志条目的索引+1
	// 	rf.matchIndex[i] = 0           //初始值为0
	// }
	//有快照机制之后
	for i := 0; i < len(rf.peers); i++ {
		rf.nextIndex[i] = rf.realLogLen()       //初始值为真实日志长度
		rf.matchIndex[i] = rf.lastIncludedIndex //初始值为快照点
	}

	// 领导人的nextIndex和matchIndex是确定的
	rf.matchIndex[rf.me] = rf.lastLogIndex()
	rf.nextIndex[rf.me] = rf.lastLogIndex() + 1
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
/*
使用Raft的服务（例如，键值服务器）希望开始对下一个要追加到Raft日志的命令进行协议。如果该服务器不是领导者，则返回false。否则，启动协议并立即返回。
不能保证该命令会被提交到Raft日志，因为领导者可能会失败或失去选举。即使Raft实例已经被终止，该函数也应该平稳地返回。
第一个返回值是该命令如果最终提交，会出现在Raft日志中的索引。第二个返回值是当前的任期。第三个返回值是如果服务器认为它是领导者，则返回true。
*/
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := false

	// Your code here (3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != Leader {
		return -1, -1, false
	}

	// Leader追加日志
	rf.logs = append(rf.logs, LogEntry{
		Cmd:  command,
		Term: rf.currentTerm,
	})

	rf.persist() //持久化

	rf.matchIndex[rf.me] = rf.lastLogIndex()
	rf.nextIndex[rf.me] = rf.realLogLen() //有快照机制后要返回真实长度

	index = rf.lastLogIndex()
	term = rf.currentTerm
	isLeader = true

	//唤醒追加日志协程
	go func() {
		rf.addLogSignal <- struct{}{}
	}()

	Debug(dClient, "S%d Start cmd:%v,index:%d", rf.me, command, index)
	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// 监控 electon timer 的 go routine 来开始选举
func (rf *Raft) ticker() {
	for !rf.killed() {
		// Your code here (3A)
		// Check if a leader election should be started.

		//监听选举超时定时器
		<-rf.electionTimer.C
		rf.mu.Lock()
		rf.state = Candidate
		// 开始选举
		rf.startElection()
		rf.mu.Unlock()
	}
}
