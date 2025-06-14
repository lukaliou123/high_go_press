package metrics

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// MetricsManager 指标管理器
type MetricsManager struct {
	registry *prometheus.Registry
	logger   *zap.Logger

	// HTTP 指标
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight *prometheus.GaugeVec

	// gRPC 指标
	grpcRequestsTotal    *prometheus.CounterVec
	grpcRequestDuration  *prometheus.HistogramVec
	grpcRequestsInFlight *prometheus.GaugeVec

	// 系统指标
	systemCPUUsage    prometheus.Gauge
	systemMemoryUsage prometheus.Gauge
	systemGoroutines  prometheus.Gauge
	systemGCDuration  prometheus.Gauge

	// 业务指标
	businessCounters   *prometheus.CounterVec
	businessGauges     *prometheus.GaugeVec
	businessHistograms *prometheus.HistogramVec

	// 数据库指标
	dbConnectionsActive *prometheus.GaugeVec
	dbConnectionsIdle   *prometheus.GaugeVec
	dbQueryDuration     *prometheus.HistogramVec
	dbQueryTotal        *prometheus.CounterVec

	// 缓存指标
	cacheHits              *prometheus.CounterVec
	cacheMisses            *prometheus.CounterVec
	cacheOperationDuration *prometheus.HistogramVec

	// 服务健康指标
	serviceHealth *prometheus.GaugeVec
	serviceUptime prometheus.Gauge

	mu sync.RWMutex
}

