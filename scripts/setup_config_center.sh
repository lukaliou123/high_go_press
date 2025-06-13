#!/bin/bash

# 配置中心初始化脚本
# Week 4 Day 11 - 统一配置管理

set -e

echo "=== HighGoPress 配置中心初始化 ==="
echo "时间: $(date)"
echo

# 配置信息
CONSUL_ADDR="localhost:8500"
CONFIG_FILE="configs/config.yaml"
ENVIRONMENTS=("dev" "test" "prod")
SERVICES=("gateway" "counter" "analytics")

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查Consul是否运行
check_consul() {
    print_status "检查Consul状态..."
    if ! curl -s http://${CONSUL_ADDR}/v1/status/leader > /dev/null; then
        print_error "Consul未运行或无法访问 (${CONSUL_ADDR})"
        exit 1
    fi
    print_success "Consul运行正常"
}

# 构建配置工具
build_config_tool() {
    print_status "构建配置管理工具..."
    if go build -o bin/config-tool cmd/config-tool/main.go 2>/dev/null; then
        print_success "配置工具构建成功"
    else
        print_error "配置工具构建失败"
        exit 1
    fi
}

# 推送单个服务配置
push_service_config() {
    local service=$1
    local environment=$2
    
    print_status "推送 ${service} 服务配置到 ${environment} 环境..."
    
    if ./bin/config-tool -action=put -service=${service} -env=${environment} -config=${CONFIG_FILE} -consul=${CONSUL_ADDR} > /dev/null 2>&1; then
        print_success "配置推送成功: ${service} -> ${environment}"
    else
        print_error "配置推送失败: ${service} -> ${environment}"
        return 1
    fi
}

# 验证配置
verify_config() {
    local service=$1
    local environment=$2
    
    print_status "验证 ${service} 服务配置 (${environment} 环境)..."
    
    local temp_file=$(mktemp)
    if ./bin/config-tool -action=get -service=${service} -env=${environment} -consul=${CONSUL_ADDR} > ${temp_file} 2>/dev/null; then
        local config_size=$(wc -c < ${temp_file})
        if [[ ${config_size} -gt 100 ]]; then
            print_success "配置验证成功: ${service} -> ${environment} (${config_size} bytes)"
        else
            print_warning "配置内容可能不完整: ${service} -> ${environment}"
        fi
    else
        print_error "配置验证失败: ${service} -> ${environment}"
        rm ${temp_file}
        return 1
    fi
    rm ${temp_file}
}

# 推送所有配置
push_all_configs() {
    print_status "开始推送所有服务配置..."
    
    local total_configs=0
    local success_count=0
    
    for environment in "${ENVIRONMENTS[@]}"; do
        for service in "${SERVICES[@]}"; do
            total_configs=$((total_configs + 1))
            if push_service_config ${service} ${environment}; then
                success_count=$((success_count + 1))
            fi
        done
    done
    
    print_success "配置推送完成: ${success_count}/${total_configs} 成功"
}

# 验证所有配置
verify_all_configs() {
    print_status "开始验证所有配置..."
    
    local total_configs=0
    local success_count=0
    
    for environment in "${ENVIRONMENTS[@]}"; do
        for service in "${SERVICES[@]}"; do
            total_configs=$((total_configs + 1))
            if verify_config ${service} ${environment}; then
                success_count=$((success_count + 1))
            fi
        done
    done
    
    print_success "配置验证完成: ${success_count}/${total_configs} 成功"
}

# 显示配置中心统计
show_config_stats() {
    print_status "配置中心统计信息..."
    
    # 计算Consul中的配置数量
    local keys=$(curl -s http://${CONSUL_ADDR}/v1/kv/high-go-press/config?recurse | jq -r '.[].Key' | wc -l)
    print_success "总配置数量: ${keys}"
    
    # 显示配置树结构
    echo "配置树结构:"
    for environment in "${ENVIRONMENTS[@]}"; do
        echo "  ├── ${environment}/"
        for service in "${SERVICES[@]}"; do
            local key="high-go-press/config/${environment}/${service}"
            local response=$(curl -s http://${CONSUL_ADDR}/v1/kv/${key})
            if [[ -n "$response" && "$response" != "null" ]]; then
                local size=$(echo $response | jq -r '.[0].Value' | base64 -d | wc -c)
                echo "  │   ├── ${service} (${size} bytes)"
            else
                echo "  │   ├── ${service} (未找到)"
            fi
        done
    done
}

# 创建配置模板
create_config_templates() {
    print_status "创建特定环境配置模板..."
    
    # 创建test环境配置
    cat > configs/test-specific.yaml << 'EOF'
# Test环境特定配置
environment: "test"

gateway:
  server:
    host: "0.0.0.0"
    port: 8081
    mode: "test"

counter:
  server:
    port: 9011
  performance:
    worker_pool_size: 500

analytics:
  server:
    port: 9012

redis:
  address: "localhost:6380"
  db: 1

kafka:
  mode: "mock"
  topic: "counter-events-test"

log:
  level: "debug"
EOF

    # 创建prod环境配置
    cat > configs/prod-specific.yaml << 'EOF'
# Production环境特定配置
environment: "prod"

gateway:
  server:
    host: "0.0.0.0"
    port: 8080
    mode: "release"

counter:
  server:
    port: 9001
  performance:
    worker_pool_size: 2000

analytics:
  server:
    port: 9002

redis:
  address: "redis-cluster:6379"
  pool_size: 50

kafka:
  mode: "real"
  brokers: ["kafka1:9092", "kafka2:9092", "kafka3:9092"]
  topic: "counter-events-prod"

log:
  level: "warn"
  format: "json"
  output: "file"
  file:
    path: "/var/log/high-go-press"
EOF

    print_success "配置模板创建完成"
}

# 清理配置
cleanup_configs() {
    print_status "清理配置中心数据..."
    
    for environment in "${ENVIRONMENTS[@]}"; do
        for service in "${SERVICES[@]}"; do
            ./bin/config-tool -action=delete -service=${service} -env=${environment} -consul=${CONSUL_ADDR} > /dev/null 2>&1 || true
        done
    done
    
    rm -f bin/config-tool configs/test-specific.yaml configs/prod-specific.yaml
    print_success "清理完成"
}

# 主函数
main() {
    echo "=== 开始配置中心初始化 ==="
    
    # 前置检查
    check_consul
    
    # 构建工具
    build_config_tool
    
    # 创建配置模板
    create_config_templates
    
    echo
    echo "=== 推送配置到配置中心 ==="
    push_all_configs
    
    echo
    echo "=== 验证配置 ==="
    verify_all_configs
    
    echo
    echo "=== 配置中心统计 ==="
    show_config_stats
    
    echo
    print_success "配置中心初始化完成！"
    echo
    echo "使用方法:"
    echo "  # 获取配置"
    echo "  ./bin/config-tool -action=get -service=gateway -env=dev"
    echo
    echo "  # 推送配置"
    echo "  ./bin/config-tool -action=put -service=gateway -env=dev -config=configs/config.yaml"
    echo
    echo "  # 监听配置变化"
    echo "  ./bin/config-tool -action=watch -service=gateway -env=dev"
    echo
    echo "  # 查看配置历史"
    echo "  ./bin/config-tool -action=history -service=gateway -env=dev"
    echo
}

# 如果传入cleanup参数，则执行清理
if [[ "$1" == "cleanup" ]]; then
    check_consul
    build_config_tool
    cleanup_configs
    exit 0
fi

# 错误处理
trap 'print_error "初始化过程中出现错误"; exit 1' ERR

# 执行主流程
main "$@" 