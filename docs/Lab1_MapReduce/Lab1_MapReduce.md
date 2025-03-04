# Lab1 MapReduce

## 实验网站地址

https://pdos.csail.mit.edu/6.824/labs/lab-mr.html

## 原理视频

<video src="Lab1 MapReduce.assets/MapReduce.mp4" />

## Paper中MapReduce模型中的Fault Tolerance机制

![image-20241216213623476](Lab1%20MapReduce.assets/image-20241216213623476.png)

## Rules以及Hints中需要注意的地方

![image-20241216213644385](Lab1%20MapReduce.assets/image-20241216213644385.png)

## 各个test的主要逻辑

![image-20241216213807222](Lab1%20MapReduce.assets/image-20241216213807222.png)

## 主要文件

**`src/mr/coordinator.go`**
**`src/mr/rpc.go`**
**`src/mr/worker.go`**

## 实现

### 1、小任务：worker发RPC请求

一种开始的方法是修改 mr/worker.go 的 Worker（） 以向协调器发送 RPC 请求任务。然后修改协调器以使用尚未启动的映射任务的文件名进行响应。然后修改 worker 以读取该文件并调用应用程序 Map 函数，就像在 mrsequential.go 中一样。

coordinator.go

```go
//定义任务实现类
type Task struct{
	name string
}

type Coordinator struct {
	// Your definitions here.
	task Task

}
```

rpc.go

```go
// RPC定义
type TaskRequest struct{
	X int
}

type TaskResponse struct{
	Name string
}
```

coordinator.go

```go
//RPC示例
func (c *Coordinator) GetTask(args *TaskRequest, reply *TaskResponse) error {
	reply.Name = c.task.name
	return nil
}
```

work.go

```go
func CallGetTask() {

	// declare an argument structure.
	args := TaskRequest{}

	// fill in the argument(s).

	// declare a reply structure.
	reply := TaskResponse{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.GetTask", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Name %s\n", reply.Name)
	} else {
		fmt.Printf("call failed!\n")
	}
}
```

调用

```go
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.

	// uncomment to send the Example RPC to the coordinator.
	// CallExample()
	CallGetTask()

}
```

coordinator.go初始化

```go
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{task: Task{name: "hello"}}

	// Your code here.


	c.server()
	return &c
}
```

### 2、最终实现

#### 相关结构体定义

coordinator.go

```go
// 任务超时时间
const Timeout = 10

// 定义任务实现类
type Task struct {
	MapId    int
	ReduceId int
	FileName string
}

// master
type Coordinator struct {
	// int32 类型，保证原子操作
	State          int32 //0 Map 1 Reduce 2完成
	NumMapTasks    int
	NumReduceTasks int
	// 任务通道，保障任务的并发安全
	MapTask    chan Task
	ReduceTask chan Task
	// 类别{id,TaskState}   任务id：任务时间戳及完成状态
	MapTaskStates    sync.Map //并发安全的Map，不需要额外加锁
	ReduceTaskStates sync.Map
	// 任务文件列表
	files []string
}

// 任务时间戳及完成状态
type TaskState struct {
	// 任务未完成时存储提交时间，完成时存储完成时间
	Time int64
	// 任务完成状态
	Fin bool
}
```

rpc.go

```go
//worker只是向coordinator获取一个任务，所以不需要传入参数
type TaskRequest struct {}

type TaskResponse struct {
	XTask             Task //Map or Reduce Task
	NumMapTasks       int
	NumReduceTasks    int
	CurNumMapTasks    int
	CurNumReduceTasks int
	State             int32 //保障原子操作
}
```

#### 初始化

```go
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		State:          0,
		MapTask:        make(chan Task, len(files)),
		ReduceTask:     make(chan Task, nReduce),
		NumMapTasks:    len(files),
		NumReduceTasks: nReduce,
		files:          files,
	}

	// Map任务通道
	for id, filename := range files {
		c.MapTask <- Task{FileName: filename, MapId: id}
	}

	// 开启一个线程监听worker.go的RPC请求
	c.server()
	return &c
}

```

