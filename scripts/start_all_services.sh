#!/bin/bash

# HighGoPress 完整服务启动脚本
# 启动所有必要的服务：Kafka、Redis、Gateway、Counter、Analytics

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

# 检查端口是否被占用
check_port() {
    local port=$1
    local service=$2
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        log_warning "$service 端口 $port 已被占用"
        return 1
    fi
    return 0
}

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
        
        if [[ $attempt -eq $max_attempts ]]; then
            log_error "$service 服务启动超时"
            return 1
        fi
        
        sleep 2
        ((attempt++))
    done
}

# 启动Kafka和Zookeeper
start_kafka() {
    log_info "启动 Kafka 和 Zookeeper..."
    
    # 创建临时的docker-compose文件用于Kafka
    cat > /tmp/kafka-compose.yml << 'EOF'
version: '3.8'
services:
  zookeeper:
    image: confluentinc/cp-zookeeper:7.4.0
    container_name: highgopress-zookeeper
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    ports:
      - "2181:2181"
    networks:
      - highgopress

  kafka:
    image: confluentinc/cp-kafka:7.4.0
    container_name: highgopress-kafka
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: true
    networks:
      - highgopress

networks:
  highgopress:
    driver: bridge
EOF

    # 启动Kafka
    docker-compose -f /tmp/kafka-compose.yml up -d
    
    # 等待Kafka启动
    wait_for_service localhost 9092 "Kafka"
    log_info "Kafka port is open. Giving it extra time to initialize..."
    sleep 10
}

# 启动Redis (如果没有运行)
start_redis() {
    if ! docker ps | grep -q highgopress-redis; then
        log_info "启动 Redis..."
        docker run -d --name highgopress-redis-standalone \
            -p 6379:6379 \
            --network highgopress \
            redis:7.2-alpine redis-server --requirepass ""
        
        wait_for_service localhost 6379 "Redis"
    else
        log_success "Redis 已在运行"
    fi
}

# 修复Go依赖
fix_dependencies() {
    log_info "修复 Go 依赖..."
    go mod tidy
    go mod download
    log_success "依赖修复完成"
}

# 启动微服务
start_microservices() {
    log_info "启动微服务..."
    
    # 创建日志目录
    mkdir -p logs
    
    # 启动Counter服务
    log_info "启动 Counter 服务..."
    nohup go run cmd/counter/main.go > logs/counter.log 2>&1 &
    echo $! > logs/counter.pid

    # 等待Counter服务启动 (假设它在9001端口)
    wait_for_service localhost 9001 "Counter"

    # 启动Analytics服务
    log_info "启动 Analytics 服务..."
    nohup go run cmd/analytics/main.go > logs/analytics.log 2>&1 &
    echo $! > logs/analytics.pid

    # 等待Analytics服务启动 (假设它在9002端口)
    wait_for_service localhost 9002 "Analytics"

    # 启动Gateway服务
    log_info "启动 Gateway 服务..."
    nohup go run cmd/gateway/main.go > logs/gateway.log 2>&1 &
    echo $! > logs/gateway.pid
    
    # 等待Gateway启动
    wait_for_service localhost 8080 "Gateway"
}

# 验证服务状态
verify_services() {
    log_info "验证服务状态..."
    
    # 检查Gateway健康状态
    if curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
        log_success "✅ Gateway 服务正常"
    else
        log_error "❌ Gateway 服务异常"
    fi
    
    # 检查Consul服务注册
    if curl -s http://localhost:8500/v1/agent/services > /dev/null 2>&1; then
        log_success "✅ Consul 服务发现正常"
    else
        log_warning "⚠️ Consul 服务发现可能有问题"
    fi
    
    # 显示服务状态
    echo ""
    log_info "服务访问地址:"
    echo "  🌐 Gateway API:    http://localhost:8080"
    echo "  🔍 Consul UI:      http://localhost:8500"
    echo "  📊 Prometheus:     http://localhost:9090"
    echo "  📈 Grafana:        http://localhost:3000"
    echo "  🚨 AlertManager:   http://localhost:9093"
    echo ""
    log_info "日志文件位置:"
    echo "  📝 Gateway:        logs/gateway.log"
    echo "  📝 Counter:        logs/counter.log"
    echo "  📝 Analytics:      logs/analytics.log"
}

# 主函数
main() {
    echo "🚀 HighGoPress 完整服务启动"
    echo "============================"
    echo
    
    # 检查必要工具
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose 未安装"
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装"
        exit 1
    fi
    
    # 创建网络
    docker network create highgopress 2>/dev/null || true
    
    # 启动基础服务
    start_kafka
    start_redis
    
    # 修复依赖
    fix_dependencies
    
    # 启动微服务
    start_microservices
    
    # 验证服务
    verify_services
    
    echo
    log_success "🎉 所有服务启动完成！"
    echo
    log_info "现在可以运行压测："
    echo "  ./scripts/quick_load_test.sh"
    echo
    log_info "停止所有服务："
    echo "  ./scripts/stop_all_services.sh"
}

# 错误处理
trap 'log_error "脚本执行失败"; exit 1' ERR

# 执行主函数
main "$@" 