# GPU Proxy

A high-performance GPU proxy service that aggregates vast.ai and io.net GPU farms with support for multiple protocols, payment methods, and comprehensive management tools.

## Features

- **Multi-Provider GPU Access**: Seamlessly access GPUs from vast.ai and io.net
- **ML Runtime Support**: ONNX, PyTorch, TensorFlow, scikit-learn model serving with 4 runtimes
- **OCI Object Storage**: Native Oracle Cloud Infrastructure storage integration (replaces AWS S3)
- **Storage Quotas**: Per-user storage limits with hourly/daily upload rate limiting
- **Load Balancing**: 5 strategies (Round Robin, Equal Weighted, Weighted Round Robin, Least Connections, Least Response Time)
- **GPU Reservation**: Reserve up to 16 GPUs at once with automatic load balancing
- **Protocol Support**: HTTP/HTTPS, gRPC, MCP (Model Context Protocol), and Open Inference Protocol
- **Multiple Databases**: PostgreSQL or SQLite support with PgBouncer connection pooling
- **Flexible Session Management**: Redis, SQL, or balanced mode
- **Authentication**: JWT tokens and API keys with bcrypt hashing
- **Payment Integration**: Stripe, Crypto, and AfterDark billing
- **WebSocket Streaming**: Real-time GPU inference streaming
- **Credit System**: Track usage, quotas, and credits per client
- **Rate Limiting**: Configurable per-user rate limits
- **Guard Rails**: Spending control across 17 time windows (5min to 72h)
- **Agent Protocols**: MCP, A2A, ACP, FIPA ACL, KQML, and LangChain support
- **Centralized Logging**: Syslog support with file logging and AISERVE_LOG_FILE
- **CLI Tools**: Advanced client with load monitoring and admin utility
- **Developer Mode**: Enhanced debugging and development features
- **Production Hardened**: Memory safety fixes, goroutine lifecycle management, panic recovery

## Architecture

```
┌─────────────┐
│   Client    │
│  CLI/API    │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────┐
│         GPU Proxy Server            │
│  ┌─────────────────────────────┐   │
│  │  API Handlers               │   │
│  │  - Auth, GPU, Billing       │   │
│  └─────────────────────────────┘   │
│  ┌─────────────────────────────┐   │
│  │  Protocol Support           │   │
│  │  - HTTP/HTTPS, MCP, OI      │   │
│  └─────────────────────────────┘   │
│  ┌─────────────────────────────┐   │
│  │  Provider Integrations      │   │
│  │  - vast.ai, io.net          │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
       │              │
       ▼              ▼
┌────────────┐  ┌──────────┐
│ PostgreSQL │  │  Redis   │
│  or SQLite │  │          │
└────────────┘  └──────────┘
```

## Binaries

After building, you'll have three binaries:

- **aiserve-gpuproxyd** - Main GPU proxy server daemon
- **aiserve-gpuproxy-client** - CLI client for interacting with the API
- **aiserve-gpuproxy-admin** - Administrative utility for managing users, migrations, etc.

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 15+ or SQLite3
- Redis 7+
- Docker (optional)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/straticus1/aiserve-gpuproxyd.git
cd aiserve-gpuproxyd
```

2. Copy environment file:
```bash
cp .env.example .env
```

3. Edit `.env` with your configuration:
```bash
# Required
JWT_SECRET=your-secure-random-string
VASTAI_API_KEY=your-vast-api-key
IONET_API_KEY=your-ionet-api-key
```

4. Build the project:
```bash
make build
```

5. Run database migrations:
```bash
./bin/aiserve-gpuproxy-admin migrate
```

6. Start the server:
```bash
./bin/aiserve-gpuproxyd
```

### Using Docker

```bash
# Start all services
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

## Configuration

### Database Options

Choose between PostgreSQL or SQLite:

**PostgreSQL** (.env):
```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=changeme
DB_NAME=gpuproxy
```

**SQLite** (.env):
```env
DB_TYPE=sqlite
DB_NAME=gpuproxy.db
```

### Session Management

Configure session storage mode:

- `sql` - Store sessions in database only
- `redis` - Store sessions in Redis only
- `balanced` - Store sessions in both (recommended)

```env
SESSION_MODE=balanced
```

### Developer & Debug Modes

Run with enhanced logging and features:

