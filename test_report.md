# HighGoPress Phase 1 Week 2 Performance Test Report

## Test Environment
- **Test Time**: 2025年 06月 11日 星期三 17:09:57 CST
- **Test Article**: article_1749632955
- **Service URL**: http://localhost:8080

## Performance Summary (High Load: 10k req, 100 concurrent)
```

Summary:
  Total:	0.4770 secs
  Slowest:	0.0386 secs
  Fastest:	0.0001 secs
  Average:	0.0046 secs
  Requests/sec:	20962.8585
  
  Total data:	1460000 bytes
  Size/request:	146 bytes

Response time histogram:
  0.000 [1]	|
  0.004 [7248]	|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.008 [1157]	|■■■■■■
  0.012 [345]	|■■
  0.016 [404]	|■■
  0.019 [396]	|■■
  0.023 [247]	|■
  0.027 [162]	|■
  0.031 [9]	|
  0.035 [28]	|
  0.039 [3]	|


Latency distribution:
  10% in 0.0006 secs
  25% in 0.0013 secs
  50% in 0.0024 secs
  75% in 0.0044 secs
  90% in 0.0144 secs
  95% in 0.0190 secs
  99% in 0.0246 secs

Details (average, fastest, slowest):
  DNS+dialup:	0.0000 secs, 0.0001 secs, 0.0386 secs
  DNS-lookup:	0.0000 secs, 0.0000 secs, 0.0065 secs
  req write:	0.0000 secs, 0.0000 secs, 0.0066 secs
  resp wait:	0.0043 secs, 0.0001 secs, 0.0384 secs
  resp read:	0.0002 secs, 0.0000 secs, 0.0109 secs

Status code distribution:
  [200]	10000 responses
```

## Worker Pool Stats
```json
{
  "data": {
    "general_pool": {
      "capacity": 2400,
      "running": 0,
      "waiting": 0,
      "free": 2400
    },
    "counter_pool": {
      "capacity": 1200,
      "running": 0,
      "waiting": 0,
      "free": 1200
    }
  },
  "status": "success",
  "timestamp": 1749632997
}
```

## Object Pool Stats  
```json
{
  "data": {
    "response": {
      "gets": 269345,
      "puts": 269345,
      "hit_rate": 100
    },
    "request": {
      "gets": 269345,
      "puts": 269345,
      "hit_rate": 100
    },
    "buffer": {
      "gets": 0,
      "puts": 0,
      "hit_rate": 0
    },
    "string_slice": {
      "gets": 3,
      "puts": 3,
      "hit_rate": 100
    }
  },
  "status": "success"
}
```

## Kafka Stats
```json
{
  "data": {
    "messages_sent": 196856,
    "events_sent": 181226,
    "messages_queued": 196856,
    "events_queued": 181226,
    "errors_count": 0,
    "last_message_time": 0
  },
  "status": "success"
}
```

## Files Generated
- cpu_profile.txt - CPU profiling analysis
- memory_profile.txt - Memory usage analysis  
- goroutine_profile.txt - Goroutine analysis
- test_report.md - This report

