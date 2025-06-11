package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"high-go-press/api/proto/common"
	"high-go-press/api/proto/counter"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
)

// SimpleCounterServer 简化的Counter服务实现
type SimpleCounterServer struct {
	counter.UnimplementedCounterServiceServer
	logger *zap.Logger
	store  map[string]int64 // 简单的内存存储
}

func NewSimpleCounterServer(logger *zap.Logger) *SimpleCounterServer {
	return &SimpleCounterServer{
		logger: logger,
		store:  make(map[string]int64),
	}
}

func (s *SimpleCounterServer) IncrementCounter(ctx context.Context, req *counter.IncrementRequest) (*counter.IncrementResponse, error) {
	if req.ResourceId == "" || req.CounterType == "" {
		return &counter.IncrementResponse{
			Status: &common.Status{
				Success: false,
				Message: "resource_id and counter_type are required",
				Code:    int32(codes.InvalidArgument),
			},
		}, nil
	}

	delta := req.Delta
	if delta == 0 {
		delta = 1
	}

	key := req.ResourceId + ":" + req.CounterType
	s.store[key] += delta

	s.logger.Info("Counter incremented",
		zap.String("key", key),
		zap.Int64("delta", delta),
		zap.Int64("new_value", s.store[key]))

	return &counter.IncrementResponse{
		Status: &common.Status{
			Success: true,
			Message: "Counter incremented successfully",
			Code:    int32(codes.OK),
		},
		CurrentValue: s.store[key],
		ResourceId:   req.ResourceId,
		CounterType:  req.CounterType,
	}, nil
}

func (s *SimpleCounterServer) GetCounter(ctx context.Context, req *counter.GetCounterRequest) (*counter.GetCounterResponse, error) {
	if req.ResourceId == "" || req.CounterType == "" {
		return &counter.GetCounterResponse{
			Status: &common.Status{
				Success: false,
				Message: "resource_id and counter_type are required",
				Code:    int32(codes.InvalidArgument),
			},
		}, nil
	}

	key := req.ResourceId + ":" + req.CounterType
	value := s.store[key]

	return &counter.GetCounterResponse{
		Status: &common.Status{
			Success: true,
			Message: "Counter retrieved successfully",
			Code:    int32(codes.OK),
		},
		Value:       value,
		ResourceId:  req.ResourceId,
		CounterType: req.CounterType,
		LastUpdated: &common.Timestamp{
			Seconds: time.Now().Unix(),
			Nanos:   int32(time.Now().Nanosecond()),
		},
	}, nil
}

func (s *SimpleCounterServer) BatchGetCounters(ctx context.Context, req *counter.BatchGetRequest) (*counter.BatchGetResponse, error) {
	results := make([]*counter.GetCounterResponse, 0, len(req.Requests))

	for _, r := range req.Requests {
		if r.ResourceId == "" || r.CounterType == "" {
			continue
		}

		key := r.ResourceId + ":" + r.CounterType
		value := s.store[key]

		results = append(results, &counter.GetCounterResponse{
			Status: &common.Status{
				Success: true,
				Message: "Success",
				Code:    int32(codes.OK),
			},
			Value:       value,
			ResourceId:  r.ResourceId,
			CounterType: r.CounterType,
			LastUpdated: &common.Timestamp{
				Seconds: time.Now().Unix(),
				Nanos:   int32(time.Now().Nanosecond()),
			},
		})
	}

	return &counter.BatchGetResponse{
		Status: &common.Status{
			Success: true,
			Message: "Batch get completed",
			Code:    int32(codes.OK),
		},
		Counters: results,
	}, nil
}

func (s *SimpleCounterServer) HealthCheck(ctx context.Context, req *counter.HealthCheckRequest) (*counter.HealthCheckResponse, error) {
	return &counter.HealthCheckResponse{
		Status: &common.Status{
			Success: true,
			Message: "Service is healthy",
			Code:    int32(codes.OK),
		},
		Service: "counter",
		Details: map[string]string{
			"status":     "healthy",
			"store_size": string(len(s.store)),
		},
	}, nil
}

func main() {
	// 创建logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("Starting Simple Counter microservice...")

	// 创建gRPC服务器
	grpcServer := grpc.NewServer()

	// 注册Counter服务
	counterSrv := NewSimpleCounterServer(logger)
	counter.RegisterCounterServiceServer(grpcServer, counterSrv)

	// 启用反射 (用于grpcurl等工具)
	reflection.Register(grpcServer)

	// 监听端口
	listen, err := net.Listen("tcp", ":9001")
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	// 启动服务器
	go func() {
		logger.Info("Counter gRPC server starting",
			zap.String("address", listen.Addr().String()))

		if err := grpcServer.Serve(listen); err != nil {
			logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Counter service...")

	// 优雅关闭
	grpcServer.GracefulStop()

	logger.Info("Counter service stopped gracefully")
}
