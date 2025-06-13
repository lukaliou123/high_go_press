#!/bin/bash

# 真实Kafka性能测试脚本
# 比较Mock Kafka vs Real Kafka的性能差异

set -e

echo "🚀 HighGoPress 真实Kafka性能测试"
echo "=================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 检查依赖
check_dependencies() {
    echo -e "${BLUE}📋 检查依赖服务...${NC}"
    
    # 检查Consul
    if ! curl -s http://localhost:8500/v1/status/leader > /dev/null; then
        echo -e "${RED}❌ Consul未运行，请先启动Consul${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ Consul运行正常${NC}"
    
    # 检查Kafka
    if ! docker ps | grep kafka-highgopress > /dev/null; then
        echo -e "${RED}❌ Kafka容器未运行${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ Kafka容器运行正常${NC}"
    
    # 验证Kafka连通性
    if ! docker exec kafka-highgopress /opt/kafka/bin/kafka-topics.sh --list --bootstrap-server localhost:9092 > /dev/null 2>&1; then
        echo -e "${RED}❌ 无法连接到Kafka${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ Kafka连接正常${NC}"
}

# 启动微服务
start_services() {
    echo -e "${BLUE}🔧 启动微服务...${NC}"
    
    # 清理之前的进程
    pkill -f "counter_service\|analytics_service\|gateway" 2>/dev/null || true
    sleep 2
    
    # 启动Counter服务
    echo "启动Counter服务..."
    cd cmd/counter && go build -o ../../bin/counter_service . && cd ../..
    ./bin/counter_service &
    COUNTER_PID=$!
    sleep 3
    
    # 启动Analytics服务
    echo "启动Analytics服务..."
    cd cmd/analytics && go build -o ../../bin/analytics_service . && cd ../..
    ./bin/analytics_service &
    ANALYTICS_PID=$!
    sleep 3
    
    # 启动Gateway
    echo "启动Gateway..."
    cd cmd/gateway && go build -o ../../bin/gateway . && cd ../..
    ./bin/gateway &
    GATEWAY_PID=$!
    sleep 5
    
    echo -e "${GREEN}✅ 所有服务启动完成${NC}"
    echo "Counter PID: $COUNTER_PID"
    echo "Analytics PID: $ANALYTICS_PID"  
    echo "Gateway PID: $GATEWAY_PID"
}

# 健康检查
health_check() {
    echo -e "${BLUE}🩺 执行健康检查...${NC}"
    
    # 检查Counter服务
    if curl -s http://localhost:9001/health > /dev/null; then
        echo -e "${GREEN}✅ Counter服务健康${NC}"
    else
        echo -e "${RED}❌ Counter服务异常${NC}"
        return 1
    fi
    
    # 检查Analytics服务
    if curl -s http://localhost:9002/health > /dev/null; then
        echo -e "${GREEN}✅ Analytics服务健康${NC}"
    else
        echo -e "${RED}❌ Analytics服务异常${NC}"
        return 1
    fi
    
    # 检查Gateway
    if curl -s http://localhost:8080/health > /dev/null; then
        echo -e "${GREEN}✅ Gateway健康${NC}"
    else
        echo -e "${RED}❌ Gateway异常${NC}"
        return 1
    fi
    
    # 检查Consul服务注册
    echo "检查Consul服务注册状态..."
    consul members 2>/dev/null || echo "Consul成员列表获取失败"
}

# Kafka连接测试
test_kafka_connection() {
    echo -e "${BLUE}🔗 测试Kafka连接...${NC}"
    
    # 测试生产者
    echo "test-message-$(date +%s)" | docker exec -i kafka-highgopress /opt/kafka/bin/kafka-console-producer.sh --topic counter-events --bootstrap-server localhost:9092
    
    # 测试消费者（超时获取消息）
    timeout 5s docker exec kafka-highgopress /opt/kafka/bin/kafka-console-consumer.sh --topic counter-events --bootstrap-server localhost:9092 --from-beginning || echo "消费者测试完成"
    
    echo -e "${GREEN}✅ Kafka连接测试完成${NC}"
}

