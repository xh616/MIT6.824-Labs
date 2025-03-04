# Lab3 Raft

## 实验网站地址

https://pdos.csail.mit.edu/6.824/labs/lab-raft1.html

## 主要代码

`src/raft`下的文件

## 具体实验

### 3A ：领导者选举（中等）

实现 Raft leader 选举和心跳（没有日志条目的` AppendEntries RPC`）。第 3A 部分的目标是选举一个领导者，如果没有故障，则领导者保持领导者地位，如果旧领导者发生故障或传入/传出旧领导者的数据包丢失，则由新领导者接管。运行 `go test -run 3A `来测试你的 3A 代码。

![image-20241223160631249](Lab3%20Raft.assets/image-20241223160631249.png)

![image-20241223193255839](Lab3%20Raft.assets/image-20241223193255839.png)

![image-20241223193307919](Lab3%20Raft.assets/image-20241223193307919.png)

![image-20241223193323109](Lab3%20Raft.assets/image-20241223193323109.png)

![image-20241223193344987](Lab3%20Raft.assets/image-20241223193344987.png)

**Hint**

- 按照论文的图 2 进行操作。此时，您关心发送和接收 RequestVote RPC、与选举相关的服务器规则以及与领导者选举相关的 State
- 将图 2 的 leader election 状态添加到 raft.go 的 Raft 结构体中。您还需要定义一个结构来保存有关每个日志条目的信息。
- 填写` RequestVoteArgs `和` RequestVoteReply `结构。修改 `Make()` 以创建一个后台协程，当它有一段时间没有收到另一个 `peer` 的消息时，它会通过发送 `RequestVote RPC` 来定期启动 leader 选举。实现 `RequestVote() RPC `处理程序，以便服务器相互投票。
- 要实现心跳，请定义一个 `AppendEntries RPC` 结构体（尽管你可能还不需要所有参数），并让 leader 定期发送它们。编写 `AppendEntries RPC` 处理程序方法。
- 测试者要求你的 Raft 在旧 leader 失败后的 5 秒内选出一个新的 leader（如果大多数 Peer 节点仍然可以通信）
- 本文的第 5.2 节提到了 150 到 300 毫秒范围内的选举超时。只有当 leader 发送心跳的频率远高于每 150 毫秒一次（例如，每 10 毫秒发送一次）时，这样的范围才有意义。因为测试器限制了您每秒几十次的心跳，所以您必须使用大于论文的 150 到 300 毫秒的选举超时，但不能太大，因为那样您可能无法在 5 秒内选出领导者。
- 您可能会发现 Go 的 rand 很有用。
- 您需要编写代码来定期或在时间延迟后执行操作。最简单的方法是创建一个带有调用 time 的循环的 goroutine调用time.Sleep()；请参阅 `Make()`为此目的创建的 `ticker()`协程。不要使用 Go 的`time.Timer`或是`time.Ticker`，它们很难正确使用。
- 如果您的代码无法通过测试，请再次阅读本文的图 2；领导者选举的完整逻辑分布在图的多个部分
- 不要忘记实现` GetState()`
- 测试者调用 Raft 的 `rf.Kill()` 的实例。您可以使用` rf.killed()` 检查` Kill() `是否已被调用。你可能希望在所有循环中执行此操作，以避免死 Raft 实例打印令人困惑的消息。
- Go RPC 只发送名称以大写字母开头的结构体字段。子结构还必须具有大写的字段名称（例如，数组中日志记录的字段）。labgob 包会警告你这一点;不要忽略警告。
- 本实验中最具挑战性的部分可能是调试。花一些时间使您的实现易于调试。有关调试提示，请参阅指南页面
- cd src/raft                                                                                                      go test -run 3A

#### 分析

**一个任期内，节点只能将选票投给某一个节点**。因此当节点任期更新时要将 `votedfor` 置为 -1。

