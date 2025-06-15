#!/bin/bash

echo "🚀 HighGoPress 快速启动"
echo "======================"

# 1. 修复Go依赖
echo "📦 修复Go依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "❌ Go依赖修复失败"
    exit 1
fi
echo "✅ Go依赖修复完成"

# 2. 启动Kafka (简化版)
echo "🔥 启动Kafka..."
docker run -d --name highgopress-kafka-quick \
    -p 9092:9092 \
    -e KAFKA_ZOOKEEPER_CONNECT=localhost:2181 \
    -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 \
    -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
    -e KAFKA_AUTO_CREATE_TOPICS_ENABLE=true \
    confluentinc/cp-kafka:7.4.0 &

# 3. 启动Redis
echo "💾 启动Redis..."
docker run -d --name highgopress-redis-quick \
    -p 6379:6379 \
    redis:7.2-alpine &

# 等待服务启动
echo "⏳ 等待服务启动..."
sleep 10

# 4. 测试单个服务启动
echo "🧪 测试Gateway服务..."
go run cmd/gateway/main.go &
GATEWAY_PID=$!

echo "Gateway PID: $GATEWAY_PID"
echo "等待5秒后测试..."
sleep 5

# 测试Gateway是否正常
if curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
    echo "✅ Gateway服务正常启动"
else
    echo "❌ Gateway服务启动失败"
    kill $GATEWAY_PID 2>/dev/null
    exit 1
fi

echo "🎉 快速启动完成！"
echo "Gateway服务运行在: http://localhost:8080"
echo "停止服务: kill $GATEWAY_PID" 