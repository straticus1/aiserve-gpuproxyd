# AIProxy Standard v1.0
## Universal AI Workload Orchestration Protocol

**Status**: Draft Standard
**Version**: 1.0.0
**Date**: January 2026
**Authors**: AIServe.Farm Team

---

## Abstract

The AIProxy Standard defines a universal protocol for intelligent AI workload routing across heterogeneous compute resources including local GPUs, edge AI platforms (Cloudflare Workers AI), and cloud AI services. This standard enables federated AI deployment where multiple proxy nodes form a mesh network capable of dynamic load balancing, intelligent failover, and cost-optimized routing.

## 1. Problem Statement

Current AI infrastructure faces several challenges:

1. **Vendor Lock-in**: Applications tied to specific providers (OpenAI, Anthropic, etc.)
2. **Underutilized Resources**: Local GPUs sit idle while cloud costs accumulate
3. **Single Points of Failure**: No automatic failover between providers
4. **Latency Optimization**: No intelligent routing based on user geography
5. **Cost Inefficiency**: No automatic selection of cheapest provider for workload
6. **Scalability**: Difficult to add capacity dynamically

## 2. Solution: AIProxy Standard

AIProxy defines a standard for intelligent AI routing proxies that can:

- **Route dynamically** based on model, cost, latency, and availability
- **Federate seamlessly** across multiple nodes in a mesh topology
- **Fail over automatically** to backup providers or nodes
- **Load balance** across available compute resources
- **Optimize costs** by selecting cheapest provider that meets requirements
- **Support multiple protocols**: OpenAI-compatible, native REST, gRPC

## 3. Architecture

### 3.1 Components

```
┌─────────────────────────────────────────────────────────────┐
│                        AIProxy Node                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Router     │  │  Config      │  │  Health      │      │
│  │   Engine     │─▶│  Manager     │─▶│  Monitor     │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│         │                                     │              │
│         ▼                                     ▼              │
│  ┌──────────────────────────────────────────────────┐      │
│  │           Provider Backends                       │      │
│  │  ┌──────┐  ┌──────────┐  ┌────────┐  ┌────────┐ │      │
│  │  │Local │  │Cloudflare│  │ OpenAI │  │  Mesh  │ │      │
│  │  │ GPU  │  │Workers AI│  │        │  │  Peer  │ │      │
│  │  └──────┘  └──────────┘  └────────┘  └────────┘ │      │
│  └──────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Node Types

1. **Standalone Node**: Single proxy serving requests
2. **Mesh Node**: Part of federated network, can route to peers
3. **Edge Node**: Optimized for low-latency, uses edge providers
4. **GPU Node**: Primarily uses local GPU resources

### 3.3 Deployment Modes

#### Mode 1: Single Proxy (Development)
```bash
aiserve-gpuproxyd --config aiproxy.yaml
# Serves on http://localhost:8080
# Routes between local and cloud providers
```

#### Mode 2: Federated Mesh (Production)
```bash
# Node 1 (GPU-heavy)
aiserve-gpuproxyd --config aiproxy-gpu1.yaml --peers node2,node3

# Node 2 (Edge-optimized)
aiserve-gpuproxyd --config aiproxy-edge.yaml --peers node1,node3

# Node 3 (Cloud gateway)
aiserve-gpuproxyd --config aiproxy-cloud.yaml --peers node1,node2
```

Mesh nodes share:
- Load information
- Model availability
- Health status
- Cost metrics

## 4. Configuration Standard

### 4.1 Configuration File Format

AIProxy uses YAML or JSON configuration with this schema:

```yaml
# aiproxy.yaml - AIProxy Standard Configuration v1.0

