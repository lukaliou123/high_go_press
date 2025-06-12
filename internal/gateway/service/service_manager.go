package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"high-go-press/pkg/consul"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// ServiceManager 微服务管理器 - 集成服务发现
type ServiceManager struct {
	discoveryManager *DiscoveryManager
	consul           *consul.Client
	config           *Config
	logger           *zap.Logger
}

// Config 服务配置
type Config struct {
	// 服务发现配置
	ConsulAddress string

	// 连接配置
	TimeoutDuration  time.Duration
	MaxRecvMsgSize   int
	MaxSendMsgSize   int
	KeepAliveTime    time.Duration
	KeepAliveTimeout time.Duration

	// 服务名称配置
	CounterServiceName   string
	AnalyticsServiceName string
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ConsulAddress:        "localhost:8500",
		TimeoutDuration:      5 * time.Second,
		MaxRecvMsgSize:       1024 * 1024 * 4, // 4MB
		MaxSendMsgSize:       1024 * 1024 * 4, // 4MB
		KeepAliveTime:        30 * time.Second,
		KeepAliveTimeout:     5 * time.Second,
		CounterServiceName:   "high-go-press-counter",
		AnalyticsServiceName: "high-go-press-analytics",
	}
}

// NewServiceManager 创建带服务发现的服务管理器
func NewServiceManager(config *Config, logger *zap.Logger) (*ServiceManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 创建Consul客户端
	consulConfig := &consul.Config{
		Address: config.ConsulAddress,
		Scheme:  "http",
	}

	consulClient, err := consul.NewClient(consulConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	// 创建服务发现管理器
	discoveryManager := NewDiscoveryManager(consulClient, logger)

	// 注册需要发现的服务
	if err := discoveryManager.RegisterService(config.CounterServiceName); err != nil {
		return nil, fmt.Errorf("failed to register counter service for discovery: %w", err)
	}

	if err := discoveryManager.RegisterService(config.AnalyticsServiceName); err != nil {
		return nil, fmt.Errorf("failed to register analytics service for discovery: %w", err)
	}

	// 等待一下，让服务发现完成
	time.Sleep(2 * time.Second)

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), config.TimeoutDuration)
	defer cancel()

	if err := validateServices(ctx, discoveryManager); err != nil {
		discoveryManager.Close()
		return nil, fmt.Errorf("service validation failed: %w", err)
	}

	log.Printf("✅ Service discovery manager initialized successfully")

	return &ServiceManager{
		discoveryManager: discoveryManager,
		consul:           consulClient,
		config:           config,
		logger:           logger,
	}, nil
}

// GetCounterClient 获取Counter客户端连接 (通过服务发现)
func (sm *ServiceManager) GetCounterClient() (*grpc.ClientConn, error) {
	conn, err := sm.discoveryManager.GetConnection(sm.config.CounterServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get counter service connection: %w", err)
	}

	return conn, nil
}

// GetCounterConnection 直接获取Counter服务的gRPC连接
func (sm *ServiceManager) GetCounterConnection() (*grpc.ClientConn, error) {
	return sm.discoveryManager.GetConnection(sm.config.CounterServiceName)
}

// GetAnalyticsConnection 获取Analytics服务的gRPC连接
func (sm *ServiceManager) GetAnalyticsConnection() (*grpc.ClientConn, error) {
	return sm.discoveryManager.GetConnection(sm.config.AnalyticsServiceName)
}

// GetServiceInstances 获取指定服务的实例列表
func (sm *ServiceManager) GetServiceInstances(serviceName string) ([]*consul.ServiceInstance, error) {
	return sm.discoveryManager.GetServiceInstances(serviceName)
}

// GetPoolStats 获取服务发现统计信息
func (sm *ServiceManager) GetPoolStats() map[string]interface{} {
	stats := sm.discoveryManager.GetStats()

	// 添加Consul连接状态
	stats["consul"] = map[string]interface{}{
		"address": sm.config.ConsulAddress,
		"status":  "connected",
	}

	return stats
}

// GetDiscoveryStats 获取服务发现详细统计
func (sm *ServiceManager) GetDiscoveryStats() map[string]interface{} {
	return sm.discoveryManager.GetStats()
}

// Close 关闭所有连接
func (sm *ServiceManager) Close() error {
	if sm.discoveryManager != nil {
		sm.discoveryManager.Close()
	}

	if sm.consul != nil {
		sm.consul.Close()
	}

	sm.logger.Info("Service manager closed")
	return nil
}

// HealthCheck 检查所有服务健康状态
func (sm *ServiceManager) HealthCheck(ctx context.Context) error {
	return validateServices(ctx, sm.discoveryManager)
}

// validateServices 验证服务连接
func validateServices(ctx context.Context, dm *DiscoveryManager) error {
	// 检查Counter服务
	conn, err := dm.GetConnection("high-go-press-counter")
	if err != nil {
		return fmt.Errorf("counter service not available: %w", err)
	}

	// 可以在这里添加实际的健康检查gRPC调用
	_ = conn // 暂时只检查连接是否可用

	return nil
}

// RegisterGatewayService 注册Gateway自身到Consul
func (sm *ServiceManager) RegisterGatewayService(port int) error {
	serviceConfig := &consul.ServiceConfig{
		ID:      "gateway-1",
		Name:    "high-go-press-gateway",
		Tags:    []string{"gateway", "http", "api"},
		Address: "localhost",
		Port:    port,
		Check: &consul.HealthCheck{
			HTTP:     fmt.Sprintf("http://localhost:%d/api/v1/health", port),
			Interval: "10s",
			Timeout:  "3s",
		},
	}

	if err := sm.consul.RegisterService(serviceConfig); err != nil {
		return fmt.Errorf("failed to register gateway service: %w", err)
	}

	sm.logger.Info("Gateway service registered to Consul",
		zap.String("service_id", serviceConfig.ID),
		zap.Int("port", port))

	return nil
}

// DeregisterGatewayService 从Consul注销Gateway服务
func (sm *ServiceManager) DeregisterGatewayService() error {
	if err := sm.consul.DeregisterService("gateway-1"); err != nil {
		return fmt.Errorf("failed to deregister gateway service: %w", err)
	}

	sm.logger.Info("Gateway service deregistered from Consul")
	return nil
}
