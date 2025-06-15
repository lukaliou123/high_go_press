#!/bin/bash

# HighGoPress 监控系统启动脚本
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

# 检查Docker和Docker Compose
check_prerequisites() {
    log_info "检查系统依赖..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose 未安装，请先安装 Docker Compose"
        exit 1
    fi
    
    # 检查Docker是否运行
    if ! docker info &> /dev/null; then
        log_error "Docker 服务未运行，请启动 Docker 服务"
        exit 1
    fi
    
    log_success "系统依赖检查通过"
}

# 创建必要的目录
create_directories() {
    log_info "创建监控系统目录结构..."
    
    # 创建数据目录
    mkdir -p deploy/prometheus/data
    mkdir -p deploy/grafana/data
    mkdir -p deploy/alertmanager/data
    
    # 创建配置目录
    mkdir -p deploy/prometheus/rules
    mkdir -p deploy/grafana/provisioning/datasources
    mkdir -p deploy/grafana/provisioning/dashboards
    mkdir -p deploy/grafana/dashboards/{overview,services,business,infrastructure,alerts,performance}
    mkdir -p deploy/alertmanager/templates
    
    # 设置权限
    chmod -R 755 deploy/
    
    log_success "目录结构创建完成"
}

# 检查配置文件
check_configs() {
    log_info "检查配置文件..."
    
    local configs=(
        "deploy/docker-compose.monitoring.yml"
        "deploy/prometheus/prometheus.yml"
        "deploy/grafana/provisioning/datasources/prometheus.yml"
        "deploy/grafana/provisioning/dashboards/dashboard.yml"
        "deploy/alertmanager/alertmanager.yml"
        "deploy/prometheus/rules/highgopress-alerts.yml"
    )
    
    for config in "${configs[@]}"; do
        if [[ ! -f "$config" ]]; then
            log_error "配置文件不存在: $config"
            exit 1
        fi
    done
    
    log_success "配置文件检查通过"
}

# 启动监控服务
start_monitoring_services() {
    log_info "启动监控服务..."
    
    # 进入部署目录
    cd deploy
    
    # 拉取最新镜像
    log_info "拉取Docker镜像..."
    docker-compose -f docker-compose.monitoring.yml pull
    
    # 启动服务
    log_info "启动监控服务容器..."
    docker-compose -f docker-compose.monitoring.yml up -d
    
    # 返回原目录
    cd ..
    
    log_success "监控服务启动完成"
}

# 等待服务就绪
wait_for_services() {
    log_info "等待服务启动..."
    
    local services=(
        "prometheus:9090"
        "grafana:3000"
        "alertmanager:9093"
    )
    
    for service in "${services[@]}"; do
        local name=$(echo $service | cut -d: -f1)
        local port=$(echo $service | cut -d: -f2)
        
        log_info "等待 $name 服务启动..."
        
        local max_attempts=30
        local attempt=1
        
        while [[ $attempt -le $max_attempts ]]; do
            if curl -s "http://localhost:$port" > /dev/null 2>&1; then
                log_success "$name 服务已启动"
                break
            fi
            
            if [[ $attempt -eq $max_attempts ]]; then
                log_error "$name 服务启动超时"
                return 1
            fi
            
            sleep 2
            ((attempt++))
        done
    done
    
    log_success "所有服务已就绪"
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
    echo "  📋 Node Exporter: http://localhost:9100"
    echo "  🐳 cAdvisor:    http://localhost:8080"
    echo
}

# 验证监控系统
verify_monitoring() {
    log_info "验证监控系统..."
    
    # 检查Prometheus targets
    log_info "检查 Prometheus targets..."
    local targets_response=$(curl -s "http://localhost:9090/api/v1/targets" | jq -r '.data.activeTargets | length')
    if [[ $targets_response -gt 0 ]]; then
        log_success "Prometheus targets: $targets_response 个"
    else
        log_warning "Prometheus targets 数量为 0，请检查配置"
    fi
    
    # 检查Grafana数据源
    log_info "检查 Grafana 数据源..."
    local grafana_response=$(curl -s -u admin:highgopress2024 "http://localhost:3000/api/datasources" | jq -r 'length')
    if [[ $grafana_response -gt 0 ]]; then
        log_success "Grafana 数据源: $grafana_response 个"
    else
        log_warning "Grafana 数据源配置可能有问题"
    fi
    
    # 检查告警规则
    log_info "检查告警规则..."
    local rules_response=$(curl -s "http://localhost:9090/api/v1/rules" | jq -r '.data.groups | length')
    if [[ $rules_response -gt 0 ]]; then
        log_success "告警规则组: $rules_response 个"
    else
        log_warning "告警规则未加载，请检查配置"
    fi
}

# 主函数
main() {
    echo "🚀 HighGoPress 监控系统启动脚本"
    echo "=================================="
    echo
    
    check_prerequisites
    create_directories
    check_configs
    start_monitoring_services
    
    log_info "等待服务启动完成..."
    sleep 10
    
    wait_for_services
    show_service_status
    verify_monitoring
    
    echo
    log_success "🎉 监控系统启动完成！"
    echo
    log_info "下一步操作:"
    echo "  1. 访问 Grafana: http://localhost:3000"
    echo "  2. 使用账号: admin / highgopress2024"
    echo "  3. 查看预配置的仪表板"
    echo "  4. 启动 HighGoPress 应用服务以查看实际指标"
    echo
    log_info "停止监控系统: ./scripts/stop_monitoring.sh"
}

# 错误处理
trap 'log_error "脚本执行失败，请检查错误信息"; exit 1' ERR

# 执行主函数
main "$@" 