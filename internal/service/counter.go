package service

import (
	"context"
	"fmt"
	"time"

	"high-go-press/internal/biz"
	"high-go-press/internal/dao"
	"high-go-press/pkg/kafka"
	"high-go-press/pkg/logger"
	"high-go-press/pkg/pool"

	"go.uber.org/zap"
)

type CounterService struct {
	dao        *dao.RedisRepo
	workerPool *pool.WorkerPool
	objectPool *pool.ObjectPool
	producer   kafka.Producer
	logger     *zap.Logger
}

func NewCounterService(
	dao *dao.RedisRepo,
	workerPool *pool.WorkerPool,
	objectPool *pool.ObjectPool,
	producer kafka.Producer,
	logger *zap.Logger,
) biz.CounterService {
	return &CounterService{
		dao:        dao,
		workerPool: workerPool,
		objectPool: objectPool,
		producer:   producer,
		logger:     logger,
	}
}

func (s *CounterService) IncrementCounter(req *biz.IncrementRequest) (*biz.CounterResponse, error) {
	// 使用对象池获取响应对象
	resp := s.objectPool.GetCounterResponse()
	defer s.objectPool.PutCounterResponse(resp)

	// 参数验证
	if req.ResourceID == "" || req.CounterType == "" {
		resp.Success = false
		resp.Message = "resource_id and counter_type are required"
		return resp, fmt.Errorf("invalid parameters")
	}

	// 默认增量为1
	if req.Delta == 0 {
		req.Delta = 1
	}

	// 构建Redis key
	key := fmt.Sprintf("counter:%s:%s", req.ResourceID, req.CounterType)

	// 执行计数器增量操作
	ctx := context.Background()
	newValue, err := s.dao.IncrementCounter(ctx, key, req.Delta)
	if err != nil {
		s.logger.Error("Failed to increment counter",
			zap.String("resource_id", req.ResourceID),
			zap.String("counter_type", req.CounterType),
			zap.Int64("delta", req.Delta),
			zap.Error(err))

		resp.Success = false
		resp.Message = "Failed to increment counter"
		return resp, err
	}

	// 异步发送Kafka事件
	go func() {
		event := &kafka.CounterEvent{
			EventID:     fmt.Sprintf("%s-%d", key, time.Now().UnixNano()),
			ResourceID:  req.ResourceID,
			CounterType: req.CounterType,
			Delta:       req.Delta,
			NewValue:    newValue,
			Timestamp:   time.Now(),
			Source:      "API",
		}

		if err := s.producer.SendCounterEvent(context.Background(), event); err != nil {
			s.logger.Error("Failed to send counter event to kafka", zap.Error(err))
		}
	}()

	// 构建响应
	result := biz.NewCounterResponse(req.ResourceID, req.CounterType, newValue, true, "")
	return result, nil
}

func (s *CounterService) GetCounter(resourceID, counterType string) (*biz.Counter, error) {
	// 参数验证
	if resourceID == "" || counterType == "" {
		return nil, fmt.Errorf("resource_id and counter_type are required")
	}

	// 构建Redis key
	key := fmt.Sprintf("counter:%s:%s", resourceID, counterType)

	// 获取计数器值
	ctx := context.Background()
	value, err := s.dao.GetCounter(ctx, key)
	if err != nil {
		s.logger.Error("Failed to get counter",
			zap.String("resource_id", resourceID),
			zap.String("counter_type", counterType),
			zap.Error(err))
		return nil, err
	}

	counter := biz.NewCounter(resourceID, counterType, value)
	return counter, nil
}

func (s *CounterService) BatchGetCounters(req *biz.BatchRequest) (*biz.BatchResponse, error) {
	if len(req.Items) == 0 {
		return &biz.BatchResponse{
			Results: []biz.Counter{},
			Total:   0,
		}, nil
	}

	// 使用对象池获取字符串切片
	keys := s.objectPool.GetStringSlice()
	defer s.objectPool.PutStringSlice(keys)

	// 构建所有keys
	itemToKey := make(map[string]*biz.BatchItem)
	for i := range req.Items {
		item := &req.Items[i]
		if item.ResourceID == "" || item.CounterType == "" {
			continue
		}
		key := fmt.Sprintf("counter:%s:%s", item.ResourceID, item.CounterType)
		*keys = append(*keys, key)
		itemToKey[key] = item
	}

	// 批量获取计数器值
	ctx := context.Background()
	counts, err := s.dao.GetMultiCounters(ctx, *keys)
	if err != nil {
		s.logger.Error("Failed to batch get counters", zap.Error(err))
		return nil, err
	}

	// 构建响应
	results := make([]biz.Counter, 0, len(req.Items))
	for _, key := range *keys {
		item := itemToKey[key]
		if item == nil {
			continue
		}

		value := counts[key] // 如果key不存在，会返回0值
		counter := *biz.NewCounter(item.ResourceID, item.CounterType, value)
		results = append(results, counter)
	}

	return &biz.BatchResponse{
		Results: results,
		Total:   len(results),
	}, nil
}

func (s *CounterService) GetHotRank(ctx context.Context, query *biz.HotRankQuery) ([]*biz.HotRankItem, error) {
	// 注意：这是一个简化版的热点排行实现
	// 在真实场景中，应该使用Redis的ZSET或专门的排行榜数据结构
	// 这里为了演示，我们暂时返回空列表

	logger.Info("GetHotRank called",
		zap.String("counter_type", string(query.CounterType)),
		zap.Int("limit", query.Limit),
		zap.String("period", query.Period))

	// TODO: 实现真正的热点排行逻辑
	// 1. 可以使用Redis ZSET存储排行榜数据
	// 2. 或者通过定时任务计算热点数据并缓存
	// 3. 这里先返回空数组

	return []*biz.HotRankItem{}, nil
}