在领导选举的过程中，`AppendEntries RPC` 用来实现 Leader 的[心跳机制](https://zhida.zhihu.com/search?content_id=209019903&content_type=Article&match_order=1&q=心跳机制&zhida_source=entity)。

**如果来自其他节点的 RPC 请求中，或发给其他节点的 RPC 的回复中，任期高于自身任期，则更新自身任期，并转变为 Follower。**

1. **Follower**
   - 响应来自 Candidate 和 Leader 的 RPC 请求。
   - 如果在 election timeout 到期时，Follower 未收到来自当前 Leader 的 AppendEntries RPC，也没有收到来自 Candidate 的 RequestVote RPC，则转变为 Candidate。

2. **Candidate**
   - 转变 Candidate时，开始一轮选举：

   - - currentTerm++
     - 为自己投票（votedFor = me）
     - 重置 election timer
     - 向其他所有节点**并行**发送 RequestVote RPC

   - 如果收到了大多数节点的选票（voteCnt > n/2），当选 Leader。

   - 在选举过程中，如果收到了来自新 Leader 的 AppendEntries RPC，停止选举，转变为 Follower。

   - 如果 election timer 超时时，还未当选 Leader，则放弃此轮选举，开启新一轮选举。

3. **Leader**

   - 刚上任时，向所有节点发送一轮心跳信息
   - 此后，每隔一段固定时间，向所有节点发送一轮心跳信息，重置其他节点的 election timer，以维持自己 Leader 的身份。

![image-20250124160503880](Lab3%20Raft.assets/image-20250124160503880.png)

#### 实现

**定义**

```go
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	// 3A
	currentTerm int // 当前任期
	votedFor    int // 投票给谁，一个任期内，节点只能将选票投给某一个节点，所以当节点任期更新时将投票重置为-1
	state       int // 状态
	// 收到 RequestVote RPC 和 AppendEntries RPC都要重置选举超时计时器
	electionTimer   *time.Timer  // 选举超时计时器（每个都有）
	heartbeatTicker *time.Ticker // 心跳定时器（只有leader有）
}

// 状态
const (
	Follower = iota
	Candidate
	Leader
)

const noVote = -1

// 广播心跳时间(broadcastTime)<<选举超时时间
func HeartBeatTimeout() time.Duration {
	return 50 * time.Millisecond
}

// 随机超时选举时间生成（150ms-300ms）
func randomElectionTimeout() time.Duration {
	return time.Duration(150+rand.Int63()%150) * time.Millisecond
}
```

**RequestVote**

```go
// 获取投票请求
// 当 Candidate 任期小于当前节点任期时，返回 false（放弃 Candidate 身份并更新自己的任期）。
// 如果 votedFor 为 -1（即当前任期内此节点还未投票）或者 votedFor为 candidateId（即当前任期内此节点已经向此 Candidate 投过票），则同意投票；否则拒绝投票。
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// 如果Candidate任期小于我的任期，则不投票
	if args.Term < rf.currentTerm {
		// 告知Candidate我的任期
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// 如果Candidate任期大于我的任期，则更新我的任期并重置我的投票，并转为Follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = noVote // 无论何时，任期更新则重置投票
		rf.state = Follower
	}

	// 没投票或者已经投票给这个Candidate
	if (rf.votedFor == noVote || rf.votedFor == args.CandidateId) {
		rf.votedFor = args.CandidateId
		rf.electionTimer.Reset(randomElectionTimeout()) // 重置选举超时器
		log.Printf("In term %d, machine %d vote for machine %d", rf.currentTerm, rf.me, args.CandidateId)
		reply.VoteGranted = true
		reply.Term = rf.currentTerm // 告知Candidate我的任期
		return
	}

	// 失败处理
	reply.VoteGranted = false
	reply.Term = rf.currentTerm // 告知Candidate我的任期
}
```

**AppendEntries**

```go
// 当 Leader 任期小于当前节点任期时，返回 false，否则返回 true。
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// 如果leader任期小于我的任期，leader任期落后，会变为follower，则不接受该leader
	if rf.currentTerm > args.Term {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}
    
    //正常收到心跳就变为Follower并刷新选举超时计时器
	rf.state = Follower
	rf.electionTimer.Reset(randomElectionTimeout())
    
	// 如果leader任期大于我的任期，则更新我的任期并重置我的投票
	if rf.currentTerm < args.Term {
		rf.currentTerm = args.Term
		rf.votedFor = noVote
	}
	// leader任期等于我的任期时，接受该leader心跳
	reply.Term = rf.currentTerm
	reply.Success = true
}
```

**选举**

```go
// 监控 electon timer 的 go routine 来开始选举
func (rf *Raft) ticker() {
	for !rf.killed() {

		// Your code here (3A)
		// Check if a leader election should be started.

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		// 50ms-300ms
		// ms := 50 + (rand.Int63() % 300)
		// time.Sleep(time.Duration(ms) * time.Millisecond)

		//监听选举超时定时器
		<-rf.electionTimer.C
		rf.mu.Lock()
		if rf.state == Leader {
			rf.mu.Unlock()
			break
		}
		rf.state = Candidate
		rf.mu.Unlock()
		// 开始选举
		go rf.startElection()
		// 为什么将选举过程也作为一个 go routine，而不是阻塞地调用函数？
		// 1、避免阻塞主循环：如果直接调用 rf.startElection()，选举过程会阻塞当前协程，导致无法处理其他事件（如心跳、日志复制等）。
		// 2、支持超时重试：如果选举超时（electionTimer 再次触发），而当前选举仍未完成，Raft 需要能够启动新一轮选举。如果选举过程是同步的，就无法实现这一点
	}
}
```

```go
// 开始选举
/*
使用 n-1 个协程向其他节点并行地发送 RequestVote 请求。协程获得 response 后，向 voteCh 发送结果，startElection 协程进行结果统计。
统计过程中，若发现失去了 Candidate 身份，则停止统计。若获得票数过半，则成功当选 Leader，启动 heartbeat 协程。
若所有成员已投票，且未当选 Leader，则退出统计。
要注意的是，for 循环会一直试图从 voteCh 中读取，因此需要确保 voteCh 不会被阻塞。
*/
func (rf *Raft) startElection() {
	rf.mu.Lock()
	rf.votedFor = rf.me
	rf.currentTerm++
	rf.electionTimer.Reset(randomElectionTimeout())

	// 发送投票请求
	args := &RequestVoteArgs{Term: rf.currentTerm, CandidateId: rf.me}
	rf.mu.Unlock()
	// 定义一个channel，用于接收投票结果
	voteCh := make(chan bool, len(rf.peers)-1)
	// 跟踪所有发送协程，确保完成后关闭voteCh，使统计循环正常退出
	var wg sync.WaitGroup
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		wg.Add(1)
		// 并发发送请求，并处理投票完的结果
		go func(i int) {
			defer wg.Done()
			reply := &RequestVoteReply{}
			// 投票RPC未得到响应
			if ok := rf.sendRequestVote(i, args, reply); !ok {
				voteCh <- false
				return
			}
			// 在发完RPC请求后再上锁
			rf.mu.Lock()
			// 对方任期大于自己任期，更新自己任期，重置投票，转换为Follower
			if reply.Term > rf.currentTerm {
				rf.state = Follower
				rf.currentTerm = reply.Term
				rf.votedFor = -1
				rf.mu.Unlock()
				return
			}
			rf.mu.Unlock()
			// 收集一个投票
			voteCh <- reply.VoteGranted
		}(i)
	}

	// 单独协程等待所有RPC完成并关闭通道
	go func() {
		wg.Wait()
		close(voteCh)
	}()

	// 统计投票结果
	voteGrantedCnt := 1
	for voteGranted := range voteCh {
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		// 因为我可能在此过程中因任期小已经变为Follower了
		if state != Candidate {
			return
		}
		if voteGranted {
			voteGrantedCnt++
		}
		if voteGrantedCnt > len(rf.peers)/2 {
			rf.mu.Lock()
			// 如果我还是Candidate
			if rf.state == Candidate {
				rf.state = Leader
				// 启动心跳
				go rf.heartbeat()
			}
			rf.mu.Unlock()
			return
		}
	}
}
```

**心跳**

```go
// 心跳
func (rf *Raft) heartbeat() {
	//成为leader后立马发一次心跳
	rf.broadcastHeartbeat()

	//设置定时器，周期性发送心跳
	rf.heartbeatTimer = time.NewTicker(HeartBeatTimeout())
	defer rf.heartbeatTimer.Stop()
	for range rf.heartbeatTimer.C {
		rf.broadcastHeartbeat()
	}
}

// 广播心跳具体逻辑
func (rf *Raft) broadcastHeartbeat() {
	// 如果不是Leader或者已经被杀死，则退出
	if _, isLeader := rf.GetState(); !isLeader || rf.killed() {
		return
	}
	// 并行发送心跳
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		rf.mu.Lock() //锁任期
		args := &AppendEntriesArgs{Term: rf.currentTerm, LeaderId: rf.me}
		rf.mu.Unlock()
		reply := &AppendEntriesReply{}
		go func(server int) {
			if ok := rf.SendAppendEntries(server, args, reply); !ok {
				return
			}
			rf.mu.Lock()
			// 若对方任期大于自己任期，更新自己任期，重置投票，转换为Follower
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.votedFor = novote
				rf.state = Follower
				rf.mu.Unlock()
				return
			}
			rf.mu.Unlock()
		}(i)
	}
}
```

#### **测试**

![image-20250224204347984](Lab3%20Raft.assets/image-20250224204347984.png)

```shell
#!/bin/bash

set -euo pipefail

readonly TOTAL_RUNS=50
readonly TEST_CASES=(
    "TestInitialElection3A"
    "TestReElection3A" 
    "TestManyElections3A"
)
readonly PACKAGE_PATH="6.5840/raft"  # 添加包路径

function run_test() {
    local test_case=$1
    local run_num=$2
    
    echo -n "第${run_num}次运行 ${test_case}..."
    
    if ! output=$(go test -timeout 30s -run "^${test_case}$" ${PACKAGE_PATH} 2>&1); then
        echo -e "\033[31m失败\033[0m"
        echo "=== 错误详情 ==="
        echo "$output"
        exit 1
    fi
    
    echo -e "\033[32m通过\033[0m"
}

function main() {
    echo "▶ 开始执行Raft选举测试套件（${TOTAL_RUNS}次循环）"
    
    for ((i=1; i<=TOTAL_RUNS; i++)); do
        echo ""
        echo "===== 第 ${i} 轮测试 ====="
        
        for test_case in "${TEST_CASES[@]}"; do
            run_test "$test_case" "$i"
        done
    done
    
    echo ""
    echo -e "\033[42;37m 所有测试通过！ \033[0m"
}

main
```

![image-20250125151918490](Lab3%20Raft.assets/image-20250125151918490.png)

### 3B：日志复制（困难）

实现leader和follower代码以追加新的日志条目，以便 `go test -run 3B `测试通过。

**Hint**

- 你的首要目标应该是通过 `TestBasicAgree3B()` 。首先实现 `Start()` ，然后编写代码通过 `AppendEntries` RPC 发送和接收新的日志条目，遵循图 2。在每个`peer`上通过 `applyCh` 发送每个新提交的条目。
- 您需要实现选举限制（论文中的第 5.4.1 节）。
- 您的代码可能包含循环，这些循环会反复检查某些事件。 不要让这些循环持续执行而不暂停，因为 会减慢你的实现速度，以至于无法通过测试。 使用 Go 的 [条件变量](https://golang.org/pkg/sync/#Cond)（sync.Cond）, 或插入一个 `time.Sleep(10 * time.Millisecond)` 在每次循环迭代中。
- 如果你测试失败，看看 `test_test.go` 和 `config.go` 用于理解正在测试的内容。 `config.go` 还展示了测试人员如何使用 Raft API。

#### 分析

##### 总览

```go
logs           []LogEntry // 日志
// 所有机器
commitIndex int        //已知已提交的日志条目的最高索引值（初始值为 0，单调递增）
lastApplied    int        //已应用到状态机的日志条目的最高索引值（初始值为 0，单调递增）
// leader 
nextIndex[] int //针对每个服务器，记录要发送给该服务器的下一个日志条目的索引值（初始化为leader最后一个日志条目的索引值 +1）
matchIndex[] int //针对每个服务器，记录已知已复制到该服务器的日志条目的最高索引值（初始值为 0，单调递增）

// LogEntry
type LogEntry struct {
	Term int         // 从 leader 接受到日志的任期(索引初始化为 1)
	Cmd  interface{} // 待施加至状态机的命令
}
```

当节点发现 commitIndex > lastApplied 时，代表着 commitIndex 和 lastApplied 间的 entries 处于已提交，未应用的状态。因此应将其间的 entries **按序**应用至状态机。

对于 Follower，commitIndex 通过 Leader AppendEntries RPC 的参数 leaderCommit 更新。对于 Leader，commitIndex 通过其维护的 matchIndex 数组更新。

- `nextIndex[]` 由 Leader 维护，`nextIndex[i]` 代表需要同步给 `peer[i]` 的下一个 entry 的 index。在 Leader 当选后，重新初始化为 Leader 的 last log index + 1。
- `matchIndex[]` 由 Leader 维护，`matchIndex[i]` 代表 Leader 已知的已在 `peer[i]` 上成功复制的最高 entry index。在 Leader 当选后，重新初始化为 0。

不能简单地认为 matchIndex = nextIndex - 1，当有节点宕机时，nextIndex 会大于 matchIndex

nextIndex 是对追加位置的一种猜测，是乐观的估计。因此，当 Leader 上任时，会将 nextIndex 全部初始化为 last log index + 1，即乐观地估计所有 Follower 的 log 已经与自身相同。

matchIndex 则是对同步情况的保守确认，为了保证安全性。matchIndex 及此前的 entry 一定都成功地同步。matchIndex 的作用是帮助 Leader 更新自身的 commitIndex。当 Leader 发现一个 N 值，N 大于过半数的 matchIndex，则可将其 commitIndex 更新为 N（需要注意任期号的问题）。matchIndex 在 Leader 上任时被初始化为 0。

nextIndex 是最乐观的估计，被初始化为最大可能值；matchIndex 是最悲观的估计，被初始化为最小可能值。在一次次心跳中，nextIndex 不断减小，matchIndex 不断增大，直至 matchIndex = nextIndex - 1，则代表该 Follower 已经与 Leader 成功同步。

##### AppendEntries RPC

Args

```go
PrevLogIndex int        // 前一条日志的索引
PrevLogTerm  int        // 前一条日志的任期
Entries      []LogEntry // 待复制的日志（若为空则是一次心跳）
LeaderCommit int        // leader的commitIndex，帮助 Follower 更新自身的 commitIndex
```

Follower的实现： 

1. 如果 leader 的任期小于自己的任期返回 false。(5.1) 
2. 若 Follower 在 prevLogIndex 位置的 entry 的 term 与 prevLogTerm 不同（或者 prevLogIndex 的位置没有 entry），返回 false。(5.3) 
3. 如果存在一条日志索引和 prevLogIndex 相等， 但是任期和 prevLogItem 不相同的日志， 需要删除这条日志及所有后继日志。（5.3） 需要特别注意的是，**假如没有冲突，不能删除任何 entry**。因为存在 Follower 的 log 更 up-to-date 的可能
4. 如果 leader 复制的日志本地没有，则直接追加存储。 
5. 如果 leaderCommit>commitIndex， 设置本地 commitIndex 为 leaderCommit 和最新日志索引中 较小的一个。

##### RequestVote RPC

Args

```go
LastLogIndex int // Candidate 最后一条日志的索引，是投票的额外判据
LastLogTerm  int // Candidate 最后一条日志的任期
```

接受者的实现： 

1. 如果 leader 的任期小于自己的任期返回 false。(5.1) 

2. <u>RPC中包含了candidate的日志信息，如果投票者自己的日志比candidate的还**新**，它会拒绝掉该投票请求</u>。即只有 Candidate 的 log 至少与自己的 log 一样新（**up-to-date**）时，才同意投票。(5.2 和 5.4)

**如果两份日志最后条目的任期号不同，那么任期号大的日志更“新”**

**如果两份日志最后条目的任期号相同，那么日志较长的那个更“新”。**

这里投票的额外限制是为了保证已经被 commit 的 entry 一定不会被覆盖。仅有当 Candidate 的 log 包含所有已提交的 entry，才有可能当选为 Leader。

##### 规则

**All Severs**

如果 commitIndex > lastApplied，lastApplied++，将 log[lastApplied] 应用到状态机。即前文提到的 entry 从已提交状态到已应用状态的过程。

**Leaders**

- 如果收到了来自 client 的 command，将 command 以 entry 的形式添加到日志。在 lab2B 中，client 通过 Start() 函数传入 command。

- 如果 LastLogIndex >= nextIndex[i]，向 peer[i] 发送 AppendEntries RPC，RPC 中包含从 nextIndex[i] 开始的日志。

- - 如果返回值为 true，更新 nextIndex[i] 和 matchIndex[i]。
  - 如果因为 entry 冲突，RPC 返回值为 false，则将 nextIndex[i] 减1并重试。这里的重试不一定代表需要立即重试，实际上可以仅将 nextIndex[i] 减1，下次心跳时则是以新值重试。

- 如果存在 index 值 N 满足：

- - N > commitIndex
  - 过半数 matchIndex[i] >= N
  - log[N].term == currentTerm

​	则令 commitIndex = N。

这里则是 Leader 更新 commitIndex 的方式。前两个要求都比较好理解，第三个要求是 Raft 的一个特性，即 Leader 仅会直接提交其任期内的 entry。存在这样一种情况，Leader 上任时，其最新的一些条目可能被认为处于未被提交的状态（但这些条目实际已经成功同步到了大部分节点上）。Leader 在上任时并不会检查这些 entry 是不是实际上已经可以被提交，而是通过提交此后的 entry 来间接地提交这些 entry。这种做法能够 work 的基础是 Log Matching Property：如果两个日志包含具有相同 index 和 term 的条目，则日志在给定索引之前的所有条目中都是相同的。

这样简化了 Leader 当选的初始化工作，也成功避免了简单地通过 counting replicas 提交时，可能出现的已提交 entry 被覆盖的问题。

#### 实现

太多的线程和 RPC 让系统的复杂性骤升，未持有锁的时刻什么都有可能发生。另外还有一个令人纠结的地方，就是各种时机。例如，接收到了 client 的一个请求，什么时候将这条 entry 同步给 Follower？什么时候将已提交的 entry 应用至状态机？更新某一变量时，是起一线程轮询监听，还是用 channel 或者 sync.Cond 唤醒，还是采取 lazy 策略，我这里使用的是起一轮后台 go routines实时监听，并用sync.Cond 唤醒

对于所有的初始节点（Follower 节点），包含如下后台 go routines：

- `ticker`：监听选举超时定时器。超时事件发生时，Follower 转变为 Candidate，发起一轮选举。
- `heartbeatEvent`：监听 heartbeatTimer 的超时事件，仅在节点为 Leader 时工作。heartbeatTimer 超时后，Leader 立即广播一次心跳命令，日志复制也在心跳RPC中实现。
- `AddLogEvent`：启动 AddLogEvent goroutine 来监听 addLog 事件，仅在节点为 Leader 时工作，并发送心跳完成新增日志复制请求。
- `applierEvent`：启动状态机应用日志的 goroutine，当节点认为需要一次 apply 时，向 applierCh 发送一次信号，applierEvent接收信号后会将当前 lastApplied 和 commitIndex 间的所有 entry 提交。

##### RequestVote

主要修改点在于成功投票的判断还需要加一个判断：Candidate的日志至少和我的一样新

```go
// example RequestVote RPC handler.
// 获取投票请求
// 当 Candidate 任期小于当前节点任期时，返回 false（放弃 Candidate 身份并更新自己的任期）。
// 如果 votedFor 为 -1（即当前任期内此节点还未投票）或者 votedFor为 candidateId（即当前任期内此节点已经向此 Candidate 投过票），则同意投票；否则拒绝投票。
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dVote, "after called RequestVote, S%d status{votedFor:%d,role:%s,currentTerm:%d}",
			rf.me, rf.votedFor, StateToString(rf.state), rf.currentTerm)
	}()
	Debug(dVote, "before called RequestVote, S%d status{votedFor:%d,role:%s,currentTerm:%d}",
		rf.me, rf.votedFor, StateToString(rf.state), rf.currentTerm)

	// 如果Candidate任期小于我的任期，则不投票
	if args.Term < rf.currentTerm {
		// 告知Candidate我的任期
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// 如果Candidate任期大于我的任期，则更新我的任期并重置我的投票，并转为Follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = noVote // 无论何时，任期更新则重置投票
		rf.state = Follower
	}

	// 没投票或者已经投票给这个Candidate，并且Candidate的日志至少和我的一样新，则投票
	if (rf.votedFor == noVote || rf.votedFor == args.CandidateId) &&
		rf.isLogUpToDate(args.LastLogTerm, args.LastLogIndex) {
		rf.votedFor = args.CandidateId
		rf.electionTimer.Reset(randomElectionTimeout()) // 重置选举超时器
		Debug(dVote, "S%d vote to S%d", rf.me, args.CandidateId)
		reply.VoteGranted = true
		reply.Term = rf.currentTerm // 告知Candidate我的任期
		return
	}

	// 失败处理
	reply.VoteGranted = false
	reply.Term = rf.currentTerm // 告知Candidate我的任期
}
```

##### AppendEntries

新增逻辑：如果我宕机，落后了leader很多日志，拒绝leader的日志复制，或者该条目的任期在 prevLogIndex 上不能和 prevLogTerm 匹配上，则返回false

日志复制：遍历日志块，如果有日志重叠，进而判断是否发生冲突，如果有冲突则删除冲突的日志和之后的日志，并追加新的日志；无冲突则不需要做操作。如果没有日志重叠，则直接追加日志

更新commitIndex，如果Leader的LeaderCommit大于了我的commitIndex，我对自己的commitIndex更新后通知应用日志协程进行日志应用

```go
// 当 Leader 任期小于当前节点任期时，返回 false，否则返回 true。
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dLog, "after AppendEntries S%d status{currentTerm:%d,role:%s,log:%v,lastApplied:%d,commitIndex:%d,leaderCommit:%d}",
			rf.me, rf.currentTerm, StateToString(rf.state), rf.logs, rf.lastApplied, rf.commitIndex, args.LeaderCommit)
	}()
	Debug(dLog, "before AppendEntries S%d status{currentTerm:%d,role:%s,log:%v,lastApplied:%d,commitIndex:%d,leaderCommit:%d}",
		rf.me, rf.currentTerm, StateToString(rf.state), rf.logs, rf.lastApplied, rf.commitIndex, args.LeaderCommit)

	// 如果leader任期小于我的任期，leader任期落后，会变为follower，则不接受该leader
	if rf.currentTerm > args.Term {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	//正常收到心跳就变为Follower并刷新选举超时计时器
	rf.state = Follower
	rf.electionTimer.Reset(randomElectionTimeout())

	// 如果leader任期大于我的任期，则更新我的任期并重置我的投票
	if rf.currentTerm < args.Term {
		rf.currentTerm = args.Term
		rf.votedFor = noVote
	}

	// 如果我宕机，落后了leader很多日志，拒绝leader的日志复制
	// 或者该条目的任期在 prevLogIndex 上不能和 prevLogTerm 匹配上，则返回false
	if len(rf.logs) <= args.PrevLogIndex || rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// 将leader的日志复制到自己的日志中
	for i, entry := range args.Entries {
		index := args.PrevLogIndex + i + 1
		if index < len(rf.logs) { //有日志重叠
			if rf.logs[index].Term != entry.Term { // 看是否发生冲突
				// rf.logs[:index]代表从 rf.logs 切片中获取从开始到 index（不包括 index）的部分
				rf.logs = rf.logs[:index]        // 有冲突，删除冲突的日志和之后的日志
				rf.logs = append(rf.logs, entry) // 追加新的日志
			}
			// 如果没有冲突，就不需要做任何操作
		} else if index == len(rf.logs) { //没有重叠，且刚好在下一个位置
			rf.logs = append(rf.logs, entry)
		}
	}

	// 更新commitIndex，领导者提交日志时，需要等到领导者下一次广播才能让跟随者也跟着提交。
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.lastLogIndex())
		rf.cond.Signal() //通知applier协程应用日志
	}

	reply.Term = rf.currentTerm
	reply.Success = true
}
```

##### ticker

```go
// 监控 electon timer 的 go routine 来开始选举
func (rf *Raft) ticker() {
	for !rf.killed() {
		// Your code here (3A)
		// Check if a leader election should be started.

		//监听选举超时定时器
		<-rf.electionTimer.C
		rf.mu.Lock()
		rf.state = Candidate
		// 开始选举
		rf.startElection()
		rf.mu.Unlock()
	}
}
```

```go
// 开始选举
/*
使用 n-1 个协程向其他节点并行地发送 RequestVote 请求。
统计过程中，若发现失去了 Candidate 身份，则停止统计。
若获得票数过半，则成功当选 Leader，广播心跳并进行日志复制。
*/
func (rf *Raft) startElection() { //选举已在锁中，无需继续内部加锁
	rf.votedFor = rf.me
	rf.currentTerm++
	rf.electionTimer.Reset(randomElectionTimeout())
	Debug(dTimer, "S%d start election", rf.me)
	voteGrantedCnt := 1 //先给自己投一票

	// 发送投票请求
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		args := &RequestVoteArgs{
			Term:         rf.currentTerm,
			CandidateId:  rf.me,
			LastLogIndex: rf.lastLogIndex(),
			LastLogTerm:  rf.lastLogTerm(),
		}
		Debug(dVote, "sendRequestVote S%d -> S%d, args{%+v}", rf.me, i, args)
		// 并发发送请求，并处理投票完的结果
		go func(i int) {
			reply := &RequestVoteReply{}
			// 投票RPC未得到响应
			if ok := rf.sendRequestVote(i, args, reply); !ok {
				return
			}
			// 在发完RPC请求后再上锁
			rf.mu.Lock()
			defer rf.mu.Unlock()
			defer func() {
				Debug(dVote, "sendRequestVote S%d's reply, grant %v at term: %d", i, reply.VoteGranted, reply.Term)
			}()
			if rf.currentTerm != args.Term || rf.state != Candidate { // 已经到下一个任期了或者不是Candidate了，提前结束
				return
			}

			// 对方任期大于自己任期，更新自己任期，重置投票，转换为Follower
			if reply.Term > rf.currentTerm {
				rf.state = Follower
				rf.currentTerm = reply.Term
				rf.votedFor = noVote
			} else if reply.Term == rf.currentTerm && rf.state == Candidate {
				// 如果对方任期等于自己任期，并且自己是Candidate，则收集投票
				if reply.VoteGranted {
					voteGrantedCnt++
				}
				if voteGrantedCnt > len(rf.peers)/2 {
					rf.state = Leader
					// Leader上的易失性状态，选举后可能换leader需要重新初始化
					rf.initializeLeaderEasilyLostState()
					// 成为leader后立马广播心跳，防止选举超时
					rf.broadcastHeartbeat()
				}

			}
		}(i)
	}
}
```

nextIndex和matchIndex选举后可能换leader，需要重新初始化，第一次当选leader时也需进行初始化

```go
// 选举后，领导人的易失状态需要重新初始化
func (rf *Raft) initializeLeaderEasilyLostState() {
	defer func() {
		Debug(dLeader, "S%d become new leader, nextIndex:%v,matchIndex:%v", rf.me, rf.nextIndex, rf.matchIndex)
	}()

	for i := 0; i < len(rf.peers); i++ {
		rf.nextIndex[i] = len(rf.logs) //初始值为领导人最后的日志条目的索引+1
		rf.matchIndex[i] = 0           //初始值为0
	}

	// 领导人的nextIndex和matchIndex是确定的
	rf.matchIndex[rf.me] = rf.lastLogIndex()
	rf.nextIndex[rf.me] = rf.lastLogIndex() + 1
}
```

##### heartbeatEvent

```go
// 后台心跳协程，监听定时器，如果是leader就广播心跳
func (rf *Raft) heartbeatEvent() {
	for !rf.killed() {
		<-rf.heartbeatTicker.C
		rf.mu.Lock()
		if rf.state == Leader {
			// 广播心跳
			rf.broadcastHeartbeat()
			rf.electionTimer.Reset(randomElectionTimeout()) // leader广播完毕时，也应该把自己的选举超时器刷新一下
		}
		rf.mu.Unlock()
	}
}
```

```go
// 广播心跳及日志复制具体逻辑
func (rf *Raft) broadcastHeartbeat() {
	Debug(dTimer, "S%d start broadcast", rf.me)

	// 并行发送心跳，并更新日志
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		args := &AppendEntriesArgs{
			Term:         rf.currentTerm,
			LeaderId:     rf.me,
			PrevLogIndex: rf.nextIndex[i] - 1,
			PrevLogTerm:  rf.logs[rf.nextIndex[i]-1].Term,
			Entries:      make([]LogEntry, 0),
			LeaderCommit: rf.commitIndex,
		}
		args.Entries = append(args.Entries, rf.logs[rf.nextIndex[i]:]...)

		Debug(dLog, `sendAppendEntries S%d -> S%d, args{PrevLogIndex:%d,PrevLogTerm:%d,LeaderCommit:%d,entries_len:%d}`,
			rf.me, i, args.PrevLogIndex, args.PrevLogTerm, args.LeaderCommit, len(args.Entries))

		go func(server int) {
			reply := &AppendEntriesReply{}
			if ok := rf.SendAppendEntries(server, args, reply); !ok {
				return
			}
			rf.mu.Lock()
			defer rf.mu.Unlock()
			defer func() {
				Debug(dLog, `sendAppendEntries S%d's reply, nextIndex:%d matchIndex:%d`, i, rf.nextIndex[i], rf.matchIndex[i])
			}()

			if rf.currentTerm != args.Term || rf.state != Leader { //已经到下一个任期或不是leader，没有必要在进行广播
				return
			}
			// 若对方任期大于自己任期，更新自己任期，转换为Follower
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.state = Follower
				return
			}
			// 心跳成功或日志复制成功
			if reply.Success {
				rf.matchIndex[server] = args.PrevLogIndex + len(args.Entries)
				rf.nextIndex[server] = rf.matchIndex[server] + 1

				//超过半数节点追加成功，也就是已提交，并且还是leader，那么就可以应用当前任期里的日志到状态机里。
				rf.checkAndCommitLogs()
			} else { //失败，减小nextIndex重试
				rf.nextIndex[server]--
				if rf.nextIndex[server] < 1 {
					rf.nextIndex[server] = 1
				}
			}
		}(i)
	}
}
```

判断是否能提交日志，这块的逻辑：遍历对等点，找到相同的最大的那个N，即找最大的可以提交的日志索引，<u>这里需要注意判断日志任期是否相同</u>，当N大于commitIndex时可以应用日志

```go
// 判断是否能提交日志，并判断是否能进行应用日志到状态机
func (rf *Raft) checkAndCommitLogs() {
	N := rf.commitIndex
	//遍历对等点，找到相同的最大的那个N，即找最大的可以提交的日志索引
	for _N := rf.commitIndex + 1; _N < len(rf.logs); _N++ {
		succeedNum := 0
		for p := 0; p < len(rf.peers); p++ {
			if _N <= rf.matchIndex[p] && rf.logs[_N].Term == rf.currentTerm {
				succeedNum++
			}
		}
		if succeedNum > len(rf.peers)/2 {
			N = _N
		}
	}

	if N > rf.commitIndex { //Leader可以应用日志了
		Debug(dLog, `S%d commit to index: %d`, rf.me, N)
		rf.commitIndex = N
		rf.cond.Signal() //通知applierEvent应用日志
	}
}
```

##### AddLogEvent

```go
// 新增日志goroutine 来监听 addLog 事件，发送日志复制请求
func (rf *Raft) AddLogEvent() {
	for !rf.killed() {
		<-rf.addLogSignal
		rf.mu.Lock()
		if rf.state == Leader {
			// 发心跳进行日志复制
			rf.broadcastHeartbeat()
			// 刷新选举超时计时器
			rf.electionTimer.Reset(randomElectionTimeout())
			// 新增log引起的快速log复制算一次心跳，减少不必要的心跳发送
			rf.heartbeatTicker.Reset(HeartBeatTimeout())
		}
		rf.mu.Unlock()
	}
}
```

```go
使用Raft的服务（例如，键值服务器）希望开始对下一个要追加到Raft日志的命令进行协议。如果该服务器不是领导者，则返回false。否则，启动协议并立即返回。
不能保证该命令会被提交到Raft日志，因为领导者可能会失败或失去选举。即使Raft实例已经被终止，该函数也应该平稳地返回。
第一个返回值是该命令如果最终提交，会出现在Raft日志中的索引。第二个返回值是当前的任期。第三个返回值是如果服务器认为它是领导者，则返回true。
*/
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := false

	// Your code here (3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != Leader {
		return -1, -1, false
	}

	// Leader追加日志
	rf.logs = append(rf.logs, LogEntry{
		Cmd:  command,
		Term: rf.currentTerm,
	})

	rf.matchIndex[rf.me] = rf.lastLogIndex()
	rf.nextIndex[rf.me] = len(rf.logs)

	index = rf.lastLogIndex()
	term = rf.currentTerm
	isLeader = true

	//唤醒追加日志协程
	go func() {
		rf.addLogSignal <- struct{}{}
	}()

	Debug(dClient, "S%d Start cmd:%v,index:%d", rf.me, command, index)
	return index, term, isLeader
}
```

##### applierEvent

```go
// 应用日志到状态机
func (rf *Raft) applierEvent() {
	for !rf.killed() {
		rf.mu.Lock()
		for rf.commitIndex <= rf.lastApplied { //用for防止虚假唤醒
			rf.cond.Wait() //等待commitIndex变化唤醒该协程
		}
		msgs := make([]ApplyMsg, 0, rf.commitIndex-rf.lastApplied)
		for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
			msgs = append(msgs, ApplyMsg{
				CommandValid: true,
				Command:      rf.logs[i].Cmd,
				CommandIndex: i,
			})
		}
		rf.mu.Unlock()
		Debug(dLog2, "S%d need apply msg{%+v}", rf.me, msgs)
		for _, msg := range msgs {
			rf.applyMsg <- msg
			rf.mu.Lock()
			rf.lastApplied = max(msg.CommandIndex, rf.lastApplied)
			Debug(dLog2, "S%d lastApplied:%d", rf.me, rf.lastApplied)
			rf.mu.Unlock()
		}
	}
}
```

##### 测试

**Debug**

https://blog.josejg.com/debugging-pretty/ Debug技巧参考

```go
type logTopic string

