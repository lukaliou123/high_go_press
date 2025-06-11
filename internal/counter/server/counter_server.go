package server

import (
	"context"
	"fmt"
	"time"

	"high-go-press/api/proto/common"
	"high-go-press/api/proto/counter"
	"high-go-press/internal/dao"
	"high-go-press/pkg/kafka"
	"high-go-press/pkg/pool"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CounterServer gRPC服务端实现
type CounterServer struct {
	counter.UnimplementedCounterServiceServer
	dao        *dao.RedisRepo
	workerPool *pool.WorkerPool
	objectPool *pool.ObjectPool
	producer   kafka.Producer
	logger     *zap.Logger
}

// NewCounterServer 创建Counter服务端
func NewCounterServer(
	dao *dao.RedisRepo,
	workerPool *pool.WorkerPool,
	objectPool *pool.ObjectPool,
	producer kafka.Producer,
	logger *zap.Logger,
) *CounterServer {
	return &CounterServer{
		dao:        dao,
		workerPool: workerPool,
		objectPool: objectPool,
		producer:   producer,
		logger:     logger,
	}
}

// IncrementCounter 实现计数器增量操作
func (s *CounterServer) IncrementCounter(ctx context.Context, req *counter.IncrementRequest) (*counter.IncrementResponse, error) {
	// 参数验证
	if req.ResourceId == "" || req.CounterType == "" {
		return &counter.IncrementResponse{
			Status: &common.Status{
				Success: false,
				Message: "resource_id and counter_type are required",
				Code:    int32(codes.InvalidArgument),
			},
		}, status.Errorf(codes.InvalidArgument, "resource_id and counter_type are required")
	}

	// 默认增量为1
	delta := req.Delta
	if delta == 0 {
		delta = 1
	}

	// 构建Redis key
	key := fmt.Sprintf("counter:%s:%s", req.ResourceId, req.CounterType)

	// 执行计数器增量操作
	newValue, err := s.dao.IncrementCounter(ctx, key, delta)
	if err != nil {
		s.logger.Error("Failed to increment counter",
			zap.String("resource_id", req.ResourceId),
			zap.String("counter_type", req.CounterType),
			zap.Int64("delta", delta),
			zap.Error(err))

		return &counter.IncrementResponse{
			Status: &common.Status{
				Success: false,
				Message: "Failed to increment counter",
				Code:    int32(codes.Internal),
			},
		}, status.Errorf(codes.Internal, "failed to increment counter: %v", err)
	}

	// 异步发送Kafka事件 (使用Worker Pool)
	s.workerPool.SubmitTask(func() {
		event := &kafka.CounterEvent{
			EventID:     fmt.Sprintf("%s-%d", key, time.Now().UnixNano()),
			ResourceID:  req.ResourceId,
			CounterType: req.CounterType,
			Delta:       delta,
			NewValue:    newValue,
			Timestamp:   time.Now(),
			Source:      "gRPC",
		}

		if err := s.producer.SendCounterEvent(context.Background(), event); err != nil {
			s.logger.Error("Failed to send counter event to kafka", zap.Error(err))
		}
	})

	// 构建成功响应
	return &counter.IncrementResponse{
		Status: &common.Status{
			Success: true,
			Message: "Counter incremented successfully",
			Code:    int32(codes.OK),
		},
		CurrentValue: newValue,
		ResourceId:   req.ResourceId,
		CounterType:  req.CounterType,
	}, nil
}

// GetCounter 获取单个计数器值
func (s *CounterServer) GetCounter(ctx context.Context, req *counter.GetCounterRequest) (*counter.GetCounterResponse, error) {
	// 参数验证
	if req.ResourceId == "" || req.CounterType == "" {
		return &counter.GetCounterResponse{
			Status: &common.Status{
				Success: false,
				Message: "resource_id and counter_type are required",
				Code:    int32(codes.InvalidArgument),
			},
		}, status.Errorf(codes.InvalidArgument, "resource_id and counter_type are required")
	}

	// 构建Redis key
	key := fmt.Sprintf("counter:%s:%s", req.ResourceId, req.CounterType)

	// 获取计数器值
	value, err := s.dao.GetCounter(ctx, key)
	if err != nil {
		s.logger.Error("Failed to get counter",
			zap.String("resource_id", req.ResourceId),
			zap.String("counter_type", req.CounterType),
			zap.Error(err))

		return &counter.GetCounterResponse{
			Status: &common.Status{
				Success: false,
				Message: "Failed to get counter",
				Code:    int32(codes.Internal),
			},
		}, status.Errorf(codes.Internal, "failed to get counter: %v", err)
	}

	// 构建成功响应
	return &counter.GetCounterResponse{
		Status: &common.Status{
			Success: true,
			Message: "Counter retrieved successfully",
			Code:    int32(codes.OK),
		},
		Value:       value,
		ResourceId:  req.ResourceId,
		CounterType: req.CounterType,
		LastUpdated: &common.Timestamp{
			Seconds: time.Now().Unix(),
			Nanos:   int32(time.Now().Nanosecond()),
		},
	}, nil
}

// BatchGetCounters 批量获取计数器
func (s *CounterServer) BatchGetCounters(ctx context.Context, req *counter.BatchGetRequest) (*counter.BatchGetResponse, error) {
	if len(req.Requests) == 0 {
		return &counter.BatchGetResponse{
			Status: &common.Status{
				Success: true,
				Message: "Empty batch request",
				Code:    int32(codes.OK),
			},
			Counters: []*counter.GetCounterResponse{},
		}, nil
	}

	// 使用对象池获取字符串切片
	keys := s.objectPool.GetStringSlice()
	defer s.objectPool.PutStringSlice(keys)

	// 构建所有keys和请求映射
	reqToKey := make(map[string]*counter.GetCounterRequest)
	for _, r := range req.Requests {
		if r.ResourceId == "" || r.CounterType == "" {
			continue
		}
		key := fmt.Sprintf("counter:%s:%s", r.ResourceId, r.CounterType)
		*keys = append(*keys, key)
		reqToKey[key] = r
	}

	// 批量获取计数器值
	counts, err := s.dao.GetMultiCounters(ctx, *keys)
	if err != nil {
		s.logger.Error("Failed to batch get counters", zap.Error(err))
		return &counter.BatchGetResponse{
			Status: &common.Status{
				Success: false,
				Message: "Failed to batch get counters",
				Code:    int32(codes.Internal),
			},
		}, status.Errorf(codes.Internal, "failed to batch get counters: %v", err)
	}

	// 构建响应
	results := make([]*counter.GetCounterResponse, 0, len(*keys))
	for _, key := range *keys {
		r := reqToKey[key]
		if r == nil {
			continue
		}

		value := counts[key] // 如果key不存在，会返回0值
		results = append(results, &counter.GetCounterResponse{
			Status: &common.Status{
				Success: true,
				Message: "Success",
				Code:    int32(codes.OK),
			},
			Value:       value,
			ResourceId:  r.ResourceId,
			CounterType: r.CounterType,
			LastUpdated: &common.Timestamp{
				Seconds: time.Now().Unix(),
				Nanos:   int32(time.Now().Nanosecond()),
			},
		})
	}

	return &counter.BatchGetResponse{
		Status: &common.Status{
			Success: true,
			Message: "Batch get completed",
			Code:    int32(codes.OK),
		},
		Counters: results,
	}, nil
}

// HealthCheck 健康检查
func (s *CounterServer) HealthCheck(ctx context.Context, req *counter.HealthCheckRequest) (*counter.HealthCheckResponse, error) {
	// 检查Redis连接 - 简单测试获取一个不存在的key
	_, err := s.dao.GetCounter(ctx, "health_check_test")
	if err != nil {
		return &counter.HealthCheckResponse{
			Status: &common.Status{
				Success: false,
				Message: "Redis connection failed",
				Code:    int32(codes.Unavailable),
			},
			Service: "counter",
			Details: map[string]string{
				"redis": "unhealthy",
				"error": err.Error(),
			},
		}, nil
	}

	// 检查Worker Pool状态
	poolStats := s.workerPool.GetStats()
	objectStats := s.objectPool.GetStats()

	return &counter.HealthCheckResponse{
		Status: &common.Status{
			Success: true,
			Message: "Service is healthy",
			Code:    int32(codes.OK),
		},
		Service: "counter",
		Details: map[string]string{
			"redis":                   "healthy",
			"worker_pool_general_cap": fmt.Sprintf("%d", poolStats.GeneralPool.Cap),
			"worker_pool_counter_cap": fmt.Sprintf("%d", poolStats.CounterPool.Cap),
			"object_pool_hit_rate":    fmt.Sprintf("%.2f", objectStats.Response.Hit),
		},
	}, nil
}
