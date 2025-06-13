package config

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// ConfigCenter 配置中心接口
type ConfigCenter interface {
	// 从配置中心获取配置
	GetConfig(ctx context.Context, service, environment string) (*Config, error)
	// 推送配置到配置中心
	PutConfig(ctx context.Context, service, environment string, config *Config) error
	// 监听配置变化
	WatchConfig(ctx context.Context, service, environment string, callback ConfigChangeCallback) error
	// 停止监听
	StopWatch(service, environment string)
	// 删除配置
	DeleteConfig(ctx context.Context, service, environment string) error
	// 获取配置历史版本
	GetConfigHistory(ctx context.Context, service, environment string) ([]*ConfigVersion, error)
}

// ConfigChangeCallback 配置变更回调函数
type ConfigChangeCallback func(oldConfig, newConfig *Config) error

// ConfigVersion 配置版本信息
type ConfigVersion struct {
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Config    *Config   `json:"config"`
	Comment   string    `json:"comment"`
}

// ConsulConfigCenter 基于Consul的配置中心实现
type ConsulConfigCenter struct {
	client   *api.Client
	logger   *zap.Logger
	watchers map[string]*ConfigWatcher
	mutex    sync.RWMutex
}

// ConfigWatcher 配置监听器
type ConfigWatcher struct {
	service     string
	environment string
	callback    ConfigChangeCallback
	stopCh      chan struct{}
	lastConfig  *Config
	lastIndex   uint64
	running     bool
}

// NewConsulConfigCenter 创建Consul配置中心
func NewConsulConfigCenter(consulAddress string, logger *zap.Logger) (*ConsulConfigCenter, error) {
	config := api.DefaultConfig()
	config.Address = consulAddress

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	// 测试连接
	_, err = client.Status().Leader()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to consul: %w", err)
	}

	return &ConsulConfigCenter{
		client:   client,
		logger:   logger,
		watchers: make(map[string]*ConfigWatcher),
	}, nil
}

// GetConfig 从配置中心获取配置
func (cc *ConsulConfigCenter) GetConfig(ctx context.Context, service, environment string) (*Config, error) {
	key := cc.buildConfigKey(service, environment)

	pair, _, err := cc.client.KV().Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get config from consul: %w", err)
	}

	if pair == nil {
		return nil, fmt.Errorf("config not found for service %s in environment %s", service, environment)
	}

	var config Config
	if err := json.Unmarshal(pair.Value, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cc.logger.Info("Config retrieved from consul",
		zap.String("service", service),
		zap.String("environment", environment),
		zap.String("key", key))

	return &config, nil
}

// PutConfig 推送配置到配置中心
func (cc *ConsulConfigCenter) PutConfig(ctx context.Context, service, environment string, config *Config) error {
	key := cc.buildConfigKey(service, environment)

	// 序列化配置
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 保存当前版本到历史
	if err := cc.saveConfigHistory(ctx, service, environment, config); err != nil {
		cc.logger.Warn("Failed to save config history", zap.Error(err))
	}

	// 写入Consul
	pair := &api.KVPair{
		Key:   key,
		Value: data,
	}

	_, err = cc.client.KV().Put(pair, nil)
	if err != nil {
		return fmt.Errorf("failed to put config to consul: %w", err)
	}

	cc.logger.Info("Config pushed to consul",
		zap.String("service", service),
		zap.String("environment", environment),
		zap.String("key", key))

	return nil
}

// WatchConfig 监听配置变化
func (cc *ConsulConfigCenter) WatchConfig(ctx context.Context, service, environment string, callback ConfigChangeCallback) error {
	watcherKey := cc.buildWatcherKey(service, environment)

	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	// 检查是否已存在监听器
	if watcher, exists := cc.watchers[watcherKey]; exists && watcher.running {
		return fmt.Errorf("watcher already exists for service %s in environment %s", service, environment)
	}

	// 创建新的监听器
	watcher := &ConfigWatcher{
		service:     service,
		environment: environment,
		callback:    callback,
		stopCh:      make(chan struct{}),
		running:     true,
	}

	cc.watchers[watcherKey] = watcher

	// 启动监听协程
	go cc.runWatcher(ctx, watcher)

	cc.logger.Info("Config watcher started",
		zap.String("service", service),
		zap.String("environment", environment))

	return nil
}

// StopWatch 停止监听
func (cc *ConsulConfigCenter) StopWatch(service, environment string) {
	watcherKey := cc.buildWatcherKey(service, environment)

	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	if watcher, exists := cc.watchers[watcherKey]; exists {
		watcher.running = false
		close(watcher.stopCh)
		delete(cc.watchers, watcherKey)

		cc.logger.Info("Config watcher stopped",
			zap.String("service", service),
			zap.String("environment", environment))
	}
}

// DeleteConfig 删除配置
func (cc *ConsulConfigCenter) DeleteConfig(ctx context.Context, service, environment string) error {
	key := cc.buildConfigKey(service, environment)

	_, err := cc.client.KV().Delete(key, nil)
	if err != nil {
		return fmt.Errorf("failed to delete config from consul: %w", err)
	}

	cc.logger.Info("Config deleted from consul",
		zap.String("service", service),
		zap.String("environment", environment),
		zap.String("key", key))

	return nil
}