#### RPC操作函数

coordinator.go

```go
// 统计任务完成数量
// 类别{id,TaskState}   任务id：任务时间戳及完成状态
func lenTaskFin(m *sync.Map) int {
	len := 0
	m.Range(func(key, value interface{}) bool {
		//这是 Go 的类型断言（Type Assertion）语法。
		//value 是 sync.Map 中存储的元素，它的实际类型是 interface{}。
		//因此，你需要通过类型断言将它转换为 TaskState 类型。
		if value.(TaskState).Fin {
			len++
		}
		return true
	})
	return len
}

// Your code here -- RPC handlers for the worker to call.

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
// 获取任务的RPC
func (c *Coordinator) GetTask(args *TaskRequest, reply *TaskResponse) error {
	// 原子操作获取任务状态
	state := atomic.LoadInt32(&c.State)
	// Map任务
	if state == 0 {
		//读Map，加这个判断是防止为空的时候阻塞住
		if len(c.MapTask) != 0 {
			if maptask, ok := <-c.MapTask; ok {
				reply.XTask = maptask
				c.MapTaskStates.Store(maptask.MapId, TaskState{time.Now().Unix(), false})
			}
			reply.CurNumMapTasks = len(c.MapTask)
		} else {
			// 如果Map任务为空，则返回-1
			reply.CurNumMapTasks = -1
		}
		reply.CurNumReduceTasks = len(c.ReduceTask)
	} else if c.State == 1 { //Map任务全部完成，读Reduce任务
		if len(c.ReduceTask) != 0 {
			if reduceTask, ok := <-c.ReduceTask; ok {
				reply.XTask = reduceTask
				c.ReduceTaskStates.Store(reduceTask.ReduceId, TaskState{time.Now().Unix(), false})
			}
			reply.CurNumReduceTasks = len(c.ReduceTask)
		} else {
			// 如果Reduce任务为空，则返回-1
			reply.CurNumReduceTasks = -1
		}
		reply.CurNumMapTasks = -1
	}
	// 返回任务总数
	reply.NumMapTasks = c.NumMapTasks
	reply.NumReduceTasks = c.NumReduceTasks
	// 返回任务状态
	reply.State = state
	return nil
}

// 任务完成时通知的RPC
func (c *Coordinator) TaskFin(args *Task, reply *TaskResponse) error {
	timeNow := time.Now().Unix()
	// Map任务没有完成
	if lenTaskFin(&c.MapTaskStates) != c.NumMapTasks {
		curState, _ := c.MapTaskStates.Load(args.MapId)
		// 如果Map任务超时，则返回
		// 断言TaskState类别
		if timeNow-curState.(TaskState).Time > Timeout {
			return nil
		}
		//打标记该任务完成
		c.MapTaskStates.Store(args.MapId, TaskState{timeNow, true})
		// fmt.Printf("Map task %d is finished\n", args.MapId)
		// 如果所有Map任务完成，则开始Reduce任务
		if lenTaskFin(&c.MapTaskStates) == c.NumMapTasks {
			// 设置状态为Reduce，原子操作
			atomic.StoreInt32(&c.State, 1)
			// 向ReduceTask通道中发送Reduce任务
			for i := 0; i < c.NumReduceTasks; i++ {
				c.ReduceTask <- Task{ReduceId: i}
			}
		}
	} else if lenTaskFin(&c.ReduceTaskStates) != c.NumReduceTasks { //Map任务全部完成，读Reduce任务
		// Reduce任务没有完成
		curState, _ := c.ReduceTaskStates.Load(args.ReduceId)
		// 如果Reduce任务超时，则返回
		if timeNow-curState.(TaskState).Time > Timeout {
			return nil
		}
		// 打标记该任务完成
		c.ReduceTaskStates.Store(args.ReduceId, TaskState{timeNow, true})
		// fmt.Printf("Reduce task %d is finished\n", args.ReduceId)
		// 如果所有Reduce任务完成，则设置状态为完成
		if lenTaskFin(&c.ReduceTaskStates) == c.NumReduceTasks {
			atomic.StoreInt32(&c.State, 2)
		}
	}
	return nil
}
```

