package service

import (
	"context"
	"fmt"
	"high-go-press/internal/biz"
	"high-go-press/pkg/logger"

	"go.uber.org/zap"
)

type CounterService struct {
	repo biz.CounterRepo
}

func NewCounterService(repo biz.CounterRepo) biz.CounterUseCase {
	return &CounterService{
		repo: repo,
	}
}

func (s *CounterService) Increment(ctx context.Context, req *biz.CounterReq) (*biz.CounterResp, error) {
	// 参数验证
	if req.ResourceID == "" || req.UserID == "" {
		return nil, fmt.Errorf("resource_id and user_id are required")
	}

	// 默认增量为1
	if req.Increment == 0 {
		req.Increment = 1
	}

	// 构建Redis key
	key := biz.BuildCounterKey(req.ResourceID, req.CounterType)

	// 执行计数器增量操作
	count, err := s.repo.IncrementCounter(ctx, key, req.Increment)
	if err != nil {
		logger.Error("Failed to increment counter in service",
			zap.String("resource_id", req.ResourceID),
			zap.String("counter_type", string(req.CounterType)),
			zap.String("user_id", req.UserID),
			zap.Int64("increment", req.Increment),
			zap.Error(err))
		return &biz.CounterResp{
			ResourceID:  req.ResourceID,
			CounterType: req.CounterType,
			Success:     false,
		}, err
	}

	logger.Info("Counter incremented successfully",
		zap.String("resource_id", req.ResourceID),
		zap.String("counter_type", string(req.CounterType)),
		zap.String("user_id", req.UserID),
		zap.Int64("increment", req.Increment),
		zap.Int64("new_count", count))

	return &biz.CounterResp{
		ResourceID:  req.ResourceID,
		CounterType: req.CounterType,
		Count:       count,
		Success:     true,
	}, nil
}

func (s *CounterService) Get(ctx context.Context, query *biz.CounterQuery) (*biz.CounterResp, error) {
	// 参数验证
	if query.ResourceID == "" {
		return nil, fmt.Errorf("resource_id is required")
	}

	// 构建Redis key
	key := biz.BuildCounterKey(query.ResourceID, query.CounterType)

	// 获取计数器值
	count, err := s.repo.GetCounter(ctx, key)
	if err != nil {
		logger.Error("Failed to get counter in service",
			zap.String("resource_id", query.ResourceID),
			zap.String("counter_type", string(query.CounterType)),
			zap.Error(err))
		return &biz.CounterResp{
			ResourceID:  query.ResourceID,
			CounterType: query.CounterType,
			Success:     false,
		}, err
	}

	return &biz.CounterResp{
		ResourceID:  query.ResourceID,
		CounterType: query.CounterType,
		Count:       count,
		Success:     true,
	}, nil
}

func (s *CounterService) GetBatch(ctx context.Context, queries []*biz.CounterQuery) ([]*biz.CounterResp, error) {
	if len(queries) == 0 {
		return []*biz.CounterResp{}, nil
	}

	// 构建所有keys
	keys := make([]string, 0, len(queries))
	keyToQuery := make(map[string]*biz.CounterQuery)

	for _, query := range queries {
		if query.ResourceID == "" {
			continue
		}
		key := biz.BuildCounterKey(query.ResourceID, query.CounterType)
		keys = append(keys, key)
		keyToQuery[key] = query
	}

	// 批量获取计数器值
	counts, err := s.repo.GetMultiCounters(ctx, keys)
	if err != nil {
		logger.Error("Failed to get batch counters in service", zap.Error(err))
		return nil, err
	}

	// 构建响应
	responses := make([]*biz.CounterResp, 0, len(queries))
	for _, query := range queries {
		if query.ResourceID == "" {
			continue
		}

		key := biz.BuildCounterKey(query.ResourceID, query.CounterType)
		count := counts[key] // 如果key不存在，会返回0值

		responses = append(responses, &biz.CounterResp{
			ResourceID:  query.ResourceID,
			CounterType: query.CounterType,
			Count:       count,
			Success:     true,
		})
	}

	return responses, nil
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
