# HighGoPress: 高并发实时计数服务技术方案

## 1. 项目概述

### 1.1 项目目标

**HighGoPress** 是一个专为展示现代后端技术栈而设计的高并发实时计数服务。项目旨在模拟社交媒体中的点赞、关注、访问量统计等高频写入场景，构建一个支持**5万QPS**以上、具备高可用性和可观测性的微服务系统。

**核心目的**：作为求职者的技术亮点项目，系统性地展示在Go语言、微服务架构、中间件应用、性能优化和系统监控方面的综合能力。

### 1.2 业务场景

- **实时点赞**：用户对文章、评论等内容进行点赞，系统实时更新计数值。
- **热点排行**：实时计算并展示点赞数最高的文章排行榜。
- **用户关注计数**：实时增减用户关注数和粉丝数。
- **API访问统计**：对网关的API调用进行实时计数。

---

## 2. 核心技术特性 (面试亮点)

- **高性能并发模型**:
  - 采用 **Worker Pool（Goroutine池）** + **Channel** 模型，精细化控制并发粒度，防止goroutine泄露。
  - 大量使用 **原子操作 (atomic)** 处理核心计数字段，避免锁竞争。
  - 通过 **`sync.Pool`** 复用高频创建的对象（如API响应体），降低GC压力。

- **微服务架构**:
  - 采用 **gRPC + Protobuf** 作为服务间高效的二进制通信协议。
  - 使用 **Consul** 实现服务自动注册与发现，提升系统动态伸缩能力。
  - **Gateway层**作为统一流量入口，实现路由、认证、**令牌桶限流**等策略。

- **高可用中间件栈**:
  - **Redis**: 作为主缓存层，利用 **Pipeline** 批量操作和**本地缓存(Local Cache)**组合，解决热点Key问题，承载核心读写流量。
  - **Kafka**: 作为异步消息总线，实现**削峰填谷**、**数据持久化**和**事件驱动**，将核心写操作与非核心流程解耦，极大降低API响应延迟。

- **全链路可观测性**:
  - **Prometheus** 作为指标存储与查询引擎，对服务关键性能指标（QPS, Latency, Error Rate）进行全面采集。
  - **Grafana** 实现指标的可视化监控大盘，实时洞察系统健康状况。
  - （可选）**Jaeger/OpenTelemetry** 实现分布式链路追踪，快速定位微服务调用链中的瓶颈。

---

## 3. 系统架构设计

### 3.1 整体架构图

```mermaid
graph TD
    subgraph "用户端"
        Client[客户端]
    end

    subgraph "流量接入层"
        Gateway[API Gateway (Gin)<br/>- 路由分发<br/>- 令牌桶限流<br/>- 认证鉴权]
    end

    subgraph "核心微服务 (gRPC)"
        Counter[Counter Service<br/>核心计数服务<br/>(Go, gRPC)]
        Analytics[Analytics Service<br/>统计分析服务<br/>(Go, gRPC)]
        User[User Service<br/>用户服务(可选)<br/>(Go, gRPC)]
    end

    subgraph "共享中间件"
        Redis[(Redis<br/>- 实时计数缓存<br/>- Pipeline优化)]
        Kafka[(Kafka<br/>- 异步持久化<br/>- 削峰填谷)]
        Consul[(Consul<br/>服务注册与发现)]
        Prometheus[(Prometheus<br/>指标监控)]
    end
    
    subgraph "数据持久化与监控"
        MySQL[(MySQL<br/>数据落盘)]
        Grafana[Grafana<br/>可视化大盘]
    end

    Client --> Gateway
    Gateway -- gRPC --> Counter
    Gateway -- gRPC --> Analytics
    Gateway -- gRPC --> User

    Counter -- "读写" --> Redis
    Counter -- "异步生产消息" --> Kafka

    Analytics -- "消费消息/查询" --> Kafka
    Analytics -- "读" --> MySQL
    
    Kafka_Consumer[Kafka Consumer<br/>(独立部署)] -- "消费消息" --> Kafka
    Kafka_Consumer -- "批量写入" --> MySQL

    Counter -- "注册" --> Consul
    Analytics -- "注册" --> Consul
    User -- "注册" --> Consul
    Gateway -- "发现服务" --> Consul
    
    Prometheus -- "采集Metrics" --> Gateway
    Prometheus -- "采集Metrics" --> Counter
    Prometheus -- "采集Metrics" --> Analytics
    Grafana -- "查询" --> Prometheus

```

