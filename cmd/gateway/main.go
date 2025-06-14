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
	"high-go-press/internal/gateway/service"
	"high-go-press/pkg/config"
	"high-go-press/pkg/logger"
	"high-go-press/pkg/metrics"
	"high-go-press/pkg/middleware"
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

	log.Info("Starting HighGoPress Gateway service",
		zap.String("host", cfg.Gateway.Server.Host),
		zap.Int("port", cfg.Gateway.Server.Port))

	// 初始化指标管理器
	var metricsManager *metrics.MetricsManager
	if cfg.Monitoring.Prometheus.Enabled {
		metricsConfig := &metrics.Config{
			Namespace:      cfg.Monitoring.Prometheus.Namespace,
			Subsystem:      cfg.Monitoring.Prometheus.Subsystem,
			EnableSystem:   cfg.Monitoring.Prometheus.EnableSystem,
			EnableBusiness: cfg.Monitoring.Prometheus.EnableBusiness,
			EnableDB:       cfg.Monitoring.Prometheus.EnableDB,
			EnableCache:    cfg.Monitoring.Prometheus.EnableCache,
		}
		metricsManager = metrics.NewMetricsManager(metricsConfig, log)

		// 设置服务健康状态
		metricsManager.SetServiceHealth("gateway", "main", true)

		log.Info("✅ Metrics manager initialized",
			zap.String("namespace", metricsConfig.Namespace),
			zap.Bool("system_metrics", metricsConfig.EnableSystem))
	}

	// 初始化Object Pool (仍需要用于请求对象复用)
	objectPool := pool.NewObjectPool()

	// 初始化微服务管理器
	serviceConfig := &service.Config{
		CounterServiceAddr: "localhost:9001", // Counter微服务地址
		TimeoutDuration:    5 * time.Second,
		// 连接池优化配置
		PoolSize:         20,               // 20个连接支持高并发
		MaxRecvMsgSize:   1024 * 1024 * 4,  // 4MB
		MaxSendMsgSize:   1024 * 1024 * 4,  // 4MB
		KeepAliveTime:    30 * time.Second, // 30秒keep-alive
		KeepAliveTimeout: 5 * time.Second,  // 5秒超时
	}

	serviceManager, err := service.NewServiceManager(serviceConfig, log)
	if err != nil {
		log.Fatal("Failed to initialize service manager", zap.Error(err))
	}
	defer serviceManager.Close()

	log.Info("✅ All microservices connected successfully")

	// 初始化处理器 - 使用微服务客户端
	healthHandler := handlers.NewHealthHandler()
	counterHandler := handlers.NewCounterHandler(serviceManager.GetCounterClient(), objectPool)

	// 创建Gin路由器
	if cfg.Gateway.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 添加指标收集中间件
	if metricsManager != nil {
		router.Use(middleware.HTTPMetricsMiddleware(metricsManager, "gateway"))
		log.Info("✅ HTTP metrics middleware enabled")
	}

	// 添加pprof路由（开发环境）
	if cfg.Gateway.Server.Mode != "release" {
		pprof.AddPprofRoutes(router)
		log.Info("Pprof routes enabled", zap.String("path", "/debug/pprof"))
	}

	// 添加指标暴露端点
	if metricsManager != nil {
		router.GET(cfg.Monitoring.Prometheus.Path, gin.WrapH(metricsManager.GetHandler()))
		log.Info("✅ Prometheus metrics endpoint enabled",
			zap.String("path", cfg.Monitoring.Prometheus.Path))
	}

	// API路由 - 保持现有API接口不变
	v1 := router.Group("/api/v1")
	{
		// 健康检查
		v1.GET("/health", healthHandler.HealthCheck)

		// 计数器相关 - 现在转发到Counter微服务
		counterGroup := v1.Group("/counter")
		{
			counterGroup.POST("/increment", counterHandler.IncrementCounter)
			counterGroup.GET("/:resource_id/:counter_type", counterHandler.GetCounter)
			counterGroup.POST("/batch", counterHandler.BatchGetCounters)
		}

		// 系统监控 - 保留必要的监控功能
		systemGroup := v1.Group("/system")
		{
			// 对象池统计
			systemGroup.GET("/object-pools", func(c *gin.Context) {
				stats := objectPool.GetStats()
				c.JSON(http.StatusOK, gin.H{
					"status": "success",
					"data":   stats,
				})
			})

			// 连接池统计 - 新增
			systemGroup.GET("/grpc-pools", func(c *gin.Context) {
				poolStats := serviceManager.GetPoolStats()
				c.JSON(http.StatusOK, gin.H{
					"status": "success",
					"data":   poolStats,
				})
			})

			// 微服务健康检查
			systemGroup.GET("/services/health", func(c *gin.Context) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				if err := serviceManager.HealthCheck(ctx); err != nil {
					// 更新健康状态指标
					if metricsManager != nil {
						metricsManager.SetServiceHealth("gateway", "services", false)
					}

					c.JSON(http.StatusServiceUnavailable, gin.H{
						"status":  "error",
						"error":   "Service health check failed",
						"details": err.Error(),
					})
					return
				}

				// 更新健康状态指标
				if metricsManager != nil {
					metricsManager.SetServiceHealth("gateway", "services", true)
				}

				c.JSON(http.StatusOK, gin.H{
					"status":  "success",
					"message": "All services are healthy",
				})
			})

			// 指标统计端点
			if metricsManager != nil {
				systemGroup.GET("/metrics/stats", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{
						"status": "success",
						"data": gin.H{
							"metrics_enabled": true,
							"namespace":       cfg.Monitoring.Prometheus.Namespace,
							"endpoint":        cfg.Monitoring.Prometheus.Path,
						},
					})
				})
			}
		}
	}

	// 启动指标服务器（独立端口）
	var metricsServer *http.Server
	if metricsManager != nil && cfg.Monitoring.Prometheus.Port != cfg.Server.Port {
		metricsRouter := gin.New()
		metricsRouter.GET(cfg.Monitoring.Prometheus.Path, gin.WrapH(metricsManager.GetHandler()))

		metricsServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Monitoring.Prometheus.Port),
			Handler: metricsRouter,
		}

		go func() {
			log.Info("Metrics server starting",
				zap.Int("port", cfg.Monitoring.Prometheus.Port),
				zap.String("path", cfg.Monitoring.Prometheus.Path))
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("Metrics server error", zap.Error(err))
			}
		}()
	}

	// 启动HTTP服务器
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Gateway.Server.Host, cfg.Gateway.Server.Port),
		Handler: router,
	}

	// 启动服务器
	go func() {
		log.Info("Gateway server starting",
			zap.String("addr", server.Addr),
			zap.String("mode", "microservices"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Gateway server...")

	// 优雅关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭主服务器
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	// 关闭指标服务器
	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Error("Metrics server forced to shutdown", zap.Error(err))
		}
	}

	// 关闭指标管理器
	if metricsManager != nil {
		if err := metricsManager.Shutdown(ctx); err != nil {
			log.Error("Failed to shutdown metrics manager", zap.Error(err))
		}
	}

	log.Info("Gateway server exited")
}
