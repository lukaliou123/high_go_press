package handlers

import (
	"net/http"
	"time"

	"high-go-press/pkg/pool"

	"github.com/gin-gonic/gin"
)

// PoolHandler Worker Pool处理器
type PoolHandler struct {
	workerPool *pool.WorkerPool
}

// NewPoolHandler 创建Pool处理器
func NewPoolHandler(workerPool *pool.WorkerPool) *PoolHandler {
	return &PoolHandler{
		workerPool: workerPool,
	}
}

// GetPoolStats 获取Worker Pool统计信息
func (h *PoolHandler) GetPoolStats(c *gin.Context) {
	stats := h.workerPool.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"data":      stats,
		"timestamp": time.Now().Unix(),
	})
}

// TestWorkerPool 测试Worker Pool
func (h *PoolHandler) TestWorkerPool(c *gin.Context) {
	// 简单的测试：提交一些任务到worker pool
	taskCount := 10

	for i := 0; i < taskCount; i++ {
		h.workerPool.SubmitTask(func() {
			// 模拟一些工作
			time.Sleep(time.Millisecond * 10)
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "success",
		"message":         "Worker pool test completed",
		"tasks_submitted": taskCount,
	})
}

// calculateUsage 计算使用率
func calculateUsage(pool struct {
	Cap     int32
	Running int32
	Waiting int32
	Free    int32
}) float64 {
	if pool.Cap == 0 {
		return 0
	}
	return float64(pool.Running) / float64(pool.Cap) * 100
}
