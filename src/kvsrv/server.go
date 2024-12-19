package kvsrv

import (
	"log"
	"sync"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type KVServer struct {
	mu sync.Mutex

	// Your definitions here.
	// 存储key-value的map
	data map[string]string
	// 记录客户端请求的Map,key为客户端请求id,value为值
	MessageMap map[int64]string // 0 修改状态，1 汇报状态
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	// 加锁
	kv.mu.Lock()
	// 解锁
	defer kv.mu.Unlock()
	// 获取key对应的value
	reply.Value = kv.data[args.Key]
	// Get请求不用记录,因为Get是读操作,不需要等待其他操作完成
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	// 加锁
	kv.mu.Lock()
	// 解锁
	defer kv.mu.Unlock()
	// Your code here.
	if args.RequestType == Report {
		// 报告操作删除请求记录并返回
		delete(kv.MessageMap, args.RequestId)
		return
	}
	// 如果请求id在MessageMap中，说明请求已经完成，直接返回
	if _, ok := kv.MessageMap[args.RequestId]; ok {
		return
	}
	// 存储key-value
	kv.data[args.Key] = args.Value
	// 记录客户端请求
	kv.MessageMap[args.RequestId] = args.Value
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	// 加锁
	kv.mu.Lock()
	// 解锁
	defer kv.mu.Unlock()
	// Your code here.
	if args.RequestType == Report {
		// 报告操作删除请求记录并返回
		delete(kv.MessageMap, args.RequestId)
		return
	}
	// 如果请求id在MessageMap中，说明请求已经完成，直接返回之前的结果
	if res, ok := kv.MessageMap[args.RequestId]; ok {
		reply.Value = res
		return
	}
	// 追加value并返回旧值（题目要求）
	oldValue := kv.data[args.Key]
	reply.Value = oldValue
	kv.data[args.Key] = oldValue + args.Value
	// 记录客户端请求
	kv.MessageMap[args.RequestId] = oldValue
}

func StartKVServer() *KVServer {
	kv := new(KVServer)
	// 初始化数据
	// You may need initialization code here.
	kv.data = make(map[string]string)
	kv.MessageMap = make(map[int64]string)

	return kv
}
