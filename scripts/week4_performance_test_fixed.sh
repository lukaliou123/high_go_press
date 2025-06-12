#!/bin/bash

# Week 4 性能对比测试脚本 - 修复版本
# 对比 Phase 1 (21k QPS) vs Phase 2 Week 4 (连接池优化)

set -e

echo "🚀 HighGoPress Week 4 性能对比测试 (修复版)"
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

# 确保bin目录存在
mkdir -p bin logs

# 清理之前的进程
echo "🧹 清理之前的进程..."
pkill -f 'bin/counter' 2>/dev/null || true
pkill -f 'bin/analytics' 2>/dev/null || true  
pkill -f 'bin/gateway' 2>/dev/null || true
pkill -f 'cmd/gateway/main.go' 2>/dev/null || true
sleep 3

echo "🔧 编译所有服务..."
go build -o bin/counter cmd/counter/main.go
go build -o bin/analytics cmd/analytics/main.go  
go build -o bin/gateway cmd/gateway/main.go

# 启动微服务
echo "🚀 启动微服务..."

# 启动Counter服务
echo "Starting Counter Service..."
./bin/counter > logs/counter.log 2>&1 &
COUNTER_PID=$!

# 启动Analytics服务  
echo "Starting Analytics Service..."
./bin/analytics > logs/analytics.log 2>&1 &
ANALYTICS_PID=$!

# 启动Gateway服务 (使用编译后的二进制)
echo "Starting Gateway Service..."
./bin/gateway > logs/gateway.log 2>&1 &
GATEWAY_PID=$!

echo "Started services - Counter:$COUNTER_PID, Analytics:$ANALYTICS_PID, Gateway:$GATEWAY_PID"

# 等待服务启动
echo "⏳ 等待服务启动..."
sleep 8

# 健康检查
echo "🔍 服务健康检查..."
for i in {1..15}; do
    if curl -s http://localhost:8080/api/v1/health > /dev/null; then
        echo "✅ 服务启动成功"
        break
    fi
    echo "等待服务启动... ($i/15)"
    sleep 2
    if [ $i -eq 15 ]; then
        echo "❌ 服务启动超时，检查日志:"
        echo "Gateway日志:"
        tail -10 logs/gateway.log 2>/dev/null || echo "无法读取Gateway日志"
        echo "Counter日志:"
        tail -10 logs/counter.log 2>/dev/null || echo "无法读取Counter日志"
        exit 1
    fi
done

# 测试连接池状态
echo "🔍 测试连接池状态..."
POOL_STATUS=$(curl -s "http://localhost:8080/api/v1/system/grpc-pools" || echo "failed")
if [[ "$POOL_STATUS" == *"pool_size"* ]]; then
    echo "✅ 连接池状态正常"
else
    echo "⚠️  连接池状态异常: $POOL_STATUS"
fi

# 清理计数器
echo "🧹 清理测试数据..."
curl -s -X DELETE "http://localhost:8080/api/v1/counter/perf_test/like" > /dev/null || true

echo ""
echo "📊 开始性能测试..."
echo "========================================="

