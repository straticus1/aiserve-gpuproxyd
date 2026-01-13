# Deployment Guide

## Quick Start

### Full Stack (All Services)

Includes PostgreSQL, PgBouncer, Redis, and the GPU proxy server:

```bash
docker-compose up -d
```

**Services:**
- PostgreSQL: `localhost:5433`
- PgBouncer: `localhost:6432`
- Redis: `localhost:6380`
- HTTP API: `http://localhost:8080`
- gRPC API: `localhost:9090`

### External Database (Bring Your Own)

Use existing PostgreSQL and Redis instances:

```bash
docker-compose -f docker-compose.external-db.yml up -d
```

**Requirements:**
- PostgreSQL instance (local or remote)
- Redis instance (local or remote)
- Configure via `.env` file (see below)

## Configuration

### Environment Variables

Create a `.env` file or set environment variables:

```bash
# Database (required for external-db mode)
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your-secure-password
DB_NAME=gpuproxy

# Redis (required for external-db mode)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your-redis-password

# Auth (required for production)
JWT_SECRET=generate-a-secure-random-secret-key

# GPU Providers (optional - can start without)
GPU_ALLOW_START_WITHOUT_PROVIDERS=true
GPU_PREFERRED_BACKEND=auto  # auto, cuda, rocm, oneapi
VASTAI_API_KEY=your-vast-ai-key
IONET_API_KEY=your-ionet-key
```

### Full Configuration Template

See `.env.example` for all available configuration options.

## GPU Backend Detection

The server automatically detects local GPU backends at startup:

### Supported Backends

1. **NVIDIA CUDA**
   - Detects: `nvidia-smi` + CUDA toolkit
   - Path: `/usr/local/cuda` or `$CUDA_PATH`

2. **AMD ROCm**
   - Detects: `rocm-smi` + ROCm installation
   - Path: `/opt/rocm` or `$ROCM_PATH`

3. **Intel OneAPI**
   - Detects: `sycl-ls` or `clinfo` + OneAPI installation
   - Path: `/opt/intel/oneapi` or `$ONEAPI_ROOT`

### Hybrid Architecture

**Local GPU (bootstrap) + Cloud GPU (heavy compute):**

```bash
# Start with local GPU only
GPU_ALLOW_START_WITHOUT_PROVIDERS=true
GPU_PREFERRED_BACKEND=cuda

# Add cloud providers later for scaling
VASTAI_API_KEY=your-key
IONET_API_KEY=your-key
```

**Strategy:**
- Cheap local GPU handles lightweight tasks, API routing, preprocessing
- Expensive cloud GPUs (Vast.ai, io.net) handle heavy inference
- Automatic load balancing between local and cloud

## Network Configuration

### IPv4 Only

```bash
SERVER_HOST=0.0.0.0
```

### IPv6 Dual-Stack (Default)

```bash
SERVER_HOST=::
```

Listens on **both** IPv4 and IPv6.

### IPv6 Only

```bash
SERVER_HOST=::1
```

## Production Deployment

### 1. Security Checklist

- [ ] Change `JWT_SECRET` from default
- [ ] Set strong database passwords
- [ ] Enable TLS for gRPC (set `GRPC_TLS_CERT` and `GRPC_TLS_KEY`)
- [ ] Configure firewall rules
- [ ] Set `ENVIRONMENT=production`
- [ ] Review guard rails settings
- [ ] Enable syslog or file logging

### 2. Database Configuration

**With PgBouncer (recommended for high concurrency):**

```bash
DB_HOST=pgbouncer-hostname
DB_PORT=6432
DB_USE_PGBOUNCER=true
DB_PGBOUNCER_POOL_MODE=transaction
DB_MAX_CONNS=200
```

**Direct PostgreSQL (simpler, lower concurrency):**

```bash
DB_HOST=postgres-hostname
DB_PORT=5432
DB_USE_PGBOUNCER=false
DB_MAX_CONNS=25
```

See [PGBOUNCER_SETUP.md](./PGBOUNCER_SETUP.md) for detailed PgBouncer configuration.

### 3. Scaling

**Horizontal Scaling:**

```bash
# Deploy multiple instances behind a load balancer
# Each instance connects to same PostgreSQL + Redis

# Instance 1
SERVER_PORT=8080
GRPC_PORT=9090

# Instance 2
SERVER_PORT=8081
GRPC_PORT=9091
```

**Load Balancer:**
- Use nginx, HAProxy, or cloud load balancer
- Round-robin or least-connections strategy
- Health check endpoint: `GET /health`

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok",
  "timestamp": "2026-01-13T06:59:35Z"
}
```

### Service Status

```bash
docker-compose ps
```

### Logs

```bash
# All services
docker-compose logs -f

# Server only
docker logs -f gpuproxy-server

# Last 100 lines
docker logs --tail 100 gpuproxy-server
```

### Metrics

Check logs for:
- GPU backend detection results
- Connection pool statistics (if PgBouncer)
- Request rates and errors
- Guard rails triggers

## Troubleshooting

### Server Won't Start

**Check logs:**
```bash
docker logs gpuproxy-server
```

**Common issues:**
- Database connection failed → verify DB_HOST, DB_PORT, credentials
- Redis connection failed → verify REDIS_HOST, REDIS_PORT
- Port already in use → change SERVER_PORT or stop conflicting service

### No GPU Backends Detected

```
WARNING: No local GPU backends and no cloud provider API keys configured.
Server will start but GPU operations will fail until providers are configured.
```

**Solutions:**
1. Install CUDA, ROCm, or OneAPI drivers
2. Add cloud provider API keys (VASTAI_API_KEY, IONET_API_KEY)
3. Set `GPU_ALLOW_START_WITHOUT_PROVIDERS=true` to start anyway

### PgBouncer Connection Issues

See [PGBOUNCER_SETUP.md](./PGBOUNCER_SETUP.md) for detailed troubleshooting.

**Quick fix - use direct connection:**
```bash
DB_HOST=postgres
DB_PORT=5432
DB_USE_PGBOUNCER=false
```

### Out of Disk Space

```bash
# Clean up Docker
docker system prune -af --volumes

# Check space
df -h
docker system df
```

## Advanced

### Custom Protocols

The server supports 6 agent communication protocols:
- MCP (Model Context Protocol)
- A2A (Agent-to-Agent)
- ACP (Agent Communication Protocol)
- CUIC (QUIC-inspired Unified Inter-agent Communication)
- FIPA ACL (Foundation for Intelligent Physical Agents)
- KQML (Knowledge Query and Manipulation Language)

### Guard Rails

Spending limits per time window:

```bash
GUARDRAILS_ENABLED=true
GUARDRAILS_MAX_1440MIN_RATE=100.00  # $100/day
GUARDRAILS_MAX_48H_RATE=150.00      # $150/2 days
```

### TLS/SSL

**gRPC with TLS:**
```bash
# Generate certificates
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes

# Configure
GRPC_TLS_CERT=./server.crt
GRPC_TLS_KEY=./server.key
```

## Support

- GitHub Issues: https://github.com/straticus1/aiserve-gpuproxyd/issues
- Documentation: https://github.com/straticus1/aiserve-gpuproxyd/tree/main/docs