# Node identity and mesh configuration
node:
  id: "gpu-node-01"
  name: "Primary GPU Node"
  region: "us-west-2"
  type: "gpu"  # gpu, edge, cloud, hybrid

  # Mesh networking (optional)
  mesh:
    enabled: true
    listen_addr: "0.0.0.0:9090"
    peers:
      - id: "edge-node-01"
        addr: "edge-node-01.local:9090"
        priority: 2
      - id: "cloud-node-01"
        addr: "cloud-node-01.local:9090"
        priority: 3

    # Mesh behavior
    share_load: true
    peer_timeout: 5s
    heartbeat_interval: 10s

# HTTP Server configuration
server:
  listen_addr: "0.0.0.0:8080"
  read_timeout: 30s
  write_timeout: 120s
  max_request_size: 10MB

  # API compatibility modes
  endpoints:
    - path: "/v1/chat/completions"
      protocol: "openai"
      enabled: true
    - path: "/v1/embeddings"
      protocol: "openai"
      enabled: true
    - path: "/v1/models"
      protocol: "openai"
      enabled: true
    - path: "/aiproxy/predict"
      protocol: "native"
      enabled: true

# Provider configurations
providers:
  # Local GPU provider
  local:
    type: "local"
    enabled: true
    priority: 1  # Try first (lowest = highest priority)

    runtimes:
      - onnx
      - pytorch
      - golearn

    models:
      - name: "llama-3-8b"
        path: "/models/llama-3-8b.onnx"
        runtime: "onnx"
        capabilities: ["text-generation"]
        cost_per_1k_tokens: 0.0  # Free - local compute

      - name: "clip-vit-base"
        path: "/models/clip-vit-base.onnx"
        runtime: "onnx"
        capabilities: ["embeddings", "image-classification"]
        cost_per_1k_tokens: 0.0

  # Cloudflare Workers AI
  cloudflare:
    type: "cloudflare"
    enabled: true
    priority: 2

    credentials:
      account_id: "${CLOUDFLARE_ACCOUNT_ID}"
      api_token: "${CLOUDFLARE_API_TOKEN}"

    endpoint: "https://api.cloudflare.com/client/v4"

    # Model mapping: local name -> Cloudflare model ID
    models:
      - name: "llama-3.1-8b"
        cloudflare_model: "@cf/meta/llama-3.1-8b-instruct"
        capabilities: ["text-generation"]
        cost_per_1k_tokens: 0.001  # Edge pricing

      - name: "llama-4-scout"
        cloudflare_model: "@cf/meta/llama-4-scout-17b-16e"
        capabilities: ["text-generation", "vision"]
        cost_per_1k_tokens: 0.002

      - name: "qwen-coder"
        cloudflare_model: "@cf/qwen/qwen2.5-coder-7b-instruct"
        capabilities: ["code-generation"]
        cost_per_1k_tokens: 0.001

      - name: "stable-diffusion-xl"
        cloudflare_model: "@cf/stabilityai/stable-diffusion-xl-base-1.0"
        capabilities: ["text-to-image"]
        cost_per_generation: 0.01

  # OpenAI (fallback)
  openai:
    type: "openai"
    enabled: true
    priority: 3

    credentials:
      api_key: "${OPENAI_API_KEY}"

    endpoint: "https://api.openai.com/v1"

    models:
      - name: "gpt-4o"
        openai_model: "gpt-4o"
        capabilities: ["text-generation", "vision"]
        cost_per_1k_tokens: 0.005

      - name: "gpt-4o-mini"
        openai_model: "gpt-4o-mini"
        capabilities: ["text-generation"]
        cost_per_1k_tokens: 0.0003

