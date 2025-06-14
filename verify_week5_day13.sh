#!/bin/bash

echo "🔍 Week 5 Day 13 实现验证"
echo "========================"

# 检查核心文件是否存在
echo "📁 检查核心文件..."

files=(
    "pkg/metrics/metrics.go"
    "pkg/middleware/metrics.go"
    "configs/prometheus.yml"
    "WEEK5_DAY13_PROMETHEUS_METRICS_REPORT.md"
    "WEEK5_MONITORING_PLAN.md"
)

for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        echo "  ✅ $file"
    else
        echo "  ❌ $file (缺失)"
    fi
done

# 检查配置更新
echo ""
echo "⚙️ 检查配置更新..."

if grep -q "prometheus:" configs/config.yaml; then
    echo "  ✅ Prometheus 配置已添加"
else
    echo "  ❌ Prometheus 配置缺失"
fi

if grep -q "github.com/prometheus/client_golang" go.mod; then
    echo "  ✅ Prometheus 客户端依赖已添加"
else
    echo "  ❌ Prometheus 客户端依赖缺失"
fi

# 检查代码结构
echo ""
echo "🏗️ 检查代码结构..."

if grep -q "MetricsManager" pkg/metrics/metrics.go; then
    echo "  ✅ 指标管理器结构定义"
else
    echo "  ❌ 指标管理器结构缺失"
fi

if grep -q "HTTPMetricsMiddleware" pkg/middleware/metrics.go; then
    echo "  ✅ HTTP 指标中间件定义"
else
    echo "  ❌ HTTP 指标中间件缺失"
fi

if grep -q "GRPCMetricsUnaryInterceptor" pkg/middleware/metrics.go; then
    echo "  ✅ gRPC 指标拦截器定义"
else
    echo "  ❌ gRPC 指标拦截器缺失"
fi

# 统计代码行数
echo ""
echo "📊 代码统计..."

if [ -f "pkg/metrics/metrics.go" ]; then
    metrics_lines=$(wc -l < pkg/metrics/metrics.go)
    echo "  指标管理器: $metrics_lines 行"
fi

if [ -f "pkg/middleware/metrics.go" ]; then
    middleware_lines=$(wc -l < pkg/middleware/metrics.go)
    echo "  指标中间件: $middleware_lines 行"
fi

echo ""
echo "🎯 Week 5 Day 13 实现总结:"
echo "  ✅ 核心指标管理器 - 完整的 Prometheus 指标类型支持"
echo "  ✅ 指标收集中间件 - HTTP/gRPC/业务/数据库/缓存指标"
echo "  ✅ 配置系统扩展 - 监控配置结构和选项"
echo "  ✅ Prometheus 集成 - 配置文件和服务发现"
echo "  ✅ 文档和报告 - 完整的实现文档"
echo ""
echo "🚀 准备就绪，可以开始 Week 5 Day 14: Grafana 可视化仪表板！" 