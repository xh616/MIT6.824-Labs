package main

//
// a word-count application "plugin" for MapReduce.
//
// go build -buildmode=plugin wc.go
//

import (
	"strconv"
	"strings"
	"unicode"

	"6.5840/mr"
)

// The map function is called once for each file of input. The first
// argument is the name of the input file, and the second is the
// file's complete contents. You should ignore the input file name,
// and look only at the contents argument. The return value is a slice
// of key/value pairs.
func Map(filename string, contents string) []mr.KeyValue {
	// function to detect word separators.
	// 定义了一个匿名函数ff，用于检测字符是否是分隔符
	// 检查字符r是否是字母。如果不是字母，返回true（表示该字符是分隔符
	ff := func(r rune) bool { return !unicode.IsLetter(r) }

	// split contents into an array of words.
	// strings.FieldsFunc: 按照给定的分隔规则ff将contents拆分成字符串数组words。
	words := strings.FieldsFunc(contents, ff)

	kva := []mr.KeyValue{}
	for _, w := range words {
		// key是单词，val是1
		kv := mr.KeyValue{w, "1"}
		kva = append(kva, kv)
	}
	return kva
}

// The reduce function is called once for each key generated by the
// map tasks, with a list of all the values created for that key by
// any map task.
func Reduce(key string, values []string) string {
	// return the number of occurrences of this word.
	// len(values): 计算切片values的长度，这个长度等于单词key在输入数据中的总出现次数。
	// strconv.Itoa: 将整数转换为字符串，因为返回值类型是string。
	return strconv.Itoa(len(values))
}
