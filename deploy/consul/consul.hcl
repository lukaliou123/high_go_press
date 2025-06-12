# Consul 配置文件
# HighGoPress Phase 2 服务发现

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

# 服务注册配置
services {
  name = "high-go-press-gateway"
  id = "gateway-1"
  port = 8080
  tags = ["gateway", "http", "api"]
  
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
  tags = ["counter", "grpc", "microservice"]
  
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
  tags = ["analytics", "grpc", "microservice"]
  
  check {
    name = "Analytics TCP Health Check"
    tcp = "localhost:9002"
    interval = "10s"
    timeout = "3s"
  }
} 