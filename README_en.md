# HighGoPress: From High-Performance Monolith to Production-Grade Microservices

HighGoPress is a backend project demonstrating the complete architectural evolution from a high-performance Go monolith to a resilient, observable, and scalable microservices ecosystem. It showcases advanced concepts in system design, performance engineering, and service governance.

**Final Performance Metrics:** **16,878 QPS** @ 1000 concurrent users, with a **P99 latency of 0.14ms**.

## 🏗️ Final Architecture

![Architecture Diagram](https://mermaid.ink/svg/pako:eNqNVMFuwyAM_RXhPqBCo0eOnboKdaimTjttA8UEKTYkNgkk6a_XqSftJIt9sR977LEPzEwFq2Iu2I7Gq6v8hI1m433L6a7E0nC9_U7X8D6_hS0PqL0gA7-oYdAC2d3YnC_4i7c0Bq0R9p6G0ZJjW4kFzE5c3Jv8u8mY-ZgQh0-mN7i_uYQ8n6z4s7vQh8gD4lR-5Q63L78B7V0w9436LqgG5y7T75q4N12eJz8T_hGgK8p2u1i_28N2Vn31QYV6W1K3x4a24W5w7mC9Ww2_l-Xk-sHj3vH40N8hCgB8NfLw6-14G2p6C68QG3t7fUjD7R3e0fQ6jI9pDq8zWq_hO0Q9D32_H8J889V2F29eF_nL0x-4Xw7vP99-3_Y_8xUfO_Y-5V07tP3t3l3a-v234bXlP2r529w7Gj-U37XW2m4LhYyG9c_j2qG1o7mjt4O1p76hvaO9o7Ohtae9ob2jt4Oxo72juaO1p7-huaO_g6mjsa-5o7-vuae_o7mjvaO_o7Gjt6e9o7-jsYO1o7mju6e1r7ujt4Oxo7Wjuae3p72ju4O1o7mjua-3p72nu5MROd_c_k)

## 📖 Project Summary

This project systematically documents the journey of building and evolving a backend system through three distinct phases:

1.  **Phase 1: High-Performance Monolith**: Building a foundational service in Go, focusing on raw performance and establishing a performance testing benchmark.
2.  **Phase 2: Microservice Migration**: Refactoring the monolith into a distributed system to solve scalability and coupling issues, tackling the inherent challenges of RPC communication.
3.  **Phase 3: Production-Grade Ecosystem**: Augmenting the microservices with a complete service governance and observability stack, transforming it into a resilient, production-ready framework.

## 🏆 Key Achievements

-   **Performance Breakthrough**: Achieved **16,878 QPS** with **0.14ms P99 latency** in a full microservice environment, successfully recovering and optimizing performance to near-monolithic levels.
-   **Complete Architectural Evolution**: Successfully migrated from a 21k QPS monolith to a scalable, decoupled microservice architecture without sacrificing performance.
-   **Production-Ready Service Governance**: Implemented a full resilience stack, including circuit breakers, intelligent retries, and service fallbacks.
-   **Enterprise-Grade Observability**: Built a comprehensive monitoring platform with Prometheus, Grafana, and Jaeger, providing deep insights into system health and performance.
-   **Scientific Performance Engineering**: Established a standardized 5-level progressive load testing methodology, ensuring every architectural change was quantitatively validated.

## 🚀 Project Evolution

### ✅ Phase 1: High-Performance Monolith (Completed)

-   [x] **Architecture**: Gin + Redis + Goroutine Pool.
-   [x] **Performance**: Achieved **21,000+ QPS** with P99 latency < 50ms.
-   [x] **Key Techniques**: `sync.Pool` for object reuse, dynamic worker pools, and Redis pipeline optimizations.
-   [x] **Outcome**: Established a solid, high-performance baseline.

### ✅ Phase 2: Microservice Migration (Completed)

-   [x] **Architecture**: Split into API Gateway, Counter Service, and Analytics Service.
-   [x] **Communication**: Implemented gRPC for inter-service communication.
-   [x] **Core Challenge**: Solved the initial performance drop (from 21k to <8k QPS) by engineering a custom non-blocking, load-balancing `ServiceManager` for gRPC connections.
-   [x] **Outcome**: Created a scalable, decoupled foundation, ready for production features.

### ✅ Phase 3: Production-Grade Ecosystem (Completed)

-   [x] **Service Discovery**: Integrated **Consul** for service registration, discovery, and health checks.
-   [x] **Resilience**: Implemented a full **Circuit Breaker, Retry, and Fallback** stack.
-   [x] **Observability**: Deployed a **Prometheus, Grafana, and Jaeger** stack for metrics, visualization, and tracing.
-   [x] **Asynchronous Processing**: Leveraged **Kafka** for fully asynchronous event processing between services.
-   [x] **Outcome**: Achieved a final, stable performance of **16,878 QPS** with full observability and resilience, marking the project as production-ready.

## 🛠️ System Components

### Core Infrastructure

| Component | Technology | Purpose |
| :--- | :--- | :--- |
| **API Gateway** | Gin | Handles all incoming HTTP traffic, performs routing, and translates requests to gRPC. |
| **Service Discovery** | Consul | Allows services to dynamically find and communicate with each other. Manages health checks. |
| **Dynamic Config** | Consul KV | Centralized configuration management, enabling real-time config updates. |
| **Event Bus** | Kafka | Decouples services by providing a reliable, asynchronous message queue for events. |
| **Data Store** | Redis | High-performance primary storage for counters and system state. |

### Service Governance (Resilience)

| Component | Implementation | Purpose |
| :--- | :--- | :--- |
| **Load Balancing** | Custom gRPC `ServiceManager` | Distributes traffic across healthy service instances using Round-Robin. |
| **Health Checks** | Consul + gRPC | Actively monitors service health and automatically removes failed instances. |
| **Circuit Breaker** | Custom gRPC Interceptor | Prevents network or service failures from cascading to other services. |
| **Retry Mechanism** | Custom gRPC Interceptor | Automatically retries failed requests with exponential backoff and jitter. |
| **Service Fallback**| Custom gRPC Interceptor | Provides a degraded-functionality response when a service is unavailable. |

### Observability Stack

| Component | Technology | Purpose |
| :--- | :--- | :--- |
| **Metrics Collection** | Prometheus | Collects detailed metrics (HTTP, gRPC, Business, System) from all services. |
| **Dashboards** | Grafana | Visualizes system performance and health with pre-built, multi-layered dashboards. |
| **Alerting**| AlertManager | Fires alerts based on predefined rules (e.g., high error rates, high latency). |
| **Distributed Tracing**| Jaeger | Provides infrastructure for tracing requests as they travel across services. |
| **Structured Logging**| Zap | Generates structured, high-performance logs for easier parsing and analysis. |

## ⚙️ Technology Stack

-   **Language**: Go
-   **Frameworks**: Gin, gRPC
-   **Service Mesh & Discovery**: Consul
-   **Messaging**: Kafka
-   **Database**: Redis
-   **Observability**: Prometheus, Grafana, Jaeger, Zap Logger
-   **Containerization**: Docker, Docker Compose
-   **Load Testing**: `hey`

## 📈 Performance Testing

This project relies on a scientific, progressive load testing methodology to validate every optimization.

```bash
# Install the testing tool
go install github.com/rakyll/hey@latest

# Run the complete 5-level load test script
./scripts/load_test.sh

# Analyze performance using pprof
go tool pprof http://localhost:8080/debug/pprof/profile
```

### Test Levels

-   **Level 1**: 1k requests @ 10 concurrency (Sanity Check)
-   **Level 2**: 5k requests @ 50 concurrency (Moderate Load)
-   **Level 3**: 10k requests @ 100 concurrency (High Load)
-   **Level 4**: 50k requests @ 500 concurrency (Stress Test)
-   **Level 5**: 100k requests @ 1000 concurrency (Overload/Breaking Point Test)

## 📝 API Documentation

### Increment Counter

```http
POST /api/v1/counter/increment
Content-Type: application/json

{
  "resource_id": "article_001",
  "counter_type": "like",
  "delta": 1
}
```

### Get Counter

```http
GET /api/v1/counter/:resource_id/:counter_type
```

### Batch Get Counters

```http
POST /api/v1/counter/batch
Content-Type: application/json

{
  "queries": [
    {"resource_id": "article_001", "counter_type": "like"},
    {"resource_id": "article_002", "counter_type": "view"}
  ]
}
```

### Health & Metrics

```http
GET /api/v1/health   # Gateway health
GET /metrics         # Prometheus metrics endpoint
```

## 🚀 How to Run

1.  **Start Infrastructure**:
    ```bash
    ./scripts/start_monitoring.sh # Starts Prometheus, Grafana, etc.
    # Ensure Consul, Kafka, and Redis are running separately
    ```
2.  **Start Services**:
    ```bash
    ./scripts/start_all_services.sh
    ```
3.  **Run Load Test**:
    ```bash
    ./scripts/test_microservices_load.sh
    ```
4.  **View Dashboards**:
    -   **Grafana**: `http://localhost:3000`
    -   **Prometheus**: `http://localhost:9090`
    -   **Consul UI**: `http://localhost:8500`

## 📄 License

MIT License