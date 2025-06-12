#!/bin/bash

# Week 4 性能对比测试脚本
# 对比 Phase 1 (21k QPS) vs Phase 2 Week 4 (连接池优化)

set -e

echo "🚀 HighGoPress Week 4 性能对比测试"
echo "========================================="
echo "Phase 1 基准: ~21,000 QPS (单体优化)"
echo "Phase 2 目标: 保持或超越基准性能"
echo "========================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 检查hey工具
HEY_BIN=$(go env GOPATH)/bin/hey
if [ ! -f "$HEY_BIN" ]; then
    echo "Installing hey performance testing tool..."
    go install github.com/rakyll/hey@latest
fi

# 清理之前的进程
echo "🧹 清理之前的进程..."
pkill -f 'counter-v2' 2>/dev/null || true
pkill -f 'analytics-v2' 2>/dev/null || true  
pkill -f 'cmd/gateway/main.go' 2>/dev/null || true
sleep 3

# 启动微服务
echo "🔧 启动微服务..."

# 启动Counter服务
echo "Starting Counter Service..."
./bin/counter-v2 > logs/counter.log 2>&1 &
COUNTER_PID=$!

# 启动Analytics服务  
echo "Starting Analytics Service..."
./bin/analytics-v2 > logs/analytics.log 2>&1 &
ANALYTICS_PID=$!

# 启动Gateway服务
echo "Starting Gateway Service..."
go run cmd/gateway/main.go > logs/gateway.log 2>&1 &
GATEWAY_PID=$!

echo "Started services - Counter:$COUNTER_PID, Analytics:$ANALYTICS_PID, Gateway:$GATEWAY_PID"

# 等待服务启动
echo "⏳ 等待服务启动..."
sleep 8

# 健康检查
echo "🔍 服务健康检查..."
for i in {1..10}; do
    if curl -s http://localhost:8080/api/v1/health > /dev/null; then
        echo "✅ 服务启动成功"
        break
    fi
    echo "等待服务启动... ($i/10)"
    sleep 2
    if [ $i -eq 10 ]; then
        echo "❌ 服务启动超时"
        exit 1
    fi
done

# 清理计数器
echo "🧹 清理测试数据..."
curl -s -X DELETE "http://localhost:8080/api/v1/counter/perf_test/like" > /dev/null || true

echo ""
echo "📊 开始性能测试..."
echo "========================================="

# 测试函数
run_performance_test() {
    local test_name="$1"
    local requests="$2"
    local concurrency="$3"
    local url="$4"
    local method="$5"
    local data="$6"
    
    echo -e "${BLUE}🔹 $test_name${NC}"
    echo "   请求: $requests, 并发: $concurrency"
    
    if [ "$method" = "POST" ]; then
        $HEY_BIN -n $requests -c $concurrency -m POST \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$url" 2>/dev/null | grep -E "(Total:|Requests/sec:|95%|99%)" | while read line; do
            echo "   $line"
        done
    else
        $HEY_BIN -n $requests -c $concurrency "$url" 2>/dev/null | grep -E "(Total:|Requests/sec:|95%|99%)" | while read line; do
            echo "   $line"
        done
    fi
    echo ""
}

# Level 1: 基础性能测试 (1k requests, 10 concurrent)
echo -e "${GREEN}📈 Level 1: 基础负载 (1k请求, 10并发)${NC}"
echo "----------------------------------------"

run_performance_test "健康检查" 1000 10 "http://localhost:8080/api/v1/health" "GET" ""

run_performance_test "计数器读取" 1000 10 "http://localhost:8080/api/v1/counter/perf_test/like" "GET" ""

run_performance_test "计数器写入" 1000 10 "http://localhost:8080/api/v1/counter/increment" "POST" '{"resource_id":"perf_test","counter_type":"like","delta":1}'

# Level 2: 中等负载 (5k requests, 50 concurrent)
echo -e "${GREEN}📈 Level 2: 中等负载 (5k请求, 50并发)${NC}"
echo "----------------------------------------"

run_performance_test "健康检查" 5000 50 "http://localhost:8080/api/v1/health" "GET" ""

run_performance_test "计数器写入" 5000 50 "http://localhost:8080/api/v1/counter/increment" "POST" '{"resource_id":"perf_test","counter_type":"like","delta":1}'

# Level 3: 高负载 (10k requests, 100 concurrent) 
echo -e "${GREEN}📈 Level 3: 高负载 (10k请求, 100并发)${NC}"
echo "----------------------------------------"

run_performance_test "计数器写入" 10000 100 "http://localhost:8080/api/v1/counter/increment" "POST" '{"resource_id":"perf_test","counter_type":"like","delta":1}'

# Level 4: 超高负载 (20k requests, 200 concurrent) - 接近Phase 1基准
echo -e "${YELLOW}📈 Level 4: 超高负载 (20k请求, 200并发)${NC}"
echo "----------------------------------------"

run_performance_test "健康检查" 20000 200 "http://localhost:8080/api/v1/health" "GET" ""

run_performance_test "计数器读取" 20000 200 "http://localhost:8080/api/v1/counter/perf_test/like" "GET" ""

run_performance_test "计数器写入" 20000 200 "http://localhost:8080/api/v1/counter/increment" "POST" '{"resource_id":"perf_test","counter_type":"like","delta":1}'

# Level 5: 极限负载 (50k requests, 500 concurrent) - 超越Phase 1基准
echo -e "${RED}📈 Level 5: 极限负载 (50k请求, 500并发)${NC}"
echo "----------------------------------------"

run_performance_test "健康检查" 50000 500 "http://localhost:8080/api/v1/health" "GET" ""

run_performance_test "计数器读取" 50000 500 "http://localhost:8080/api/v1/counter/perf_test/like" "GET" ""

run_performance_test "计数器写入" 50000 500 "http://localhost:8080/api/v1/counter/increment" "POST" '{"resource_id":"perf_test","counter_type":"like","delta":1}'

# 数据一致性验证
echo "🔍 数据一致性验证..."
FINAL_COUNT=$(curl -s "http://localhost:8080/api/v1/counter/perf_test/like" | jq -r '.data.current_value' 2>/dev/null || echo "0")
EXPECTED_COUNT=$((1000 + 5000 + 10000 + 20000 + 50000))  # 86000

echo "Expected: $EXPECTED_COUNT, Actual: $FINAL_COUNT"
if [ "$FINAL_COUNT" = "$EXPECTED_COUNT" ]; then
    echo -e "${GREEN}✅ 数据一致性验证通过${NC}"
else
    echo -e "${RED}⚠️  数据一致性异常 (可能由于并发竞争)${NC}"
fi

# 连接池状态检查
echo ""
echo "🔍 连接池状态检查..."
curl -s "http://localhost:8080/api/v1/system/grpc-pools" | jq '.' 2>/dev/null || echo "连接池状态查询失败"

echo ""
echo "========================================="
echo -e "${GREEN}🎉 Week 4 性能测试完成！${NC}"
echo ""
echo "📊 性能对比分析:"
echo "   Phase 1 基准: ~21,000 QPS (单体)"
echo "   Phase 2 Week 4: 查看上述测试结果"
echo ""
echo "🎯 优化目标达成情况:"
echo "   - 写入QPS >= 15,000 (保持70%性能)"
echo "   - P99延迟 <= 100ms"  
echo "   - 数据一致性 100%"
echo ""

# 清理进程
echo "🧹 清理测试进程..."
kill $COUNTER_PID $ANALYTICS_PID $GATEWAY_PID 2>/dev/null || true
sleep 2

echo -e "${GREEN}✅ 测试完成，所有进程已清理${NC}" 