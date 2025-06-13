#!/bin/bash

# HighGoPress Kafka KRaft模式启动脚本
# 无需Zookeeper，与Consul架构保持一致

set -e

echo "🚀 启动 Kafka KRaft 模式（无需Zookeeper）"
echo "============================================="

# 配置 - 使用最新稳定版本
KAFKA_VERSION="2.13-3.9.1"
KAFKA_DIR="kafka_${KAFKA_VERSION}"
DOWNLOAD_URL="https://downloads.apache.org/kafka/3.9.1/kafka_${KAFKA_VERSION}.tgz"
KRAFT_LOG_DIR="/tmp/kraft-combined-logs"

# 检查是否已下载Kafka
if [ ! -d "$KAFKA_DIR" ]; then
    echo "📥 下载 Kafka 3.9.1 ..."
    wget $DOWNLOAD_URL
    tar -xzf kafka_${KAFKA_VERSION}.tgz
    echo "✅ Kafka 下载完成"
fi

# 创建KRaft配置文件
echo "⚙️  创建 KRaft 配置..."
cat > $KAFKA_DIR/config/kraft/server-highgopress.properties << EOF
# HighGoPress KRaft配置
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.

############################
# HighGoPress KRaft Server 配置
############################

# 身份配置
process.roles=broker,controller
node.id=1
controller.quorum.voters=1@localhost:9093

# 网络配置
listeners=PLAINTEXT://localhost:9092,CONTROLLER://localhost:9093
advertised.listeners=PLAINTEXT://localhost:9092
controller.listener.names=CONTROLLER
inter.broker.listener.name=PLAINTEXT

# 存储配置
log.dirs=${KRAFT_LOG_DIR}
num.network.threads=3
num.io.threads=8
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600

# 日志保留策略（针对高流量优化）
num.partitions=3
default.replication.factor=1
min.insync.replicas=1
log.retention.hours=168
log.segment.bytes=1073741824
log.retention.check.interval.ms=300000

# 性能优化配置
num.replica.fetchers=1
replica.fetch.max.bytes=1048576
message.max.bytes=1000000
replica.fetch.wait.max.ms=500
fetch.purgatory.purge.interval.requests=1000
producer.purgatory.purge.interval.requests=1000
delete.records.purgatory.purge.interval.requests=1

# HighGoPress 主题自动创建
auto.create.topics.enable=true
EOF

# 检查是否需要初始化存储
if [ ! -d "$KRAFT_LOG_DIR" ]; then
    echo "🔧 格式化存储目录..."
    mkdir -p $KRAFT_LOG_DIR
    
    # 生成集群UUID
    CLUSTER_UUID=$($KAFKA_DIR/bin/kafka-storage.sh random-uuid)
    echo "🆔 集群UUID: $CLUSTER_UUID"
    
    # 格式化存储
    $KAFKA_DIR/bin/kafka-storage.sh format \
        -t $CLUSTER_UUID \
        -c $KAFKA_DIR/config/kraft/server-highgopress.properties
    
    echo "✅ 存储格式化完成"
else
    echo "✅ 存储目录已存在，跳过格式化"
fi

# 启动Kafka
echo "🚀 启动 Kafka KRaft 服务器..."
$KAFKA_DIR/bin/kafka-server-start.sh $KAFKA_DIR/config/kraft/server-highgopress.properties &

# 等待Kafka启动
echo "⏳ 等待 Kafka 启动（30秒）..."
sleep 30

# 创建counter-events主题
echo "📋 创建 counter-events 主题..."
$KAFKA_DIR/bin/kafka-topics.sh --create \
    --topic counter-events \
    --bootstrap-server localhost:9092 \
    --partitions 3 \
    --replication-factor 1 \
    --config retention.ms=604800000 \
    --config segment.ms=86400000 \
    --if-not-exists

# 验证主题创建
echo "✅ 验证主题创建..."
$KAFKA_DIR/bin/kafka-topics.sh --list \
    --bootstrap-server localhost:9092

echo ""
echo "🎉 Kafka KRaft 模式启动成功！"
echo "================================="
echo "📍 Broker地址: localhost:9092"
echo "📍 无需Zookeeper - 使用KRaft协议"
echo "📍 主题: counter-events (3分区)"
echo "⚡ 现在可以设置 KAFKA_MODE=real 来启用真实Kafka!"
echo ""
echo "🔍 测试连接:"
echo "   $KAFKA_DIR/bin/kafka-console-producer.sh --broker-list localhost:9092 --topic counter-events"
echo "   $KAFKA_DIR/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic counter-events --from-beginning" 