```bash
# Developer mode
./bin/aiserve-gpuproxyd -dv
./bin/aiserve-gpuproxyd --developer-mode

# Debug mode
./bin/aiserve-gpuproxyd -dm
./bin/aiserve-gpuproxyd --debug-mode

# Both
./bin/aiserve-gpuproxyd -dv -dm
```

## API Usage

### Authentication

#### Register a user
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "secure-password",
    "name": "John Doe"
  }'
```

#### Login
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "secure-password"
  }'
```

#### Create API Key
```bash
curl -X POST http://localhost:8080/api/v1/auth/apikey \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My API Key"
  }'
```

### GPU Management

#### List available GPUs
```bash
# All providers
curl -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/api/v1/gpu/instances

# Specific provider
curl -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/api/v1/gpu/instances?provider=vast.ai

# With filters
curl -H "X-API-Key: YOUR_API_KEY" \
  "http://localhost:8080/api/v1/gpu/instances?min_vram=16&max_price=1.5"
```

#### Create GPU instance
```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"image": "nvidia/cuda:12.0.0-base-ubuntu22.04"}' \
  http://localhost:8080/api/v1/gpu/instances/vast.ai/12345
```

#### Destroy GPU instance
```bash
curl -X DELETE \
  -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/api/v1/gpu/instances/vast.ai/12345
```

### Protocol Proxy

#### HTTP/HTTPS Request
```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "https",
    "target_url": "https://api.example.com/inference",
    "method": "POST",
    "headers": {"Content-Type": "application/json"},
    "body": {"prompt": "Hello world"}
  }' \
  http://localhost:8080/api/v1/gpu/proxy
```

#### MCP Request
```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "mcp",
    "target_url": "https://mcp.example.com",
    "body": {
      "method": "inference",
      "params": {"model": "llama-2", "prompt": "Hello"}
    }
  }' \
  http://localhost:8080/api/v1/gpu/proxy
```

#### Open Inference Request
```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "openinference",
    "target_url": "https://inference.example.com",
    "body": {
      "model": "gpt-4",
      "prompt": "Hello world",
      "max_tokens": 100
    }
  }' \
  http://localhost:8080/api/v1/gpu/proxy
```

### Payment & Billing

#### Create payment
```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "X-Preferred-Payment: card:4242424242424242:12/25:123" \
  -H "X-Billing: Stripe" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 50.00,
    "currency": "USD",
    "provider": "stripe"
  }' \
  http://localhost:8080/api/v1/billing/payment
```

#### Payment with crypto
```bash
curl -X POST \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "X-Preferred-Payment: crypto:ethereum:0x123..." \
  -H "X-Billing: Crypto" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 50.00,
    "currency": "USD",
    "provider": "crypto"
  }' \
  http://localhost:8080/api/v1/billing/payment
```

#### Get transaction history
```bash
curl -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/api/v1/billing/transactions
```

### Custom Headers

The API supports several custom headers:

- `X-API-Key` - Your API key for authentication
- `X-Preferred-Payment` - Payment preference
  - Card: `card:num:expr:ccv`
  - Crypto: `crypto:network:wallet`
- `X-Timelimit` - Maximum execution time
- `X-Billing` - Billing provider (AfterDark, Stripe, Crypto)

## CLI Client

### List GPUs
```bash
# All providers
./bin/aiserve-gpuproxy-client -key YOUR_API_KEY list

# Specific provider
./bin/aiserve-gpuproxy-client -key YOUR_API_KEY list vast.ai
./bin/aiserve-gpuproxy-client -key YOUR_API_KEY list io.net
```

### Create Instance
```bash
./bin/aiserve-gpuproxy-client -key YOUR_API_KEY create vast.ai 12345
```

### Destroy Instance
```bash
./bin/aiserve-gpuproxy-client -key YOUR_API_KEY destroy vast.ai 12345
```

### Proxy Request
```bash
./bin/aiserve-gpuproxy-client -key YOUR_API_KEY proxy https https://api.example.com
```

### Developer Mode
```bash
./bin/aiserve-gpuproxy-client -key YOUR_API_KEY -dv -dm list
```

## Admin Utility

### List Users
```bash
./bin/aiserve-gpuproxy-admin users
```

### Create User
```bash
./bin/aiserve-gpuproxy-admin create-user user@example.com password123 "John Doe"
```

### Make Admin
```bash
./bin/aiserve-gpuproxy-admin make-admin user@example.com
```

