#!/bin/bash

echo "=== Starting HighGoPress Microservices ==="

# 创建必要的目录
mkdir -p logs bin

# 函数：启动服务并检查
start_service() {
    local service_name=$1
    local port=$2
    local log_file="logs/${service_name}.log"
    
    echo "Starting ${service_name}..."
    
    # 编译服务
    if go build -o "bin/${service_name}" "cmd/${service_name}/main.go"; then
        echo "✅ ${service_name} compiled successfully"
        
        # 启动服务
        nohup "./bin/${service_name}" > "${log_file}" 2>&1 &
        local pid=$!
        
        # 等待服务启动
        sleep 3
        
        # 检查进程是否还在运行
        if kill -0 $pid 2>/dev/null; then
            echo "✅ ${service_name} started successfully (PID: $pid)"
            echo $pid > "logs/${service_name}.pid"
            return 0
        else
            echo "❌ ${service_name} failed to start"
            echo "Log output:"
            tail -10 "${log_file}"
            return 1
        fi
    else
        echo "❌ ${service_name} compilation failed"
        return 1
    fi
}

# 停止所有服务
stop_services() {
    echo "Stopping all services..."
    pkill -f "bin/counter" 2>/dev/null
    pkill -f "bin/analytics" 2>/dev/null
    pkill -f "bin/gateway" 2>/dev/null
    rm -f logs/*.pid
    echo "All services stopped"
}

# 捕获Ctrl+C信号
trap stop_services INT

# 启动Counter服务 (端口9001)
start_service "counter" 9001

# 启动Analytics服务 (端口9002)  
start_service "analytics" 9002

# 启动Gateway服务 (端口8080)
start_service "gateway" 8080

if [ $? -eq 0 ]; then
    echo ""
    echo "🎉 All services started successfully!"
    echo ""
    echo "Services:"
    echo "  - Counter:   http://localhost:9001"
    echo "  - Analytics: http://localhost:9002" 
    echo "  - Gateway:   http://localhost:8080"
    echo ""
    echo "Testing endpoints:"
    echo "  - Health: curl http://localhost:8080/api/v1/health"
    echo "  - Pool:   curl http://localhost:8080/api/v1/system/grpc-pools"
    echo ""
    echo "Press Ctrl+C to stop all services"
    
    # 保持运行直到用户按Ctrl+C
    wait
else
    echo "❌ Failed to start some services"
    stop_services
    exit 1
fi 