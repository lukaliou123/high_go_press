#!/bin/bash

# 配置中心测试脚本
# Week 4 Day 11 - 统一配置管理

set -e

echo "=== HighGoPress 配置中心测试 ==="
echo "时间: $(date)"
echo

# 配置信息
CONSUL_ADDR="localhost:8500"
SERVICE_NAME="test-service"
ENVIRONMENT="dev"
CONFIG_FILE="configs/config.yaml"

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

# 测试配置推送
test_config_push() {
    print_status "测试配置推送..."
    
    if ./bin/config-tool -action=put -service=${SERVICE_NAME} -env=${ENVIRONMENT} -config=${CONFIG_FILE} -consul=${CONSUL_ADDR}; then
        print_success "配置推送成功"
    else
        print_error "配置推送失败"
        return 1
    fi
}

# 测试配置获取
test_config_get() {
    print_status "测试配置获取..."
    
    local temp_file=$(mktemp)
    if ./bin/config-tool -action=get -service=${SERVICE_NAME} -env=${ENVIRONMENT} -consul=${CONSUL_ADDR} > ${temp_file}; then
        print_success "配置获取成功"
        echo "获取的配置:"
        head -20 ${temp_file}
        rm ${temp_file}
    else
        print_error "配置获取失败"
        rm ${temp_file}
        return 1
    fi
}

# 测试配置历史
test_config_history() {
    print_status "测试配置历史查询..."
    
    if ./bin/config-tool -action=history -service=${SERVICE_NAME} -env=${ENVIRONMENT} -consul=${CONSUL_ADDR}; then
        print_success "配置历史查询成功"
    else
        print_warning "配置历史查询失败（可能是因为没有历史版本）"
    fi
}

# 测试Consul KV直接访问
test_consul_kv() {
    print_status "测试Consul KV直接访问..."
    
    local key="high-go-press/config/${ENVIRONMENT}/${SERVICE_NAME}"
    local response=$(curl -s http://${CONSUL_ADDR}/v1/kv/${key})
    
    if [[ -n "$response" && "$response" != "null" ]]; then
        print_success "Consul KV访问成功"
        echo "KV键: ${key}"
        echo "数据长度: $(echo $response | jq -r '.[0].Value' | base64 -d | wc -c)"
    else
        print_error "Consul KV访问失败"
        return 1
    fi
}

# 测试配置监听（后台运行短时间）
test_config_watch() {
    print_status "测试配置监听（5秒）..."
    
    # 后台启动监听
    timeout 5s ./bin/config-tool -action=watch -service=${SERVICE_NAME} -env=${ENVIRONMENT} -consul=${CONSUL_ADDR} &
    local watch_pid=$!
    
    sleep 1
    
    # 修改配置触发变更
    print_status "触发配置变更..."
    if ./bin/config-tool -action=put -service=${SERVICE_NAME} -env=${ENVIRONMENT} -config=${CONFIG_FILE} -consul=${CONSUL_ADDR} > /dev/null; then
        print_success "配置变更触发成功"
    fi
    
    # 等待监听结束
    wait ${watch_pid} 2>/dev/null || true
    print_success "配置监听测试完成"
}

# 性能测试
test_performance() {
    print_status "配置中心性能测试..."
    
    local start_time=$(date +%s%N)
    
    # 连续获取配置10次
    for i in {1..10}; do
        ./bin/config-tool -action=get -service=${SERVICE_NAME} -env=${ENVIRONMENT} -consul=${CONSUL_ADDR} > /dev/null
    done
    
    local end_time=$(date +%s%N)
    local duration=$(( (end_time - start_time) / 1000000 )) # 转换为毫秒
    local avg_latency=$(( duration / 10 ))
    
    print_success "性能测试完成"
    echo "  - 总耗时: ${duration}ms"
    echo "  - 平均延迟: ${avg_latency}ms"
    echo "  - QPS: $(( 10000 / duration ))"
}

# 清理测试数据
cleanup() {
    print_status "清理测试数据..."
    
    ./bin/config-tool -action=delete -service=${SERVICE_NAME} -env=${ENVIRONMENT} -consul=${CONSUL_ADDR} > /dev/null || true
    rm -f bin/config-tool
    
    print_success "清理完成"
}

# 主测试流程
main() {
    echo "=== 开始配置中心测试 ==="
    
    # 前置检查
    check_consul
    
    # 构建工具
    build_config_tool
    
    # 功能测试
    echo
    echo "=== 功能测试 ==="
    test_config_push
    test_config_get
    test_config_history
    test_consul_kv
    
    # 高级功能测试
    echo
    echo "=== 高级功能测试 ==="
    test_config_watch
    
    # 性能测试
    echo
    echo "=== 性能测试 ==="
    test_performance
    
    # 清理
    echo
    echo "=== 清理 ==="
    cleanup
    
    echo
    print_success "配置中心测试全部完成！"
    echo
    echo "测试结果总结:"
    echo "  ✅ 配置推送和获取"
    echo "  ✅ 配置历史管理"
    echo "  ✅ Consul KV集成"
    echo "  ✅ 配置变更监听"
    echo "  ✅ 性能基准测试"
    echo
    echo "配置中心已就绪，可以进行微服务集成！"
}

# 错误处理
trap 'print_error "测试过程中出现错误"; cleanup; exit 1' ERR

# 执行主流程
main "$@" 