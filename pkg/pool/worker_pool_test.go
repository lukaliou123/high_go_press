package pool

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// 模拟任务负载
func simulateWork(duration time.Duration) {
	time.Sleep(duration)
}

// BenchmarkNativeGoroutines 原生goroutine基准测试
func BenchmarkNativeGoroutines(b *testing.B) {
	workDuration := time.Microsecond * 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup

		// 批量提交任务
		for j := 0; j < 1000; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				simulateWork(workDuration)
			}()
		}

		wg.Wait()
	}
}

// BenchmarkAntsPool 使用ants pool的基准测试
func BenchmarkAntsPool(b *testing.B) {
	logger := zap.NewNop()
	workDuration := time.Microsecond * 100

	// 创建worker pool
	pool, err := NewWorkerPool(logger)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Shutdown(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup

		// 批量提交任务
		for j := 0; j < 1000; j++ {
			wg.Add(1)
			err := pool.SubmitTask(func() {
				defer wg.Done()
				simulateWork(workDuration)
			})
			if err != nil {
				b.Fatal(err)
			}
		}

		wg.Wait()
	}
}

// BenchmarkAntsPoolWithFunc 使用ants PoolWithFunc的基准测试
func BenchmarkAntsPoolWithFunc(b *testing.B) {
	logger := zap.NewNop()

	// 创建worker pool
	pool, err := NewWorkerPool(logger)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Shutdown(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup

		// 批量提交计数任务
		for j := 0; j < 1000; j++ {
			wg.Add(1)
			task := &CounterTask{
				ResourceID:  "article_001",
				CounterType: "like",
				Delta:       1,
				Callback: func(err error) {
					wg.Done()
				},
			}

			err := pool.SubmitCounterTask(task)
			if err != nil {
				b.Fatal(err)
			}
		}

		wg.Wait()
	}
}

// TestWorkerPoolStats 测试池状态统计
func TestWorkerPoolStats(t *testing.T) {
	logger := zap.NewNop()

	pool, err := NewWorkerPool(logger)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Shutdown(context.Background())

	// 提交一些任务
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		pool.SubmitTask(func() {
			defer wg.Done()
			time.Sleep(time.Millisecond * 100)
		})
	}

	// 检查统计信息
	stats := pool.GetStats()
	t.Logf("General Pool - Cap: %d, Running: %d, Waiting: %d, Free: %d",
		stats.GeneralPool.Cap, stats.GeneralPool.Running,
		stats.GeneralPool.Waiting, stats.GeneralPool.Free)

	t.Logf("Counter Pool - Cap: %d, Running: %d, Waiting: %d, Free: %d",
		stats.CounterPool.Cap, stats.CounterPool.Running,
		stats.CounterPool.Waiting, stats.CounterPool.Free)

	wg.Wait()
}

// TestWorkerPoolShutdown 测试优雅关闭
func TestWorkerPoolShutdown(t *testing.T) {
	logger := zap.NewNop()

	pool, err := NewWorkerPool(logger)
	if err != nil {
		t.Fatal(err)
	}

	// 提交一些长时间任务
	for i := 0; i < 5; i++ {
		pool.SubmitTask(func() {
			time.Sleep(time.Millisecond * 200)
		})
	}

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	err = pool.Shutdown(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// 关闭后提交任务应该失败
	err = pool.SubmitTask(func() {})
	if err != ErrPoolClosed {
		t.Errorf("Expected ErrPoolClosed, got: %v", err)
	}
}