### Create API Key
```bash
./bin/aiserve-gpuproxy-admin create-apikey user@example.com "Production Key"
```

### View Usage
```bash
./bin/aiserve-gpuproxy-admin usage user@example.com
```

### System Stats
```bash
./bin/aiserve-gpuproxy-admin stats
```

### Run Migrations
```bash
./bin/aiserve-gpuproxy-admin migrate
```

## gRPC API

The GPU Proxy also provides a high-performance gRPC API for all operations.

### Configuration

Set the gRPC port in `.env`:
```env
GRPC_PORT=9090
```

### Available Services

All HTTP API operations are available via gRPC:
- Authentication (Login, CreateAPIKey)
- GPU Management (List, Create, Destroy, Get)
- Proxy Requests (Unary and Streaming)
- Billing (CreatePayment, GetTransactions)
- Guard Rails (GetSpendingInfo, CheckSpendingLimit)
- Load Balancing (SetStrategy, GetLoadInfo, ReserveGPUs)
- Health Check

### Protocol Buffers

See `proto/gpuproxy.proto` for the complete service definition.

### Example: Go Client

```go
package main

import (
    "context"
    "log"

    pb "github.com/aiserve/gpuproxy/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
)

func main() {
    conn, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewGPUProxyServiceClient(conn)

    // Login
    loginResp, err := client.Login(context.Background(), &pb.LoginRequest{
        Email:    "user@example.com",
        Password: "password",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Use token for authenticated requests
    ctx := metadata.AppendToOutgoingContext(
        context.Background(),
        "authorization", "Bearer "+loginResp.Token,
    )

    // List GPU instances
    instances, err := client.ListGPUInstances(ctx, &pb.ListGPUInstancesRequest{
        Provider: "all",
        MinVram:  16,
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found %d instances", instances.TotalCount)

    // Reserve 4 GPUs with automatic creation
    reserved, err := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
        Count:    4,
        Provider: "vast.ai",
        MinVram:  16,
        MaxPrice: 2.0,
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Reserved %d GPUs:", reserved.ReservedCount)
    for _, inst := range reserved.ReservedInstances {
        contractID := inst.Metadata["contract_id"]
        log.Printf("  - %s: %s (Contract: %s, $%.2f/hr)",
            inst.GpuModel, inst.Id, contractID, inst.PricePerHour)
    }
}
```

### Example: Python Client

```python
import grpc
from proto import gpuproxy_pb2, gpuproxy_pb2_grpc

# Connect to server
channel = grpc.insecure_channel('localhost:9090')
stub = gpuproxy_pb2_grpc.GPUProxyServiceStub(channel)

# Login
login_response = stub.Login(gpuproxy_pb2.LoginRequest(
    email="user@example.com",
    password="password"
))

# Create metadata with token
metadata = [('authorization', f'Bearer {login_response.token}')]

# List GPU instances
instances = stub.ListGPUInstances(
    gpuproxy_pb2.ListGPUInstancesRequest(
        provider="all",
        min_vram=16
    ),
    metadata=metadata
)

print(f"Found {instances.total_count} instances")
```

### Streaming Proxy Requests

gRPC supports bidirectional streaming for real-time inference:

```go
stream, err := client.StreamProxyRequest(ctx)
if err != nil {
    log.Fatal(err)
}

// Send requests
go func() {
    for i := 0; i < 10; i++ {
        stream.Send(&pb.ProxyRequestMessage{
            Protocol:  "https",
            TargetUrl: "https://api.example.com/inference",
            Method:    "POST",
            Body:      []byte(`{"prompt": "Hello"}`),
        })
    }
    stream.CloseSend()
}()

// Receive responses
for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Response: %d - %s", resp.StatusCode, resp.Body)
}
```

## WebSocket Streaming

