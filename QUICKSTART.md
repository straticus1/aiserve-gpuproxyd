# Quick Start Guide

## Build & Run

```bash
# Install dependencies
make deps

# Build all binaries
make build

# Copy environment file
cp .env.example .env

# Edit configuration (add your API keys)
nano .env

# Start with Docker (recommended)
make docker-up

# Or run locally
make run

# Or with developer/debug mode
make run-dev
```

## First Steps

### 1. Create Admin User
```bash
./bin/aiserve-gpuproxy-admin create-user admin@example.com SecurePass123 "Admin User"
./bin/aiserve-gpuproxy-admin make-admin admin@example.com
```

### 2. Create API Key
```bash
./bin/aiserve-gpuproxy-admin create-apikey admin@example.com "Production Key"
```

### 3. Test the API
```bash
export GPUPROXY_API_KEY="your-api-key-here"

# List GPUs
./bin/aiserve-gpuproxy-client -key $GPUPROXY_API_KEY list

# List from specific provider
./bin/aiserve-gpuproxy-client -key $GPUPROXY_API_KEY list vast.ai
```

### 4. Create GPU Instances

**Single Instance:**
```bash
curl -X POST http://localhost:8080/api/v1/gpu/instances/vast.ai/12345 \
  -H "X-API-Key: $GPUPROXY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"image": "nvidia/cuda:12.0.0-base-ubuntu22.04"}'
```

**Batch Create (up to 8 from each provider):**
```bash
curl -X POST http://localhost:8080/api/v1/gpu/instances/batch \
  -H "X-API-Key: $GPUPROXY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "vastai_count": 4,
    "ionet_count": 2,
    "config": {"image": "nvidia/cuda:12.0.0-base-ubuntu22.04"}
  }'
```

Default: 1 GPU from each provider if counts not specified.
Max: 8 GPUs per provider (16 total).

### 5. Export Account Data
```bash
curl -H "X-API-Key: $GPUPROXY_API_KEY" \
  http://localhost:8080/api/v1/user/export > account-export.json
```

## Database Options

### PostgreSQL (Default)
```env
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
DB_NAME=gpuproxy
```

### SQLite
```env
DB_TYPE=sqlite
DB_NAME=gpuproxy.db
```

## Session Storage

Choose session storage mode:

```env
# Store in both SQL and Redis (recommended)
SESSION_MODE=balanced

# SQL only
SESSION_MODE=sql

# Redis only
SESSION_MODE=redis
```

## Developer Mode

```bash
# Enable developer and debug mode
./bin/aiserve-gpuproxyd -dv -dm

# Client with dev mode
./bin/aiserve-gpuproxy-client -key $GPUPROXY_API_KEY -dv -dm list
```

## Monitoring

### Health Check
```bash
curl http://localhost:8080/health
```

### View Stats
```bash
./bin/aiserve-gpuproxy-admin stats
```

### View Logs
```bash
make docker-logs
```

## Common Tasks

### List Users
```bash
./bin/aiserve-gpuproxy-admin users
```

### Check Usage
```bash
./bin/aiserve-gpuproxy-admin usage user@example.com
```

### Run Migrations
```bash
./bin/aiserve-gpuproxy-admin migrate
```

## Load Balancing

**Set Strategy:**
```bash
./bin/aiserve-gpuproxy-client -key $KEY lb-strategy least_connections
```

**View Load:**
```bash
./bin/aiserve-gpuproxy-client -key $KEY load
```

**Reserve Multiple GPUs:**
```bash
./bin/aiserve-gpuproxy-client -key $KEY reserve 8
```

## Endpoints

- `GET /health` - Health check
- `POST /api/v1/auth/register` - Register user
- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/apikey` - Create API key
- `GET /api/v1/user/export` - Export account data
- `GET /api/v1/gpu/instances` - List GPUs
- `POST /api/v1/gpu/instances/batch` - Batch create instances
- `POST /api/v1/gpu/instances/reserve` - Reserve 1-16 GPUs
- `POST /api/v1/gpu/instances/{provider}/{id}` - Create instance
- `DELETE /api/v1/gpu/instances/{provider}/{id}` - Destroy instance
- `POST /api/v1/gpu/proxy` - Proxy request (HTTP/HTTPS/MCP/OI)
- `GET /api/v1/loadbalancer/loads` - Get all instance loads
- `GET /api/v1/loadbalancer/strategy` - Get LB strategy
- `PUT /api/v1/loadbalancer/strategy` - Set LB strategy
- `POST /api/v1/billing/payment` - Create payment
- `GET /api/v1/billing/transactions` - Get transactions
- `WS /ws` - WebSocket streaming

## Admin Dashboard

Access at: http://localhost:8080/ (when in developer mode)

## Support

- README.md - Full documentation
- INTEGRATION.md - Integration guide
- GitHub: https://github.com/straticus1/aiserve-gpuproxyd
