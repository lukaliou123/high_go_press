package kafka

import (
	"fmt"

	"go.uber.org/zap"
)

// KafkaMode Kafka模式
type KafkaMode string

const (
	ModeMock KafkaMode = "mock"
	ModeReal KafkaMode = "real"
)

// KafkaConfig 整体Kafka配置
type KafkaConfig struct {
	Mode     KafkaMode       `yaml:"mode"` // "mock" 或 "real"
	Producer *ProducerConfig `yaml:"producer"`
	Consumer *ConsumerConfig `yaml:"consumer"`
}

// DefaultKafkaConfig 默认Kafka配置
func DefaultKafkaConfig() *KafkaConfig {
	return &KafkaConfig{
		Mode:     ModeMock, // 默认使用Mock模式
		Producer: DefaultProducerConfig(),
		Consumer: DefaultConsumerConfig(),
	}
}

// ProducerFactory Producer工厂
type ProducerFactory struct{}

// NewProducerFactory 创建Producer工厂
func NewProducerFactory() *ProducerFactory {
	return &ProducerFactory{}
}

// CreateProducer 根据配置创建Producer
func (f *ProducerFactory) CreateProducer(config *KafkaConfig, logger *zap.Logger) (Producer, error) {
	switch config.Mode {
	case ModeMock:
		logger.Info("Creating Mock Kafka Producer")
		return NewMockProducer(logger), nil

	case ModeReal:
		logger.Info("Creating Real Kafka Producer")
		return NewRealProducer(config.Producer, logger)

	default:
		return nil, fmt.Errorf("unsupported kafka mode: %s", config.Mode)
	}
}

// ConsumerFactory Consumer工厂
type ConsumerFactory struct{}

// NewConsumerFactory 创建Consumer工厂
func NewConsumerFactory() *ConsumerFactory {
	return &ConsumerFactory{}
}

// CreateConsumer 根据配置创建Consumer
func (f *ConsumerFactory) CreateConsumer(config *KafkaConfig, producer Producer, logger *zap.Logger) (Consumer, error) {
	switch config.Mode {
	case ModeMock:
		logger.Info("Creating Mock Kafka Consumer")
		// Mock Consumer需要Producer来模拟消息流
		if mockProducer, ok := producer.(*MockProducer); ok {
			return NewMockConsumer(mockProducer, logger), nil
		} else {
			return nil, fmt.Errorf("mock consumer requires mock producer")
		}

	case ModeReal:
		logger.Info("Creating Real Kafka Consumer")
		return NewRealConsumer(config.Consumer, logger)

	default:
		return nil, fmt.Errorf("unsupported kafka mode: %s", config.Mode)
	}
}

// KafkaManager Kafka管理器，统一管理Producer和Consumer
type KafkaManager struct {
	config   *KafkaConfig
	producer Producer
	consumer Consumer
	logger   *zap.Logger
}

// NewKafkaManager 创建Kafka管理器
func NewKafkaManager(config *KafkaConfig, logger *zap.Logger) (*KafkaManager, error) {
	producerFactory := NewProducerFactory()
	consumerFactory := NewConsumerFactory()

	// 创建Producer
	producer, err := producerFactory.CreateProducer(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	// 创建Consumer
	consumer, err := consumerFactory.CreateConsumer(config, producer, logger)
	if err != nil {
		producer.Close() // 清理已创建的Producer
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	return &KafkaManager{
		config:   config,
		producer: producer,
		consumer: consumer,
		logger:   logger,
	}, nil
}

// GetProducer 获取Producer
func (m *KafkaManager) GetProducer() Producer {
	return m.producer
}

// GetConsumer 获取Consumer
func (m *KafkaManager) GetConsumer() Consumer {
	return m.consumer
}

// GetMode 获取当前模式
func (m *KafkaManager) GetMode() KafkaMode {
	return m.config.Mode
}

// Close 关闭所有连接
func (m *KafkaManager) Close() error {
	m.logger.Info("Closing Kafka manager")

	var errs []error

	if m.consumer != nil {
		if err := m.consumer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("consumer close error: %w", err))
		}
	}

	if m.producer != nil {
		if err := m.producer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("producer close error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("kafka manager close errors: %v", errs)
	}

	return nil
}

// HealthCheck 健康检查
func (m *KafkaManager) HealthCheck() map[string]interface{} {
	health := make(map[string]interface{})

	health["mode"] = string(m.config.Mode)
	health["producer_stats"] = m.producer.GetStats()
	health["consumer_stats"] = m.consumer.GetStats()

	if realConsumer, ok := m.consumer.(*RealConsumer); ok {
		health["consumer_running"] = realConsumer.IsRunning()
	} else if mockConsumer, ok := m.consumer.(*MockConsumer); ok {
		health["consumer_running"] = mockConsumer.IsRunning()
	}

	return health
}
