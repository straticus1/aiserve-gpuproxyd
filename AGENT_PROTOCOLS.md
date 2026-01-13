# Agent Communication Protocols

AIServe GPU Proxy supports **6 major agent communication protocols**, making it compatible with virtually all agent frameworks and systems.

## Supported Protocols

1. **MCP** - Model Context Protocol (Anthropic)
2. **A2A** - Agent-to-Agent Protocol
3. **ACP** - Agent Communications Protocol
4. **FIPA ACL** - Foundation for Intelligent Physical Agents (Academic/Research Standard)
5. **KQML** - Knowledge Query and Manipulation Language (Legacy Standard)
6. **LangChain** - LangChain Agent Protocol (Python/TypeScript Framework)

## Quick Comparison

| Protocol | Best For | Complexity | Industry | Auto-Detect |
|----------|---------|------------|----------|-------------|
| **MCP** | Claude Desktop, AI assistants | Medium | Production | ✅ |
| **A2A** | Custom agent systems | Low | Production | ✅ |
| **ACP** | Enterprise agent networks | Medium | Production | ✅ |
| **FIPA ACL** | Academic research, multi-agent systems | High | Research | ✅ |
| **KQML** | Legacy systems, research | Medium | Legacy | ✅ |
| **LangChain** | Python/TS apps, LLM chains | Low | Production | ✅ |

---

## 1. MCP (Model Context Protocol)

### Overview
Modern protocol from Anthropic for connecting AI assistants to external tools and data.

### Endpoint
`POST /api/v1/mcp`

### Example
```bash
curl -X POST http://localhost:8080/api/v1/mcp \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "list_gpu_instances",
      "arguments": {"provider": "all"}
    },
    "id": 1
  }'
```

### Documentation
See [MCP.md](MCP.md) for complete documentation.

---

## 2. A2A (Agent-to-Agent Protocol)

### Overview
Simple, flexible protocol for direct agent communication with discovery and metadata.

### Endpoint
`POST /api/v1/a2a`

### Message Format
```json
{
  "version": "1.0",
  "message_id": "uuid",
  "from_agent": "my-agent",
  "to_agent": "aiserve-gpuproxy",
  "action": "gpu.list",
  "parameters": {
    "provider": "vast.ai"
  },
  "timestamp": "2024-01-12T10:00:00Z"
}
```

### Supported Actions
- `agent.discover` - Get agent capabilities
- `agent.ping` - Health check
- `gpu.list` - List GPU instances
- `gpu.create` - Create instance
- `gpu.destroy` - Destroy instance
- `billing.transactions` - Get transactions
- `guardrails.spending` - Get spending info
- `guardrails.check` - Check spending limits

### Example
```bash
curl -X POST http://localhost:8080/api/v1/a2a \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "action": "gpu.list",
    "from_agent": "my-agent",
    "parameters": {"provider": "all"}
  }'
```

---

## 3. ACP (Agent Communications Protocol)

### Overview
Structured protocol with message types, priorities, and conversation tracking.

### Endpoint
`POST /api/v1/acp`

### Message Types
- `request` - Request for action
- `response` - Response to request
- `notification` - One-way notification
- `query` - Information query
- `command` - Command execution
- `event` - Event notification

### Priority Levels
- `low` - Background operations
- `normal` - Standard requests
- `high` - Important operations
- `critical` - Emergency operations

### Message Format
```json
{
  "header": {
    "version": "1.0",
    "message_id": "uuid",
    "conversation_id": "conv-uuid",
    "sender": "my-agent",
    "recipient": "aiserve-gpuproxy",
    "message_type": "command",
    "timestamp": "2024-01-12T10:00:00Z",
    "priority": "normal"
  },
  "payload": {
    "command": "gpu.list",
    "parameters": {
      "provider": "all"
    }
  }
}
```

### Example
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
      "parameters": {"provider": "vast.ai"}
    }
  }'
