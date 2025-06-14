package grpc

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FallbackStrategy 降级策略类型
type FallbackStrategy int

const (
	// FallbackToCache 降级到缓存
	FallbackToCache FallbackStrategy = iota
	// FallbackToDefault 降级到默认值
	FallbackToDefault
	// FallbackToStatic 降级到静态响应
	FallbackToStatic
	// FallbackToAlternative 降级到备用服务
	FallbackToAlternative
)

// FallbackConfig 降级配置
type FallbackConfig struct {
	// 启用降级
	Enabled bool
	// 降级策略
	Strategy FallbackStrategy
	// 降级触发条件
	TriggerConditions []FallbackCondition
	// 缓存TTL
	CacheTTL time.Duration
	// 默认响应
	DefaultResponse interface{}
	// 备用服务地址
	AlternativeService string
	// 降级超时时间
	FallbackTimeout time.Duration
}

// FallbackCondition 降级触发条件
type FallbackCondition struct {
	// 条件类型
	Type FallbackConditionType
	// 阈值
	Threshold interface{}
	// 时间窗口
	TimeWindow time.Duration
}

// FallbackConditionType 降级条件类型
type FallbackConditionType int

const (
	// ConditionErrorRate 错误率条件
	ConditionErrorRate FallbackConditionType = iota
	// ConditionLatency 延迟条件
	ConditionLatency
	// ConditionCircuitOpen 熔断器开启条件
	ConditionCircuitOpen
	// ConditionResourceUsage 资源使用率条件
	ConditionResourceUsage
)

// FallbackHandler 降级处理器接口
type FallbackHandler interface {
	// Handle 处理降级请求
	Handle(ctx context.Context, req interface{}) (interface{}, error)
	// CanHandle 检查是否可以处理该请求
	CanHandle(req interface{}) bool
}

// CacheFallbackHandler 缓存降级处理器
type CacheFallbackHandler struct {
	cache  map[string]CacheEntry
	mutex  sync.RWMutex
	ttl    time.Duration
	logger *zap.Logger
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Data      interface{}
	Timestamp time.Time
	TTL       time.Duration
}

// NewCacheFallbackHandler 创建缓存降级处理器
func NewCacheFallbackHandler(ttl time.Duration, logger *zap.Logger) *CacheFallbackHandler {
	return &CacheFallbackHandler{
		cache:  make(map[string]CacheEntry),
		ttl:    ttl,
		logger: logger,
	}
}

// Handle 处理缓存降级
func (h *CacheFallbackHandler) Handle(ctx context.Context, req interface{}) (interface{}, error) {
	key := h.generateCacheKey(req)

	h.mutex.RLock()
	entry, exists := h.cache[key]
	h.mutex.RUnlock()

	if exists && !h.isExpired(entry) {
		h.logger.Info("Fallback to cache hit", zap.String("key", key))
		return entry.Data, nil
	}

	h.logger.Warn("Fallback to cache miss", zap.String("key", key))
	return nil, ErrFallbackCacheMiss
}

// CanHandle 检查是否可以处理
func (h *CacheFallbackHandler) CanHandle(req interface{}) bool {
	key := h.generateCacheKey(req)
	h.mutex.RLock()
	entry, exists := h.cache[key]
	h.mutex.RUnlock()

	return exists && !h.isExpired(entry)
}

// Set 设置缓存
func (h *CacheFallbackHandler) Set(key string, data interface{}) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.cache[key] = CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
		TTL:       h.ttl,
	}
}

// generateCacheKey 生成缓存键
func (h *CacheFallbackHandler) generateCacheKey(req interface{}) string {
	// 简单实现，实际应该根据请求内容生成唯一键
	return "fallback_cache_key"
}

// isExpired 检查是否过期
func (h *CacheFallbackHandler) isExpired(entry CacheEntry) bool {
	return time.Since(entry.Timestamp) > entry.TTL
}

// DefaultFallbackHandler 默认值降级处理器
type DefaultFallbackHandler struct {
	defaultResponse interface{}
	logger          *zap.Logger
}

// NewDefaultFallbackHandler 创建默认值降级处理器
func NewDefaultFallbackHandler(defaultResponse interface{}, logger *zap.Logger) *DefaultFallbackHandler {
	return &DefaultFallbackHandler{
		defaultResponse: defaultResponse,
		logger:          logger,
	}
}

// Handle 处理默认值降级
func (h *DefaultFallbackHandler) Handle(ctx context.Context, req interface{}) (interface{}, error) {
	h.logger.Info("Fallback to default response")
	return h.defaultResponse, nil
}

// CanHandle 检查是否可以处理
func (h *DefaultFallbackHandler) CanHandle(req interface{}) bool {
	return h.defaultResponse != nil
}

// FallbackManager 降级管理器
type FallbackManager struct {
	config   *FallbackConfig
	handlers map[FallbackStrategy]FallbackHandler
	stats    FallbackStats
	logger   *zap.Logger
	mutex    sync.RWMutex
}

// FallbackStats 降级统计信息
type FallbackStats struct {
	TotalFallbacks       int64
	CacheFallbacks       int64
	DefaultFallbacks     int64
	StaticFallbacks      int64
	AlternativeFallbacks int64
	FailedFallbacks      int64
	LastFallbackTime     time.Time
}

// NewFallbackManager 创建降级管理器
func NewFallbackManager(config *FallbackConfig, logger *zap.Logger) *FallbackManager {
	fm := &FallbackManager{
		config:   config,
		handlers: make(map[FallbackStrategy]FallbackHandler),
		logger:   logger,
	}

	// 初始化处理器
	fm.initHandlers()

	return fm
}

