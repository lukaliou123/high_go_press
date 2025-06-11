package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// RealConsumer 真实的Kafka消费者（Consumer Group）
type RealConsumer struct {
	consumerGroup sarama.ConsumerGroup
	topics        []string
	groupID       string
	handler       MessageHandler
	logger        *zap.Logger
	stats         ConsumerStats
	mu            sync.RWMutex
	running       bool
}

// ConsumerConfig Kafka消费者配置
type ConsumerConfig struct {
	Brokers           []string `yaml:"brokers"`
	GroupID           string   `yaml:"group_id"`
	Topics            []string `yaml:"topics"`
	AutoOffsetReset   string   `yaml:"auto_offset_reset"` // earliest, latest
	SessionTimeout    int      `yaml:"session_timeout_ms"`
	HeartbeatInterval int      `yaml:"heartbeat_interval_ms"`
}

// DefaultConsumerConfig 默认消费者配置
func DefaultConsumerConfig() *ConsumerConfig {
	return &ConsumerConfig{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "analytics-group",
		Topics:            []string{"counter-events"},
		AutoOffsetReset:   "latest",
		SessionTimeout:    10000, // 10s
		HeartbeatInterval: 3000,  // 3s
	}
}

// NewRealConsumer 创建真实的Kafka消费者
func NewRealConsumer(config *ConsumerConfig, logger *zap.Logger) (*RealConsumer, error) {
	// 创建Sarama配置
	saramaConfig := sarama.NewConfig()

	// Consumer配置
	saramaConfig.Consumer.Group.Session.Timeout = time.Duration(config.SessionTimeout) * time.Millisecond
	saramaConfig.Consumer.Group.Heartbeat.Interval = time.Duration(config.HeartbeatInterval) * time.Millisecond
	saramaConfig.Consumer.Return.Errors = true

	// Offset配置
	switch config.AutoOffsetReset {
	case "earliest":
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	case "latest":
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	default:
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	}

	// 版本配置
	saramaConfig.Version = sarama.V2_6_0_0

	// 创建Consumer Group
	consumerGroup, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, saramaConfig)
	if err != nil {
		return nil, err
	}

	realConsumer := &RealConsumer{
		consumerGroup: consumerGroup,
		topics:        config.Topics,
		groupID:       config.GroupID,
		logger:        logger,
		stats:         ConsumerStats{},
	}

	logger.Info("Real Kafka consumer created",
		zap.Strings("brokers", config.Brokers),
		zap.String("group_id", config.GroupID),
		zap.Strings("topics", config.Topics))

	return realConsumer, nil
}

// Subscribe 订阅主题
func (c *RealConsumer) Subscribe(topics []string) error {
	c.topics = topics
	c.logger.Info("Subscribed to topics", zap.Strings("topics", topics))
	return nil
}

// ConsumeMessages 消费消息
func (c *RealConsumer) ConsumeMessages(ctx context.Context, handler MessageHandler) error {
	c.mu.Lock()
	c.running = true
	c.handler = handler
	c.mu.Unlock()

	c.logger.Info("Starting real Kafka consumer",
		zap.String("group_id", c.groupID),
		zap.Strings("topics", c.topics))

	// 创建消费者处理器
	consumerHandler := &consumerGroupHandler{
		consumer: c,
		logger:   c.logger,
	}

	// 启动错误处理goroutine
	go c.handleErrors(ctx)

	// 开始消费
	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			return ctx.Err()
		default:
			// 消费消息，这是阻塞调用
			if err := c.consumerGroup.Consume(ctx, c.topics, consumerHandler); err != nil {
				c.logger.Error("Consumer group consume error", zap.Error(err))
				c.mu.Lock()
				c.stats.ErrorsCount++
				c.mu.Unlock()

				// 如果是致命错误，退出
				if err == sarama.ErrClosedConsumerGroup {
					return err
				}

				// 其他错误，短暂等待后重试
				time.Sleep(time.Second)
			}
		}
	}
}

// handleErrors 处理错误
func (c *RealConsumer) handleErrors(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-c.consumerGroup.Errors():
			c.logger.Error("Consumer group error", zap.Error(err))
			c.mu.Lock()
			c.stats.ErrorsCount++
			c.mu.Unlock()
		}
	}
}

// Close 关闭消费者
func (c *RealConsumer) Close() error {
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	c.logger.Info("Closing real Kafka consumer")
	return c.consumerGroup.Close()
}

// GetStats 获取统计信息
func (c *RealConsumer) GetStats() ConsumerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// IsRunning 检查是否正在运行
func (c *RealConsumer) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// consumerGroupHandler Sarama Consumer Group处理器
type consumerGroupHandler struct {
	consumer *RealConsumer
	logger   *zap.Logger
}

// Setup 消费者组启动时调用
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Consumer group session setup")
	return nil
}

// Cleanup 消费者组关闭时调用
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Consumer group session cleanup")
	return nil
}

// ConsumeClaim 消费消息
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case saramaMsg := <-claim.Messages():
			if saramaMsg == nil {
				return nil
			}

			// 转换为内部Message格式
			msg := &Message{
				Topic:     saramaMsg.Topic,
				Key:       string(saramaMsg.Key),
				Value:     saramaMsg.Value,
				Headers:   make(map[string]string),
				Timestamp: saramaMsg.Timestamp,
			}

			// 转换Headers
			for _, header := range saramaMsg.Headers {
				msg.Headers[string(header.Key)] = string(header.Value)
			}

			h.logger.Debug("Processing message",
				zap.String("topic", msg.Topic),
				zap.String("key", msg.Key),
				zap.Int32("partition", saramaMsg.Partition),
				zap.Int64("offset", saramaMsg.Offset))

			// 调用消息处理器
			if err := h.consumer.handler(session.Context(), msg); err != nil {
				h.logger.Error("Failed to process message",
					zap.Error(err),
					zap.String("topic", msg.Topic),
					zap.String("key", msg.Key))

				h.consumer.mu.Lock()
				h.consumer.stats.ErrorsCount++
				h.consumer.mu.Unlock()

				// 根据策略决定是否跳过这条消息
				// 这里我们选择跳过并继续处理下一条
			} else {
				h.consumer.mu.Lock()
				h.consumer.stats.MessagesProcessed++
				h.consumer.stats.LastMessageTime = time.Now().Unix()
				h.consumer.mu.Unlock()
			}

			// 标记消息已处理（提交offset）
			session.MarkMessage(saramaMsg, "")
		}
	}
}
