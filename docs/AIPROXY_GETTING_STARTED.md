# AIProxy: Getting Started Guide

## Overview

AIProxy is a revolutionary AI workload orchestration system built into **aiserve-gpuproxyd** that intelligently routes inference requests across:

- **Local GPUs** (ONNX, PyTorch, GoLearn)
- **Cloudflare Workers AI** (50+ edge models)
- **OpenAI** (GPT-4, etc.)
- **Anthropic** (Claude, etc.)
- **Mesh peers** (federated deployment)

**Key Benefits:**
- ✅ **Cost optimization**: Automatically use cheapest provider
- ✅ **High availability**: Automatic failover across providers
- ✅ **Low latency**: Route to nearest/fastest provider
- ✅ **OpenAI-compatible**: Drop-in replacement for OpenAI API
- ✅ **Vendor independence**: One API for all providers

## Quick Start

### 1. Install Dependencies

```bash
cd /Users/ryan/development/aiserve-gpuproxyd
go get gopkg.in/yaml.v3
```

### 2. Get Cloudflare Credentials

Sign up for Cloudflare Workers AI and get your credentials:

1. Go to https://dash.cloudflare.com/
2. Get your **Account ID** from the URL or dashboard
3. Create an API token with **Workers AI** permissions
4. Save credentials to environment variables:

```bash
export CLOUDFLARE_ACCOUNT_ID="your-account-id"
export CLOUDFLARE_API_TOKEN="your-api-token"
```

### 3. Create Configuration

Create `aiproxy.yaml`:

```yaml
node:
  id: "my-node"
  type: "standalone"

server:
  listen_addr: "0.0.0.0:8080"

providers:
  cloudflare:
    enabled: true
    priority: 1
    credentials:
      account_id: "${CLOUDFLARE_ACCOUNT_ID}"
      api_token: "${CLOUDFLARE_API_TOKEN}"
    models:
      - name: "llama-3.1-8b"
        cloudflare_model: "@cf/meta/llama-3.1-8b-instruct"
        capabilities: ["text-generation"]
        cost_per_1k_tokens: 0.001

routing:
  strategy: "cost_optimized"
  failover:
    enabled: true
    max_retries: 3
```

### 4. Start AIProxy Server

```bash
./aiserve-gpuproxyd --aiproxy-config aiproxy.yaml
```

### 5. Make Your First Request

**Using cURL:**

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3.1-8b",
    "messages": [
      {"role": "user", "content": "Hello! Tell me about AI."}
    ]
  }'
```

**Using Python:**

```python
import openai

# Point to your AIProxy server
client = openai.OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="not-needed"  # Optional if auth disabled
)

response = client.chat.completions.create(
    model="llama-3.1-8b",
    messages=[
        {"role": "user", "content": "Hello! Tell me about AI."}
    ]
)

print(response.choices[0].message.content)
print(f"Provider: {response.x_aiproxy['provider']}")
print(f"Cost: ${response.x_aiproxy['cost']:.6f}")
```

**Using Node.js:**

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8080/v1',
  apiKey: 'not-needed'
});

const response = await client.chat.completions.create({
  model: 'llama-3.1-8b',
  messages: [
    { role: 'user', content: 'Hello! Tell me about AI.' }
  ]
});

console.log(response.choices[0].message.content);
console.log('Provider:', response.x_aiproxy.provider);
console.log('Cost: $' + response.x_aiproxy.cost);
```

## Available Cloudflare Models

### Text Generation

| Model Name | Model ID | Capabilities | Cost/1K tokens |
|------------|----------|--------------|----------------|
| `llama-3.1-8b` | `@cf/meta/llama-3.1-8b-instruct` | Text generation | $0.001 |
| `llama-4-scout` | `@cf/meta/llama-4-scout-17b-16e` | Text + Vision | $0.002 |
| `qwen-coder` | `@cf/qwen/qwen2.5-coder-7b-instruct` | Code generation | $0.001 |
| `qwq-reasoning` | `@cf/qwen/qwq-32b-preview` | Reasoning | $0.003 |

