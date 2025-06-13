#!/bin/bash

# 简单的Kafka功能测试
echo "🚀 简单Kafka功能测试"
echo "==================="

# 检查Kafka是否运行
if ! docker ps | grep kafka-highgopress > /dev/null; then
    echo "❌ Kafka容器未运行"
    exit 1
fi
echo "✅ Kafka容器运行正常"

# 检查topic是否存在
echo "📋 检查topic..."
docker exec kafka-highgopress /opt/kafka/bin/kafka-topics.sh --list --bootstrap-server localhost:9092

# 发送测试消息
echo "📤 发送测试消息..."
echo "test-message-$(date +%s)" | docker exec -i kafka-highgopress /opt/kafka/bin/kafka-console-producer.sh --topic counter-events --bootstrap-server localhost:9092

# 检查消息数量
echo "📊 检查消息数量..."
message_count=$(docker exec kafka-highgopress /opt/kafka/bin/kafka-run-class.sh kafka.tools.GetOffsetShell --broker-list localhost:9092 --topic counter-events --time -1 | awk -F':' '{sum += $3} END {print sum}' 2>/dev/null || echo "0")
echo "Topic counter-events 中的消息数量: $message_count"

# 检查消费者组
echo "👥 检查消费者组..."
docker exec kafka-highgopress /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --list 2>/dev/null || echo "暂无消费者组"

# 检查Counter和Analytics服务日志中的Kafka相关信息
echo "📝 检查服务日志..."
echo "=== Counter服务Kafka日志 ==="
grep -i kafka logs/counter.log | tail -3 || echo "无Kafka相关日志"

echo "=== Analytics服务Kafka日志 ==="
grep -i kafka logs/analytics.log | tail -3 || echo "无Kafka相关日志"

echo "✅ Kafka功能测试完成" 