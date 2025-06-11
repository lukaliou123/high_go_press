package handlers

import (
	"context"
	"net/http"
	"time"

	"high-go-press/internal/biz"
	"high-go-press/internal/gateway/client"
	"high-go-press/pkg/pool"

	"github.com/gin-gonic/gin"
)

// CounterHandler 计数器处理器 - 微服务版本
type CounterHandler struct {
	counterClient *client.CounterClient
	objPool       *pool.ObjectPool
	timeout       time.Duration
}

// NewCounterHandler 创建计数器处理器
func NewCounterHandler(counterClient *client.CounterClient, objPool *pool.ObjectPool) *CounterHandler {
	return &CounterHandler{
		counterClient: counterClient,
		objPool:       objPool,
		timeout:       5 * time.Second, // 默认5秒超时
	}
}

// IncrementCounter 增量计数器 - HTTP转gRPC
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

	// 创建gRPC请求上下文（为后续gRPC调用准备）
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	_ = ctx // 暂时忽略，等待protobuf完成后使用

	// HTTP请求转换为gRPC请求（暂时用简化的结构）
	grpcReq := &struct {
		ResourceId  string
		CounterType string
		Delta       int64
	}{
		ResourceId:  req.ResourceID,
		CounterType: req.CounterType,
		Delta:       req.Delta,
	}

	// 暂时创建模拟的响应，等待protobuf编译完成后替换
	resp := &biz.CounterResponse{
		ResourceID:   grpcReq.ResourceId,
		CounterType:  grpcReq.CounterType,
		CurrentValue: grpcReq.Delta, // 简化处理
		Success:      true,
		Timestamp:    time.Now().Unix(),
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}

// GetCounter 获取计数器 - HTTP转gRPC
func (h *CounterHandler) GetCounter(c *gin.Context) {
	resourceID := c.Param("resource_id")
	counterType := c.Param("counter_type")

	if resourceID == "" || counterType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "resource_id and counter_type are required",
		})
		return
	}

	// 创建gRPC请求上下文
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	_ = ctx // 暂时忽略，等待protobuf完成后使用

	// 暂时创建模拟响应
	counter := &biz.Counter{
		ResourceID:   resourceID,
		CounterType:  counterType,
		CurrentValue: 0, // 默认值
		UpdatedAt:    time.Now().Unix(),
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   counter,
	})
}

// BatchGetCounters 批量获取计数器 - HTTP转gRPC
func (h *CounterHandler) BatchGetCounters(c *gin.Context) {
	req := new(biz.BatchRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// 创建gRPC请求上下文
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	_ = ctx // 暂时忽略，等待protobuf完成后使用

	// 构建响应数据
	results := make([]biz.Counter, len(req.Items))
	for i, item := range req.Items {
		results[i] = biz.Counter{
			ResourceID:   item.ResourceID,
			CounterType:  item.CounterType,
			CurrentValue: 0, // 默认值
			UpdatedAt:    time.Now().Unix(),
		}
	}

	resp := &biz.BatchResponse{
		Results: results,
		Total:   len(results),
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}
