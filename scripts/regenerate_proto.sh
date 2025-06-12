#!/bin/bash

# HighGoPress Protobuf 重新生成脚本
# 修复 import 路径问题

set -e

echo "🔄 重新生成 Protobuf 代码..."

# 清理旧的生成文件
echo "🧹 清理旧的生成文件..."
find api/proto -name "*.pb.go" -delete || true
find api/proto -name "*_grpc.pb.go" -delete || true

# 生成 common 类型 (必须先生成)
echo "📋 生成通用类型..."
protoc --go_out=. --go_opt=paths=source_relative \
       api/proto/common/types.proto

# 生成 counter 服务 (依赖 common)
echo "⚡ 生成 Counter 服务..."
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       -I. \
       api/proto/counter/counter.proto

echo "✅ Protobuf 代码重新生成完成！"

# 验证生成的文件
echo ""
echo "📁 生成的文件:"
find api/proto -name "*.pb.go" -o -name "*_grpc.pb.go" | sort

echo ""
echo "🧪 验证编译..."
go mod tidy

echo "🎉 Protobuf 重新生成成功！" 