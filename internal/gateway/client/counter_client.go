package client

import (
	"context"
	"fmt"
	"time"

	pb "high-go-press/api/proto/counter"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CounterClient gRPC客户端封装
type CounterClient struct {
	conn   *grpc.ClientConn
	client pb.CounterServiceClient
}

// NewCounterClient 创建Counter gRPC客户端
func NewCounterClient(addr string) (*CounterClient, error) {
	// 创建gRPC连接
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to counter service: %w", err)
	}

	return &CounterClient{
		conn:   conn,
		client: pb.NewCounterServiceClient(conn),
	}, nil
}

// IncrementCounter 增量计数器
func (c *CounterClient) IncrementCounter(ctx context.Context, req *pb.IncrementRequest) (*pb.IncrementResponse, error) {
	return c.client.IncrementCounter(ctx, req)
}

// GetCounter 获取计数器
func (c *CounterClient) GetCounter(ctx context.Context, req *pb.GetCounterRequest) (*pb.GetCounterResponse, error) {
	return c.client.GetCounter(ctx, req)
}

// BatchGetCounters 批量获取计数器
func (c *CounterClient) BatchGetCounters(ctx context.Context, req *pb.BatchGetRequest) (*pb.BatchGetResponse, error) {
	return c.client.BatchGetCounters(ctx, req)
}

// HealthCheck 健康检查
func (c *CounterClient) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return c.client.HealthCheck(ctx, req)
}

// Close 关闭连接
func (c *CounterClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected 检查连接状态
func (c *CounterClient) IsConnected() bool {
	if c.conn == nil {
		return false
	}
	state := c.conn.GetState()
	return state.String() == "READY" || state.String() == "CONNECTING"
}
