# Lab2 Key/Value Server

## 实验网站地址

[https://pdos.csail.mit.edu/6.824/labs/lab-mr.html](https://pdos.csail.mit.edu/6.824/labs/lab-kvsrv.html)

### 主要代码

`src/kvsrv`下的文件

## **实验要求**

在本次 Lab 中，你将在单机上构建一个 key/value 服务器，以确保即使网络出现故障，每个操作也只能执行一次，并且操作是可线性化的。

客户端可以向 key/value 服务器发送三个不同的 RPC： `Put(key, value)` 、 `Append(key, arg)` 和` Get(key)` 。服务器在内存中维护 key/value 对的`map`。键和值是字符串。 `Put(key, value)` 设置或替换map中给定键的值， `Append(key, arg) `将 arg 附加到键的值并返回旧值，` Get(key) `获取键的当前值。不存在的键的 Get请求应返回空字符串；对于不存在的键的 Append 请求应该表现为现有值是零长度字符串。每个客户端都通过`Clerk`的 `Put/Append/Get `方法与服务器进行通信。 `Clerk` 管理与服务器的 RPC 交互。

你的服务器必须保证应用程序对`Clerk Get/Put/Append `方法的调用是线性一致的。 如果客户端请求不是**并发**的，每个客户端 `Get/Put/Append `调用时能够看到之前调用序列导致的状态变更。 对于并发的请求来说，返回的结果和最终状态都必须和这些操作顺序执行的结果一致。如果一些请求在时间上重叠，则它们是并发的：例如，如果客户端 X 调用 `Clerk.Put()` ，并且客户端 Y 调用 `Clerk.Append() `，然后客户端 X 的调用 返回。 一个请求必须能够看到已完成的所有调用导致的状态变更。

一个应用实现线性一致性就像一台单机服务器一次处理一个请求的行为一样简单。 例如，如果一个客户端发起一个更新请求并从服务器获取了响应，随后从其他客户端发起的读操作可以保证能看到改更新的结果。在单台服务器上提供线性一致性是相对比较容易的。

Lab 在 `src/kvsrv` 中提供了框架代码和单元测试。你需要更改` kvsrv/client.go`、`kvsrv/server.go `和 `kvsrv/common.go` 文件。

## 实现

### 1、无网络故障的key/value 服务器

您的第一个任务是实现一个在没有丢失消息的情况下有效的解决方案。需要在 `client.go` 中，在 `Clerk`的 `Put/Append/Get` 方法中添加 RPC 的发送代码；并且实现 `server.go` 中 Put、Append、Get 三个 RPC handler。

当你通过了前两个测试 case：one client、many clients 时表示完成该任务。

使用 go test -race 检查您的代码是否没有争用。

- `server.go`

```go
type KVServer struct {
	mu sync.Mutex

	// Your definitions here.
	// 存储key-value的map
	data map[string]string
}


func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	// 加锁
	kv.mu.Lock()
	// 解锁
	defer kv.mu.Unlock()
	// 获取key对应的value
	reply.Value = kv.data[args.Key]
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	// 加锁
	kv.mu.Lock()
	// 解锁
	defer kv.mu.Unlock()
	// 存储key-value
	kv.data[args.Key] = args.Value
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	// 加锁
	kv.mu.Lock()
	// 解锁
	defer kv.mu.Unlock()
	// 追加value并返回旧值
	oldValue := kv.data[args.Key]
	kv.data[args.Key] = oldValue + args.Value	
	reply.Value = oldValue
}

func StartKVServer() *KVServer {
	kv := new(KVServer)

	// You may need initialization code here.
	kv.data = make(map[string]string)

	return kv
}
```

- `client.go`

```go
func (ck *Clerk) Get(key string) string {
	// 创建GetArgs
	args := GetArgs{Key: key}
	// 创建GetReply
	reply := GetReply{}
	// 发送RPC请求
	ck.server.Call("KVServer.Get", &args, &reply)
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
	args := PutAppendArgs{Key: key, Value: value}
	// 创建PutAppendReply
	reply := PutAppendReply{}
	// 发送RPC请求
	ck.server.Call("KVServer."+op, &args, &reply)
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
```

### 2、可能丢弃消息的Key/value 服务器

现在，您应该修改您的解决方案，以便在遇到丢失的消息（例如 RPC 请求和 RPC 回复）时继续工作。如果消息丢失，则客户端的` ck.server.Call() `将返回 `false` （更准确地说， `Call()` 等待响应直至超时，如果在此时间内没有响应就返回`false`）。您将面临的一个问题是 `Clerk `可能需要多次发送 RPC，直到成功为止。但是，**每次调用 `Clerk.Put()` 或` Clerk.Append() `应该只会导致一次执行，因此您必须确保重新发送不会导致服务器执行请求两次。**

你的任务是在 `Clerk` 中添加重试逻辑，并且在 `server.go` 中来过滤重复请求。

1. 您需要唯一地标识client操作，以确保Key/value服务器仅执行每个操作一次。

2. 您必须仔细考虑server必须维持什么状态来处理重复的` Get()` 、 `Put() `和` Append() `请求（如果有的话）。
3. 您的重复检测方案应该**快速释放服务器内存**，例如让每个 RPC 暗示client已看到其前一个 RPC 的回复。可以假设client一次只向Clerk发起一次调用。

**方案**

为`Put`和`Append`请求添加标识ID（`Get`请求只需不断重试，不会有影响）

使用`Map`用于在Key/value服务器中跟踪处理过的请求ID，以防止重复处理请求。

为解决请求丢失的现象，给`Put`和`Append`请求设置两个状态，首先是`修改`状态，待接收到修改完成的消息后改为`汇报`状态，向server汇报，server接收到完成请求后，再把`Map`中的请求记录删除，代表完成

![image-20241218170248257](Lab2%20KeyValue%20Server.assets/image-20241218170248257.png)

#### 实现

`common.go`

```go
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
```

`client.go`

```go
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
```

`server.go`

```go
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
```

## 运行测试

`cd kvsrv`

`go test`

**可以通过所有测试**

![image-20250304214345249](Lab2%20KeyValue%20Server.assets/image-20250304214345249.png)