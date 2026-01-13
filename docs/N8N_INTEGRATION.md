# n8n Integration Guide

## Overview

Integrate GPU Proxy with n8n for visual workflow automation of GPU operations. This guide covers three integration approaches: HTTP Request nodes, custom n8n nodes, and MCP protocol integration.

## Why n8n + GPU Proxy?

**Benefits:**
- **Visual Workflows**: Build complex GPU orchestration without code
- **400+ Integrations**: Connect GPUs to Slack, Discord, databases, cloud storage, APIs
- **Event-Driven**: Trigger GPU workloads from webhooks, schedules, file uploads, database changes
- **Low-Code**: Empower non-developers to manage GPU infrastructure
- **Cost Optimization**: Automate GPU lifecycle management and cleanup

**Use Cases:**
- Auto-scale GPU instances based on queue depth
- Route AI workloads to optimal GPU types
- Generate and send billing reports
- Monitor metrics and send alerts
- Batch process files uploaded to S3/GCS
- Schedule model training jobs
- Clean up idle GPU instances

## Integration Approach 1: HTTP Request Node (Quickest)

### Setup

1. **Install n8n**:
```bash
npx n8n
# or
docker run -p 5678:5678 n8nio/n8n
```

2. **Create Workflow**:
   - Add HTTP Request node
   - Configure authentication
   - Set endpoint URL

### Authentication

#### Option A: API Key Header
```yaml
Authentication: Header Auth
Name: X-API-Key
Value: {{ $credentials.apiKey }}
```

#### Option B: Bearer Token
```yaml
Authentication: Generic Credential Type
Header Auth
Name: Authorization
Value: Bearer {{ $credentials.token }}
```

### Example Workflows

#### 1. Provision GPU on Webhook
```
Webhook Trigger
  ↓
HTTP Request (POST /api/v1/gpu/instances/vastai/12345)
  ↓
Slack (Send notification)
```

**HTTP Request Configuration:**
- Method: `POST`
- URL: `https://your-domain.com/api/v1/gpu/instances/vastai/12345`
- Headers: `X-API-Key: your-api-key`
- Body:
```json
{
  "gpu_model": "H100",
  "duration": 3600
}
```

#### 2. Auto-Scale Based on Queue Depth
```
Schedule Trigger (every 5 minutes)
  ↓
HTTP Request (GET /api/v1/polling)
  ↓
IF (requests_in_flight > 100)
  ↓ Yes
  HTTP Request (POST /api/v1/gpu/instances/reserve)
  ↓ No
  (Do nothing)
```

**Polling Check:**
```json
{
  "url": "https://your-domain.com/api/v1/polling",
  "method": "GET"
}
```

**Conditional Logic:**
```javascript
// In IF node
return items[0].json.metrics.requests_in_flight > 100;
```

#### 3. Cost Monitoring & Alerts
```
Schedule Trigger (daily at 9 AM)
  ↓
HTTP Request (GET /api/v1/stats)
  ↓
Function (Calculate daily cost)
  ↓
IF (cost > $1000)
  ↓ Yes
  Slack (Send alert to #finance)
```

**Function Node:**
```javascript
const stats = items[0].json;
const dailyCost = stats.gpu.total_cost_usd;

return {
  json: {
    dailyCost,
    instanceCount: stats.gpu.active_instances,
    timestamp: new Date().toISOString()
  }
};
```

#### 4. Batch Processing Pipeline
```
S3 Trigger (new file upload)
  ↓
HTTP Request (POST /api/v1/gpu/proxy)
  Body: { "model": "stable-diffusion", "input": file_url }
  ↓
Wait (for completion)
  ↓
HTTP Request (GET result)
  ↓
S3 (Upload result)
  ↓
Discord (Notify user)
```

#### 5. GPU Cleanup Job
```
Schedule Trigger (hourly)
  ↓
HTTP Request (GET /api/v1/gpu/instances)
  ↓
Function (Filter idle instances > 30 min)
  ↓
Loop (for each idle instance)
  ↓
  HTTP Request (DELETE /api/v1/gpu/instances/{provider}/{id})
```

**Filter Function:**
```javascript
const instances = items[0].json.instances || [];
const idleThreshold = 30 * 60 * 1000; // 30 minutes

const idleInstances = instances.filter(inst => {
  const lastUsed = new Date(inst.last_used_at);
  const now = new Date();
  return (now - lastUsed) > idleThreshold;
});

return idleInstances.map(inst => ({ json: inst }));
```

