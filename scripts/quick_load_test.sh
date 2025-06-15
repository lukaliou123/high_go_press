#!/bin/bash

# HighGoPress 快速负载测试脚本
#
# 该脚本使用 'hey' 工具对 Gateway 服务进行快速的负载测试，
# 以便在 Prometheus 和 Grafana 中生成可供观察的指标数据。
#
# 使用方法: ./scripts/quick_load_test.sh

# --- 配置 ---
TARGET_URL="http://localhost:8080/api/v1/counter/increment"
DURATION="60s"  # 测试持续时间
CONCURRENCY=50 # 并发用户数
TOTAL_REQUESTS=1000000 # 总请求数

# 颜色定义
BLUE='\033[0;34m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# 检查 hey 是否安装
if ! command -v hey &> /dev/null
then
    echo -e "${BLUE}hey 未安装。正在尝试安装...${NC}"
    # 尝试为 Debian/Ubuntu 安装
    if command -v apt-get &> /dev/null; then
        sudo apt-get update && sudo apt-get install -y hey
    # 尝试为 MacOS 安装
    elif command -v brew &> /dev/null; then
        brew install hey
    else
        echo "无法自动安装 hey。请手动安装后重试: https://github.com/rakyll/hey"
        exit 1
    fi
fi

echo -e "${BLUE}🚀 开始对 HighGoPress Gateway 进行负载测试...${NC}"
echo "----------------------------------------------------"
echo "  Target URL:   $TARGET_URL"
echo "  Duration:     $DURATION"
echo "  Concurrency:  $CONCURRENCY"
echo "----------------------------------------------------"

# 执行压测
hey -z $DURATION -c $CONCURRENCY -n $TOTAL_REQUESTS \
  -m POST -H "Content-Type: application/json" \
  -d '{"resource_id":"test_resource_123","counter_type":"load_test_like","delta":1}' \
  $TARGET_URL

echo ""
echo -e "${GREEN}✅ 负载测试完成!${NC}"
echo "现在你可以在 Prometheus 和 Grafana 中查看生成的指标数据了。"
echo "  -> Prometheus: http://localhost:9090"
echo "  -> Grafana:    http://localhost:3000" 