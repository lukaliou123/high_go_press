package grpc

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ResilienceConfig 弹性配置
type ResilienceConfig struct {
	// 熔断器配置
	CircuitBreaker *CircuitBreakerConfig
	// 重试配置
	Retry *RetryConfig
	// 降级配置
	Fallback *FallbackConfig
	// 错误处理配置
	ErrorHandling *ErrorHandlingConfig
}

// ErrorHandlingConfig 错误处理配置
type ErrorHandlingConfig struct {
	// 启用错误处理
	Enabled bool
	// 错误统计窗口
	StatsWindow time.Duration
	// 错误率阈值
	ErrorRateThreshold float64
	// 错误日志级别
	LogLevel string
}

// ResilienceManager 弹性管理器
type ResilienceManager struct {
	config          *ResilienceConfig
	circuitBreaker  *CircuitBreaker
	retryer         *Retryer
	fallbackManager *FallbackManager
	errorHandler    ErrorHandler
	errorConverter  *ErrorConverter
	logger          *zap.Logger
	stats           ResilienceStats
	mutex           sync.RWMutex
}

// ResilienceStats 弹性统计信息
type ResilienceStats struct {
	TotalRequests       int64
	SuccessRequests     int64
	FailedRequests      int64
	CircuitBreakerTrips int64
	RetryAttempts       int64
	FallbackExecutions  int64
	AvgResponseTime     time.Duration
	LastRequestTime     time.Time
	SuccessRate         float64
}

// NewResilienceManager 创建弹性管理器
func NewResilienceManager(config *ResilienceConfig, logger *zap.Logger) *ResilienceManager {
	if config == nil {
		config = DefaultResilienceConfig()
	}

	rm := &ResilienceManager{
		config: config,
		logger: logger,
	}

	// 初始化组件
	rm.initComponents()

	return rm
}

// initComponents 初始化组件
func (rm *ResilienceManager) initComponents() {
	// 初始化熔断器
	if rm.config.CircuitBreaker != nil {
		rm.circuitBreaker = NewCircuitBreaker(rm.config.CircuitBreaker, rm.logger)
	}

	// 初始化重试器
	if rm.config.Retry != nil {
		rm.retryer = NewRetryer(rm.config.Retry, rm.logger)
	}

	// 初始化降级管理器
	if rm.config.Fallback != nil {
		rm.fallbackManager = NewFallbackManager(rm.config.Fallback, rm.logger)
	}

	// 初始化错误处理器
	if rm.config.ErrorHandling != nil && rm.config.ErrorHandling.Enabled {
		rm.errorHandler = NewDefaultErrorHandler(rm.logger)
		rm.errorConverter = NewErrorConverter(rm.logger)
	}
}

// Execute 执行带弹性保护的函数
func (rm *ResilienceManager) Execute(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	startTime := time.Now()

	rm.mutex.Lock()
	rm.stats.TotalRequests++
	rm.stats.LastRequestTime = startTime
	rm.mutex.Unlock()

	var result interface{}
	var err error

	// 执行函数，应用所有弹性策略
	if rm.circuitBreaker != nil {
		// 使用熔断器保护
		err = rm.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
			result, err = rm.executeWithRetry(ctx, fn)
			return err
		})
	} else {
		// 直接执行重试逻辑
		result, err = rm.executeWithRetry(ctx, fn)
	}

	// 如果有错误且配置了降级，尝试降级
	if err != nil && rm.fallbackManager != nil {
		result, err = rm.fallbackManager.Execute(ctx, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
			return result, err
		})
	}

	// 更新统计信息
	duration := time.Since(startTime)
	rm.updateStats(err == nil, duration)

	// 处理错误
	if err != nil && rm.errorHandler != nil {
		errorInfo := &ErrorInfo{
			Type:        rm.errorHandler.GetErrorType(err),
			Code:        0, // 需要从错误中提取
			Message:     err.Error(),
			Details:     make(map[string]interface{}),
			Timestamp:   time.Now(),
			RequestID:   rm.getRequestID(ctx),
			ServiceName: "resilience-manager",
			Method:      "execute",
			Retryable:   rm.errorHandler.ShouldRetry(err),
		}

		err = rm.errorHandler.HandleError(ctx, err, errorInfo)
	}

	return result, err
}

// executeWithRetry 执行带重试的函数
func (rm *ResilienceManager) executeWithRetry(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	if rm.retryer != nil {
		var result interface{}
		err := rm.retryer.Execute(ctx, func(ctx context.Context) error {
			var execErr error
			result, execErr = fn(ctx)
			return execErr
		})
		return result, err
	}

	// 没有重试器，直接执行
	return fn(ctx)
}

