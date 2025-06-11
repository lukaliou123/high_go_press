package dao

import (
	"context"
	"time"
)

// CounterStats 计数器统计数据
type CounterStats struct {
	ResourceID  string
	CounterType string
	Total       int64
	Average     float64
	Peak        int64
	TimeSeries  []TimeSeriesPoint
	LastUpdated time.Time
}

// TimeSeriesPoint 时间序列数据点
type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
}

// CounterItem 计数器项目
type CounterItem struct {
	ResourceID     string
	CounterType    string
	Value          int64
	IncrementCount int64
	LastUpdated    time.Time
}

// AnalyticsDAO Analytics数据访问接口
type AnalyticsDAO interface {
	// GetTopCounters 获取热门计数器排行榜
	GetTopCounters(ctx context.Context, counterType, timeRange string, limit int) ([]*CounterItem, error)

	// GetCounterStats 获取计数器统计信息
	GetCounterStats(ctx context.Context, resourceID, counterType, timeRange string) (*CounterStats, error)

	// UpdateCounterStats 更新计数器统计数据（从Kafka事件）
	UpdateCounterStats(ctx context.Context, resourceID, counterType string, delta int64) error

	// GetCounterHistory 获取计数器历史数据
	GetCounterHistory(ctx context.Context, resourceID, counterType, timeRange string) ([]TimeSeriesPoint, error)
}

// MemoryAnalyticsDAO 内存版本DAO（用于开发测试）
type MemoryAnalyticsDAO struct {
	counters   map[string]*CounterItem
	timeSeries map[string][]TimeSeriesPoint
}

// NewMemoryAnalyticsDAO 创建内存版DAO
func NewMemoryAnalyticsDAO() *MemoryAnalyticsDAO {
	return &MemoryAnalyticsDAO{
		counters:   make(map[string]*CounterItem),
		timeSeries: make(map[string][]TimeSeriesPoint),
	}
}

// GetTopCounters 获取热门计数器排行榜
func (dao *MemoryAnalyticsDAO) GetTopCounters(ctx context.Context, counterType, timeRange string, limit int) ([]*CounterItem, error) {
	// 模拟热门数据
	mockCounters := []*CounterItem{
		{
			ResourceID:     "article_123",
			CounterType:    counterType,
			Value:          1500,
			IncrementCount: 150,
			LastUpdated:    time.Now().Add(-time.Hour),
		},
		{
			ResourceID:     "article_456",
			CounterType:    counterType,
			Value:          1200,
			IncrementCount: 120,
			LastUpdated:    time.Now().Add(-30 * time.Minute),
		},
		{
			ResourceID:     "article_789",
			CounterType:    counterType,
			Value:          980,
			IncrementCount: 98,
			LastUpdated:    time.Now().Add(-15 * time.Minute),
		},
		{
			ResourceID:     "article_321",
			CounterType:    counterType,
			Value:          750,
			IncrementCount: 75,
			LastUpdated:    time.Now().Add(-45 * time.Minute),
		},
		{
			ResourceID:     "article_654",
			CounterType:    counterType,
			Value:          620,
			IncrementCount: 62,
			LastUpdated:    time.Now().Add(-2 * time.Hour),
		},
	}

	if limit > 0 && limit < len(mockCounters) {
		return mockCounters[:limit], nil
	}

	return mockCounters, nil
}

// GetCounterStats 获取计数器统计信息
func (dao *MemoryAnalyticsDAO) GetCounterStats(ctx context.Context, resourceID, counterType, timeRange string) (*CounterStats, error) {
	// 模拟统计数据
	now := time.Now()
	timeSeries := []TimeSeriesPoint{
		{Timestamp: now.Add(-4 * time.Hour), Value: 100},
		{Timestamp: now.Add(-3 * time.Hour), Value: 250},
		{Timestamp: now.Add(-2 * time.Hour), Value: 400},
		{Timestamp: now.Add(-1 * time.Hour), Value: 720},
		{Timestamp: now, Value: 1000},
	}

	return &CounterStats{
		ResourceID:  resourceID,
		CounterType: counterType,
		Total:       1000,
		Average:     200.0,
		Peak:        1000,
		TimeSeries:  timeSeries,
		LastUpdated: now,
	}, nil
}

// UpdateCounterStats 更新计数器统计数据
func (dao *MemoryAnalyticsDAO) UpdateCounterStats(ctx context.Context, resourceID, counterType string, delta int64) error {
	key := resourceID + ":" + counterType

	if counter, exists := dao.counters[key]; exists {
		counter.Value += delta
		counter.IncrementCount++
		counter.LastUpdated = time.Now()
	} else {
		dao.counters[key] = &CounterItem{
			ResourceID:     resourceID,
			CounterType:    counterType,
			Value:          delta,
			IncrementCount: 1,
			LastUpdated:    time.Now(),
		}
	}

	// 添加时间序列数据点
	tsKey := key + ":timeseries"
	dao.timeSeries[tsKey] = append(dao.timeSeries[tsKey], TimeSeriesPoint{
		Timestamp: time.Now(),
		Value:     float64(delta),
	})

	return nil
}

// GetCounterHistory 获取计数器历史数据
func (dao *MemoryAnalyticsDAO) GetCounterHistory(ctx context.Context, resourceID, counterType, timeRange string) ([]TimeSeriesPoint, error) {
	key := resourceID + ":" + counterType + ":timeseries"

	if series, exists := dao.timeSeries[key]; exists {
		return series, nil
	}

	return []TimeSeriesPoint{}, nil
}
