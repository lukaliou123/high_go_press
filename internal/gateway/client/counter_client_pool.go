package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "high-go-press/api/proto/counter"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// CounterClientPool gRPC连接池
type CounterClientPool struct {
	address     string
	poolSize    int
	connections []*grpc.ClientConn
	clients     []pb.CounterServiceClient
	index       int
	mutex       sync.RWMutex
	logger      *zap.Logger
}

// PoolConfig 连接池配置
type PoolConfig struct {
	Address              string
	PoolSize             int
	MaxRecvMsgSize       int
	MaxSendMsgSize       int
	InitialWindowSize    int32
	InitialConnWindow    int32
	MaxConcurrentStreams uint32
	KeepAliveTime        time.Duration
	KeepAliveTimeout     time.Duration
	KeepAlivePermit      bool
}

// DefaultPoolConfig 默认连接池配置
func DefaultPoolConfig(address string) *PoolConfig {
	return &PoolConfig{
		Address:              address,
		PoolSize:             20,              // 20个连接提供更好的并发性能
		MaxRecvMsgSize:       1024 * 1024 * 4, // 4MB
		MaxSendMsgSize:       1024 * 1024 * 4, // 4MB
		InitialWindowSize:    1024 * 1024,     // 1MB
		InitialConnWindow:    1024 * 1024 * 2, // 2MB
		MaxConcurrentStreams: 1000,
		KeepAliveTime:        30 * time.Second,
		KeepAliveTimeout:     5 * time.Second,
		KeepAlivePermit:      true,
	}
}

// NewCounterClientPool 创建Counter gRPC客户端连接池
func NewCounterClientPool(config *PoolConfig, logger *zap.Logger) (*CounterClientPool, error) {
	pool := &CounterClientPool{
		address:     config.Address,
		poolSize:    config.PoolSize,
		connections: make([]*grpc.ClientConn, config.PoolSize),
		clients:     make([]pb.CounterServiceClient, config.PoolSize),
		logger:      logger,
	}

	// gRPC连接选项优化
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(config.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(config.MaxSendMsgSize),
		),
		grpc.WithInitialWindowSize(config.InitialWindowSize),
		grpc.WithInitialConnWindowSize(config.InitialConnWindow),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.KeepAliveTime,
			Timeout:             config.KeepAliveTimeout,
			PermitWithoutStream: config.KeepAlivePermit,
		}),
		// 暂时禁用重试避免数据一致性问题
		grpc.WithDefaultServiceConfig(`{
			"methodConfig": [{
				"name": [{"service": "counter.CounterService"}],
				"retryPolicy": {
					"MaxAttempts": 1
				}
			}]
		}`),
	}

	// 创建连接池
	for i := 0; i < config.PoolSize; i++ {
		conn, err := grpc.Dial(config.Address, dialOpts...)
		if err != nil {
			// 清理已创建的连接
			pool.Close()
			return nil, fmt.Errorf("failed to create connection %d: %w", i, err)
		}

		pool.connections[i] = conn
		pool.clients[i] = pb.NewCounterServiceClient(conn)

		logger.Debug("Created gRPC connection",
			zap.Int("connection_id", i),
			zap.String("address", config.Address))
	}

	logger.Info("Counter gRPC client pool created",
		zap.String("address", config.Address),
		zap.Int("pool_size", config.PoolSize))

	return pool, nil
}

// getClient 获取下一个可用的客户端 (Round Robin)
func (p *CounterClientPool) getClient() pb.CounterServiceClient {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	client := p.clients[p.index]
	p.index = (p.index + 1) % p.poolSize
	return client
}

// IncrementCounter 增量计数器 - 使用连接池
func (p *CounterClientPool) IncrementCounter(ctx context.Context, req *pb.IncrementRequest) (*pb.IncrementResponse, error) {
	client := p.getClient()
	return client.IncrementCounter(ctx, req)
}

// GetCounter 获取计数器 - 使用连接池
func (p *CounterClientPool) GetCounter(ctx context.Context, req *pb.GetCounterRequest) (*pb.GetCounterResponse, error) {
	client := p.getClient()
	return client.GetCounter(ctx, req)
}

// BatchGetCounters 批量获取计数器 - 使用连接池
func (p *CounterClientPool) BatchGetCounters(ctx context.Context, req *pb.BatchGetRequest) (*pb.BatchGetResponse, error) {
	client := p.getClient()
	return client.BatchGetCounters(ctx, req)
}

// HealthCheck 健康检查 - 使用连接池
func (p *CounterClientPool) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	client := p.getClient()
	return client.HealthCheck(ctx, req)
}

// GetPoolStats 获取连接池统计信息
func (p *CounterClientPool) GetPoolStats() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["pool_size"] = p.poolSize
	stats["address"] = p.address
	stats["current_index"] = p.index

	// 检查连接状态
	readyConnections := 0
	for _, conn := range p.connections {
		if conn.GetState().String() == "READY" {
			readyConnections++
		}
	}
	stats["ready_connections"] = readyConnections
	stats["ready_rate"] = float64(readyConnections) / float64(p.poolSize)

	return stats
}

// Close 关闭连接池
func (p *CounterClientPool) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	var lastErr error
	for i, conn := range p.connections {
		if conn != nil {
			if err := conn.Close(); err != nil {
				p.logger.Error("Failed to close connection",
					zap.Int("connection_id", i),
					zap.Error(err))
				lastErr = err
			}
		}
	}

	p.logger.Info("Counter gRPC client pool closed")
	return lastErr
}
