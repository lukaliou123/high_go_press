package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Message Kafka消息结构
type Message struct {
	Topic     string            `json:"topic"`
	Key       string            `json:"key"`
	Value     []byte            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// CounterEvent 计数事件消息
type CounterEvent struct {
	EventID     string    `json:"event_id"`
	ResourceID  string    `json:"resource_id"`
	CounterType string    `json:"counter_type"`
	Delta       int64     `json:"delta"`
	NewValue    int64     `json:"new_value"`
	UserID      string    `json:"user_id,omitempty"`
	IP          string    `json:"ip,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"` // API, BATCH, SYSTEM等
}

// Producer Kafka生产者接口
type Producer interface {
	SendMessage(ctx context.Context, msg *Message) error
	SendCounterEvent(ctx context.Context, event *CounterEvent) error
	Close() error
	GetStats() ProducerStats
}

// MockProducer 模拟Kafka生产者（用于开发和测试）
type MockProducer struct {
	messages []Message
	events   []CounterEvent
	mu       sync.RWMutex
	logger   *zap.Logger
	stats    ProducerStats
}

// NewMockProducer 创建模拟生产者
func NewMockProducer(logger *zap.Logger) *MockProducer {
	return &MockProducer{
		messages: make([]Message, 0),
		events:   make([]CounterEvent, 0),
		logger:   logger,
		stats:    ProducerStats{},
	}
}

// SendMessage 发送消息
func (p *MockProducer) SendMessage(ctx context.Context, msg *Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 模拟网络延迟
	time.Sleep(time.Millisecond * 1)

	p.messages = append(p.messages, *msg)
	p.stats.MessagesSent++

	p.logger.Debug("Message sent to kafka",
		zap.String("topic", msg.Topic),
		zap.String("key", msg.Key),
		zap.Int("value_size", len(msg.Value)))

	return nil
}

// SendCounterEvent 发送计数事件
func (p *MockProducer) SendCounterEvent(ctx context.Context, event *CounterEvent) error {
	// 序列化事件
	eventJSON, err := json.Marshal(event)
	if err != nil {
		p.stats.ErrorsCount++
		return fmt.Errorf("failed to marshal counter event: %w", err)
	}

	// 构造Kafka消息
	msg := &Message{
		Topic: "counter-events",
		Key:   fmt.Sprintf("%s:%s", event.ResourceID, event.CounterType),
		Value: eventJSON,
		Headers: map[string]string{
			"event_type": "counter_update",
			"source":     event.Source,
		},
		Timestamp: event.Timestamp,
	}

	// 发送消息
	if err := p.SendMessage(ctx, msg); err != nil {
		p.stats.ErrorsCount++
		return err
	}

	p.mu.Lock()
	p.events = append(p.events, *event)
	p.stats.EventsSent++
	p.mu.Unlock()

	return nil
}

// Close 关闭生产者
func (p *MockProducer) Close() error {
	p.logger.Info("Mock producer closed",
		zap.Int("total_messages", len(p.messages)),
		zap.Int("total_events", len(p.events)))
	return nil
}

// GetStats 获取统计信息
func (p *MockProducer) GetStats() ProducerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := p.stats
	stats.MessagesQueued = int64(len(p.messages))
	stats.EventsQueued = int64(len(p.events))

	return stats
}

// GetMessages 获取所有消息（测试用）
func (p *MockProducer) GetMessages() []Message {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]Message, len(p.messages))
	copy(result, p.messages)
	return result
}

// GetEvents 获取所有事件（测试用）
func (p *MockProducer) GetEvents() []CounterEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]CounterEvent, len(p.events))
	copy(result, p.events)
	return result
}

// ProducerStats 生产者统计信息
type ProducerStats struct {
	MessagesSent    int64 `json:"messages_sent"`
	EventsSent      int64 `json:"events_sent"`
	MessagesQueued  int64 `json:"messages_queued"`
	EventsQueued    int64 `json:"events_queued"`
	ErrorsCount     int64 `json:"errors_count"`
	LastMessageTime int64 `json:"last_message_time"`
}

// ProducerConfig Kafka生产者配置
type ProducerConfig struct {
	Brokers          []string `yaml:"brokers"`
	Topic            string   `yaml:"topic"`
	EnableAsync      bool     `yaml:"enable_async"`
	BatchSize        int      `yaml:"batch_size"`
	FlushInterval    int      `yaml:"flush_interval_ms"`
	CompressionType  string   `yaml:"compression_type"`
	Retries          int      `yaml:"retries"`
	EnableIdempotent bool     `yaml:"enable_idempotent"`
}

// DefaultProducerConfig 默认配置
func DefaultProducerConfig() *ProducerConfig {
	return &ProducerConfig{
		Brokers:          []string{"localhost:9092"},
		Topic:            "counter-events",
		EnableAsync:      true,
		BatchSize:        100,
		FlushInterval:    100, // 100ms
		CompressionType:  "snappy",
		Retries:          3,
		EnableIdempotent: true,
	}
}
