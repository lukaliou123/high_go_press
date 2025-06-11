#!/bin/bash

set -e

echo "🚀 Starting HighGoPress Safe Integration Test (Day 5-6)"
echo "========================================================="

# 更安全的进程清理 - 只清理我们明确知道的进程
echo "📋 Step 1: Safely cleaning up previous processes..."

# 查找并杀死我们的特定进程（避免使用"main"关键词）
pgrep -f "counter-v2" | xargs -r kill 2>/dev/null || true
pgrep -f "analytics-v2" | xargs -r kill 2>/dev/null || true
pgrep -f "cmd/gateway/main.go" | xargs -r kill 2>/dev/null || true

sleep 3

# 检查端口占用（不强制杀死）
for port in 8080 9001 9002; do
    if ss -tln | grep ":$port " >/dev/null 2>&1; then
        echo "⚠️  Port $port is still occupied. Please manually stop the process:"
        ss -tlnp | grep ":$port "
        echo "If safe to do so, run: fuser -k $port/tcp"
        exit 1
    fi
done

echo "✅ All ports available for testing"

# 创建日志目录
mkdir -p logs

# 启动Counter服务
echo "🔄 Step 2: Starting Counter microservice..."
./bin/counter-v2 > logs/counter.log 2>&1 &
COUNTER_PID=$!
echo "   Counter started with PID: $COUNTER_PID"

# 等待并验证Counter启动
sleep 3
if ! pgrep -f "counter-v2" >/dev/null; then
    echo "❌ Counter service failed to start"
    echo "Counter logs:"
    cat logs/counter.log
    exit 1
fi

# 启动Analytics服务
echo "📊 Step 3: Starting Analytics microservice..."
./bin/analytics-v2 > logs/analytics.log 2>&1 &
ANALYTICS_PID=$!
echo "   Analytics started with PID: $ANALYTICS_PID"

# 等待并验证Analytics启动
sleep 3
if ! pgrep -f "analytics-v2" >/dev/null; then
    echo "❌ Analytics service failed to start"
    echo "Analytics logs:"
    cat logs/analytics.log
    exit 1
fi

# 启动Gateway服务
echo "🌐 Step 4: Starting Gateway service..."
go run cmd/gateway/main.go > logs/gateway.log 2>&1 &
GATEWAY_PID=$!
echo "   Gateway started with PID: $GATEWAY_PID"

# 等待并验证Gateway启动
echo "   Waiting for Gateway to be ready..."
for i in {1..15}; do
    if curl -s http://localhost:8080/api/v1/health >/dev/null 2>&1; then
        echo "   ✅ Gateway is ready"
        break
    fi
    if [ $i -eq 15 ]; then
        echo "   ❌ Gateway health check timeout"
        echo "Gateway logs:"
        cat logs/gateway.log
        echo ""
        echo "Cleaning up processes..."
        kill $COUNTER_PID $ANALYTICS_PID $GATEWAY_PID 2>/dev/null || true
        exit 1
    fi
    sleep 1
done

# 验证所有服务状态
echo "✅ Step 5: Verifying services are running..."

services_ok=true

if pgrep -f "counter-v2" >/dev/null && ss -tln | grep ":9001 " >/dev/null 2>&1; then
    echo "   ✅ Counter service (port 9001): OK"
else
    echo "   ❌ Counter service (port 9001): NOT RUNNING"
    services_ok=false
fi

if pgrep -f "analytics-v2" >/dev/null && ss -tln | grep ":9002 " >/dev/null 2>&1; then
    echo "   ✅ Analytics service (port 9002): OK"
else
    echo "   ❌ Analytics service (port 9002): NOT RUNNING"
    services_ok=false
fi

if pgrep -f "cmd/gateway/main.go" >/dev/null && ss -tln | grep ":8080 " >/dev/null 2>&1; then
    echo "   ✅ Gateway service (port 8080): OK"
else
    echo "   ❌ Gateway service (port 8080): NOT RUNNING"
    services_ok=false