const (
	dClient  logTopic = "CLNT"
	dCommit  logTopic = "CMIT"
	dDrop    logTopic = "DROP"
	dError   logTopic = "ERRO"
	dInfo    logTopic = "INFO"
	dLeader  logTopic = "LEAD"
	dLog     logTopic = "LOG1"
	dLog2    logTopic = "LOG2"
	dPersist logTopic = "PERS"
	dSnap    logTopic = "SNAP"
	dTerm    logTopic = "TERM"
	dTest    logTopic = "TEST"
	dTimer   logTopic = "TIMR"
	dTrace   logTopic = "TRCE"
	dVote    logTopic = "VOTE"
	dWarn    logTopic = "WARN"
)

var debugStart time.Time
var debugVerbosity int

func init() {
	debugVerbosity = getVerbosity()
	debugStart = time.Now()

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
}

func Debug(topic logTopic, format string, a ...interface{}) {
	if debugVerbosity == 1 || debugVerbosity == -1 {
		time := time.Since(debugStart).Microseconds()
		time /= 100
		prefix := fmt.Sprintf("%06d %v ", time, string(topic))
		format = prefix + format
		log.Printf(format, a...)
	}
}

// Retrieve the verbosity level from an environment variable
func getVerbosity() int {
	v := os.Getenv("VERBOSE")
	level := 0
	if v != "" {
		var err error
		level, err = strconv.Atoi(v)
		if err != nil {
			log.Fatalf("Invalid verbosity %v", v)
		}
	}
	return level
}
```

**测试脚本**

```shell
#!/usr/bin/env bash

