package kvsrv

import (
	"crypto/rand"
	"math/big"

	"6.5840/labrpc"
)

type Clerk struct {
	server *labrpc.ClientEnd
	// You will have to modify this struct.
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(server *labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.server = server
	// You'll have to add code here.
	return ck
}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.server.Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) Get(key string) string {
	// 创建GetArgs
	args := GetArgs{Key: key}
	// 创建GetReply
	reply := GetReply{Value: ""}
	// 发送RPC请求，直到成功
	for !ck.server.Call("KVServer.Get", &args, &reply) {
	}
	// 返回获取到的value
	return reply.Value
}

// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.server.Call("KVServer."+op, &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) PutAppend(key string, value string, op string) string {
	// You will have to modify this function.
	// 创建PutAppendArgs
	// args := PutAppendArgs{Key: key, Value: value}
	// 修改请求
	args := PutAppendArgs{
		Key:         key,
		Value:       value,
		RequestId:   nrand(),
		RequestType: Modify,
	}
	// 创建PutAppendReply
	reply := PutAppendReply{Value: ""}
	// 发送修改RPC请求，直到成功
	for !ck.server.Call("KVServer."+op, &args, &reply) {
	}
	// 报告请求，定义新参不然会报labgob warning: Decoding into a non-default variable/field Value may not work
	reportArgs := args
	reportArgs.RequestType = Report
	reportReply := PutAppendReply{Value: ""}
	// 发送报告RPC请求，直到成功
	for !ck.server.Call("KVServer."+op, &reportArgs, &reportReply) {
	}
	// 返回获取到的value
	return reply.Value
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}

// Append value to key's value and return that value
func (ck *Clerk) Append(key string, value string) string {
	return ck.PutAppend(key, value, "Append")
}
