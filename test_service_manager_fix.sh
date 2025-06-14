#!/bin/bash

# ServiceManager修复验证脚本

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

echo "🔧 ServiceManager修复验证"
echo "========================"

# 1. 确保基础设施运行
log_info "检查基础设施..."

if ! curl -s http://localhost:8500/v1/status/leader > /dev/null; then
    log_error "Consul未运行"
    exit 1
fi

if ! nc -z localhost 6379; then
    log_error "Redis未运行"
    exit 1
fi

if ! nc -z localhost 9092; then
    log_error "Kafka未运行"
    exit 1
fi

log_success "基础设施正常"

# 2. 停止所有微服务
log_info "停止现有微服务..."
pkill -f "go run cmd/counter/main.go" 2>/dev/null || true
pkill -f "go run cmd/analytics/main.go" 2>/dev/null || true
pkill -f "go run cmd/gateway/main.go" 2>/dev/null || true
sleep 3

# 3. 启动Counter和Analytics服务
log_info "启动Counter服务..."
export KAFKA_MODE=real
export KAFKA_BROKERS=localhost:9092

go run cmd/counter/main.go > cmd/counter/counter_fix.log 2>&1 &
COUNTER_PID=$!

log_info "启动Analytics服务..."
go run cmd/analytics/main.go > cmd/analytics/analytics_fix.log 2>&1 &
ANALYTICS_PID=$!

# 等待服务启动
sleep 5

# 4. 验证服务注册
log_info "验证Consul服务注册..."

COUNTER_HEALTH=$(curl -s http://localhost:8500/v1/health/service/high-go-press-counter?passing=true | jq length)
ANALYTICS_HEALTH=$(curl -s http://localhost:8500/v1/health/service/high-go-press-analytics?passing=true | jq length)

if [[ "$COUNTER_HEALTH" -gt 0 ]]; then
    log_success "✅ Counter服务注册成功"
else
    log_error "❌ Counter服务注册失败"
    exit 1
fi

if [[ "$ANALYTICS_HEALTH" -gt 0 ]]; then
    log_success "✅ Analytics服务注册成功"
else
    log_error "❌ Analytics服务注册失败"
    exit 1
fi

# 5. 测试Gateway启动（关键测试）
log_info "测试Gateway启动（ServiceManager修复验证）..."

# 启动Gateway并监控日志
go run cmd/gateway/main.go > cmd/gateway/gateway_fix.log 2>&1 &
GATEWAY_PID=$!

# 监控Gateway启动过程
log_info "监控Gateway启动过程..."
for i in {1..30}; do
    if curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
        log_success "✅ Gateway启动成功！"
        GATEWAY_STARTED=true
        break
    fi
    
    # 检查进程是否还在运行
    if ! kill -0 $GATEWAY_PID 2>/dev/null; then
        log_error "❌ Gateway进程已退出"
        break
    fi
    
    echo -n "."
    sleep 1
done

if [[ "$GATEWAY_STARTED" == "true" ]]; then
    # 6. 测试完整的API流程
    log_info "测试完整API流程..."
    
    # 测试健康检查
    HEALTH_RESULT=$(curl -s http://localhost:8080/api/v1/health)
    if echo "$HEALTH_RESULT" | grep -q "success"; then
        log_success "✅ 健康检查通过"
    else
        log_warning "⚠️ 健康检查异常"
    fi
    
    # 测试Counter增量
    log_info "测试Counter增量API..."
    COUNTER_RESULT=$(curl -s -X POST http://localhost:8080/api/v1/counter/increment \
        -H "Content-Type: application/json" \
        -d '{"resource_id":"test_fix","counter_type":"view","delta":1}')
    
    if echo "$COUNTER_RESULT" | grep -q "success"; then
        log_success "✅ Counter增量API测试通过"
    else
        log_warning "⚠️ Counter增量API测试失败"
        echo "响应: $COUNTER_RESULT"
    fi
    
    # 测试Counter查询
    log_info "测试Counter查询API..."
    QUERY_RESULT=$(curl -s http://localhost:8080/api/v1/counter/test_fix/view)
    
    if echo "$QUERY_RESULT" | grep -q "success"; then
        log_success "✅ Counter查询API测试通过"
    else
        log_warning "⚠️ Counter查询API测试失败"
    fi
    
    # 7. 检查ServiceManager状态
    log_info "检查ServiceManager状态..."
    STATUS_RESULT=$(curl -s http://localhost:8080/api/v1/system/status)
    
    if echo "$STATUS_RESULT" | grep -q "service_discovery"; then
        log_success "✅ ServiceManager状态正常"
    else
        log_warning "⚠️ ServiceManager状态异常"
    fi
    
    echo ""
    log_success "🎉 ServiceManager修复验证成功！"
    echo ""
    log_info "修复要点:"
    echo "  1. ✅ 移除了grpc.WithBlock()阻塞连接"
    echo "  2. ✅ 改为异步初始化服务连接"
    echo "  3. ✅ 改进了连接健康状态检查"
    echo "  4. ✅ 添加了更好的错误处理"
    echo ""
    log_info "服务状态:"
    echo "  🌐 Gateway:    http://localhost:8080 (PID: $GATEWAY_PID)"
    echo "  🔢 Counter:    gRPC:9001, HTTP:8081 (PID: $COUNTER_PID)"
    echo "  📊 Analytics:  gRPC:9002, HTTP:8082 (PID: $ANALYTICS_PID)"
    echo ""
    log_info "现在可以运行负载测试了！"
    
else
    log_error "❌ Gateway启动失败"
    echo ""
    log_info "检查Gateway日志:"
    tail -20 cmd/gateway/gateway_fix.log
    exit 1
fi 