package handlers

import (
	"context"
	"net/http"
	"time"

	pb "high-go-press/api/proto/counter"
	"high-go-press/internal/biz"
	"high-go-press/internal/gateway/client"
	"high-go-press/pkg/pool"

	"github.com/gin-gonic/gin"
)

// CounterHandler 计数器处理器 - 微服务版本 (使用连接池)
type CounterHandler struct {
	counterClientPool *client.CounterClientPool
	objPool           *pool.ObjectPool
	timeout           time.Duration
}

// NewCounterHandler 创建计数器处理器 - 使用连接池
func NewCounterHandler(counterClientPool *client.CounterClientPool, objPool *pool.ObjectPool) *CounterHandler {
	return &CounterHandler{
		counterClientPool: counterClientPool,
		objPool:           objPool,
		timeout:           5 * time.Second, // 默认5秒超时
	}
}

// IncrementCounter 增量计数器 - HTTP转gRPC (使用连接池)
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

	// 创建gRPC请求上下文
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	// HTTP请求转换为gRPC请求
	grpcReq := &pb.IncrementRequest{
		ResourceId:  req.ResourceID,
		CounterType: req.CounterType,
		Delta:       req.Delta,
	}

	// 调用Counter gRPC服务 - 使用连接池
	grpcResp, err := h.counterClientPool.IncrementCounter(ctx, grpcReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Failed to increment counter",
			"details": err.Error(),
		})
		return
	}

	// 转换gRPC响应为HTTP响应
	resp := &biz.CounterResponse{
		ResourceID:   grpcReq.ResourceId,
		CounterType:  grpcReq.CounterType,
		CurrentValue: grpcResp.CurrentValue,
		Success:      grpcResp.Status.Success,
		Timestamp:    time.Now().Unix(),
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}

// GetCounter 获取计数器 - HTTP转gRPC (使用连接池)
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

	// 创建gRPC请求
	grpcReq := &pb.GetCounterRequest{
		ResourceId:  resourceID,
		CounterType: counterType,
	}

	// 调用Counter gRPC服务 - 使用连接池
	grpcResp, err := h.counterClientPool.GetCounter(ctx, grpcReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Failed to get counter",
			"details": err.Error(),
		})
		return
	}

	// 转换gRPC响应为HTTP响应
	counter := &biz.Counter{
		ResourceID:   resourceID,
		CounterType:  counterType,
		CurrentValue: grpcResp.Value,
		UpdatedAt:    time.Now().Unix(),
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   counter,
	})
}

// BatchGetCounters 批量获取计数器 - HTTP转gRPC (使用连接池)
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

	// 转换HTTP请求为gRPC请求
	grpcRequests := make([]*pb.GetCounterRequest, len(req.Items))
	for i, item := range req.Items {
		grpcRequests[i] = &pb.GetCounterRequest{
			ResourceId:  item.ResourceID,
			CounterType: item.CounterType,
		}
	}

	grpcReq := &pb.BatchGetRequest{
		Requests: grpcRequests,
	}

	// 调用Counter gRPC服务 - 使用连接池
	grpcResp, err := h.counterClientPool.BatchGetCounters(ctx, grpcReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"error":   "Failed to batch get counters",
			"details": err.Error(),
		})
		return
	}

	// 转换gRPC响应为HTTP响应
	results := make([]biz.Counter, len(grpcResp.Counters))
	for i, result := range grpcResp.Counters {
		results[i] = biz.Counter{
			ResourceID:   result.ResourceId,
			CounterType:  result.CounterType,
			CurrentValue: result.Value,
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
