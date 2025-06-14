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

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "high-go-press/api/proto/analytics"
	"high-go-press/internal/analytics/dao"
	"high-go-press/internal/analytics/server"
	"high-go-press/pkg/config"
	"high-go-press/pkg/consul"
	"high-go-press/pkg/kafka"
	"high-go-press/pkg/logger"
	"high-go-press/pkg/metrics"
	"high-go-press/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// setupHTTPMonitoringServer 设置HTTP监控服务器
func setupHTTPMonitoringServer(metricsManager *metrics.MetricsManager, logger *zap.Logger) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// 添加HTTP指标中间件
	router.Use(middleware.HTTPMetricsMiddleware(metricsManager, "analytics"))

	// 健康检查端点
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "analytics",
			"timestamp": time.Now().Unix(),
			"version":   "2.0.0",
		})
	})

	// Prometheus指标端点
	router.GET("/metrics", gin.WrapH(metricsManager.GetHandler()))

	// 服务状态端点
	router.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "analytics",
			"ports": gin.H{
				"grpc":       9002,
				"monitoring": 8082,
			},
			"endpoints": gin.H{
				"health":  "/health",
				"metrics": "/metrics",
				"status":  "/status",
			},
		})
	})

	server := &http.Server{
		Addr:    ":8082",
		Handler: router,
	}

	return server
}