## Integration Approach 2: Custom n8n Node (Advanced)

### Create Custom Node Package

```bash
npm init n8n-node
```

### Node Implementation

```typescript
// GPUProxy.node.ts
import {
  IExecuteFunctions,
  INodeExecutionData,
  INodeType,
  INodeTypeDescription,
} from 'n8n-workflow';

export class GPUProxy implements INodeType {
  description: INodeTypeDescription = {
    displayName: 'GPU Proxy',
    name: 'gpuProxy',
    group: ['transform'],
    version: 1,
    description: 'Interact with GPU Proxy API',
    defaults: {
      name: 'GPU Proxy',
    },
    inputs: ['main'],
    outputs: ['main'],
    credentials: [
      {
        name: 'gpuProxyApi',
        required: true,
      },
    ],
    properties: [
      {
        displayName: 'Operation',
        name: 'operation',
        type: 'options',
        options: [
          {
            name: 'List Instances',
            value: 'listInstances',
          },
          {
            name: 'Create Instance',
            value: 'createInstance',
          },
          {
            name: 'Destroy Instance',
            value: 'destroyInstance',
          },
          {
            name: 'Proxy Request',
            value: 'proxyRequest',
          },
          {
            name: 'Get Stats',
            value: 'getStats',
          },
        ],
        default: 'listInstances',
      },
      {
        displayName: 'Provider',
        name: 'provider',
        type: 'options',
        displayOptions: {
          show: {
            operation: ['createInstance', 'destroyInstance'],
          },
        },
        options: [
          { name: 'Vast.ai', value: 'vastai' },
          { name: 'IO.net', value: 'ionet' },
        ],
        default: 'vastai',
      },
      {
        displayName: 'Instance ID',
        name: 'instanceId',
        type: 'string',
        displayOptions: {
          show: {
            operation: ['createInstance', 'destroyInstance'],
          },
        },
        default: '',
        description: 'GPU instance identifier',
      },
      {
        displayName: 'GPU Preferences',
        name: 'preferences',
        type: 'fixedCollection',
        displayOptions: {
          show: {
            operation: ['createInstance'],
          },
        },
        typeOptions: {
          multipleValues: true,
        },
        default: {},
        options: [
          {
            name: 'preferredGPUs',
            displayName: 'Preferred GPUs',
            values: [
              {
                displayName: 'Model',
                name: 'model',
                type: 'options',
                options: [
                  { name: 'H100', value: 'H100' },
                  { name: 'H200', value: 'H200' },
                  { name: 'A100', value: 'A100' },
                  { name: 'V100', value: 'V100' },
                  { name: 'RTX 4090', value: 'RTX 4090' },
                ],
                default: 'H100',
              },
              {
                displayName: 'Priority',
                name: 'priority',
                type: 'number',
                default: 1,
                description: '1 = highest priority',
              },
            ],
          },
        ],
      },
    ],
  };

  async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
    const items = this.getInputData();
    const returnData: INodeExecutionData[] = [];
    const operation = this.getNodeParameter('operation', 0) as string;
    const credentials = await this.getCredentials('gpuProxyApi');

    const baseUrl = credentials.url as string;
    const apiKey = credentials.apiKey as string;

    for (let i = 0; i < items.length; i++) {
      let responseData;

      if (operation === 'listInstances') {
        const response = await this.helpers.request({
          method: 'GET',
          url: `${baseUrl}/api/v1/gpu/instances`,
          headers: {
            'X-API-Key': apiKey,
          },
          json: true,
        });
        responseData = response;
      }

      if (operation === 'createInstance') {
        const provider = this.getNodeParameter('provider', i) as string;
        const instanceId = this.getNodeParameter('instanceId', i) as string;
        const preferences = this.getNodeParameter('preferences', i) as any;

        const response = await this.helpers.request({
          method: 'POST',
          url: `${baseUrl}/api/v1/gpu/instances/${provider}/${instanceId}`,
          headers: {
            'X-API-Key': apiKey,
          },
          body: preferences,
          json: true,
        });
        responseData = response;
      }

      if (operation === 'getStats') {
        const response = await this.helpers.request({
          method: 'GET',
          url: `${baseUrl}/api/v1/stats`,
          headers: {
            'X-API-Key': apiKey,
          },
          json: true,
        });
        responseData = response;
      }

      returnData.push({ json: responseData });
    }

    return [returnData];
  }
}
```

