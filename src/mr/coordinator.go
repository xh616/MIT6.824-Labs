package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	// "strconv"
	"sync"
	"sync/atomic"
	"time"
)

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

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	// 注册RPC
	rpc.Register(c)
	// 开启HTTP服务
	rpc.HandleHTTP()
	// 监听1234端口
	//l, e := net.Listen("tcp", ":1234")
	// 监听unix套接字
	sockname := coordinatorSock()
	// 删除已存在的套接字
	os.Remove(sockname)
	// 监听unix套接字
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	// 开启HTTP服务
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	// 检查任务是否超时
	c.TimeTick()
	ret := false

	// 任务完成
	if lenTaskFin(&c.ReduceTaskStates) == c.NumReduceTasks {
		ret = true
	}

	return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
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
