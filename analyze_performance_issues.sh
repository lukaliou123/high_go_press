#!/bin/bash

echo "🔍 HighGoPress Week 4 性能问题分析"
echo "================================="

# 1. 检查服务进程状态
echo "1. 检查服务进程..."
ps aux | grep -E "(counter|analytics|gateway)" | grep -v grep | head -10

# 2. 检查日志文件大小
echo -e "\n2. 检查日志文件..."
ls -lh logs/ 2>/dev/null || echo "No logs directory"

# 3. 查看最后的错误信息
echo -e "\n3. Counter服务最后50行日志..."
if [ -f "logs/counter.log" ]; then
    tail -50 logs/counter.log | grep -E "(ERROR|error|Error|FATAL|panic|failed)"
else
    echo "Counter日志文件不存在"
fi

echo -e "\n4. Gateway服务最后50行日志..."
if [ -f "logs/gateway.log" ]; then
    tail -50 logs/gateway.log | grep -E "(ERROR|error|Error|FATAL|panic|failed)"
else
    echo "Gateway日志文件不存在"
fi

# 5. 测试Counter服务gRPC连接
echo -e "\n5. 测试Counter服务连接..."
if command -v grpcurl >/dev/null 2>&1; then
    echo "使用grpcurl测试Counter服务..."
    timeout 5s grpcurl -plaintext localhost:9001 list || echo "grpcurl测试失败"
else
    echo "grpcurl未安装，使用netcat测试端口..."
    timeout 2s nc -z localhost 9001 && echo "✅ Counter端口9001可达" || echo "❌ Counter端口9001不可达"
fi

# 6. 测试Gateway健康检查
echo -e "\n6. 测试Gateway健康检查..."
timeout 5s curl -s http://localhost:8080/api/v1/health || echo "Gateway健康检查失败"

# 7. 测试连接池状态API
echo -e "\n7. 测试连接池状态..."
timeout 5s curl -s http://localhost:8080/api/v1/system/grpc-pools || echo "连接池状态API失败"

# 8. 测试Redis连接
echo -e "\n8. 检查Redis连接..."
if command -v redis-cli >/dev/null 2>&1; then
    redis-cli ping 2>/dev/null || echo "Redis连接失败"
else
    echo "redis-cli未安装，跳过Redis测试"
fi

# 9. 分析数据一致性问题
echo -e "\n9. 分析数据一致性..."
echo "期望写入: 86,000"
echo "实际结果: 96,000"
echo "差异: +10,000 (可能的重复请求或重试)"

# 10. 性能瓶颈分析
echo -e "\n10. 性能对比分析..."
echo "Phase 1 基准: ~21,000 QPS"
echo "Week 4 实测: ~750 QPS"
echo "性能损失: 96.4% (严重性能退化)"
echo ""
echo "可能原因:"
echo "  - gRPC服务间通信开销"
echo "  - Counter服务处理能力不足"  
echo "  - Redis连接池问题"
echo "  - 网络延迟累积"
echo "  - 错误重试机制影响"

echo -e "\n=== 分析完成 ===" 