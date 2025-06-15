#!/bin/bash

# HighGoPress 监控系统停止脚本
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

# 停止监控服务
stop_monitoring_services() {
    log_info "停止监控服务..."
    
    cd deploy
    
    # 停止并删除容器
    if docker-compose -f docker-compose.monitoring.yml ps -q | grep -q .; then
        log_info "停止监控服务容器..."
        docker-compose -f docker-compose.monitoring.yml down
        log_success "监控服务已停止"
    else
        log_warning "监控服务未运行"
    fi
    
    cd ..
}

# 清理资源（可选）
cleanup_resources() {
    local cleanup_data=false
    local cleanup_images=false
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --cleanup-data)
                cleanup_data=true
                shift
                ;;
            --cleanup-images)
                cleanup_images=true
                shift
                ;;
            --cleanup-all)
                cleanup_data=true
                cleanup_images=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done
    
    if [[ "$cleanup_data" == true ]]; then
        log_warning "清理监控数据..."
        read -p "确定要删除所有监控数据吗？(y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            cd deploy
            docker-compose -f docker-compose.monitoring.yml down -v
            cd ..
            
            # 删除数据目录
            rm -rf deploy/prometheus/data
            rm -rf deploy/grafana/data
            rm -rf deploy/alertmanager/data
            
            log_success "监控数据已清理"
        else
            log_info "取消数据清理"
        fi
    fi
    
    if [[ "$cleanup_images" == true ]]; then
        log_warning "清理Docker镜像..."
        read -p "确定要删除监控相关的Docker镜像吗？(y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            # 删除监控相关镜像
            local images=(
                "prom/prometheus"
                "grafana/grafana"
                "prom/alertmanager"
                "prom/node-exporter"
                "gcr.io/cadvisor/cadvisor"
                "oliver006/redis_exporter"
                "redis"
                "jaegertracing/all-in-one"
            )
            
            for image in "${images[@]}"; do
                if docker images -q "$image" | grep -q .; then
                    log_info "删除镜像: $image"
                    docker rmi $(docker images -q "$image") 2>/dev/null || true
                fi
            done
            
            log_success "Docker镜像已清理"
        else
            log_info "取消镜像清理"
        fi
    fi
}

# 显示服务状态
show_service_status() {
    log_info "检查监控服务状态..."
    
    cd deploy
    
    if docker-compose -f docker-compose.monitoring.yml ps -q | grep -q .; then
        log_warning "以下监控服务仍在运行:"
        docker-compose -f docker-compose.monitoring.yml ps
    else
        log_success "所有监控服务已停止"
    fi
    
    cd ..
}

# 显示帮助信息
show_help() {
    echo "HighGoPress 监控系统停止脚本"
    echo
    echo "用法: $0 [选项]"
    echo
    echo "选项:"
    echo "  --cleanup-data     清理所有监控数据（Prometheus、Grafana、AlertManager）"
    echo "  --cleanup-images   清理监控相关的Docker镜像"
    echo "  --cleanup-all      清理数据和镜像"
    echo "  -h, --help         显示此帮助信息"
    echo
    echo "示例:"
    echo "  $0                    # 仅停止服务"
    echo "  $0 --cleanup-data     # 停止服务并清理数据"
    echo "  $0 --cleanup-all      # 停止服务并清理所有资源"
}

# 主函数
main() {
    # 检查帮助参数
    if [[ "$1" == "-h" || "$1" == "--help" ]]; then
        show_help
        exit 0
    fi
    
    echo "🛑 HighGoPress 监控系统停止脚本"
    echo "=================================="
    echo
    
    stop_monitoring_services
    show_service_status
    cleanup_resources "$@"
    
    echo
    log_success "🎉 监控系统停止完成！"
    echo
    log_info "重新启动监控系统: ./scripts/start_monitoring.sh"
}

# 错误处理
trap 'log_error "脚本执行失败，请检查错误信息"; exit 1' ERR

# 执行主函数
main "$@" 