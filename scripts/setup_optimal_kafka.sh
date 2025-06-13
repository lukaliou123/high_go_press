#!/bin/bash

# HighGoPress 最优Kafka配置脚本
# 设置4个分区的counter-events topic以实现最佳性能

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 HighGoPress 最优Kafka配置${NC}"
echo "=================================="

# 检查Kafka目录
KAFKA_DIR="./kafka_2.13-3.9.1"
if [ ! -d "$KAFKA_DIR" ]; then
    echo -e "${RED}❌ Kafka目录不存在: $KAFKA_DIR${NC}"
    exit 1
fi

# 停止现有的Kafka进程
echo -e "${YELLOW}🛑 停止现有Kafka进程...${NC}"
pkill -f kafka || true
sleep 3

# 清理旧的日志目录
echo -e "${YELLOW}🧹 清理旧的日志目录...${NC}"
rm -rf /tmp/kraft-combined-logs
rm -rf /tmp/kafka-logs

# 创建最优配置文件
echo -e "${BLUE}⚙️ 创建最优Kafka配置...${NC}"
cat > $KAFKA_DIR/config/kraft/server-optimal.properties << 'EOF'
# HighGoPress 最优KRaft配置
# 针对高并发计数服务优化

############################
# 基础配置
############################
process.roles=broker,controller
node.id=1
controller.quorum.voters=1@localhost:9093

############################
# 网络配置
############################
listeners=PLAINTEXT://localhost:9092,CONTROLLER://localhost:9093
advertised.listeners=PLAINTEXT://localhost:9092
controller.listener.names=CONTROLLER
inter.broker.listener.name=PLAINTEXT
listener.security.protocol.map=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT

############################
# 存储配置
############################
log.dirs=/tmp/kraft-combined-logs

############################
# 性能优化配置
############################
# 网络线程数 - 处理网络请求
num.network.threads=8

# IO线程数 - 处理磁盘IO
num.io.threads=16

# 网络缓冲区大小
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600

# 默认分区数设置为4（最优并发）
num.partitions=4

# 副本配置
default.replication.factor=1
min.insync.replicas=1

# 日志段配置
log.segment.bytes=1073741824
log.retention.hours=24
log.retention.check.interval.ms=300000

# 生产者优化
num.replica.fetchers=4
replica.fetch.max.bytes=1048576
message.max.bytes=1000000

# 批处理优化
replica.fetch.wait.max.ms=100
fetch.purgatory.purge.interval.requests=1000
producer.purgatory.purge.interval.requests=1000

# 自动创建topic
auto.create.topics.enable=true

# 内部topic配置
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1

# 压缩配置
compression.type=snappy
EOF

# 初始化存储
echo -e "${BLUE}🔧 初始化Kafka存储...${NC}"
CLUSTER_UUID=$($KAFKA_DIR/bin/kafka-storage.sh random-uuid)
echo -e "${GREEN}🆔 集群UUID: $CLUSTER_UUID${NC}"

$KAFKA_DIR/bin/kafka-storage.sh format \
    -t $CLUSTER_UUID \
    -c $KAFKA_DIR/config/kraft/server-optimal.properties

# 启动Kafka
echo -e "${BLUE}🚀 启动优化的Kafka服务器...${NC}"
$KAFKA_DIR/bin/kafka-server-start.sh $KAFKA_DIR/config/kraft/server-optimal.properties &

# 等待Kafka启动
echo -e "${YELLOW}⏳ 等待Kafka启动（30秒）...${NC}"
sleep 30

# 验证Kafka是否启动成功
for i in {1..10}; do
    if $KAFKA_DIR/bin/kafka-broker-api-versions.sh --bootstrap-server localhost:9092 &>/dev/null; then
        echo -e "${GREEN}✅ Kafka启动成功！${NC}"
        break
    fi
    if [ $i -eq 10 ]; then
        echo -e "${RED}❌ Kafka启动超时${NC}"
        exit 1
    fi
    echo "   等待Kafka就绪... ($i/10)"
    sleep 3
done

# 删除现有的counter-events topic（如果存在）
echo -e "${YELLOW}🗑️ 删除现有的counter-events topic...${NC}"
$KAFKA_DIR/bin/kafka-topics.sh --delete \
    --topic counter-events \
    --bootstrap-server localhost:9092 2>/dev/null || true

sleep 5

# 创建最优的counter-events topic
echo -e "${BLUE}📋 创建最优的counter-events topic...${NC}"
$KAFKA_DIR/bin/kafka-topics.sh --create \
    --topic counter-events \
    --bootstrap-server localhost:9092 \
    --partitions 4 \
    --replication-factor 1 \
    --config retention.ms=86400000 \
    --config segment.ms=3600000 \
    --config compression.type=snappy \
    --config min.insync.replicas=1 \
    --config unclean.leader.election.enable=false

# 验证topic创建
echo -e "${BLUE}🔍 验证topic配置...${NC}"
$KAFKA_DIR/bin/kafka-topics.sh --describe \
    --topic counter-events \
    --bootstrap-server localhost:9092

echo ""
echo -e "${GREEN}🎉 最优Kafka配置完成！${NC}"
echo "=================================="
echo -e "${GREEN}📍 Broker地址: localhost:9092${NC}"
echo -e "${GREEN}📍 Topic: counter-events${NC}"
echo -e "${GREEN}📍 分区数: 4 (最优并发)${NC}"
echo -e "${GREEN}📍 副本数: 1${NC}"
echo -e "${GREEN}📍 压缩: snappy${NC}"
echo -e "${GREEN}📍 保留时间: 24小时${NC}"
echo ""
echo -e "${BLUE}🧪 测试命令:${NC}"
echo "   # 生产者测试:"
echo "   $KAFKA_DIR/bin/kafka-console-producer.sh --broker-list localhost:9092 --topic counter-events"
echo ""
echo "   # 消费者测试:"
echo "   $KAFKA_DIR/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic counter-events --from-beginning"
echo ""
echo -e "${YELLOW}⚡ 现在可以设置 KAFKA_MODE=real 来启用真实Kafka！${NC}" 