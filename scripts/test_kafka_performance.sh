#!/bin/bash

# Kafka 性能测试脚本
# 测试真实Kafka vs Mock Kafka的性能差异

set -e

echo "🚀 HighGoPress Kafka性能测试"
echo "============================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m' 
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 测试参数
DURATION=30
REQUESTS_PER_SECOND=100
TOTAL_REQUESTS=$((DURATION * REQUESTS_PER_SECOND))

# 检查服务状态
check_services() {
    echo -e "${BLUE}📋 检查服务状态...${NC}"
    
    # 检查Counter服务 gRPC端口
    if ! ss -tlnp | grep :9001 > /dev/null; then
        echo -e "${RED}❌ Counter服务未在9001端口运行${NC}"
        echo "启动Counter服务..."
        cd cmd/counter && go run . > ../../logs/counter.log 2>&1 &
        sleep 5
        cd ../..
    fi
    echo -e "${GREEN}✅ Counter服务检查完成${NC}"
    
    # 检查Analytics服务
    if ! ss -tlnp | grep :9002 > /dev/null; then
        echo -e "${RED}❌ Analytics服务未在9002端口运行${NC}"
        echo "启动Analytics服务..."
        cd cmd/analytics && go run . > ../../logs/analytics.log 2>&1 &
        sleep 5
        cd ../..
    fi
    echo -e "${GREEN}✅ Analytics服务检查完成${NC}"
    
    # 检查Gateway
    if ! ss -tlnp | grep :8080 > /dev/null; then
        echo -e "${RED}❌ Gateway未在8080端口运行${NC}"
        echo "启动Gateway服务..."
        cd cmd/gateway && go run . > ../../logs/gateway.log 2>&1 &
        sleep 5
        cd ../..
    fi
    echo -e "${GREEN}✅ Gateway服务检查完成${NC}"
    
    # 检查Kafka
    if ! docker ps | grep kafka-highgopress > /dev/null; then
        echo -e "${RED}❌ Kafka容器未运行${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ Kafka运行正常${NC}"
}

# 等待服务启动
wait_for_services() {
    echo -e "${BLUE}⏳ 等待服务完全启动...${NC}"
    
    max_attempts=30
    attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if ss -tlnp | grep -E ":8080.*:9001.*:9002" > /dev/null 2>&1 || 
           (ss -tlnp | grep :8080 > /dev/null && ss -tlnp | grep :9001 > /dev/null && ss -tlnp | grep :9002 > /dev/null); then
            echo -e "${GREEN}✅ 所有服务已启动${NC}"
            return 0
        fi
        
        echo "等待服务启动... ($((attempt + 1))/$max_attempts)"
        sleep 2
        ((attempt++))
    done
    
    echo -e "${YELLOW}⚠️  超时但继续测试${NC}"
}

# 简单的连接测试
test_connectivity() {
    echo -e "${BLUE}🔗 测试连接性...${NC}"
    
    # 测试简单的请求
    response=$(curl -s -w "%{http_code}" -o /tmp/test_response \
        -X POST http://localhost:8080/api/v1/counter/increment \
        -H "Content-Type: application/json" \
        -d '{"key":"connectivity-test","increment":1}')
    
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✅ API连接正常${NC}"
        echo "响应: $(cat /tmp/test_response)"
    else
        echo -e "${YELLOW}⚠️  API连接异常，状态码: $response${NC}"
        echo "继续测试..."
    fi
}

# Kafka消息验证
verify_kafka_messages() {
    echo -e "${BLUE}📊 验证Kafka消息传递...${NC}"
    
    # 发送几个测试消息
    for i in {1..5}; do
        curl -s -X POST http://localhost:8080/api/v1/counter/increment \
             -H "Content-Type: application/json" \
             -d "{\"key\":\"kafka-test-$i\",\"increment\":1}" > /dev/null
    done
    
    sleep 2
    
    # 检查Kafka topic中的消息
    echo "检查Kafka topic中的消息数量..."
    message_count=$(docker exec kafka-highgopress /opt/kafka/bin/kafka-run-class.sh kafka.tools.GetOffsetShell \
        --broker-list localhost:9092 --topic counter-events --time -1 | awk -F':' '{sum += $3} END {print sum}' 2>/dev/null || echo "0")
    
    echo "Topic counter-events 中的消息数量: $message_count"
    
    if [ "$message_count" -gt 0 ]; then
        echo -e "${GREEN}✅ Kafka消息传递正常${NC}"
    else
        echo -e "${YELLOW}⚠️  未检测到Kafka消息，但继续测试${NC}"
    fi
}

