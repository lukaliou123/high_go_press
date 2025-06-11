# Phase 2 微服务架构目录结构

## 🎯 目标架构
```
/high-go-press/
├── api/                          # 📋 gRPC & Protobuf定义
│   ├── proto/
│   │   ├── counter/
│   │   │   └── counter.proto     # 计数服务接口
│   │   ├── analytics/
│   │   │   └── analytics.proto   # 分析服务接口
│   │   └── common/
│   │       └── types.proto       # 通用类型定义
│   └── generated/                # 生成的gRPC代码
│       ├── counter/
│       └── analytics/

├── cmd/                          # 🚀 各微服务入口
│   ├── gateway/                  # API网关 (现有,需改造)
│   │   └── main.go
│   ├── counter/                  # 计数服务 (新增)
│   │   └── main.go
│   └── analytics/                # 分析服务 (新增)
│       └── main.go

├── internal/                     # 🔒 内部业务逻辑
│   ├── gateway/                  # 网关相关代码
│   │   ├── handlers/             # HTTP处理器
│   │   ├── middleware/           # 中间件
│   │   └── client/               # gRPC客户端
│   ├── counter/                  # 计数服务业务逻辑
│   │   ├── service/              # 从现有代码迁移
│   │   ├── dao/                  # 数据访问层
│   │   └── server/               # gRPC服务实现
│   └── analytics/                # 分析服务业务逻辑
│       ├── service/
│       ├── dao/
│       └── server/

├── pkg/                          # 📦 共享组件库 (基本保持不变)
│   ├── kafka/
│   ├── pool/
│   ├── logger/
│   ├── config/
│   ├── pprof/
│   ├── consul/                   # 新增: 服务发现
│   └── grpc/                     # 新增: gRPC通用组件
│       ├── client/
│       ├── server/
│       └── interceptor/

├── configs/                      # ⚙️ 配置文件
│   ├── gateway.yaml
│   ├── counter.yaml
│   ├── analytics.yaml
│   └── consul.yaml

├── deploy/                       # 🐳 部署配置
│   ├── docker-compose.yml        # 多服务编排
│   └── consul/
│       └── config.json

└── scripts/                      # 🔧 工具脚本
    ├── generate_proto.sh         # 生成Protobuf代码
    ├── build_all.sh             # 构建所有服务
    └── performance_test_v2.sh   # 微服务版本压测
```

## 🔄 迁移路径

### Week 3 迁移计划

#### Day 1-2: Protobuf定义 & 代码生成
- [ ] 定义counter.proto接口
- [ ] 定义analytics.proto接口
- [ ] 生成gRPC代码
- [ ] 搭建基础框架

#### Day 3-4: 核心服务拆分
- [ ] 将CounterService拆分为独立的gRPC服务
- [ ] 创建Analytics服务框架
- [ ] 实现服务间通信

#### Day 5-7: Gateway改造 & 集成测试
- [ ] Gateway HTTP→gRPC转换
- [ ] 端到端测试
- [ ] 性能对比验证

### Week 4: 服务发现 & 生产特性
- [ ] 集成Consul
- [ ] 配置中心
- [ ] 错误处理 & 重试
- [ ] 监控适配

## 📊 风险评估

### 低风险 ✅
- pkg/目录下的组件基本不变
- 核心业务逻辑可以直接复用
- 现有的性能优化保持不变

### 中风险 ⚠️
- HTTP→gRPC转换可能引入延迟
- 服务拆分后的数据一致性
- 部署复杂度增加

### 缓解策略 🛡️
- 保持现有HTTP接口作为兼容层
- 渐进式迁移，逐步切流量
- 完整的测试覆盖 