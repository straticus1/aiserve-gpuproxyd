# AIServe.Farm API Reference

**Complete REST API Documentation**

Base URL: `http://<server>:<port>` (default port: 8080)

## Table of Contents

- [Authentication](#authentication)
- [User Management](#user-management)
- [GPU Management](#gpu-management)
- [GPU Preferences](#gpu-preferences)
- [Load Balancing](#load-balancing)
- [Billing](#billing)
- [Guardrails](#guardrails)
- [Model Serving](#model-serving)
- [Agent Protocols](#agent-protocols)
- [Health & Monitoring](#health--monitoring)
- [Error Responses](#error-responses)

## Base Information

- **API Base Path**: `/api/v1`
- **Response Format**: JSON
- **Authentication**: JWT Bearer Token (except public endpoints)
- **Rate Limiting**: 100 requests per window
- **API Version**: 1.0.1

## Authentication

### Register User

Create a new user account.

```http
POST /api/v1/auth/register
Content-Type: application/json
```

**Request:**
```json
{
  "email": "user@example.com",
  "password": "secure_password123",
  "name": "User Name"
}
```

**Response:** `201 Created`
```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "User Name",
    "created_at": "2026-01-13T12:00:00Z"
  }
}
```

### Login

Authenticate and receive JWT tokens.

```http
POST /api/v1/auth/login
Content-Type: application/json
```

**Request:**
```json
{
  "email": "user@example.com",
  "password": "secure_password123"
}
```

**Response:** `200 OK`
```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com"
  },
  "tokens": {
    "access_token": "eyJhbGc...",
    "refresh_token": "eyJhbGc...",
    "expires_in": 86400
  }
}
```

### Create API Key

Generate an API key for programmatic access.

```http
POST /api/v1/auth/apikey
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "name": "production_api_key",
  "expires_at": "2027-12-31T23:59:59Z"
}
```

**Response:** `201 Created`
```json
{
  "api_key": "ak_live_1234567890abcdef",
  "message": "Save this API key securely. It will not be shown again.",
  "created_at": "2026-01-13T12:00:00Z",
  "expires_at": "2027-12-31T23:59:59Z"
}
```

## User Management

### Export User Account

Download complete account data including usage, transactions, and API keys.

```http
GET /api/v1/user/export
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "user": { /* user info */ },
  "api_keys": [ /* array of API keys */ ],
  "usage_quota": { /* quota info */ },
  "gpu_usage": [ /* GPU usage records */ ],
  "transactions": [ /* transaction history */ ],
  "credits": [ /* credit records */ ],
  "exported_at": "2026-01-13T12:00:00Z"
}
```

## GPU Management

### List GPU Instances

Query available GPU instances across providers.

```http
GET /api/v1/gpu/instances?provider=all&min_vram=16&max_price=2.5
Authorization: Bearer <jwt_token>
```

**Query Parameters:**
- `provider` (string): "vastai", "ionet", or "all" (default)
- `min_vram` (integer): Minimum VRAM in GB
- `max_price` (float): Maximum price per hour (USD)
- `gpu_model` (string): Specific GPU model (e.g., "RTX 4090")
- `location` (string): Preferred location (e.g., "US")

**Response:** `200 OK`
```json
{
  "instances": [
    {
      "id": "instance_123",
      "provider": "vastai",
      "gpu_model": "RTX 4090",
      "vram_gb": 24,
      "price_per_hour": 1.25,
      "location": "US-WEST",
      "available": true
    }
  ],
  "count": 15,
  "provider": "all"
}
```

### Create Single Instance

Provision a specific GPU instance.

```http
POST /api/v1/gpu/instances/{provider}/{instanceId}
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**URL Parameters:**
- `provider`: "vastai" or "ionet"
- `instanceId`: Instance identifier from provider

**Request:**
```json
{
  "duration_hours": 2,
  "auto_renew": false
}
```

**Response:** `201 Created`
```json
{
  "contract_id": "contract_abc123",
  "provider": "vastai",
  "instance_id": "instance_123",
  "status": "provisioning",
  "created_at": "2026-01-13T12:00:00Z"
}
```

### Destroy Instance

Terminate a running GPU instance.

```http
DELETE /api/v1/gpu/instances/{provider}/{instanceId}
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "message": "Instance destroyed successfully",
  "instance_id": "instance_123"
}
```

### Batch Create Instances

Create multiple GPU instances at once.

```http
POST /api/v1/gpu/instances/batch
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "vastai_count": 2,
  "ionet_count": 2,
  "config": {
    "duration_hours": 4,
    "auto_renew": false
  }
}
```

**Response:** `201 Created`
```json
{
  "vastai_instances": ["contract_1", "contract_2"],
  "ionet_instances": ["contract_3", "contract_4"],
  "total_created": 4,
  "errors": []
}
```

### Reserve Instances

Smart GPU reservation with automatic load balancing.

```http
POST /api/v1/gpu/instances/reserve
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "count": 4,
  "filters": {
    "min_vram": 24,
    "gpu_model": "RTX 4090",
    "location": "US"
  },
  "config": {
    "duration_hours": 8,
    "auto_renew": true
  }
}
```

**Response:** `201 Created`
```json
{
  "reserved": [
    {
      "instance_id": "instance_1",
      "provider": "vastai",
      "contract_id": "contract_1"
    }
  ],
  "count": 4,
  "requested": 4,
  "errors": []
}
```

### Proxy Request

Forward requests directly to a GPU instance.

```http
POST /api/v1/gpu/proxy
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "method": "POST",
  "target": "instance_123",
  "path": "/v1/inference",
  "payload": {
    "model": "llama-2-70b",
    "prompt": "Hello world"
  }
}
```

**Response:** `200 OK`
(Response depends on proxied service)

## GPU Preferences

### Get User Preferences

Retrieve GPU selection preferences.

```http
GET /api/v1/gpu/preferences
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "preferred_vendors": ["NVIDIA", "AMD"],
  "preferred_tiers": ["high-end", "mid-range"],
  "min_vram": 16,
  "max_price_per_hour": 2.5,
  "priority": "performance",
  "is_default": false,
  "created_at": "2026-01-13T12:00:00Z"
}
```

### Set User Preferences

Configure GPU selection preferences.

```http
POST /api/v1/gpu/preferences
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "preferred_vendors": ["NVIDIA"],
  "preferred_tiers": ["high-end"],
  "min_vram": 24,
  "max_price_per_hour": 3.0,
  "priority": "performance"
}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "message": "Preferences saved successfully",
  "preferences": { /* saved preferences */ }
}
```

### Test GPU Selection

Preview GPU selection based on preferences.

```http
POST /api/v1/gpu/preferences/test
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Response:** `200 OK`
```json
{
  "selected_gpu": {
    "name": "RTX 4090",
    "vendor": "NVIDIA",
    "vram_gb": 24,
    "tier": "high-end",
    "score": 95.5
  },
  "alternatives": [ /* other options */ ]
}
```

### Get Available GPUs

List all available GPU models.

```http
GET /api/v1/gpu/available?vendor=NVIDIA&tier=high-end
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "gpus": [
    {
      "name": "RTX 4090",
      "vendor": "NVIDIA",
      "vram_gb": 24,
      "tier": "high-end",
      "avg_price_per_hour": 1.25
    }
  ],
  "count": 8
}
```

### Get Example Preferences (Public)

Get example preference configurations.

```http
GET /api/v1/gpu/examples
```

**Response:** `200 OK`
```json
{
  "examples": {
    "cost_optimized": { /* config */ },
    "performance_focused": { /* config */ },
    "nvidia_only": { /* config */ }
  },
  "available_examples": ["cost_optimized", "performance_focused", "nvidia_only", "amd_preferred"]
}
```

## Load Balancing

### Get All Instance Loads

View load across all GPU instances.

```http
GET /api/v1/loadbalancer/loads
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "strategy": "round_robin",
  "loads": {
    "instance_1": {
      "connections": 15,
      "load": 0.65,
      "response_time_ms": 45
    }
  },
  "count": 10
}
```

### Get Instance Load

View load for specific instance.

```http
GET /api/v1/loadbalancer/load?instance_id=instance_123
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "instance_id": "instance_123",
  "connections": 15,
  "load": 0.65,
  "response_time_ms": 45
}
```

### Get Strategy

Get current load balancing strategy.

```http
GET /api/v1/loadbalancer/strategy
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "strategy": "round_robin"
}
```

### Set Strategy

Change load balancing strategy.

```http
PUT /api/v1/loadbalancer/strategy
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "strategy": "least_connections"
}
```

**Strategies:**
- `round_robin`: Distribute evenly in rotation
- `equal_weighted`: Equal weight to all instances
- `weighted_round_robin`: Weight-based distribution
- `least_connections`: Route to least busy instance
- `least_response_time`: Route to fastest instance

**Response:** `200 OK`
```json
{
  "strategy": "least_connections",
  "message": "Load balancing strategy updated"
}
```

## Billing

### Create Payment

Process a payment.

```http
POST /api/v1/billing/payment
Authorization: Bearer <jwt_token>
Content-Type: application/json
X-Preferred-Payment: card
X-Billing: stripe
```

**Request:**
```json
{
  "amount": 100.00,
  "currency": "USD",
  "provider": "stripe",
  "payment_preference": {
    "type": "card",
    "card_token": "tok_visa"
  }
}
```

**Response:** `201 Created`
```json
{
  "transaction": {
    "id": "txn_123",
    "amount": 100.00,
    "currency": "USD",
    "status": "succeeded",
    "created_at": "2026-01-13T12:00:00Z"
  }
}
```

### Get Transactions

List user transactions.

```http
GET /api/v1/billing/transactions
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "transactions": [
    {
      "id": "txn_123",
      "amount": 100.00,
      "currency": "USD",
      "status": "succeeded",
      "created_at": "2026-01-13T12:00:00Z"
    }
  ],
  "count": 25
}
```

## Guardrails

### Get Spending Info

View current spending status.

```http
GET /api/v1/guardrails/spending
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "window_spent": 150.50,
  "window_limit": 1000.00,
  "window_name": "monthly",
  "percentage_used": 15.05,
  "violations": []
}
```

### Record Spending

Record spending amount.

```http
POST /api/v1/guardrails/spending/record
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "amount": 25.50
}
```

**Response:** `200 OK`
```json
{
  "message": "Spending recorded successfully",
  "new_total": 176.00
}
```

### Check Spending

Check if spending is allowed.

```http
POST /api/v1/guardrails/spending/check
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "estimated_cost": 50.00
}
```

**Response:** `200 OK` (if allowed)
```json
{
  "allowed": true,
  "spent": 176.00,
  "limit": 1000.00
}
```

**Response:** `402 Payment Required` (if violated)
```json
{
  "allowed": false,
  "violations": [
    {
      "window": "hourly",
      "spent": 105.00,
      "limit": 100.00
    }
  ],
  "spent": 176.00
}
```

### Reset Spending

Reset spending counters.

```http
POST /api/v1/guardrails/spending/reset
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "window_name": "monthly"
}
```

**Response:** `200 OK`
```json
{
  "message": "Spending reset successfully"
}
```

## Model Serving

### Upload Model

Upload a machine learning model.

```http
POST /api/v1/models/upload
Authorization: Bearer <jwt_token>
Content-Type: multipart/form-data
```

**Form Fields:**
- `model` (file): Model file
- `name` (string, optional): Model name
- `format` (string, optional): Model format (auto-detected if not provided)
- `framework` (string, optional): ML framework
- `version` (string, optional): Model version (default: "1.0.0")
- `gpu_required` (boolean, optional): Requires GPU for inference
- `gpu_type` (string, optional): Preferred GPU type

**Response:** `201 Created`
```json
{
  "model_id": "uuid",
  "name": "my_model",
  "format": "onnx",
  "endpoint": "/api/v1/models/uuid/predict",
  "status": "loading",
  "message": "Model uploaded successfully and loading"
}
```

### List Models

List user's models.

```http
GET /api/v1/models
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "models": [
    {
      "id": "uuid",
      "name": "my_model",
      "format": "onnx",
      "status": "ready",
      "created_at": "2026-01-13T12:00:00Z"
    }
  ],
  "count": 5
}
```

### Get Model Details

Get detailed model information.

```http
GET /api/v1/models/{model_id}
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "name": "my_model",
  "format": "onnx",
  "version": "1.0.0",
  "framework": "PyTorch",
  "status": "ready",
  "replicas": 2,
  "total_requests": 1250,
  "average_latency": 45.5,
  "error_rate": 0.02,
  "created_at": "2026-01-13T12:00:00Z",
  "updated_at": "2026-01-13T13:00:00Z"
}
```

### Delete Model

Delete a model.

```http
DELETE /api/v1/models/{model_id}
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "message": "Model deleted successfully"
}
```

### Run Inference

Execute model prediction.

```http
POST /api/v1/models/{model_id}/predict
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:**
```json
{
  "inputs": {
    "features": [1.0, 2.0, 3.0, 4.0]
  },
  "parameters": {
    "temperature": 0.7
  }
}
```

**Response:** `200 OK`
```json
{
  "model_id": "uuid",
  "outputs": {
    "predictions": [0.85, 0.15]
  },
  "latency_ms": 45.3,
  "metadata": {
    "format": "onnx",
    "runtime": "onnxruntime",
    "version": "1.0.0",
    "used_gpu": true
  }
}
```

### Get Model Metrics

Get model performance metrics.

```http
GET /api/v1/models/{model_id}/metrics
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "model_id": "uuid",
  "name": "my_model",
  "total_requests": 1250,
  "average_latency": 45.5,
  "p50_latency": 42.1,
  "p95_latency": 68.3,
  "p99_latency": 95.7,
  "error_rate": 0.02,
  "status": "ready",
  "replicas": 2
}
```

### Get Supported Formats (Public)

List supported model formats.

```http
GET /api/v1/models/formats
```

**Response:** `200 OK`
```json
{
  "formats": [
    {
      "format": "onnx",
      "extensions": [".onnx"],
      "framework": "ONNX Runtime",
      "description": "Open Neural Network Exchange format"
    },
    {
      "format": "pytorch",
      "extensions": [".pt", ".pth"],
      "framework": "PyTorch",
      "description": "PyTorch model checkpoints"
    }
  ],
  "count": 13
}
```

### Get Storage Quota

Check model storage quota.

```http
GET /api/v1/quota
Authorization: Bearer <jwt_token>
```

**Response:** `200 OK`
```json
{
  "user_id": "uuid",
  "storage": {
    "used_bytes": 52428800000,
    "limit_bytes": 107374182400,
    "used_pct": 48.8
  },
  "file_size": {
    "max_bytes": 10737418240
  },
  "rate_limits": {
    "uploads_last_hour": 12,
    "hourly_limit": 50,
    "uploads_last_day": 87,
    "daily_limit": 500
  }
}
```

## Agent Protocols

### MCP (Model Context Protocol)

Interact via Model Context Protocol.

```http
POST /api/v1/mcp
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:** (JSON-RPC 2.0 format)
```json
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "params": {},
  "id": 1
}
```

**Response:** `200 OK`
```json
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [ /* MCP tools */ ]
  },
  "id": 1
}
```

### MCP Server-Sent Events

Subscribe to MCP event stream.

```http
GET /api/v1/mcp/sse
Authorization: Bearer <jwt_token>
```

**Response:** Server-Sent Events stream

### Unified Agent Handler

Auto-detecting agent protocol handler.

```http
POST /api/v1/agent
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Request:** (Format depends on protocol)
```json
{
  "protocol": "a2a",
  "from_agent": "agent1",
  "to_agent": "agent2",
  "action": "query",
  "data": {}
}
```

**Response:** `200 OK`
(Response format depends on detected protocol)

## Health & Monitoring

### Health Check

System health status.

```http
GET /health
```

**Response:** `200 OK`
```json
{
  "status": "ok",
  "timestamp": "2026-01-13T12:00:00Z",
  "checks": {
    "database": {
      "healthy": true,
      "response_time_ms": 5
    },
    "redis": {
      "healthy": true,
      "response_time_ms": 2
    }
  }
}
```

### Prometheus Metrics

Export metrics in Prometheus format.

```http
GET /metrics
```

**Response:** `200 OK` (Prometheus text format)

### System Statistics

JSON-formatted system statistics.

```http
GET /stats
```

**Response:** `200 OK`
```json
{
  "timestamp": "2026-01-13T12:00:00Z",
  "requests_total": 125000,
  "requests_per_second": 42,
  "active_connections": 156,
  "gpu_instances_active": 45,
  "models_loaded": 128
}
```

### WebSocket Connection

Real-time WebSocket connection for streaming.

```http
GET /ws
Upgrade: websocket
Connection: Upgrade
```

**Messages:**
- `ping` → Server responds with `pong`
- `subscribe:{stream}` → Subscribe to event stream

## Error Responses

All endpoints return standardized error responses:

```json
{
  "error": "Human-readable error message",
  "code": "ERROR_CODE",
  "details": { /* optional additional context */ }
}
```

### HTTP Status Codes

- `200 OK` - Success
- `201 Created` - Resource created successfully
- `400 Bad Request` - Invalid input or malformed request
- `401 Unauthorized` - Authentication required or failed
- `402 Payment Required` - Spending limit exceeded
- `403 Forbidden` - Access denied (ownership/permission issue)
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service temporarily unavailable

### Common Error Codes

- `AUTH_REQUIRED` - Authentication token missing
- `AUTH_INVALID` - Invalid authentication credentials
- `RATE_LIMITED` - Too many requests
- `QUOTA_EXCEEDED` - Storage or upload quota exceeded
- `SPENDING_LIMIT` - Spending guardrail violation
- `NOT_FOUND` - Resource not found
- `VALIDATION_ERROR` - Input validation failed
- `INSUFFICIENT_CREDITS` - Not enough credits for operation

---

**Last Updated:** 2026-01-13
**API Version:** 1.0.1
**Platform:** AIServe.Farm by AfterDark Systems (ADS)
