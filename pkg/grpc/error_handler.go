package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorType 错误类型
type ErrorType int

const (
	// ErrorTypeUnknown 未知错误
	ErrorTypeUnknown ErrorType = iota
	// ErrorTypeValidation 参数验证错误
	ErrorTypeValidation
	// ErrorTypeBusiness 业务逻辑错误
	ErrorTypeBusiness
	// ErrorTypeSystem 系统错误
	ErrorTypeSystem
	// ErrorTypeNetwork 网络错误
	ErrorTypeNetwork
	// ErrorTypeTimeout 超时错误
	ErrorTypeTimeout
	// ErrorTypeRateLimit 限流错误
	ErrorTypeRateLimit
)

// ErrorInfo 错误信息
type ErrorInfo struct {
	Type        ErrorType
	Code        codes.Code
	Message     string
	Details     map[string]interface{}
	Timestamp   time.Time
	RequestID   string
	ServiceName string
	Method      string
	Retryable   bool
}

// ErrorStats 错误统计信息
type ErrorStats struct {
	TotalErrors      int64
	ValidationErrors int64
	BusinessErrors   int64
	SystemErrors     int64
	NetworkErrors    int64
	TimeoutErrors    int64
	RateLimitErrors  int64
	UnknownErrors    int64
	LastErrorTime    time.Time
	ErrorRate        float64
}

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	// HandleError 处理错误
	HandleError(ctx context.Context, err error, info *ErrorInfo) error
	// ShouldRetry 判断是否应该重试
	ShouldRetry(err error) bool
	// GetErrorType 获取错误类型
	GetErrorType(err error) ErrorType
}

// DefaultErrorHandler 默认错误处理器
type DefaultErrorHandler struct {
	logger *zap.Logger
	stats  ErrorStats
	mutex  sync.RWMutex
}

// NewDefaultErrorHandler 创建默认错误处理器
func NewDefaultErrorHandler(logger *zap.Logger) *DefaultErrorHandler {
	return &DefaultErrorHandler{
		logger: logger,
	}
}

// HandleError 处理错误
func (h *DefaultErrorHandler) HandleError(ctx context.Context, err error, info *ErrorInfo) error {
	h.mutex.Lock()
	h.stats.TotalErrors++
	h.stats.LastErrorTime = time.Now()

	// 根据错误类型更新统计
	switch info.Type {
	case ErrorTypeValidation:
		h.stats.ValidationErrors++
	case ErrorTypeBusiness:
		h.stats.BusinessErrors++
	case ErrorTypeSystem:
		h.stats.SystemErrors++
	case ErrorTypeNetwork:
		h.stats.NetworkErrors++
	case ErrorTypeTimeout:
		h.stats.TimeoutErrors++
	case ErrorTypeRateLimit:
		h.stats.RateLimitErrors++
	default:
		h.stats.UnknownErrors++
	}
	h.mutex.Unlock()

	// 记录错误日志
	h.logger.Error("Request failed",
		zap.String("error_type", h.getErrorTypeName(info.Type)),
		zap.String("grpc_code", info.Code.String()),
		zap.String("message", info.Message),
		zap.String("service", info.ServiceName),
		zap.String("method", info.Method),
		zap.String("request_id", info.RequestID),
		zap.Bool("retryable", info.Retryable),
		zap.Any("details", info.Details),
		zap.Error(err))

	// 转换为gRPC状态错误
	return status.Error(info.Code, info.Message)
}

// ShouldRetry 判断是否应该重试
func (h *DefaultErrorHandler) ShouldRetry(err error) bool {
	if grpcErr, ok := status.FromError(err); ok {
		code := grpcErr.Code()
		switch code {
		case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted:
			return true
		default:
			return false
		}
	}
	return false
}

// GetErrorType 获取错误类型
func (h *DefaultErrorHandler) GetErrorType(err error) ErrorType {
	if grpcErr, ok := status.FromError(err); ok {
		code := grpcErr.Code()
		switch code {
		case codes.InvalidArgument, codes.OutOfRange:
			return ErrorTypeValidation
		case codes.FailedPrecondition, codes.AlreadyExists, codes.NotFound:
			return ErrorTypeBusiness
		case codes.Internal, codes.DataLoss, codes.Unknown:
			return ErrorTypeSystem
		case codes.Unavailable:
			return ErrorTypeNetwork
		case codes.DeadlineExceeded:
			return ErrorTypeTimeout
		case codes.ResourceExhausted:
			return ErrorTypeRateLimit
		default:
			return ErrorTypeUnknown
		}
	}
	return ErrorTypeUnknown
}

// getErrorTypeName 获取错误类型名称
func (h *DefaultErrorHandler) getErrorTypeName(errorType ErrorType) string {
	switch errorType {
	case ErrorTypeValidation:
		return "validation"
	case ErrorTypeBusiness:
		return "business"
	case ErrorTypeSystem:
		return "system"
	case ErrorTypeNetwork:
		return "network"
	case ErrorTypeTimeout:
		return "timeout"
	case ErrorTypeRateLimit:
		return "rate_limit"
	default:
		return "unknown"
	}
}

// GetStats 获取错误统计信息
func (h *DefaultErrorHandler) GetStats() ErrorStats {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	stats := h.stats
	if stats.TotalErrors > 0 {
		// 计算错误率（简化实现）
		stats.ErrorRate = float64(stats.TotalErrors) / float64(stats.TotalErrors+1000) // 假设总请求数
	}

	return stats
}

// Reset 重置统计信息
func (h *DefaultErrorHandler) Reset() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.stats = ErrorStats{}
}

