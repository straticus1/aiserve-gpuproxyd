# Model Context Protocol (MCP) Support

AIServe GPU Proxy implements the Model Context Protocol, allowing AI assistants and other MCP clients to directly manage GPU instances, track spending, and handle billing operations.

## Overview

The MCP server exposes GPU proxy functionality through a standardized protocol that can be consumed by:
- Claude Desktop
- Custom MCP clients
- AI agents and assistants
- Automation tools

## Features

### Tools

The MCP server provides the following tools:

#### 1. `list_gpu_instances`
List available GPU instances from all providers.

**Parameters:**
- `provider` (optional): Filter by provider (`vast.ai`, `io.net`, or `all`)
- `min_vram` (optional): Minimum VRAM in GB
- `max_price` (optional): Maximum price per hour in USD

**Example:**
```json
{
  "name": "list_gpu_instances",
  "arguments": {
    "provider": "vast.ai",
    "min_vram": 24,
    "max_price": 1.5
  }
}
```

#### 2. `create_gpu_instance`
Create a GPU instance on a specific provider.

**Parameters:**
- `provider` (required): Provider (`vast.ai` or `io.net`)
- `instance_id` (required): Instance ID from the provider
- `image` (optional): Docker image to use

**Example:**
```json
{
  "name": "create_gpu_instance",
  "arguments": {
    "provider": "vast.ai",
    "instance_id": "12345",
    "image": "nvidia/cuda:12.0.0-base-ubuntu22.04"
  }
}
```

#### 3. `destroy_gpu_instance`
Destroy a GPU instance.

**Parameters:**
- `provider` (required): Provider (`vast.ai` or `io.net`)
- `instance_id` (required): Instance ID to destroy

**Example:**
```json
{
  "name": "destroy_gpu_instance",
  "arguments": {
    "provider": "vast.ai",
    "instance_id": "12345"
  }
}
```

#### 4. `get_spending_info`
Get current spending information across all guard rails time windows.

**Parameters:** None

**Example:**
```json
{
  "name": "get_spending_info",
  "arguments": {}
}
```

#### 5. `check_spending_limit`
Check if a request with estimated cost would exceed spending limits.

**Parameters:**
- `estimated_cost` (required): Estimated cost in USD

**Example:**
```json
{
  "name": "check_spending_limit",
  "arguments": {
    "estimated_cost": 25.00
  }
}
```

#### 6. `get_billing_transactions`
Get billing transaction history for the authenticated user.

**Parameters:** None

**Example:**
```json
{
  "name": "get_billing_transactions",
  "arguments": {}
}
```

#### 7. `proxy_inference_request`
Proxy an inference request to a GPU instance.

**Parameters:**
- `target_url` (required): Target URL for the inference request
- `method` (optional): HTTP method (default: `POST`)
- `body` (optional): Request body
- `headers` (optional): Request headers

**Example:**
```json
{
  "name": "proxy_inference_request",
  "arguments": {
    "target_url": "https://gpu-instance.example.com/v1/inference",
    "method": "POST",
    "body": {
      "model": "llama-2-70b",
      "prompt": "Hello, world!"
    }
  }
}
```

### Resources

The MCP server exposes the following resources:

#### 1. `gpu://instances`
List of all available GPU instances across providers.

#### 2. `spending://current`
Current spending information across all guard rails time windows.

## Configuration

### Claude Desktop Integration

Add the MCP server to your Claude Desktop configuration:

**macOS/Linux:** `~/.config/claude/claude_desktop_config.json`
**Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "aiserve-gpuproxy": {
      "command": "curl",
      "args": [
        "-X", "POST",
        "-H", "Content-Type: application/json",
        "-H", "X-API-Key: YOUR_API_KEY_HERE",
        "http://localhost:8080/api/v1/mcp"
      ]
    }
  }
}
```

Replace `YOUR_API_KEY_HERE` with your actual API key from the GPU proxy.

### Custom MCP Client

For custom MCP clients, connect to the MCP endpoint:

```
POST http://localhost:8080/api/v1/mcp
Headers:
  Content-Type: application/json
  X-API-Key: YOUR_API_KEY_HERE
```

## API Endpoints

### POST /api/v1/mcp
Main MCP protocol endpoint for JSON-RPC 2.0 requests.

**Authentication:** Required (API key or JWT)

**Request Format:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "list_gpu_instances",
    "arguments": {
      "provider": "all"
    }
  },
  "id": 1
}
```

**Response Format:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\n  \"instances\": [...]\n}"
      }
    ]
  },
  "id": 1
}
```

### GET /api/v1/mcp/sse
Server-Sent Events endpoint for streaming MCP responses.

**Authentication:** Required (API key or JWT)

## MCP Protocol Methods

### initialize
Initialize the MCP connection and retrieve server capabilities.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {},
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "protocolVersion": "2024-11-05",
    "serverInfo": {
      "name": "aiserve-gpuproxy",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {},
      "resources": {}
    }
  },
  "id": 1
}
```

### tools/list
List all available tools.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "params": {},
  "id": 2
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [
      {
        "name": "list_gpu_instances",
        "description": "List available GPU instances...",
        "inputSchema": {...}
      }
    ]
  },
  "id": 2
}
```

### tools/call
Execute a tool.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "list_gpu_instances",
    "arguments": {
      "provider": "vast.ai"
    }
  },
  "id": 3
}
```

