#!/bin/bash

echo "=== Testing Gateway Build and Start ==="

# 创建logs目录
mkdir -p logs

# 尝试编译
echo "Building Gateway..."
go build -o bin/gateway cmd/gateway/main.go

if [ $? -eq 0 ]; then
    echo "✅ Gateway build successful"
    
    # 尝试启动（5秒后超时）
    echo "Starting Gateway (timeout 5s)..."
    timeout 5s ./bin/gateway > logs/gateway_test.log 2>&1 &
    GATEWAY_PID=$!
    
    # 等待一下然后检查进程
    sleep 2
    
    if kill -0 $GATEWAY_PID 2>/dev/null; then
        echo "✅ Gateway started successfully"
        
        # 测试HTTP endpoint
        echo "Testing health endpoint..."
        curl -s http://localhost:8080/api/v1/health || echo "❌ Health endpoint failed"
        
        # 测试连接池状态
        echo "Testing gRPC pool status..."
        curl -s http://localhost:8080/api/v1/system/grpc-pools || echo "❌ Pool status failed"
        
        # 停止进程
        kill $GATEWAY_PID 2>/dev/null
        echo "Gateway stopped"
    else
        echo "❌ Gateway failed to start"
        echo "Check logs:"
        cat logs/gateway_test.log
    fi
else
    echo "❌ Gateway build failed"
fi 