### Credentials Setup

```typescript
// GPUProxyApi.credentials.ts
import {
  ICredentialType,
  INodeProperties,
} from 'n8n-workflow';

export class GPUProxyApi implements ICredentialType {
  name = 'gpuProxyApi';
  displayName = 'GPU Proxy API';
  properties: INodeProperties[] = [
    {
      displayName: 'API URL',
      name: 'url',
      type: 'string',
      default: 'https://your-domain.com',
    },
    {
      displayName: 'API Key',
      name: 'apiKey',
      type: 'string',
      typeOptions: {
        password: true,
      },
      default: '',
    },
  ];
}
```

### Publish to npm

```bash
npm publish
```

Users can then install:
```bash
npm install n8n-nodes-gpuproxy
```

## Integration Approach 3: MCP Protocol (Optimal)

### Available MCP Tools

Your n8n MCP server provides these tools:
- `search_nodes` - Find n8n nodes
- `get_node` - Get node configuration
- `validate_node` - Validate node config
- `search_templates` - Find workflow templates
- `get_template` - Get template details
- `n8n_create_workflow` - Create workflows programmatically
- `n8n_get_workflow` - Retrieve workflows
- `n8n_update_workflow` - Update workflows
- `n8n_list_workflows` - List all workflows
- `n8n_test_workflow` - Test workflow execution

### GPU Proxy MCP Server

Create MCP server that exposes GPU operations as tools:

```javascript
// mcp-server-gpuproxy.js
const { MCPServer } = require('@anthropic/mcp-server');

const server = new MCPServer({
  name: 'gpuproxy',
  version: '1.0.0',
  tools: [
    {
      name: 'provision_gpu',
      description: 'Provision a GPU instance',
      inputSchema: {
        type: 'object',
        properties: {
          provider: { type: 'string', enum: ['vastai', 'ionet'] },
          gpu_model: { type: 'string' },
          duration: { type: 'number' },
        },
        required: ['provider', 'gpu_model'],
      },
      handler: async ({ provider, gpu_model, duration }) => {
        // Call GPU Proxy API
        const response = await fetch(`${GPU_PROXY_URL}/api/v1/gpu/instances/reserve`, {
          method: 'POST',
          headers: {
            'X-API-Key': API_KEY,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            preferred_gpus: [{ model: gpu_model, priority: 1 }],
            duration,
          }),
        });
        return await response.json();
      },
    },
    {
      name: 'get_gpu_stats',
      description: 'Get GPU usage statistics',
      inputSchema: { type: 'object', properties: {} },
      handler: async () => {
        const response = await fetch(`${GPU_PROXY_URL}/api/v1/stats`, {
          headers: { 'X-API-Key': API_KEY },
        });
        return await response.json();
      },
    },
    {
      name: 'cleanup_idle_gpus',
      description: 'Clean up idle GPU instances',
      inputSchema: {
        type: 'object',
        properties: {
          idle_threshold_minutes: { type: 'number', default: 30 },
        },
      },
      handler: async ({ idle_threshold_minutes }) => {
        // Get instances
        const instances = await fetch(`${GPU_PROXY_URL}/api/v1/gpu/instances`, {
          headers: { 'X-API-Key': API_KEY },
        }).then(r => r.json());

        // Filter and destroy idle instances
        const results = [];
        for (const inst of instances.instances || []) {
          const idleMinutes = (Date.now() - new Date(inst.last_used_at)) / 60000;
          if (idleMinutes > idle_threshold_minutes) {
            const response = await fetch(
              `${GPU_PROXY_URL}/api/v1/gpu/instances/${inst.provider}/${inst.id}`,
              {
                method: 'DELETE',
                headers: { 'X-API-Key': API_KEY },
              }
            );
            results.push({ instance: inst.id, destroyed: response.ok });
          }
        }
        return { cleaned_up: results.length, details: results };
      },
    },
  ],
});

server.listen(3000);
```

### n8n Workflow with MCP

Use n8n's MCP integration to call GPU Proxy tools:

```
MCP Node (provision_gpu)
  Input: { "gpu_model": "H100", "provider": "vastai" }
  ↓
Wait (30 seconds)
  ↓
MCP Node (get_gpu_stats)
  ↓
Slack (Send stats)
```