# 提示信息：介绍脚本的用途和如何使用
echo "欢迎使用raft_test.sh脚本！"
echo "该脚本用于测试不同的测试项。"

# 检查参数个数，确保传入了足够的参数
if [ $# -ne 3 ]; then
    echo "错误: 参数数量不正确！"
    echo "请使用: raft_test.sh <测试项> <测试次数> <VERBOSE>"
    echo "  VERBOSE=1    输出详细日志"
    echo "  VERBOSE=0    不输出日志"
    exit 1
fi

# 获取 VERBOSE 参数
verbose=$3

# 检查 VERBOSE 参数是否合法（确保是 0 或 1）
if [ "$verbose" != "0" ] && [ "$verbose" != "1" ]; then
    echo "错误: VERBOSE 参数无效！请设置为 1（输出日志）或 0（不输出日志）"
    echo "请使用: raft_test.sh <测试项> <测试次数> <VERBOSE>"
    exit 1
fi

cnt=$2

# 检查输入的测试次数是否是有效的数字
if ! echo "$cnt" | grep -q '^[0-9]\+$'; then
    echo "错误: '$cnt' 不是有效的数字！"
    echo "请使用: raft_test.sh <测试项> <测试次数> <VERBOSE>"
    exit 1
fi

# 提示: 输入的测试项与次数
echo "您选择的测试项是: $1"
echo "您希望进行 $cnt 次测试。"
echo "日志输出设置为: VERBOSE=$verbose"

# 测试函数
tests() {
    num=$1
    test_type=$2
    while [ $num -gt 0 ]; do
        echo "正在执行测试3$test_type，剩余次数: $num"
        if [ "$verbose" -eq 1 ]; then
            VERBOSE=1 go test -race -run 3$test_type >> test3${test_type}_${cnt}_times
        else
            VERBOSE=0 go test -race -run 3$test_type
        fi
        if [ $? -ne 0 ]; then
            echo "错误: 在执行测试3$test_type 时发生了问题，脚本终止。"
            exit 1
        fi
        num=$((num - 1))  # 使用 $(( ... )) 代替 let
    done

    echo "测试3$test_type 通过，总次数: $cnt"
    if [ "$verbose" -eq 1 ]; then
        echo "测试结果已保存到 test3${test_type}_${cnt}_times 文件中。"
    fi
}

num=$cnt

# 根据测试项执行对应的测试
case $1 in
  A)
    echo "开始执行 lab3A 测试，次数: ${cnt}..."
    tests $cnt 'A' $cnt
    ;;
  B)
    echo "开始执行 lab3B 测试，次数: ${cnt}..."
    tests $cnt 'B' $cnt
    ;;
  C)
    echo "开始执行 lab3C 测试，次数: ${cnt}..."
    tests $cnt 'C' $cnt
    ;;
  D)
    echo "开始执行 lab3D 测试，次数: ${cnt}..."
    tests $cnt 'D' $cnt
    ;;
  *)
    # 错误处理：无效的测试项
    echo "错误: '$1' 不是有效的测试项！"
    echo "有效的测试项为: A, B, C, D"
    echo "请使用: raft_test.sh <测试项> <测试次数> <VERBOSE>"
    exit 1
    ;;
