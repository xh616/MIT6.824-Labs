package main

//
// simple sequential MapReduce.
//
// go run mrsequential.go wc.so pg*.txt
//

import (
	"fmt"
	"io"
	"log"
	"os"
	"plugin"
	"sort"

	"6.5840/mr"
)

// for sorting by key.
type ByKey []mr.KeyValue

// for sorting by key.
// 实现 sort.Interface 接口
// Go 的 sort.Interface 接口要求我们实现三个方法：
// Len(): 返回切片的长度。
// Swap(i, j int): 交换切片中两个元素的位置。
// Less(i, j int): 判断切片中第 i 个元素是否小于第 j 个元素（用于排序的比较函数）。
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: mrsequential xxx.so inputfiles...\n")
		os.Exit(1)
	}

	// go run mrsequential.go wc.so pg*.txt
	// wc.so是go build -buildmode=plugin ../mrapps/wc.go生成的
	// 返回两个函数
	mapf, reducef := loadPlugin(os.Args[1])

	//
	// read each input file,
	// pass it to Map,
	// accumulate the intermediate Map output.
	//
	intermediate := []mr.KeyValue{}
	// pg*.txt
	for _, filename := range os.Args[2:] {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("cannot read %v", filename)
		}
		file.Close()
		kva := mapf(filename, string(content))
		//将kva切片的内容追加到临时intermediate中。kva...是将kva切片展开，并将其中的元素逐个追加到intermediate中。
		// kva 是一个切片（类型为 []mr.KeyValue），它可能包含多个 mr.KeyValue 元素。
		// kva... 是 Go 的切片展开操作（variadic argument expansion），它将 kva 切片的每一个元素单独拆开，作为独立的元素传递给 append 函数。
		intermediate = append(intermediate, kva...)
	}

	//
	// a big difference from real MapReduce is that all the
	// intermediate data is in one place, intermediate[],
	// rather than being partitioned into NxM buckets.
	//

	sort.Sort(ByKey(intermediate))

	oname := "mr-out-0"
	ofile, _ := os.Create(oname)

	//
	// call Reduce on each distinct key in intermediate[],
	// and print the result to mr-out-0.
	//
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
		fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

		i = j
	}

	ofile.Close()
}

// load the application Map and Reduce functions
// from a plugin file, e.g. ../mrapps/wc.so
func loadPlugin(filename string) (func(string, string) []mr.KeyValue, func(string, []string) string) {
	p, err := plugin.Open(filename)
	if err != nil {
		log.Fatalf("cannot load plugin %v", filename)
	}
	//在加载的插件中查找名为Map的符号
	xmapf, err := p.Lookup("Map")
	if err != nil {
		log.Fatalf("cannot find Map in %v", filename)
	}
	//将找到的符号xmapf类型断言为Map函数的类型
	//传filename和内容返回<key,v> List
	mapf := xmapf.(func(string, string) []mr.KeyValue)
	xreducef, err := p.Lookup("Reduce")
	if err != nil {
		log.Fatalf("cannot find Reduce in %v", filename)
	}
	//传key和v返回v长度
	reducef := xreducef.(func(string, []string) string)

	return mapf, reducef
}