### resources/list
List all available resources.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "resources/list",
  "params": {},
  "id": 4
}
```

### resources/read
Read a specific resource.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "resources/read",
  "params": {
    "uri": "gpu://instances"
  },
  "id": 5
}
```

## Usage Examples

### Example 1: List GPUs with Claude Desktop

After configuring the MCP server in Claude Desktop, you can interact with it naturally:

```
User: Show me all available GPU instances under $2/hour

Claude: I'll use the GPU proxy to list instances for you.
[Calls list_gpu_instances with max_price: 2.0]

Here are the available GPU instances under $2/hour:
1. vast.ai - RTX 3090 (24GB VRAM) - $0.50/hour
2. io.net - RTX 4090 (24GB VRAM) - $1.80/hour
...
```

### Example 2: Check Spending Before Creating Instance

```
User: Can I afford to spin up a $3/hour instance?

Claude: Let me check your current spending limits.
[Calls check_spending_limit with estimated_cost: 3.0]

Based on your guard rails:
- 60min window: $45 spent, limit $50 - OK
- 1440min window: $750 spent, limit $1000 - OK

Yes, you can afford this instance!
```

### Example 3: Create and Destroy Instance

```
User: Create instance 12345 on vast.ai

Claude: [Calls create_gpu_instance]
Instance 12345 created successfully on vast.ai!

User: Thanks, I'm done. Destroy it.

Claude: [Calls destroy_gpu_instance]
Instance 12345 has been destroyed.
```

### Example 4: Using curl

```bash
# Initialize
curl -X POST http://localhost:8080/api/v1/mcp \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "method": "initialize",
    "params": {},
    "id": 1
  }'

# List tools
curl -X POST http://localhost:8080/api/v1/mcp \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/list",
    "params": {},
    "id": 2
  }'

# List GPU instances
curl -X POST http://localhost:8080/api/v1/mcp \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "list_gpu_instances",
      "arguments": {"provider": "all"}
    },
    "id": 3
  }'
```

## Error Handling

MCP errors follow JSON-RPC 2.0 specification:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": {
      "details": "provider is required"
    }
  },
  "id": 1
}
```

### Error Codes

- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `-32000`: Server error (custom)

## Security

### Authentication

All MCP requests must include authentication via:
- **API Key**: `X-API-Key` header
- **JWT Token**: `Authorization: Bearer <token>` header

### Rate Limiting

MCP endpoints are subject to the same rate limiting as other API endpoints (100 requests/minute by default).

### Guard Rails

Guard rails spending limits apply to GPU operations initiated through MCP.

## Development

### Testing MCP Server

Use the included test script:

```bash
# Test initialize
./scripts/test-mcp.sh initialize

# Test list tools
./scripts/test-mcp.sh tools/list

# Test calling a tool
./scripts/test-mcp.sh tools/call list_gpu_instances '{"provider":"all"}'
```

### Custom MCP Client

Example Python client:

```python
import requests
import json

class GPUProxyMCPClient:
    def __init__(self, base_url, api_key):
        self.base_url = base_url
        self.headers = {
            'Content-Type': 'application/json',
            'X-API-Key': api_key
        }
        self.request_id = 0

    def call_tool(self, tool_name, arguments=None):
        self.request_id += 1
        payload = {
            'jsonrpc': '2.0',
            'method': 'tools/call',
            'params': {
                'name': tool_name,
                'arguments': arguments or {}
            },
            'id': self.request_id
        }

        response = requests.post(
            f'{self.base_url}/api/v1/mcp',
            headers=self.headers,
            json=payload
        )
        return response.json()

    def list_gpus(self, provider='all'):
        return self.call_tool('list_gpu_instances', {
            'provider': provider
        })

    def get_spending(self):
        return self.call_tool('get_spending_info')

# Usage
client = GPUProxyMCPClient('http://localhost:8080', 'your-api-key')
gpus = client.list_gpus(provider='vast.ai')
print(json.dumps(gpus, indent=2))
```

## Limitations

- WebSocket support: Coming soon
- Streaming responses: Currently via SSE only
- Notifications: Not yet implemented

## Troubleshooting

### Connection Issues

1. Verify server is running: `curl http://localhost:8080/health`
2. Check API key is valid
3. Ensure MCP endpoint is accessible: `curl -X POST http://localhost:8080/api/v1/mcp`

### Tool Call Failures

1. Check authentication headers
2. Verify tool name and parameters
3. Check server logs for errors
4. Ensure required parameters are provided

### Claude Desktop Integration Issues

1. Restart Claude Desktop after config changes
2. Check config file syntax is valid JSON
3. Verify API key has proper permissions
4. Test MCP endpoint directly with curl first

## Future Enhancements

Planned features:
- WebSocket support for bidirectional communication
- Streaming tool responses
- Server-initiated notifications
- Batch tool execution
- Tool composition and chaining
- Resource subscriptions for real-time updates

## References

- [Model Context Protocol Specification](https://modelcontextprotocol.io)
- [Claude Desktop MCP Documentation](https://docs.anthropic.com/claude/docs/model-context-protocol)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