// updateStats 更新统计信息
func (rm *ResilienceManager) updateStats(success bool, duration time.Duration) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if success {
		rm.stats.SuccessRequests++
	} else {
		rm.stats.FailedRequests++
	}

	// 更新平均响应时间
	if rm.stats.TotalRequests > 0 {
		rm.stats.AvgResponseTime = time.Duration(
			(int64(rm.stats.AvgResponseTime)*rm.stats.TotalRequests + int64(duration)) / (rm.stats.TotalRequests + 1),
		)
	} else {
		rm.stats.AvgResponseTime = duration
	}

	// 计算成功率
	if rm.stats.TotalRequests > 0 {
		rm.stats.SuccessRate = float64(rm.stats.SuccessRequests) / float64(rm.stats.TotalRequests)
	}

	// 更新其他统计信息
	if rm.circuitBreaker != nil {
		cbStats := rm.circuitBreaker.GetStats()
		rm.stats.CircuitBreakerTrips = cbStats.StateChanges
	}

	if rm.retryer != nil {
		retryStats := rm.retryer.GetStats()
		rm.stats.RetryAttempts = retryStats.RetriedRequests
	}

	if rm.fallbackManager != nil {
		fallbackStats := rm.fallbackManager.GetStats()
		rm.stats.FallbackExecutions = fallbackStats.TotalFallbacks
	}
}

// getRequestID 获取请求ID
func (rm *ResilienceManager) getRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return "unknown"
}

// GetStats 获取弹性统计信息
func (rm *ResilienceManager) GetStats() ResilienceStats {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return rm.stats
}

// GetDetailedStats 获取详细统计信息
func (rm *ResilienceManager) GetDetailedStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 基础统计
	stats["resilience"] = rm.GetStats()

	// 熔断器统计
	if rm.circuitBreaker != nil {
		stats["circuit_breaker"] = rm.circuitBreaker.GetStats()
	}

	// 重试统计
	if rm.retryer != nil {
		stats["retry"] = rm.retryer.GetStats()
	}

	// 降级统计
	if rm.fallbackManager != nil {
		stats["fallback"] = rm.fallbackManager.GetStats()
	}

	// 错误处理统计
	if rm.errorHandler != nil {
		if defaultHandler, ok := rm.errorHandler.(*DefaultErrorHandler); ok {
			stats["error_handling"] = defaultHandler.GetStats()
		}
	}

	return stats
}

// Reset 重置所有统计信息
func (rm *ResilienceManager) Reset() {
	rm.mutex.Lock()
	rm.stats = ResilienceStats{}
	rm.mutex.Unlock()

	if rm.circuitBreaker != nil {
		// 熔断器不提供重置方法，这里可以重新创建
	}

	if rm.retryer != nil {
		rm.retryer.Reset()
	}

	if rm.fallbackManager != nil {
		rm.fallbackManager.Reset()
	}

	if rm.errorHandler != nil {
		if defaultHandler, ok := rm.errorHandler.(*DefaultErrorHandler); ok {
			defaultHandler.Reset()
		}
	}
}

// IsHealthy 检查系统健康状态
func (rm *ResilienceManager) IsHealthy() bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	// 检查成功率
	if rm.stats.TotalRequests > 10 && rm.stats.SuccessRate < 0.8 {
		return false
	}

	// 检查熔断器状态
	if rm.circuitBreaker != nil && rm.circuitBreaker.IsOpen() {
		return false
	}

	return true
}

// GetHealthStatus 获取健康状态详情
func (rm *ResilienceManager) GetHealthStatus() map[string]interface{} {
	status := make(map[string]interface{})

	status["healthy"] = rm.IsHealthy()
	status["success_rate"] = rm.stats.SuccessRate
	status["avg_response_time"] = rm.stats.AvgResponseTime.String()

	if rm.circuitBreaker != nil {
		status["circuit_breaker_state"] = rm.circuitBreaker.GetState().String()
		status["circuit_breaker_open"] = rm.circuitBreaker.IsOpen()
	}

	return status
}

// DefaultResilienceConfig 默认弹性配置
func DefaultResilienceConfig() *ResilienceConfig {
	return &ResilienceConfig{
		CircuitBreaker: DefaultCircuitBreakerConfig(),
		Retry:          DefaultRetryConfig(),
		Fallback:       DefaultFallbackConfig(),
		ErrorHandling: &ErrorHandlingConfig{
			Enabled:            true,
			StatsWindow:        5 * time.Minute,
			ErrorRateThreshold: 0.1, // 10%错误率
			LogLevel:           "error",
		},
	}
}

// ResilienceMiddleware 弹性中间件
type ResilienceMiddleware struct {
	manager *ResilienceManager
	logger  *zap.Logger
}

// NewResilienceMiddleware 创建弹性中间件
func NewResilienceMiddleware(manager *ResilienceManager, logger *zap.Logger) *ResilienceMiddleware {
	return &ResilienceMiddleware{
		manager: manager,
		logger:  logger,
	}
}

// WrapFunction 包装函数，添加弹性保护
func (m *ResilienceMiddleware) WrapFunction(fn func(context.Context) (interface{}, error)) func(context.Context) (interface{}, error) {
	return func(ctx context.Context) (interface{}, error) {
		return m.manager.Execute(ctx, fn)
	}
}

// WrapGRPCCall 包装gRPC调用
func (m *ResilienceMiddleware) WrapGRPCCall(call func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := m.manager.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			return nil, call(ctx)
		})
		return err
	}
}
