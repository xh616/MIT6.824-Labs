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
	return rf.logs[len(rf.logs)-1].Term
}

func (rf *Raft) lastLogIndex() int {
	return len(rf.logs) - 1
}

// 论文里安全性的保证：参数的日志是否至少和自己一样新
func (rf *Raft) isLogUpToDate(lastLogTerm, lastLogIndex int) bool {
	return lastLogTerm > rf.lastLogTerm() || (lastLogTerm == rf.lastLogTerm() && lastLogIndex >= rf.lastLogIndex())
}