fi

if [ "$services_ok" = false ]; then
    echo "❌ Some services failed to start properly"
    echo "Cleaning up..."
    kill $COUNTER_PID $ANALYTICS_PID $GATEWAY_PID 2>/dev/null || true
    exit 1
fi

echo ""
echo "🧪 Step 6: Running End-to-End Tests..."
echo "======================================"

# 测试1: 健康检查
echo "Test 1: Health Check"
response=$(curl -s http://localhost:8080/api/v1/health)
if echo "$response" | grep -q '"status":"healthy"'; then
    echo "   ✅ Gateway health check: PASSED"
    echo "   Response: $response"
else
    echo "   ❌ Gateway health check: FAILED"
    echo "   Response: $response"
fi

# 测试2: Counter服务增量操作
echo "Test 2: Counter Increment"
response=$(curl -s -X POST http://localhost:8080/api/v1/counter/increment \
    -H "Content-Type: application/json" \
    -d '{"resource_id":"test_article_001","counter_type":"like","delta":5}')

if echo "$response" | grep -q '"success":true'; then
    echo "   ✅ Counter increment: PASSED"
    echo "   Response: $response"
else
    echo "   ❌ Counter increment: FAILED"
    echo "   Response: $response"
fi

# 等待数据同步
echo "   Waiting for data synchronization..."
sleep 3

# 测试3: Counter服务查询操作 - 关键测试！
echo "Test 3: Counter Get (Critical Data Consistency Test)"
response=$(curl -s "http://localhost:8080/api/v1/counter/test_article_001/like")

if echo "$response" | grep -q '"status":"success"'; then
    current_value=$(echo "$response" | grep -o '"current_value":[0-9]*' | cut -d: -f2)
    if [ "$current_value" -ge 5 ]; then
        echo "   🎉 Counter get: PASSED (value: $current_value) - DATA CONSISTENCY OK!"
    else
        echo "   ⚠️  Counter get: DATA INCONSISTENCY (expected >=5, got: $current_value)"
        echo "   This indicates Redis integration issues"
    fi
    echo "   Response: $response"
else
    echo "   ❌ Counter get: FAILED"
    echo "   Response: $response"
fi

# 测试4: 批量操作
echo "Test 4: Batch Operations"
response=$(curl -s -X POST http://localhost:8080/api/v1/counter/batch \
    -H "Content-Type: application/json" \
    -d '{"Items":[{"resource_id":"article_1","counter_type":"view"},{"resource_id":"article_2","counter_type":"like"}]}')

if echo "$response" | grep -q '"status":"success"'; then
    echo "   ✅ Batch operations: PASSED"
    echo "   Response: $response"
else
    echo "   ❌ Batch operations: FAILED"
    echo "   Response: $response"
fi

echo ""
echo "🔍 Step 7: Service Status Summary"
echo "================================"

echo "Running processes:"
echo "Counter PID: $COUNTER_PID - $(pgrep -f "counter-v2" | wc -l) instance(s)"
echo "Analytics PID: $ANALYTICS_PID - $(pgrep -f "analytics-v2" | wc -l) instance(s)"
echo "Gateway PID: $GATEWAY_PID - $(pgrep -f "cmd/gateway/main.go" | wc -l) instance(s)"

echo ""
echo "Port bindings:"
ss -tlnp | grep -E ":(8080|9001|9002) " 2>/dev/null || echo "No port bindings found"

echo ""
echo "📊 Safe Integration test completed!"
echo "Logs available in:"
echo "  - logs/counter.log"
echo "  - logs/analytics.log"  
echo "  - logs/gateway.log"

echo ""
echo "🧹 To stop services safely:"
echo "  kill $COUNTER_PID $ANALYTICS_PID $GATEWAY_PID"
echo "  or run: pkill -f 'counter-v2'; pkill -f 'analytics-v2'; pkill -f 'cmd/gateway/main.go'" 