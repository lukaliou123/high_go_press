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

	pb "high-go-press/api/proto/counter"
)

func main() {
	fmt.Println("🚀 HighGoPress gRPC性能测试 (真实Kafka)")
	fmt.Println("=====================================")

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
	fmt.Printf("  - 目标服务: localhost:9001 (Counter gRPC)\n")
	fmt.Printf("  - Kafka模式: 真实Kafka\n\n")

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
					log.Printf("Worker %d 请求失败: %v", workerID, err)
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
	fmt.Println("\n📊 真实Kafka gRPC性能测试结果:")
	fmt.Printf("  - 总请求数: %d\n", totalRequests)
	fmt.Printf("  - 成功请求: %d\n", successCount)
	fmt.Printf("  - 失败请求: %d\n", errorCount)
	fmt.Printf("  - 实际耗时: %.2f秒\n", actualDuration.Seconds())
	fmt.Printf("  - QPS: %.2f\n", qps)
	fmt.Printf("  - 成功率: %.2f%%\n", successRate)

	// 与历史数据对比
	fmt.Println("\n📈 性能对比:")
	fmt.Printf("  - Phase 1 (单体): ~21,000 QPS\n")
	fmt.Printf("  - Phase 2 (Mock Kafka): ~738 QPS\n")
	fmt.Printf("  - Phase 2 (Real Kafka gRPC): %.2f QPS\n", qps)

	if qps > 738 {
		improvement := (qps - 738) / 738 * 100
		fmt.Printf("  - Real vs Mock 改进: +%.2f%%\n", improvement)
	} else {
		decline := (738 - qps) / 738 * 100
		fmt.Printf("  - Real vs Mock 下降: -%.2f%%\n", decline)
	}

	fmt.Println("\n✅ 测试完成！")
}
