package grpc

import (
	"context"
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryConfig 重试配置
type RetryConfig struct {
	// 最大重试次数
	MaxAttempts int
	// 初始退避时间
	InitialBackoff time.Duration
	// 最大退避时间
	MaxBackoff time.Duration
	// 退避倍数
	BackoffMultiplier float64
	// 抖动因子 (0-1)
	Jitter float64
	// 可重试的错误码
	RetryableStatusCodes []codes.Code
	// 重试超时时间
	RetryTimeout time.Duration
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
		RetryableStatusCodes: []codes.Code{
			codes.Unavailable,
			codes.DeadlineExceeded,
			codes.ResourceExhausted,
			codes.Aborted,
			codes.Internal,
		},
		RetryTimeout: 60 * time.Second,
	}
}

// RetryStats 重试统计信息
type RetryStats struct {
	TotalAttempts   int64
	SuccessAttempts int64
	FailedAttempts  int64
	RetriedRequests int64
	TotalRetryDelay time.Duration
	MaxRetryDelay   time.Duration
	AvgRetryDelay   time.Duration
}

// Retryer 重试器
type Retryer struct {
	config *RetryConfig
	logger *zap.Logger
	stats  RetryStats
}

// NewRetryer 创建重试器
func NewRetryer(config *RetryConfig, logger *zap.Logger) *Retryer {
	if config == nil {
		config = DefaultRetryConfig()
	}

	return &Retryer{
		config: config,
		logger: logger,
	}
}

// Execute 执行函数，带重试机制
func (r *Retryer) Execute(ctx context.Context, fn func(context.Context) error) error {
	// 创建重试上下文
	retryCtx, cancel := context.WithTimeout(ctx, r.config.RetryTimeout)
	defer cancel()

	var lastErr error
	backoff := r.config.InitialBackoff

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		r.stats.TotalAttempts++

		// 执行函数
		err := fn(retryCtx)
		if err == nil {
			r.stats.SuccessAttempts++
			if attempt > 1 {
				r.logger.Info("Request succeeded after retry",
					zap.Int("attempt", attempt),
					zap.Duration("total_delay", r.getTotalDelay(attempt-1)))
			}
			return nil
		}

		lastErr = err
		r.stats.FailedAttempts++

		// 检查是否应该重试
		if !r.shouldRetry(err, attempt) {
			break
		}

		// 如果不是最后一次尝试，则等待退避时间
		if attempt < r.config.MaxAttempts {
			delay := r.calculateBackoff(backoff)
			r.stats.TotalRetryDelay += delay
			if delay > r.stats.MaxRetryDelay {
				r.stats.MaxRetryDelay = delay
			}
			r.stats.RetriedRequests++

			r.logger.Warn("Request failed, retrying",
				zap.Int("attempt", attempt),
				zap.Int("max_attempts", r.config.MaxAttempts),
				zap.Duration("delay", delay),
				zap.Error(err))

			// 等待退避时间
			select {
			case <-time.After(delay):
				// 继续重试
			case <-retryCtx.Done():
				return retryCtx.Err()
			}

			// 计算下次退避时间
			backoff = r.nextBackoff(backoff)
		}
	}

	r.logger.Error("Request failed after all retries",
		zap.Int("max_attempts", r.config.MaxAttempts),
		zap.Error(lastErr))

	return lastErr
}

// shouldRetry 判断是否应该重试
func (r *Retryer) shouldRetry(err error, attempt int) bool {
	// 检查重试次数
	if attempt >= r.config.MaxAttempts {
		return false
	}

	// 检查错误类型
	if grpcErr, ok := status.FromError(err); ok {
		code := grpcErr.Code()
		for _, retryableCode := range r.config.RetryableStatusCodes {
			if code == retryableCode {
				return true
			}
		}
	}

	return false
}

