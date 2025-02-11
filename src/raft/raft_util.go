package raft

func (rf *Raft) lastLogTerm() int {
	if len(rf.logs) < 2 { //只能从快照拿最后日志的term
		return rf.logs[0].Term
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

// 论文里安全性的保证：参数的日志是否至少和自己一样新
func (rf *Raft) isLogUpToDate(lastLogTerm, lastLogIndex int) bool {
	return lastLogTerm > rf.lastLogTerm() || (lastLogTerm == rf.lastLogTerm() && lastLogIndex >= rf.lastLogIndex())
}