```

---

## 4. FIPA ACL (Foundation for Intelligent Physical Agents)

### Overview
Academic/research standard for multi-agent systems with formal semantics and interaction protocols.

### Endpoint
`POST /api/v1/fipa`

### Communicative Acts (Performatives)
- **Information**: `inform`, `confirm`, `disconfirm`, `not-understood`
- **Requests**: `query-if`, `query-ref`, `subscribe`, `request`
- **Negotiation**: `propose`, `accept-proposal`, `reject-proposal`, `cfp`
- **Agreement**: `agree`, `refuse`, `failure`, `cancel`

### Interaction Protocols
- `fipa-request` - Request protocol
- `fipa-query` - Query protocol
- `fipa-contract-net` - Contract Net protocol
- `fipa-subscribe` - Subscribe protocol
- `fipa-propose` - Propose protocol

### Message Format
```json
{
  "performative": "query-ref",
  "sender": {
    "name": "my-agent",
    "addresses": ["http://localhost:8081"]
  },
  "receiver": [{
    "name": "aiserve-gpuproxy"
  }],
  "content": {
    "query": "gpu-instances",
    "provider": "all"
  },
  "language": "JSON",
  "ontology": "gpu-proxy-ontology",
  "protocol": "fipa-query",
  "conversation-id": "conv-123",
  "reply-with": "msg-456"
}
```

### Example - Query GPUs
```bash
curl -X POST http://localhost:8080/api/v1/fipa \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "performative": "query-ref",
    "sender": {"name": "my-agent"},
    "receiver": [{"name": "aiserve-gpuproxy"}],
    "content": {
      "query": "gpu-instances",
      "provider": "vast.ai"
    }
  }'
```

### Example - Request Action
```bash
curl -X POST http://localhost:8080/api/v1/fipa \
  -H "X-API-Key": YOUR_KEY" \
  -d '{
    "performative": "request",
    "sender": {"name": "my-agent"},
    "receiver": [{"name": "aiserve-gpuproxy"}],
    "content": {
      "action": "create-gpu-instance",
      "provider": "vast.ai",
      "instance_id": "12345"
    }
  }'
```

### Example - Check Condition
```bash
curl -X POST http://localhost:8080/api/v1/fipa \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "performative": "query-if",
    "sender": {"name": "my-agent"},
    "receiver": [{"name": "aiserve-gpuproxy"}],
    "content": {
      "condition": "gpu-available",
      "provider": "vast.ai"
    }
  }'
```

---

## 5. KQML (Knowledge Query and Manipulation Language)

### Overview
Legacy academic standard for agent communication, precursor to FIPA ACL.

### Endpoint
`POST /api/v1/kqml`

### Performatives
- **Information**: `ask`, `ask-one`, `ask-all`, `tell`, `untell`, `reply`, `sorry`
- **Action**: `achieve`, `cancel`
- **Capability**: `advertise`, `subscribe`, `monitor`
- **Network**: `register`, `unregister`, `forward`, `broadcast`
- **Meta**: `error`

### Message Format (JSON)
```json
{
  "performative": "ask",
  "sender": "my-agent",
  "receiver": "aiserve-gpuproxy",
  "reply-with": "msg-123",
  "language": "JSON",
  "ontology": "gpu-proxy-v1",
  "content": {
    "query": "gpu-instances",
    "provider": "all"
  }
}
```

### Example - Ask Query
```bash
curl -X POST http://localhost:8080/api/v1/kqml \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "performative": "ask",
    "sender": "my-agent",
    "content": {
      "query": "gpu-instances",
      "provider": "vast.ai"
    }
  }'
```

### Example - Achieve Goal
```bash
curl -X POST http://localhost:8080/api/v1/kqml \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "performative": "achieve",
    "sender": "my-agent",
    "content": {
      "goal": "create-gpu-instance",
      "provider": "vast.ai",
      "instance_id": "12345"
    }
  }'
```

### Example - Advertise Capabilities
```bash
curl -X POST http://localhost:8080/api/v1/kqml \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "performative": "advertise",
    "sender": "my-agent"
  }'
```

---

## 6. LangChain Agent Protocol

### Overview
Protocol for LangChain framework (Python/TypeScript), widely used in LLM applications.

### Endpoints
- `GET /api/v1/langchain/tools` - Get available tools
- `POST /api/v1/langchain` - Execute chain/tool

### Available Tools
1. **list_gpu_instances** - List available GPUs
2. **create_gpu_instance** - Create GPU instance
3. **destroy_gpu_instance** - Destroy GPU instance
4. **get_spending_info** - Get guard rails spending
5. **check_spending_limit** - Check spending limits
6. **get_billing_transactions** - Get transactions
7. **record_spending** - Record spending

### Get Tools
```bash
curl http://localhost:8080/api/v1/langchain/tools \
  -H "X-API-Key: YOUR_KEY"
