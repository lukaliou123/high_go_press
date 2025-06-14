#!/bin/bash

echo "🔧 修复Consul服务注册地址"
echo "========================"

# 注销现有的服务
echo "1. 注销现有服务..."
curl -X PUT http://localhost:8500/v1/agent/service/deregister/counter-1
curl -X PUT http://localhost:8500/v1/agent/service/deregister/analytics-1
curl -X PUT http://localhost:8500/v1/agent/service/deregister/gateway-1

sleep 2

# 重新注册Counter服务（带正确地址）
echo "2. 重新注册Counter服务..."
curl -X PUT http://localhost:8500/v1/agent/service/register \
  -H "Content-Type: application/json" \
  -d '{
    "ID": "counter-1",
    "Name": "high-go-press-counter",
    "Tags": ["counter", "grpc", "microservice", "v2.0"],
    "Address": "localhost",
    "Port": 9001,
    "Check": {
      "Name": "Counter TCP Health Check",
      "TCP": "localhost:9001",
      "Interval": "10s",
      "Timeout": "3s"
    }
  }'

# 重新注册Analytics服务（带正确地址）
echo "3. 重新注册Analytics服务..."
curl -X PUT http://localhost:8500/v1/agent/service/register \
  -H "Content-Type: application/json" \
  -d '{
    "ID": "analytics-1",
    "Name": "high-go-press-analytics",
    "Tags": ["analytics", "grpc", "microservice", "v2.0"],
    "Address": "localhost",
    "Port": 9002,
    "Check": {
      "Name": "Analytics TCP Health Check",
      "TCP": "localhost:9002",
      "Interval": "10s",
      "Timeout": "3s"
    }
  }'

# 重新注册Gateway服务（带正确地址）
echo "4. 重新注册Gateway服务..."
curl -X PUT http://localhost:8500/v1/agent/service/register \
  -H "Content-Type: application/json" \
  -d '{
    "ID": "gateway-1",
    "Name": "high-go-press-gateway",
    "Tags": ["gateway", "http", "api", "v2.0"],
    "Address": "localhost",
    "Port": 8080,
    "Check": {
      "Name": "Gateway Health Check",
      "HTTP": "http://localhost:8080/api/v1/health",
      "Interval": "10s",
      "Timeout": "3s"
    }
  }'

sleep 3

# 验证注册结果
echo "5. 验证服务注册..."
echo "Counter服务地址:"
curl -s http://localhost:8500/v1/health/service/high-go-press-counter?passing=true | jq '.[].Service | {Address: .Address, Port: .Port}'

echo "Analytics服务地址:"
curl -s http://localhost:8500/v1/health/service/high-go-press-analytics?passing=true | jq '.[].Service | {Address: .Address, Port: .Port}'

echo "Gateway服务地址:"
curl -s http://localhost:8500/v1/health/service/high-go-press-gateway?passing=true | jq '.[].Service | {Address: .Address, Port: .Port}'

echo ""
echo "✅ Consul服务注册修复完成！" 