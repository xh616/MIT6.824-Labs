package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

// Add your RPC definitions here.
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

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