Connect to WebSocket for real-time updates:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'subscribe'
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data);
};
```

## Credit System

The system tracks detailed credit usage per client:

- `credits_remaining` - Available credits
- `credits_total` - Total credits allocated
- `credits_overage` - Usage beyond allocation
- `credits_cap` - Maximum allowed overage
- `credits_auto_renew` - Automatic renewal flag

Credits are tracked per session with:
- Client ID
- IP address
- Date and time
- Duration
- Full credit details

## Security

- JWT-based authentication
- API key support with bcrypt hashing
- Rate limiting per user
- CORS protection
- SQL injection prevention
- Input validation

## Performance

- Connection pooling (PostgreSQL)
- Redis caching
- Concurrent provider queries
- WebSocket for real-time streaming
- Optimized database indexes

## Monitoring

### Health Check
```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok",
  "timestamp": "2024-01-12T10:30:00Z"
}
```

## Development

### Project Structure
```
gpuproxy/
├── cmd/
│   ├── server/        # Main server
│   ├── client/        # CLI client
│   ├── admin/         # Admin utility
│   └── seed/          # Database seeding tool
├── internal/
│   ├── api/           # API handlers
│   ├── auth/          # Authentication
│   ├── billing/       # Payment processing
│   ├── config/        # Configuration
│   ├── database/      # Database layer
│   ├── gpu/           # GPU management
│   ├── ml/            # ML runtime system (NEW)
│   │   ├── onnx_runtime.go        # ONNX Runtime
│   │   ├── pytorch_converter.go   # PyTorch → ONNX
│   │   ├── golearn_runtime.go     # GoLearn
│   │   ├── gomlx_runtime.go       # GoMLX (GPU)
│   │   ├── sklearn_runtime.go     # Sklearn
│   │   └── runtime_orchestrator.go # Runtime routing
│   ├── middleware/    # HTTP middleware
│   ├── models/        # Data models
│   └── storage/       # DarkStorage integration (NEW)
├── pkg/
│   ├── vastai/        # vast.ai client
│   └── ionet/         # io.net client
├── docs/              # Documentation
│   ├── ML_RUNTIME_IMPLEMENTATION.md  # ML runtime docs (NEW)
│   ├── AI_PLATFORM_ARCHITECTURE.md   # Training platform (NEW)
│   ├── GETTING_STARTED_TRAINING_PLATFORM.md  # Training guide (NEW)
│   └── HYBRID_COMPUTE_ARCHITECTURE.md  # Compute architecture (NEW)
└── web/
    └── admin/         # Admin dashboard
```

### Running Tests
```bash
make test
```

### Building
```bash
make build
```

### Clean
```bash
make clean
```

## Load Balancing

GPU Proxy includes advanced load balancing with 5 strategies:

1. **Round Robin** - Equal distribution across all GPUs
2. **Equal Weighted** - Balance based on total connections
3. **Weighted Round Robin** - Prioritize by GPU specs (VRAM, price)
4. **Least Connections** - Route to GPU with fewest active connections
5. **Least Response Time** - Route to fastest GPU

### Set Strategy
```bash
# Via CLI
./bin/aiserve-gpuproxy-client -key $KEY lb-strategy least_connections

# Via API
curl -X PUT -H "X-API-Key: $KEY" \
  -d '{"strategy": "least_connections"}' \
  http://localhost:8080/api/v1/loadbalancer/strategy
```

### View Load
```bash
# Server load (tracked instances)
./bin/aiserve-gpuproxy-client -key $KEY load server

# Provider load (available instances)
./bin/aiserve-gpuproxy-client -key $KEY load provider

# All load info
./bin/aiserve-gpuproxy-client -key $KEY load
```

### Reserve GPUs
```bash
# Reserve up to 16 GPUs with automatic load balancing
./bin/aiserve-gpuproxy-client -key $KEY reserve 16
```

See [GPU_RESERVATIONS.md](GPU_RESERVATIONS.md) for complete reservation guide and [LOADBALANCING.md](LOADBALANCING.md) for load balancing details.

## Guard Rails

Prevent out-of-control spending with configurable limits across multiple time windows:

```bash
# Enable guard rails with spending limits
GUARDRAILS_ENABLED=true
GUARDRAILS_MAX_60MIN_RATE=100.00    # $100/hour
GUARDRAILS_MAX_1440MIN_RATE=1000.00 # $1000/day
GUARDRAILS_MAX_72H_RATE=2500.00     # $2500/3 days
```

### Check Spending
```bash
# View spending status
curl -H "X-API-Key: $KEY" \
  http://localhost:8080/api/v1/guardrails/spending

# Admin: View user spending
./bin/aiserve-gpuproxy-admin guardrails-spending user@example.com

