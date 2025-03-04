package kvraft

import (
	"bytes"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raft"
)

// const Debug = false

// func DPrintf(format string, a ...interface{}) (n int, err error) {
// 	if Debug {
// 		log.Printf(format, a...)
// 	}
// 	return
// }

type OpType string

const (
	GET    OpType = "Get"
	PUT    OpType = "Put"
	APPEND OpType = "Append"
)

// 客户端等待 RPC 响应的超时时间
// HeartBeatTimeout < rpcTimeout < ElectionTimeout
const rpcTimeout = 100 * time.Millisecond

// 客户端的请求参数封装成Op结构体
type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Type       OpType
	Key        string
	Value      string
	ClientId   int64
	SequenceId int64
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	// 何时快照
	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	persister   *raft.Persister
	lastApplied int // 用于快照，由底层raft维护

	duptable   map[int64]int64                // raft的command判重，键是 ClientId，值是已经处理过的最大 SequenceId
	data       map[string]string              // 存储key-value的map
	wakeClient map[int]chan reqIdentification // 存储每个index处的请求编号，用以唤醒对应客户端
}

// 唯一对应一个请求
type reqIdentification struct {
	ClientId   int64
	SequenceId int64
	Err        Err
}

func (kv *KVServer) clean(i int) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.wakeClient, i) //清除map记录
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	defer func() {
		Debug(dGet, "S%d(%s) Get args{%+v} reply{%+v}", kv.me, kv.rf.GetStringState(), args, reply)
	}()

	op := Op{
		Type:       GET,
		Key:        args.Key,
		ClientId:   args.ClientId,
		SequenceId: args.SequenceId,
	}

	// 调用raft.Start()追加一个日志发送给raft
	// 如果是leader则会追加成功；否则会失败并返回当前不是leader的错误，客户端定位下一个server
	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.Value = ""
		reply.Err = ErrWrongLeader
		reply.Leader = false
		return
	}

	// 不成功client会一直重试，直到成功，走到这里说明client成功了
	// 处理map要加锁
	kv.mu.Lock()
	// 当前日志索引处初始化一个channel，用于在 Raft 日志提交后唤醒等待的客户端
	wakeCh := make(chan reqIdentification)
	kv.wakeClient[index] = wakeCh
	kv.mu.Unlock()

	// 延迟释放资源
	defer func() {
		kv.clean(index)
	}()

	select {
	case <-time.After(rpcTimeout): //超时还没有提交
	case r := <-wakeCh: //阻塞等待唤醒
		//是对应请求的响应
		if r.ClientId == args.ClientId && r.SequenceId == args.SequenceId {
			kv.mu.Lock()
			// 从 kv.data 中查找键对应的值，并返回给客户端
			// raft达成共识后，数据已经被应用到状态机，可以直接读取
			if val, ok := kv.data[args.Key]; ok {
				reply.Value = val
				reply.Err = OK
				reply.Leader = true
			} else { // 没有找到对应的键
				reply.Value = ""
				reply.Err = ErrNoKey
				reply.Leader = true
			}
			kv.mu.Unlock()
			return
		}
	}

	// 超时后的逻辑
	reply.Value = ""
	reply.Err = ErrWrongLeader
	reply.Leader = false
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	defer func() {
		Debug(dAppend, "S%d(%s) PutAppend args{%+v} reply{%+v}", kv.me, kv.rf.GetStringState(), args, reply)
	}()

	op := Op{
		Type:       OpType(args.Op),
		Key:        args.Key,
		Value:      args.Value,
		ClientId:   args.ClientId,
		SequenceId: args.SequenceId,
	}

	kv.mu.Lock()
	// put和append会影响状态机，所以要判重，如果raft已经成功响应请求过了，可以直接返回了
	if seq, ok := kv.duptable[args.ClientId]; ok && seq >= args.SequenceId {
		kv.mu.Unlock()
		reply.Leader = true
		reply.Err = OK
		return
	}
	kv.mu.Unlock()

	// 调用raft.Start()追加一个日志发送给raft
	// 如果是leader则会追加成功；否则会失败并返回当前不是leader的错误，客户端定位下一个server
	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.Leader = false
		reply.Err = ErrWrongLeader
		return
	}

	// 不成功client会一直重试，直到成功，走到这里说明client成功了
	// 处理map要加锁
	kv.mu.Lock()
	// 当前日志索引处初始化一个channel，用于在 Raft 日志提交后唤醒等待的客户端
	wakeCh := make(chan reqIdentification)
	kv.wakeClient[index] = wakeCh
	kv.mu.Unlock()

	// 延迟释放资源
	defer func() {
		kv.clean(index)
	}()

	select {
	case <-time.After(rpcTimeout): //超时还没有提交
	case r := <-wakeCh: //阻塞等待唤醒
		//是对应请求的响应，直接返回，因为apply协程里面已经做了修改数据的操作
		if r.ClientId == args.ClientId && r.SequenceId == args.SequenceId {
			reply.Leader = true
			reply.Err = OK
			return
		}
	}

	// 超时后的逻辑
	reply.Leader = false
	reply.Err = ErrWrongLeader
}