// initHandlers 初始化处理器
func (fm *FallbackManager) initHandlers() {
	// 缓存降级处理器
	fm.handlers[FallbackToCache] = NewCacheFallbackHandler(fm.config.CacheTTL, fm.logger)

	// 默认值降级处理器
	if fm.config.DefaultResponse != nil {
		fm.handlers[FallbackToDefault] = NewDefaultFallbackHandler(fm.config.DefaultResponse, fm.logger)
	}
}

// Execute 执行降级逻辑
func (fm *FallbackManager) Execute(ctx context.Context, req interface{}, primaryFn func(context.Context, interface{}) (interface{}, error)) (interface{}, error) {
	if !fm.config.Enabled {
		return primaryFn(ctx, req)
	}

	// 尝试执行主要逻辑
	result, err := primaryFn(ctx, req)
	if err == nil {
		// 成功时缓存结果
		fm.cacheResult(req, result)
		return result, nil
	}

	// 检查是否需要降级
	if !fm.shouldFallback(err) {
		return result, err
	}

	// 执行降级
	return fm.performFallback(ctx, req, err)
}

// shouldFallback 检查是否应该降级
func (fm *FallbackManager) shouldFallback(err error) bool {
	// 检查降级触发条件
	for _, condition := range fm.config.TriggerConditions {
		if fm.checkCondition(condition, err) {
			return true
		}
	}

	// 默认降级条件：任何错误都降级
	return true
}

// checkCondition 检查降级条件
func (fm *FallbackManager) checkCondition(condition FallbackCondition, err error) bool {
	switch condition.Type {
	case ConditionErrorRate:
		// 检查错误率
		return true // 简化实现
	case ConditionLatency:
		// 检查延迟
		return true // 简化实现
	case ConditionCircuitOpen:
		// 检查熔断器状态
		return true // 简化实现
	case ConditionResourceUsage:
		// 检查资源使用率
		return true // 简化实现
	default:
		return false
	}
}

// performFallback 执行降级
func (fm *FallbackManager) performFallback(ctx context.Context, req interface{}, originalErr error) (interface{}, error) {
	fm.mutex.Lock()
	fm.stats.TotalFallbacks++
	fm.stats.LastFallbackTime = time.Now()
	fm.mutex.Unlock()

	handler, exists := fm.handlers[fm.config.Strategy]
	if !exists {
		fm.mutex.Lock()
		fm.stats.FailedFallbacks++
		fm.mutex.Unlock()
		return nil, ErrFallbackHandlerNotFound
	}

	if !handler.CanHandle(req) {
		fm.mutex.Lock()
		fm.stats.FailedFallbacks++
		fm.mutex.Unlock()
		return nil, ErrFallbackCannotHandle
	}

	result, err := handler.Handle(ctx, req)
	if err != nil {
		fm.mutex.Lock()
		fm.stats.FailedFallbacks++
		fm.mutex.Unlock()
		fm.logger.Error("Fallback handler failed", zap.Error(err))
		return nil, err
	}

	// 更新统计信息
	fm.mutex.Lock()
	switch fm.config.Strategy {
	case FallbackToCache:
		fm.stats.CacheFallbacks++
	case FallbackToDefault:
		fm.stats.DefaultFallbacks++
	case FallbackToStatic:
		fm.stats.StaticFallbacks++
	case FallbackToAlternative:
		fm.stats.AlternativeFallbacks++
	}
	fm.mutex.Unlock()

	fm.logger.Info("Fallback executed successfully",
		zap.String("strategy", fm.getStrategyName(fm.config.Strategy)),
		zap.Error(originalErr))

	return result, nil
}

// cacheResult 缓存结果
func (fm *FallbackManager) cacheResult(req interface{}, result interface{}) {
	if cacheHandler, ok := fm.handlers[FallbackToCache].(*CacheFallbackHandler); ok {
		key := cacheHandler.generateCacheKey(req)
		cacheHandler.Set(key, result)
	}
}

// getStrategyName 获取策略名称
func (fm *FallbackManager) getStrategyName(strategy FallbackStrategy) string {
	switch strategy {
	case FallbackToCache:
		return "cache"
	case FallbackToDefault:
		return "default"
	case FallbackToStatic:
		return "static"
	case FallbackToAlternative:
		return "alternative"
	default:
		return "unknown"
	}
}

// GetStats 获取统计信息
func (fm *FallbackManager) GetStats() FallbackStats {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
	return fm.stats
}

// Reset 重置统计信息
func (fm *FallbackManager) Reset() {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	fm.stats = FallbackStats{}
}

// 错误定义
var (
	ErrFallbackCacheMiss       = errors.New("fallback cache miss")
	ErrFallbackHandlerNotFound = errors.New("fallback handler not found")
	ErrFallbackCannotHandle    = errors.New("fallback handler cannot handle request")
)

// DefaultFallbackConfig 默认降级配置
func DefaultFallbackConfig() *FallbackConfig {
	return &FallbackConfig{
		Enabled:         true,
		Strategy:        FallbackToCache,
		CacheTTL:        5 * time.Minute,
		FallbackTimeout: 1 * time.Second,
		TriggerConditions: []FallbackCondition{
			{
				Type:       ConditionErrorRate,
				Threshold:  0.5, // 50%错误率
				TimeWindow: 1 * time.Minute,
			},
		},
	}
}
