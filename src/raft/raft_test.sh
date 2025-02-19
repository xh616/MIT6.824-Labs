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
