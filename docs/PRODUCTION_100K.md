# Production Deployment Guide for 100,000+ Concurrent Connections

## Overview

This guide covers deploying GPU Proxy as the backbone infrastructure for aiserve.farm, supporting 100,000+ concurrent connections with autonomous agent management, enterprise observability, and n8n workflow automation.

## System Requirements

### Hardware (per instance)

**Minimum:**
- CPU: 32 cores (64 threads)
- RAM: 64GB
- Network: 10Gbps NIC
- Storage: 500GB NVMe SSD

**Recommended:**
- CPU: 64 cores (128 threads)
- RAM: 128GB
- Network: 25Gbps NIC or higher
- Storage: 1TB NVMe SSD (RAID 1)

### Operating System

**Linux Kernel 5.10+** with optimized network stack:
```bash
# Check kernel version
uname -r
```

## System Tuning

### 1. Network Stack Optimization

Create `/etc/sysctl.d/99-gpuproxy-100k.conf`:

```bash
# TCP settings for 100k+ connections
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 65536
net.ipv4.tcp_max_syn_backlog = 65536
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_fin_timeout = 15
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_max_tw_buckets = 2000000

# Socket buffer sizes (10MB)
net.core.rmem_max = 10485760
net.core.wmem_max = 10485760
net.core.rmem_default = 10485760
net.core.wmem_default = 10485760
net.ipv4.tcp_rmem = 4096 87380 10485760
net.ipv4.tcp_wmem = 4096 65536 10485760
net.ipv4.tcp_mem = 786432 1048576 26777216

# TCP keepalive
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_probes = 3
net.ipv4.tcp_keepalive_intvl = 10

# Disable slow start after idle
net.ipv4.tcp_slow_start_after_idle = 0

# Enable TCP Fast Open
net.ipv4.tcp_fastopen = 3

# Congestion control (BBR for high-throughput)
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
```

Apply settings:
```bash
sudo sysctl -p /etc/sysctl.d/99-gpuproxy-100k.conf
```

### 2. File Descriptor Limits

Edit `/etc/security/limits.conf`:
```
*  soft  nofile  1000000
*  hard  nofile  1000000
root  soft  nofile  1000000
root  hard  nofile  1000000
```

For systemd services, edit `/etc/systemd/system/gpuproxy.service.d/limits.conf`:
```ini
[Service]
LimitNOFILE=1000000
```

### 3. Process Limits

Edit `/etc/security/limits.conf`:
```
*  soft  nproc  unlimited
*  hard  nproc  unlimited
```

### 4. PostgreSQL Tuning

Edit `postgresql.conf` for 100k+ connections:
```ini
# Connection settings
max_connections = 500  # PgBouncer handles client connections

# Memory settings (for 128GB RAM)
shared_buffers = 32GB
effective_cache_size = 96GB
maintenance_work_mem = 2GB
work_mem = 64MB

# Checkpoint settings
checkpoint_completion_target = 0.9
wal_buffers = 16MB
min_wal_size = 2GB
max_wal_size = 8GB

# Query planner
random_page_cost = 1.1  # For SSD
effective_io_concurrency = 200

# Write ahead log
wal_level = replica
max_wal_senders = 10
synchronous_commit = off  # For high-throughput (trade durability for speed)

# Logging
log_min_duration_statement = 1000  # Log slow queries (>1s)
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '
```

### 5. PgBouncer Configuration

Create `pgbouncer.ini`:
```ini
[databases]
gpuproxy = host=postgres port=5432 dbname=gpuproxy

[pgbouncer]
listen_addr = *
listen_port = 6432
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt

# CRITICAL for 100k connections
pool_mode = transaction
max_client_conn = 100000
default_pool_size = 500  # Connections to PostgreSQL
reserve_pool_size = 50
reserve_pool_timeout = 5

# Connection management
server_lifetime = 1800
server_idle_timeout = 300
server_connect_timeout = 15
query_timeout = 0
query_wait_timeout = 120

# Resource limits
max_db_connections = 500
max_user_connections = 500

# Logging
admin_users = postgres
stats_users = stats, postgres
log_connections = 0
log_disconnections = 0
log_pooler_errors = 1
```

### 6. Redis Tuning

Edit `redis.conf`:
```
# Network
bind 0.0.0.0
port 6379
tcp-backlog 65535
timeout 300
tcp-keepalive 60

# Memory
maxmemory 16gb
maxmemory-policy allkeys-lru

# Persistence (disable for cache-only)
save ""
appendonly no

# Performance
io-threads 4
io-threads-do-reads yes
```

## Docker Deployment (Production)