// raft提交的command，应用层应用，这个是针对所有的server
// 对于leader：把所有已提交的command，执行并应用到状态机中；并且leader也需要让客户端reply
// 对于follower：也需要把已提交的command，执行并应用到状态机中；follower没有客户端请求，无需等待
// 应用状态机的时候，只需要应用Put/Append即可，Get不会对状态机造成任何影响
func (kv *KVServer) apply() {
	for msg := range kv.applyCh { //遍历raft的应用管道，来一个取一个，没有会被阻塞
		if kv.killed(){
			return
		}
		kv.mu.Lock()

		// 是日志
		if msg.CommandValid {
			// 出现这种情况，代表快照已经被加载了
			if msg.CommandIndex <= kv.lastApplied {
				kv.mu.Unlock()
				continue
			}
			op := msg.Command.(Op) //Command是interface{}类型，需要转换成Op类型，.()是空接口的类型断言
			// 命令不重复，或者命令Id大于之前的命令Id  -> 应用状态机，并记录table
			if PreSequenceId, ok := kv.duptable[op.ClientId]; !ok || PreSequenceId < op.SequenceId {
				kv.duptable[op.ClientId] = op.SequenceId
				switch op.Type {
				case PUT:
					kv.data[op.Key] = op.Value
				case APPEND:
					if _, ok := kv.data[op.Key]; ok {
						// kv.data[op.Key] = kv.data[op.Key] + op.Value
						// 使用strings.Builder高效拼接字符串
						build := strings.Builder{}
						build.WriteString(kv.data[op.Key])
						build.WriteString(op.Value)
						kv.data[op.Key] = build.String()
					} else {
						kv.data[op.Key] = op.Value
					}
				case GET:
					// do nothing
				}
			}
			// 唤醒客户端
			if wakeCh, ok := kv.wakeClient[msg.CommandIndex]; ok {
				Debug(dClient, "S%d wakeup client", kv.me)
				wakeCh <- reqIdentification{
					ClientId:   op.ClientId,
					SequenceId: op.SequenceId,
					Err:        OK,
				}
			}
			Debug(dApply, "apply msg{%+v}", msg)
			kv.lastApplied = msg.CommandIndex // 直接依赖底层raft的实现，不在应用层自己维护lastApplied
			kv.snapshot(msg.CommandIndex)     // 将定期快照和follower应用快照串行化处理
		} else if msg.SnapshotValid { // 是快照
			snapshotStatus := &SnapshotStatus{}
			// 快照解码失败
			if err := labgob.NewDecoder(bytes.NewBuffer(msg.Snapshot)).Decode(snapshotStatus); err != nil {
				Debug(dError, "snapshot gob encode snapshotStatus err:%v", err)
				kv.mu.Unlock()
				return
			}
			//从快照中恢复状态
			kv.lastApplied = msg.SnapshotIndex
			kv.data = snapshotStatus.Data
			kv.duptable = snapshotStatus.Duptable
		}
		kv.mu.Unlock()
	}
}

// 快照处理
func (kv *KVServer) snapshot(lastApplied int) {
	if kv.maxraftstate == -1 { //无需快照
		return
	}

	// RaftStateSize达到maxraftstate的0.9，就进行快照
	// 先强转float64再除是为了避免整数除法
	rate := float64(kv.persister.RaftStateSize()) / float64(kv.maxraftstate)
	if rate >= 0.9 {
		snapshotStatus := &SnapshotStatus{
			Data:     kv.data,
			Duptable: kv.duptable,
		}

		w := new(bytes.Buffer)
		// labgob 解码器
		if err := labgob.NewEncoder(w).Encode(snapshotStatus); err != nil {
			Debug(dError, "snapshot gob encode snapshotStatus err:%v", err)
			return
		}
		// 裁剪日志，存储快照
		kv.rf.Snapshot(lastApplied, w.Bytes())
	}
}

// 读取快照
func (kv *KVServer) readPersist() {
	snapshot := kv.persister.ReadSnapshot()
	if len(snapshot) < 1 { //没有快照
		return
	}
	snapshotStatus := &SnapshotStatus{}
	if err := labgob.NewDecoder(bytes.NewBuffer(snapshot)).Decode(snapshotStatus); err != nil {
		Debug(dError, "snapshot gob encode snapshotStatus err:%v", err)
		return
	}
	kv.data = snapshotStatus.Data
	kv.duptable = snapshotStatus.Duptable
}

// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})
	applyCh := make(chan raft.ApplyMsg)

	// 初始化kv
	kv := &KVServer{
		me:           me,
		rf:           raft.Make(servers, me, persister, applyCh), //channel是引用类型，直接传递给了raft做修改
		applyCh:      applyCh,
		dead:         0,
		persister:    persister,
		maxraftstate: maxraftstate,
		lastApplied:  0,
		duptable:     make(map[int64]int64),
		data:         make(map[string]string),
		wakeClient:   make(map[int]chan reqIdentification),
	}

	// 读取快照
	kv.readPersist()

	go kv.apply() //开个协程处理applyCh中的command

	return kv
}