// Config 指标配置
type Config struct {
	Namespace      string            `yaml:"namespace"`
	Subsystem      string            `yaml:"subsystem"`
	Labels         map[string]string `yaml:"labels"`
	EnableSystem   bool              `yaml:"enable_system"`
	EnableBusiness bool              `yaml:"enable_business"`
	EnableDB       bool              `yaml:"enable_db"`
	EnableCache    bool              `yaml:"enable_cache"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Namespace:      "highgopress",
		Subsystem:      "",
		Labels:         make(map[string]string),
		EnableSystem:   true,
		EnableBusiness: true,
		EnableDB:       true,
		EnableCache:    true,
	}
}

// NewMetricsManager 创建指标管理器
func NewMetricsManager(config *Config, logger *zap.Logger) *MetricsManager {
	if config == nil {
		config = DefaultConfig()
	}

	registry := prometheus.NewRegistry()

	mm := &MetricsManager{
		registry: registry,
		logger:   logger,
	}

	mm.initHTTPMetrics(config)
	mm.initGRPCMetrics(config)

	if config.EnableSystem {
		mm.initSystemMetrics(config)
	}

	if config.EnableBusiness {
		mm.initBusinessMetrics(config)
	}

	if config.EnableDB {
		mm.initDBMetrics(config)
	}

	if config.EnableCache {
		mm.initCacheMetrics(config)
	}

	mm.initServiceMetrics(config)

	// 注册所有指标到 registry
	mm.registerMetrics()

	// 启动系统指标收集
	if config.EnableSystem {
		go mm.collectSystemMetrics()
	}

	logger.Info("Metrics manager initialized",
		zap.String("namespace", config.Namespace),
		zap.String("subsystem", config.Subsystem))

	return mm
}

// initHTTPMetrics 初始化 HTTP 指标
func (mm *MetricsManager) initHTTPMetrics(config *Config) {
	mm.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code", "service"},
	)

	mm.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint", "service"},
	)

	mm.httpRequestsInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
		[]string{"service"},
	)
}

// initGRPCMetrics 初始化 gRPC 指标
func (mm *MetricsManager) initGRPCMetrics(config *Config) {
	mm.grpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "grpc_requests_total",
			Help:      "Total number of gRPC requests",
		},
		[]string{"method", "service", "status"},
	)

	mm.grpcRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "grpc_request_duration_seconds",
			Help:      "gRPC request duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "service"},
	)

	mm.grpcRequestsInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "grpc_requests_in_flight",
			Help:      "Number of gRPC requests currently being processed",
		},
		[]string{"service"},
	)
}

// initSystemMetrics 初始化系统指标
func (mm *MetricsManager) initSystemMetrics(config *Config) {
	mm.systemCPUUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "system_cpu_usage_percent",
			Help:      "Current CPU usage percentage",
		},
	)

	mm.systemMemoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "system_memory_usage_bytes",
			Help:      "Current memory usage in bytes",
		},
	)

	mm.systemGoroutines = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "system_goroutines_total",
			Help:      "Current number of goroutines",
		},
	)

	mm.systemGCDuration = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "system_gc_duration_seconds",
			Help:      "Time spent in garbage collection",
		},
	)
}

// initBusinessMetrics 初始化业务指标
func (mm *MetricsManager) initBusinessMetrics(config *Config) {
	mm.businessCounters = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "business_operations_total",
			Help:      "Total number of business operations",
		},
		[]string{"operation", "service", "status"},
	)

	mm.businessGauges = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "business_current_value",
			Help:      "Current business metric value",
		},
		[]string{"metric", "service"},
	)

	mm.businessHistograms = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "business_operation_duration_seconds",
			Help:      "Business operation duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "service"},
	)
}

// initDBMetrics 初始化数据库指标
func (mm *MetricsManager) initDBMetrics(config *Config) {
	mm.dbConnectionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "db_connections_active",
			Help:      "Number of active database connections",
		},
		[]string{"database", "service"},
	)

	mm.dbConnectionsIdle = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "db_connections_idle",
			Help:      "Number of idle database connections",
		},
		[]string{"database", "service"},
	)

	mm.dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "db_query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "database", "service"},
	)

	mm.dbQueryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "db_queries_total",
			Help:      "Total number of database queries",
		},
		[]string{"operation", "database", "service", "status"},
	)
}

// initCacheMetrics 初始化缓存指标
func (mm *MetricsManager) initCacheMetrics(config *Config) {
	mm.cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		},
		[]string{"cache", "service"},
	)

	mm.cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		},
		[]string{"cache", "service"},
	)

	mm.cacheOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "cache_operation_duration_seconds",
			Help:      "Cache operation duration in seconds",
			Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"operation", "cache", "service"},
	)
}

// initServiceMetrics 初始化服务指标
func (mm *MetricsManager) initServiceMetrics(config *Config) {
	mm.serviceHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "service_health",
			Help:      "Service health status (1 = healthy, 0 = unhealthy)",
		},
		[]string{"service", "component"},
	)

	mm.serviceUptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "service_uptime_seconds",
			Help:      "Service uptime in seconds",
		},
	)
}

// registerMetrics 注册所有指标
func (mm *MetricsManager) registerMetrics() {
	// HTTP 指标
	mm.registry.MustRegister(mm.httpRequestsTotal)
	mm.registry.MustRegister(mm.httpRequestDuration)
	mm.registry.MustRegister(mm.httpRequestsInFlight)

	// gRPC 指标
	mm.registry.MustRegister(mm.grpcRequestsTotal)
	mm.registry.MustRegister(mm.grpcRequestDuration)
	mm.registry.MustRegister(mm.grpcRequestsInFlight)

	// 系统指标
	if mm.systemCPUUsage != nil {
		mm.registry.MustRegister(mm.systemCPUUsage)
		mm.registry.MustRegister(mm.systemMemoryUsage)
		mm.registry.MustRegister(mm.systemGoroutines)
		mm.registry.MustRegister(mm.systemGCDuration)
	}

	// 业务指标
	if mm.businessCounters != nil {
		mm.registry.MustRegister(mm.businessCounters)
		mm.registry.MustRegister(mm.businessGauges)
		mm.registry.MustRegister(mm.businessHistograms)
	}

	// 数据库指标
	if mm.dbConnectionsActive != nil {
		mm.registry.MustRegister(mm.dbConnectionsActive)
		mm.registry.MustRegister(mm.dbConnectionsIdle)
		mm.registry.MustRegister(mm.dbQueryDuration)
		mm.registry.MustRegister(mm.dbQueryTotal)
	}

	// 缓存指标
	if mm.cacheHits != nil {
		mm.registry.MustRegister(mm.cacheHits)
		mm.registry.MustRegister(mm.cacheMisses)
		mm.registry.MustRegister(mm.cacheOperationDuration)
	}

	// 服务指标
	mm.registry.MustRegister(mm.serviceHealth)
	mm.registry.MustRegister(mm.serviceUptime)
}

// collectSystemMetrics 收集系统指标
func (mm *MetricsManager) collectSystemMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for range ticker.C {
		// 收集 Goroutine 数量
		mm.systemGoroutines.Set(float64(runtime.NumGoroutine()))

		// 收集内存使用情况
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		mm.systemMemoryUsage.Set(float64(memStats.Alloc))

		// 收集 GC 时间
		mm.systemGCDuration.Set(float64(memStats.PauseTotalNs) / 1e9)

		// 更新服务运行时间
		mm.serviceUptime.Set(time.Since(startTime).Seconds())
	}
}

// GetRegistry 获取 Prometheus 注册器
func (mm *MetricsManager) GetRegistry() *prometheus.Registry {
	return mm.registry
}

// GetHandler 获取 HTTP 处理器
func (mm *MetricsManager) GetHandler() http.Handler {
	return promhttp.HandlerFor(mm.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordHTTPRequest 记录 HTTP 请求指标
func (mm *MetricsManager) RecordHTTPRequest(method, endpoint, statusCode, service string, duration time.Duration) {
	mm.httpRequestsTotal.WithLabelValues(method, endpoint, statusCode, service).Inc()
	mm.httpRequestDuration.WithLabelValues(method, endpoint, service).Observe(duration.Seconds())
}

// IncHTTPInFlight 增加正在处理的 HTTP 请求数
func (mm *MetricsManager) IncHTTPInFlight(service string) {
	mm.httpRequestsInFlight.WithLabelValues(service).Inc()
}

// DecHTTPInFlight 减少正在处理的 HTTP 请求数
func (mm *MetricsManager) DecHTTPInFlight(service string) {
	mm.httpRequestsInFlight.WithLabelValues(service).Dec()
}

// RecordGRPCRequest 记录 gRPC 请求指标
func (mm *MetricsManager) RecordGRPCRequest(method, service, status string, duration time.Duration) {
	mm.grpcRequestsTotal.WithLabelValues(method, service, status).Inc()
	mm.grpcRequestDuration.WithLabelValues(method, service).Observe(duration.Seconds())
}

// IncGRPCInFlight 增加正在处理的 gRPC 请求数
func (mm *MetricsManager) IncGRPCInFlight(service string) {
	mm.grpcRequestsInFlight.WithLabelValues(service).Inc()
}

// DecGRPCInFlight 减少正在处理的 gRPC 请求数
func (mm *MetricsManager) DecGRPCInFlight(service string) {
	mm.grpcRequestsInFlight.WithLabelValues(service).Dec()
}

// RecordBusinessOperation 记录业务操作指标
func (mm *MetricsManager) RecordBusinessOperation(operation, service, status string, duration time.Duration) {
	if mm.businessCounters != nil {
		mm.businessCounters.WithLabelValues(operation, service, status).Inc()
		mm.businessHistograms.WithLabelValues(operation, service).Observe(duration.Seconds())
	}
}

// SetBusinessGauge 设置业务指标值
func (mm *MetricsManager) SetBusinessGauge(metric, service string, value float64) {
	if mm.businessGauges != nil {
		mm.businessGauges.WithLabelValues(metric, service).Set(value)
	}
}

// RecordDBOperation 记录数据库操作指标
func (mm *MetricsManager) RecordDBOperation(operation, database, service, status string, duration time.Duration) {
	if mm.dbQueryTotal != nil {
		mm.dbQueryTotal.WithLabelValues(operation, database, service, status).Inc()
		mm.dbQueryDuration.WithLabelValues(operation, database, service).Observe(duration.Seconds())
	}
}

// SetDBConnections 设置数据库连接数
func (mm *MetricsManager) SetDBConnections(database, service string, active, idle int) {
	if mm.dbConnectionsActive != nil {
		mm.dbConnectionsActive.WithLabelValues(database, service).Set(float64(active))
		mm.dbConnectionsIdle.WithLabelValues(database, service).Set(float64(idle))
	}
}

// RecordCacheOperation 记录缓存操作指标
func (mm *MetricsManager) RecordCacheOperation(operation, cache, service string, hit bool, duration time.Duration) {
	if mm.cacheHits != nil {
		if hit {
			mm.cacheHits.WithLabelValues(cache, service).Inc()
		} else {
			mm.cacheMisses.WithLabelValues(cache, service).Inc()
		}
		mm.cacheOperationDuration.WithLabelValues(operation, cache, service).Observe(duration.Seconds())
	}
}

// SetServiceHealth 设置服务健康状态
func (mm *MetricsManager) SetServiceHealth(service, component string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	mm.serviceHealth.WithLabelValues(service, component).Set(value)
}

// Shutdown 关闭指标管理器
func (mm *MetricsManager) Shutdown(ctx context.Context) error {
	mm.logger.Info("Shutting down metrics manager")
	return nil
}