### docker-compose.production-100k.yml

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    container_name: gpuproxy-postgres
    environment:
      POSTGRES_USER: gpuproxy
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: gpuproxy
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./postgresql.conf:/etc/postgresql/postgresql.conf
    command: postgres -c config_file=/etc/postgresql/postgresql.conf
    deploy:
      resources:
        limits:
          cpus: '16'
          memory: 40G
        reservations:
          cpus: '8'
          memory: 32G
    networks:
      - gpuproxy-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U gpuproxy"]
      interval: 10s
      timeout: 5s
      retries: 5

  pgbouncer:
    image: edoburu/pgbouncer:latest
    container_name: gpuproxy-pgbouncer
    environment:
      DATABASE_URL: postgres://gpuproxy:${DB_PASSWORD}@postgres:5432/gpuproxy
      POOL_MODE: transaction
      MAX_CLIENT_CONN: 100000
      DEFAULT_POOL_SIZE: 500
      RESERVE_POOL_SIZE: 50
    ports:
      - "6432:5432"
    depends_on:
      - postgres
    deploy:
      resources:
        limits:
          cpus: '8'
          memory: 8G
        reservations:
          cpus: '4'
          memory: 4G
    networks:
      - gpuproxy-network

  redis:
    image: redis:7-alpine
    container_name: gpuproxy-redis
    command: redis-server --maxmemory 16gb --maxmemory-policy allkeys-lru --tcp-backlog 65535
    volumes:
      - redis-data:/data
    deploy:
      resources:
        limits:
          cpus: '8'
          memory: 18G
        reservations:
          cpus: '4'
          memory: 16G
    networks:
      - gpuproxy-network

  gpuproxy:
    image: aiserve/gpuproxyd:production-100k
    container_name: gpuproxy-server
    environment:
      - CONFIG_FILE=/app/config.production-100k.toml
      - DB_PASSWORD=${DB_PASSWORD}
      - JWT_SECRET=${JWT_SECRET}
      - VASTAI_API_KEY=${VASTAI_API_KEY}
      - IONET_API_KEY=${IONET_API_KEY}
      - STRIPE_SECRET_KEY=${STRIPE_SECRET_KEY}
      - GOMEMLIMIT=16GB
      - GOMAXPROCS=32
    ports:
      - "8080:8080"  # HTTP API
      - "9090:9090"  # gRPC + Prometheus metrics
    volumes:
      - ./config.production-100k.toml:/app/config.production-100k.toml
      - gpuproxy-logs:/app/logs
    depends_on:
      - postgres
      - pgbouncer
      - redis
    deploy:
      replicas: 3  # Run 3 instances behind load balancer
      resources:
        limits:
          cpus: '32'
          memory: 20G
        reservations:
          cpus: '16'
          memory: 16G
    networks:
      - gpuproxy-network
    ulimits:
      nofile:
        soft: 1000000
        hard: 1000000
      nproc:
        soft: unlimited
        hard: unlimited

  # MCP Server for agent-sdk-go integration
  mcp-server:
    image: aiserve/gpuproxyd-mcp:latest
    container_name: gpuproxy-mcp
    environment:
      - GPU_PROXY_URL=http://gpuproxy:8080
      - GPU_PROXY_API_KEY=${ADMIN_API_KEY}
    networks:
      - gpuproxy-network

  # Load balancer (HAProxy or Nginx)
  loadbalancer:
    image: haproxy:2.8-alpine
    container_name: gpuproxy-lb
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro
      - ./ssl:/etc/ssl/certs:ro
    depends_on:
      - gpuproxy
    deploy:
      resources:
        limits:
          cpus: '16'
          memory: 8G
    networks:
      - gpuproxy-network

  # Prometheus monitoring
  prometheus:
    image: prom/prometheus:latest
    container_name: gpuproxy-prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--storage.tsdb.retention.time=30d'
    networks:
      - gpuproxy-network

  # Grafana dashboards
  grafana:
    image: grafana/grafana:latest
    container_name: gpuproxy-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana-dashboards:/etc/grafana/provisioning/dashboards
    networks:
      - gpuproxy-network

volumes:
  postgres-data:
  redis-data:
  gpuproxy-logs:
  prometheus-data:
  grafana-data:

networks:
  gpuproxy-network:
    driver: bridge
