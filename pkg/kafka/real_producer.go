package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// RealProducer 真实的Kafka生产者
type RealProducer struct {
	producer  sarama.SyncProducer
	asyncProd sarama.AsyncProducer
	config    *ProducerConfig
	logger    *zap.Logger
	stats     ProducerStats
	isAsync   bool
}

// NewRealProducer 创建真实的Kafka生产者
func NewRealProducer(config *ProducerConfig, logger *zap.Logger) (*RealProducer, error) {
	// 创建Sarama配置
	saramaConfig := sarama.NewConfig()

	// 基础配置
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll // 等待所有副本确认
	saramaConfig.Producer.Retry.Max = config.Retries
	saramaConfig.Producer.Flush.Frequency = time.Duration(config.FlushInterval) * time.Millisecond

	// 批处理配置
	if config.BatchSize > 0 {
		saramaConfig.Producer.Flush.Messages = config.BatchSize
	}

	// 压缩配置
	switch config.CompressionType {
	case "gzip":
		saramaConfig.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		saramaConfig.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaConfig.Producer.Compression = sarama.CompressionZSTD
	default:
		saramaConfig.Producer.Compression = sarama.CompressionNone
	}

	// 幂等性配置
	if config.EnableIdempotent {
		saramaConfig.Producer.Idempotent = true
		saramaConfig.Net.MaxOpenRequests = 1
	}

	// 版本配置
	saramaConfig.Version = sarama.V2_6_0_0

	realProd := &RealProducer{
		config:  config,
		logger:  logger,
		stats:   ProducerStats{},
		isAsync: config.EnableAsync,
	}

	if config.EnableAsync {
		// 创建异步生产者
		asyncProd, err := sarama.NewAsyncProducer(config.Brokers, saramaConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create async producer: %w", err)
		}
		realProd.asyncProd = asyncProd

		// 启动错误和成功处理goroutine
		go realProd.handleAsyncResponses()
	} else {
		// 创建同步生产者
		syncProd, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create sync producer: %w", err)
		}
		realProd.producer = syncProd
	}

	logger.Info("Real Kafka producer created",
		zap.Strings("brokers", config.Brokers),
		zap.Bool("async", config.EnableAsync),
		zap.String("compression", config.CompressionType))

	return realProd, nil
}

// SendMessage 发送消息
func (p *RealProducer) SendMessage(ctx context.Context, msg *Message) error {
	// 创建Sarama消息
	saramaMsg := &sarama.ProducerMessage{
		Topic:     msg.Topic,
		Key:       sarama.StringEncoder(msg.Key),
		Value:     sarama.ByteEncoder(msg.Value),
		Headers:   make([]sarama.RecordHeader, 0, len(msg.Headers)),
		Timestamp: msg.Timestamp,
	}

	// 添加Headers
	for k, v := range msg.Headers {
		saramaMsg.Headers = append(saramaMsg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	if p.isAsync {
		return p.sendAsync(ctx, saramaMsg)
	} else {
		return p.sendSync(ctx, saramaMsg)
	}
}

// sendSync 同步发送
func (p *RealProducer) sendSync(ctx context.Context, msg *sarama.ProducerMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.stats.ErrorsCount++
		p.logger.Error("Failed to send message",
			zap.String("topic", msg.Topic),
			zap.Error(err))
		return err
	}

	p.stats.MessagesSent++
	p.stats.LastMessageTime = time.Now().Unix()

	p.logger.Debug("Message sent successfully",
		zap.String("topic", msg.Topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset))

	return nil
}

// sendAsync 异步发送
func (p *RealProducer) sendAsync(ctx context.Context, msg *sarama.ProducerMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.asyncProd.Input() <- msg:
		return nil
	}
}

// handleAsyncResponses 处理异步响应
func (p *RealProducer) handleAsyncResponses() {
	for {
		select {
		case success := <-p.asyncProd.Successes():
			p.stats.MessagesSent++
			p.stats.LastMessageTime = time.Now().Unix()
			p.logger.Debug("Async message sent successfully",
				zap.String("topic", success.Topic),
				zap.Int32("partition", success.Partition),
				zap.Int64("offset", success.Offset))

		case err := <-p.asyncProd.Errors():
			p.stats.ErrorsCount++
			p.logger.Error("Async message send failed",
				zap.String("topic", err.Msg.Topic),
				zap.Error(err.Err))
		}
	}
}

// SendCounterEvent 发送计数事件
func (p *RealProducer) SendCounterEvent(ctx context.Context, event *CounterEvent) error {
	// 序列化事件
	eventJSON, err := json.Marshal(event)
	if err != nil {
		p.stats.ErrorsCount++
		return fmt.Errorf("failed to marshal counter event: %w", err)
	}

	// 构造Kafka消息
	msg := &Message{
		Topic: p.config.Topic,
		Key:   fmt.Sprintf("%s:%s", event.ResourceID, event.CounterType),
		Value: eventJSON,
		Headers: map[string]string{
			"event_type":   "counter_update",
			"source":       event.Source,
			"event_id":     event.EventID,
			"content_type": "application/json",
		},
		Timestamp: event.Timestamp,
	}

	// 发送消息
	if err := p.SendMessage(ctx, msg); err != nil {
		return err
	}

	p.stats.EventsSent++

	p.logger.Info("Counter event sent to Kafka",
		zap.String("event_id", event.EventID),
		zap.String("resource_id", event.ResourceID),
		zap.String("counter_type", event.CounterType),
		zap.Int64("delta", event.Delta))

	return nil
}

// Close 关闭生产者
func (p *RealProducer) Close() error {
	p.logger.Info("Closing real Kafka producer")

	if p.isAsync && p.asyncProd != nil {
		return p.asyncProd.Close()
	} else if p.producer != nil {
		return p.producer.Close()
	}

	return nil
}

// GetStats 获取统计信息
func (p *RealProducer) GetStats() ProducerStats {
	return p.stats
}

// IsConnected 检查连接状态
func (p *RealProducer) IsConnected() bool {
	// 对于Sarama，我们假设只要没有Close就是连接的
	if p.isAsync {
		return p.asyncProd != nil
	}
	return p.producer != nil
}
