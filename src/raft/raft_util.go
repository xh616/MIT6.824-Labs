package raft

// 打印用
func StateToString(s int) string {
	switch s {
	case Follower:
		return "follower"
	case Candidate:
		return "candidate"
	case Leader:
		return "leader"
	default:
		return "NO NAME"
	}
}

func (rf *Raft) lastLogTerm() int {
	if len(rf.logs) < 2 {//只能从快照拿最后日志的term
		return rf.logs[0].Term //快照点的任期
	}
	return rf.logs[len(rf.logs)-1].Term
}

func (rf *Raft) lastLogIndex() int {
	// 快照：{nil 1 2 3 4 5 6 7 8 9} {nil,1}
	// lastIndex: 9+2-1=10
	// 无快照：{nil 1 2 3 4 5 6 7 8 9}
	// lastIndex: 10-1=9
	return rf.lastIncludedIndex + len(rf.logs) - 1
}

// 真实index在log中的索引
func (rf *Raft) logIndex(realIndex int) int {
	// 快照：{nil 1 2 3 4 5 6 7 8 9} {nil,10,11}
	// logIndex: 10-9 = 1
	// 无快照：{nil 1 2 3 4 5 6 7 8 9 10}
	// logIndex: 10-0=10
	return realIndex - rf.lastIncludedIndex
}

// log中的索引在log和快照里的索引
func (rf *Raft) realIndex(logIndex int) int {
	// 快照：{nil 1 2 3 4 5 6 7 8 9} {nil,10,11}
	// realIndex: 9+1 = 10
	// 无快照：{nil 1 2 3 4 5 6 7 8 9 10}
	// realIndex: 0+1=1
	return rf.lastIncludedIndex + logIndex
}

// 真实日志长度
func (rf *Raft) realLogLen() int {
	// 快照：{nil 1 2 3 4 5 6 7 8 9} {nil,10,11}
	// realLogLen: 9+3=12
	// 无快照：{nil 1 2 3 4 5 6 7 8 9}
	// realLogLen: 0+10
	return rf.lastIncludedIndex + len(rf.logs)
}

// 论文里安全性的保证：参数的日志是否至少和自己一样新
func (rf *Raft) isLogUpToDate(lastLogTerm, lastLogIndex int) bool {
	return lastLogTerm > rf.lastLogTerm() || (lastLogTerm == rf.lastLogTerm() && lastLogIndex >= rf.lastLogIndex())
}
