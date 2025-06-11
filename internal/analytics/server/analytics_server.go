package server

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	pb "high-go-press/api/proto/analytics"
	commonpb "high-go-press/api/proto/common"
	"high-go-press/internal/analytics/dao"
	"high-go-press/pkg/kafka"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

// AnalyticsServer Analytics gRPC服务器
type AnalyticsServer struct {
	pb.UnimplementedAnalyticsServiceServer

	dao      dao.AnalyticsDAO
	consumer kafka.Consumer
	logger   *zap.Logger

	// 内存缓存热点数据
	topCountersCache map[string][]*pb.CounterItem
	statsCache       map[string]*pb.StatsResponse
	cacheMu          sync.RWMutex
	lastCacheUpdate  time.Time
}

// NewAnalyticsServer 创建Analytics服务器
func NewAnalyticsServer(dao dao.AnalyticsDAO, consumer kafka.Consumer, logger *zap.Logger) *AnalyticsServer {
	server := &AnalyticsServer{
		dao:              dao,
		consumer:         consumer,
		logger:           logger,
		topCountersCache: make(map[string][]*pb.CounterItem),
		statsCache:       make(map[string]*pb.StatsResponse),
	}

	// 启动缓存更新goroutine
	go server.startCacheUpdater()

	return server
}

// GetTopCounters 获取热门计数器排行榜
func (s *AnalyticsServer) GetTopCounters(ctx context.Context, req *pb.TopCountersRequest) (*pb.TopCountersResponse, error) {
	s.logger.Info("GetTopCounters called",
		zap.String("counter_type", req.CounterType),
		zap.Int32("limit", req.Limit),
		zap.String("time_range", req.TimeRange))

	// 参数验证
	if req.CounterType == "" {
		return &pb.TopCountersResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.InvalidArgument),
				Message: "counter_type is required",
			},
		}, nil
	}

	if req.Limit <= 0 {
		req.Limit = 10 // 默认返回10条
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("%s:%s:%d", req.CounterType, req.TimeRange, req.Limit)

	// 尝试从缓存获取
	s.cacheMu.RLock()
	if cached, exists := s.topCountersCache[cacheKey]; exists {
		s.cacheMu.RUnlock()

		// 处理分页
		start, end := s.calculatePagination(len(cached), req.Pagination)
		result := cached[start:end]

		return &pb.TopCountersResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.OK),
				Message: "Success",
			},
			Counters: result,
			Pagination: &commonpb.PaginationResponse{
				Total:   int32(len(cached)),
				Page:    req.Pagination.GetPage(),
				Size:    req.Limit,
				HasNext: end < len(cached),
			},
		}, nil
	}
	s.cacheMu.RUnlock()

	// 缓存未命中，从数据源获取
	counters, err := s.dao.GetTopCounters(ctx, req.CounterType, req.TimeRange, int(req.Limit))
	if err != nil {
		s.logger.Error("Failed to get top counters from DAO", zap.Error(err))
		return &pb.TopCountersResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to get top counters",
			},
		}, nil
	}

	// 转换为protobuf格式
	pbCounters := make([]*pb.CounterItem, len(counters))
	for i, counter := range counters {
		pbCounters[i] = &pb.CounterItem{
			ResourceId:     counter.ResourceID,
			CounterType:    counter.CounterType,
			Value:          counter.Value,
			IncrementCount: counter.IncrementCount,
			LastUpdated: &commonpb.Timestamp{
				Seconds: counter.LastUpdated.Unix(),
				Nanos:   int32(counter.LastUpdated.Nanosecond()),
			},
		}
	}

	// 更新缓存
	s.cacheMu.Lock()
	s.topCountersCache[cacheKey] = pbCounters
	s.cacheMu.Unlock()

	// 处理分页
	start, end := s.calculatePagination(len(pbCounters), req.Pagination)
	result := pbCounters[start:end]

	return &pb.TopCountersResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Success",
		},
		Counters: result,
		Pagination: &commonpb.PaginationResponse{
			Total:   int32(len(pbCounters)),
			Page:    req.Pagination.GetPage(),
			Size:    int32(len(result)),
			HasNext: end < len(pbCounters),
		},
	}, nil
}

