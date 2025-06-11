#!/bin/bash

# HighGoPress Protobuf 代码生成脚本
# Day 1 基础设施搭建

set -e

echo "🚀 开始生成 Protobuf 代码..."

# 确保目标目录存在
mkdir -p api/generated/{common,counter,analytics}

# 生成 common 类型
echo "📋 生成通用类型..."
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/common/types.proto

# 生成 counter 服务
echo "⚡ 生成 Counter 服务..."
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/counter/counter.proto

# 生成 analytics 服务
echo "📊 生成 Analytics 服务..."
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/analytics/analytics.proto

echo "✅ Protobuf 代码生成完成！"

# 验证生成的文件
echo ""
echo "📁 生成的文件:"
find api/proto -name "*.pb.go" -o -name "*_grpc.pb.go" | sort

echo ""
echo "🧪 验证编译..."
go mod tidy
go build ./...

echo "🎉 Day 1 任务 1.3 完成：代码生成工具链配置成功！" 