#!/bin/bash

# 快速性能测试脚本
set -e

echo "🚀 HighGoPress Quick Performance Test"
echo "====================================="

BASE_URL="http://localhost:8080"
TEST_ARTICLE="article_$(date +%s)"
HEY_BIN="$HOME/go/bin/hey"

# 检查hey工具
if [ ! -f "$HEY_BIN" ]; then
    echo "Installing hey..."
    go install github.com/rakyll/hey@latest
fi

# 封装hey并解析结果
run_and_parse() {
    local title="$1"
    shift
    
    echo "$title"
    # 将hey的输出存到临时变量
    local output
    output=$($HEY_BIN "$@")
    
    echo "$output"

    # 提取关键指标
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

# 检查服务状态
echo "🔍 Checking service..."
if curl -s "$BASE_URL/api/v1/health" > /dev/null; then
    echo "✅ Service is running"
else
    echo "❌ Service is not running"
    exit 1
fi

echo ""
echo "📊 Running Performance Tests..."
echo "==============================="

# 测试1：健康检查基准
run_and_parse "1. Health Check Performance (1000 requests, 10 concurrent)" \
    -n 1000 -c 10 "$BASE_URL/api/v1/health"

echo ""
echo "2. Counter Read Performance (1000 requests, 10 concurrent)"
# 先创建一个计数器
curl -s -X POST "$BASE_URL/api/v1/counter/increment" \
    -H "Content-Type: application/json" \
    -d "{\"resource_id\":\"$TEST_ARTICLE\",\"counter_type\":\"like\",\"delta\":1}" > /dev/null

run_and_parse "Counter Read Performance" \
    -n 1000 -c 10 "$BASE_URL/api/v1/counter/$TEST_ARTICLE/like"

echo ""
echo "3. Counter Write Performance (1000 requests, 10 concurrent)"
run_and_parse "3. Counter Write Performance (1000 requests, 10 concurrent)" \
    -n 1000 -c 10 -m POST \
    -H "Content-Type: application/json" \
    -D <(echo "{\"resource_id\":\"$TEST_ARTICLE\",\"counter_type\":\"like\",\"delta\":1}") \
    "$BASE_URL/api/v1/counter/increment"

echo ""
echo "4. High Load Test (5000 requests, 50 concurrent)"
run_and_parse "4. High Load Test (5000 requests, 50 concurrent)" \
    -n 5000 -c 50 -m POST \
    -H "Content-Type: application/json" \
    -D <(echo "{\"resource_id\":\"$TEST_ARTICLE\",\"counter_type\":\"like\",\"delta\":1}") \
    "$BASE_URL/api/v1/counter/increment"

echo ""
echo "🔍 Verifying Data Consistency..."
final_count=$(curl -s "$BASE_URL/api/v1/counter/$TEST_ARTICLE/like" | grep -o '"current_value":[0-9]*' | cut -d':' -f2)
echo "Final count for $TEST_ARTICLE: $final_count"

if [ "$final_count" -gt 6000 ]; then
    echo "✅ Data consistency verified (expected ~6001, got $final_count)"
else
    echo "⚠️  Possible data consistency issue (expected ~6001, got $final_count)"
fi

echo ""
echo "✅ Quick performance test completed!" 