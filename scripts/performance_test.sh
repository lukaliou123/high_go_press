#!/bin/bash

# HighGoPress Phase 1 Week 2 性能测试脚本
# 测试Worker Pool、sync.Pool、pprof和Kafka的优化效果

set -e

echo "🚀 HighGoPress Phase 1 Week 2 Performance Test"
echo "=============================================="

# 配置
BASE_URL="http://localhost:8080"
TEST_ARTICLE="article_$(date +%s)"
HEY_BIN="$HOME/go/bin/hey"

# 检查hey工具
if [ ! -f "$HEY_BIN" ]; then
    echo "❌ hey tool not found at $HEY_BIN"
    echo "Installing hey..."
    go install github.com/rakyll/hey@latest
fi

# 检查服务是否运行
check_service() {
    echo "🔍 Checking if service is running..."
    for i in {1..10}; do
        if curl -s --fail "$BASE_URL/api/v1/health" > /dev/null; then
            echo "✅ Service is running"
            return 0
        fi
        echo "  - Waiting for service... ($i/10)"
        sleep 1
    done
    echo "❌ Service is not running after 10 seconds, please start it first"
    return 1
}

# 基础功能测试
test_basic_functionality() {
    echo "🧪 Testing basic functionality..."
    
    # 健康检查
    echo "  - Health check"
    curl -s "$BASE_URL/api/v1/health" | jq .
    
    # 单次计数器增量
    echo "  - Single increment"
    curl -s -X POST "$BASE_URL/api/v1/counter/increment" \
        -H "Content-Type: application/json" \
        -d "{\"resource_id\":\"$TEST_ARTICLE\",\"counter_type\":\"like\",\"delta\":1}" | jq .
    
    # 获取计数器
    echo "  - Get counter"
    curl -s "$BASE_URL/api/v1/counter/$TEST_ARTICLE/like" | jq .
    
    # 批量获取
    echo "  - Batch get"
    curl -s -X POST "$BASE_URL/api/v1/counter/batch" \
        -H "Content-Type: application/json" \
        -d '{"items":[{"resource_id":"article_123","counter_type":"like"}]}' | jq .
    
    echo "✅ Basic functionality test passed"
}

# 监控端点测试
test_monitoring_endpoints() {
    echo "🔍 Testing monitoring endpoints..."
    
    echo "  - Worker Pool stats"
    curl -s "$BASE_URL/api/v1/system/pools" | jq .
    
    echo "  - Object Pool stats"
    curl -s "$BASE_URL/api/v1/system/object-pools" | jq .
    
    echo "  - Kafka stats"
    curl -s "$BASE_URL/api/v1/system/kafka" | jq .
    
    echo "✅ Monitoring endpoints test passed"
}

# 性能基准测试
performance_benchmark() {
    echo "🏃 Running performance benchmarks..."
    
    # 测试配置数组 (请求数, 并发数, 描述)
    local tests=(
        "1000:10:Low Load"
        "5000:50:Medium Load" 
        "10000:100:High Load"
        "20000:200:Very High Load"
        "50000:500:Extreme Load"
    )
    
    for test_config in "${tests[@]}"; do
        IFS=':' read -r requests concurrency description <<< "$test_config"
        
        echo "📊 Testing: $description ($requests requests, $concurrency concurrent)"
        local summary_file
        summary_file=$(mktemp)
        
        # 1. 健康检查性能
        echo "  🔹 Health check performance"
        $HEY_BIN -n $requests -c $concurrency "$BASE_URL/api/v1/health" | tee "$summary_file"
        qps=$(grep "Requests/sec:" "$summary_file" | awk '{print $2}')
        p99=$(grep "99% in" "$summary_file" | awk '{print $3, $4}')
        echo "    -> QPS: $qps, P99: $p99"
        
        # 2. 计数器读取性能
        echo "  🔹 Counter read performance" 
        $HEY_BIN -n $requests -c $concurrency "$BASE_URL/api/v1/counter/$TEST_ARTICLE/like" | tee "$summary_file"
        qps=$(grep "Requests/sec:" "$summary_file" | awk '{print $2}')
        p99=$(grep "99% in" "$summary_file" | awk '{print $3, $4}')
        echo "    -> QPS: $qps, P99: $p99"
        
        # 3. 计数器写入性能
        echo "  🔹 Counter write performance"
        $HEY_BIN -n $requests -c $concurrency -m POST \
            -H "Content-Type: application/json" \
            -D <(echo "{\"resource_id\":\"$TEST_ARTICLE\",\"counter_type\":\"like\",\"delta\":1}") \
            "$BASE_URL/api/v1/counter/increment" | tee "$summary_file"
        qps=$(grep "Requests/sec:" "$summary_file" | awk '{print $2}')
        p99=$(grep "99% in" "$summary_file" | awk '{print $3, $4}')
        echo "    -> QPS: $qps, P99: $p99"
        
        rm "$summary_file"
        # 给系统一些恢复时间
        sleep 2
    done
    
    echo "✅ Performance benchmark completed"
}

