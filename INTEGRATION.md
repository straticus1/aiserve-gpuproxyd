# Integration Guide

GPU Proxy is designed for easy integration with upstream and downstream systems. This guide covers various integration scenarios.

## API Integration

### RESTful API

All endpoints follow RESTful conventions and return JSON responses.

**Base URL**: `http://your-server:8080/api/v1`

**Authentication**: Include either:
- `X-API-Key: your-api-key` header
- `Authorization: Bearer jwt-token` header

### Response Format

Success responses:
```json
{
  "data": { ... },
  "count": 10,
  "status": "success"
}
```

Error responses:
```json
{
  "error": "Error message",
  "code": "ERROR_CODE"
}
```

## Upstream System Integration

### Consuming GPU Proxy as a Service

#### Python Example
```python
import requests

class GPUProxyClient:
    def __init__(self, api_url, api_key):
        self.api_url = api_url
        self.api_key = api_key
        self.headers = {"X-API-Key": api_key}

    def list_gpus(self, provider="all"):
        url = f"{self.api_url}/gpu/instances"
        params = {"provider": provider}
        response = requests.get(url, headers=self.headers, params=params)
        return response.json()

    def create_instance(self, provider, instance_id, config=None):
        url = f"{self.api_url}/gpu/instances/{provider}/{instance_id}"
        response = requests.post(url, headers=self.headers, json=config or {})
        return response.json()

    def proxy_request(self, protocol, target_url, method="POST", body=None):
        url = f"{self.api_url}/gpu/proxy"
        payload = {
            "protocol": protocol,
            "target_url": target_url,
            "method": method,
            "body": body
        }
        response = requests.post(url, headers=self.headers, json=payload)
        return response.json()

client = GPUProxyClient("http://localhost:8080/api/v1", "your-api-key")
gpus = client.list_gpus()
print(f"Found {gpus['count']} GPUs")
```

#### Node.js Example
```javascript
const axios = require('axios');

class GPUProxyClient {
  constructor(apiUrl, apiKey) {
    this.apiUrl = apiUrl;
    this.client = axios.create({
      baseURL: apiUrl,
      headers: { 'X-API-Key': apiKey }
    });
  }

  async listGPUs(provider = 'all') {
    const response = await this.client.get('/gpu/instances', {
      params: { provider }
    });
    return response.data;
  }

  async createInstance(provider, instanceId, config = {}) {
    const response = await this.client.post(
      `/gpu/instances/${provider}/${instanceId}`,
      config
    );
    return response.data;
  }

  async proxyRequest(protocol, targetUrl, method = 'POST', body = null) {
    const response = await this.client.post('/gpu/proxy', {
      protocol,
      target_url: targetUrl,
      method,
      body
    });
    return response.data;
  }
}

const client = new GPUProxyClient('http://localhost:8080/api/v1', 'your-api-key');
const gpus = await client.listGPUs();
console.log(`Found ${gpus.count} GPUs`);
```

## Downstream System Integration

### Webhook Support

Configure webhooks to receive events from GPU Proxy:

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-system.com/webhook",
    "events": ["instance.created", "instance.destroyed", "payment.completed"],
    "secret": "webhook-secret-for-verification"
  }'
```

Webhook payload format:
```json
{
  "event": "instance.created",
  "timestamp": "2024-01-12T10:30:00Z",
  "data": {
    "instance_id": "vast-12345",
    "provider": "vast.ai",
    "user_id": "uuid",
    "gpu_model": "RTX 4090"
  },
  "signature": "hmac-sha256-signature"
}
```

### Data Export

#### Export User Account Data
```bash
curl -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/api/v1/user/export > account-data.json
```

The export includes:
- User profile
- API keys (without secrets)
- Usage quotas
- GPU usage history
- Transaction history
- Credit history

#### Bulk Export (Admin Only)
```bash
curl -H "Authorization: Bearer ADMIN_JWT" \
  http://localhost:8080/api/v1/admin/export/users > users-export.json
```

### Database Access

For direct database integration:

**PostgreSQL Connection String**:
```
postgresql://user:password@host:5432/gpuproxy?sslmode=disable
```

**SQLite Path**:
```
./gpuproxy.db
```

#### Schema Access

Key tables:
- `users` - User accounts
- `api_keys` - API authentication
- `gpu_usage` - GPU usage logs
- `billing_transactions` - Payment records
- `credits` - Credit tracking

Example query:
```sql
SELECT
  u.email,
  SUM(g.duration) as total_hours,
  SUM(g.cost) as total_cost
FROM users u
JOIN gpu_usage g ON u.id = g.user_id
WHERE g.start_time > NOW() - INTERVAL '30 days'
GROUP BY u.email;
```

## Protocol Support

### HTTP/HTTPS Proxy

```python
proxy_request = {
    "protocol": "https",
    "target_url": "https://api.example.com/endpoint",
    "method": "POST",
    "headers": {"Content-Type": "application/json"},
    "body": {"data": "value"}
}