# 改进的测试函数 - 加入错误检测
run_performance_test() {
    local test_name="$1"
    local requests="$2"
    local concurrency="$3"
    local url="$4"
    local method="$5"
    local data="$6"
    
    echo -e "${BLUE}🔹 $test_name${NC}"
    echo "   请求: $requests, 并发: $concurrency"
    
    # 预热请求
    if [ "$method" = "POST" ]; then
        curl -s -X POST -H "Content-Type: application/json" -d "$data" "$url" > /dev/null || echo "预热失败"
    else
        curl -s "$url" > /dev/null || echo "预热失败"
    fi
    
    if [ "$method" = "POST" ]; then
        RESULT=$($HEY_BIN -n $requests -c $concurrency -m POST \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$url" 2>&1)
    else
        RESULT=$($HEY_BIN -n $requests -c $concurrency "$url" 2>&1)
    fi
    
    # 检查是否有错误
    ERROR_COUNT=$(echo "$RESULT" | grep -o "Status \[.*\]" | grep -v "Status \[200\]" | wc -l || echo "0")
    if [ "$ERROR_COUNT" -gt 0 ]; then
        echo -e "   ${RED}⚠️  发现 $ERROR_COUNT 个错误响应${NC}"
        echo "$RESULT" | grep "Status \[" | head -3
    fi
    
    # 显示性能指标
    echo "$RESULT" | grep -E "(Total:|Requests/sec:|95%|99%)" | while read line; do
        echo "   $line"
    done
    echo ""
}

# 降低负载进行逐步测试
echo -e "${GREEN}📈 Level 1: 轻量负载 (500请求, 5并发)${NC}"
echo "----------------------------------------"

run_performance_test "健康检查" 500 5 "http://localhost:8080/api/v1/health" "GET" ""

run_performance_test "计数器写入" 500 5 "http://localhost:8080/api/v1/counter/increment" "POST" '{"resource_id":"perf_test","counter_type":"like","delta":1}'

echo -e "${GREEN}📈 Level 2: 中等负载 (2k请求, 20并发)${NC}"
echo "----------------------------------------"

run_performance_test "计数器写入" 2000 20 "http://localhost:8080/api/v1/counter/increment" "POST" '{"resource_id":"perf_test","counter_type":"like","delta":1}'

echo -e "${GREEN}📈 Level 3: 高负载 (5k请求, 50并发)${NC}"
echo "----------------------------------------"

run_performance_test "计数器写入" 5000 50 "http://localhost:8080/api/v1/counter/increment" "POST" '{"resource_id":"perf_test","counter_type":"like","delta":1}'

# 数据一致性验证
echo "🔍 数据一致性验证..."
sleep 2  # 等待所有请求完成
FINAL_COUNT=$(curl -s "http://localhost:8080/api/v1/counter/perf_test/like" | jq -r '.data.current_value' 2>/dev/null || echo "0")
EXPECTED_COUNT=$((500 + 2000 + 5000))  # 7500

echo "Expected: $EXPECTED_COUNT, Actual: $FINAL_COUNT"
if [ "$FINAL_COUNT" = "$EXPECTED_COUNT" ]; then
    echo -e "${GREEN}✅ 数据一致性验证通过${NC}"
else
    DIFF=$((FINAL_COUNT - EXPECTED_COUNT))
    echo -e "${RED}⚠️  数据一致性异常，差异: $DIFF${NC}"
    if [ $DIFF -gt 0 ]; then
        echo "   可能原因: 重试机制导致重复请求"
    else
        echo "   可能原因: 请求丢失或服务错误"
    fi
fi

# 连接池详细状态
echo ""
echo "🔍 连接池详细状态..."
curl -s "http://localhost:8080/api/v1/system/grpc-pools" | jq '.' 2>/dev/null || echo "无法获取连接池状态"

echo ""
echo "🔍 检查服务日志错误..."
echo "Gateway错误:"
grep -i error logs/gateway.log | tail -3 || echo "无Gateway错误"
echo "Counter错误:"  
grep -i error logs/counter.log | tail -3 || echo "无Counter错误"

echo ""
echo "========================================="
echo -e "${GREEN}🎉 Week 4 性能测试完成！${NC}"
echo ""
echo "📊 关键问题分析:"
echo "   - 如果QPS < 1000，检查gRPC连接"
echo "   - 如果数据不一致，检查重试配置"
echo "   - 如果大量错误，检查服务健康状态"
echo ""

# 清理进程
echo "🧹 清理测试进程..."
kill $COUNTER_PID $ANALYTICS_PID $GATEWAY_PID 2>/dev/null || true
sleep 2

echo -e "${GREEN}✅ 测试完成，所有进程已清理${NC}" 