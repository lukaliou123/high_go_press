#!/bin/bash

echo "🔍 Week 4 性能问题完整诊断"
echo "========================="

# 1. 环境检查
echo "1. 环境检查..."
echo "Go版本: $(go version)"
echo "当前目录: $(pwd)"
echo "可用内存: $(free -h | grep Mem | awk '{print $7}')"

# 2. 编译测试
echo -e "\n2. 编译测试..."
mkdir -p bin logs

echo "编译Counter..."
if go build -o bin/counter cmd/counter/main.go 2>logs/counter_build.log; then
    echo "✅ Counter编译成功"
else
    echo "❌ Counter编译失败:"
    cat logs/counter_build.log
fi

echo "编译Gateway..."
if go build -o bin/gateway cmd/gateway/main.go 2>logs/gateway_build.log; then
    echo "✅ Gateway编译成功"
else
    echo "❌ Gateway编译失败:"
    cat logs/gateway_build.log
fi

# 3. 服务启动测试
echo -e "\n3. 服务启动测试..."

# 清理旧进程
pkill -f 'bin/counter' 2>/dev/null || true
pkill -f 'bin/gateway' 2>/dev/null || true
sleep 2

# 启动Counter
echo "启动Counter服务..."
./bin/counter > logs/counter.log 2>&1 &
COUNTER_PID=$!
echo "Counter PID: $COUNTER_PID"

sleep 3

# 检查Counter是否运行
if kill -0 $COUNTER_PID 2>/dev/null; then
    echo "✅ Counter服务运行中"
    
    # 测试Counter端口
    if nc -z localhost 9001 2>/dev/null; then
        echo "✅ Counter端口9001可达"
    else
        echo "❌ Counter端口9001不可达"
    fi
else
    echo "❌ Counter服务启动失败"
    echo "Counter日志:"
    cat logs/counter.log
fi

# 启动Gateway
echo "启动Gateway服务..."
./bin/gateway > logs/gateway.log 2>&1 &
GATEWAY_PID=$!
echo "Gateway PID: $GATEWAY_PID"

sleep 5

# 检查Gateway是否运行
if kill -0 $GATEWAY_PID 2>/dev/null; then
    echo "✅ Gateway服务运行中"
    
    # 测试Gateway端口
    if nc -z localhost 8080 2>/dev/null; then
        echo "✅ Gateway端口8080可达"
    else
        echo "❌ Gateway端口8080不可达"
    fi
else
    echo "❌ Gateway服务启动失败"
    echo "Gateway日志:"
    cat logs/gateway.log
fi

# 4. API测试
echo -e "\n4. API测试..."

