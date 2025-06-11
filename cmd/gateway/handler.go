package main

import (
	"net/http"
	"strconv"

	"high-go-press/internal/biz"
	"high-go-press/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	counterUseCase biz.CounterUseCase
}

func NewHandler(counterUseCase biz.CounterUseCase) *Handler {
	return &Handler{
		counterUseCase: counterUseCase,
	}
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Data    interface{} `json:"data"`
	Success bool        `json:"success"`
}

// incrementCounter 增加计数器
// POST /api/v1/counter/increment
func (h *Handler) incrementCounter(c *gin.Context) {
	var req biz.CounterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request format",
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	resp, err := h.counterUseCase.Increment(c.Request.Context(), &req)
	if err != nil {
		logger.Error("Failed to increment counter",
			zap.String("resource_id", req.ResourceID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Internal server error",
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Data:    resp,
		Success: true,
	})
}

// getCounter 获取计数器值
// GET /api/v1/counter/:resource_id/:counter_type
func (h *Handler) getCounter(c *gin.Context) {
	resourceID := c.Param("resource_id")
	counterTypeStr := c.Param("counter_type")

	if resourceID == "" || counterTypeStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid parameters",
			Code:    400,
			Message: "resource_id and counter_type are required",
		})
		return
	}

	query := &biz.CounterQuery{
		ResourceID:  resourceID,
		CounterType: biz.CounterType(counterTypeStr),
	}

	resp, err := h.counterUseCase.Get(c.Request.Context(), query)
	if err != nil {
		logger.Error("Failed to get counter",
			zap.String("resource_id", resourceID),
			zap.String("counter_type", counterTypeStr),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Internal server error",
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Data:    resp,
		Success: true,
	})
}

// BatchGetRequest 批量获取请求
type BatchGetRequest struct {
	Queries []*biz.CounterQuery `json:"queries" binding:"required"`
}

// batchGetCounters 批量获取计数器值
// POST /api/v1/counter/batch
func (h *Handler) batchGetCounters(c *gin.Context) {
	var req BatchGetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Failed to bind batch request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request format",
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	resp, err := h.counterUseCase.GetBatch(c.Request.Context(), req.Queries)
	if err != nil {
		logger.Error("Failed to batch get counters", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Internal server error",
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Data:    resp,
		Success: true,
	})
}

// getHotRank 获取热点排行
// GET /api/v1/counter/hot/:counter_type?limit=10&period=day
func (h *Handler) getHotRank(c *gin.Context) {
	counterTypeStr := c.Param("counter_type")
	if counterTypeStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid parameters",
			Code:    400,
			Message: "counter_type is required",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	period := c.DefaultQuery("period", "day")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	query := &biz.HotRankQuery{
		CounterType: biz.CounterType(counterTypeStr),
		Limit:       limit,
		Period:      period,
	}

	resp, err := h.counterUseCase.GetHotRank(c.Request.Context(), query)
	if err != nil {
		logger.Error("Failed to get hot rank",
			zap.String("counter_type", counterTypeStr),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Internal server error",
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Data:    resp,
		Success: true,
	})
}

// health 健康检查
// GET /health
func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "high-go-press",
		"version": "1.0.0",
	})
}

// setupRoutes 设置路由
func (h *Handler) setupRoutes(r *gin.Engine) {
	// 健康检查
	r.GET("/health", h.health)

	// API v1
	v1 := r.Group("/api/v1")
	{
		counter := v1.Group("/counter")
		{
			counter.POST("/increment", h.incrementCounter)
			counter.GET("/:resource_id/:counter_type", h.getCounter)
			counter.POST("/batch", h.batchGetCounters)
			counter.GET("/hot/:counter_type", h.getHotRank)
		}
	}
}
