package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config 统一配置结构
type Config struct {
	Environment string           `mapstructure:"environment" validate:"required,oneof=dev test prod"`
	Gateway     GatewayConfig    `mapstructure:"gateway"`
	Counter     CounterConfig    `mapstructure:"counter"`
	Analytics   AnalyticsConfig  `mapstructure:"analytics"`
	Discovery   DiscoveryConfig  `mapstructure:"discovery"`
	Redis       RedisConfig      `mapstructure:"redis"`
	Kafka       KafkaConfig      `mapstructure:"kafka"`
	Log         LogConfig        `mapstructure:"log"`
	Monitoring  MonitoringConfig `mapstructure:"monitoring"`
}

// GatewayConfig Gateway服务配置
type GatewayConfig struct {
	Server   ServerConfig   `mapstructure:"server"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Timeout  TimeoutConfig  `mapstructure:"timeout"`
	Security SecurityConfig `mapstructure:"security"`
}

// CounterConfig Counter服务配置
type CounterConfig struct {
	Server      ServerConfig      `mapstructure:"server"`
	GRPC        GRPCConfig        `mapstructure:"grpc"`
	Performance PerformanceConfig `mapstructure:"performance"`
}

// AnalyticsConfig Analytics服务配置
type AnalyticsConfig struct {
	Server ServerConfig `mapstructure:"server"`
	GRPC   GRPCConfig   `mapstructure:"grpc"`
	Cache  CacheConfig  `mapstructure:"cache"`
}

// DiscoveryConfig 服务发现配置
type DiscoveryConfig struct {
	Type   string       `mapstructure:"type" validate:"required,oneof=consul static"`
	Consul ConsulConfig `mapstructure:"consul"`
}

// ConsulConfig Consul配置
type ConsulConfig struct {
	Address string        `mapstructure:"address" validate:"required"`
	Scheme  string        `mapstructure:"scheme" validate:"oneof=http https"`
	Token   string        `mapstructure:"token"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `mapstructure:"host" validate:"required"`
	Port int    `mapstructure:"port" validate:"min=1,max=65535"`
	Mode string `mapstructure:"mode" validate:"oneof=debug release test"`
}

// GRPCConfig gRPC配置
type GRPCConfig struct {
	MaxRecvMsgSize int                  `mapstructure:"max_recv_msg_size"`
	MaxSendMsgSize int                  `mapstructure:"max_send_msg_size"`
	MaxConnections int                  `mapstructure:"max_connections"`
	KeepAlive      KeepAliveConfig      `mapstructure:"keep_alive"`
	ConnectionPool ConnectionPoolConfig `mapstructure:"connection_pool"`
}

// KeepAliveConfig Keep-Alive配置
type KeepAliveConfig struct {
	Time    time.Duration `mapstructure:"time"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// ConnectionPoolConfig 连接池配置
type ConnectionPoolConfig struct {
	Size        int           `mapstructure:"size"`
	MaxIdleTime time.Duration `mapstructure:"max_idle_time"`
	HealthCheck bool          `mapstructure:"health_check"`
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	Read  time.Duration `mapstructure:"read"`
	Write time.Duration `mapstructure:"write"`
	Idle  time.Duration `mapstructure:"idle"`
	GRPC  time.Duration `mapstructure:"grpc"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	CORS      CORSConfig      `mapstructure:"cors"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled bool `mapstructure:"enabled"`
	RPS     int  `mapstructure:"rps"`
	Burst   int  `mapstructure:"burst"`
}

// CORSConfig CORS配置
type CORSConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Origins []string `mapstructure:"origins"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	WorkerPoolSize    int  `mapstructure:"worker_pool_size"`
	ObjectPoolEnabled bool `mapstructure:"object_pool_enabled"`
	BatchSize         int  `mapstructure:"batch_size"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	TTL             time.Duration `mapstructure:"ttl"`
	MaxSize         int           `mapstructure:"max_size"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Address      string        `mapstructure:"address" validate:"required"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Mode     string         `mapstructure:"mode" validate:"oneof=real mock"`
	Brokers  []string       `mapstructure:"brokers"`
	Topic    string         `mapstructure:"topic"`
	Producer ProducerConfig `mapstructure:"producer"`
	Consumer ConsumerConfig `mapstructure:"consumer"`
}

// ProducerConfig Kafka生产者配置
type ProducerConfig struct {
	BatchSize    int `mapstructure:"batch_size"`
	LingerMs     int `mapstructure:"linger_ms"`
	BufferMemory int `mapstructure:"buffer_memory"`
}

// ConsumerConfig Kafka消费者配置
type ConsumerConfig struct {
	GroupID         string `mapstructure:"group_id"`
	AutoOffsetReset string `mapstructure:"auto_offset_reset"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string     `mapstructure:"level" validate:"oneof=debug info warn error"`
	Format string     `mapstructure:"format" validate:"oneof=json console"`
	Output string     `mapstructure:"output" validate:"oneof=stdout file"`
	File   FileConfig `mapstructure:"file"`
}