## Production Deployment

### Docker Compose Setup

```yaml
version: '3.8'

services:
  n8n:
    image: n8nio/n8n
    ports:
      - "5678:5678"
    environment:
      - N8N_BASIC_AUTH_ACTIVE=true
      - N8N_BASIC_AUTH_USER=admin
      - N8N_BASIC_AUTH_PASSWORD=admin
      - WEBHOOK_URL=https://n8n.your-domain.com
    volumes:
      - n8n_data:/home/node/.n8n
    networks:
      - gpuproxy-network

  gpuproxy:
    image: aiserve-gpuproxyd
    ports:
      - "8080:8080"
    environment:
      - JWT_SECRET=your-secret
      - DB_HOST=postgres
    networks:
      - gpuproxy-network

networks:
  gpuproxy-network:

volumes:
  n8n_data:
```

### Security Best Practices

1. **Use API Keys**: Never hardcode credentials in workflows
2. **Environment Variables**: Store secrets in n8n's credential system
3. **HTTPS Only**: Use SSL/TLS for all API communication
4. **Rate Limiting**: Implement rate limits on GPU Proxy API
5. **Audit Logging**: Log all workflow executions

### Monitoring Integration

Connect n8n to GPU Proxy metrics:

```
Schedule Trigger (every 5 minutes)
  ↓
HTTP Request (GET /metrics)
  ↓
Parse Prometheus Metrics
  ↓
IF (error_rate > 5%)
  ↓
  PagerDuty (Create incident)
```

## Example Use Cases

### 1. AI Image Generation Service

```
Webhook (POST /generate)
  ↓
Set Variables (extract prompt, style, size)
  ↓
HTTP Request (GET /api/v1/gpu/available)
  Filter: { "model": "RTX 4090", "vram": 24 }
  ↓
IF (gpu_available)
  ↓ Yes
  HTTP Request (POST /api/v1/gpu/proxy)
    Body: {
      "model": "stable-diffusion",
      "prompt": "{{ $json.prompt }}",
      "gpu_preference": "RTX 4090"
    }
  ↓
  S3 (Upload image)
  ↓
  Respond to Webhook
  ↓ No
  Queue (Add to processing queue)
```

### 2. Cost Optimization Pipeline

```
Schedule (daily at 2 AM UTC)
  ↓
HTTP Request (GET /api/v1/stats)
  ↓
Function (Analyze cost trends)
  ↓
IF (daily_cost > budget * 1.2)
  ↓ Yes
  HTTP Request (GET /api/v1/gpu/instances)
  ↓
  Function (Sort by cost, find most expensive)
  ↓
  HTTP Request (DELETE least utilized instances)
  ↓
  Slack (Alert team)
```

### 3. Multi-Model Router

```
Webhook (POST /inference)
  ↓
Function (Detect model from request)
  ↓
Switch (based on model type)
  ├─ LLM (>100B params)
  │   └─ Route to H100 cluster
  ├─ LLM (7B-70B params)
  │   └─ Route to A100 cluster
  └─ Vision Model
      └─ Route to RTX 4090 cluster
```

## Advanced Patterns

### Error Handling

```
HTTP Request (with error handling)
  ↓
Error Trigger (if request fails)
  ↓
  Wait (exponential backoff)
  ↓
  Retry (max 3 attempts)
  ↓
  If still failing:
    ↓
    Log to Database
    ↓
    Alert on-call engineer
```

### Circuit Breaker Pattern

```
Function (check failure rate)
  ↓
IF (failure_rate > 50% in last 5 min)
  ↓ Yes (Open circuit)
  Return cached response
  ↓ No (Closed circuit)
  Continue to GPU Proxy
```

### Rate Limiting

```
Redis (increment counter)
  ↓
IF (requests_per_minute > limit)
  ↓ Yes
  Respond with 429
  ↓ No
  Continue processing
```

## Support & Resources

- GPU Proxy API Docs: `/docs`
- n8n Community: https://community.n8n.io
- n8n MCP Tools: Available in your Claude Code session
- GPU Proxy Platform: https://aiserve.farm

## Next Steps

1. Start with HTTP Request node approach for quick wins
2. Build custom node for production deployments
3. Explore MCP integration for advanced workflows
4. Monitor metrics and optimize based on usage patterns