# 健康检查
echo "测试健康检查API..."
HEALTH_RESP=$(curl -s http://localhost:8080/api/v1/health 2>/dev/null || echo "failed")
if [[ "$HEALTH_RESP" == *"success"* ]]; then
    echo "✅ 健康检查API正常"
else
    echo "❌ 健康检查API失败: $HEALTH_RESP"
fi

# 连接池状态
echo "测试连接池状态API..."
POOL_RESP=$(curl -s http://localhost:8080/api/v1/system/grpc-pools 2>/dev/null || echo "failed")
if [[ "$POOL_RESP" == *"pool_size"* ]]; then
    echo "✅ 连接池状态API正常"
    echo "连接池信息: $POOL_RESP"
else
    echo "❌ 连接池状态API失败: $POOL_RESP"
fi

# 计数器写入测试
echo "测试计数器写入API..."
WRITE_RESP=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"resource_id":"test_diagnose","counter_type":"like","delta":1}' \
    http://localhost:8080/api/v1/counter/increment 2>/dev/null || echo "failed")

if [[ "$WRITE_RESP" == *"success"* ]]; then
    echo "✅ 计数器写入API正常"
    echo "写入响应: $WRITE_RESP"
else
    echo "❌ 计数器写入API失败: $WRITE_RESP"
fi

# 计数器读取测试
echo "测试计数器读取API..."
READ_RESP=$(curl -s http://localhost:8080/api/v1/counter/test_diagnose/like 2>/dev/null || echo "failed")
if [[ "$READ_RESP" == *"success"* ]]; then
    echo "✅ 计数器读取API正常"
    echo "读取响应: $READ_RESP"
else
    echo "❌ 计数器读取API失败: $READ_RESP"
fi

# 5. 性能快速测试
echo -e "\n5. 性能快速测试..."

# 检查hey工具
HEY_BIN=$(go env GOPATH)/bin/hey
if [ ! -f "$HEY_BIN" ]; then
    echo "安装hey工具..."
    go install github.com/rakyll/hey@latest
fi

if [ -f "$HEY_BIN" ]; then
    echo "执行快速性能测试 (100请求, 5并发)..."
    
    # 测试写入性能
    PERF_RESULT=$($HEY_BIN -n 100 -c 5 -m POST \
        -H "Content-Type: application/json" \
        -d '{"resource_id":"perf_diagnose","counter_type":"like","delta":1}' \
        http://localhost:8080/api/v1/counter/increment 2>&1)
    
    QPS=$(echo "$PERF_RESULT" | grep "Requests/sec" | awk '{print $2}')
    ERRORS=$(echo "$PERF_RESULT" | grep -o "Status \[.*\]" | grep -v "Status \[200\]" | wc -l)
    
    echo "写入QPS: $QPS"
    echo "错误数量: $ERRORS"
    
    if (( $(echo "$QPS > 100" | bc -l) )); then
        echo "✅ 基础性能正常"
    else
        echo "❌ 基础性能异常"
    fi
else
    echo "hey工具不可用，跳过性能测试"
fi

# 6. 日志分析
echo -e "\n6. 日志分析..."

echo "Gateway错误日志:"
grep -i -E "(error|ERROR|panic|PANIC|fatal|FATAL)" logs/gateway.log | tail -5 || echo "无错误"

echo "Counter错误日志:"
grep -i -E "(error|ERROR|panic|PANIC|fatal|FATAL)" logs/counter.log | tail -5 || echo "无错误"

# 7. 资源使用
echo -e "\n7. 资源使用..."
if kill -0 $GATEWAY_PID 2>/dev/null; then
    GATEWAY_CPU=$(ps -p $GATEWAY_PID -o %cpu --no-headers 2>/dev/null || echo "N/A")
    GATEWAY_MEM=$(ps -p $GATEWAY_PID -o %mem --no-headers 2>/dev/null || echo "N/A")
    echo "Gateway CPU: ${GATEWAY_CPU}%, MEM: ${GATEWAY_MEM}%"
fi

if kill -0 $COUNTER_PID 2>/dev/null; then
    COUNTER_CPU=$(ps -p $COUNTER_PID -o %cpu --no-headers 2>/dev/null || echo "N/A")
    COUNTER_MEM=$(ps -p $COUNTER_PID -o %mem --no-headers 2>/dev/null || echo "N/A")
    echo "Counter CPU: ${COUNTER_CPU}%, MEM: ${COUNTER_MEM}%"
fi

# 8. 总结建议
echo -e "\n8. 总结建议..."
echo "================================="

if [[ "$HEALTH_RESP" == *"success"* ]] && [[ "$QPS" != "" ]] && (( $(echo "$QPS > 50" | bc -l) )); then
    echo "🎯 基础功能正常，可以进行深度性能测试"
    echo "建议运行: chmod +x scripts/week4_performance_test_fixed.sh && ./scripts/week4_performance_test_fixed.sh"
else
    echo "⚠️  发现问题，需要修复后再进行性能测试:"
    
    if [[ "$HEALTH_RESP" != *"success"* ]]; then
        echo "  - Gateway服务或健康检查API异常"
    fi
    
    if [[ "$POOL_RESP" != *"pool_size"* ]]; then
        echo "  - 连接池状态API异常"
    fi
    
    if [[ "$WRITE_RESP" != *"success"* ]]; then
        echo "  - 计数器写入API异常"
    fi
    
    if [[ "$QPS" == "" ]] || (( $(echo "$QPS < 50" | bc -l) )); then
        echo "  - 基础性能过低"
    fi
fi

# 清理进程
echo -e "\n🧹 清理测试进程..."
kill $COUNTER_PID $GATEWAY_PID 2>/dev/null || true
sleep 2

echo "✅ 诊断完成" 