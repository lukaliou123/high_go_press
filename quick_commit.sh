#!/bin/bash

echo "🧹 清理大文件..."
rm -f kafka_2.13-3.9.1.tgz
rm -rf kafka_2.13-3.9.1/
rm -f main

echo "📦 提交 Week 4 Day 12 错误处理版本..."
git add .
git commit -m "feat: Week 4 Day 12 - 错误处理和重试机制

✨ 实现功能:
- 熔断器机制 (Circuit Breaker)
- 智能重试策略 (Exponential Backoff)
- 服务降级机制 (Fallback)
- 错误处理中间件
- 弹性管理器

🔧 技术特性:
- 三状态熔断器
- 指数退避算法
- 多层级降级
- 统一错误处理
- 配置热更新

📝 更新 .gitignore 忽略大文件"

echo "✅ 提交完成！" 