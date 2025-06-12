package service

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"high-go-press/pkg/consul"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// DiscoveryManager 服务发现管理器
type DiscoveryManager struct {
	consul     *consul.Client
	logger     *zap.Logger
	services   map[string]*ServiceEndpoints
	serviceMux sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// ServiceEndpoints 服务端点信息
type ServiceEndpoints struct {
	Name        string
	Connections []*grpc.ClientConn
	Instances   []*consul.ServiceInstance
	LastUpdated time.Time
	mutex       sync.RWMutex
}

// NewDiscoveryManager 创建服务发现管理器
func NewDiscoveryManager(consulClient *consul.Client, logger *zap.Logger) *DiscoveryManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &DiscoveryManager{
		consul:   consulClient,
		logger:   logger,
		services: make(map[string]*ServiceEndpoints),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterService 注册需要发现的服务
func (dm *DiscoveryManager) RegisterService(serviceName string) error {
	dm.serviceMux.Lock()
	defer dm.serviceMux.Unlock()

	if _, exists := dm.services[serviceName]; exists {
		return fmt.Errorf("service %s already registered", serviceName)
	}

	dm.services[serviceName] = &ServiceEndpoints{
		Name:        serviceName,
		Connections: make([]*grpc.ClientConn, 0),
		Instances:   make([]*consul.ServiceInstance, 0),
		LastUpdated: time.Now(),
	}

	// 立即发现一次服务
	if err := dm.updateService(serviceName); err != nil {
		dm.logger.Error("Failed to update service on registration",
			zap.String("service", serviceName),
			zap.Error(err))
		return err
	}

	// 启动服务监听
	go dm.watchService(serviceName)

	dm.logger.Info("Service registered for discovery",
		zap.String("service", serviceName))

	return nil
}

// GetConnection 获取服务的gRPC连接（负载均衡）
func (dm *DiscoveryManager) GetConnection(serviceName string) (*grpc.ClientConn, error) {
	dm.serviceMux.RLock()
	service, exists := dm.services[serviceName]
	dm.serviceMux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("service %s not registered", serviceName)
	}

	service.mutex.RLock()
	defer service.mutex.RUnlock()

	if len(service.Connections) == 0 {
		return nil, fmt.Errorf("no healthy connections available for service %s", serviceName)
	}

	// 简单的轮询负载均衡
	index := rand.Intn(len(service.Connections))
	conn := service.Connections[index]

	// 检查连接状态
	if conn.GetState() == connectivity.TransientFailure || conn.GetState() == connectivity.Shutdown {
		dm.logger.Warn("Connection unhealthy, triggering service update",
			zap.String("service", serviceName),
			zap.String("state", conn.GetState().String()))

		// 异步更新服务
		go dm.updateService(serviceName)

		// 返回任意一个连接，让调用者处理失败
		return conn, nil
	}

	return conn, nil
}

// GetServiceInstances 获取服务实例列表
func (dm *DiscoveryManager) GetServiceInstances(serviceName string) ([]*consul.ServiceInstance, error) {
	dm.serviceMux.RLock()
	service, exists := dm.services[serviceName]
	dm.serviceMux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("service %s not registered", serviceName)
	}

	service.mutex.RLock()
	defer service.mutex.RUnlock()

	// 返回实例的副本
	instances := make([]*consul.ServiceInstance, len(service.Instances))
	copy(instances, service.Instances)

	return instances, nil
}

// watchService 监听服务变化
func (dm *DiscoveryManager) watchService(serviceName string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			if err := dm.updateService(serviceName); err != nil {
				dm.logger.Error("Failed to update service",
					zap.String("service", serviceName),
					zap.Error(err))
			}
		}
	}
}

// updateService 更新服务端点
func (dm *DiscoveryManager) updateService(serviceName string) error {
	// 从Consul发现服务实例
	instances, err := dm.consul.DiscoverService(serviceName, true)
	if err != nil {
		return fmt.Errorf("failed to discover service %s: %w", serviceName, err)
	}

	dm.serviceMux.RLock()
	service, exists := dm.services[serviceName]
	dm.serviceMux.RUnlock()

	if !exists {
		return fmt.Errorf("service %s not registered", serviceName)
	}

	service.mutex.Lock()
	defer service.mutex.Unlock()

	// 检查是否有变化
	if !dm.instancesChanged(service.Instances, instances) {
		dm.logger.Debug("No changes in service instances",
			zap.String("service", serviceName))
		return nil
	}

	// 关闭旧连接
	for _, conn := range service.Connections {
		conn.Close()
	}

	// 创建新连接
	newConnections := make([]*grpc.ClientConn, 0, len(instances))
	for _, instance := range instances {
		conn, err := dm.createConnection(instance.GetAddress())
		if err != nil {
			dm.logger.Warn("Failed to create connection to instance",
				zap.String("service", serviceName),
				zap.String("address", instance.GetAddress()),
				zap.Error(err))
			continue
		}
		newConnections = append(newConnections, conn)
	}

	// 更新服务信息
	service.Connections = newConnections
	service.Instances = instances
	service.LastUpdated = time.Now()

	dm.logger.Info("Service endpoints updated",
		zap.String("service", serviceName),
		zap.Int("instances", len(instances)),
		zap.Int("connections", len(newConnections)))

	return nil
}

// createConnection 创建gRPC连接
func (dm *DiscoveryManager) createConnection(address string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithInsecure(), // 开发环境，生产环境应使用TLS
		grpc.WithBlock(),
		grpc.WithDefaultServiceConfig(`{
			"methodConfig": [{
				"name": [{"service": ""}],
				"retryPolicy": {
					"MaxAttempts": 3,
					"InitialBackoff": "0.1s",
					"MaxBackoff": "1s",
					"BackoffMultiplier": 2.0,
					"RetryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
				},
				"timeout": "5s"
			}]
		}`),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	return conn, nil
}

// instancesChanged 检查服务实例是否有变化
func (dm *DiscoveryManager) instancesChanged(old, new []*consul.ServiceInstance) bool {
	if len(old) != len(new) {
		return true
	}

	// 创建地址映射进行比较
	oldAddrs := make(map[string]bool)
	for _, instance := range old {
		oldAddrs[instance.GetAddress()] = true
	}

	for _, instance := range new {
		if !oldAddrs[instance.GetAddress()] {
			return true
		}
	}

	return false
}

// GetStats 获取服务发现统计信息
func (dm *DiscoveryManager) GetStats() map[string]interface{} {
	dm.serviceMux.RLock()
	defer dm.serviceMux.RUnlock()

	stats := make(map[string]interface{})

	for name, service := range dm.services {
		service.mutex.RLock()
		stats[name] = map[string]interface{}{
			"instances":    len(service.Instances),
			"connections":  len(service.Connections),
			"last_updated": service.LastUpdated.Unix(),
		}
		service.mutex.RUnlock()
	}

	return stats
}

// Close 关闭服务发现管理器
func (dm *DiscoveryManager) Close() error {
	dm.cancel()

	dm.serviceMux.Lock()
	defer dm.serviceMux.Unlock()

	// 关闭所有连接
	for _, service := range dm.services {
		service.mutex.Lock()
		for _, conn := range service.Connections {
			conn.Close()
		}
		service.mutex.Unlock()
	}

	dm.logger.Info("Discovery manager closed")
	return nil
}