esac

echo "测试完成！"

```

测试结果：跑50次无一出错

![image-20250219212206332](Lab3%20Raft.assets/image-20250219212206332.png)

![image-20250219213821717](Lab3%20Raft.assets/image-20250219213821717.png)

### **3C：持久性（困难）**

**Hint**

- 如果一个基于 Raft 的服务器重新启动，它应该从上次停止的地方继续服务。这要求 Raft 保持一个在重新启动后仍然存在的持久状态。论文中的图 2 提到了应该保持哪些状态
- 一个真正的实现会在 Raft 的持久状态每次更改时将其写入磁盘，并在重启后重新启动时从磁盘读取状态。你的实现不会使用磁盘；相反，它将从 `Persister` 对象（见 `persister.go` ）中保存和恢复持久状态。调用 `Raft.Make()` 的人提供了一个 `Persister` ，它最初包含 Raft 最近持久化的状态（如果有的话）。Raft 应从该 `Persister` 初始化其状态，并应使用它来保存每次状态更改时的持久状态。使用 `Persister` 的 `ReadRaftState()` 和 `Save()` 方法。
- 完成 `raft.go` 中的函数 `persist()` 和 `readPersist()` ，通过添加代码来保存和恢复持久状态。你需要将状态编码（或“序列化”）为字节数组，以便传递给 `Persister` 。使用 `labgob` 编码器；参见 `persist()` 和 `readPersist()` 中的注释。 `labgob` 类似于 Go 的 `gob` 编码器，但如果你尝试编码具有小写字段名的结构，则会打印错误消息。目前，将 `nil` 作为 `persister.Save()` 的第二个参数传递。在实现更改持久状态的点插入 `persist()` 的调用。完成这些后，如果你的实现其余部分正确，你应该通过所有 3C 测试。
- 您可能需要一次备份多个条目的 nextIndex 的优化。查看从第 7 页底部到第 8 页顶部（由灰色线条标记）的扩展 Raft 论文。论文对细节描述模糊；您需要填补空白。一种可能性是在AppendEntriesReply中包含：

```go
	XTerm  int //Follower发现与leader日志不一致时的任期（如果存在的话）
	XIndex int //具有该任期的第一个日志的索引（如果存在的话）
	XLen   int //日志长度
