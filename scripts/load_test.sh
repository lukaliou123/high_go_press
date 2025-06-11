#!/bin/bash

# 简单的负载测试脚本
# 使用hey工具进行压测

echo "=== HighGoPress Load Testing ==="

# 检查hey是否安装
if ! command -v hey &> /dev/null; then
    echo "Installing hey tool..."
    go install github.com/rakyll/hey@latest
fi

# 服务地址
HOST="http://localhost:8080"

echo "1. 测试健康检查接口 - 10000 请求, 100 并发"
hey -n 10000 -c 100 $HOST/health

echo ""
echo "2. 测试计数器查询接口 - 5000 请求, 50 并发"
hey -n 5000 -c 50 $HOST/api/v1/counter/article_001/like

echo ""
echo "3. 测试计数器增量接口 - 3000 请求, 30 并发"
hey -n 3000 -c 30 -m POST \
    -H "Content-Type: application/json" \
    -d '{"resource_id": "article_loadtest", "counter_type": "like", "user_id": "user_loadtest", "increment": 1}' \
    $HOST/api/v1/counter/increment

echo ""
echo "4. 查看最终计数结果"
curl -s $HOST/api/v1/counter/article_loadtest/like

echo ""
echo "=== Load Testing Complete ===" 