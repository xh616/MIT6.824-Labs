package kvsrv

const (
	Modify = iota // 0 修改操作
	Report        // 1 报告操作
	/*
		为解决请求丢失的现象，给Put和Append请求设置两个状态，
		首先是修改状态，待接收到修改完成的消息后改为汇报状态，
		向server汇报，server接收到完成请求后，再把Map中的请求记录删除，代表完成
	*/
)

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	RequestId   int64 //客户端请求id，nrand函数返回类别是int64
	RequestType int   //请求操作类别（0 修改操作，1 报告操作）
}

type PutAppendReply struct {
	Value string 
}

type GetArgs struct {
	Key string
}

type GetReply struct {
	Value string
}
