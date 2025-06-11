package biz

import (
	"context"
	"time"
)

// CounterType 计数器类型
type CounterType string

const (
	CounterTypeLike   CounterType = "like"   // 点赞
	CounterTypeView   CounterType = "view"   // 浏览
	CounterTypeFollow CounterType = "follow" // 关注
)

// CounterReq 计数请求
type CounterReq struct {
	ResourceID  string      `json:"resource_id" binding:"required"`  // 资源ID（如文章ID、用户ID）
	CounterType CounterType `json:"counter_type" binding:"required"` // 计数类型
	UserID      string      `json:"user_id" binding:"required"`      // 用户ID
	Increment   int64       `json:"increment"`                       // 增量，默认为1
}

// CounterResp 计数响应
type CounterResp struct {
	ResourceID  string      `json:"resource_id"`
	CounterType CounterType `json:"counter_type"`
	Count       int64       `json:"count"`
	Success     bool        `json:"success"`
}

// CounterQuery 查询请求
type CounterQuery struct {
	ResourceID  string      `json:"resource_id"`
	CounterType CounterType `json:"counter_type"`
}

// HotRankQuery 热点排行查询
type HotRankQuery struct {
	CounterType CounterType `json:"counter_type"`
	Limit       int         `json:"limit"`  // 限制返回数量
	Period      string      `json:"period"` // 时间范围: hour, day, week
}

// HotRankItem 热点排行项
type HotRankItem struct {
	ResourceID  string      `json:"resource_id"`
	CounterType CounterType `json:"counter_type"`
	Count       int64       `json:"count"`
	Rank        int         `json:"rank"`
}

// CounterUseCase 计数器业务用例接口
type CounterUseCase interface {
	// Increment 增加计数
	Increment(ctx context.Context, req *CounterReq) (*CounterResp, error)

	// Get 获取计数
	Get(ctx context.Context, query *CounterQuery) (*CounterResp, error)

	// GetBatch 批量获取计数
	GetBatch(ctx context.Context, queries []*CounterQuery) ([]*CounterResp, error)

	// GetHotRank 获取热点排行
	GetHotRank(ctx context.Context, query *HotRankQuery) ([]*HotRankItem, error)
}

// CounterRepo 计数器数据仓库接口
type CounterRepo interface {
	// IncrementCounter 增加计数器
	IncrementCounter(ctx context.Context, key string, increment int64) (int64, error)

	// GetCounter 获取计数器值
	GetCounter(ctx context.Context, key string) (int64, error)

	// GetMultiCounters 批量获取计数器
	GetMultiCounters(ctx context.Context, keys []string) (map[string]int64, error)

	// SetCounter 设置计数器值（用于恢复等场景）
	SetCounter(ctx context.Context, key string, value int64) error
}

// buildCounterKey 构建计数器的Redis key
func BuildCounterKey(resourceID string, counterType CounterType) string {
	return "counter:" + string(counterType) + ":" + resourceID
}

// Event 事件定义（用于Kafka）
type CounterEvent struct {
	ResourceID  string      `json:"resource_id"`
	CounterType CounterType `json:"counter_type"`
	UserID      string      `json:"user_id"`
	Increment   int64       `json:"increment"`
	Count       int64       `json:"count"` // 操作后的计数值
	Timestamp   time.Time   `json:"timestamp"`
}

// Counter 计数器实体
type Counter struct {
	ResourceID   string `json:"resource_id"`
	CounterType  string `json:"counter_type"`
	CurrentValue int64  `json:"current_value"`
	UpdatedAt    int64  `json:"updated_at"`
}

// IncrementRequest 增量请求
type IncrementRequest struct {
	ResourceID  string `json:"resource_id" binding:"required"`
	CounterType string `json:"counter_type" binding:"required"`
	Delta       int64  `json:"delta,omitempty"`
}

// CounterResponse 计数器响应
type CounterResponse struct {
	ResourceID   string `json:"resource_id"`
	CounterType  string `json:"counter_type"`
	CurrentValue int64  `json:"current_value"`
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	Timestamp    int64  `json:"timestamp"`
}

// BatchRequest 批量查询请求
type BatchRequest struct {
	Items []BatchItem `json:"items" binding:"required,dive"`
}

// BatchItem 批量查询项
type BatchItem struct {
	ResourceID  string `json:"resource_id" binding:"required"`
	CounterType string `json:"counter_type" binding:"required"`
}

// BatchResponse 批量查询响应
type BatchResponse struct {
	Results []Counter `json:"results"`
	Total   int       `json:"total"`
}

// NewCounterResponse 创建计数器响应
func NewCounterResponse(resourceID, counterType string, value int64, success bool, message string) *CounterResponse {
	return &CounterResponse{
		ResourceID:   resourceID,
		CounterType:  counterType,
		CurrentValue: value,
		Success:      success,
		Message:      message,
		Timestamp:    time.Now().Unix(),
	}
}

// NewCounter 创建计数器实体
func NewCounter(resourceID, counterType string, value int64) *Counter {
	return &Counter{
		ResourceID:   resourceID,
		CounterType:  counterType,
		CurrentValue: value,
		UpdatedAt:    time.Now().Unix(),
	}
}

// CounterService 计数器服务接口
type CounterService interface {
	IncrementCounter(req *IncrementRequest) (*CounterResponse, error)
	GetCounter(resourceID, counterType string) (*Counter, error)
	BatchGetCounters(req *BatchRequest) (*BatchResponse, error)
}