response = client.proxy_request(**proxy_request)
```

### Model Context Protocol (MCP)

```python
mcp_request = {
    "protocol": "mcp",
    "target_url": "https://mcp.example.com",
    "body": {
        "method": "inference",
        "params": {
            "model": "llama-2",
            "prompt": "Hello, world!",
            "max_tokens": 100
        }
    }
}

response = client.proxy_request(**mcp_request)
```

### Open Inference Protocol

```python
oi_request = {
    "protocol": "openinference",
    "target_url": "https://inference.example.com",
    "body": {
        "model": "gpt-4",
        "messages": [
            {"role": "user", "content": "Hello!"}
        ],
        "temperature": 0.7,
        "max_tokens": 150
    }
}

response = client.proxy_request(**oi_request)
```

## WebSocket Streaming

For real-time updates:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'subscribe',
    channels: ['gpu_updates', 'billing_events']
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data.type, data.payload);

  switch(data.type) {
    case 'gpu_available':
      // Handle new GPU availability
      break;
    case 'payment_completed':
      // Handle payment completion
      break;
  }
};
```

## Monitoring & Observability

### Health Check Endpoint

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok",
  "timestamp": "2024-01-12T10:30:00Z",
  "version": "1.0.0",
  "services": {
    "database": "healthy",
    "redis": "healthy",
    "vast_ai": "healthy",
    "io_net": "healthy"
  }
}
```

### Metrics Endpoint (Prometheus Compatible)

```bash
curl http://localhost:8080/metrics
```

Key metrics:
- `gpuproxy_requests_total` - Total API requests
- `gpuproxy_active_instances` - Active GPU instances
- `gpuproxy_latency_seconds` - Request latency
- `gpuproxy_errors_total` - Error count

### Logging

Configure log output:

```bash
export LOG_LEVEL=debug
export LOG_FORMAT=json
./bin/aiserve-gpuproxyd
```

JSON log format:
```json
{
  "level": "info",
  "timestamp": "2024-01-12T10:30:00Z",
  "message": "Instance created",
  "user_id": "uuid",
  "instance_id": "vast-12345",
  "provider": "vast.ai"
}
```

## Security

### API Key Management

Rotate API keys programmatically:

```bash
# Create new key
NEW_KEY=$(curl -X POST http://localhost:8080/api/v1/auth/apikey \
  -H "Authorization: Bearer JWT" \
  -d '{"name":"New Key"}' | jq -r '.api_key')

# Update upstream system with new key

# Revoke old key
curl -X DELETE http://localhost:8080/api/v1/auth/apikey/OLD_KEY_ID \
  -H "Authorization: Bearer JWT"
```

### Rate Limiting

Default: 100 requests/minute per user

Custom limits via headers:
```bash
curl -H "X-API-Key: KEY" \
     -H "X-Rate-Limit-Override: 1000" \
     http://localhost:8080/api/v1/gpu/instances
```

## Billing Integration

### Payment Webhooks

Configure payment notifications:

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/billing \
  -H "X-API-Key: KEY" \
  -d '{
    "url": "https://your-billing-system.com/webhook",
    "events": ["payment.success", "payment.failed", "subscription.renewed"]
  }'
```

### Credit System Integration

Track credits programmatically:

```python
# Get current credit balance
credits = client.get('/user/credits')
print(f"Remaining: {credits['credits_remaining']}")
print(f"Total: {credits['credits_total']}")

# Add credits
client.post('/admin/credits/add', {
    'user_id': 'uuid',
    'amount': 100.00
})
```

## Examples

### Complete Integration Example

```python
import gpuproxy

proxy = gpuproxy.Client(
    api_url='http://localhost:8080/api/v1',
    api_key='your-api-key'
)

gpus = proxy.list_gpus(provider='vast.ai', min_vram=16)
best_gpu = sorted(gpus, key=lambda x: x['price_per_hour'])[0]

instance = proxy.create_instance(
    provider='vast.ai',
    instance_id=best_gpu['id'],
    config={'image': 'nvidia/cuda:12.0.0-base-ubuntu22.04'}
)

result = proxy.proxy_request(
    protocol='https',
    target_url=f"http://{instance['ip']}:8000/inference",
    body={'prompt': 'Hello, AI!'}
)

proxy.destroy_instance('vast.ai', instance['contract_id'])
```

## Support

For integration assistance:
- GitHub Issues: https://github.com/aiserve/gpuproxy/issues
- Documentation: https://github.com/aiserve/gpuproxy/wiki
- Discord: https://discord.gg/gpuproxy