### Image Generation

| Model Name | Model ID | Cost/generation |
|------------|----------|-----------------|
| `stable-diffusion-xl` | `@cf/stabilityai/stable-diffusion-xl-base-1.0` | $0.01 |

See [Cloudflare Workers AI Models](https://developers.cloudflare.com/workers-ai/models/) for complete list.

## Routing Strategies

### Cost Optimized (Default)

Routes to cheapest provider that meets requirements:

```yaml
routing:
  strategy: "cost_optimized"
```

**Use case**: Minimize costs while maintaining quality

### Latency Optimized

Routes to fastest provider based on historical latency:

```yaml
routing:
  strategy: "latency_optimized"
```

**Use case**: Real-time applications, chatbots

### Availability

Routes to highest-priority available provider:

```yaml
routing:
  strategy: "availability"
```

**Use case**: Guaranteed uptime, critical systems

### Round Robin

Distributes requests evenly across providers:

```yaml
routing:
  strategy: "round_robin"
```

**Use case**: Load testing, even distribution

## Advanced Configuration

### Multi-Provider Setup

```yaml
providers:
  # Free local inference
  local:
    enabled: true
    priority: 1
    models:
      - name: "llama-3-8b-local"
        path: "/models/llama-3-8b.onnx"
        cost_per_1k_tokens: 0.0

  # Low-cost edge inference
  cloudflare:
    enabled: true
    priority: 2
    credentials:
      account_id: "${CLOUDFLARE_ACCOUNT_ID}"
      api_token: "${CLOUDFLARE_API_TOKEN}"
    models:
      - name: "llama-3.1-8b"
        cloudflare_model: "@cf/meta/llama-3.1-8b-instruct"
        cost_per_1k_tokens: 0.001

  # High-quality fallback
  openai:
    enabled: true
    priority: 3
    credentials:
      api_key: "${OPENAI_API_KEY}"
    models:
      - name: "gpt-4o"
        openai_model: "gpt-4o"
        cost_per_1k_tokens: 0.005

routing:
  strategy: "cost_optimized"
  failover:
    enabled: true
    fallback_chain:
      - "local"         # Try free local first
      - "cloudflare"    # Fallback to edge
      - "openai"        # Last resort
```

**Cost savings**: This setup prioritizes free local compute, falls back to low-cost Cloudflare ($0.001/1K tokens), and only uses OpenAI ($0.005/1K tokens) if others fail.

### Budget Controls

```yaml
budget:
  enabled: true
  daily_limit: 50.00      # USD per day
  monthly_limit: 1000.00

  alerts:
    - threshold: 80  # Alert at 80% of limit
      action: "log"
    - threshold: 95  # Block paid providers at 95%
      action: "disable_paid_providers"
```

### Security

```yaml
security:
  auth:
    enabled: true
    type: "api_key"
    api_keys:
      - key: "${AIPROXY_API_KEY}"
        name: "production"
        rate_limit: 1000  # requests/hour

  rate_limiting:
    enabled: true
    default_limit: 100
    by_ip: true
```

Then use API key in requests:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "llama-3.1-8b", "messages": [...]}'
```

## Federated Deployment (Mesh Mode)

Deploy AIProxy across multiple machines for distributed load balancing:

**Node 1 (GPU server):**

```yaml
node:
  id: "gpu-node-01"
  type: "gpu"
  mesh:
    enabled: true
    listen_addr: "0.0.0.0:9090"
    peers:
      - id: "edge-node-01"
        addr: "edge-node-01.local:9090"

providers:
  local:
    enabled: true
    priority: 1
```

**Node 2 (Edge server):**

```yaml
node:
  id: "edge-node-01"
  type: "edge"
  mesh:
    enabled: true
    listen_addr: "0.0.0.0:9090"
    peers:
      - id: "gpu-node-01"
        addr: "gpu-node-01.local:9090"

