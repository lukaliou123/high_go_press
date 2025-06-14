package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"high-go-press/api/proto/common"
	"high-go-press/api/proto/counter"
	"high-go-press/internal/dao"
	"high-go-press/pkg/consul"
	"high-go-press/pkg/kafka"
	"high-go-press/pkg/metrics"
	"high-go-press/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
)

// CounterServer 带Redis和Kafka集成的Counter服务实现
type CounterServer struct {
	counter.UnimplementedCounterServiceServer
	logger         *zap.Logger
	redisDAO       *dao.RedisRepo
	kafkaManager   *kafka.KafkaManager
	metricsManager *metrics.MetricsManager
	eventCounter   int64 // 事件计数器
}

func NewCounterServer(logger *zap.Logger, redisDAO *dao.RedisRepo, kafkaManager *kafka.KafkaManager, metricsManager *metrics.MetricsManager) *CounterServer {
	return &CounterServer{
		logger:         logger,
		redisDAO:       redisDAO,
		kafkaManager:   kafkaManager,
		metricsManager: metricsManager,
		eventCounter:   0,
	}
}

func (s *CounterServer) IncrementCounter(ctx context.Context, req *counter.IncrementRequest) (*counter.IncrementResponse, error) {
	start := time.Now()

	// 记录gRPC指标
	defer func() {
		duration := time.Since(start)
		s.metricsManager.RecordGRPCRequest("/counter.CounterService/IncrementCounter", "counter", "OK", duration)
	}()

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

	// 记录业务指标
	businessWrapper := middleware.NewBusinessMetricsWrapper(s.metricsManager, "counter", s.logger)
	var newValue int64
	var err error

	businessErr := businessWrapper.WrapOperation("increment_counter", func() error {
		newValue, err = s.redisDAO.IncrementCounter(ctx, key, delta)
		return err
	})

	if businessErr != nil {
		s.logger.Error("Failed to increment counter in Redis",
			zap.String("key", key),
			zap.Int64("delta", delta),
			zap.Error(businessErr))

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

	// 更新业务指标
	businessWrapper.SetGauge("current_counter_value", float64(newValue))

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
	start := time.Now()

	// 记录gRPC指标
	defer func() {
		duration := time.Since(start)
		s.metricsManager.RecordGRPCRequest("/counter.CounterService/GetCounter", "counter", "OK", duration)
	}()

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

	// 记录数据库指标
	dbWrapper := middleware.NewDBMetricsWrapper(s.metricsManager, "counter", "redis", s.logger)
	var value int64
	var err error

	_, dbErr := dbWrapper.WrapQueryWithResult("get", func() (interface{}, error) {
		value, err = s.redisDAO.GetCounter(ctx, key)
		return value, err
	})

	if dbErr != nil {
		s.logger.Error("Failed to get counter from Redis",
			zap.String("key", key),
			zap.Error(dbErr))

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
	start := time.Now()

	// 记录gRPC指标
	defer func() {
		duration := time.Since(start)
		s.metricsManager.RecordGRPCRequest("/counter.CounterService/BatchGetCounters", "counter", "OK", duration)
	}()

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
	dbWrapper := middleware.NewDBMetricsWrapper(s.metricsManager, "counter", "redis", s.logger)
	var values map[string]int64
	var err error

	_, dbErr := dbWrapper.WrapQueryWithResult("batch_get", func() (interface{}, error) {
		values, err = s.redisDAO.GetMultiCounters(ctx, keys)
		return values, err
	})

	if dbErr != nil {
		s.logger.Error("Failed to batch get counters from Redis", zap.Error(dbErr))
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
		// 更新健康状态指标
		s.metricsManager.SetServiceHealth("counter", "redis", false)

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

	// 更新健康状态指标
	s.metricsManager.SetServiceHealth("counter", "redis", true)
	s.metricsManager.SetServiceHealth("counter", "kafka", true)

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

// setupHTTPMonitoringServer 设置HTTP监控服务器
func setupHTTPMonitoringServer(metricsManager *metrics.MetricsManager, logger *zap.Logger) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// 添加HTTP指标中间件
	router.Use(middleware.HTTPMetricsMiddleware(metricsManager, "counter"))

	// 健康检查端点
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "counter",
			"timestamp": time.Now().Unix(),
			"version":   "2.0.0",
		})
	})

	// Prometheus指标端点
	router.GET("/metrics", gin.WrapH(metricsManager.GetHandler()))

	// 服务状态端点
	router.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "counter",
			"ports": gin.H{
				"grpc":       9001,
				"monitoring": 8081,
			},
			"endpoints": gin.H{
				"health":  "/health",
				"metrics": "/metrics",
				"status":  "/status",
			},
		})
	})

	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	return server
}

