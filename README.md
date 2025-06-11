# HighGoPress - 高并发实时计数服务

基于Go语言的高性能、高并发实时计数微服务系统。专为展示现代后端技术栈设计，支持万级QPS的点赞、访问、关注等计数场景。

## ⚡ 快速开始

### 环境要求
- Go 1.22+
- Redis 6.0+

### 启动服务

1. **克隆项目**
```bash
git clone <repository>
cd high-go-press
```

2. **安装依赖**
```bash
go mod tidy
```

3. **启动Redis**
```bash
sudo systemctl start redis-server
```

4. **构建并运行**
```bash
go build -o bin/gateway ./cmd/gateway
./bin/gateway
```

### API 测试

```bash
# 健康检查
curl http://localhost:8080/health

# 增加计数
curl -X POST http://localhost:8080/api/v1/counter/increment \
  -H "Content-Type: application/json" \
  -d '{"resource_id": "article_001", "counter_type": "like", "user_id": "user_123", "increment": 1}'

# 查询计数
curl http://localhost:8080/api/v1/counter/article_001/like

# 批量查询
curl -X POST http://localhost:8080/api/v1/counter/batch \
  -H "Content-Type: application/json" \
  -d '{"queries": [{"resource_id": "article_001", "counter_type": "like"}]}'
```

## 🎯 核心特性

### 高性能并发
- **Worker Pool模式**: 精细控制Goroutine数量，防止资源泄露
- **原子操作**: 基于`atomic`包的无锁计数，避免锁竞争
- **Redis Pipeline**: 批量操作优化，提升吞吐量
- **连接池**: 复用Redis连接，降低连接开销

### 架构设计
- **分层架构**: biz -> service -> dao 清晰分层
- **依赖注入**: 接口导向的可测试设计
- **配置管理**: 支持YAML配置和环境变量
- **结构化日志**: 基于zap的高性能日志

### 业务功能
- **多种计数类型**: 点赞(like)、浏览(view)、关注(follow)
- **批量操作**: 支持批量查询，减少网络往返
- **原子性保证**: Redis INCR确保计数准确性
- **热点排行**: (TODO) 基于ZSET的实时排行榜

## 📊 性能指标

基于本地测试环境的压测结果：

| 操作类型 | QPS | 平均延迟 | P95延迟 |
|---------|-----|----------|---------|
| 健康检查 | 22,356 | 0.4ms | 1.2ms |
| 计数查询 | ~15,000 | 0.6ms | 1.5ms |
| 计数增量 | 10,406 | 1.9ms | 3.8ms |

## 🏗️ 系统架构

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │───▶│   Gateway   │───▶│   Service   │
└─────────────┘    │   (Gin)     │    │  (Counter)  │
                   └─────────────┘    └─────────────┘
                           │                   │
                           ▼                   ▼
                   ┌─────────────┐    ┌─────────────┐
                   │    Redis    │    │   Logger    │
                   │  (Caching)  │    │   (Zap)     │
                   └─────────────┘    └─────────────┘
```

## 🔧 配置说明

配置文件：`configs/config.yaml`

```yaml
server:
  host: "0.0.0.0"
  port: 8080

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

log:
  level: "info"
  format: "json"
```

## 📈 压力测试

运行压力测试：

```bash
# 安装hey工具
go install github.com/rakyll/hey@latest

# 运行压测脚本
./scripts/load_test.sh
```

## 🚀 开发计划

### Phase 1: 核心功能 ✅
- [x] 基础HTTP服务(Gin)
- [x] Redis计数器实现
- [x] 基础API (increment, get, batch)
- [x] 性能压测 (目标: 1万QPS) ✅ **达成10k+ QPS**

### Phase 2: 微服务架构 (计划中)
- [ ] gRPC服务拆分
- [ ] Consul服务发现
- [ ] Kafka异步消息
- [ ] API Gateway

### Phase 3: 生产特性 (计划中)
- [ ] Prometheus监控
- [ ] Grafana可视化
- [ ] 热点排行榜
- [ ] 压测优化 (目标: 5万QPS)

## 🛠️ 技术栈

- **语言**: Go 1.22
- **Web框架**: Gin
- **缓存**: Redis
- **配置**: Viper
- **日志**: Zap
- **压测**: hey

## 📝 API 文档

### 计数器增量
```
POST /api/v1/counter/increment
Content-Type: application/json

{
  "resource_id": "article_001",
  "counter_type": "like",
  "user_id": "user_123", 
  "increment": 1
}
```

### 查询计数
```
GET /api/v1/counter/{resource_id}/{counter_type}
```

### 批量查询
```
POST /api/v1/counter/batch
Content-Type: application/json

{
  "queries": [
    {"resource_id": "article_001", "counter_type": "like"},
    {"resource_id": "article_002", "counter_type": "like"}
  ]
}
```

## 🤝 贡献

欢迎提交Issue和Pull Request！

## �� 许可证

MIT License 