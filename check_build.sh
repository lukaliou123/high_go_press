#!/bin/bash

echo "=== Checking Build Issues ==="

# 创建必要目录
mkdir -p logs bin

echo "1. Checking Go dependencies..."
go mod tidy
go mod verify

echo -e "\n2. Testing Gateway compilation..."
if go build -v -o bin/gateway cmd/gateway/main.go 2>logs/build_error.log; then
    echo "✅ Gateway compiled successfully"
    
    echo -e "\n3. Testing Gateway startup..."
    timeout 5s ./bin/gateway >logs/gateway_startup.log 2>&1 &
    GATEWAY_PID=$!
    
    sleep 2
    
    if kill -0 $GATEWAY_PID 2>/dev/null; then
        echo "✅ Gateway started successfully"
        kill $GATEWAY_PID 2>/dev/null
        
        echo -e "\n4. Gateway startup log:"
        head -20 logs/gateway_startup.log
    else
        echo "❌ Gateway startup failed"
        echo "Startup log:"
        cat logs/gateway_startup.log
    fi
else
    echo "❌ Gateway compilation failed"
    echo "Build errors:"
    cat logs/build_error.log
fi

echo -e "\n5. Testing Counter compilation..."
go build -v -o bin/counter cmd/counter/main.go 2>logs/counter_build.log
if [ $? -eq 0 ]; then
    echo "✅ Counter compiled successfully"
else
    echo "❌ Counter compilation failed"
    cat logs/counter_build.log
fi

echo -e "\n6. Testing Analytics compilation..."
go build -v -o bin/analytics cmd/analytics/main.go 2>logs/analytics_build.log
if [ $? -eq 0 ]; then
    echo "✅ Analytics compiled successfully"
else
    echo "❌ Analytics compilation failed"
    cat logs/analytics_build.log
fi 