# Routing engine configuration
routing:
  # Default routing strategy
  strategy: "cost_optimized"  # cost_optimized, latency_optimized, availability, round_robin

  # Routing policies
  policies:
    # Cost optimization
    - name: "minimize_cost"
      type: "cost_optimized"
      rules:
        - if: "request.tokens < 1000"
          then: "provider.priority ASC"
        - if: "request.tokens >= 1000"
          then: "provider.cost_per_1k_tokens ASC"

    # Latency optimization
    - name: "minimize_latency"
      type: "latency_optimized"
      rules:
        - if: "provider.type == 'local'"
          then: "priority = 1"
        - if: "provider.type == 'cloudflare'"
          then: "priority = 2"
        - else: "priority = 3"

    # Model-specific routing
    - name: "route_by_model"
      type: "model_specific"
      rules:
        - if: "request.model matches 'llama-*'"
          then: "prefer provider.local > provider.cloudflare"
        - if: "request.model == 'gpt-4*'"
          then: "require provider.openai"

  # Failover configuration
  failover:
    enabled: true
    max_retries: 3
    retry_delay: 1s
    fallback_chain:
      - "local"
      - "cloudflare"
      - "openai"
      - "mesh_peer"

  # Load balancing
  load_balancing:
    enabled: true
    strategy: "least_loaded"  # round_robin, least_loaded, weighted

    # Health-based routing
    health_aware: true
    unhealthy_threshold: 0.5  # Route away if error rate > 50%

# Budget and cost controls
budget:
  enabled: true
  daily_limit: 100.00  # USD
  monthly_limit: 2000.00

  # Cost tracking
  track_costs: true
  cost_db: "sqlite:///data/costs.db"

  # Alerts
  alerts:
    - threshold: 80  # percent
      action: "log"
    - threshold: 95
      action: "disable_paid_providers"

# Observability
observability:
  # Logging
  logging:
    level: "info"  # debug, info, warn, error
    format: "json"
    output: "stdout"

  # Metrics
  metrics:
    enabled: true
    prometheus_port: 9091

    collect:
      - request_count
      - request_duration
      - provider_errors
      - cost_per_request
      - tokens_processed

  # Tracing
  tracing:
    enabled: false
    provider: "jaeger"
    endpoint: "http://jaeger:14268/api/traces"

# Security
security:
  # Authentication
  auth:
    enabled: true
    type: "api_key"  # api_key, jwt, oauth2

    api_keys:
      - key: "${AIPROXY_API_KEY_1}"
        name: "production"
        rate_limit: 1000  # requests/hour
      - key: "${AIPROXY_API_KEY_2}"
        name: "development"
        rate_limit: 100

  # Rate limiting
  rate_limiting:
    enabled: true
    default_limit: 100  # requests/hour
    by_ip: true
    by_api_key: true

  # CORS
  cors:
    enabled: true
    allowed_origins:
      - "http://localhost:3000"
      - "https://app.aiserve.farm"
```

### 4.2 Minimal Configuration

For quick setup:

```yaml
# aiproxy-minimal.yaml
node:
  id: "local-node"
  type: "standalone"

server:
  listen_addr: "0.0.0.0:8080"

providers:
  local:
    enabled: true
    models:
      - name: "my-model"
        path: "/models/model.onnx"

  cloudflare:
    enabled: true
    credentials:
      account_id: "your-account-id"
      api_token: "your-api-token"
    models:
      - name: "llama-3.1-8b"
        cloudflare_model: "@cf/meta/llama-3.1-8b-instruct"
```

## 5. API Specification

### 5.1 OpenAI-Compatible Endpoint

```http
POST /v1/chat/completions HTTP/1.1
Host: localhost:8080
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json

{
  "model": "llama-3.1-8b",
  "messages": [
    {"role": "user", "content": "Hello!"}
  ],
  "temperature": 0.7
}
```

Response:
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "llama-3.1-8b",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Hello! How can I help you?"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21
  },
  "x-aiproxy": {
    "provider": "cloudflare",
    "node": "gpu-node-01",
    "cost": 0.000021,
    "latency_ms": 234
  }
}
```

### 5.2 Native AIProxy Endpoint

```http
POST /aiproxy/predict HTTP/1.1
Host: localhost:8080
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json

{
  "model": "llama-3.1-8b",
  "input": "Hello, world!",
  "routing": {
    "strategy": "cost_optimized",
    "max_cost": 0.01,
    "timeout": 30000
  }
}
```

