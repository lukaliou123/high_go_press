package pool

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

// WorkerPool 封装ants池的管理器
type WorkerPool struct {
	// 通用任务池 - 用于处理各种异步任务
	generalPool *ants.Pool

	// 专用计数池 - 使用PoolWithFunc优化计数操作
	counterPool *ants.PoolWithFunc

	logger *zap.Logger
	mu     sync.RWMutex
	closed bool
}

// CounterTask 计数任务结构
type CounterTask struct {
	ResourceID  string
	CounterType string
	Delta       int64
	Callback    func(error)
}

// NewWorkerPool 创建worker pool管理器
func NewWorkerPool(logger *zap.Logger) (*WorkerPool, error) {
	// 获取CPU核心数
	numCPU := runtime.NumCPU()
	if numCPU == 0 {
		numCPU = 4 // 默认值
	}

	// 创建通用任务池，设置合理的大小
	generalPoolSize := numCPU * 200
	generalPool, err := ants.NewPool(generalPoolSize,
		ants.WithOptions(ants.Options{
			ExpiryDuration: 10 * time.Second, // 空闲10秒后回收goroutine
			Nonblocking:    false,            // 阻塞模式，保证任务不丢失
			PreAlloc:       false,            // 在WSL环境下，预分配可能导致问题，改为false
		}))
	if err != nil {
		return nil, err
	}

	wp := &WorkerPool{
		generalPool: generalPool,
		logger:      logger,
	}

	// 创建专用计数池
	counterPoolSize := numCPU * 100
	counterPool, err := ants.NewPoolWithFunc(counterPoolSize, wp.executeCounterTask,
		ants.WithOptions(ants.Options{
			ExpiryDuration: 10 * time.Second,
			Nonblocking:    false,
			PreAlloc:       false, // 同上
		}))
	if err != nil {
		generalPool.Release()
		return nil, err
	}

	wp.counterPool = counterPool

	logger.Info("Worker pool initialized",
		zap.Int("general_pool_cap", generalPool.Cap()),
		zap.Int("counter_pool_cap", counterPool.Cap()),
		zap.Int("cpus", numCPU))

	return wp, nil
}

// SubmitTask 提交通用异步任务
func (wp *WorkerPool) SubmitTask(task func()) error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if wp.closed {
		return ErrPoolClosed
	}

	return wp.generalPool.Submit(task)
}

// SubmitCounterTask 提交计数任务（高性能优化）
func (wp *WorkerPool) SubmitCounterTask(task *CounterTask) error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if wp.closed {
		return ErrPoolClosed
	}

	return wp.counterPool.Invoke(task)
}

// executeCounterTask 执行计数任务（PoolWithFunc的回调）
func (wp *WorkerPool) executeCounterTask(payload interface{}) {
	task, ok := payload.(*CounterTask)
	if !ok {
		wp.logger.Error("Invalid counter task payload")
		return
	}

	start := time.Now()

	// 这里暂时只是模拟，实际应该调用Redis操作
	// 在后续集成时会替换为真实的计数逻辑
	err := wp.simulateCounterOperation(task)

	duration := time.Since(start)

	if task.Callback != nil {
		task.Callback(err)
	}

	if err != nil {
		wp.logger.Error("Counter task failed",
			zap.String("resource_id", task.ResourceID),
			zap.String("counter_type", task.CounterType),
			zap.Error(err),
			zap.Duration("duration", duration))
	} else {
		wp.logger.Debug("Counter task completed",
			zap.String("resource_id", task.ResourceID),
			zap.String("counter_type", task.CounterType),
			zap.Duration("duration", duration))
	}
}

// simulateCounterOperation 模拟计数操作
func (wp *WorkerPool) simulateCounterOperation(task *CounterTask) error {
	// 模拟一些计算和I/O耗时
	time.Sleep(time.Microsecond * 100)
	return nil
}

// GetStats 获取池状态统计
func (wp *WorkerPool) GetStats() PoolStats {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	return PoolStats{
		GeneralPool: PoolStat{
			Cap:     wp.generalPool.Cap(),
			Running: wp.generalPool.Running(),
			Waiting: wp.generalPool.Waiting(),
			Free:    wp.generalPool.Free(),
		},
		CounterPool: PoolStat{
			Cap:     wp.counterPool.Cap(),
			Running: wp.counterPool.Running(),
			Waiting: wp.counterPool.Waiting(),
			Free:    wp.counterPool.Free(),
		},
	}
}

// Shutdown 优雅关闭worker pool
func (wp *WorkerPool) Shutdown(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.closed {
		return nil
	}

	wp.closed = true

	// 关闭池，等待任务完成
	wp.generalPool.Release()
	wp.counterPool.Release()

	wp.logger.Info("Worker pool shutdown completed")
	return nil
}

// PoolStats 池统计信息
type PoolStats struct {
	GeneralPool PoolStat `json:"general_pool"`
	CounterPool PoolStat `json:"counter_pool"`
}

// PoolStat 单个池的统计
type PoolStat struct {
	Cap     int `json:"capacity"`
	Running int `json:"running"`
	Waiting int `json:"waiting"`
	Free    int `json:"free"`
}

// 错误定义
var (
	ErrPoolClosed = fmt.Errorf("worker pool is closed")
)
