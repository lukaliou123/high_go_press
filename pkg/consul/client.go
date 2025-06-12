package consul

import (
	"fmt"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// Client Consul客户端封装
type Client struct {
	client *consulapi.Client
	logger *zap.Logger
}

// Config Consul客户端配置
type Config struct {
	Address string `yaml:"address"`
	Scheme  string `yaml:"scheme"`
	Token   string `yaml:"token"`
}

// ServiceConfig 服务注册配置
type ServiceConfig struct {
	ID      string            `yaml:"id"`
	Name    string            `yaml:"name"`
	Tags    []string          `yaml:"tags"`
	Address string            `yaml:"address"`
	Port    int               `yaml:"port"`
	Meta    map[string]string `yaml:"meta"`
	Check   *HealthCheck      `yaml:"check"`
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	HTTP                           string `yaml:"http"`
	GRPC                           string `yaml:"grpc"`
	TCP                            string `yaml:"tcp"`
	Interval                       string `yaml:"interval"`
	Timeout                        string `yaml:"timeout"`
	DeregisterCriticalServiceAfter string `yaml:"deregister_critical_service_after"`
}

// NewClient 创建Consul客户端
func NewClient(config *Config, logger *zap.Logger) (*Client, error) {
	consulConfig := consulapi.DefaultConfig()

	if config.Address != "" {
		consulConfig.Address = config.Address
	}

	if config.Scheme != "" {
		consulConfig.Scheme = config.Scheme
	}

	if config.Token != "" {
		consulConfig.Token = config.Token
	}

	client, err := consulapi.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &Client{
		client: client,
		logger: logger,
	}, nil
}

// RegisterService 注册服务到Consul
func (c *Client) RegisterService(config *ServiceConfig) error {
	service := &consulapi.AgentServiceRegistration{
		ID:      config.ID,
		Name:    config.Name,
		Tags:    config.Tags,
		Address: config.Address,
		Port:    config.Port,
		Meta:    config.Meta,
	}

	// 配置健康检查
	if config.Check != nil {
		check := &consulapi.AgentServiceCheck{
			Interval:                       config.Check.Interval,
			Timeout:                        config.Check.Timeout,
			DeregisterCriticalServiceAfter: config.Check.DeregisterCriticalServiceAfter,
		}

		if config.Check.HTTP != "" {
			check.HTTP = config.Check.HTTP
		} else if config.Check.GRPC != "" {
			check.GRPC = config.Check.GRPC
		} else if config.Check.TCP != "" {
			check.TCP = config.Check.TCP
		}

		service.Check = check
	}

	if err := c.client.Agent().ServiceRegister(service); err != nil {
		return fmt.Errorf("failed to register service %s: %w", config.Name, err)
	}

	c.logger.Info("Service registered successfully",
		zap.String("service_id", config.ID),
		zap.String("service_name", config.Name),
		zap.String("address", config.Address),
		zap.Int("port", config.Port))

	return nil
}

// DeregisterService 从Consul注销服务
func (c *Client) DeregisterService(serviceID string) error {
	if err := c.client.Agent().ServiceDeregister(serviceID); err != nil {
		return fmt.Errorf("failed to deregister service %s: %w", serviceID, err)
	}

	c.logger.Info("Service deregistered successfully",
		zap.String("service_id", serviceID))

	return nil
}

// DiscoverService 发现服务
func (c *Client) DiscoverService(serviceName string, healthy bool) ([]*ServiceInstance, error) {
	var services []*consulapi.ServiceEntry
	var err error

	if healthy {
		// 只返回健康的服务实例
		services, _, err = c.client.Health().Service(serviceName, "", true, nil)
	} else {
		// 返回所有服务实例
		services, _, err = c.client.Health().Service(serviceName, "", false, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to discover service %s: %w", serviceName, err)
	}

	instances := make([]*ServiceInstance, 0, len(services))
	for _, service := range services {
		instance := &ServiceInstance{
			ID:      service.Service.ID,
			Name:    service.Service.Service,
			Address: service.Service.Address,
			Port:    service.Service.Port,
			Tags:    service.Service.Tags,
			Meta:    service.Service.Meta,
		}

		// 设置健康状态
		instance.Healthy = true
		for _, check := range service.Checks {
			if check.Status != consulapi.HealthPassing {
				instance.Healthy = false
				break
			}
		}

		instances = append(instances, instance)
	}

	c.logger.Debug("Service discovery completed",
		zap.String("service_name", serviceName),
		zap.Int("instances_found", len(instances)),
		zap.Bool("healthy_only", healthy))

	return instances, nil
}

// ServiceInstance 服务实例信息
type ServiceInstance struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Address string            `json:"address"`
	Port    int               `json:"port"`
	Tags    []string          `json:"tags"`
	Meta    map[string]string `json:"meta"`
	Healthy bool              `json:"healthy"`
}

// GetAddress 获取服务实例的完整地址
func (s *ServiceInstance) GetAddress() string {
	return fmt.Sprintf("%s:%d", s.Address, s.Port)
}

// WatchService 监听服务变化
func (c *Client) WatchService(serviceName string, callback func([]*ServiceInstance)) error {
	// 创建一个简单的轮询机制
	// 在生产环境中，这里应该使用Consul的阻塞查询功能
	ticker := time.NewTicker(30 * time.Second)

	go func() {
		defer ticker.Stop()
		for range ticker.C {
			instances, err := c.DiscoverService(serviceName, true)
			if err != nil {
				c.logger.Error("Failed to discover service during watch",
					zap.String("service_name", serviceName),
					zap.Error(err))
				continue
			}
			callback(instances)
		}
	}()

	return nil
}

// Close 关闭Consul客户端
func (c *Client) Close() error {
	c.logger.Info("Consul client closed")
	return nil
}
