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
	"log"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
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
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
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
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (3D).

}

// example RequestVote RPC handler.
// 获取投票请求
// 当 Candidate 任期小于当前节点任期时，返回 false（放弃 Candidate 身份并更新自己的任期）。
// 如果 votedFor 为 -1（即当前任期内此节点还未投票）或者 votedFor为 candidateId（即当前任期内此节点已经向此 Candidate 投过票），则同意投票；否则拒绝投票。
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// 如果Candidate任期小于我的任期，则不投票
	if args.Term < rf.currentTerm {
		// 告知Candidate我的任期
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// 如果Candidate任期大于我的任期，则更新我的任期并重置我的投票，并转为Follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = -1 // 无论何时，任期更新则重置投票
		rf.state = Follower
	}

	// 如果已经投票并且没投给这个Candidate，则返回失败
	if rf.votedFor != -1 && rf.votedFor != args.CandidateId {
		reply.VoteGranted = false
		reply.Term = rf.currentTerm // 告知Candidate我的任期
		return
	}

	//投票处理，并重置选举超时时间
	rf.votedFor = args.CandidateId
	rf.electionTimer.Reset(randomElectionTimeout())
	reply.VoteGranted = true
	reply.Term = rf.currentTerm
	log.Printf("In term %d, machine %d vote for machine %d", rf.currentTerm, rf.me, args.CandidateId)
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
	// 如果leader任期小于我的任期，则不接受该leader
	if rf.currentTerm > args.Term {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}
	// 如果leader任期大于我的任期，则更新我的任期并重置我的投票，并转为Follower
	if rf.currentTerm < args.Term {
		rf.currentTerm = args.Term
		rf.votedFor = -1
		rf.state = Follower
	}
	// leader任期等于我的任期时，接受该leader心跳并重置选举超时计数器
	reply.Term = rf.currentTerm
	reply.Success = true
	rf.electionTimer.Reset(randomElectionTimeout())
}

// 实现leader的心跳机制，后续用来日志复制
func (rf *Raft) SendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
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
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (3B).

	return index, term, isLeader
}

// 开始选举
/*
使用 n-1 个协程向其他节点并行地发送 RequestVote 请求。协程获得 response 后，向 voteCh 发送结果，startElection 协程进行结果统计。
统计过程中，若发现失去了 Candidate 身份，则停止统计。若获得票数过半，则成功当选 Leader，启动 heartbeat 协程。
若所有成员已投票，且未当选 Leader，则退出统计。
要注意的是，for 循环会一直试图从 voteCh 中读取，因此需要确保 voteCh 不会被阻塞。
*/
func (rf *Raft) startElection() {
	rf.mu.Lock()
	rf.votedFor = rf.me
	rf.currentTerm++
	rf.electionTimer.Reset(randomElectionTimeout())

	// 发送投票请求
	args := &RequestVoteArgs{Term: rf.currentTerm, CandidateId: rf.me}
	rf.mu.Unlock()
	// 定义一个channel，用于接收投票结果
	voteCh := make(chan bool, len(rf.peers)-1)
	// 跟踪所有发送协程，确保完成后关闭voteCh，使统计循环正常退出
	var wg sync.WaitGroup
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		wg.Add(1)
		// 并发发送请求，并处理投票完的结果
		go func(i int) {
			defer wg.Done()
			reply := &RequestVoteReply{}
			// 投票RPC未得到响应
			if ok := rf.sendRequestVote(i, args, reply); !ok {
				voteCh <- false
				return
			}
			// 在发完RPC请求后再上锁
			rf.mu.Lock()
			// 对方任期大于自己任期，更新自己任期，重置投票，转换为Follower
			if reply.Term > rf.currentTerm {
				rf.state = Follower
				rf.currentTerm = reply.Term
				rf.votedFor = -1
				rf.mu.Unlock()
				return
			}
			rf.mu.Unlock()
			// 收集一个投票
			voteCh <- reply.VoteGranted
		}(i)
	}

	// 单独协程等待所有RPC完成并关闭通道
	go func() {
		wg.Wait()
		close(voteCh)
	}()

	// 统计投票结果
	voteGrantedCnt := 1
	for voteGranted := range voteCh {
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		// 因为我可能在此过程中因任期小已经变为Follower了
		if state != Candidate {
			return
		}
		if voteGranted {
			voteGrantedCnt++
		}
		if voteGrantedCnt > len(rf.peers)/2 {
			rf.mu.Lock()
			// 如果我还是Candidate
			if rf.state == Candidate {
				rf.state = Leader
				// 启动心跳
				go rf.heartbeat()
			}
			rf.mu.Unlock()
			return
		}
	}
}

// 心跳
func (rf *Raft) heartbeat() {
	//成为leader后立马发一次心跳
	rf.broadcastHeartbeat()

	//设置定时器，周期性发送心跳
	rf.heartbeatTimer = time.NewTicker(HeartBeatTimeout)
	defer rf.heartbeatTimer.Stop()
	for range rf.heartbeatTimer.C {
		rf.broadcastHeartbeat()
	}
}

// 广播心跳具体逻辑
func (rf *Raft) broadcastHeartbeat() {
	// 如果不是Leader或者已经被杀死，则退出
	if _, isLeader := rf.GetState(); !isLeader || rf.killed() {
		return
	}
	// 并行发送心跳
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		rf.mu.Lock() //锁任期
		args := &AppendEntriesArgs{Term: rf.currentTerm, LeaderId: rf.me}
		rf.mu.Unlock()
		reply := &AppendEntriesReply{}
		go func(server int) {
			if ok := rf.SendAppendEntries(server, args, reply); !ok {
				return
			}
			rf.mu.Lock()
			// 若对方任期大于自己任期，更新自己任期，重置投票，转换为Follower
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.votedFor = -1
				rf.state = Follower
				rf.mu.Unlock()
				return
			}
			rf.mu.Unlock()
		}(i)
	}
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

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		// ms := 50 + (rand.Int63() % 300)
		// time.Sleep(time.Duration(ms) * time.Millisecond)

		//监听选举超时定时器
		<-rf.electionTimer.C
		rf.mu.Lock()
		if rf.state == Leader {
			rf.mu.Unlock()
			break
		}
		rf.state = Candidate
		rf.mu.Unlock()
		// 开始选举
		go rf.startElection()
		// 为什么将选举过程也作为一个 go routine，而不是阻塞地调用函数？
		// 1、避免阻塞主循环：如果直接调用 rf.startElection()，选举过程会阻塞当前协程，导致无法处理其他事件（如心跳、日志复制等）。
		// 2、支持超时重试：如果选举超时（electionTimer 再次触发），而当前选举仍未完成，Raft 需要能够启动新一轮选举。如果选举过程是同步的，就无法实现这一点
	}
}