// ErrorMiddleware 错误处理中间件
type ErrorMiddleware struct {
	handler     ErrorHandler
	serviceName string
	logger      *zap.Logger
}

// NewErrorMiddleware 创建错误处理中间件
func NewErrorMiddleware(handler ErrorHandler, serviceName string, logger *zap.Logger) *ErrorMiddleware {
	return &ErrorMiddleware{
		handler:     handler,
		serviceName: serviceName,
		logger:      logger,
	}
}

// UnaryServerInterceptor 一元服务器拦截器
func (m *ErrorMiddleware) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 执行请求
		resp, err := handler(ctx, req)

		if err != nil {
			// 构建错误信息
			errorInfo := &ErrorInfo{
				Type:        m.handler.GetErrorType(err),
				Code:        status.Code(err),
				Message:     err.Error(),
				Details:     make(map[string]interface{}),
				Timestamp:   time.Now(),
				RequestID:   m.getRequestID(ctx),
				ServiceName: m.serviceName,
				Method:      info.FullMethod,
				Retryable:   m.handler.ShouldRetry(err),
			}

			// 处理错误
			processedErr := m.handler.HandleError(ctx, err, errorInfo)
			return resp, processedErr
		}

		return resp, nil
	}
}

// StreamServerInterceptor 流服务器拦截器
func (m *ErrorMiddleware) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// 包装流以捕获错误
		wrappedStream := &errorHandlingStream{
			ServerStream: ss,
			middleware:   m,
			info:         info,
		}

		err := handler(srv, wrappedStream)

		if err != nil {
			// 构建错误信息
			errorInfo := &ErrorInfo{
				Type:        m.handler.GetErrorType(err),
				Code:        status.Code(err),
				Message:     err.Error(),
				Details:     make(map[string]interface{}),
				Timestamp:   time.Now(),
				RequestID:   m.getRequestID(ss.Context()),
				ServiceName: m.serviceName,
				Method:      info.FullMethod,
				Retryable:   m.handler.ShouldRetry(err),
			}

			// 处理错误
			return m.handler.HandleError(ss.Context(), err, errorInfo)
		}

		return nil
	}
}

// errorHandlingStream 错误处理流包装器
type errorHandlingStream struct {
	grpc.ServerStream
	middleware *ErrorMiddleware
	info       *grpc.StreamServerInfo
}

// SendMsg 发送消息
func (s *errorHandlingStream) SendMsg(m interface{}) error {
	err := s.ServerStream.SendMsg(m)
	if err != nil {
		s.middleware.logger.Error("Stream send error",
			zap.String("method", s.info.FullMethod),
			zap.Error(err))
	}
	return err
}

// RecvMsg 接收消息
func (s *errorHandlingStream) RecvMsg(m interface{}) error {
	err := s.ServerStream.RecvMsg(m)
	if err != nil {
		s.middleware.logger.Error("Stream receive error",
			zap.String("method", s.info.FullMethod),
			zap.Error(err))
	}
	return err
}

// getRequestID 获取请求ID
func (m *ErrorMiddleware) getRequestID(ctx context.Context) string {
	// 尝试从上下文中获取请求ID
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}

	// 生成新的请求ID
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// BusinessError 业务错误
type BusinessError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("business error [%s]: %s", e.Code, e.Message)
}

// NewBusinessError 创建业务错误
func NewBusinessError(code, message string) *BusinessError {
	return &BusinessError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithDetails 添加错误详情
func (e *BusinessError) WithDetails(key string, value interface{}) *BusinessError {
	e.Details[key] = value
	return e
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// NewValidationError 创建验证错误
func NewValidationError(field, message string, value interface{}) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}

// SystemError 系统错误
type SystemError struct {
	Component string
	Operation string
	Cause     error
}

func (e *SystemError) Error() string {
	return fmt.Sprintf("system error in %s.%s: %v", e.Component, e.Operation, e.Cause)
}

func (e *SystemError) Unwrap() error {
	return e.Cause
}

// NewSystemError 创建系统错误
func NewSystemError(component, operation string, cause error) *SystemError {
	return &SystemError{
		Component: component,
		Operation: operation,
		Cause:     cause,
	}
}

// ErrorConverter 错误转换器
type ErrorConverter struct {
	logger *zap.Logger
}

// NewErrorConverter 创建错误转换器
func NewErrorConverter(logger *zap.Logger) *ErrorConverter {
	return &ErrorConverter{
		logger: logger,
	}
}

// ConvertError 转换错误为gRPC状态
func (c *ErrorConverter) ConvertError(err error) error {
	switch e := err.(type) {
	case *BusinessError:
		return status.Error(codes.FailedPrecondition, e.Message)
	case *ValidationError:
		return status.Error(codes.InvalidArgument, e.Message)
	case *SystemError:
		return status.Error(codes.Internal, e.Error())
	default:
		// 检查是否已经是gRPC错误
		if _, ok := status.FromError(err); ok {
			return err
		}
		// 默认转换为内部错误
		return status.Error(codes.Internal, err.Error())
	}
}

// IsRetryableError 检查是否为可重试错误
func (c *ErrorConverter) IsRetryableError(err error) bool {
	if grpcErr, ok := status.FromError(err); ok {
		code := grpcErr.Code()
		switch code {
		case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted:
			return true
		default:
			return false
		}
	}

	// 检查自定义错误类型
	switch err.(type) {
	case *SystemError:
		return true // 系统错误通常可以重试
	default:
		return false
	}
}
