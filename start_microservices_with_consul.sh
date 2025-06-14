#!/bin/bash

# HighGoPress 微服务启动脚本 (修复Consul服务注册)
# 解决服务注册地址问题

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo "🚀 HighGoPress 微服务启动 (修复版)"
echo "=================================="

# 等待服务启动
wait_for_service() {
    local host=$1
    local port=$2
    local service=$3
    local max_attempts=30
    local attempt=1
    
    log_info "等待 $service 服务启动..."
    
    while [[ $attempt -le $max_attempts ]]; do
        if nc -z $host $port 2>/dev/null; then
            log_success "$service 服务已启动"
            return 0
        fi
        
        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done
    
    log_error "$service 服务启动超时"
    return 1
}

# 1. 检查基础设施
log_info "检查基础设施服务..."

# 检查Consul
if ! curl -s http://localhost:8500/v1/status/leader > /dev/null; then
    log_error "Consul未运行，请先启动: ./start_complete_monitoring.sh"
    exit 1
fi
log_success "Consul正常运行"

# 检查Redis
if ! nc -z localhost 6379; then
    log_error "Redis未运行，请先启动Redis"
    exit 1
fi
log_success "Redis正常运行"

# 检查Kafka
if ! nc -z localhost 9092; then
    log_error "Kafka未运行，请先启动Kafka"
    exit 1
fi
log_success "Kafka正常运行"

# 2. 停止现有服务
log_info "停止现有微服务..."
pkill -f "go run cmd/counter/main.go" 2>/dev/null || true
pkill -f "go run cmd/analytics/main.go" 2>/dev/null || true
pkill -f "go run cmd/gateway/main.go" 2>/dev/null || true
sleep 3

# 3. 设置环境变量
export KAFKA_MODE=real
export KAFKA_BROKERS=localhost:9092

# 4. 启动Counter服务
log_info "启动Counter服务 (带Consul注册)..."
go run cmd/counter/main.go > cmd/counter/counter.log 2>&1 &
COUNTER_PID=$!

wait_for_service localhost 9001 "Counter"

# 5. 启动Analytics服务
log_info "启动Analytics服务 (带Consul注册)..."
go run cmd/analytics/main.go > cmd/analytics/analytics.log 2>&1 &
ANALYTICS_PID=$!

wait_for_service localhost 9002 "Analytics"

# 6. 等待服务在Consul中注册
log_info "等待服务在Consul中注册..."
sleep 5

# 7. 验证Consul服务注册
log_info "验证Consul服务注册..."

# 检查Counter服务注册
COUNTER_ADDRESS=$(curl -s http://localhost:8500/v1/health/service/high-go-press-counter?passing=true | jq -r '.[0].Service.Address // "empty"')
if [[ "$COUNTER_ADDRESS" == "localhost" ]]; then
    log_success "✅ Counter服务注册成功 (地址: $COUNTER_ADDRESS)"
else
    log_error "❌ Counter服务注册失败 (地址: $COUNTER_ADDRESS)"
fi

# 检查Analytics服务注册
ANALYTICS_ADDRESS=$(curl -s http://localhost:8500/v1/health/service/high-go-press-analytics?passing=true | jq -r '.[0].Service.Address // "empty"')
if [[ "$ANALYTICS_ADDRESS" == "localhost" ]]; then
    log_success "✅ Analytics服务注册成功 (地址: $ANALYTICS_ADDRESS)"
else
    log_error "❌ Analytics服务注册失败 (地址: $ANALYTICS_ADDRESS)"
fi

# 8. 启动Gateway服务
log_info "启动Gateway服务 (带服务发现)..."
go run cmd/gateway/main.go > cmd/gateway/gateway.log 2>&1 &
GATEWAY_PID=$!

wait_for_service localhost 8080 "Gateway"

# 9. 验证完整系统
log_info "验证完整系统..."

# 测试Gateway健康检查
if curl -s http://localhost:8080/api/v1/health | grep -q "success"; then
    log_success "✅ Gateway健康检查通过"
else
    log_error "❌ Gateway健康检查失败"
fi

# 测试Counter增量API
log_info "测试Counter增量API..."
COUNTER_RESULT=$(curl -s -X POST http://localhost:8080/api/v1/counter/increment \
    -H "Content-Type: application/json" \
    -d '{"resource_id":"test","counter_type":"view","delta":1}')

if echo "$COUNTER_RESULT" | grep -q "success"; then
    log_success "✅ Counter增量API测试通过"
else
    log_error "❌ Counter增量API测试失败"
    echo "响应: $COUNTER_RESULT"
fi

# 10. 显示服务状态
echo ""
log_info "🎉 微服务系统启动完成！"
echo ""
log_info "服务访问地址:"
echo "  🌐 Gateway API:    http://localhost:8080"
echo "  🔍 Consul UI:      http://localhost:8500"
echo "  📊 Prometheus:     http://localhost:9090"
echo "  📈 Grafana:        http://localhost:3000"
echo ""
log_info "服务进程ID:"
echo "  Counter:   $COUNTER_PID"
echo "  Analytics: $ANALYTICS_PID"
echo "  Gateway:   $GATEWAY_PID"
echo ""
log_info "日志文件:"
echo "  Counter:   cmd/counter/counter.log"
echo "  Analytics: cmd/analytics/analytics.log"
echo "  Gateway:   cmd/gateway/gateway.log"
echo ""
log_info "测试命令:"
echo "  增量计数: curl -X POST http://localhost:8080/api/v1/counter/increment -H 'Content-Type: application/json' -d '{\"resource_id\":\"test\",\"counter_type\":\"view\",\"delta\":1}'"
echo "  查询计数: curl http://localhost:8080/api/v1/counter/test/view"
echo "  系统状态: curl http://localhost:8080/api/v1/system/status"
echo ""
log_success "🎯 现在可以运行负载测试了！" 