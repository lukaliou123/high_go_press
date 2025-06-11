# 📋 HighGoPress Phase 2 微服务迁移方案

## 🎯 目标概述

将现有的高性能单体应用拆分为微服务架构，保持现有性能优势，增强系统的可扩展性和可维护性。

**核心原则**: 
- 🔒 **零影响迁移**: 保持现有功能和性能
- 🧪 **充分验证**: 每步都有完整的测试覆盖
- 🔄 **可回滚**: 每个阶段都能安全回退

---

## 📅 迁移时间线

### **Week 3: 核心微服务拆分 (7天)**
### **Week 4: 服务发现与生产特性 (7天)**

---

## 🏗️ Week 3 详细迁移计划

### **Day 1: 基础设施搭建**

#### 📋 任务清单
```bash
□ 1.1 创建微服务目录结构
□ 1.2 设计 Protobuf 接口定义
□ 1.3 配置代码生成工具链
□ 1.4 创建基础配置文件
```

#### 🔧 具体实施

**1.1 目录结构创建**
```bash
# 创建新目录结构
mkdir -p api/proto/{counter,analytics,common}
mkdir -p api/generated/{counter,analytics}
mkdir -p cmd/{counter,analytics}
mkdir -p internal/{gateway,counter,analytics}
mkdir -p pkg/{consul,grpc/{client,server,interceptor}}
mkdir -p configs
mkdir -p deploy/consul
```

**1.2 Protobuf接口设计**
```protobuf
// api/proto/counter/counter.proto
syntax = "proto3";
package counter;
option go_package = "high-go-press/api/generated/counter";

service CounterService {
  rpc IncrementCounter(IncrementRequest) returns (IncrementResponse);
  rpc GetCounter(GetCounterRequest) returns (GetCounterResponse);
  rpc BatchGetCounters(BatchGetRequest) returns (BatchGetResponse);
}

message IncrementRequest {
  string resource_id = 1;
  string counter_type = 2;
  int64 delta = 3;
}

message IncrementResponse {
  bool success = 1;
  int64 current_value = 2;
  string message = 3;
}
```

**1.3 代码生成脚本**
```bash
# scripts/generate_proto.sh
#!/bin/bash
protoc --go_out=. --go-grpc_out=. api/proto/**/*.proto
```

#### 🧪 Day 1 测试
```bash
# 验证目录结构
□ tree 命令检查目录完整性
□ protoc 代码生成测试
□ go mod tidy 依赖检查
```

---

### **Day 2: Counter微服务核心实现**

#### 📋 任务清单
```bash
□ 2.1 实现 Counter gRPC 服务端
□ 2.2 迁移现有业务逻辑代码
□ 2.3 保持 Worker Pool 和 sync.Pool 优化
□ 2.4 实现基础配置管理
```

#### 🔧 具体实施

**2.1 gRPC服务实现**
```go
// cmd/counter/main.go
package main

import (
    "context"
    "net"
    "google.golang.org/grpc"
    pb "high-go-press/api/generated/counter"
    "high-go-press/internal/counter/server"
)

func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }
    
    s := grpc.NewServer()
    pb.RegisterCounterServiceServer(s, server.NewCounterServer())
    
    log.Printf("Counter service listening at %v", lis.Addr())
    if err := s.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
```

**2.2 业务逻辑迁移**
```go
// internal/counter/server/counter_server.go
// 将现有 internal/service/counter.go 的逻辑迁移到这里
// 保持所有性能优化：Worker Pool, sync.Pool, atomic操作等
```

#### 🧪 Day 2 测试
```bash
# 单独启动 Counter 服务测试
□ go run cmd/counter/main.go
□ grpcurl 工具测试基础gRPC调用
□ 验证Redis连接和Kafka集成
□ 确认Worker Pool正常工作
```

---

### **Day 3: Gateway改造实现**

#### 📋 任务清单
```bash
□ 3.1 创建 gRPC 客户端封装
□ 3.2 改造现有 HTTP Handler
□ 3.3 实现 HTTP to gRPC 转换
□ 3.4 保持现有API接口不变
```

#### 🔧 具体实施

**3.1 gRPC客户端**
```go
// internal/gateway/client/counter_client.go
package client

import (
    "context"
    "google.golang.org/grpc"
    pb "high-go-press/api/generated/counter"
)

type CounterClient struct {
    conn   *grpc.ClientConn
    client pb.CounterServiceClient
}

func NewCounterClient(addr string) (*CounterClient, error) {
    conn, err := grpc.Dial(addr, grpc.WithInsecure())
    if err != nil {
        return nil, err
    }
    
    return &CounterClient{
        conn:   conn,
        client: pb.NewCounterServiceClient(conn),
    }, nil
}
```

**3.2 Handler改造**
```go
// 改造现有的 cmd/gateway/handlers/counter.go
// HTTP请求 → gRPC调用 → HTTP响应
func (h *CounterHandler) IncrementCounter(c *gin.Context) {
    var req models.IncrementCounterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // 调用gRPC服务
    grpcReq := &pb.IncrementRequest{
        ResourceId:  req.ResourceID,
        CounterType: req.CounterType,
        Delta:       req.Delta,
    }
    
    resp, err := h.counterClient.IncrementCounter(context.Background(), grpcReq)
    // 处理响应...
}
```

#### 🧪 Day 3 测试
```bash
# Gateway + Counter 联合测试
□ 启动 Counter 服务: go run cmd/counter/main.go
□ 启动 Gateway 服务: go run cmd/gateway/main.go  
□ HTTP API 功能测试: curl 所有现有接口
□ 确认响应格式与Phase 1完全一致
```

---

