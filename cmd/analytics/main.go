package main

import (
	"context"
	"fmt"
	"net"
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
	"high-go-press/pkg/kafka"
	"high-go-press/pkg/logger"
)

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

	log.Info("Starting Analytics microservice with Kafka integration...",
		zap.String("service", "analytics"),
		zap.String("version", "2.0.0"))

	// 初始化Analytics DAO
	analyticsDAO := dao.NewMemoryAnalyticsDAO()

	// 🔥 初始化Kafka Manager
	kafkaConfig := kafka.DefaultKafkaConfig()
	kafkaConfig.Mode = kafka.ModeMock // 默认Mock模式

	// 如果设置了环境变量，切换到真实Kafka
	if os.Getenv("KAFKA_MODE") == "real" {
		kafkaConfig.Mode = kafka.ModeReal
		kafkaConfig.Consumer.Brokers = []string{os.Getenv("KAFKA_BROKERS")}
		if len(kafkaConfig.Consumer.Brokers) == 0 || kafkaConfig.Consumer.Brokers[0] == "" {
			kafkaConfig.Consumer.Brokers = []string{"localhost:9092"}
		}
		kafkaConfig.Consumer.GroupID = "analytics-group"
		kafkaConfig.Consumer.Topics = []string{"counter-events"}
		kafkaConfig.Consumer.AutoOffsetReset = "latest"

		log.Info("Using real Kafka",
			zap.Strings("brokers", kafkaConfig.Consumer.Brokers),
			zap.String("group_id", kafkaConfig.Consumer.GroupID))
	}

	kafkaManager, err := kafka.NewKafkaManager(kafkaConfig, log)
	if err != nil {
		log.Fatal("Failed to initialize Kafka manager", zap.Error(err))
	}
	defer kafkaManager.Close()

	log.Info("✅ Kafka manager initialized successfully",
		zap.String("mode", string(kafkaManager.GetMode())))

	// 订阅counter-events主题
	kafkaConsumer := kafkaManager.GetConsumer()
	if err := kafkaConsumer.Subscribe([]string{"counter-events"}); err != nil {
		log.Fatal("Failed to subscribe to Kafka topics", zap.Error(err))
	}

	// 创建计数器事件处理器
	eventHandler := kafka.NewCounterEventHandler(
		func(ctx context.Context, event *kafka.CounterEvent) error {
			// 更新Analytics统计数据
			log.Info("Processing counter event in Analytics",
				zap.String("event_id", event.EventID),
				zap.String("resource_id", event.ResourceID),
				zap.String("counter_type", event.CounterType),
				zap.Int64("delta", event.Delta),
				zap.Int64("new_value", event.NewValue))

			return analyticsDAO.UpdateCounterStats(ctx, event.ResourceID, event.CounterType, event.Delta)
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

	// 创建gRPC服务器
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(log)),
	)

	// 注册服务
	pb.RegisterAnalyticsServiceServer(grpcServer, analyticsServer)

	// 启用gRPC反射 (用于grpcurl测试)
	reflection.Register(grpcServer)

	// 监听端口
	lis, err := net.Listen("tcp", ":9002")
	if err != nil {
		log.Fatal("Failed to listen", zap.Error(err))
	}

	// 启动gRPC服务器
	go func() {
		log.Info("Analytics gRPC server starting",
			zap.String("address", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("Failed to serve gRPC", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Analytics service...")

	// 优雅关闭
	cancel() // 停止Kafka消费者
	grpcServer.GracefulStop()

	log.Info("Analytics service stopped")
}

// loggingInterceptor gRPC请求日志拦截器
func loggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)

		logger.Info("gRPC call",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.Error(err))

		return resp, err
	}
}