worker.go

```go
func CallGetTask(args *TaskRequest, reply *TaskResponse) {

	// // declare an argument structure.
	// args := TaskRequest{}

	// // fill in the argument(s).

	// // declare a reply structure.
	// reply := TaskResponse{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.GetTask", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("call get task ok!\n")
	} else {
		fmt.Printf("call failed!\n")
	}
}

func CallTaskFin(args *Task) {
	reply := TaskResponse{}
	ok := call("Coordinator.TaskFin", &args, &reply)
	if ok {
		fmt.Printf("call task finish!\n")
	} else {
		fmt.Printf("call failed!\n")
	}
}
```

#### worker处理逻辑

```go
// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.

	// uncomment to send the Example RPC to the coordinator.
	// CallExample()

	for {
		args := TaskRequest{}
		reply := TaskResponse{}
		// 发获取任务请求
		CallGetTask(&args, &reply)
		CurNumMapTasks := reply.CurNumMapTasks
		CurNumReduceTasks := reply.CurNumReduceTasks
		state := reply.State
		if CurNumMapTasks >= 0 && state == 0 { //Map Task
			if err := DoMap(&reply, mapf); err != nil {
                log.Fatalf("Map task failed: %v", err)
            }
			// 发送Map任务完成信号
			// 不能直接写成	reply.MapTaskFin <- true ，跨进程信用RPC封装
			CallTaskFin(&reply.XTask)
		} else if CurNumReduceTasks >= 0 && state == 1 { //Reduce Task
			if err := DoReduce(&reply, reducef); err != nil {
                log.Fatalf("Reduce task failed: %v", err)
            }
			//标记已完成
			CallTaskFin(&reply.XTask)
		} else if state == 2 { //所有任务均完成
			fmt.Println(">>>> finish!")
			break
		} else { //等待所有reduce任务都结束后，再退出
			continue
		}
		// Worker 有时需要等待，例如，在最后一个 map 完成之前reduce不应该启动。
		//这边是保正在任务完成协程写入最后一条数据写完之后再做判断
		// 预防TaskFin发送的时间过长，导致Coordinator没有接收到信息，导致reduce过程无法发生
		time.Sleep(time.Second)
	}
}

// Map
func DoMap(reply *TaskResponse, mapf func(string, string) []KeyValue) error {
	filename := reply.XTask.FileName
	// strconv.Itoa 将int转换为string
	MapId := strconv.Itoa(reply.XTask.MapId)
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open mapTask %v", filename)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read mapTask %v", filename)
	}
	file.Close()
	//传filename和内容返回<key,v> List
	kva := mapf(filename, string(content))

	//map 阶段应将中间键划分为 nReduce reduce 任务的存储桶，其中 nReduce 是 reduce 任务的数量
	//建立nReduce个数的桶，判断分给哪个reduce
	nReduce := reply.NumReduceTasks
	buckets := make([][]KeyValue, nReduce)
	for _, kv := range kva {
		//使用ihash为给定的 key 选择 reduce 任务
		num := ihash(kv.Key) % nReduce
		buckets[num] = append(buckets[num], kv)
	}
	//为了确保在崩溃的情况下没有人观察到部分写入的文件，MapReduce 论文提到了使用临时文件
	//并在完全写入后对其进行原子重命名的技巧。您可以使用 ioutil.TempFile（或 os.CreateTemp
	//（如果您运行的是 Go 1.17 或更高版本）创建临时文件和 os.Rename 以原子方式重命名它
	for i := 0; i < nReduce; i++ {
		tmpFile, error := os.CreateTemp("", "mr-map-*")
		if error != nil {
			log.Fatalf("cannot create temp file: %v", err)
		}

		// 创建一个 JSON 编码器
		enc := json.NewEncoder(tmpFile)
		err := enc.Encode(buckets[i])
		if err != nil {
			log.Fatalf("encode bucket error")
		}
		tmpFile.Close()
		//中间文件的合理命名约定是 mr-X-Y，其中 X 是 Map 任务编号，Y 是 reduce 任务编号。
		outFileName := "mr-" + MapId + "-" + strconv.Itoa(i)
		os.Rename(tmpFile.Name(), outFileName)
	}
	return nil
}

// Reduce
func DoReduce(reply *TaskResponse, reducef func(string, []string) string) error {
	//读取map中间文件存储键值对
	intermediate := []KeyValue{}
	//定位文件
	ReduceId := strconv.Itoa(reply.XTask.ReduceId)
	for i := 0; i < reply.NumMapTasks; i++ {
		MapOutFileName := "mr-" + strconv.Itoa(i) + "-" + ReduceId
		file, err := os.Open(MapOutFileName)
		if err != nil {
			log.Fatalf("cannot open reduceTask %v", MapOutFileName)
		}
		//解析json
		dec := json.NewDecoder(file)
		for {
			var kv []KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			// 将kv切片解包添加到intermediate
			intermediate = append(intermediate, kv...)
		}
	}
	//按键值排序
	sort.Sort(ByKey(intermediate))
	//仍然先生成临时文件写，再重命名。
	tmpFile, err := os.CreateTemp("", "mr-reduce-*")
	if err != nil {
		log.Fatalf("cannot create temp file: %v", err)
	}
	//mrsequential中的聚合操作
	i := 0
	for i < len(intermediate) {
		j := i + 1
		// 找到相同键的所有元素
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		// 收集相同键的所有值
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		//聚合
		output := reducef(intermediate[i].Key, values)

		// this is the correct format for each line of Reduce output.
		fmt.Fprintf(tmpFile, "%v %v\n", intermediate[i].Key, output)

		i = j
	}
	tmpFile.Close()
	//第 X 个 reduce 任务的输出放在文件 mr-out-X 中。
	outFileName := "mr-out-" + ReduceId
	os.Rename(tmpFile.Name(), outFileName)
	return nil
}
```

