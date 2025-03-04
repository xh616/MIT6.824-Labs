# Lab 4: 容错key/value服务

## 实验网站地址

http://nil.csail.mit.edu/6.5840/2024/labs/lab-kvraft.html

### 主要代码

`src/kvraft`下的文件

## **实验要求**

在本实验中，您将构建一个容错的key/value存储系统 使用您的 Raft 库的服务 [实验 3](http://nil.csail.mit.edu/6.5840/2024/labs/lab-raft.html). 你的key/value服务将是一个复制的状态机，由几个key/value服务器，每个服务器都维护一个key/value对数据库，如 [实验 2](http://nil.csail.mit.edu/6.5840/2024/labs/lab-raft.html)，但额外使用 Raft 进行复制。只要大多数服务器存活并能通信，您的键/值服务应继续处理客户端请求，尽管存在其他故障或网络分区。在实验 4 之后，您将实现[Raft 交互图](http://nil.csail.mit.edu/6.5840/2024/notes/raft_diagram.pdf)中显示的所有部分（Clerk、Service 和 Raft）。

![image-20250303184202681](Lab%204%20%E5%AE%B9%E9%94%99keyvalue%E6%9C%8D%E5%8A%A1.assets/image-20250303184202681.png)

客户端将以与实验 2 非常相似的方式与您的key/value服务交互。具体来说，客户端可以向key/value服务发送三种不同的 RPC：

- `Put(key, value)` : 替换数据库中特定键的值
- `Append(key, arg)` : 将参数附加到键的值（如果键不存在，则将现有值视为空字符串）
- `Get(key)` ：获取键的当前值（对于不存在的键返回空字符串）

key和value都是字符串。请注意，与实验 2 不同， `Put` 和 `Append` 都不应向客户端返回值。每个客户端通过具有 Put/Append/Get 方法的 `Clerk` 与服务进行通信。 `Clerk` 管理与服务器交互的RPC

您的服务必须确保对 `Clerk` 的 Get/Put/Append 方法的应用程序调用是可线性化的。如果一次只调用一个，Get/Put/Append 方法的行为应如同系统只有一个状态副本，并且每次调用都应观察到由前一系列调用所隐含的状态修改。对于并发调用，返回值和最终状态必须与操作按某种顺序一次执行一个时相同。如果调用在时间上重叠，则它们是并发的：例如，如果客户端 X 调用 `Clerk.Put()` ，客户端 Y 调用 `Clerk.Append()` ，然后客户端 X 的调用返回。一个调用必须观察到在调用开始之前已完成的所有调用的效果。

提供线性化性对于单个服务器来说相对容易。如果服务被复制，则会更难，因为所有服务器必须为并发请求选择相同的执行顺序，必须避免使用未更新的状态回复客户端，并且必须在故障后以保留所有已确认客户端更新的方式恢复其状态。

本实验分为两部分。在 A 部分中，您将使用您的 Raft 实现来构建一个复制的键/值服务，但不使用快照功能。在 B 部分中，您将使用来自 Lab 3D 的快照实现，这将使 Raft 能够丢弃旧的日志条目。请分别在各自的截止日期前提交每部分内容。

您应该查看 [扩展的 Raft 论文](http://nil.csail.mit.edu/6.5840/2024/papers/raft-extended.pdf), 特别是第 7 和第 8 节。对于更广泛的 视角，看看 Chubby，Paxos Made Live， Spanner、Zookeeper、Harp、Viewstamped Replication 和 [Bolosky 等人。](http://static.usenix.org/event/nsdi11/tech/full_papers/Bolosky.pdf)

我们为您提供 `src/kvraft` 中的骨架代码和测试。您需要修改 `kvraft/client.go` 、 `kvraft/server.go` ，可能还需要修改 `kvraft/common.go` 。

### 4A 无快照的key/value服务（[中等/困难](http://nil.csail.mit.edu/6.5840/2024/labs/guidance.html)）

#### 任务

每个key/value服务器（"kvserver"）都将有一个关联的 Raft peer。Clerks 向关联的 Raft 为Leader的 kvserver 发送 `Put()` 、 `Append()` 和 `Get()` RPC。kvserver 代码将 Put/Append/Get 操作提交给 Raft，以便 Raft 日志中保存一系列 Put/Append/Get 操作。所有 kvserver 按顺序从 Raft 日志中执行操作，将这些操作应用到它们的键/值数据库中；目的是让服务器维护键/值数据库的相同副本。

`Clerk` 有时不知道哪个 kvserver 是 Raft 领导者。如果 `Clerk` 向错误的 kvserver 发送 RPC，或者无法到达 kvserver， `Clerk` 应通过向不同的 kvserver 发送请求来重试。如果 key/value 服务将操作提交到其 Raft 日志（从而将操作应用于键/值状态机），领导者通过响应其 RPC 向 `Clerk` 报告结果。如果操作未能提交（例如，如果领导者被替换），服务器会报告错误， `Clerk` 会尝试使用不同的服务器重试。

您的 kvservers 不应直接通信；它们应仅通过 Raft 相互交互。

**Task**

- 你的第一个任务是实现一个在没有消息丢失且没有服务器故障时能正常工作的解决方案。
- 请随意将您的客户端代码从实验 2（ `kvsrv/client.go` ）复制到 `kvraft/client.go` 中。您需要添加逻辑来决定将每个 RPC 发送到哪个 kvserver。请注意， `Append()` 不再向 Clerk 返回值。
- 你还需要在 `server.go` 中实现 `Put()` 、 `Append()` 和 `Get()` 的 RPC 处理程序。这些处理程序应进入 `Op` 在 Raft 日志中使用 `Start()` ；你应该在 `server.go` 中填写 `Op` 结构体定义，以便描述 Put/Append/Get 操作。每个服务器应在 Raft 提交 `Op` 命令时执行它们，即当它们出现在 `applyCh` 上时。RPC 处理程序应在 Raft 提交其 `Op` 时注意到，然后回复 RPC。
- 您已完成此任务当你**可靠地**通过测试套件中的第一个测试："One client"。

**Hint**

调用 `Start()` 后，您的 kvservers 需要等待 Raft 完成一致性协议。已达成一致的命令会到达 `applyCh` 。您的代码需要持续读取 `applyCh` ，同时 `Put()` 、 `Append()` 和 `Get()` 处理程序使用 `Start()` 向 Raft 日志提交命令。注意 kvserver 与其 Raft 库之间的死锁。

如果 kvserver 不是多数派的一部分，它不应该完成一个 `Get()` RPC（这样它就不会提供过时的数据）。一个简单的解决方案是在 Raft 日志中输入每个 `Get()` （以及每个 `Put()` 和 `Append()` ）。你不必实现第 8 节中描述的只读操作优化。

您不应需要向 Raft `ApplyMsg` 或 Raft RPC（如 `AppendEntries` ）添加任何字段，但您被允许这样做。

最好从一开始就添加锁定，因为 需要避免死锁有时会影响整体代码设计。检查 你的代码是无竞争的使用 `go test -race` .



现在你应该修改你的解决方案，以在网络问题出现时继续运行 和服务器故障。 你将面临的一个问题是 `Clerk` 可能需要多次发送 RPC，直到找到一个 kvserver 肯定回复。如果领导者刚失败 将条目提交到 Raft 日志时， `Clerk` 可能不会 收到回复，因此可能 将请求重新发送给另一个领导者。 每次调用 `Clerk.Put()` 或 `Clerk.Append()` 应该 导致仅执行一次，因此您必须确保 重新发送不会导致服务器执行 请求两次。

**Task**

添加代码以处理故障，并应对重复的 `Clerk` 请求，包括 `Clerk` 在一个任期内向 kvserver 领导者发送请求、等待回复超时并在另一个任期内重新向新领导者发送请求的情况。请求应仅执行一次。这些说明包括关于[重复检测](http://nil.csail.mit.edu/6.5840/2024/notes/l-raft-QA.txt)的指导。您的代码应通过 `go test -run 4A` 测试。

**Hint**

- 您的解决方案需要处理这样一种情况：领导者调用了 Clerk 的 RPC 的 Start()，但在请求提交到日志之前失去了领导权。在这种情况下，您应安排 Clerk 将请求重新发送到其他服务器，直到找到新的领导者。一种实现方式是让服务器通过注意到 Raft 的任期已更改或 Start()返回的索引处出现了不同的请求，来检测自己是否失去了领导权。如果前领导者被自身分区隔离，它将无法知晓新领导者的存在；但同一分区内的任何客户端也无法与新领导者通信，因此在这种情况下，服务器和客户端无限期等待直到分区恢复也是可以接受的。

- 你可能需要修改你的 Clerk，使其记住上一次 RPC 中哪个服务器被确认为领导者，并首先将下一个 RPC 发送到该服务器。这样可以避免每次 RPC 都浪费时间寻找领导者，这可能有助于你足够快地通过一些测试。
- 您应该使用类似于实验 2 中的重复检测方案。它应快速释放服务器内存，例如通过让每个 RPC 暗示客户端已看到其前一个 RPC 的回复。可以假设客户端一次只会向 Clerk 发出一个调用。您可能会发现需要更改实验 2 中存储在重复检测表中的信息。

#### **分析**

- client如何与leader通信的

  只有leader才会处理client的请求，如果不是leader的服务器接收到了client的请求，将会把请求进行重定向到leader来处理。（在实现的时候，并没有采取这种策略，主要实现没有实现成功，就直接客户端暴力挨个请求了

- 需要额外处理leader reply超时

- 实现线性化语义

  线性化语义可以通过前面已经实现的raft底层保证，但是raft并不保证command的不重复，需要在应用层额外保证command（即client Get/Put/Append重复发送的RPC）的不重复性。引入`duplicate table`实现。

- 只读操作不记录日志（不要求实现）

![image-20231212150951790](Lab%204%20%E5%AE%B9%E9%94%99keyvalue%E6%9C%8D%E5%8A%A1.assets/image-20231212150951790.png)

1. 如果`Get`或`Put`RPC timeout了(也就是`Call() return false`)，client应该怎么处理？

   > - 如果服务端dead或者请求丢失，client re-send即可
   > - 如果服务器已经执行了请求，但是reply在网络中丢失了，re-send是有危险的

2. 不好区分上面的两种情况。上面两种情况对于client而言，看起来是一样的(no reply)。如果请求已经被执行过了，那么client仍然需要这个reply

   > 让`kvserver` 检测重复client的请求。client每次请求都会携带一个唯一ID，并在re-send一个rpc时，携带与上次RPC相同的id。`kvserver` 维护一个按id索引的"duplicate table"，在执行后为每个rpc创建一个条目，用以在"duplicate table" 中记录reply。如果第二个rpc以相同的id到达，则它是重复的，可以根据"duplicate table"来生成reply。

3. 新的领导者如何获得"duplicate table"？

   > 将 id 放入传递给 Raft 的记录操作中，所有副本都应在执行时更新其table，以便如果它们成为领导者后这些信息依然保留。

4. 如果服务器崩溃，它如何恢复"duplicate table"？

   > - 如果没有快照，日志重播将填充"duplicate table"
   > - 如果有快照，快照必须包含"duplicate table"的副本

5. 如果重复请求在原始请求执行之前到达怎么办？

   再次调用`Start()`，会导致它可能在log中两次。当`cmd`出现在`applyCh`上时，如果table里已经有了，就没有必要执行了。

6. 保持table较小的idea

   - 每个client一个条目，而不是每次 RPC 一个条目

   - 每个client只有一次 RPC 没有完成

     > 每个客户端一次只能有一个未完成的RPC，这意味着客户端在发送一个RPC请求之后，必须等待服务器对该请求的响应，才能发送下一个RPC请求。这样可以确保每个客户端一次只有一个RPC请求在处理中，简化了并发控制。

   - 每个client按顺序对 RPC 进行编号

   - 当服务器收到client 第10个RPC 时，它可以忽略client的较低条目，因为这意味着client永远不会重新发送旧的 RPC

7. 一些细节

   - 每个client都需要一个唯一的client id（64位随机数字即可）
   - client需要在每次rpc中发送client id和seq，如果需要re-send，client就携带相同的seq
   - kvserver中按client id索引的table仅包含 seq 和value（如果已执行）
   - RPC 处理程序首先检查table，只有 seq 大于 table 条目的seq才能执行`Start()`
   - 每个log条目必须包含client id和seq
   - 当operation出现在 `applyCh` 上时，更新client table条目中的 seq 和 value，唤醒正在等待的 RPC 处理程序（如果有）

8. `kvserver`可能会返回table中的旧值，但是返回的确实是当前值，没有问题。

   > C1 C2
   >
   > -- -- put(x,10) first send of get(x), reply(10) dropped put(x,20) re-sends get(x), server gets 10 from table, not 20
   >
   > get(x) and put(x,20) run concurrently, so could run before or after; so, returning the remembered value 10 is correct

#### 实现

##### client call

不断轮询server，直到请求到leader，为了区分重复rpc，需要携带客户端标识和rpc重复标识

```go
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
```

```go
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
```

##### server rpc

- Get
  1. 先调用`Start`在raft中达成共识
  2. kvserver应用层等待raft提交并应用command
  3. 应用状态机过后，才可以返回；否则超时失败返回

```go
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
	reply.Value=""
	reply.Err=ErrNoKey
	reply.Leader=false
}
```

- Put/Append
  1. 由于该类请求会改变状态机，所以不能重复执行，先按照tips里去重。如果已经请求过了，那么可以直接返回
  2. 后面的步骤和Get一样了

##### 额外的apply实现

kvserver位于raft的应用层，需要将raft提交的command应用到状态机里，也就是执行command。需要注意的是，**为了保证新leader也同时具备相同的log和状态机，所以不论是leader还是follower都需要从applyCh管道取出log并还原出原来的command，然后顺序command。**

等待应用完状态机过后，leader才reply客户端。

```go
func (kv *KVServer) apply() {
	for msg := range kv.applyCh { //遍历raft的应用管道，来一个取一个，没有会被阻塞
		kv.mu.Lock()

		op := msg.Command.(Op) //Command是interface{}类型，需要转换成Op类型，.()是空接口的类型断言
		// 命令不重复，或者命令Id大于之前的命令Id  -> 应用状态机，并记录table
		if PreSequenceId, ok := kv.duptable[op.ClientId]; !ok || PreSequenceId < op.SequenceId {
			kv.duptable[op.ClientId] = op.SequenceId
			switch op.Type {
			case PUT:
				kv.data[op.Key] = op.Value
			case APPEND:
				if _, ok := kv.data[op.Key]; ok {
					kv.data[op.Key] = kv.data[op.Key] + op.Value
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
		kv.mu.Unlock()
	}
}
```

#### 测试

![image-20250304160231472](Lab%204%20%E5%AE%B9%E9%94%99keyvalue%E6%9C%8D%E5%8A%A1.assets/image-20250304160231472.png)

### 4B 具有快照功能的key/value服务（困难）

#### 任务

当前情况下，您的key/value服务器没有调用您的 Raft 库的 `Snapshot()` 方法，因此重启的服务必须重新播放完整的持久化 Raft 日志以恢复其状态。现在您将修改 kvserver 以与 Raft 合作来节省日志空间，并减少重启时间，使用 Lab 3D 中的 Raft 的 `Snapshot()` 。

测试器将 `maxraftstate` 传递给您的 `StartKVServer()` 。 `maxraftstate` 表示您持久化 Raft 状态允许的最大大小（以字节为单位，包括日志，但不包括快照）。您应该比较 `maxraftstate` 与 `persister.RaftStateSize()` 。每当您的key/value服务器检测到 Raft 状态大小接近此阈值时，它应该通过调用 Raft 的 `Snapshot` 来保存快照。如果 `maxraftstate` 为-1，则无需快照。 `maxraftstate` 适用于 Raft 传递给 `persister.Save()` 的第一个参数的 GOB 编码字节。

**Task**

修改您的 kvserver，使其能够检测持久化的 Raft 状态增长过大，然后将快照交给 Raft。当 kvserver 服务器重启时，它应从 `persister` 读取快照并从快照中恢复其状态。

**Hint**

- 思考 kvserver 何时应该快照其状态以及快照中应包含什么。Raft 使用 `Save()` 在持久化对象中存储每个快照，以及相应的 Raft 状态。您可以使用 `ReadSnapshot()` 读取最新存储的快照。

- 您的 kvserver 必须能够检测到检查点日志中的重复操作，因此您用于检测它们的任何状态都必须包含在快照中。

- 将快照中存储的结构体的所有字段首字母大写。
- 您可能在您的 Raft 库中存在此实验暴露的 bug。如果您对 Raft 实现进行了修改，请确保它仍然可以通过实验 3 的所有测试。
- 一个合理的 Lab 4 测试时间应为 400 秒的真实时间和 700 秒的 CPU 时间。此外， `go test -run TestSnapshotSize` 应少于 20 秒的真实时间。

#### 分析

- 重新启动kvserver时，如果存在快照，则直接把快照里的数据放到状态机里
- kvserver需要检测`RaftStateSize`是否接近`maxraftstate`，如果大于就快照
- 应用层从`applyCh`管道中接收到的follower快照需要替换当前状态机，以进行快速恢复

#### 实现

**snapshot**

设阈值0.9，快照存Data和Duptable

```go
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
```

**修改apply**

```go
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
```

#### 结果

![image-20250304204237495](Lab%204%20%E5%AE%B9%E9%94%99keyvalue%E6%9C%8D%E5%8A%A1.assets/image-20250304204237495.png)

![image-20250304204820537](Lab%204%20%E5%AE%B9%E9%94%99keyvalue%E6%9C%8D%E5%8A%A1.assets/image-20250304204820537.png)

