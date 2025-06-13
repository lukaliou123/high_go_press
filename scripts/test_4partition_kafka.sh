#!/bin/bash

# 4分区Kafka性能测试脚本
# 测试最优Kafka配置的QPS性能

set -e

echo "🚀 HighGoPress 4分区Kafka性能测试"
echo "=================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 检查hey工具
HEY_BIN="$HOME/go/bin/hey"
if [ ! -f "$HEY_BIN" ]; then
    echo "安装hey性能测试工具..."
    go install github.com/rakyll/hey@latest
fi

# 检查Redis
echo -e "${BLUE}🔍 检查Redis状态...${NC}"
if ! redis-cli ping > /dev/null 2>&1; then
    echo -e "${RED}❌ Redis未运行，请先启动Redis${NC}"
    echo "启动命令: redis-server"
    exit 1
fi
echo -e "${GREEN}✅ Redis运行正常${NC}"

# 检查Kafka
echo -e "${BLUE}🔍 检查Kafka状态...${NC}"
if ! ./kafka_2.13-3.9.1/bin/kafka-broker-api-versions.sh --bootstrap-server localhost:9092 &>/dev/null; then
    echo -e "${RED}❌ Kafka未运行，请先启动Kafka${NC}"
    echo "启动命令: ./scripts/setup_optimal_kafka.sh"
    exit 1
fi
echo -e "${GREEN}✅ Kafka运行正常${NC}"

# 验证4分区配置
echo -e "${BLUE}📋 验证4分区配置...${NC}"
PARTITION_COUNT=$(./kafka_2.13-3.9.1/bin/kafka-topics.sh --describe --topic counter-events --bootstrap-server localhost:9092 | grep "PartitionCount:" | awk '{print $6}')
if [ "$PARTITION_COUNT" = "4" ]; then
    echo -e "${GREEN}✅ 确认4分区配置正确${NC}"
else
    echo -e "${RED}❌ 分区配置错误，当前分区数: $PARTITION_COUNT${NC}"
    echo "完整输出:"
    ./kafka_2.13-3.9.1/bin/kafka-topics.sh --describe --topic counter-events --bootstrap-server localhost:9092
    exit 1
fi

# 清理之前的进程
echo -e "${BLUE}🧹 清理之前的进程...${NC}"
pkill -f 'bin/counter' 2>/dev/null || true
pkill -f 'bin/analytics' 2>/dev/null || true
pkill -f 'bin/gateway' 2>/dev/null || true
sleep 3

# 编译服务
echo -e "${BLUE}🔧 编译服务...${NC}"
go build -o bin/counter cmd/counter/main.go
go build -o bin/analytics cmd/analytics/main.go

# 启动微服务（不使用Gateway，直接测试gRPC）
echo -e "${BLUE}🚀 启动微服务...${NC}"

# 启动Counter服务
echo "启动Counter服务..."
KAFKA_MODE=real ./bin/counter > logs/counter.log 2>&1 &
COUNTER_PID=$!

# 启动Analytics服务
echo "启动Analytics服务..."
KAFKA_MODE=real ./bin/analytics > logs/analytics.log 2>&1 &
ANALYTICS_PID=$!

echo "服务PID - Counter:$COUNTER_PID, Analytics:$ANALYTICS_PID"

# 等待服务启动
echo -e "${YELLOW}⏳ 等待服务启动...${NC}"
sleep 8

# 检查服务状态
echo -e "${BLUE}🔍 检查服务状态...${NC}"
if ! ss -tlnp | grep :9001 > /dev/null; then
    echo -e "${RED}❌ Counter服务未在9001端口运行${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Counter服务运行正常${NC}"

if ! ss -tlnp | grep :9002 > /dev/null; then
    echo -e "${RED}❌ Analytics服务未在9002端口运行${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Analytics服务运行正常${NC}"

# 创建gRPC性能测试程序
echo -e "${BLUE}🔧 创建gRPC性能测试程序...${NC}"
cat > test_grpc_performance.go << 'EOF'
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "high-go-press/api/generated/counter"
)

