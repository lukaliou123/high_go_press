package middleware

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"high-go-press/pkg/metrics"
)

// HTTPMetricsMiddleware HTTP 指标收集中间件
func HTTPMetricsMiddleware(metricsManager *metrics.MetricsManager, serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 增加正在处理的请求数
		metricsManager.IncHTTPInFlight(serviceName)
		defer metricsManager.DecHTTPInFlight(serviceName)

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(start)
		statusCode := strconv.Itoa(c.Writer.Status())

		metricsManager.RecordHTTPRequest(
			c.Request.Method,
			c.FullPath(),
			statusCode,
			serviceName,
			duration,
		)
	}
}

// GRPCMetricsUnaryInterceptor gRPC 一元调用指标收集拦截器
func GRPCMetricsUnaryInterceptor(metricsManager *metrics.MetricsManager, serviceName string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// 增加正在处理的请求数
		metricsManager.IncGRPCInFlight(serviceName)
		defer metricsManager.DecGRPCInFlight(serviceName)

		// 处理请求
		resp, err := handler(ctx, req)

		// 记录指标
		duration := time.Since(start)
		statusCode := "OK"
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code().String()
			} else {
				statusCode = "UNKNOWN"
			}
		}

		metricsManager.RecordGRPCRequest(
			info.FullMethod,
			serviceName,
			statusCode,
			duration,
		)

		return resp, err
	}
}

// GRPCMetricsStreamInterceptor gRPC 流式调用指标收集拦截器
func GRPCMetricsStreamInterceptor(metricsManager *metrics.MetricsManager, serviceName string) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		// 增加正在处理的请求数
		metricsManager.IncGRPCInFlight(serviceName)
		defer metricsManager.DecGRPCInFlight(serviceName)

		// 处理请求
		err := handler(srv, stream)

		// 记录指标
		duration := time.Since(start)
		statusCode := "OK"
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code().String()
			} else {
				statusCode = "UNKNOWN"
			}
		}

		metricsManager.RecordGRPCRequest(
			info.FullMethod,
			serviceName,
			statusCode,
			duration,
		)

		return err
	}
}

// BusinessMetricsWrapper 业务操作指标包装器
type BusinessMetricsWrapper struct {
	metricsManager *metrics.MetricsManager
	serviceName    string
	logger         *zap.Logger
}

// NewBusinessMetricsWrapper 创建业务指标包装器
func NewBusinessMetricsWrapper(metricsManager *metrics.MetricsManager, serviceName string, logger *zap.Logger) *BusinessMetricsWrapper {
	return &BusinessMetricsWrapper{
		metricsManager: metricsManager,
		serviceName:    serviceName,
		logger:         logger,
	}
}

// WrapOperation 包装业务操作
func (bmw *BusinessMetricsWrapper) WrapOperation(operation string, fn func() error) error {
	start := time.Now()

	err := fn()

	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}

	bmw.metricsManager.RecordBusinessOperation(operation, bmw.serviceName, status, duration)

	return err
}

// WrapOperationWithResult 包装带返回值的业务操作
func (bmw *BusinessMetricsWrapper) WrapOperationWithResult(operation string, fn func() (interface{}, error)) (interface{}, error) {
	start := time.Now()

	result, err := fn()

	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}

	bmw.metricsManager.RecordBusinessOperation(operation, bmw.serviceName, status, duration)

	return result, err
}

// SetGauge 设置业务指标值
func (bmw *BusinessMetricsWrapper) SetGauge(metric string, value float64) {
	bmw.metricsManager.SetBusinessGauge(metric, bmw.serviceName, value)
}

// DBMetricsWrapper 数据库操作指标包装器
type DBMetricsWrapper struct {
	metricsManager *metrics.MetricsManager
	serviceName    string
	databaseName   string
	logger         *zap.Logger
}

// NewDBMetricsWrapper 创建数据库指标包装器
func NewDBMetricsWrapper(metricsManager *metrics.MetricsManager, serviceName, databaseName string, logger *zap.Logger) *DBMetricsWrapper {
	return &DBMetricsWrapper{
		metricsManager: metricsManager,
		serviceName:    serviceName,
		databaseName:   databaseName,
		logger:         logger,
	}
}

// WrapQuery 包装数据库查询操作
func (dmw *DBMetricsWrapper) WrapQuery(operation string, fn func() error) error {
	start := time.Now()

	err := fn()

	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}

	dmw.metricsManager.RecordDBOperation(operation, dmw.databaseName, dmw.serviceName, status, duration)

	return err
}

// WrapQueryWithResult 包装带返回值的数据库查询操作
func (dmw *DBMetricsWrapper) WrapQueryWithResult(operation string, fn func() (interface{}, error)) (interface{}, error) {
	start := time.Now()

	result, err := fn()

	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}

	dmw.metricsManager.RecordDBOperation(operation, dmw.databaseName, dmw.serviceName, status, duration)

	return result, err
}

// UpdateConnections 更新数据库连接数
func (dmw *DBMetricsWrapper) UpdateConnections(active, idle int) {
	dmw.metricsManager.SetDBConnections(dmw.databaseName, dmw.serviceName, active, idle)
}

// CacheMetricsWrapper 缓存操作指标包装器
type CacheMetricsWrapper struct {
	metricsManager *metrics.MetricsManager
	serviceName    string
	cacheName      string
	logger         *zap.Logger
}

// NewCacheMetricsWrapper 创建缓存指标包装器
func NewCacheMetricsWrapper(metricsManager *metrics.MetricsManager, serviceName, cacheName string, logger *zap.Logger) *CacheMetricsWrapper {
	return &CacheMetricsWrapper{
		metricsManager: metricsManager,
		serviceName:    serviceName,
		cacheName:      cacheName,
		logger:         logger,
	}
}

// WrapGet 包装缓存获取操作
func (cmw *CacheMetricsWrapper) WrapGet(fn func() (interface{}, bool, error)) (interface{}, bool, error) {
	start := time.Now()

	result, hit, err := fn()

	duration := time.Since(start)

	cmw.metricsManager.RecordCacheOperation("get", cmw.cacheName, cmw.serviceName, hit, duration)

	return result, hit, err
}

// WrapSet 包装缓存设置操作
func (cmw *CacheMetricsWrapper) WrapSet(fn func() error) error {
	start := time.Now()

	err := fn()

	duration := time.Since(start)
	hit := err == nil // 设置成功视为 hit

	cmw.metricsManager.RecordCacheOperation("set", cmw.cacheName, cmw.serviceName, hit, duration)

	return err
}

// WrapDelete 包装缓存删除操作
func (cmw *CacheMetricsWrapper) WrapDelete(fn func() error) error {
	start := time.Now()

	err := fn()

	duration := time.Since(start)
	hit := err == nil // 删除成功视为 hit

	cmw.metricsManager.RecordCacheOperation("delete", cmw.cacheName, cmw.serviceName, hit, duration)

	return err
}
