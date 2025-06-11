package kafka

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Consumer Kafka消费者接口
type Consumer interface {
	Subscribe(topics []string) error
	ConsumeMessages(ctx context.Context, handler MessageHandler) error
	Close() error
	GetStats() ConsumerStats
}

// MessageHandler 消息处理函数
type MessageHandler func(ctx context.Context, msg *Message) error

// MockConsumer 模拟Kafka消费者（用于开发和测试）
type MockConsumer struct {
	producer *MockProducer // 引用Producer来模拟消息传递
	logger   *zap.Logger
	stats    ConsumerStats
	mu       sync.RWMutex
	running  bool
}

// NewMockConsumer 创建模拟消费者
func NewMockConsumer(producer *MockProducer, logger *zap.Logger) *MockConsumer {
	return &MockConsumer{
		producer: producer,
		logger:   logger,
		stats:    ConsumerStats{},
	}
}

// Subscribe 订阅主题
func (c *MockConsumer) Subscribe(topics []string) error {
	c.logger.Info("Mock consumer subscribed to topics", zap.Strings("topics", topics))
	return nil
}

// ConsumeMessages 消费消息
func (c *MockConsumer) ConsumeMessages(ctx context.Context, handler MessageHandler) error {
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()

	c.logger.Info("Mock consumer started consuming messages")

	ticker := time.NewTicker(2 * time.Second) // 每2秒检查一次新消息
	defer ticker.Stop()

	var lastProcessed int

	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			return ctx.Err()
		case <-ticker.C:
			// 获取新消息
			messages := c.producer.GetMessages()

			// 处理未处理的消息
			for i := lastProcessed; i < len(messages); i++ {
				msg := messages[i]

				c.logger.Debug("Processing message",
					zap.String("topic", msg.Topic),
					zap.String("key", msg.Key))

				if err := handler(ctx, &msg); err != nil {
					c.logger.Error("Failed to process message", zap.Error(err))
					c.mu.Lock()
					c.stats.ErrorsCount++
					c.mu.Unlock()
				} else {
					c.mu.Lock()
					c.stats.MessagesProcessed++
					c.mu.Unlock()
				}
			}

			lastProcessed = len(messages)
		}
	}
}

// Close 关闭消费者
func (c *MockConsumer) Close() error {
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	c.logger.Info("Mock consumer closed")
	return nil
}

// GetStats 获取统计信息
func (c *MockConsumer) GetStats() ConsumerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// IsRunning 检查是否正在运行
func (c *MockConsumer) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// ConsumerStats 消费者统计信息
type ConsumerStats struct {
	MessagesProcessed int64 `json:"messages_processed"`
	ErrorsCount       int64 `json:"errors_count"`
	LastMessageTime   int64 `json:"last_message_time"`
}

// CounterEventHandler 计数器事件处理器
type CounterEventHandler struct {
	updateFunc func(ctx context.Context, event *CounterEvent) error
	logger     *zap.Logger
}

// NewCounterEventHandler 创建计数器事件处理器
func NewCounterEventHandler(updateFunc func(ctx context.Context, event *CounterEvent) error, logger *zap.Logger) *CounterEventHandler {
	return &CounterEventHandler{
		updateFunc: updateFunc,
		logger:     logger,
	}
}

// HandleMessage 处理消息
func (h *CounterEventHandler) HandleMessage(ctx context.Context, msg *Message) error {
	// 检查是否是计数器事件
	if msg.Headers["event_type"] != "counter_update" {
		h.logger.Debug("Skipping non-counter event", zap.String("event_type", msg.Headers["event_type"]))
		return nil
	}

	// 反序列化计数器事件
	var event CounterEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return err
	}

	h.logger.Info("Processing counter event",
		zap.String("resource_id", event.ResourceID),
		zap.String("counter_type", event.CounterType),
		zap.Int64("delta", event.Delta))

	// 调用更新函数
	return h.updateFunc(ctx, &event)
}