#### 任务超时检测处理

```go
// 检查任务是否超时
func (c *Coordinator) TimeTick() {
	state := atomic.LoadInt32(&c.State)
	timeNow := time.Now().Unix()

	// Map任务是否超时
	if state == 0 {
		for i := 0; i < c.NumMapTasks; i++ {
			// c.MapTaskStates有i这个key，再去Load
			if _, ok := c.MapTaskStates.Load(i); ok {
				tmp, _ := c.MapTaskStates.Load(i)
				// 如果任务未完成且超时，则重新发送任务
				if !tmp.(TaskState).Fin && timeNow-tmp.(TaskState).Time > Timeout {
					fmt.Printf("Map task %d is timeout\n", i)
					c.MapTask <- Task{FileName: c.files[i], MapId: i}
				}
			}
		}
	} else if state == 1 { // Reduce任务是否超时
		for i := 0; i < c.NumReduceTasks; i++ {
			// c.ReduceTaskStates有i这个key，再去Load
			if _, ok := c.ReduceTaskStates.Load(i); ok {
				tmp, _ := c.ReduceTaskStates.Load(i)
				// 如果任务未完成且超时，则重新发送任务
				if !tmp.(TaskState).Fin && timeNow-tmp.(TaskState).Time > Timeout {
					fmt.Printf("Reduce task %d is timeout\n", i)
					c.ReduceTask <- Task{ReduceId: i}
				}
			}
		}
	}
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	// 检查任务是否超时
	c.TimeTick()
	ret := false

	// Your code here.
	// 任务完成
	if lenTaskFin(&c.ReduceTaskStates) == c.NumReduceTasks {
		ret = true
	}

	return ret
}
```

## 运行测试

**`cd src/main`**
**`bash test-mr.sh`**

**可以通过所有测试：**

![img](Lab1%20MapReduce.assets/396498748-396c84bd-bc81-4f77-bfca-fead9bc18a5d.png)