// GetConfigHistory 获取配置历史版本
func (cc *ConsulConfigCenter) GetConfigHistory(ctx context.Context, service, environment string) ([]*ConfigVersion, error) {
	historyKey := cc.buildConfigHistoryKey(service, environment)

	pairs, _, err := cc.client.KV().List(historyKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get config history from consul: %w", err)
	}

	var versions []*ConfigVersion
	for _, pair := range pairs {
		var version ConfigVersion
		if err := json.Unmarshal(pair.Value, &version); err != nil {
			cc.logger.Warn("Failed to unmarshal config version",
				zap.String("key", pair.Key), zap.Error(err))
			continue
		}
		versions = append(versions, &version)
	}

	return versions, nil
}

// runWatcher 运行配置监听器
func (cc *ConsulConfigCenter) runWatcher(ctx context.Context, watcher *ConfigWatcher) {
	defer func() {
		if r := recover(); r != nil {
			cc.logger.Error("Config watcher panic recovered",
				zap.String("service", watcher.service),
				zap.String("environment", watcher.environment),
				zap.Any("panic", r))
		}
	}()

	key := cc.buildConfigKey(watcher.service, watcher.environment)
	ticker := time.NewTicker(5 * time.Second) // 每5秒检查一次配置变化
	defer ticker.Stop()

	for {
		select {
		case <-watcher.stopCh:
			cc.logger.Info("Config watcher stopped",
				zap.String("service", watcher.service),
				zap.String("environment", watcher.environment))
			return

		case <-ctx.Done():
			cc.logger.Info("Config watcher context cancelled",
				zap.String("service", watcher.service),
				zap.String("environment", watcher.environment))
			return

		case <-ticker.C:
			if err := cc.checkConfigChange(watcher, key); err != nil {
				cc.logger.Error("Failed to check config change",
					zap.String("service", watcher.service),
					zap.String("environment", watcher.environment),
					zap.Error(err))
			}
		}
	}
}

// checkConfigChange 检查配置变化
func (cc *ConsulConfigCenter) checkConfigChange(watcher *ConfigWatcher, key string) error {
	queryOptions := &api.QueryOptions{
		WaitIndex: watcher.lastIndex,
		WaitTime:  30 * time.Second,
	}

	pair, meta, err := cc.client.KV().Get(key, queryOptions)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// 更新最后查询索引
	watcher.lastIndex = meta.LastIndex

	if pair == nil {
		// 配置被删除
		if watcher.lastConfig != nil {
			cc.logger.Info("Config deleted",
				zap.String("service", watcher.service),
				zap.String("environment", watcher.environment))

			if err := watcher.callback(watcher.lastConfig, nil); err != nil {
				cc.logger.Error("Config change callback failed",
					zap.Error(err))
			}
			watcher.lastConfig = nil
		}
		return nil
	}

	// 解析新配置
	var newConfig Config
	if err := json.Unmarshal(pair.Value, &newConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 检查配置是否真的发生了变化
	if !cc.configChanged(watcher.lastConfig, &newConfig) {
		return nil
	}

	cc.logger.Info("Config changed detected",
		zap.String("service", watcher.service),
		zap.String("environment", watcher.environment))

	// 调用回调函数
	oldConfig := watcher.lastConfig
	if err := watcher.callback(oldConfig, &newConfig); err != nil {
		cc.logger.Error("Config change callback failed",
			zap.Error(err))
		return err
	}

	watcher.lastConfig = &newConfig
	return nil
}

// configChanged 检查配置是否发生变化
func (cc *ConsulConfigCenter) configChanged(oldConfig, newConfig *Config) bool {
	if oldConfig == nil && newConfig != nil {
		return true
	}
	if oldConfig != nil && newConfig == nil {
		return true
	}
	if oldConfig == nil && newConfig == nil {
		return false
	}

	// 简单的JSON比较
	oldData, _ := json.Marshal(oldConfig)
	newData, _ := json.Marshal(newConfig)
	return string(oldData) != string(newData)
}

// saveConfigHistory 保存配置历史版本
func (cc *ConsulConfigCenter) saveConfigHistory(ctx context.Context, service, environment string, config *Config) error {
	version := &ConfigVersion{
		Version:   fmt.Sprintf("v%d", time.Now().Unix()),
		Timestamp: time.Now(),
		Config:    config,
		Comment:   "Auto-saved by config center",
	}

	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config version: %w", err)
	}

	historyKey := path.Join(cc.buildConfigHistoryKey(service, environment), version.Version)
	pair := &api.KVPair{
		Key:   historyKey,
		Value: data,
	}

	_, err = cc.client.KV().Put(pair, nil)
	return err
}

// buildConfigKey 构建配置键名
func (cc *ConsulConfigCenter) buildConfigKey(service, environment string) string {
	return fmt.Sprintf("high-go-press/config/%s/%s", environment, service)
}

// buildConfigHistoryKey 构建配置历史键名
func (cc *ConsulConfigCenter) buildConfigHistoryKey(service, environment string) string {
	return fmt.Sprintf("high-go-press/config-history/%s/%s", environment, service)
}

// buildWatcherKey 构建监听器键名
func (cc *ConsulConfigCenter) buildWatcherKey(service, environment string) string {
	return fmt.Sprintf("%s-%s", service, environment)
}