### 3.2 数据流: 点赞操作

1.  **同步路径 (5ms内完成)**:
    - `Client` 发送点赞请求到 `Gateway`。
    - `Gateway` 进行限流检查，通过后将请求路由到 `Counter Service`。
    - `Counter Service` 调用 `Redis` 的 `INCR` 命令，原子性地增加计数值。
    - `Redis` 返回最新计数值。
    - `Counter Service` 立即将成功和最新计数值返回给 `Gateway`，并最终响应给 `Client`。

2.  **异步路径 (后台执行)**:
    - 在 `Redis` 操作成功后，`Counter Service` **异步地**向 `Kafka` 的 `user-actions` Topic 发送一条消息（包含用户ID、文章ID、操作类型等）。
    - 独立部署的 `Kafka Consumer` 订阅该Topic，批量拉取消息。
    - `Consumer` 将聚合后的数据批量写入 `MySQL` 进行持久化存储。
    - `Analytics Service` 也可订阅此Topic，进行实时热点分析等复杂计算。

---

## 4. 开发与测试方案

### 4.1 开发路线图 (6周计划)

- **Phase 1: 核心高并发单体 (2周)**
  - **Week 1**: 搭建基础HTTP服务(Gin)，实现核心计数API，集成Redis，完成基础压测（目标1万QPS）。
  - **Week 2**: 引入Worker Pool和`sync.Pool`进行性能优化，使用pprof进行瓶颈分析，集成Kafka实现异步落盘。

- **Phase 2: 微服务拆分 (2周)**
  - **Week 3**: 定义Protobuf接口，将单体拆分为`Gateway`, `Counter`, `Analytics`三个微服务，实现gRPC通信。
  - **Week 4**: 集成Consul实现服务发现，统一配置中心，完善服务间调用的错误处理和重试机制。

- **Phase 3: 生产级特性 (2周)**
  - **Week 5**: 集成Prometheus和Grafana，为关键服务和业务逻辑添加Metrics监控，搭建可视化大盘。
  - **Week 6**: 进行大规模联合压测（目标5万QPS），识别并修复性能瓶颈，编写项目文档和面试材料。

### 4.2 测试策略

- **单元测试**: 对每个服务内部的核心函数、算法进行覆盖。
- **集成测试**: 测试服务与中间件（Redis, Kafka）的交互是否正确。
- **端到端(E2E)测试**: 编写脚本模拟从`Gateway`到后端服务的完整调用链路。
- **性能/压力测试**:
  - **工具**: `hey`, `k6`, `wrk`。
  - **指标**: 重点关注 **QPS**, **P95/P99延迟**, **错误率**, **CPU/内存使用率**。
  - **目标**: 验证系统在不同压力下的性能表现，找出并解决瓶颈。

---

## 5. 目录结构 (建议)

```
/high-go-press
├── api/                  # Protobuf 定义文件 (.proto)
├── cmd/                  # 各服务main函数入口
│   ├── gateway/
│   ├── counter/
│   └── analytics/
├── configs/              # 配置文件 (yaml)
├── deploy/               # 部署相关 (docker-compose.yml)
├── internal/             # 内部代码，不对外暴露
│   ├── dao/              # 数据访问层 (redis, mysql)
│   ├── service/          # 业务逻辑实现
│   └── biz/              # 业务实体和接口定义
├── pkg/                  # 项目内共享的公共库 (logger, config, mq)
├── scripts/              # 辅助脚本 (压测, 构建)
├── HighGoPress_Tech_Doc.md # 本技术文档
└── go.mod
``` 