# 性能测试
performance_test() {
    echo -e "${BLUE}⚡ 开始性能测试...${NC}"
    
    # 预热
    echo "预热系统..."
    for i in {1..100}; do
        curl -s -X POST http://localhost:8080/api/v1/counter/increment \
             -H "Content-Type: application/json" \
             -d '{"key":"warmup","increment":1}' > /dev/null
    done
    
    sleep 2
    
    # 性能测试参数
    DURATION=30
    CONCURRENT=50
    
    echo -e "${YELLOW}🎯 性能测试参数:${NC}"
    echo "  - 测试时长: ${DURATION}秒"
    echo "  - 并发数: ${CONCURRENT}"
    echo "  - 测试目标: http://localhost:8080/api/v1/counter/increment"
    
    # 执行压测
    echo -e "${BLUE}🔥 执行压力测试...${NC}"
    
    # 使用wrk进行压测
    if command -v wrk > /dev/null; then
        wrk -t${CONCURRENT} -c${CONCURRENT} -d${DURATION}s -s scripts/post_increment.lua http://localhost:8080/api/v1/counter/increment
    else
        echo -e "${YELLOW}⚠️  wrk未安装，使用curl进行简单测试${NC}"
        
        # 简单并发测试
        start_time=$(date +%s)
        request_count=0
        
        for i in $(seq 1 $CONCURRENT); do
            {
                while [ $(($(date +%s) - start_time)) -lt $DURATION ]; do
                    curl -s -X POST http://localhost:8080/api/v1/counter/increment \
                         -H "Content-Type: application/json" \
                         -d "{\"key\":\"test-key-$i\",\"increment\":1}" > /dev/null
                    ((request_count++))
                done
            } &
        done
        
        wait
        
        end_time=$(date +%s)
        total_time=$((end_time - start_time))
        qps=$((request_count / total_time))
        
        echo -e "${GREEN}📊 测试结果:${NC}"
        echo "  - 总请求数: $request_count"
        echo "  - 总耗时: ${total_time}秒"
        echo "  - QPS: $qps"
    fi
}

# Kafka性能监控
monitor_kafka() {
    echo -e "${BLUE}📊 监控Kafka性能...${NC}"
    
    echo "Topic详情:"
    docker exec kafka-highgopress /opt/kafka/bin/kafka-topics.sh --describe --topic counter-events --bootstrap-server localhost:9092
    
    echo -e "\n消费者组状态:"
    docker exec kafka-highgopress /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --list 2>/dev/null || echo "暂无消费者组"
}

# 清理函数
cleanup() {
    echo -e "${YELLOW}🧹 清理资源...${NC}"
    
    # 停止服务进程
    if [ ! -z "$COUNTER_PID" ]; then
        kill $COUNTER_PID 2>/dev/null || true
    fi
    if [ ! -z "$ANALYTICS_PID" ]; then
        kill $ANALYTICS_PID 2>/dev/null || true
    fi
    if [ ! -z "$GATEWAY_PID" ]; then
        kill $GATEWAY_PID 2>/dev/null || true
    fi
    
    # 等待进程结束
    sleep 3
    
    # 强制清理
    pkill -f "counter_service\|analytics_service\|gateway" 2>/dev/null || true
    
    echo -e "${GREEN}✅ 清理完成${NC}"
}

# 主函数
main() {
    trap cleanup EXIT
    
    check_dependencies
    start_services
    sleep 5
    health_check
    test_kafka_connection
    monitor_kafka
    performance_test
    
    echo -e "${GREEN}🎉 真实Kafka性能测试完成！${NC}"
}

# Lua脚本创建（用于wrk）
create_lua_script() {
    cat > scripts/post_increment.lua << 'EOF'
wrk.method = "POST"
wrk.body   = '{"key":"test-key","increment":1}'
wrk.headers["Content-Type"] = "application/json"
EOF
}

# 创建必要的目录和脚本
mkdir -p scripts bin
create_lua_script

# 运行主函数
main "$@" 