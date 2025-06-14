package grpc

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CircuitBreakerState 熔断器状态
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	// 失败阈值：连续失败多少次后开启熔断
	FailureThreshold int
	// 成功阈值：半开状态下连续成功多少次后关闭熔断
	SuccessThreshold int
	// 超时时间：熔断开启后多长时间尝试半开
	Timeout time.Duration
	// 最大请求数：半开状态下允许的最大请求数
	MaxRequests int
	// 统计窗口：失败率统计的时间窗口
	StatWindow time.Duration
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		MaxRequests:      10,
		StatWindow:       60 * time.Second,
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config *CircuitBreakerConfig
	logger *zap.Logger

	mutex         sync.RWMutex
	state         CircuitBreakerState
	failureCount  int
	successCount  int
	requestCount  int
	lastFailTime  time.Time
	lastStateTime time.Time

	// 统计信息
	stats CircuitBreakerStats
}

// CircuitBreakerStats 熔断器统计信息
type CircuitBreakerStats struct {
	TotalRequests    int64
	SuccessRequests  int64
	FailureRequests  int64
	RejectedRequests int64
	StateChanges     int64
	CurrentState     string
	LastStateChange  time.Time
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config *CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config:        config,
		logger:        logger,
		state:         StateClosed,
		lastStateTime: time.Now(),
		stats: CircuitBreakerStats{
			CurrentState:    StateClosed.String(),
			LastStateChange: time.Now(),
		},
	}
}

// Execute 执行函数，带熔断保护
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	// 检查是否允许执行
	if !cb.allowRequest() {
		cb.recordRejection()
		return errors.New("circuit breaker is open")
	}

	// 执行函数
	err := fn(ctx)

	// 记录结果
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// allowRequest 检查是否允许请求
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.stats.TotalRequests++

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以转为半开状态
		if time.Since(cb.lastStateTime) > cb.config.Timeout {
			cb.setState(StateHalfOpen)
			cb.requestCount = 0
			return true
		}
		return false
	case StateHalfOpen:
		// 半开状态下限制请求数量
		if cb.requestCount < cb.config.MaxRequests {
			cb.requestCount++
			return true
		}
		return false
	default:
		return false
	}
}

// recordSuccess 记录成功
func (cb *CircuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.stats.SuccessRequests++

	switch cb.state {
	case StateClosed:
		cb.failureCount = 0
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
			cb.reset()
		}
	}
}

// recordFailure 记录失败
func (cb *CircuitBreaker) recordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.stats.FailureRequests++
	cb.failureCount++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		cb.setState(StateOpen)
	}
}

// recordRejection 记录拒绝
func (cb *CircuitBreaker) recordRejection() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.stats.RejectedRequests++
}

// setState 设置状态
func (cb *CircuitBreaker) setState(state CircuitBreakerState) {
	if cb.state == state {
		return
	}

	oldState := cb.state
	cb.state = state
	cb.lastStateTime = time.Now()
	cb.stats.StateChanges++
	cb.stats.CurrentState = state.String()
	cb.stats.LastStateChange = time.Now()

	cb.logger.Info("Circuit breaker state changed",
		zap.String("from", oldState.String()),
		zap.String("to", state.String()),
		zap.Int("failure_count", cb.failureCount),
		zap.Int("success_count", cb.successCount))
}

// reset 重置计数器
func (cb *CircuitBreaker) reset() {
	cb.failureCount = 0
	cb.successCount = 0
	cb.requestCount = 0
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats 获取统计信息
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.stats
}

// IsOpen 检查熔断器是否开启
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.GetState() == StateOpen
}

// IsClosed 检查熔断器是否关闭
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.GetState() == StateClosed
}

// IsHalfOpen 检查熔断器是否半开
func (cb *CircuitBreaker) IsHalfOpen() bool {
	return cb.GetState() == StateHalfOpen
}