// FileConfig 文件日志配置
type FileConfig struct {
	Path       string `mapstructure:"path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Pprof       PprofConfig       `mapstructure:"pprof"`
	Prometheus  PrometheusConfig  `mapstructure:"prometheus"`
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
}

// PprofConfig Pprof配置
type PprofConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

// PrometheusConfig Prometheus配置
type PrometheusConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Port int    `mapstructure:"port"`
	Path string `mapstructure:"path"`
}

// Manager 配置管理器
type Manager struct {
	config       *Config
	logger       *zap.Logger
	configCenter ConfigCenter
	watchers     []ConfigChangeCallback
	mutex        sync.RWMutex
}

// NewManager 创建配置管理器
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:   logger,
		watchers: make([]ConfigChangeCallback, 0),
	}
}

// NewManagerWithCenter 创建带配置中心的配置管理器
func NewManagerWithCenter(logger *zap.Logger, configCenter ConfigCenter) *Manager {
	return &Manager{
		logger:       logger,
		configCenter: configCenter,
		watchers:     make([]ConfigChangeCallback, 0),
	}
}

// SetConfigCenter 设置配置中心
func (m *Manager) SetConfigCenter(configCenter ConfigCenter) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.configCenter = configCenter
}

// Load 加载配置
func (m *Manager) Load(configPath string) (*Config, error) {
	return m.LoadWithServiceInfo(configPath, "", "")
}

// LoadWithServiceInfo 加载配置（带服务信息）
func (m *Manager) LoadWithServiceInfo(configPath, serviceName, environment string) (*Config, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var config *Config
	var err error

	// 优先从配置中心加载
	if m.configCenter != nil && serviceName != "" && environment != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		config, err = m.configCenter.GetConfig(ctx, serviceName, environment)
		if err != nil {
			m.logger.Warn("Failed to load config from config center, fallback to file",
				zap.String("service", serviceName),
				zap.String("environment", environment),
				zap.Error(err))
		} else {
			m.logger.Info("Config loaded from config center",
				zap.String("service", serviceName),
				zap.String("environment", environment))
			m.config = config
			return config, nil
		}
	}

	// 从文件加载配置
	config, err = m.loadFromFile(configPath)
	if err != nil {
		return nil, err
	}

	m.config = config

	// 如果配置中心可用且服务信息完整，推送配置到配置中心
	if m.configCenter != nil && serviceName != "" && environment != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := m.configCenter.PutConfig(ctx, serviceName, environment, config); err != nil {
			m.logger.Warn("Failed to push config to config center",
				zap.String("service", serviceName),
				zap.String("environment", environment),
				zap.Error(err))
		} else {
			m.logger.Info("Config pushed to config center",
				zap.String("service", serviceName),
				zap.String("environment", environment))
		}
	}

	return config, nil
}

// loadFromFile 从文件加载配置
func (m *Manager) loadFromFile(configPath string) (*Config, error) {
	// 设置环境变量前缀
	viper.SetEnvPrefix("HIGH_GO_PRESS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 设置配置文件
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// 根据环境变量决定配置文件
		env := os.Getenv("HIGH_GO_PRESS_ENVIRONMENT")
		if env == "" {
			env = "dev"
		}

		viper.SetConfigName(env)
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath("../configs")
		viper.AddConfigPath("../../configs")
	}

	// 设置默认值
	m.setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			m.logger.Warn("Config file not found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// 解析配置
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 验证配置
	if err := m.validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	m.logger.Info("Configuration loaded successfully",
		zap.String("environment", config.Environment),
		zap.String("config_file", viper.ConfigFileUsed()))

	return &config, nil
}

// setDefaults 设置默认值
func (m *Manager) setDefaults() {
	// 环境设置
	viper.SetDefault("environment", "dev")

	// Gateway默认值
	viper.SetDefault("gateway.server.host", "0.0.0.0")
	viper.SetDefault("gateway.server.port", 8080)
	viper.SetDefault("gateway.server.mode", "debug")
	viper.SetDefault("gateway.timeout.read", "30s")
	viper.SetDefault("gateway.timeout.write", "30s")
	viper.SetDefault("gateway.timeout.idle", "120s")
	viper.SetDefault("gateway.timeout.grpc", "5s")
	viper.SetDefault("gateway.security.rate_limit.enabled", false)
	viper.SetDefault("gateway.security.cors.enabled", true)

	// Counter服务默认值
	viper.SetDefault("counter.server.host", "0.0.0.0")
	viper.SetDefault("counter.server.port", 9001)
	viper.SetDefault("counter.server.mode", "debug")
	viper.SetDefault("counter.grpc.max_recv_msg_size", 4194304) // 4MB
	viper.SetDefault("counter.grpc.max_send_msg_size", 4194304) // 4MB
	viper.SetDefault("counter.grpc.max_connections", 1000)
	viper.SetDefault("counter.grpc.keep_alive.time", "60s")
	viper.SetDefault("counter.grpc.keep_alive.timeout", "10s")
	viper.SetDefault("counter.grpc.connection_pool.size", 20)
	viper.SetDefault("counter.grpc.connection_pool.max_idle_time", "300s")
	viper.SetDefault("counter.performance.worker_pool_size", 1000)
	viper.SetDefault("counter.performance.object_pool_enabled", true)
	viper.SetDefault("counter.performance.batch_size", 100)

	// Analytics服务默认值
	viper.SetDefault("analytics.server.host", "0.0.0.0")
	viper.SetDefault("analytics.server.port", 9002)
	viper.SetDefault("analytics.server.mode", "debug")
	viper.SetDefault("analytics.grpc.max_recv_msg_size", 4194304) // 4MB
	viper.SetDefault("analytics.grpc.max_send_msg_size", 4194304) // 4MB
	viper.SetDefault("analytics.cache.ttl", "300s")
	viper.SetDefault("analytics.cache.max_size", 10000)

	// 服务发现默认值
	viper.SetDefault("discovery.type", "consul")
	viper.SetDefault("discovery.consul.address", "localhost:8500")
	viper.SetDefault("discovery.consul.scheme", "http")
	viper.SetDefault("discovery.consul.timeout", "10s")

	// Redis默认值
	viper.SetDefault("redis.address", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 20)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")

	// Kafka默认值
	viper.SetDefault("kafka.mode", "mock")
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topic", "counter-events")
	viper.SetDefault("kafka.producer.batch_size", 16384)
	viper.SetDefault("kafka.producer.linger_ms", 10)
	viper.SetDefault("kafka.producer.buffer_memory", 33554432)
	viper.SetDefault("kafka.consumer.group_id", "high_go_press_analytics")
	viper.SetDefault("kafka.consumer.auto_offset_reset", "earliest")

	// 日志默认值
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "console")
	viper.SetDefault("log.output", "stdout")

	// 监控默认值
	viper.SetDefault("monitoring.pprof.enabled", true)
	viper.SetDefault("monitoring.pprof.port", 6060)
	viper.SetDefault("monitoring.prometheus.enabled", false)
	viper.SetDefault("monitoring.prometheus.port", 2112)
	viper.SetDefault("monitoring.prometheus.path", "/metrics")
	viper.SetDefault("monitoring.health_check.port", 8090)
	viper.SetDefault("monitoring.health_check.path", "/health")
}

// validate 验证配置
func (m *Manager) validate(config *Config) error {
	// 基础验证
	if config.Environment == "" {
		return fmt.Errorf("environment is required")
	}

	// 端口冲突检查
	ports := make(map[int]string)

	if err := m.checkPortConflict(ports, config.Gateway.Server.Port, "gateway"); err != nil {
		return err
	}
	if err := m.checkPortConflict(ports, config.Counter.Server.Port, "counter"); err != nil {
		return err
	}
	if err := m.checkPortConflict(ports, config.Analytics.Server.Port, "analytics"); err != nil {
		return err
	}

	// Redis连接验证
	if config.Redis.Address == "" {
		return fmt.Errorf("redis address is required")
	}

	// Kafka配置验证
	if config.Kafka.Mode == "real" && len(config.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka brokers are required when mode is 'real'")
	}

	return nil
}

// checkPortConflict 检查端口冲突
func (m *Manager) checkPortConflict(ports map[int]string, port int, service string) error {
	if existing, exists := ports[port]; exists {
		return fmt.Errorf("port conflict: %d is used by both %s and %s", port, existing, service)
	}
	ports[port] = service
	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	return m.config
}

// Reload 重新加载配置
func (m *Manager) Reload() error {
	if m.config == nil {
		return fmt.Errorf("no config loaded")
	}

	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no config file to reload")
	}

	_, err := m.Load(configFile)
	return err
}

// 便捷函数
func Load(configPath string) (*Config, error) {
	manager := NewManager(zap.L())
	return manager.Load(configPath)
}

// 保持向后兼容的简单函数
func LoadSimple(configPath string) (*Config, error) {
	return Load(configPath)
}

// StartWatchConfig 开始监听配置变化
func (m *Manager) StartWatchConfig(ctx context.Context, serviceName, environment string) error {
	if m.configCenter == nil {
		return fmt.Errorf("config center not set")
	}

	callback := func(oldConfig, newConfig *Config) error {
		m.mutex.Lock()
		defer m.mutex.Unlock()

		// 验证新配置
		if newConfig != nil {
			if err := m.validate(newConfig); err != nil {
				m.logger.Error("New config validation failed", zap.Error(err))
				return err
			}
		}

		// 更新内部配置
		m.config = newConfig

		// 通知所有监听器
		for _, watcher := range m.watchers {
			if err := watcher(oldConfig, newConfig); err != nil {
				m.logger.Error("Config change watcher failed", zap.Error(err))
			}
		}

		m.logger.Info("Configuration updated from config center",
			zap.String("service", serviceName),
			zap.String("environment", environment))

		return nil
	}

	return m.configCenter.WatchConfig(ctx, serviceName, environment, callback)
}

// StopWatchConfig 停止监听配置变化
func (m *Manager) StopWatchConfig(serviceName, environment string) {
	if m.configCenter != nil {
		m.configCenter.StopWatch(serviceName, environment)
	}
}

// AddConfigWatcher 添加配置变化监听器
func (m *Manager) AddConfigWatcher(callback ConfigChangeCallback) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.watchers = append(m.watchers, callback)
}

// PushConfig 推送配置到配置中心
func (m *Manager) PushConfig(ctx context.Context, serviceName, environment string, config *Config) error {
	if m.configCenter == nil {
		return fmt.Errorf("config center not set")
	}

	// 验证配置
	if err := m.validate(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	return m.configCenter.PutConfig(ctx, serviceName, environment, config)
}

// GetConfigFromCenter 从配置中心获取配置
func (m *Manager) GetConfigFromCenter(ctx context.Context, serviceName, environment string) (*Config, error) {
	if m.configCenter == nil {
		return nil, fmt.Errorf("config center not set")
	}

	return m.configCenter.GetConfig(ctx, serviceName, environment)
}

// GetConfigHistory 获取配置历史版本
func (m *Manager) GetConfigHistory(ctx context.Context, serviceName, environment string) ([]*ConfigVersion, error) {
	if m.configCenter == nil {
		return nil, fmt.Errorf("config center not set")
	}

	return m.configCenter.GetConfigHistory(ctx, serviceName, environment)
}
