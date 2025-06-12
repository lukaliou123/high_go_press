#!/bin/bash

echo "=== Debug Gateway Compilation and Startup ==="

# 1. Check Go environment
echo "Go version:"
go version

echo -e "\n=== Checking dependencies ==="
go mod tidy
go mod verify

echo -e "\n=== Attempting to build Gateway ==="
echo "Building Gateway..."
if go build -v -o bin/gateway cmd/gateway/main.go 2>&1; then
    echo "✅ Gateway build successful"
    
    echo -e "\n=== Starting Gateway service ==="
    timeout 10s ./bin/gateway 2>&1 || echo "Gateway startup timeout or error"
else
    echo "❌ Gateway build failed"
fi

echo -e "\n=== Checking for log files ==="
ls -la logs/ 2>/dev/null || echo "No logs directory found" 