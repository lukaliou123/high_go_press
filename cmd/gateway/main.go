package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"high-go-press/cmd/gateway/handlers"
	"high-go-press/internal/dao"
	"high-go-press/internal/service"
	"high-go-press/pkg/config"
	"high-go-press/pkg/kafka"
	"high-go-press/pkg/logger"
	"high-go-press/pkg/pool"
	"high-go-press/pkg/pprof"
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

	log.Info("Starting HighGoPress service",
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port))

	// 初始化Redis DAO
	redisDAO, err := dao.NewRedisDAO(cfg.Redis, log)
	if err != nil {
		log.Fatal("Failed to initialize Redis DAO", zap.Error(err))
	}
	defer redisDAO.Close()

	log.Info("Connected to Redis successfully",
		zap.String("addr", cfg.Redis.Addr))

	// 初始化Object Pool
	objectPool := pool.NewObjectPool()

	// 初始化Kafka Producer (使用Mock版本)
	kafkaProducer := kafka.NewMockProducer(log)
	defer kafkaProducer.Close()

	// 初始化Worker Pool
	workerPool, err := pool.NewWorkerPool(log)
	if err != nil {
		log.Fatal("Failed to initialize worker pool", zap.Error(err))
	}
	defer workerPool.Shutdown(context.Background())

	// 初始化服务
	counterService := service.NewCounterService(redisDAO, workerPool, objectPool, kafkaProducer, log)

	// 初始化处理器
	healthHandler := handlers.NewHealthHandler()
	counterHandler := handlers.NewCounterHandler(counterService, objectPool)
	poolHandler := handlers.NewPoolHandler(workerPool)

	// 创建Gin路由器
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 添加pprof路由（开发环境）
	if cfg.Server.Mode != "release" {
		pprof.AddPprofRoutes(router)
		log.Info("Pprof routes enabled", zap.String("path", "/debug/pprof"))
	}

	// API路由
	v1 := router.Group("/api/v1")
	{
		// 健康检查
		v1.GET("/health", healthHandler.HealthCheck)

		// 计数器相关
		counterGroup := v1.Group("/counter")
		{
			counterGroup.POST("/increment", counterHandler.IncrementCounter)
			counterGroup.GET("/:resource_id/:counter_type", counterHandler.GetCounter)
			counterGroup.POST("/batch", counterHandler.BatchGetCounters)
		}

		// 系统监控
		systemGroup := v1.Group("/system")
		{
			systemGroup.GET("/pools", poolHandler.GetPoolStats)
			systemGroup.POST("/pools/test", poolHandler.TestWorkerPool)

			// 对象池统计
			systemGroup.GET("/object-pools", func(c *gin.Context) {
				stats := objectPool.GetStats()
				c.JSON(http.StatusOK, gin.H{
					"status": "success",
					"data":   stats,
				})
			})

			// Kafka统计
			systemGroup.GET("/kafka", func(c *gin.Context) {
				stats := kafkaProducer.GetStats()
				c.JSON(http.StatusOK, gin.H{
					"status": "success",
					"data":   stats,
				})
			})
		}
	}

	// 启动HTTP服务器
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	// 启动服务器
	go func() {
		log.Info("Server starting", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// 优雅关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}
