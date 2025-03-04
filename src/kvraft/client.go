package kvraft

import (
	"crypto/rand"
	"math/big"

	"6.5840/labrpc"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	leader     int   // 缓存当前的 leader
	clientId   int64 // 客户端的唯一标识
	sequenceId int64 // 客户端的请求Id，重复RPC具有相同的此项值
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.leader = 0
	ck.clientId = nrand()
	ck.sequenceId = 0
	return ck
}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer."+op, &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
// 串行调用的，无需上锁（保证每个客户端一次只能一次rpc）
func (ck *Clerk) Get(key string) string {
	defer func() {
		Debug(dGet, "C%d(Seq %d) -> S%d Get {key:%s}", ck.clientId, ck.sequenceId, ck.leader, key)
	}()
	args := &GetArgs{
		Key:        key,
		ClientId:   ck.clientId,
		SequenceId: ck.sequenceId,
	}
	ck.sequenceId++ // 递增请求Id
	server := ck.leader
	for {
		reply := &GetReply{}
		//网络正常并且相应请求的也是leader
		if ok := ck.servers[server].Call("KVServer.Get", args, reply); ok && reply.Leader {
			ck.leader = server
			return reply.Value
		} else { //网络分区或请求的不是leader，请求下一个
			server = (server + 1) % len(ck.servers)
		}
	}
}

// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	args := &PutAppendArgs{
		Key:        key,
		Value:      value,
		Op:         op,
		ClientId:   ck.clientId,
		SequenceId: ck.sequenceId,
	}
	ck.sequenceId++ // 递增请求Id
	server := ck.leader
	for {
		reply := &PutAppendReply{}
		//网络正常并且相应请求的也是leader
		if ok := ck.servers[server].Call("KVServer.PutAppend", args, reply); ok && reply.Leader {
			ck.leader = server
			return
		} else { //网络分区或请求的不是leader，请求下一个
			server = (server + 1) % len(ck.servers)
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
