#!/bin/bash

# HighGoPress 服务停止脚本

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

echo "🛑 停止 HighGoPress 所有服务"
echo "=========================="

# 停止微服务
if [ -f logs/gateway.pid ]; then
    log_info "停止 Gateway 服务..."
    kill $(cat logs/gateway.pid) 2>/dev/null || true
    rm -f logs/gateway.pid
fi

if [ -f logs/counter.pid ]; then
    log_info "停止 Counter 服务..."
    kill $(cat logs/counter.pid) 2>/dev/null || true
    rm -f logs/counter.pid
fi

if [ -f logs/analytics.pid ]; then
    log_info "停止 Analytics 服务..."
    kill $(cat logs/analytics.pid) 2>/dev/null || true
    rm -f logs/analytics.pid
fi

# 停止Docker容器
log_info "停止 Kafka 和 Zookeeper..."
docker-compose -f /tmp/kafka-compose.yml down 2>/dev/null || true

log_info "停止 Redis..."
docker stop highgopress-redis-standalone 2>/dev/null || true
docker rm highgopress-redis-standalone 2>/dev/null || true

# 清理临时文件
rm -f /tmp/kafka-compose.yml

log_success "✅ 所有服务已停止" 