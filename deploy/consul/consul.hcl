# Consul 配置文件
# HighGoPress Phase 2 服务发现 + Kafka集成

datacenter = "dc1"
data_dir = "/tmp/consul"
log_level = "INFO"
node_name = "high-go-press-consul"
server = true
bootstrap_expect = 1

# API 和 Web UI
client_addr = "0.0.0.0"
ui_config {
  enabled = true
}

# 服务端配置
ports {
  grpc = 8502
  http = 8500
  dns = 8600
}

# 健康检查配置
connect {
  enabled = true
}

# ACL 配置 (生产环境启用)
acl = {
  enabled = false
  default_policy = "allow"
}

# 🌐 微服务注册配置
services {
  name = "high-go-press-gateway"
  id = "gateway-1"
  port = 8080
  tags = ["gateway", "http", "api", "v2.0"]
  
  check {
    name = "Gateway Health Check"
    http = "http://localhost:8080/api/v1/health"
    interval = "10s"
    timeout = "3s"
  }
}

services {
  name = "high-go-press-counter"
  id = "counter-1"
  port = 9001
  tags = ["counter", "grpc", "microservice", "v2.0"]
  
  check {
    name = "Counter TCP Health Check"
    tcp = "localhost:9001"
    interval = "10s"
    timeout = "3s"
  }
}

services {
  name = "high-go-press-analytics"
  id = "analytics-1" 
  port = 9002
  tags = ["analytics", "grpc", "microservice", "v2.0"]
  
  check {
    name = "Analytics TCP Health Check"
    tcp = "localhost:9002"
    interval = "10s"
    timeout = "3s"
  }
}

# 🔥 Kafka服务注册 (KRaft模式，无需Zookeeper)
services {
  name = "high-go-press-kafka"
  id = "kafka-1"
  port = 9092
  tags = ["kafka", "messaging", "kraft", "async", "v2.8"]
  
  meta = {
    version = "2.13-2.8.0"
    mode = "kraft"
    topics = "counter-events"
    partitions = "4"
    compression = "snappy"
  }
  
  check {
    name = "Kafka TCP Health Check"
    tcp = "localhost:9092"
    interval = "15s"
    timeout = "5s"
  }
}

# 📊 Redis服务注册 (数据存储)
services {
  name = "high-go-press-redis"
  id = "redis-1"
  port = 6379
  tags = ["redis", "cache", "storage", "persistence"]
  
  meta = {
    version = "7.0"
    persistence = "true"
    max_memory = "256mb"
  }
  
  check {
    name = "Redis Health Check"
    tcp = "localhost:6379"
    interval = "10s"
    timeout = "3s"
  }
} 