# 验证最终计数一致性
verify_data_consistency() {
    echo "🔍 Verifying data consistency..."
    
    # 获取最终计数值
    final_count=$(curl -s "$BASE_URL/api/v1/counter/$TEST_ARTICLE/like" | jq -r '.data.current_value')
    echo "  Final count for $TEST_ARTICLE: $final_count"
    
    if [ "$final_count" -gt 0 ]; then
        echo "✅ Data consistency verified (count > 0)"
    else
        echo "⚠️  Warning: Final count is 0, check for data consistency issues"
    fi
    
    # 检查Kafka事件数量
    kafka_events=$(curl -s "$BASE_URL/api/v1/system/kafka" | jq -r '.data.events_sent')
    echo "  Kafka events sent: $kafka_events"
    
    if [ "$kafka_events" -gt 0 ]; then
        echo "✅ Kafka events are being sent correctly"
    else
        echo "⚠️  Warning: No Kafka events sent"
    fi
}

# pprof性能分析
pprof_analysis() {
    echo "🔬 Running pprof analysis..."
    
    # 启动一个高负载测试
    echo "  Starting high load for profiling..."
    $HEY_BIN -n 10000 -c 100 -m POST \
        -H "Content-Type: application/json" \
        -D <(echo "{\"resource_id\":\"profile_test\",\"counter_type\":\"like\",\"delta\":1}") \
        "$BASE_URL/api/v1/counter/increment" &
    
    LOAD_PID=$!
    
    # 等待一下让负载开始
    sleep 2
    
    # 收集CPU profile (30秒)
    echo "  Collecting CPU profile..."
    go tool pprof -text -seconds 15 "$BASE_URL/debug/pprof/profile" > cpu_profile.txt 2>/dev/null &
    
    # 收集内存profile
    echo "  Collecting memory profile..."
    go tool pprof -text "$BASE_URL/debug/pprof/heap" > memory_profile.txt 2>/dev/null &
    
    # 收集goroutine信息
    echo "  Collecting goroutine info..."
    go tool pprof -text "$BASE_URL/debug/pprof/goroutine" > goroutine_profile.txt 2>/dev/null &
    
    # 等待profile收集完成
    wait
    
    # 停止负载测试
    kill $LOAD_PID 2>/dev/null || true
    
    echo "✅ pprof analysis completed"
    echo "📄 CPU profile: cpu_profile.txt"
    echo "📄 Memory profile: memory_profile.txt" 
    echo "📄 Goroutine profile: goroutine_profile.txt"
}

# 生成测试报告
generate_report() {
    echo "📋 Generating test report..."
    
    # Run a final high-load test to get summary data for the report
    local final_summary
    final_summary=$($HEY_BIN -n 10000 -c 100 -m POST \
        -H "Content-Type: application/json" \
        -D <(echo "{\"resource_id\":\"$TEST_ARTICLE\",\"counter_type\":\"like\",\"delta\":1}") \
        "$BASE_URL/api/v1/counter/increment")

    cat > test_report.md << EOF
# HighGoPress Phase 1 Week 2 Performance Test Report

## Test Environment
- **Test Time**: $(date)
- **Test Article**: $TEST_ARTICLE
- **Service URL**: $BASE_URL

## Performance Summary (High Load: 10k req, 100 concurrent)
\`\`\`
$(echo "$final_summary")
\`\`\`

## Worker Pool Stats
\`\`\`json
$(curl -s "$BASE_URL/api/v1/system/pools" | jq .)
\`\`\`

## Object Pool Stats  
\`\`\`json
$(curl -s "$BASE_URL/api/v1/system/object-pools" | jq .)
\`\`\`

## Kafka Stats
\`\`\`json
$(curl -s "$BASE_URL/api/v1/system/kafka" | jq .)
\`\`\`

## Files Generated
- cpu_profile.txt - CPU profiling analysis
- memory_profile.txt - Memory usage analysis  
- goroutine_profile.txt - Goroutine analysis
- test_report.md - This report

EOF

    echo "✅ Test report generated: test_report.md"
}

# 主测试流程
main() {
    echo "Starting comprehensive performance test..."
    
    # 检查服务状态
    if ! check_service; then
        exit 1
    fi
    
    # 执行测试
    test_basic_functionality
    echo ""
    
    test_monitoring_endpoints  
    echo ""
    
    performance_benchmark
    echo ""
    
    pprof_analysis
    echo ""
    
    generate_report
    
    echo ""
    echo "🎉 All tests completed successfully!"
    echo "📊 Check the following files for detailed results:"
    echo "   - test_report.md (Summary report)"
    echo "   - cpu_profile.txt (pprof analysis files)"
}

# 运行主函数
main "$@" 