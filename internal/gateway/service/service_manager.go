package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"high-go-press/internal/gateway/client"
)

// ServiceManager 微服务管理器
type ServiceManager struct {
	counterClient *client.CounterClient
	config        *Config
}

// Config 服务配置
type Config struct {
	CounterServiceAddr string
	TimeoutDuration    time.Duration
}

// NewServiceManager 创建服务管理器
func NewServiceManager(config *Config) (*ServiceManager, error) {
	// 创建Counter服务客户端
	counterClient, err := client.NewCounterClient(config.CounterServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter client: %w", err)
	}

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), config.TimeoutDuration)
	defer cancel()

	// 执行健康检查
	if err := validateCounterService(ctx, counterClient); err != nil {
		counterClient.Close()
		return nil, fmt.Errorf("counter service health check failed: %w", err)
	}

	log.Printf("✅ Counter service connected successfully: %s", config.CounterServiceAddr)

	return &ServiceManager{
		counterClient: counterClient,
		config:        config,
	}, nil
}

// GetCounterClient 获取Counter客户端
func (sm *ServiceManager) GetCounterClient() *client.CounterClient {
	return sm.counterClient
}

// Close 关闭所有连接
func (sm *ServiceManager) Close() error {
	if sm.counterClient != nil {
		return sm.counterClient.Close()
	}
	return nil
}

// HealthCheck 检查所有服务健康状态
func (sm *ServiceManager) HealthCheck(ctx context.Context) error {
	return validateCounterService(ctx, sm.counterClient)
}

// validateCounterService 验证Counter服务
func validateCounterService(ctx context.Context, client *client.CounterClient) error {
	// 这里先简单返回nil，后续会加上真正的健康检查
	return nil
}
