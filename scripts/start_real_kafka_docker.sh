#!/bin/bash

# HighGoPress 使用Docker启动真实Kafka
# 用于性能测试对比

set -e

echo "🐳 使用Docker启动真实Kafka"
echo "=========================="

# 检查Docker是否安装
if ! command -v docker &> /dev/null; then
    echo "❌ Docker未安装，请先安装Docker"
    exit 1
fi

# 停止并清理现有容器
echo "🧹 清理现有Kafka容器..."
docker stop kafka-highgopress 2>/dev/null || true
docker rm kafka-highgopress 2>/dev/null || true

# 启动Kafka容器 (KRaft模式，无需Zookeeper)
echo "🚀 启动Kafka容器..."
docker run -d \
  --name kafka-highgopress \
  -p 9092:9092 \
  -e KAFKA_NODE_ID=1 \
  -e KAFKA_PROCESS_ROLES=broker,controller \
  -e KAFKA_CONTROLLER_QUORUM_VOTERS=1@localhost:9093 \
  -e KAFKA_LISTENERS=PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093 \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 \
  -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER \
  -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT \
  -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 \
  -e KAFKA_AUTO_CREATE_TOPICS_ENABLE=true \
  -e KAFKA_NUM_PARTITIONS=4 \
  -e KAFKA_LOG_RETENTION_HOURS=1 \
  apache/kafka:latest

echo "⏳ 等待Kafka启动..."
sleep 10

# 等待Kafka就绪
for i in {1..30}; do
    if docker exec kafka-highgopress /opt/kafka/bin/kafka-broker-api-versions.sh --bootstrap-server localhost:9092 &>/dev/null; then
        echo "✅ Kafka启动成功！"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ Kafka启动超时"
        exit 1
    fi
    echo "   等待Kafka就绪... ($i/30)"
    sleep 2
done

# 创建counter-events主题
echo "📋 创建counter-events主题..."
docker exec kafka-highgopress /opt/kafka/bin/kafka-topics.sh \
  --create \
  --topic counter-events \
  --bootstrap-server localhost:9092 \
  --partitions 4 \
  --replication-factor 1

# 验证主题创建
echo "🔍 验证主题列表..."
docker exec kafka-highgopress /opt/kafka/bin/kafka-topics.sh \
  --list \
  --bootstrap-server localhost:9092

echo ""
echo "🎉 真实Kafka启动成功！"
echo "📊 配置信息:"
echo "   - Broker: localhost:9092"
echo "   - Topic: counter-events (4 partitions)"
echo "   - 模式: KRaft (无Zookeeper)"
echo "   - 容器名: kafka-highgopress"
echo ""
echo "🧪 现在可以运行性能测试了！" 