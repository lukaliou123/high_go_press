#!/bin/bash

# 简单的负载测试脚本
# 使用hey工具进行压测

echo "=== HighGoPress Load Testing ==="

# 检查hey是否安装
if ! command -v hey &> /dev/null; then
    echo "Installing hey tool..."
    go install github.com/rakyll/hey@latest
fi

# 服务地址
HOST="http://localhost:8080"
HEY_BIN="$HOME/go/bin/hey"

# 封装hey并解析结果
run_and_parse() {
    local title="$1"
    shift
    
    echo "$title"
    local output
    output=$($HEY_BIN "$@")
    echo "$output"

    local qps
    qps=$(echo "$output" | grep "Requests/sec:" | awk '{print $2}')
    local p99
    p99=$(echo "$output" | grep "99% in" | awk '{print $3, $4}')
    
    echo "-------------------------------------"
    echo "  📊 Summary for: $title"
    echo "  - QPS (Requests/sec): $qps"
    echo "  - P99 Latency:        $p99"
    echo "-------------------------------------"
}

run_and_parse "1. 测试健康检查接口 - 10000 请求, 100 并发" \
    -n 10000 -c 100 $HOST/api/v1/health

echo ""
run_and_parse "2. 测试计数器查询接口 - 5000 请求, 50 并发" \
    -n 5000 -c 50 $HOST/api/v1/counter/article_001/like

echo ""
run_and_parse "3. 测试计数器增量接口 - 3000 请求, 30 并发" \
    -n 3000 -c 30 -m POST \
    -H "Content-Type: application/json" \
    -d '{"resource_id": "article_loadtest", "counter_type": "like", "delta": 1}' \
    $HOST/api/v1/counter/increment

echo ""
echo "4. 查看最终计数结果"
curl -s $HOST/api/v1/counter/article_loadtest/like

echo ""
echo "=== Load Testing Complete ===" 