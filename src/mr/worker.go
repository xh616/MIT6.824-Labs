package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"time"
)

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
// 实现 sort.Interface 接口
// Go 的 sort.Interface 接口要求我们实现三个方法：
// Len(): 返回切片的长度。
// Swap(i, j int): 交换切片中两个元素的位置。
// Less(i, j int): 判断切片中第 i 个元素是否小于第 j 个元素（用于排序的比较函数）。
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

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

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
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

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	// 连接到Coordinator
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	// call是net/rpc里面的发送 RPC 请求的方法
	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