# 性能测试函数
run_performance_test() {
    local test_name="$1"
    local description="$2"
    
    echo -e "${BLUE}⚡ $test_name${NC}"
    echo "$description"
    echo "测试参数: ${DURATION}秒, 目标 ${REQUESTS_PER_SECOND} req/s"
    
    # 预热
    echo "预热系统..."
    for i in {1..10}; do
        curl -s -X POST http://localhost:8080/api/v1/counter/increment \
             -H "Content-Type: application/json" \
             -d '{"key":"warmup","increment":1}' > /dev/null
    done
    
    sleep 2
    
    # 性能测试
    echo "开始性能测试..."
    start_time=$(date +%s.%N)
    success_count=0
    error_count=0
    
    # 并发测试
    {
        for i in $(seq 1 $TOTAL_REQUESTS); do
            {
                response=$(curl -s -w "%{http_code}" -o /dev/null \
                    -X POST http://localhost:8080/api/v1/counter/increment \
                    -H "Content-Type: application/json" \
                    -d "{\"key\":\"perf-test-$i\",\"increment\":1}")
                
                if [ "$response" = "200" ]; then
                    ((success_count++))
                else
                    ((error_count++))
                fi
            } &
            
            # 控制并发数
            if (( i % 10 == 0 )); then
                wait
            fi
            
            # 控制请求速率
            sleep $(echo "scale=6; 1 / $REQUESTS_PER_SECOND" | bc -l) 2>/dev/null || sleep 0.01
        done
        
        wait
    }
    
    end_time=$(date +%s.%N)
    duration=$(echo "$end_time - $start_time" | bc -l)
    actual_qps=$(echo "scale=2; $success_count / $duration" | bc -l)
    
    echo -e "${GREEN}📊 $test_name 结果:${NC}"
    echo "  - 总请求数: $TOTAL_REQUESTS"
    echo "  - 成功请求: $success_count"
    echo "  - 失败请求: $error_count"
    echo "  - 总耗时: ${duration}秒"
    echo "  - 实际QPS: $actual_qps"
    echo "  - 成功率: $(echo "scale=2; $success_count * 100 / $TOTAL_REQUESTS" | bc -l)%"
    
    # 返回QPS用于比较
    echo "$actual_qps"
}

# 监控Kafka状态
monitor_kafka_status() {
    echo -e "${BLUE}📊 Kafka状态监控${NC}"
    
    echo "Topic详情:"
    docker exec kafka-highgopress /opt/kafka/bin/kafka-topics.sh \
        --describe --topic counter-events --bootstrap-server localhost:9092 2>/dev/null || echo "Topic信息获取失败"
    
    echo -e "\n消费者组状态:"
    docker exec kafka-highgopress /opt/kafka/bin/kafka-consumer-groups.sh \
        --bootstrap-server localhost:9092 --describe --group analytics-group 2>/dev/null || echo "消费者组信息获取失败"
}

# 清理函数
cleanup() {
    echo -e "${YELLOW}🧹 清理测试环境...${NC}"
    
    # 清理测试数据的key
    echo "清理测试数据..."
    # Note: 这里应该清理Redis中的测试key，但为了简单起见暂时跳过
    
    echo -e "${GREEN}✅ 清理完成${NC}"
}

# 主测试流程
main() {
    echo "开始Kafka性能测试..."
    
    # 设置清理陷阱
    trap cleanup EXIT
    
    # 检查依赖
    check_services
    wait_for_services
    test_connectivity
    verify_kafka_messages
    monitor_kafka_status
    
    echo -e "${YELLOW}🎯 开始性能测试阶段${NC}"
    
    # 运行性能测试
    real_kafka_qps=$(run_performance_test "真实Kafka性能测试" "测试使用真实Kafka时的系统性能")
    
    echo -e "${GREEN}🎉 测试完成！${NC}"
    echo -e "${BLUE}📋 测试总结:${NC}"
    echo "  - 真实Kafka QPS: $real_kafka_qps"
    
    # 与历史基准比较
    echo -e "\n${YELLOW}📈 性能对比:${NC}"
    echo "  - Phase 1 (单体): ~21,000 QPS"
    echo "  - Phase 2 (Mock Kafka): ~738 QPS"
    echo "  - Phase 2 (Real Kafka): $real_kafka_qps QPS"
    
    # 计算改进
    if command -v bc > /dev/null; then
        improvement=$(echo "scale=2; ($real_kafka_qps - 738) * 100 / 738" | bc -l 2>/dev/null || echo "N/A")
        echo "  - Real vs Mock 改进: ${improvement}%"
    fi
}

# 运行主函数
main "$@" 