func main() {
	fmt.Println("🚀 4分区Kafka gRPC性能测试")
	fmt.Println("==========================")

	// 连接到Counter服务
	conn, err := grpc.Dial("localhost:9001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("连接Counter服务失败: %v", err)
	}
	defer conn.Close()

	client := pb.NewCounterServiceClient(conn)

	// 测试参数
	duration := 30 * time.Second
	concurrency := 50

	fmt.Printf("测试参数:\n")
	fmt.Printf("  - 测试时长: %v\n", duration)
	fmt.Printf("  - 并发数: %d\n", concurrency)
	fmt.Printf("  - Kafka分区: 4个\n")
	fmt.Printf("  - 目标服务: localhost:9001\n\n")

	// 预热
	fmt.Println("预热系统...")
	for i := 0; i < 10; i++ {
		_, err := client.IncrementCounter(context.Background(), &pb.IncrementRequest{
			ResourceId:  "warmup",
			CounterType: "test",
			Delta:       1,
		})
		if err != nil {
			log.Printf("预热请求失败: %v", err)
		}
	}
	time.Sleep(2 * time.Second)

	// 性能测试
	fmt.Println("开始性能测试...")

	var (
		successCount int64
		errorCount   int64
		wg           sync.WaitGroup
		startTime    = time.Now()
		endTime      = startTime.Add(duration)
	)

	// 启动并发goroutines
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			requestID := 0
			for time.Now().Before(endTime) {
				requestID++

				_, err := client.IncrementCounter(context.Background(), &pb.IncrementRequest{
					ResourceId:  fmt.Sprintf("perf-test-worker-%d-req-%d", workerID, requestID),
					CounterType: "test",
					Delta:       1,
				})

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}

				// 小延迟避免过度压力
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	// 等待所有goroutines完成
	wg.Wait()

	actualDuration := time.Since(startTime)
	totalRequests := successCount + errorCount
	qps := float64(successCount) / actualDuration.Seconds()
	successRate := float64(successCount) / float64(totalRequests) * 100

	// 输出结果
	fmt.Println("\n📊 4分区Kafka gRPC性能测试结果:")
	fmt.Printf("  - 总请求数: %d\n", totalRequests)
	fmt.Printf("  - 成功请求: %d\n", successCount)
	fmt.Printf("  - 失败请求: %d\n", errorCount)
	fmt.Printf("  - 实际耗时: %.2f秒\n", actualDuration.Seconds())
	fmt.Printf("  - QPS: %.2f\n", qps)
	fmt.Printf("  - 成功率: %.2f%%\n", successRate)

	// 与历史数据对比
	fmt.Println("\n📈 性能对比:")
	fmt.Printf("  - Phase 1 (单体): ~21,000 QPS\n")
	fmt.Printf("  - Phase 2 (2分区): ~738 QPS\n")
	fmt.Printf("  - Phase 2 (4分区): %.2f QPS\n", qps)

	if qps > 738 {
		improvement := (qps - 738) / 738 * 100
		fmt.Printf("  - 4分区 vs 2分区 改进: +%.2f%%\n", improvement)
	} else {
		decline := (738 - qps) / 738 * 100
		fmt.Printf("  - 4分区 vs 2分区 下降: -%.2f%%\n", decline)
	}

	fmt.Println("\n✅ 测试完成！")
}
EOF

# 运行gRPC性能测试
echo -e "${BLUE}⚡ 运行gRPC性能测试...${NC}"
go run test_grpc_performance.go

# 检查Kafka分区消息分布
echo -e "${BLUE}📊 检查Kafka分区消息分布...${NC}"
./kafka_2.13-3.9.1/bin/kafka-log-dirs.sh --bootstrap-server localhost:9092 --topic-list counter-events --describe | jq '.brokers[0].logDirs[0].partitions[] | {partition: .partition, size: .size}' 2>/dev/null || echo "无法解析JSON，使用原始输出"

# 清理
echo -e "${BLUE}🧹 清理测试环境...${NC}"
kill $COUNTER_PID $ANALYTICS_PID 2>/dev/null || true
rm -f test_grpc_performance.go
sleep 2

echo -e "${GREEN}🎉 4分区Kafka性能测试完成！${NC}"
echo ""
echo -e "${YELLOW}💡 关键观察点:${NC}"
echo "  1. QPS是否比2分区时有提升"
echo "  2. 消息是否均匀分布到4个分区"
echo "  3. 错误率是否保持在低水平"
echo "  4. 异步处理是否正常工作" 