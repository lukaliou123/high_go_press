package main

import (
	"fmt"
	"log"

	"high-go-press/internal/dao"
	"high-go-press/internal/service"
	"high-go-press/pkg/config"
	"high-go-press/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	if err := logger.Init(cfg.Log.Level, cfg.Log.Format); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Info("Starting HighGoPress service",
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port))

	// 初始化Redis客户端
	redisClient := dao.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer redisClient.Close()

	// 初始化数据仓库
	counterRepo := dao.NewRedisRepo(redisClient)

	// 初始化业务服务
	counterService := service.NewCounterService(counterRepo)

	// 初始化HTTP处理器
	handler := NewHandler(counterService)

	// 设置Gin模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建Gin引擎
	r := gin.New()

	// 添加中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 设置路由
	handler.setupRoutes(r)

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("Server starting", zap.String("addr", addr))

	if err := r.Run(addr); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
