# Enterprise Observability Guide

## Overview

The GPU Proxy system includes comprehensive enterprise observability features for monitoring, metrics collection, and operational visibility.

## Features

### 1. Structured JSON Logging

All logs are output in structured JSON format for easy parsing and aggregation by tools like:
- Splunk
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Datadog
- New Relic

**Log Levels:**
- `DEBUG` - Detailed debugging information
- `INFO` - General informational messages
- `WARN` - Warning messages
- `ERROR` - Error messages with stack traces
- `FATAL` - Fatal errors that cause shutdown

**Log Fields:**
- `timestamp` - ISO 8601 UTC timestamp
- `level` - Log level
- `service` - Service name (gpuproxy)
- `message` - Log message
- `request_id` - Unique request identifier for tracing
- `user_id` - Authenticated user ID
- `method` - HTTP method
- `path` - Request path
- `status_code` - HTTP status code
- `duration_ms` - Request duration in milliseconds
- `error` - Error message (if applicable)
- `stack_trace` - Stack trace (for errors)

**Example Log Entry:**
```json
{
  "timestamp": "2026-01-13T12:34:56.789Z",
  "level": "INFO",
  "service": "gpuproxy",
  "message": "Request completed",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "method": "POST",
  "path": "/api/v1/gpu/proxy",
  "status_code": 200,
  "duration_ms": 1234
}
```

### 2. Prometheus Metrics

Comprehensive metrics in Prometheus format available at `/metrics` endpoint.

**Request Metrics:**
- `gpuproxy_uptime_seconds` - Server uptime
- `gpuproxy_requests_total` - Total HTTP requests
- `gpuproxy_requests_failed` - Failed requests
- `gpuproxy_requests_in_flight` - Current active requests
- `gpuproxy_request_success_rate` - Success rate percentage
- `gpuproxy_request_duration_milliseconds` - Request duration percentiles (p50, p95, p99)

**GPU Metrics:**
- `gpuproxy_gpu_instances_active` - Active GPU instances
- `gpuproxy_gpu_requests_total` - Total GPU requests
- `gpuproxy_gpu_requests_failed` - Failed GPU requests
- `gpuproxy_gpu_cost_total` - Total GPU cost in USD

**Database Metrics:**
- `gpuproxy_db_connections_active` - Active database connections
- `gpuproxy_db_connections_idle` - Idle database connections
- `gpuproxy_db_queries_total` - Total database queries
- `gpuproxy_db_errors_total` - Database errors
- `gpuproxy_db_query_duration_milliseconds` - Query duration percentiles

**Cache Metrics:**
- `gpuproxy_cache_hits` - Cache hits
- `gpuproxy_cache_misses` - Cache misses
- `gpuproxy_cache_hit_rate` - Cache hit rate percentage

**System Metrics:**
- `gpuproxy_goroutines` - Number of goroutines
- `gpuproxy_memory_heap_alloc_mb` - Heap memory in MB
- `gpuproxy_gc_total` - Total GC runs

### 3. Observability Endpoints

#### GET /health
Comprehensive health check with component status.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2026-01-13T12:34:56Z",
  "checks": {
    "database": {
      "healthy": true,
      "response_time_ms": 5,
      "total_connections": 45,
      "idle_connections": 20,
      "max_connections": 1000
    },
    "redis": {
      "healthy": true,
      "response_time_ms": 2,
      "total_connections": 10,
      "idle_connections": 5
    }
  }
}
```

**Status Codes:**
- `200 OK` - All systems healthy
- `503 Service Unavailable` - One or more systems unhealthy

#### GET /metrics
Prometheus-formatted metrics for scraping.

**Example:**
```
# HELP gpuproxy_requests_total Total number of HTTP requests
# TYPE gpuproxy_requests_total counter
gpuproxy_requests_total 12345

# HELP gpuproxy_request_duration_milliseconds Request duration statistics
# TYPE gpuproxy_request_duration_milliseconds summary
gpuproxy_request_duration_milliseconds{quantile="0.95"} 123.45
```

#### GET /stats
JSON-formatted statistics for dashboards.

**Response:**
```json
{
  "uptime_seconds": 86400,
  "requests": {
    "total": 12345,
    "failed": 123,
    "in_flight": 5,
    "success_rate": 99.0,
    "duration": {
      "p50_ms": 50.0,
      "p95_ms": 150.0,
      "p99_ms": 300.0,
      "avg_ms": 75.0
    }
  },
  "gpu": {
    "active_instances": 10,
    "total_cost_usd": 1234.56,
    "requests_total": 5000,
    "requests_failed": 50
  },
  "database": {
    "connections_active": 45,
    "connections_idle": 20,
    "queries_total": 10000,
    "errors": 5
  },
  "cache": {
    "hits": 8000,
    "misses": 2000,
    "hit_rate": 80.0
  },
  "system": {
    "goroutines": 50,
    "heap_alloc_mb": 128,
    "gc_runs": 20
  }
}
```

#### GET /polling
Lightweight endpoint for real-time status polling (minimal response).

**Response:**
```json
{
  "timestamp": "2026-01-13T12:34:56Z",
  "status": "ok",
  "metrics": {
    "requests_in_flight": 5,
    "active_gpu_instances": 10,
    "db_connections_active": 45,
    "cache_hit_rate": 80.0
  }
}
```

#### GET /monitor
Comprehensive monitoring data combining health checks and metrics.

**Response:**
```json
{
  "timestamp": "2026-01-13T12:34:56Z",
  "status": "ok",
  "health": {
    "database": { "healthy": true, "response_time_ms": 5 },
    "redis": { "healthy": true, "response_time_ms": 2 }
  },
  "metrics": {
    "requests": { ... },
    "gpu": { ... },
    "database": { ... }
  }
}
```

## Integration Guides

### Prometheus Setup

1. Add to `prometheus.yml`:
```yaml
scrape_configs:
  - job_name: 'gpuproxy'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