```

然后，Leader的逻辑可能类似于：

```go
Case 1: leader doesn't have XTerm:
    nextIndex = XIndex
Case 2: leader has XTerm:
    nextIndex = (index of leader's last entry for XTerm) + 1
Case 3: follower's log is too short:
    nextIndex = XLen
```

- 3C 测试比 3A 或 3B 测试要求更高，失败可能是由 3A 或 3B 代码中的问题引起的。

#### 分析

- 需要持久化的状态：currentTerm、votedFor、log
- 按注释实现 `persist()` and `readPersist()` 两个函数，之后在 Raft 改变 currentTerm、votedFor 或 log 时，及时调用 `rf.persist()` 即可。
  - RequestVote、AppendEntries、 Start、广播心跳、startElection

- 优化nextIndex

raft论文中提到：一旦Follower进行日志一致性检测发现不一致之后，在响应 leader 请求中包含自己在这个任期存储的第一条日志。这样 leader 接受到响应后，就可以直接跳过所有冲突的日志（其中可能包含了一致的日志）。这样就变成每个有冲突日志条目的任期需要一个AppendEntries RPC，而不是每个条目一次，就可以减少寻找一致点的过程。但是在实际中我们怀疑这种优化的必要性，因为失败发生的概率本来就很低，也不会同时存在大量不 一致的日志记录。

- （Case 3: follower落后leader）如果Follower在其日志中没有 `prevLogIndex` ，则应返回 `conflictIndex = len(log)` 和 `conflictTerm = None` 。
- （Case 2: leader has XTerm:）如果跟随者在其日志中确实有 `prevLogIndex` ，但任期不匹配，则应返回 `conflictTerm = log[prevLogIndex].Term`（Follower冲突的任期） ，然后在其日志中搜索第一个条目任期等于 `conflictTerm` 的索引（Follower冲突的任期的第一个索引，回退后再复制）。
- （Case 1: leader doesn't have XTerm，即落后的节点成了leader）如果它找不到带有该任期的条目，则应设置 `nextIndex = conflictIndex` 。（leader没有该任期的日志，直接nextIndex = conflictIndex进行覆盖）

![img](Lab3%20Raft.assets/v2-a6a37b478e3f66fca407bfced132acfa_r.jpg)

![image-20250221203826646](Lab3%20Raft.assets/image-20250221203826646.png)

#### 实现

**AppendEntries修改**

```go
// 当 Leader 任期小于当前节点任期时，返回 false，否则返回 true。
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// 如果我宕机，落后了leader很多日志
	if len(rf.logs) <= args.PrevLogIndex {
		//这种情况下，该raft实例断网一段时间过后，日志落后。所以直接返回 XLen即可。
		//leader更新nextIndex为XLen即可，表示当前raft实例缺少XLen及后面的日志，leader在下次广播时带上这些日志
		// leader   0{0} 1{101 102 103} 5{104}	PrevLogIndex=3	nextIndex=4
		// follower 0{0} 1{101 102 103} 5{104}  PrevLogIndex=3  nextIndex=4
		// follower 0{0} 1{101} 5 				PrevLogIndex=1  nextIndex=2
		reply.XTerm, reply.XIndex, reply.XLen = -1, -1, len(rf.logs)
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// 该日志在 prevLogIndex 上的任期不能和 prevLogTerm 匹配，说明日志不一致
	// 返回包含自己在冲突位置的任期存储的第一条日志
	if rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
		conflictIndex, conflictTerm := -1, rf.logs[args.PrevLogIndex].Term
		for i:=args.PrevLogIndex; i> rf.commitIndex;i--{
			if rf.logs[i].Term != conflictTerm{
				break
			}
			conflictIndex = i
		}
		reply.XTerm, reply.XIndex, reply.XLen = conflictTerm, conflictIndex, len(rf.logs)
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}
}
```

**找nextIndex的方法**

1. Follower日志短，落后Leader：nextIndex=Follower日志长度
2. Follower日志不短，但上一笔日志冲突，且Leader能找到XTerm，nextIndex=第一个条目任期等于conflictTerm的索引
3. Follower日志比Leader多（Leader找不到XTerm），nextIndex = XIndex
4. Follower上一笔日志不冲突，在后面部分冲突，直接删了再复制

```go
// 快速定位nextIndex
func (rf *Raft) findNextIndex(peer int, reply *AppendEntriesReply) {
	// Case 3: follower's log is too short:
	// Follower落后leader日志时
	if reply.XTerm == -1 && reply.XIndex == -1 {
		rf.nextIndex[peer] = reply.XLen
		return
	}

	flag := false
	// Case 2: leader has XTerm，即Follower没落后但冲突
	// Follower返回在发现不一致后的当前任期中存储的第一条日志
	// Leader在其日志中搜索第一个条目任期等于conflictTerm的索引，回退后再复制
	for i, entry := range rf.logs {
		if entry.Term == reply.XTerm {
			flag = true
			rf.nextIndex[peer] = i
		}
	}

	// leader doesn't have XTerm，即落后的节点成了leader，直接nextIndex = conflictIndex进行覆盖
	if !flag {
		rf.nextIndex[peer] = reply.XIndex
	}

}
```

#### 测试

![image-20250221210242603](Lab3%20Raft.assets/image-20250221210242603.png)

### 3D：日志压缩（困难）

#### 分析

当前情况下，重启的服务器会重放完整的 Raft 日志以恢复其状态。然而，对于长期运行的服务来说，永远记住完整的 Raft 日志并不实际。相反，您将修改 Raft 以与那些定期存储其状态“快照”的服务合作，此时 Raft 将丢弃快照之前的日志条目。结果是持久数据量更小，重启更快。然而，现在可能出现跟随者落后太多，以至于领导者丢弃了它需要追赶的日志条目；此时领导者必须发送快照以及从快照时间开始的日志。扩展 Raft 论文的第 7 节概述了该方案；您将不得不设计细节。

您的 Raft 必须提供以下功能，以便服务可以使用其状态的序列化快照进行调用：

在 3D中，测试器定期调用 `Snapshot()` 。在 lab4，你将编写一个键/值服务器，该服务器调用 `Snapshot()` ；快照将包含完整的键/值对表。服务层在每个对等节点上调用 `Snapshot()` （而不仅仅是领导者）。

`index` 参数指示在快照中反映的最高日志条目。Raft 应该在这一点之前丢弃其日志条目。您需要修改 Raft 代码以在仅存储日志末尾的情况下运行。

您需要实现论文中讨论的 `InstallSnapshot` RPC，该 RPC 允许 Raft 领导者告诉落后的 Raft 节点用快照替换其状态。您可能需要思考 InstallSnapshot 应该如何与图 2 中的状态和规则交互

当Follower的 Raft 代码接收到安装快照 RPC 时，它可以使用 `applyCh` 在 `ApplyMsg` 中发送快照。 `ApplyMsg` 结构定义已经包含了您需要的字段（以及测试者期望的字段）。请注意，这些快照只会推进服务的状态，而不会使其倒退。

如果服务器崩溃，它必须从持久化数据中重新启动。您的 Raft 应该持久化 Raft 状态和相应的快照。使用 `persister.Save()` 的第二个参数来保存快照。如果没有快照，将 `nil` 作为第二个参数传递。

当服务器重启时，应用层读取持久化的快照并恢复其保存的状态。

**任务**

实现 `Snapshot()` 和 InstallSnapshot RPC，以及为支持这些功能对 Raft 所做的更改（例如，与修剪日志的操作）。当您的解决方案通过 3D 测试（以及所有之前的 Lab 3 测试）时，您的解决方案才算完成。

**Hint**

- 一个好的开始是修改你的代码，使其能够只存储从某个索引 X 开始的日志部分。最初，你可以将 X 设置为 0 并运行 3B/3C 测试。然后让 `Snapshot(index)` 在 `index` 之前丢弃日志，并将 X 设置为 `index` 。如果一切顺利，你现在应该可以通过第一个 3D 测试。
- 下一步：如果领导者没有包含使跟随者同步所需的日志条目，则让领导者发送一个 InstallSnapshot RPC。
- 发送整个快照到单个 InstallSnapshot RPC。不要实现图 13 中的 `offset` 机制来分割快照。
- Raft 必须以允许 Go 垃圾回收器释放和重新使用内存的方式丢弃旧的日志条目；这要求没有可到达的引用（指针）指向被丢弃的日志条目。
- 完成 Lab 3 全部测试（3A+3B+3C+3D）所需合理时间为 6 分钟实际时间和 1 分钟 CPU 时间。使用 `-race` 时，大约需要 10 分钟实际时间和 2 分钟 CPU 时间。

#### 实现

##### **实现`Snapshot()`方法**

- 注意点1：快照点不能超过应用点

  该方法由应用层调用，应用层来决定何时对节点进行快照，而测试脚本中是每隔10条日志就进行一次快照。快照时需要注意，在lab2D前面的实现中，是把已提交和已应用的两个阶段通过条件变量分开的，中间这个间隙可能会被快照然后裁减掉未应用未提交甚至已提交的日志，这样可能会少了一些日志。为了保证在快照时，论文中的“已提交的日志一定会被应用到状态机”的特性，在快照时需要判断当前快照点是否超过了应用点，如果没有超过，说明可以快照；如果超过了应用点，就不能裁减log，防止前面提到的问题发生。

- 注意点2：如果当前快照点小于等于上一次快照点，没有必要快照了

- 注意点3：持久化的过程中，需要保证最新的快照和最新的raft持久化状态，一起持久化，保证原子性.

  这点在`persist()`方法的注释中有提到。为此我给raft添加了`snapshot`字段用来表示raft持有的最新快照，调用`persist()`方法的时候，将快照一并持久化，从而保证原子性。

```go
// PersistentStatus 持久化状态
type PersistentStatus struct {
	Logs        []LogEntry
	CurrentTerm int
	VotedFor    int

	// 3D
	lastIncludedIndex int // 快照替换的最后一条日志记录的索引
	lastIncludedTerm  int // 快照替换的最后一条日志记录的任期
}
```

##### **实现`InstallSnapshot RPC`**

要求不做分片，34不做

![image-20250224204114930](Lab3%20Raft.assets/image-20250224204114930.png)

1. 如果`term < currentTerm`就立即回复（过期leader请求没必要处理）

2. 创建一个新的快照

3. 保存快照文件，丢弃具有较小索引的任何现有或部分快照

   > 这句话的意思是: 比较对等点的快照和leader发过来的安装快照. 要丢弃较小的快照点, 保留最大的快照点.

4. 如果现存的日志条目与快照中最后包含的日志条目具有相同的索引值和任期号，则保留其后的日志条目并进行回复

   > 这句话的意思是: 可能包含了leader的安装快照之后的新的状态变化，我们需要保留这些, 并且return.

5. 丢弃整个日志

   > 如果在第4步没有return的话, 说明现存的日志条目与快照中最后不包含的日志条目具有相同的索引值和任期号, 也就是说, 当前log是过期的. 没有必要留存,直接删除掉

6. 使用快照重置状态机（并加载快照的集群配置）

除了论文和lab tips中的实现点以外，还有一些小的coner case需要注意：

- leader安装快照的过程请求了对等点, 算是一次ping/pong, 可以刷新选举计时器以及重新置为follower
- 如果对等点任期落后, 那么依然可以继续后面的步骤, 但是需要重置旧任期的选票和更新任期
- 使用Copy-on-Write的技术优化
- 注意`lastIncludedIndex`一定要在使用旧的`lastIncludedIndex`过后更新

```go
// InstallSnapshot RPC，leader使用InstallSnapshot RPC 来拷贝快照到那些远远落后的机器。
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() {
		Debug(dSnap, "after InstallSnapshot, S%d status{currentTerm:%d,commitIndex:%d,applied:%d,lastIncludedIndex:%d,log_len:%d}",
			rf.me, rf.currentTerm, rf.commitIndex, rf.lastApplied, rf.lastIncludedIndex, len(rf.logs)-1)
	}()
	Debug(dSnap, "before InstallSnapshot, S%d status{currentTerm:%d,commitIndex:%d,applied:%d,lastIncludedIndex:%d,log_len:%d}",
		rf.me, rf.currentTerm, rf.commitIndex, rf.lastApplied, rf.lastIncludedIndex, len(rf.logs)-1)

	// 1、如果leader的term小于当前term，则返回false
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		return
	}
	// 如果当前节点落后了leader，改任期和投票后，可以接着安装快照
	if args.Term > rf.currentTerm {
		rf.currentTerm, rf.votedFor = args.Term, noVote
	}
	// leader安装快照的过程请求了对等点, 算是一次ping/pong, 可以刷新选举计时器以及重新置为follower
	rf.state = Follower
	rf.electionTimer.Reset(randomElectionTimeout())

	// 5、保存快照文件，删除快照之前的日志
	// leader快照点落后于当前节点的快照点时，无需快照
	if args.LastIncludedIndex <= rf.lastIncludedIndex {
		reply.Term = rf.currentTerm
		return
	}
	// leader快照点落后于当前节点的提交点时，无需快照
	if args.LastIncludedIndex <= rf.commitIndex {
		reply.Term = rf.currentTerm
		return
	}

	defer rf.persist() // 持久化

	// 快照信息
	msg := ApplyMsg{
		SnapshotValid: true,
		Snapshot:      args.Data,
		SnapshotTerm:  args.LastIncludedTerm,
		SnapshotIndex: args.LastIncludedIndex,
	}

	//6、如果现存的日志条目与快照点具有相同的索引值和任期号，则保留其后的日志条目并进行回复
	for i := 1; i < len(rf.logs); i++ {
		if rf.realIndex(i) == args.LastIncludedIndex && rf.logs[0].Term == args.LastIncludedTerm {
			rf.logs = append([]LogEntry{{Term: args.LastIncludedTerm}}, rf.logs[i+1:]...)
			rf.lastIncludedIndex = args.LastIncludedIndex
			rf.commitIndex = args.LastIncludedIndex
			rf.lastApplied = args.LastIncludedIndex
			rf.snapshot = args.Data
			// 使用快照的状态重置状态机，并加载快照中的配置
			go func() {
				// 等待日志复制的日志进入管道过后才能把快照塞入管道
				for rf.replicatePreSnapshot.Load() { //不断加载，直到为false
					time.Sleep(time.Millisecond * 10) //等待10ms后再次检查
				}
				rf.applyMsg <- msg
			}()

			reply.Term = rf.currentTerm
			return
		}
	}

	// 7、丢弃整个日志（因为经过上述检查后，这里整个log都是过期的）
	rf.logs = []LogEntry{{Term: args.LastIncludedTerm}}
	rf.lastIncludedIndex = args.LastIncludedIndex
	rf.commitIndex = args.LastIncludedIndex
	rf.lastApplied = args.LastIncludedIndex
	rf.snapshot = args.Data

	// 8、使用快照的状态重置状态机，并加载快照中的配置。
	go func() {
		// 等待日志复制的日志进入管道过后才能把快照塞入管道
		for rf.replicatePreSnapshot.Load() { //不断加载，直到为false
			time.Sleep(time.Millisecond * 10) //等待10ms后再次检查
		}
		rf.applyMsg <- msg
	}()

	reply.Term = rf.currentTerm
}
```

##### 调用时机

- 有可能follower处理得太慢或者新加入集群, 由于前面的日志被快照了, 那么leader就无法在log中找到要发送给follower的日志了, 只能发送快照过去.

- 即使在日志被裁剪之后，你的实现仍然需要在 AppendEntries RPC中正确发送新条目之前条目的任期号（term）和索引号（index）。这可能需要保存并引用最新快照中的 `lastIncludedTerm`和 `lastIncludedIndex`，并考虑这些数据是否应该被持久化存储。

  发送日志复制RPC的请求参数中的`LastIncludedIndex`和`LastIncludedTerm`在快照机制出现过后, 可能存在这样一种边界情况：

  ```go
  if rf.nextIndex[i] <= rf.lastIncludedIndex { //下一个要复制的日志在快照里面，发送安装快照RPC
  	args := &InstallSnapshotArgs{
  		Term:              rf.currentTerm,
  		LeaderId:          rf.me,
  		LastIncludedIndex: rf.lastIncludedIndex,
  		LastIncludedTerm:  rf.logs[0].Term,
  		Data:              rf.snapshot,
  	}
  	Debug(dSnap, `sendInstallSnapshot S%d -> S%d, args{LastIncludedIndex:%d,LastIncludedTerm:%d}`,
  		rf.me, i, args.LastIncludedIndex, args.LastIncludedTerm)
  	go rf.handleSendInstallSnapshot(i, args)
  } else { //下一个要复制的日志存在于未裁减的log中，发起日志复制RPC
  	args := &AppendEntriesArgs{
  		Term:         rf.currentTerm,
  		LeaderId:     rf.me,
  		PrevLogIndex: rf.nextIndex[i] - 1,
  		Entries:      make([]LogEntry, 0),
  		LeaderCommit: rf.commitIndex,
  	}
  	// 下一个日志在leader log里，且前一个日志没在快照里，也在leader log里
  	if args.PrevLogIndex > rf.lastIncludedIndex &&
  		args.PrevLogIndex < rf.lastIncludedIndex+len(rf.logs) {
  		args.PrevLogTerm = rf.logs[rf.logIndex(args.PrevLogIndex)].Term
  	} else if args.PrevLogIndex == rf.lastIncludedIndex { //下一个日志在leader log里，但上一个日志在快照里，没在leader log里
  		args.PrevLogTerm = rf.logs[0].Term
  	} else { // 排除极端情况：PrevLogIndex>=rf.lastIncludedIndex+len(rf.log) ,这种情况说明raft实现可能存在问题，因为leader的日志落后于follower了
  		panic(fmt.Sprintf(`S%d -> S%d may meet some errors on raft implementation: PrevLogIndex(%d)>={lastIncludedIndex+len(log)(%d)`,
  			rf.me, i, args.PrevLogIndex, rf.lastIncludedIndex+len(rf.logs)))
  	}
  }
  ```

##### 修改applier协程

```go
// 应用日志到状态机
func (rf *Raft) applierEvent() {
	for !rf.killed() {
		rf.mu.Lock()
		for rf.commitIndex <= rf.lastApplied { //用for防止虚假唤醒
			rf.cond.Wait() //等待commitIndex变化唤醒该协程
		}
		msgs := make([]ApplyMsg, 0, rf.commitIndex-rf.lastApplied)
		for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
			msgs = append(msgs, ApplyMsg{
				CommandValid: true,
				Command:      rf.logs[rf.logIndex(i)].Cmd, //有快照后要用真实index在log中的索引
				CommandIndex: i,
			})
		}
		rf.mu.Unlock()
		Debug(dLog2, "S%d need apply msg{%+v}", rf.me, msgs)
		for _, msg := range msgs {
			rf.mu.Lock()
			if msg.CommandIndex != rf.lastApplied+1 { //下一个apply的log一定是lastApplied+1，否则就是被快照按照替代了
				rf.mu.Unlock()
				continue
			}
			// 如果执行到这里了，说明下一个msg一定是日志复制，而不是快照安装
			rf.replicatePreSnapshot.Store(true) // 设true让快要执行快照安装的协程要先阻塞等待一会
			rf.mu.Unlock()
			rf.applyMsg <- msg
			rf.replicatePreSnapshot.Store(false) // 解除快照安装协程的等待
			rf.mu.Lock()
			// 每apply一个log便及时更新lastApplied
			rf.lastApplied = max(msg.CommandIndex, rf.lastApplied)
			Debug(dLog2, "S%d lastApplied:%d", rf.me, rf.lastApplied)
			rf.mu.Unlock()
		}
	}
}
```

**最后处理有快照之后实际索引和在日志中的索引之间的对应关系即可**

#### 测试

![image-20250225192047214](Lab3%20Raft.assets/image-20250225192047214.png)

![image-20250225192728545](Lab3%20Raft.assets/image-20250225192728545.png)

