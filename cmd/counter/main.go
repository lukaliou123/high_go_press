package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"high-go-press/api/proto/common"
	"high-go-press/api/proto/counter"
	"high-go-press/internal/dao"
	"high-go-press/pkg/kafka"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
)

// CounterServer 带Redis和Kafka集成的Counter服务实现
type CounterServer struct {
	counter.UnimplementedCounterServiceServer
	logger       *zap.Logger
	redisDAO     *dao.RedisRepo
	kafkaManager *kafka.KafkaManager
	eventCounter int64 // 事件计数器
}

func NewCounterServer(logger *zap.Logger, redisDAO *dao.RedisRepo, kafkaManager *kafka.KafkaManager) *CounterServer {
	return &CounterServer{
		logger:       logger,
		redisDAO:     redisDAO,
		kafkaManager: kafkaManager,
		eventCounter: 0,
	}
}

func (s *CounterServer) IncrementCounter(ctx context.Context, req *counter.IncrementRequest) (*counter.IncrementResponse, error) {
	if req.ResourceId == "" || req.CounterType == "" {
		return &counter.IncrementResponse{
			Status: &common.Status{
				Success: false,
				Message: "resource_id and counter_type are required",
				Code:    int32(codes.InvalidArgument),
			},
		}, nil
	}

	delta := req.Delta
	if delta == 0 {
		delta = 1
	}

	// 🔧 修复: 使用统一的Redis key格式
	key := fmt.Sprintf("counter:%s:%s", req.ResourceId, req.CounterType)

	// 🔧 修复: 使用Redis而不是内存存储
	newValue, err := s.redisDAO.IncrementCounter(ctx, key, delta)
	if err != nil {
		s.logger.Error("Failed to increment counter in Redis",
			zap.String("key", key),
			zap.Int64("delta", delta),
			zap.Error(err))

		return &counter.IncrementResponse{
			Status: &common.Status{
				Success: false,
				Message: "Failed to increment counter",
				Code:    int32(codes.Internal),
			},
		}, nil
	}

	s.logger.Info("Counter incremented",
		zap.String("key", key),
		zap.Int64("delta", delta),
		zap.Int64("new_value", newValue))

	// 🔥 发送Kafka事件
	if err := s.sendCounterEvent(ctx, req.ResourceId, req.CounterType, delta, newValue); err != nil {
		s.logger.Error("Failed to send counter event", zap.Error(err))
		// 注意：这里我们不返回错误，因为计数器更新已经成功
		// 只是事件发送失败，可以考虑重试或异步处理
	}

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

// sendCounterEvent 发送计数器事件到Kafka
func (s *CounterServer) sendCounterEvent(ctx context.Context, resourceID, counterType string, delta, newValue int64) error {
	s.eventCounter++

	event := &kafka.CounterEvent{
		EventID:     fmt.Sprintf("evt_%d_%d", time.Now().Unix(), s.eventCounter),
		ResourceID:  resourceID,
		CounterType: counterType,
		Delta:       delta,
		NewValue:    newValue,
		Timestamp:   time.Now(),
		Source:      "counter-microservice",
	}

	producer := s.kafkaManager.GetProducer()
	return producer.SendCounterEvent(ctx, event)
}

func (s *CounterServer) GetCounter(ctx context.Context, req *counter.GetCounterRequest) (*counter.GetCounterResponse, error) {
	if req.ResourceId == "" || req.CounterType == "" {
		return &counter.GetCounterResponse{
			Status: &common.Status{
				Success: false,
				Message: "resource_id and counter_type are required",
				Code:    int32(codes.InvalidArgument),
			},
		}, nil
	}

	// 🔧 修复: 使用统一的Redis key格式
	key := fmt.Sprintf("counter:%s:%s", req.ResourceId, req.CounterType)

	// 🔧 修复: 从Redis获取而不是内存
	value, err := s.redisDAO.GetCounter(ctx, key)
	if err != nil {
		s.logger.Error("Failed to get counter from Redis",
			zap.String("key", key),
			zap.Error(err))

		return &counter.GetCounterResponse{
			Status: &common.Status{
				Success: false,
				Message: "Failed to get counter",
				Code:    int32(codes.Internal),
			},
		}, nil
	}

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

func (s *CounterServer) BatchGetCounters(ctx context.Context, req *counter.BatchGetRequest) (*counter.BatchGetResponse, error) {
	results := make([]*counter.GetCounterResponse, 0, len(req.Requests))

	// 🔧 修复: 使用Redis批量获取
	keys := make([]string, 0, len(req.Requests))
	keyToReq := make(map[string]*counter.GetCounterRequest)

	for _, r := range req.Requests {
		if r.ResourceId == "" || r.CounterType == "" {
			continue
		}

		key := fmt.Sprintf("counter:%s:%s", r.ResourceId, r.CounterType)
		keys = append(keys, key)
		keyToReq[key] = r
	}

	if len(keys) == 0 {
		return &counter.BatchGetResponse{
			Status: &common.Status{
				Success: true,
				Message: "Empty batch request",
				Code:    int32(codes.OK),
			},
			Counters: results,
		}, nil
	}

	// 批量从Redis获取
	values, err := s.redisDAO.GetMultiCounters(ctx, keys)
	if err != nil {
		s.logger.Error("Failed to batch get counters from Redis", zap.Error(err))
		return &counter.BatchGetResponse{
			Status: &common.Status{
				Success: false,
				Message: "Failed to batch get counters",
				Code:    int32(codes.Internal),
			},
		}, nil
	}

	for _, key := range keys {
		r := keyToReq[key]
		if r == nil {
			continue
		}

		value := values[key] // Redis会返回0如果key不存在

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

func (s *CounterServer) HealthCheck(ctx context.Context, req *counter.HealthCheckRequest) (*counter.HealthCheckResponse, error) {
	// 检查Redis连接
	_, err := s.redisDAO.GetCounter(ctx, "health_check_test")
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

	// 获取Kafka健康状态
	kafkaHealth := s.kafkaManager.HealthCheck()

	details := map[string]string{
		"redis":       "healthy",
		"event_count": fmt.Sprintf("%d", s.eventCounter),
		"kafka_mode":  fmt.Sprintf("%v", kafkaHealth["mode"]),
	}

	return &counter.HealthCheckResponse{
		Status: &common.Status{
			Success: true,
			Message: "Service is healthy",
			Code:    int32(codes.OK),
		},
		Service: "counter",
		Details: details,
	}, nil
}

func main() {
	// 创建logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("Starting Counter microservice with Redis and Kafka integration...",
		zap.String("service", "counter"),
		zap.String("version", "2.0.0"))

	// 🔧 初始化Redis连接
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // 可以通过环境变量配置
		Password: "",               // 可以通过环境变量配置
		DB:       0,                // 可以通过环境变量配置
	})

	// 测试Redis连接
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	logger.Info("✅ Redis connection established successfully")

	// 创建Redis DAO
	redisDAO := &dao.RedisRepo{}
	redisDAO.SetClient(redisClient)
	redisDAO.SetLogger(logger)

	// 🔥 初始化Kafka（使用Mock模式开始）
	kafkaConfig := kafka.DefaultKafkaConfig()
	kafkaConfig.Mode = kafka.ModeMock // 可以通过环境变量或配置文件改变

	// 如果设置了环境变量，切换到真实Kafka
	if os.Getenv("KAFKA_MODE") == "real" {
		kafkaConfig.Mode = kafka.ModeReal
		kafkaConfig.Producer.Brokers = []string{os.Getenv("KAFKA_BROKERS")}
		if len(kafkaConfig.Producer.Brokers) == 0 || kafkaConfig.Producer.Brokers[0] == "" {
			kafkaConfig.Producer.Brokers = []string{"localhost:9092"}
		}
		logger.Info("Using real Kafka",
			zap.Strings("brokers", kafkaConfig.Producer.Brokers))
	}

	kafkaManager, err := kafka.NewKafkaManager(kafkaConfig, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Kafka manager", zap.Error(err))
	}
	defer kafkaManager.Close()

	logger.Info("✅ Kafka manager initialized successfully",
		zap.String("mode", string(kafkaManager.GetMode())))

	// 创建gRPC服务器
	grpcServer := grpc.NewServer()

	// 注册Counter服务
	counterSrv := NewCounterServer(logger, redisDAO, kafkaManager)
	counter.RegisterCounterServiceServer(grpcServer, counterSrv)

	// 启用反射 (用于grpcurl等工具)
	reflection.Register(grpcServer)

	// 监听端口
	listen, err := net.Listen("tcp", ":9001")
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	// 启动服务器
	go func() {
		logger.Info("Counter gRPC server starting",
			zap.String("address", listen.Addr().String()))

		if err := grpcServer.Serve(listen); err != nil {
			logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Counter service...")

	// 优雅关闭
	redisClient.Close()
	grpcServer.GracefulStop()

	logger.Info("Counter service stopped gracefully")
}