2. Verify scraping:
```bash
curl http://localhost:8080/metrics
```

### node_exporter Integration

For system-level metrics alongside application metrics:

1. Install node_exporter:
```bash
docker run -d \
  --name node_exporter \
  -p 9100:9100 \
  -v "/proc:/host/proc:ro" \
  -v "/sys:/host/sys:ro" \
  -v "/:/rootfs:ro" \
  prom/node-exporter
```

2. Add to `prometheus.yml`:
```yaml
scrape_configs:
  - job_name: 'node_exporter'
    static_configs:
      - targets: ['localhost:9100']

  - job_name: 'gpuproxy'
    static_configs:
      - targets: ['localhost:8080']
```

3. Query combined metrics in Prometheus/Grafana

### Grafana Dashboard

Import pre-built dashboard for GPU Proxy monitoring:

**Key Panels:**
- Request rate and latency percentiles
- GPU instance count and cost tracking
- Database connection pool utilization
- Cache hit rates
- System resource usage (CPU, memory, goroutines)
- Error rates and SLA compliance

**Alert Rules:**
- High error rate (>5%)
- Database connection pool exhaustion (>90%)
- High request latency (p95 > 500ms)
- Low cache hit rate (<70%)
- GPU provisioning failures

### ELK Stack Integration

Forward structured JSON logs to Logstash:

```yaml
# logstash.conf
input {
  tcp {
    port => 5000
    codec => json_lines
  }
}

filter {
  if [service] == "gpuproxy" {
    mutate {
      add_tag => ["gpuproxy"]
    }
  }
}

output {
  elasticsearch {
    hosts => ["localhost:9200"]
    index => "gpuproxy-%{+YYYY.MM.dd}"
  }
}
```

Configure syslog forwarding in `config.toml`:
```toml
[logging]
syslog_enabled = true
syslog_network = "tcp"
syslog_address = "localhost:5000"
```

### Request Tracing

Track requests across distributed systems using request IDs:

1. Client includes `X-Request-ID` header
2. Server propagates request ID through all logs
3. Query logs by request ID to trace full request lifecycle

**Example:**
```bash
# Make request with custom request ID
curl -H "X-Request-ID: my-trace-123" http://localhost:8080/api/v1/gpu/proxy

# Query logs
jq 'select(.request_id == "my-trace-123")' < logs/gpuproxy.log
```

## Performance Tuning

### Metrics Collection

Metrics are updated in real-time using atomic operations for minimal overhead:
- Request counting: ~10ns per request
- Histogram updates: ~50ns per request
- System metrics: Collected every 10 seconds in background

### Log Volume Management

Control log volume with log levels:
- **Production**: `INFO` (recommended)
- **Debugging**: `DEBUG` (verbose)
- **High-Traffic**: `WARN` (minimal)

Set log level via environment:
```bash
export LOG_LEVEL=INFO
```

Or in `config.toml`:
```toml
[logging]
level = "INFO"
```

## High-Scale Deployments (1000+ Connections)

For deployments handling 1000+ concurrent connections:

1. **Enable Metrics Sampling**:
   - Sample request duration histograms (1% sampling)
   - Reduces memory overhead by 99%

2. **Async Log Shipping**:
   - Buffer logs in memory
   - Ship to aggregator in batches
   - Use rsyslog or fluentd

3. **Dedicated Metrics Endpoint**:
   - Run on separate port (9090)
   - Isolate from application traffic
   - Prevent scraping from impacting requests

4. **Connection Pooling**:
   - Database: 1000 max connections
   - Redis: 100 connections
   - PgBouncer: 100 pool size

## Security Considerations

### Metrics Endpoint

The `/metrics` endpoint is public by default. In production:

1. **Restrict Access**:
```go
// Add middleware to require authentication
protected.HandleFunc("/metrics", observabilityHandler.HandleMetrics).Methods("GET")
```

2. **Use Separate Port**:
```yaml
# Expose metrics on internal network only
ports:
  - "8080:8080"  # Public API
  - "127.0.0.1:9090:9090"  # Metrics (localhost only)
```

3. **Firewall Rules**:
```bash
# Allow Prometheus server only
iptables -A INPUT -p tcp --dport 9090 -s <prometheus-ip> -j ACCEPT
iptables -A INPUT -p tcp --dport 9090 -j DROP
```

### Log Sanitization

Sensitive data is automatically excluded from logs:
- API keys (hashed)
- Passwords (never logged)
- Credit card numbers (PCI compliance)

## Troubleshooting

### High Memory Usage

Check goroutine leaks:
```bash
curl http://localhost:8080/stats | jq '.system.goroutines'
```

Normal: 50-200 goroutines
High: >1000 goroutines (indicates leak)

### Database Connection Pool Exhaustion

Monitor pool utilization:
```bash
curl http://localhost:8080/stats | jq '.database'
```

If `connections_active` approaches `max_connections`, increase pool size in `config.toml`.

### Cache Performance

Check cache hit rate:
```bash
curl http://localhost:8080/stats | jq '.cache.hit_rate'
```

Target: >80% hit rate
Low (<70%): Increase cache TTL or size

## API Reference

All observability endpoints return JSON with proper content-type headers and support CORS for dashboard integration.

**Authentication**: Public endpoints (no auth required)
**Rate Limiting**: Exempt from rate limits
**Caching**: Responses not cached (always fresh data)

## Support

For issues or questions:
- GitHub: https://github.com/aiserve/gpuproxy
- Platform: https://aiserve.farm