func main() {
	// 初始化配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := logger.NewLogger(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Analytics microservice with Redis, Kafka and Monitoring integration...",
		zap.String("service", "analytics"),
		zap.String("version", "2.0.0"))

	// 初始化指标管理器
	metricsConfig := &metrics.Config{
		Namespace:      "highgopress",
		Subsystem:      "analytics",
		EnableSystem:   true,
		EnableBusiness: true,
		EnableDB:       true,
		EnableCache:    true,
	}
	metricsManager := metrics.NewMetricsManager(metricsConfig, log)
	log.Info("✅ Metrics manager initialized")

	// 🔧 初始化Redis连接
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // 可以通过环境变量配置
		Password: "",               // 可以通过环境变量配置
		DB:       0,                // 可以通过环境变量配置
	})

	// 测试Redis连接
	ctx := context.Background()
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("✅ Redis connection established successfully")

	// 创建Analytics DAO
	analyticsDAO := dao.NewMemoryAnalyticsDAO()

	// 🔥 初始化Kafka
	kafkaConfig := kafka.DefaultKafkaConfig()
	kafkaConfig.Mode = kafka.ModeMock // 可以通过环境变量或配置文件改变

	// 如果设置了环境变量，切换到真实Kafka
	if os.Getenv("KAFKA_MODE") == "real" {
		kafkaConfig.Mode = kafka.ModeReal
		kafkaConfig.Consumer.Brokers = []string{os.Getenv("KAFKA_BROKERS")}
		if len(kafkaConfig.Consumer.Brokers) == 0 || kafkaConfig.Consumer.Brokers[0] == "" {
			kafkaConfig.Consumer.Brokers = []string{"localhost:9092"}
		}
		log.Info("Using real Kafka",
			zap.Strings("brokers", kafkaConfig.Consumer.Brokers))
	}

	kafkaManager, err := kafka.NewKafkaManager(kafkaConfig, log)
	if err != nil {
		log.Fatal("Failed to initialize Kafka manager", zap.Error(err))
	}
	defer kafkaManager.Close()

	log.Info("✅ Kafka manager initialized successfully",
		zap.String("mode", string(kafkaManager.GetMode())))

	// 🌐 初始化Consul客户端并注册服务
	consulConfig := &consul.Config{
		Address: "localhost:8500",
		Scheme:  "http",
	}

	consulClient, err := consul.NewClient(consulConfig, log)
	if err != nil {
		log.Fatal("Failed to create consul client", zap.Error(err))
	}
	defer consulClient.Close()

	// 注册Analytics服务到Consul
	serviceConfig := &consul.ServiceConfig{
		ID:      "analytics-1",
		Name:    "high-go-press-analytics",
		Tags:    []string{"analytics", "grpc", "microservice", "v2.0"},
		Address: "localhost",
		Port:    9002,
		Check: &consul.HealthCheck{
			TCP:      "localhost:9002",
			Interval: "10s",
			Timeout:  "3s",
		},
	}

	if err := consulClient.RegisterService(serviceConfig); err != nil {
		log.Fatal("Failed to register service to Consul", zap.Error(err))
	}

	log.Info("✅ Analytics service registered to Consul successfully")

	// 确保在退出时注销服务
	defer func() {
		if err := consulClient.DeregisterService("analytics-1"); err != nil {
			log.Error("Failed to deregister service from Consul", zap.Error(err))
		} else {
			log.Info("Analytics service deregistered from Consul")
		}
	}()

	// 订阅counter-events主题
	kafkaConsumer := kafkaManager.GetConsumer()
	if err := kafkaConsumer.Subscribe([]string{"counter-events"}); err != nil {
		log.Fatal("Failed to subscribe to Kafka topics", zap.Error(err))
	}

	// 创建计数器事件处理器，添加业务指标记录
	eventHandler := kafka.NewCounterEventHandler(
		func(ctx context.Context, event *kafka.CounterEvent) error {
			// 记录业务指标
			businessWrapper := middleware.NewBusinessMetricsWrapper(metricsManager, "analytics", log)

			return businessWrapper.WrapOperation("process_counter_event", func() error {
				// 更新Analytics统计数据
				log.Info("Processing counter event in Analytics",
					zap.String("event_id", event.EventID),
					zap.String("resource_id", event.ResourceID),
					zap.String("counter_type", event.CounterType),
					zap.Int64("delta", event.Delta),
					zap.Int64("new_value", event.NewValue))

				err := analyticsDAO.UpdateCounterStats(ctx, event.ResourceID, event.CounterType, event.Delta)

				// 更新业务指标
				if err == nil {
					businessWrapper.SetGauge("processed_events_total", float64(1))
					businessWrapper.SetGauge("latest_counter_value", float64(event.NewValue))
				}

				return err
			})
		},
		log,
	)

	// 启动Kafka消费者 (在后台goroutine中)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Info("Starting Kafka consumer for Analytics...")
		if err := kafkaConsumer.ConsumeMessages(ctx, eventHandler.HandleMessage); err != nil {
			if err != context.Canceled {
				log.Error("Kafka consumer error", zap.Error(err))
			}
		}
	}()

	// 等待一下让Consumer启动
	time.Sleep(100 * time.Millisecond)

	// 创建Analytics gRPC服务器
	analyticsServer := server.NewAnalyticsServer(analyticsDAO, kafkaConsumer, log)

	// 创建gRPC服务器，添加指标拦截器
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.GRPCMetricsUnaryInterceptor(metricsManager, "analytics")),
	)

	// 注册服务
	pb.RegisterAnalyticsServiceServer(grpcServer, analyticsServer)

	// 启用gRPC反射 (用于grpcurl测试)
	reflection.Register(grpcServer)

	// 监听gRPC端口
	grpcLis, err := net.Listen("tcp", ":9002")
	if err != nil {
		log.Fatal("Failed to listen on gRPC port", zap.Error(err))
	}

	// 设置HTTP监控服务器
	httpServer := setupHTTPMonitoringServer(metricsManager, log)

	// 启动gRPC服务器
	go func() {
		log.Info("Analytics gRPC server starting",
			zap.String("address", grpcLis.Addr().String()))
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// 启动HTTP监控服务器
	go func() {
		log.Info("Analytics HTTP monitoring server starting",
			zap.String("address", httpServer.Addr))

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP monitoring server failed", zap.Error(err))
		}
	}()

	// 设置服务健康状态
	metricsManager.SetServiceHealth("analytics", "main", true)
	metricsManager.SetServiceHealth("analytics", "kafka", true)

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Analytics service...")

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// 关闭HTTP服务器
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}

	// 停止Kafka消费者
	cancel()

	// 关闭gRPC服务器
	grpcServer.GracefulStop()

	log.Info("Analytics service stopped gracefully")
}