### **Day 4: Analytics微服务搭建**

#### 📋 任务清单
```bash
□ 4.1 定义 Analytics Protobuf 接口
□ 4.2 实现基础 Analytics 服务框架
□ 4.3 集成 Kafka 消息消费
□ 4.4 实现基础统计功能
```

#### 🔧 具体实施

**4.1 Analytics Proto定义**
```protobuf
// api/proto/analytics/analytics.proto
service AnalyticsService {
  rpc GetTopCounters(TopCountersRequest) returns (TopCountersResponse);
  rpc GetCounterStats(StatsRequest) returns (StatsResponse);
}
```

**4.2 服务实现**
```go
// cmd/analytics/main.go
// internal/analytics/server/analytics_server.go
// 基础框架，支持统计查询
```

#### 🧪 Day 4 测试
```bash
# Analytics 服务独立测试
□ 启动 Analytics 服务
□ gRPC 接口基础调用测试
□ Kafka 消息消费验证
□ 基础统计功能验证
```

---

### **Day 5-6: 集成测试与性能验证**

#### 📋 任务清单
```bash
□ 5.1 三服务联合启动测试
□ 5.2 端到端功能验证
□ 5.3 性能对比测试
□ 5.4 数据一致性验证
```

#### 🧪 Day 5-6 测试

**5.1 多服务启动**
```bash
# Terminal 1: Counter Service
go run cmd/counter/main.go

# Terminal 2: Analytics Service  
go run cmd/analytics/main.go

# Terminal 3: Gateway Service
go run cmd/gateway/main.go
```

**5.2 功能验证**
```bash
# 使用现有测试脚本验证
□ ./scripts/quick_test.sh
□ ./scripts/load_test.sh
□ 确保所有API返回格式一致
□ 验证计数器数据一致性
```

**5.3 性能对比**
```bash
# 对比Phase 1性能
□ 运行 ./scripts/performance_test.sh
□ 记录关键指标：QPS, P99延迟, 错误率
□ 性能下降应控制在 10% 以内
```

#### 📊 性能基准对比

| 测试级别 | Phase 1 (单体) | Phase 2 (微服务) | 差异 |
|---------|----------------|------------------|------|
| Low Load | 18,433 QPS | Target: >16,500 QPS | <10% |
| High Load | 23,738 QPS | Target: >21,000 QPS | <12% |
| P99 延迟 | 21.6ms | Target: <30ms | +40% |

---

### **Day 7: 问题修复与优化**

#### 📋 任务清单
```bash
□ 7.1 修复集成测试发现的问题
□ 7.2 性能调优
□ 7.3 错误处理完善
□ 7.4 日志和监控适配
```

#### 🔧 可能的优化点
- gRPC连接池优化
- 序列化/反序列化优化
- 网络调用超时配置
- 错误重试机制

---

## 🏗️ Week 4 详细计划

### **Day 8-10: Consul服务发现**

#### 📋 任务清单
```bash
□ 8.1 集成 Consul 客户端
□ 8.2 实现服务自动注册
□ 8.3 实现服务发现机制
□ 8.4 配置健康检查
```

#### 🧪 测试
```bash
□ Consul UI 验证服务注册
□ 服务发现功能测试
□ 故障转移测试
□ 健康检查验证
```

### **Day 11-12: 配置中心与错误处理**

#### 📋 任务清单
```bash
□ 11.1 统一配置管理
□ 11.2 实现重试机制
□ 11.3 添加熔断器
□ 11.4 超时控制
```

### **Day 13-14: 监控与部署**

#### 📋 任务清单
```bash
□ 13.1 适配 Prometheus 监控
□ 13.2 创建 Docker Compose 配置
□ 13.3 端到端压测
□ 13.4 文档更新
```

---

## 🧪 测试策略

### **每日测试检查点**

**功能测试**
```bash
# 基础API测试
curl -X POST localhost:8080/api/v1/counter/increment \
  -H "Content-Type: application/json" \
  -d '{"resource_id":"test","counter_type":"like","delta":1}'

# 健康检查
curl localhost:8080/api/v1/health
```

**性能测试**
```bash
# 快速性能验证
./scripts/quick_test.sh

# 每3天运行完整性能测试
./scripts/performance_test.sh
```

**数据一致性测试**
```bash
# 验证计数器数据
# 验证Kafka消息
# 验证Redis数据
```

### **回滚策略**

每个Day结束时的回滚点：
- **Day 1-2**: 回滚到Phase 1代码
- **Day 3+**: 切换到HTTP直接调用模式
- **应急措施**: 保持原有单体服务作为backup

---

## 📊 风险控制

### **高风险项**
1. **性能下降**: 目标控制在10%以内
2. **数据一致性**: Redis/Kafka数据同步
3. **服务稳定性**: gRPC连接稳定性

### **缓解措施**
1. **性能监控**: 实时监控关键指标
2. **灰度发布**: 小流量验证
3. **快速回滚**: 保持完整回滚方案

---

## ✅ 验收标准

### **功能完整性**
- [ ] 所有Phase 1 API正常工作
- [ ] 响应格式100%兼容
- [ ] 数据一致性验证通过

### **性能标准**
- [ ] QPS下降 < 10%
- [ ] P99延迟增加 < 50%
- [ ] 错误率 = 0%

### **架构质量**
- [ ] 服务间解耦合理
- [ ] 配置管理完善
- [ ] 监控体系完整

---

这个迁移方案提供了详细的每日任务清单和测试策略，确保我们能够安全、高效地完成微服务迁移。每个步骤都有明确的验收标准和回滚计划。

您觉得这个计划如何？有什么需要调整或补充的地方吗？ 