```

Response:
```json
{
  "tools": [
    {
      "name": "list_gpu_instances",
      "description": "List available GPU instances...",
      "parameters": {
        "type": "object",
        "properties": {
          "provider": {...},
          "min_vram": {...}
        }
      }
    }
  ]
}
```

### Execute Tool
```bash
curl -X POST http://localhost:8080/api/v1/langchain \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "input": {
      "action": "execute",
      "tool": "list_gpu_instances",
      "tool_input": {
        "provider": "all",
        "max_price": 2.0
      }
    }
  }'
```

Response:
```json
{
  "output": {
    "result": {
      "tool": "list_gpu_instances",
      "content": {
        "instances": [...],
        "count": 5
      }
    }
  },
  "steps": [
    {
      "action": {
        "tool": "list_gpu_instances",
        "tool_input": {...}
      },
      "observation": "..."
    }
  ]
}
```

### Python Example
```python
from langchain.agents import Tool
import requests

tools = [
    Tool(
        name="list_gpu_instances",
        func=lambda provider: requests.post(
            "http://localhost:8080/api/v1/langchain",
            headers={"X-API-Key": "YOUR_KEY"},
            json={
                "input": {
                    "action": "execute",
                    "tool": "list_gpu_instances",
                    "tool_input": {"provider": provider}
                }
            }
        ).json(),
        description="List GPU instances from providers"
    )
]
```

---

## Unified Agent Endpoint (Auto-Detection)

### Overview
Single endpoint that automatically detects the protocol and routes to the appropriate handler.

### Endpoint
`POST /api/v1/agent`

### Supported Protocols
All protocols are auto-detected based on message structure:
- MCP (JSON-RPC 2.0 format)
- A2A (has `action` and `from_agent`)
- ACP (has `header` with `message_type`)
- FIPA ACL (has `performative` and FIPA-specific terms)
- KQML (has `performative` and KQML-specific terms)
- LangChain (has `input` with `tool`)

### Example
```bash
# Sends A2A message, auto-detected
curl -X POST http://localhost:8080/api/v1/agent \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "action": "gpu.list",
    "from_agent": "my-agent",
    "parameters": {"provider": "all"}
  }'

# Sends FIPA message, auto-detected
curl -X POST http://localhost:8080/api/v1/agent \
  -H "X-API-Key: YOUR_KEY" \
  -d '{
    "performative": "query-ref",
    "sender": {"name": "my-agent"},
    "receiver": [{"name": "aiserve-gpuproxy"}],
    "content": {"query": "gpu-instances"}
  }'
