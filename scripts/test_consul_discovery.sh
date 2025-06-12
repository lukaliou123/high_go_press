#!/bin/bash

# Consul服务发现功能测试脚本
# Week 4 Day 9-10: 测试服务注册和发现

set -e

echo "🔍 Consul服务发现功能测试"
echo "=============================="

# 检查Consul是否运行
echo "📊 1. 检查Consul状态..."
if ! curl -s http://localhost:8500/v1/status/leader >/dev/null; then
    echo "❌ Consul未运行，请先启动Consul"
    exit 1
fi

echo "✅ Consul运行正常"

# 查看已注册的服务
echo ""
echo "📋 2. 查看已注册的服务..."
echo "服务列表:"
curl -s http://localhost:8500/v1/catalog/services | python3 -m json.tool

# 查看特定服务的健康状态
echo ""
echo "💊 3. 检查服务健康状态..."

services=("high-go-press-gateway" "high-go-press-counter" "high-go-press-analytics")

for service in "${services[@]}"; do
    echo ""
    echo "检查服务: $service"
    
    # 获取服务实例
    instances=$(curl -s "http://localhost:8500/v1/health/service/$service?passing=true")
    instance_count=$(echo "$instances" | python3 -c "import sys, json; print(len(json.load(sys.stdin)))")
    
    echo "  健康实例数: $instance_count"
    
    if [ "$instance_count" -gt 0 ]; then
        echo "  实例详情:"
        echo "$instances" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for i, instance in enumerate(data):
    service = instance['Service']
    checks = instance['Checks']
    health_status = 'healthy' if all(check['Status'] == 'passing' for check in checks) else 'unhealthy'
    print(f'    实例 {i+1}: {service[\"Address\"]}:{service[\"Port\"]} - {health_status}')
"
    else
        echo "  ⚠️  无健康实例"
    fi
done

# 测试服务发现API
echo ""
echo "🔍 4. 测试服务发现功能..."

echo "测试发现Counter服务:"
counter_instances=$(curl -s "http://localhost:8500/v1/health/service/high-go-press-counter?passing=true")
echo "$counter_instances" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    if data:
        for instance in data:
            service = instance['Service']
            print(f'  发现Counter服务: {service[\"Address\"]}:{service[\"Port\"]}')
    else:
        print('  ❌ 未发现Counter服务实例')
except:
    print('  ❌ 解析服务发现结果失败')
"

# 测试DNS发现
echo ""
echo "🌐 5. 测试DNS服务发现..."
echo "使用dig查询服务记录:"

if command -v dig >/dev/null 2>&1; then
    echo "Counter服务 SRV记录:"
    dig @localhost -p 8600 high-go-press-counter.service.consul SRV +short || echo "  ❌ SRV查询失败"
    
    echo "Counter服务 A记录:"
    dig @localhost -p 8600 high-go-press-counter.service.consul A +short || echo "  ❌ A记录查询失败"
else
    echo "  ⚠️  dig命令未找到，跳过DNS测试"
fi

# 性能测试
echo ""
echo "⚡ 6. 服务发现性能测试..."
echo "测试服务发现延迟:"

start_time=$(date +%s%N)
for i in {1..10}; do
    curl -s "http://localhost:8500/v1/health/service/high-go-press-counter?passing=true" >/dev/null
done
end_time=$(date +%s%N)

duration=$(( (end_time - start_time) / 1000000 ))
average_latency=$(( duration / 10 ))

echo "  10次服务发现调用平均延迟: ${average_latency}ms"

if [ "$average_latency" -lt 50 ]; then
    echo "  ✅ 服务发现性能良好"
elif [ "$average_latency" -lt 100 ]; then
    echo "  ⚠️  服务发现性能一般"
else
    echo "  ❌ 服务发现性能较差"
fi

# 总结
echo ""
echo "📊 测试总结"
echo "============"

total_services=0
healthy_services=0

for service in "${services[@]}"; do
    total_services=$((total_services + 1))
    instances=$(curl -s "http://localhost:8500/v1/health/service/$service?passing=true")
    instance_count=$(echo "$instances" | python3 -c "import sys, json; print(len(json.load(sys.stdin)))")
    
    if [ "$instance_count" -gt 0 ]; then
        healthy_services=$((healthy_services + 1))
    fi
done

echo "注册服务总数: $total_services"
echo "健康服务数量: $healthy_services"
echo "服务发现延迟: ${average_latency}ms"

if [ "$healthy_services" -eq "$total_services" ] && [ "$average_latency" -lt 100 ]; then
    echo ""
    echo "🎉 Consul服务发现功能测试通过!"
    echo "✅ 所有服务健康，性能良好"
    exit 0
elif [ "$healthy_services" -eq "$total_services" ]; then
    echo ""
    echo "⚠️  Consul服务发现基本正常"
    echo "✅ 所有服务健康，但性能有待优化"
    exit 0
else
    echo ""
    echo "❌ Consul服务发现测试失败"
    echo "部分服务不健康，请检查服务状态"
    exit 1
fi 