// GetCounterStats 获取计数器统计信息
func (s *AnalyticsServer) GetCounterStats(ctx context.Context, req *pb.StatsRequest) (*pb.StatsResponse, error) {
	s.logger.Info("GetCounterStats called",
		zap.String("resource_id", req.ResourceId),
		zap.String("counter_type", req.CounterType),
		zap.String("time_range", req.TimeRange))

	// 参数验证
	if req.ResourceId == "" || req.CounterType == "" {
		return &pb.StatsResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.InvalidArgument),
				Message: "resource_id and counter_type are required",
			},
		}, nil
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("stats:%s:%s:%s", req.ResourceId, req.CounterType, req.TimeRange)

	// 尝试从缓存获取
	s.cacheMu.RLock()
	if cached, exists := s.statsCache[cacheKey]; exists {
		s.cacheMu.RUnlock()
		return cached, nil
	}
	s.cacheMu.RUnlock()

	// 从数据源获取统计数据
	stats, err := s.dao.GetCounterStats(ctx, req.ResourceId, req.CounterType, req.TimeRange)
	if err != nil {
		s.logger.Error("Failed to get counter stats from DAO", zap.Error(err))
		return &pb.StatsResponse{
			Status: &commonpb.Status{
				Code:    int32(codes.Internal),
				Message: "Failed to get counter stats",
			},
		}, nil
	}

	// 构建响应
	response := &pb.StatsResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Success",
		},
		ResourceId:  req.ResourceId,
		CounterType: req.CounterType,
		Metrics:     make(map[string]float64),
		TimeSeries:  make([]*pb.TimeSeriesPoint, 0),
	}

	// 填充指标数据
	response.Metrics["total"] = float64(stats.Total)
	response.Metrics["avg"] = stats.Average
	response.Metrics["peak"] = float64(stats.Peak)

	// 填充时间序列数据
	for _, point := range stats.TimeSeries {
		response.TimeSeries = append(response.TimeSeries, &pb.TimeSeriesPoint{
			Timestamp: &commonpb.Timestamp{
				Seconds: point.Timestamp.Unix(),
				Nanos:   int32(point.Timestamp.Nanosecond()),
			},
			Value: point.Value,
		})
	}

	// 更新缓存
	s.cacheMu.Lock()
	s.statsCache[cacheKey] = response
	s.cacheMu.Unlock()

	return response, nil
}

// GetSystemMetrics 获取系统监控数据
func (s *AnalyticsServer) GetSystemMetrics(ctx context.Context, req *pb.SystemMetricsRequest) (*pb.SystemMetricsResponse, error) {
	s.logger.Info("GetSystemMetrics called", zap.Strings("components", req.Components))

	response := &pb.SystemMetricsResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Success",
		},
		Metrics: make(map[string]*pb.ComponentMetrics),
	}

	now := time.Now()

	for _, component := range req.Components {
		metrics := &pb.ComponentMetrics{
			Component: component,
			Values:    make(map[string]float64),
			CollectedAt: &commonpb.Timestamp{
				Seconds: now.Unix(),
				Nanos:   int32(now.Nanosecond()),
			},
		}

		// 根据组件类型收集指标
		switch component {
		case "analytics":
			metrics.Values["cache_size"] = float64(len(s.topCountersCache))
			metrics.Values["cache_hit_rate"] = 0.95 // 模拟数据
		case "memory":
			// 模拟内存指标
			metrics.Values["heap_size"] = 64.5
			metrics.Values["heap_used"] = 32.1
		default:
			metrics.Values["status"] = 1.0
		}

		response.Metrics[component] = metrics
	}

	return response, nil
}

// HealthCheck 健康检查
func (s *AnalyticsServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	// 检查各组件健康状态
	details := make(map[string]string)
	details["service"] = "analytics"
	details["status"] = "healthy"
	details["cache_size"] = strconv.Itoa(len(s.topCountersCache))
	details["uptime"] = time.Since(s.lastCacheUpdate).String()

	return &pb.HealthCheckResponse{
		Status: &commonpb.Status{
			Code:    int32(codes.OK),
			Message: "Service is healthy",
		},
		Service: "analytics",
		Details: details,
	}, nil
}

// calculatePagination 计算分页
func (s *AnalyticsServer) calculatePagination(total int, pagination *commonpb.PaginationRequest) (start, end int) {
	if pagination == nil {
		return 0, total
	}

	page := int(pagination.Page)
	pageSize := int(pagination.Size)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	start = (page - 1) * pageSize
	end = start + pageSize

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return start, end
}

// startCacheUpdater 启动缓存更新器
func (s *AnalyticsServer) startCacheUpdater() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒更新缓存
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateCache()
		}
	}
}

// updateCache 更新缓存
func (s *AnalyticsServer) updateCache() {
	s.logger.Debug("Updating analytics cache")

	s.cacheMu.Lock()
	s.lastCacheUpdate = time.Now()
	s.cacheMu.Unlock()

	// TODO: 在真实环境中，这里会从数据库预加载热点数据
}
