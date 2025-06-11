package handlers

import (
	"net/http"

	"high-go-press/internal/biz"
	"high-go-press/pkg/pool"

	"github.com/gin-gonic/gin"
)

// CounterHandler 计数器处理器
type CounterHandler struct {
	counterService biz.CounterService
	objPool        *pool.ObjectPool
}

// NewCounterHandler 创建计数器处理器
func NewCounterHandler(counterService biz.CounterService, objPool *pool.ObjectPool) *CounterHandler {
	return &CounterHandler{
		counterService: counterService,
		objPool:        objPool,
	}
}

// IncrementCounter 增量计数器
func (h *CounterHandler) IncrementCounter(c *gin.Context) {
	req := h.objPool.GetIncrementRequest()
	defer h.objPool.PutIncrementRequest(req)

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// 设置默认增量
	if req.Delta == 0 {
		req.Delta = 1
	}

	resp, err := h.counterService.IncrementCounter(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to increment counter",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}

// GetCounter 获取计数器
func (h *CounterHandler) GetCounter(c *gin.Context) {
	resourceID := c.Param("resource_id")
	counterType := c.Param("counter_type")

	if resourceID == "" || counterType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "resource_id and counter_type are required",
		})
		return
	}

	counter, err := h.counterService.GetCounter(resourceID, counterType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get counter",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   counter,
	})
}

// BatchGetCounters 批量获取计数器
func (h *CounterHandler) BatchGetCounters(c *gin.Context) {
	req := new(biz.BatchRequest)                  // 使用new来获取指针，而不是声明变量
	if err := c.ShouldBindJSON(req); err != nil { // 直接传递指针
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	resp, err := h.counterService.BatchGetCounters(req) // req本身已经是地址
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to batch get counters",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}
