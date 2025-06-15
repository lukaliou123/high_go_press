#!/bin/bash

# HighGoPress 监控系统启动脚本 (修复版)
# Week 5 Day 14: Grafana 可视化仪表板

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

# 停止现有服务
stop_existing_services() {
    log_info "停止现有监控服务..."
    cd deploy
    if docker-compose -f docker-compose.monitoring.yml ps -q | grep -q .; then
        docker-compose -f docker-compose.monitoring.yml down
        log_success "现有服务已停止"
    else
        log_info "没有运行中的监控服务"
    fi
    cd ..
}

# 创建必要的目录和权限
setup_directories() {
    log_info "设置目录和权限..."
    
    # 创建数据目录
    mkdir -p deploy/prometheus/data
    mkdir -p deploy/grafana/data
    mkdir -p deploy/alertmanager/data
    
    # 设置权限 (解决容器权限问题)
    log_info "设置容器用户权限..."
    
    # Grafana 容器使用 UID 472
    sudo chown -R 472:472 deploy/grafana/data 2>/dev/null || {
        log_warning "无法设置 Grafana 权限，可能需要先清理目录"
        return 1
    }
    
    # Prometheus 容器使用 UID 65534 (nobody)
    sudo chown -R 65534:65534 deploy/prometheus/data 2>/dev/null || true
    
    # AlertManager 容器使用 UID 65534 (nobody)
    sudo chown -R 65534:65534 deploy/alertmanager/data 2>/dev/null || true
    
    # 设置目录权限
    sudo chmod -R 755 deploy/grafana/data deploy/prometheus/data deploy/alertmanager/data 2>/dev/null || true
    
    log_success "目录设置完成"
}

# 启动监控服务
start_monitoring_services() {
    log_info "启动修复后的监控服务..."
    
    cd deploy
    
    # 启动服务
    log_info "启动监控服务容器..."
    docker-compose -f docker-compose.monitoring.yml up -d
    
    cd ..
    
    log_success "监控服务启动完成"
}

# 等待服务就绪 (简化版)
wait_for_services() {
    log_info "等待服务启动..."
    
    local services=(
        "grafana:3000"
        "prometheus:9090"
    )
    
    for service in "${services[@]}"; do
        local name=$(echo $service | cut -d: -f1)
        local port=$(echo $service | cut -d: -f2)
        
        log_info "等待 $name 服务启动..."
        
        local max_attempts=15
        local attempt=1
        
        while [[ $attempt -le $max_attempts ]]; do
            if curl -s "http://localhost:$port" > /dev/null 2>&1; then
                log_success "$name 服务已启动"
                break
            fi
            
            if [[ $attempt -eq $max_attempts ]]; then
                log_warning "$name 服务启动可能需要更多时间，请稍后手动检查"
                break
            fi
            
            sleep 3
            ((attempt++))
        done
    done
}

# 显示服务状态
show_service_status() {
    log_info "监控服务状态:"
    echo
    
    cd deploy
    docker-compose -f docker-compose.monitoring.yml ps
    cd ..
    
    echo
    log_info "服务访问地址:"
    echo "  📊 Grafana:     http://localhost:3000 (admin/highgopress2024)"
    echo "  📈 Prometheus:  http://localhost:9090"
    echo "  🚨 AlertManager: http://localhost:9093"
    echo
}

# 检查服务健康状态
check_service_health() {
    log_info "检查服务健康状态..."
    
    # 检查Grafana
    if curl -s "http://localhost:3000/api/health" > /dev/null 2>&1; then
        log_success "✅ Grafana 服务正常"
    else
        log_warning "⚠️ Grafana 服务可能还在启动中"
    fi
    
    # 检查Prometheus
    if curl -s "http://localhost:9090/-/healthy" > /dev/null 2>&1; then
        log_success "✅ Prometheus 服务正常"
    else
        log_warning "⚠️ Prometheus 服务可能还在启动中"
    fi
    
    # 检查AlertManager
    if curl -s "http://localhost:9093/-/healthy" > /dev/null 2>&1; then
        log_success "✅ AlertManager 服务正常"
    else
        log_warning "⚠️ AlertManager 服务可能还在启动中"
    fi
}

# 主函数
main() {
    echo "🚀 HighGoPress 监控系统启动脚本 (修复版)"
    echo "=========================================="
    echo
    
    stop_existing_services
    setup_directories
    start_monitoring_services
    
    log_info "等待服务启动完成..."
    sleep 15
    
    wait_for_services
    show_service_status
    check_service_health
    
    echo
    log_success "🎉 监控系统启动完成！"
    echo
    log_info "主要修复:"
    echo "  ✅ 修复了 AlertManager 配置格式问题"
    echo "  ✅ 解决了 Grafana 权限问题"
    echo "  ✅ 移除了有问题的插件安装"
    echo
    log_info "下一步操作:"
    echo "  1. 访问 Grafana: http://localhost:3000"
    echo "  2. 使用账号: admin / highgopress2024"
    echo "  3. 查看预配置的仪表板"
    echo
    log_info "如果服务还在启动中，请等待1-2分钟后再访问"
}

# 错误处理
trap 'log_error "脚本执行失败，请检查错误信息"; exit 1' ERR

# 执行主函数
main "$@" 