# Admin: Reset spending tracking
./bin/aiserve-gpuproxy-admin guardrails-reset user@example.com
```

See [GUARDRAILS.md](GUARDRAILS.md) for complete documentation.

## Model Context Protocol (MCP)

Integrate with AI assistants like Claude Desktop:

```json
{
  "mcpServers": {
    "aiserve-gpuproxy": {
      "command": "curl",
      "args": ["-X", "POST", "-H", "X-API-Key: YOUR_KEY",
               "http://localhost:8080/api/v1/mcp"]
    }
  }
}
```

### Available MCP Tools
- `list_gpu_instances` - List available GPUs
- `create_gpu_instance` - Create GPU instance
- `destroy_gpu_instance` - Destroy GPU instance
- `get_spending_info` - Check guard rails spending
- `check_spending_limit` - Validate spending limits
- `get_billing_transactions` - View transaction history
- `proxy_inference_request` - Proxy inference requests

See [MCP.md](MCP.md) for complete MCP documentation.

## Agent Communication Protocols

Support for 6 major agent protocols with auto-detection:

### A2A (Agent-to-Agent Protocol)
```bash
curl -X POST http://localhost:8080/api/v1/a2a \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "action": "gpu.list",
    "from_agent": "my-agent",
    "parameters": {"provider": "vast.ai"}
  }'
```

### ACP (Agent Communications Protocol)
```bash
curl -X POST http://localhost:8080/api/v1/acp \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "header": {
      "sender": "my-agent",
      "message_type": "command"
    },
    "payload": {
      "command": "gpu.list",
      "parameters": {"provider": "all"}
    }
  }'
```

### FIPA ACL (Foundation for Intelligent Physical Agents)
```bash
curl -X POST http://localhost:8080/api/v1/fipa \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "performative": "query-ref",
    "sender": {"name": "my-agent"},
    "receiver": [{"name": "aiserve-gpuproxy"}],
    "content": {"query": "gpu-instances"}
  }'
```

### KQML (Knowledge Query and Manipulation Language)
```bash
curl -X POST http://localhost:8080/api/v1/kqml \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "performative": "ask",
    "sender": "my-agent",
    "content": {"query": "gpu-instances"}
  }'
```

### LangChain Agent Protocol
```bash
# Get tools
curl http://localhost:8080/api/v1/langchain/tools \
  -H "X-API-Key: YOUR_KEY"

# Execute tool
curl -X POST http://localhost:8080/api/v1/langchain \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "input": {
      "action": "execute",
      "tool": "list_gpu_instances",
      "tool_input": {"provider": "all"}
    }
  }'
```

### Unified Agent Endpoint (Auto-Detection)
```bash
curl -X POST http://localhost:8080/api/v1/agent \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "action": "gpu.list",
    "from_agent": "my-agent",
    "parameters": {"provider": "vast.ai"}
  }'
```

### Agent Discovery
```bash
curl http://localhost:8080/agent/discover
```

## ML Model Serving

GPU Proxy includes a hybrid ML runtime system supporting multiple model formats:

### Supported Runtimes

1. **ONNX Runtime** (CPU + GPU)
   - Load `.onnx` models
   - CUDA acceleration support
   - Auto-optimization (graph level 99)
   - Latency: 1-10ms

2. **PyTorch Converter**
   - Automatic `.pt`/`.pth` → ONNX conversion
   - No need for native PyTorch runtime
   - Production-ready approach

3. **Sklearn Runtime** (Python bridge)
   - `.pkl` and `.joblib` models
   - Full scikit-learn support
   - Latency: 5-20ms

4. **GoLearn Runtime** (Pure Go)
   - Classical ML algorithms
   - k-NN, Decision Trees, Naive Bayes
   - Ultra-fast: 50-100μs latency

### Upload and Serve Models

```bash
# Upload PyTorch model (auto-converts to ONNX)
curl -X POST http://localhost:8080/api/v1/models/upload \
  -F "model=@my_model.pt" \
  -F "name=my-custom-model" \
  -F "format=pytorch"

# Response:
{
  "model_id": "abc-123",
  "converted_to": "onnx",
  "status": "ready"
}

# Run inference
curl -X POST http://localhost:8080/api/v1/models/abc-123/predict \
  -H "X-API-Key: YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "input": [1.0, 2.0, 3.0, 4.0]
  }'