```

---

## Protocol Selection Guide

### Choose MCP when:
- Integrating with Claude Desktop
- Building AI assistant tools
- Need modern, well-documented protocol

### Choose A2A when:
- Building custom agent systems
- Need simple, flexible protocol
- Want minimal overhead

### Choose ACP when:
- Need structured messages with priorities
- Building enterprise agent networks
- Want conversation tracking

### Choose FIPA ACL when:
- Working on academic research
- Need formal semantics
- Building complex multi-agent systems
- Using established interaction protocols

### Choose KQML when:
- Maintaining legacy systems
- Need academic compatibility
- Simple query/response patterns

### Choose LangChain when:
- Using LangChain framework (Python/TS)
- Building LLM applications
- Need ecosystem compatibility

### Use Unified Endpoint when:
- Supporting multiple agent types
- Don't know protocol in advance
- Want automatic routing

---

## Common Operations

### List GPUs
<details>
<summary>All Protocols</summary>

**MCP:**
```json
{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": "list_gpu_instances", "arguments": {"provider": "all"}}, "id": 1}
```

**A2A:**
```json
{"action": "gpu.list", "from_agent": "my-agent", "parameters": {"provider": "all"}}
```

**ACP:**
```json
{"header": {"sender": "my-agent", "message_type": "command"}, "payload": {"command": "gpu.list", "parameters": {"provider": "all"}}}
```

**FIPA:**
```json
{"performative": "query-ref", "sender": {"name": "my-agent"}, "receiver": [{"name": "aiserve-gpuproxy"}], "content": {"query": "gpu-instances", "provider": "all"}}
```

**KQML:**
```json
{"performative": "ask", "sender": "my-agent", "content": {"query": "gpu-instances", "provider": "all"}}
```

**LangChain:**
```json
{"input": {"action": "execute", "tool": "list_gpu_instances", "tool_input": {"provider": "all"}}}
```
</details>

### Create GPU Instance
<details>
<summary>All Protocols</summary>

**MCP:**
```json
{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": "create_gpu_instance", "arguments": {"provider": "vast.ai", "instance_id": "12345"}}, "id": 1}
```

**A2A:**
```json
{"action": "gpu.create", "from_agent": "my-agent", "parameters": {"provider": "vast.ai", "instance_id": "12345"}}
```

**ACP:**
```json
{"header": {"sender": "my-agent", "message_type": "command"}, "payload": {"command": "gpu.create", "parameters": {"provider": "vast.ai", "instance_id": "12345"}}}
```

**FIPA:**
```json
{"performative": "request", "sender": {"name": "my-agent"}, "receiver": [{"name": "aiserve-gpuproxy"}], "content": {"action": "create-gpu-instance", "provider": "vast.ai", "instance_id": "12345"}}
```

**KQML:**
```json
{"performative": "achieve", "sender": "my-agent", "content": {"goal": "create-gpu-instance", "provider": "vast.ai", "instance_id": "12345"}}
```

**LangChain:**
```json
{"input": {"action": "execute", "tool": "create_gpu_instance", "tool_input": {"provider": "vast.ai", "instance_id": "12345"}}}
```
</details>

### Check Spending
<details>
<summary>All Protocols</summary>

**MCP:**
```json
{"jsonrpc": "2.0", "method": "tools/call", "params": {"name": "get_spending_info"}, "id": 1}
```

**A2A:**
```json
{"action": "guardrails.spending", "from_agent": "my-agent"}
```

**ACP:**
```json
{"header": {"sender": "my-agent", "message_type": "query"}, "payload": {"query": "spending.current"}}
```

**FIPA:**
```json
{"performative": "query-ref", "sender": {"name": "my-agent"}, "receiver": [{"name": "aiserve-gpuproxy"}], "content": {"query": "spending-info"}}
```

**KQML:**
```json
{"performative": "ask", "sender": "my-agent", "content": {"query": "spending-info"}}
```

**LangChain:**
```json
{"input": {"action": "execute", "tool": "get_spending_info"}}
```
</details>

---

## Production Deployment

### Recommended Setup
```nginx
# Nginx routing example
location /api/v1/mcp { proxy_pass http://gpuproxy:8080; }
location /api/v1/a2a { proxy_pass http://gpuproxy:8080; }
location /api/v1/acp { proxy_pass http://gpuproxy:8080; }
location /api/v1/fipa { proxy_pass http://gpuproxy:8080; }
location /api/v1/kqml { proxy_pass http://gpuproxy:8080; }
location /api/v1/langchain { proxy_pass http://gpuproxy:8080; }
location /api/v1/agent { proxy_pass http://gpuproxy:8080; }  # Auto-detect
```

### Monitoring
All protocols log to syslog/file with protocol identification:
```
[INFO] MCP Request: {"jsonrpc":"2.0"...}
[INFO] A2A Request: {"action":"gpu.list"...}
[INFO] FIPA ACL Message: {"performative":"query-ref"...}
[INFO] KQML Message: {"performative":"ask"...}
[INFO] LangChain Request: {"input":...}
```

---

## Summary

AIServe GPU Proxy is now **production-ready for all major agent communication standards**:

- ✅ **Modern**: MCP (Anthropic), LangChain
- ✅ **Custom**: A2A, ACP
- ✅ **Standard**: FIPA ACL (IEEE), KQML (DARPA)
- ✅ **Auto-Detection**: Unified endpoint
- ✅ **Production**: Full logging, auth, guard rails

**Total Protocol Coverage**: 6 protocols + 1 unified endpoint = **Complete agent interoperability**
