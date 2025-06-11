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
	ResourceID   string      `json:"resource_id" binding:"required"`   // 资源ID（如文章ID、用户ID）
	CounterType  CounterType `json:"counter_type" binding:"required"`  // 计数类型
	UserID       string      `json:"user_id" binding:"required"`       // 用户ID
	Increment    int64       `json:"increment"`                        // 增量，默认为1
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
	Limit       int         `json:"limit"`    // 限制返回数量
	Period      string      `json:"period"`   // 时间范围: hour, day, week
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
	Count       int64       `json:"count"`         // 操作后的计数值
	Timestamp   time.Time   `json:"timestamp"`
} 