# Response:
{
  "output": [0.92, 0.08],
  "latency_ms": 2.3,
  "runtime": "onnx",
  "used_gpu": true
}
```

### Supported Model Formats

| Format | Runtime | GPU Support | Latency |
|--------|---------|-------------|---------|
| `.onnx` | ONNX Runtime | ✅ CUDA | 1-10ms |
| `.pt`/`.pth` | PyTorch → ONNX | ✅ CUDA | 1-10ms |
| `.pkl` | Sklearn | ❌ | 5-20ms |
| `.joblib` | Sklearn | ❌ | 5-20ms |
| `.golearn` | GoLearn | ❌ | 50-100μs |

### Installation Requirements

**ONNX Runtime (for ONNX/PyTorch models):**
```bash
# macOS
wget https://github.com/microsoft/onnxruntime/releases/download/v1.17.0/onnxruntime-osx-universal2-1.17.0.tgz
tar -xzf onnxruntime-osx-universal2-1.17.0.tgz
sudo cp onnxruntime-osx-universal2-1.17.0/lib/* /usr/local/lib/

# Linux
wget https://github.com/microsoft/onnxruntime/releases/download/v1.17.0/onnxruntime-linux-x64-1.17.0.tgz
tar -xzf onnxruntime-linux-x64-1.17.0.tgz
sudo cp onnxruntime-linux-x64-1.17.0/lib/* /usr/local/lib/
sudo ldconfig
```

**PyTorch (for model conversion):**
```bash
# CPU only
pip3 install torch torchvision

# GPU (CUDA 12.1)
pip3 install torch torchvision --index-url https://download.pytorch.org/whl/cu121
```

See [ML_RUNTIME_IMPLEMENTATION.md](docs/ML_RUNTIME_IMPLEMENTATION.md) for complete documentation.

## Logging

Configure logging to syslog or file:

```bash
# Syslog to remote server
SYSLOG_ENABLED=true
SYSLOG_NETWORK=tcp
SYSLOG_ADDRESS=logs.example.com:514
SYSLOG_FACILITY=LOG_LOCAL0

# Or log to file
LOG_FILE=/var/log/aiserve-gpuproxy.log

# Or use environment variable
AISERVE_LOG_FILE=/var/log/aiserve-gpuproxy.log

# Auto-detect /dev/log
SYSLOG_ENABLED=true
SYSLOG_ADDRESS=/dev/log
```

## Environment Variables

See `.env.example` for full configuration options.

Key variables:
- `DB_HOST`, `DB_PORT`, `DB_NAME` - Database config
- `DB_USE_PGBOUNCER` - Use PgBouncer connection pooling (default: true)
- `DB_MAX_CONNS` - Maximum database connections (default: 200)
- `REDIS_HOST`, `REDIS_PORT` - Redis config
- `SESSION_MODE` - Session storage mode
- `JWT_SECRET` - JWT signing key (REQUIRED: generate with `openssl rand -base64 64`)
- `VASTAI_API_KEY` - vast.ai API key
- `IONET_API_KEY` - io.net API key
- `OCI_STORAGE_ENDPOINT` - OCI Object Storage endpoint
- `OCI_STORAGE_NAMESPACE` - OCI namespace
- `OCI_STORAGE_BUCKET` - OCI bucket name
- `OCI_ACCESS_KEY_ID` - OCI access key ID
- `OCI_SECRET_ACCESS_KEY` - OCI secret access key
- `STRIPE_SECRET_KEY` - Stripe integration
- `AFTERDARK_API_KEY` - AfterDark billing
- `CRYPTO_ENABLED` - Enable crypto payments
- `LB_STRATEGY` - Load balancing strategy
- `LB_ENABLED` - Enable load balancing
- `GUARDRAILS_ENABLED` - Enable spending guard rails
- `GUARDRAILS_MAX_*_RATE` - Spending limits per time window
- `SYSLOG_ENABLED` - Enable syslog logging
- `SYSLOG_NETWORK` - Syslog network (tcp, udp, unix)
- `SYSLOG_ADDRESS` - Syslog server address
- `LOG_FILE` / `AISERVE_LOG_FILE` - Log file path
- `GRPC_PORT` - gRPC server port (default: 9090)

## License

MIT

## Contributing

Contributions welcome! Please submit pull requests or open issues.

## Support

For issues and questions:
- GitHub Issues: https://github.com/straticus1/aiserve-gpuproxyd/issues
- Documentation: https://github.com/straticus1/aiserve-gpuproxyd/wiki