```

## Kubernetes Deployment (Enterprise)

For enterprise scale (1M+ connections across multiple instances):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gpuproxy
  namespace: aiserve-farm
spec:
  replicas: 10  # 10 instances × 100k = 1M connections
  selector:
    matchLabels:
      app: gpuproxy
  template:
    metadata:
      labels:
        app: gpuproxy
    spec:
      containers:
      - name: gpuproxy
        image: aiserve/gpuproxyd:production-100k
        resources:
          requests:
            memory: "16Gi"
            cpu: "16"
          limits:
            memory: "20Gi"
            cpu: "32"
        env:
        - name: GOMEMLIMIT
          value: "16GB"
        - name: GOMAXPROCS
          value: "32"
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: grpc
```

## Load Testing

### Test with `hey` (100k connections):

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Test 100k requests, 10k concurrent
hey -n 100000 -c 10000 -m GET http://localhost:8080/health

# Test sustained load
hey -z 60s -c 50000 -m GET http://localhost:8080/stats
```

### Test with `wrk2` (more realistic):

```bash
# Install wrk2
git clone https://github.com/giltene/wrk2.git
cd wrk2 && make

# Test with 10k connections, 100k req/s for 5 minutes
./wrk -t32 -c10000 -d300s -R100000 http://localhost:8080/health
```

## Monitoring & Alerts

### Key Metrics to Monitor:

1. **Connection Pool Utilization**
   - Alert if > 90% for PgBouncer
   - Alert if > 85% for PostgreSQL

2. **Request Latency**
   - P50 < 100ms
   - P95 < 500ms
   - P99 < 2s

3. **Error Rate**
   - < 0.1% for 5xx errors
   - < 1% for 4xx errors

4. **System Resources**
   - CPU < 90%
   - Memory < 85%
   - Network < 80% bandwidth

5. **GPU Costs**
   - Daily spend tracking
   - Alert if > budget

### Prometheus Alerts:

```yaml
groups:
- name: gpuproxy_alerts
  rules:
  - alert: HighErrorRate
    expr: rate(gpuproxy_requests_failed[5m]) / rate(gpuproxy_requests_total[5m]) > 0.05
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "High error rate detected"

  - alert: DatabasePoolExhaustion
    expr: gpuproxy_db_connections_active / gpuproxy_db_connections_max > 0.9
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Database connection pool near exhaustion"

  - alert: HighLatency
    expr: histogram_quantile(0.99, rate(gpuproxy_request_duration_milliseconds_bucket[5m])) > 5000
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "P99 latency > 5s"
```

## Backup & Disaster Recovery

### Database Backups:

```bash
# Automated daily backups
0 2 * * * docker exec gpuproxy-postgres pg_dump -U gpuproxy gpuproxy | gzip > /backups/gpuproxy-$(date +\%Y\%m\%d).sql.gz

# Retention: 30 days
find /backups -name "gpuproxy-*.sql.gz" -mtime +30 -delete
```

### Redis Persistence (if needed):

```bash
# Enable RDB snapshots in redis.conf
save 900 1
save 300 10
save 60 10000
```

## Security Hardening

### 1. Network Isolation

- Run in private VPC
- Use security groups to restrict access
- Only expose port 80/443 publicly
- Internal services (PostgreSQL, Redis) on private network only

### 2. API Rate Limiting

- Per-IP: 1000 req/min
- Per-User: 10000 req/min
- Per-API-Key: 100000 req/min

### 3. DDoS Protection

- Cloudflare or AWS Shield
- Rate limiting at edge
- Connection limits per IP

### 4. SSL/TLS

```bash
# Use Let's Encrypt for SSL certificates
certbot certonly --standalone -d gpuproxy.aiserve.farm
```

## Cost Optimization

For 100k connections at aiserve.farm scale:

**Infrastructure Costs (monthly):**
- Compute (3×32core): $3,000
- PostgreSQL RDS: $2,000
- Redis ElastiCache: $1,500
- Bandwidth (10TB): $900
- **Total: ~$7,400/month**

**Per-Connection Cost:** $0.000074/month

**Revenue Model:**
- 100k users × $10/month = $1M/month
- Infrastructure: $7.4k
- **Profit: $992.6k/month (99.3% margin)**

## Troubleshooting

### Issue: Connection timeouts

**Solution:** Increase `net.ipv4.tcp_max_syn_backlog` and `net.core.somaxconn`

### Issue: Out of file descriptors

**Solution:** Check `ulimit -n`, increase in `/etc/security/limits.conf`

###Issue: Database connection pool exhaustion

**Solution:** Increase PgBouncer `default_pool_size` or reduce `max_client_conn`

### Issue: High memory usage

**Solution:** Check for goroutine leaks with `/stats`, tune `GOMEMLIMIT`

## Support

- Email: support@aiserve.farm
- Discord: https://discord.gg/aiserve
- GitHub Issues: https://github.com/aiserve/gpuproxy/issues
