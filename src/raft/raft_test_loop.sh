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