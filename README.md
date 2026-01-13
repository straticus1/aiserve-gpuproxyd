# GPU Proxy

A high-performance GPU proxy service that aggregates vast.ai and io.net GPU farms with support for multiple protocols, payment methods, and comprehensive management tools.

## Features

- **Multi-Provider GPU Access**: Seamlessly access GPUs from vast.ai and io.net
- **Load Balancing**: 5 strategies (Round Robin, Equal Weighted, Weighted Round Robin, Least Connections, Least Response Time)
- **GPU Reservation**: Reserve up to 16 GPUs at once with automatic load balancing
- **Protocol Support**: HTTP/HTTPS, MCP (Model Context Protocol), and Open Inference Protocol
- **Multiple Databases**: PostgreSQL or SQLite support
- **Flexible Session Management**: Redis, SQL, or balanced mode
- **Authentication**: JWT tokens and API keys
- **Payment Integration**: Stripe, Crypto, and AfterDark billing
- **WebSocket Streaming**: Real-time GPU inference streaming
- **Credit System**: Track usage, quotas, and credits per client
- **Rate Limiting**: Configurable per-user rate limits
- **CLI Tools**: Advanced client with load monitoring and admin utility
- **Developer Mode**: Enhanced debugging and development features

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
│   ├── server/     # Main server
│   ├── client/     # CLI client
│   └── admin/      # Admin utility
├── internal/
│   ├── api/        # API handlers
│   ├── auth/       # Authentication
│   ├── billing/    # Payment processing
│   ├── config/     # Configuration
│   ├── database/   # Database layer
│   ├── gpu/        # GPU management
│   ├── middleware/ # HTTP middleware
│   └── models/     # Data models
├── pkg/
│   ├── vastai/     # vast.ai client
│   └── ionet/      # io.net client
└── web/
    └── admin/      # Admin dashboard
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

See [LOADBALANCING.md](LOADBALANCING.md) for detailed guide.

## Environment Variables

See `.env.example` for full configuration options.

Key variables:
- `DB_HOST`, `DB_PORT`, `DB_NAME` - Database config
- `REDIS_HOST`, `REDIS_PORT` - Redis config
- `SESSION_MODE` - Session storage mode
- `JWT_SECRET` - JWT signing key
- `VASTAI_API_KEY` - vast.ai API key
- `IONET_API_KEY` - io.net API key
- `STRIPE_SECRET_KEY` - Stripe integration
- `AFTERDARK_API_KEY` - AfterDark billing
- `CRYPTO_ENABLED` - Enable crypto payments
- `LB_STRATEGY` - Load balancing strategy
- `LB_ENABLED` - Enable load balancing

## License

MIT

## Contributing

Contributions welcome! Please submit pull requests or open issues.

## Support

For issues and questions:
- GitHub Issues: https://github.com/straticus1/aiserve-gpuproxyd/issues
- Documentation: https://github.com/straticus1/aiserve-gpuproxyd/wiki