func main() {
	// 创建logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("Starting Counter microservice with Redis, Kafka and Monitoring integration...",
		zap.String("service", "counter"),
		zap.String("version", "2.0.0"))

	// 初始化指标管理器
	metricsConfig := &metrics.Config{
		Namespace:      "highgopress",
		Subsystem:      "counter",
		EnableSystem:   true,
		EnableBusiness: true,
		EnableDB:       true,
		EnableCache:    true,
	}
	metricsManager := metrics.NewMetricsManager(metricsConfig, logger)
	logger.Info("✅ Metrics manager initialized")

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

	// 🌐 初始化Consul客户端并注册服务
	consulConfig := &consul.Config{
		Address: "localhost:8500",
		Scheme:  "http",
	}

	consulClient, err := consul.NewClient(consulConfig, logger)
	if err != nil {
		logger.Fatal("Failed to create consul client", zap.Error(err))
	}
	defer consulClient.Close()

	// 注册Counter服务到Consul
	serviceConfig := &consul.ServiceConfig{
		ID:      "counter-1",
		Name:    "high-go-press-counter",
		Tags:    []string{"counter", "grpc", "microservice", "v2.0"},
		Address: "localhost",
		Port:    9001,
		Check: &consul.HealthCheck{
			TCP:      "localhost:9001",
			Interval: "10s",
			Timeout:  "3s",
		},
	}

	if err := consulClient.RegisterService(serviceConfig); err != nil {
		logger.Fatal("Failed to register service to Consul", zap.Error(err))
	}

	logger.Info("✅ Counter service registered to Consul successfully")

	// 确保在退出时注销服务
	defer func() {
		if err := consulClient.DeregisterService("counter-1"); err != nil {
			logger.Error("Failed to deregister service from Consul", zap.Error(err))
		} else {
			logger.Info("Counter service deregistered from Consul")
		}
	}()

	// 创建gRPC服务器，添加指标拦截器
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.GRPCMetricsUnaryInterceptor(metricsManager, "counter")),
	)

	// 注册Counter服务
	counterSrv := NewCounterServer(logger, redisDAO, kafkaManager, metricsManager)
	counter.RegisterCounterServiceServer(grpcServer, counterSrv)

	// 启用反射 (用于grpcurl等工具)
	reflection.Register(grpcServer)

	// 监听gRPC端口
	grpcListen, err := net.Listen("tcp", ":9001")
	if err != nil {
		logger.Fatal("Failed to listen on gRPC port", zap.Error(err))
	}

	// 设置HTTP监控服务器
	httpServer := setupHTTPMonitoringServer(metricsManager, logger)

	// 启动gRPC服务器
	go func() {
		logger.Info("Counter gRPC server starting",
			zap.String("address", grpcListen.Addr().String()))

		if err := grpcServer.Serve(grpcListen); err != nil {
			logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// 启动HTTP监控服务器
	go func() {
		logger.Info("Counter HTTP monitoring server starting",
			zap.String("address", httpServer.Addr))

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP monitoring server failed", zap.Error(err))
		}
	}()

	// 设置服务健康状态
	metricsManager.SetServiceHealth("counter", "main", true)

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Counter service...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭HTTP服务器
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	// 关闭gRPC服务器
	grpcServer.GracefulStop()

	// 关闭Redis连接
	redisClient.Close()

	logger.Info("Counter service stopped gracefully")
}
