package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"high-go-press/internal/gateway/client"

	"go.uber.org/zap"
)

// ServiceManager 微服务管理器
type ServiceManager struct {
	counterClientPool *client.CounterClientPool
	config            *Config
	logger            *zap.Logger
}

// Config 服务配置
type Config struct {
	CounterServiceAddr string
	TimeoutDuration    time.Duration
	// 连接池配置
	PoolSize         int
	MaxRecvMsgSize   int
	MaxSendMsgSize   int
	KeepAliveTime    time.Duration
	KeepAliveTimeout time.Duration
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		CounterServiceAddr: "localhost:9001",
		TimeoutDuration:    5 * time.Second,
		PoolSize:           20,
		MaxRecvMsgSize:     1024 * 1024 * 4, // 4MB
		MaxSendMsgSize:     1024 * 1024 * 4, // 4MB
		KeepAliveTime:      30 * time.Second,
		KeepAliveTimeout:   5 * time.Second,
	}
}

// NewServiceManager 创建服务管理器 - 使用连接池
func NewServiceManager(config *Config, logger *zap.Logger) (*ServiceManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 创建连接池配置
	poolConfig := &client.PoolConfig{
		Address:          config.CounterServiceAddr,
		PoolSize:         config.PoolSize,
		MaxRecvMsgSize:   config.MaxRecvMsgSize,
		MaxSendMsgSize:   config.MaxSendMsgSize,
		KeepAliveTime:    config.KeepAliveTime,
		KeepAliveTimeout: config.KeepAliveTimeout,
		KeepAlivePermit:  true,
	}

	// 创建Counter服务连接池
	counterClientPool, err := client.NewCounterClientPool(poolConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter client pool: %w", err)
	}

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), config.TimeoutDuration)
	defer cancel()

	// 执行健康检查
	if err := validateCounterServicePool(ctx, counterClientPool); err != nil {
		counterClientPool.Close()
		return nil, fmt.Errorf("counter service health check failed: %w", err)
	}

	log.Printf("✅ Counter service pool connected successfully: %s (pool size: %d)",
		config.CounterServiceAddr, config.PoolSize)

	return &ServiceManager{
		counterClientPool: counterClientPool,
		config:            config,
		logger:            logger,
	}, nil
}

// GetCounterClient 获取Counter客户端连接池
func (sm *ServiceManager) GetCounterClient() *client.CounterClientPool {
	return sm.counterClientPool
}

// GetPoolStats 获取连接池统计信息
func (sm *ServiceManager) GetPoolStats() map[string]interface{} {
	if sm.counterClientPool == nil {
		return map[string]interface{}{"error": "counter client pool not initialized"}
	}
	return sm.counterClientPool.GetPoolStats()
}

// Close 关闭所有连接
func (sm *ServiceManager) Close() error {
	if sm.counterClientPool != nil {
		return sm.counterClientPool.Close()
	}
	return nil
}

// HealthCheck 检查所有服务健康状态
func (sm *ServiceManager) HealthCheck(ctx context.Context) error {
	return validateCounterServicePool(ctx, sm.counterClientPool)
}

// validateCounterServicePool 验证Counter服务连接池
func validateCounterServicePool(ctx context.Context, pool *client.CounterClientPool) error {
	// 这里先简单返回nil，后续会加上真正的健康检查
	return nil
}