Response:
```json
{
  "request_id": "req_abc123",
  "model": "llama-3.1-8b",
  "output": "Hello! How can I help you?",
  "metadata": {
    "provider": "cloudflare",
    "node_id": "gpu-node-01",
    "latency_ms": 234,
    "tokens": {
      "input": 4,
      "output": 7,
      "total": 11
    },
    "cost": {
      "amount": 0.000011,
      "currency": "USD"
    },
    "routing": {
      "strategy_used": "cost_optimized",
      "providers_tried": ["local", "cloudflare"],
      "failovers": 1,
      "reason": "local_gpu_busy"
    }
  }
}
```

### 5.3 Model Discovery

```http
GET /v1/models HTTP/1.1
Host: localhost:8080
Authorization: Bearer YOUR_API_KEY
```

Response:
```json
{
  "object": "list",
  "data": [
    {
      "id": "llama-3.1-8b",
      "object": "model",
      "created": 1677610602,
      "owned_by": "meta",
      "providers": [
        {
          "name": "local",
          "available": true,
          "cost_per_1k_tokens": 0.0,
          "avg_latency_ms": 45
        },
        {
          "name": "cloudflare",
          "available": true,
          "cost_per_1k_tokens": 0.001,
          "avg_latency_ms": 230
        }
      ]
    }
  ]
}
```

### 5.4 Mesh Status

```http
GET /aiproxy/mesh/status HTTP/1.1
Host: localhost:8080
Authorization: Bearer YOUR_API_KEY
```

Response:
```json
{
  "node": {
    "id": "gpu-node-01",
    "type": "gpu",
    "region": "us-west-2",
    "uptime_seconds": 86400
  },
  "peers": [
    {
      "id": "edge-node-01",
      "addr": "edge-node-01.local:9090",
      "status": "healthy",
      "latency_ms": 12,
      "load": 0.45,
      "models_available": 15
    },
    {
      "id": "cloud-node-01",
      "addr": "cloud-node-01.local:9090",
      "status": "healthy",
      "latency_ms": 78,
      "load": 0.23,
      "models_available": 50
    }
  ],
  "providers": {
    "local": {"status": "healthy", "models": 3},
    "cloudflare": {"status": "healthy", "models": 12},
    "openai": {"status": "healthy", "models": 8}
  }
}
```

## 6. Routing Decision Flow

```
Request arrives
     │
     ▼
┌────────────────┐
│ Parse Request  │
│ - Model        │
│ - Parameters   │
│ - Constraints  │
└────────┬───────┘
         │
         ▼
┌────────────────────┐
│ Apply Routing      │
│ Policy             │
│ - Cost limits      │
│ - Latency req      │
│ - Model caps       │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│ Select Provider(s) │
│ - Priority order   │
│ - Availability     │
│ - Health status    │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│ Try Provider #1    │
└────────┬───────────┘
         │
    Success? ──Yes──▶ Return Response
         │
        No
         │
         ▼
┌────────────────────┐
│ Failover to        │
│ Provider #2        │
└────────┬───────────┘
         │
         ▼
    [Repeat until success or exhausted]
```

## 7. Use Cases

### 7.1 Cost-Optimized Development

**Scenario**: Developer wants to test with local models, fallback to cloud only when necessary.

**Config**:
```yaml
routing:
  strategy: "cost_optimized"
  policies:
    - if: "provider.type == 'local'"
      then: "priority = 1"
    - else: "priority = 999"
```

**Result**: Uses local GPU exclusively, only fails to cloud if local is down.

### 7.2 Geo-Distributed Production

**Scenario**: Global app needs low latency everywhere.

**Setup**:
- US node: Local GPU + Cloudflare US edge
- EU node: Local GPU + Cloudflare EU edge
- Asia node: Cloudflare Asia edge only

**Result**: Users automatically routed to nearest node, lowest latency.

### 7.3 Budget-Constrained Startup

**Scenario**: Limited budget, need to maximize free tier usage.