// calculateBackoff 计算退避时间（带抖动）
func (r *Retryer) calculateBackoff(baseBackoff time.Duration) time.Duration {
	// 确保不超过最大退避时间
	if baseBackoff > r.config.MaxBackoff {
		baseBackoff = r.config.MaxBackoff
	}

	// 添加抖动
	if r.config.Jitter > 0 {
		jitterRange := float64(baseBackoff) * r.config.Jitter
		jitter := (rand.Float64() - 0.5) * 2 * jitterRange
		backoff := float64(baseBackoff) + jitter

		// 确保不为负数
		if backoff < 0 {
			backoff = float64(baseBackoff) * 0.1
		}

		return time.Duration(backoff)
	}

	return baseBackoff
}

// nextBackoff 计算下次退避时间
func (r *Retryer) nextBackoff(currentBackoff time.Duration) time.Duration {
	nextBackoff := time.Duration(float64(currentBackoff) * r.config.BackoffMultiplier)
	if nextBackoff > r.config.MaxBackoff {
		return r.config.MaxBackoff
	}
	return nextBackoff
}

// getTotalDelay 获取总延迟时间
func (r *Retryer) getTotalDelay(retryCount int) time.Duration {
	var totalDelay time.Duration
	backoff := r.config.InitialBackoff

	for i := 0; i < retryCount; i++ {
		totalDelay += r.calculateBackoff(backoff)
		backoff = r.nextBackoff(backoff)
	}

	return totalDelay
}

// GetStats 获取重试统计信息
func (r *Retryer) GetStats() RetryStats {
	stats := r.stats
	if stats.RetriedRequests > 0 {
		stats.AvgRetryDelay = time.Duration(int64(stats.TotalRetryDelay) / stats.RetriedRequests)
	}
	return stats
}

// Reset 重置统计信息
func (r *Retryer) Reset() {
	r.stats = RetryStats{}
}

// RetryableError 可重试错误包装
type RetryableError struct {
	Err       error
	Retryable bool
	Code      codes.Code
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError 创建可重试错误
func NewRetryableError(err error, retryable bool) *RetryableError {
	code := codes.Unknown
	if grpcErr, ok := status.FromError(err); ok {
		code = grpcErr.Code()
	}

	return &RetryableError{
		Err:       err,
		Retryable: retryable,
		Code:      code,
	}
}

// IsRetryableError 检查是否为可重试错误
func IsRetryableError(err error) bool {
	if retryableErr, ok := err.(*RetryableError); ok {
		return retryableErr.Retryable
	}

	// 检查gRPC错误码
	if grpcErr, ok := status.FromError(err); ok {
		code := grpcErr.Code()
		retryableCodes := []codes.Code{
			codes.Unavailable,
			codes.DeadlineExceeded,
			codes.ResourceExhausted,
			codes.Aborted,
		}

		for _, retryableCode := range retryableCodes {
			if code == retryableCode {
				return true
			}
		}
	}

	return false
}

// ExponentialBackoff 指数退避计算器
type ExponentialBackoff struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	MaxElapsedTime  time.Duration
	currentInterval time.Duration
	startTime       time.Time
}

// NewExponentialBackoff 创建指数退避计算器
func NewExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     60 * time.Second,
		Multiplier:      1.5,
		MaxElapsedTime:  15 * time.Minute,
		currentInterval: 500 * time.Millisecond,
		startTime:       time.Now(),
	}
}

// NextBackOff 获取下次退避时间
func (eb *ExponentialBackoff) NextBackOff() time.Duration {
	if eb.MaxElapsedTime != 0 && time.Since(eb.startTime) > eb.MaxElapsedTime {
		return -1 // 停止重试
	}

	defer func() {
		eb.currentInterval = time.Duration(math.Min(
			float64(eb.currentInterval)*eb.Multiplier,
			float64(eb.MaxInterval),
		))
	}()

	return eb.currentInterval + time.Duration(rand.Int63n(int64(eb.currentInterval)))
}

// Reset 重置退避计算器
func (eb *ExponentialBackoff) Reset() {
	eb.currentInterval = eb.InitialInterval
	eb.startTime = time.Now()
}