providers:
  cloudflare:
    enabled: true
    priority: 1
```

**Result**: Requests automatically route between nodes based on load and availability.

## Monitoring

### Metrics Endpoint

Enable Prometheus metrics:

```yaml
observability:
  metrics:
    enabled: true
    prometheus_port: 9091
    collect:
      - request_count
      - request_duration
      - provider_errors
      - cost_per_request
```

Access metrics:

```bash
curl http://localhost:9091/metrics
```

### Health Check

```bash
curl http://localhost:8080/health
```

### Provider Status

```bash
curl http://localhost:8080/aiproxy/status
```

Response:

```json
{
  "node": {
    "id": "my-node",
    "type": "standalone",
    "uptime": 3600
  },
  "providers": {
    "cloudflare": {
      "status": "healthy",
      "models": 4,
      "requests": 1523,
      "errors": 3,
      "avg_latency_ms": 245
    }
  },
  "stats": {
    "total_requests": 1523,
    "total_cost": 1.234
  }
}
```

## Use Cases

### 1. Cost-Optimized Development

**Scenario**: Developer wants to test locally, fallback to cloud only when needed

```yaml
providers:
  local:
    enabled: true
    priority: 1
  cloudflare:
    enabled: true
    priority: 999  # Only if local fails

routing:
  strategy: "availability"
```

### 2. Global Low-Latency Service

**Scenario**: Serve users worldwide with lowest latency

**Setup**: Deploy AIProxy in multiple regions:
- US: Local GPU + Cloudflare US edge
- EU: Local GPU + Cloudflare EU edge
- Asia: Cloudflare Asia edge

```yaml
routing:
  strategy: "latency_optimized"
```

### 3. Budget-Constrained Startup

**Scenario**: $100/month budget, need to maximize free tier

```yaml
budget:
  enabled: true
  daily_limit: 3.30  # $100/month ÷ 30 days
  alerts:
    - threshold: 90
      action: "disable_paid_providers"

providers:
  local:
    enabled: true
    priority: 1  # Free!
  cloudflare:
    enabled: true
    priority: 2  # $0.001/1K tokens
  openai:
    enabled: false  # Disabled to save money
```

### 4. High-Availability Production

**Scenario**: Cannot tolerate downtime

```yaml
routing:
  failover:
    enabled: true
    max_retries: 5
    fallback_chain:
      - "local"
      - "cloudflare"
      - "openai"
      - "anthropic"

node:
  mesh:
    enabled: true
    peers: [node1, node2, node3]  # 3-node HA cluster
```

## Troubleshooting

### Error: "cloudflare.credentials.account_id is required"

Make sure environment variables are set:

```bash
export CLOUDFLARE_ACCOUNT_ID="your-account-id"
export CLOUDFLARE_API_TOKEN="your-api-token"
```

Verify they're loaded:

```bash
echo $CLOUDFLARE_ACCOUNT_ID
```

### Error: "no available providers for model"

Check that:
1. Provider is enabled in config
2. Model is listed in provider's models
3. Provider credentials are valid

### High latency

Try latency-optimized routing:

```yaml
routing:
  strategy: "latency_optimized"
```

Or enable local caching (future feature).

## Next Steps

1. ✅ Read the [AIProxy Standard](AIPROXY_STANDARD.md)
2. ✅ Explore [example configuration](../configs/aiproxy-example.yaml)
3. ✅ Set up [budget alerts](#budget-controls)
4. ✅ Enable [monitoring](#monitoring)
5. ✅ Deploy in [mesh mode](#federated-deployment-mesh-mode)

## Resources

- **Cloudflare Workers AI**: https://developers.cloudflare.com/workers-ai/
- **OpenAI API**: https://platform.openai.com/docs/api-reference
- **AIProxy Standard**: https://github.com/aiserve/aiproxy-standard

---

**Support**: https://github.com/aiserve/aiserve-gpuproxyd/issues
**License**: MIT
