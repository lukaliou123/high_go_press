## 🚀 项目进展

### ✅ Phase 1: 高性能单体架构 (已完成)
- [x] 基础HTTP服务(Gin) + 分层架构
- [x] Redis计数器实现 + Pipeline优化
- [x] Worker Pool + sync.Pool 对象复用
- [x] 完整API (increment, get, batch)
- [x] pprof性能分析 + 可观测性
- [x] **目标达成**: 25k+ QPS @ 极限并发

### 🚧 Phase 2: 微服务拆分 (进行中 - Week 4)
- [x] gRPC服务拆分 + Protocol Buffers定义
- [x] gRPC连接池实现 (20连接 + Keep-Alive)
- [x] Gateway -> Counter Service 通信
- [ ] Consul服务发现 + 健康检查
- [ ] Kafka异步消息 + 事件驱动
- [ ] 服务监控 + 熔断保护

**当前重点**: gRPC连接池优化 → 预期写入QPS提升至3,000+

### 📋 Phase 3: 生产级特性 (计划中)
- [ ] Prometheus + Grafana监控栈
- [ ] 分布式链路追踪 (Jaeger)
- [ ] 热点排行榜 + 缓存策略
- [ ] 容器化部署 (Docker + K8s)
- [ ] **终极目标**: 微服务性能超越单体 (30k+ QPS)

## 🔧 配置说明

### Phase 1配置：`configs/config.yaml`
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

### Phase 2新增配置：
```yaml
grpc:
  counter_service:
    host: "localhost:50051"
    pool_size: 20
    keep_alive: 30s

consul:
  address: "localhost:8500"
  health_interval: "10s"

kafka:
  brokers: ["localhost:9092"]
  topic: "counter_events"
```

## 📈 压力测试方法

基于科学的渐进式压测方法：

```bash
# 安装测试工具
go install github.com/rakyll/hey@latest

# 运行完整的5级压测
./scripts/load_test.sh

# 性能分析
go tool pprof http://localhost:8080/debug/pprof/profile
go tool pprof http://localhost:8080/debug/pprof/heap
```

### 测试级别说明
- **Level 1**: 1k请求/10并发 (基础验证)
- **Level 2**: 5k请求/50并发 (中等负载)
- **Level 3**: 10k请求/100并发 (高负载)
- **Level 4**: 20k请求/200并发 (极高负载)
- **Level 5**: 50k请求/500并发 (极限测试)

## 🛠️ 技术栈

### Phase 1 技术栈
- **语言**: Go 1.22
- **Web框架**: Gin
- **缓存**: Redis
- **配置**: Viper
- **日志**: Zap
- **压测**: hey
- **性能分析**: pprof

### Phase 2 新增技术
- **RPC框架**: gRPC + Protocol Buffers
- **服务发现**: Consul
- **消息队列**: Kafka
- **负载均衡**: Round-Robin连接池
- **可观测性**: 自定义指标 + 健康检查

## 💡 核心技术亮点

### 1. 高并发处理能力
- **极限并发**: 500并发仍保持21k+ QPS
- **延迟控制**: 高负载下P99 < 50ms  
- **稳定性**: 0错误率，数据一致性100%

### 2. 性能工程实践
- **科学测试**: 五级渐进式压测方法
- **全链路监控**: 从应用到系统层面
- **深度分析**: pprof + 自定义监控指标

### 3. 生产级架构
- **弹性设计**: Worker Pool自适应扩缩容
- **内存优化**: sync.Pool零GC优化
- **可观测性**: 完整的性能监控体系

## 📝 API 文档

### 计数器增量
```http
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
```http
GET /api/v1/counter/{resource_id}/{counter_type}
```

### 批量查询
```http
POST /api/v1/counter/batch
Content-Type: application/json

{
  "queries": [
    {"resource_id": "article_001", "counter_type": "like"},
    {"resource_id": "article_002", "counter_type": "like"}
  ]
}
```

### 健康检查
```http
GET /health
GET /metrics  # Phase 2: 详细指标
```

## 🤝 贡献

欢迎提交Issue和Pull Request！

## 📄 许可证

MIT License