**Config**:
```yaml
budget:
  daily_limit: 10.00
  alerts:
    - threshold: 90
      action: "disable_paid_providers"

routing:
  strategy: "cost_optimized"
```

**Result**: Uses free local/Cloudflare models until budget exhausted, then blocks expensive providers.

### 7.4 High-Availability Enterprise

**Scenario**: Cannot tolerate downtime, need multiple redundancy.

**Setup**: 5-node mesh with diverse providers

**Config**:
```yaml
mesh:
  enabled: true
  peers: [node1, node2, node3, node4]

routing:
  failover:
    enabled: true
    max_retries: 5
    fallback_chain: [local, cloudflare, openai, anthropic, mesh_peer]
```

**Result**: Automatic failover across 5 nodes × 4 providers = 20 fallback options.

## 8. Implementation Checklist

### Phase 1: Core (v0.1)
- [ ] Configuration parser (YAML/JSON)
- [ ] HTTP server with OpenAI endpoints
- [ ] Local provider integration (ONNX)
- [ ] Basic routing engine
- [ ] Health checks

### Phase 2: Providers (v0.2)
- [ ] Cloudflare Workers AI client
- [ ] OpenAI client
- [ ] Anthropic client
- [ ] Provider abstraction layer
- [ ] Cost tracking

### Phase 3: Intelligence (v0.3)
- [ ] Advanced routing strategies
- [ ] Cost optimization
- [ ] Latency optimization
- [ ] Load balancing
- [ ] Failover logic

### Phase 4: Federation (v0.4)
- [ ] Mesh networking protocol
- [ ] Peer discovery
- [ ] Load sharing
- [ ] Distributed health checks
- [ ] Consensus for routing decisions

### Phase 5: Production (v1.0)
- [ ] Security hardening
- [ ] Rate limiting
- [ ] Observability (metrics, logs, traces)
- [ ] Admin dashboard
- [ ] CLI management tools

## 9. Benefits

### For Developers
- **Simplicity**: One API for all providers
- **Flexibility**: Easy to add/remove providers
- **Cost savings**: Automatic use of cheapest option
- **Reliability**: Built-in failover

### For Organizations
- **Control**: Keep sensitive workloads on-premises
- **Optimization**: Intelligent routing reduces costs
- **Scalability**: Add capacity by adding nodes
- **Compliance**: Route sensitive data to compliant providers

### For the Ecosystem
- **Interoperability**: Standard protocol for AI routing
- **Innovation**: Enables new routing strategies
- **Competition**: Reduces vendor lock-in
- **Efficiency**: Better utilization of compute resources

## 10. Future Extensions

### v2.0 Ideas
- **Fine-tuning orchestration**: Route training jobs
- **Model caching**: Cache at edge nodes
- **Request batching**: Batch similar requests for efficiency
- **Privacy-preserving routing**: Encrypt requests across mesh
- **Marketplace**: Buy/sell spare GPU capacity

### v3.0 Ideas
- **Automatic model selection**: AI chooses best model for task
- **Cross-provider load balancing**: Split requests across providers
- **Predictive routing**: ML-based routing decisions
- **Self-healing mesh**: Automatic node recovery

## 11. Compliance

### AIProxy Standard Compliance Levels

**Level 1 - Basic**: HTTP server, single provider, OpenAI-compatible API
**Level 2 - Multi-Provider**: Multiple providers, basic routing, failover
**Level 3 - Intelligent**: Cost/latency optimization, load balancing
**Level 4 - Federated**: Mesh networking, peer routing, distributed
**Level 5 - Production**: Full security, observability, HA, compliance

## 12. References

- [Cloudflare Workers AI Docs](https://developers.cloudflare.com/workers-ai/)
- [OpenAI API Reference](https://platform.openai.com/docs/api-reference)
- AIServe.Farm Documentation

---

**Copyright**: AIServe.Farm © 2026
**License**: This standard is published under Creative Commons CC-BY-SA 4.0
**Feedback**: Submit issues and suggestions to https://github.com/aiserve/aiproxy-standard
