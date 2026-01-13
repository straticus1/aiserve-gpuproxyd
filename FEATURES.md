# AIServe GPU Proxy - Complete Feature Set

## Overview

AIServe GPU Proxy now includes advanced features for spending control, agent communication protocols, and enterprise logging.

## 1. Guard Rails - Spending Control System

### Description
Configurable spending limits across 17 time windows to prevent out-of-control GPU spending.

### Time Windows
- **Short-term**: 5min, 15min, 30min
- **Hourly**: 60min, 90min, 120min
- **Multi-hour**: 240min (4h), 300min (5h), 360min (6h), 400min, 460min, 520min
- **Half-day+**: 640min, 700min, 1440min (24h)
- **Multi-day**: 48h, 72h

### Configuration
```bash
GUARDRAILS_ENABLED=true
GUARDRAILS_MAX_60MIN_RATE=100.00    # $100/hour
GUARDRAILS_MAX_1440MIN_RATE=1000.00 # $1000/day
GUARDRAILS_MAX_72H_RATE=2500.00     # $2500/3 days
```

### Features
- Real-time spending tracking in Redis
- Automatic request blocking when limits exceeded
- Per-user spending isolation
- HTTP response headers with spending info
- Admin CLI commands for monitoring
- API endpoints for programmatic access

### API Endpoints
- `GET /api/v1/guardrails/spending` - View current spending
- `POST /api/v1/guardrails/spending/check` - Check if request allowed
- `POST /api/v1/guardrails/spending/record` - Record spending
- `POST /api/v1/guardrails/spending/reset` - Reset tracking

### Admin Commands
```bash
./bin/aiserve-gpuproxy-admin guardrails-status
./bin/aiserve-gpuproxy-admin guardrails-spending user@example.com
./bin/aiserve-gpuproxy-admin guardrails-reset user@example.com
```

### Documentation
See [GUARDRAILS.md](GUARDRAILS.md) for complete documentation.

---

## 2. Model Context Protocol (MCP) Support

### Description
Full MCP server implementation allowing AI assistants like Claude Desktop to directly manage GPU resources.

### Available Tools
1. **list_gpu_instances** - List available GPUs with filters
2. **create_gpu_instance** - Create GPU instance
3. **destroy_gpu_instance** - Destroy GPU instance
4. **get_spending_info** - Check guard rails spending
5. **check_spending_limit** - Validate spending limits
6. **get_billing_transactions** - View transaction history
7. **proxy_inference_request** - Proxy inference requests

### Resources
- `gpu://instances` - All available GPU instances
- `spending://current` - Current spending information

### Integration
#### Claude Desktop
Add to `~/.config/claude/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "aiserve-gpuproxy": {
      "command": "curl",
      "args": [
        "-X", "POST",
        "-H", "X-API-Key: YOUR_KEY",
        "http://localhost:8080/api/v1/mcp"
      ]
    }
  }
}
```

### Endpoints
- `POST /api/v1/mcp` - Main MCP endpoint
- `GET /api/v1/mcp/sse` - Server-Sent Events for streaming

### Testing
```bash
./scripts/test-mcp.sh initialize
./scripts/test-mcp.sh tools/list
./scripts/test-mcp.sh tools/call list_gpu_instances '{"provider":"all"}'
```

### Documentation
See [MCP.md](MCP.md) for complete MCP documentation.

---

## 3. Agent-to-Agent Protocol (A2A)

### Description
Standardized protocol for direct agent-to-agent communication with discovery and capability negotiation.

### Features
- Agent discovery and capability negotiation
- Structured message format with metadata
- Support for async communication
- Conversation tracking
- Priority levels and TTL

### Actions
- `agent.discover` - Get agent capabilities
- `agent.ping` - Health check
- `gpu.list` - List GPU instances
- `gpu.create` - Create instance
- `gpu.destroy` - Destroy instance
- `billing.transactions` - Get transactions
- `guardrails.spending` - Get spending info
- `guardrails.check` - Check spending limits
- `inference.proxy` - Proxy inference requests

### Request Format
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

### Response Format
```json
{
  "version": "1.0",
  "message_id": "uuid",
  "in_reply_to": "request-uuid",
  "from_agent": "aiserve-gpuproxy",
  "to_agent": "my-agent",
  "status": "success",
  "data": { ... },
  "timestamp": "2024-01-12T10:00:01Z"
}
```

### Endpoint
- `POST /api/v1/a2a` - A2A protocol endpoint
- `GET /agent/discover` - Agent discovery

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

## 4. Agent Communications Protocol (ACP)

### Description
Structured agent communication protocol with message types, priorities, and conversation tracking.

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

### Commands
- `gpu.list` - List GPU instances
- `gpu.create` - Create instance
- `gpu.destroy` - Destroy instance
- `billing.query` - Query billing
- `guardrails.check` - Check spending limits

### Queries
- `gpu.availability` - Check GPU availability
- `spending.current` - Current spending
- `capabilities` - Agent capabilities

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

### Response Format
```json
{
  "header": {
    "version": "1.0",
    "message_id": "uuid",
    "sender": "aiserve-gpuproxy",
    "recipient": "my-agent",
    "message_type": "response",
    "timestamp": "2024-01-12T10:00:01Z"
  },
  "payload": { ... },
  "status": {
    "code": 200,
    "message": "success",
    "success": true
  }
}
```

### Endpoint
- `POST /api/v1/acp` - ACP protocol endpoint

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

## 5. Centralized Logging (Syslog)

### Description
Enterprise-grade logging with support for syslog, file logging, and AISERVE_LOG_FILE environment variable.

### Features
- **Local Syslog**: Automatic /dev/log detection
- **Remote Syslog**: TCP/UDP to remote syslog servers
- **File Logging**: Direct to file with rotation support
- **Environment Variable**: AISERVE_LOG_FILE for easy configuration
- **Multi-Writer**: Log to both syslog/file and stdout
- **Structured Logging**: Component-based log messages
- **Log Levels**: EMERG, ALERT, CRIT, ERROR, WARN, NOTICE, INFO, DEBUG
- **Facilities**: LOG_LOCAL0 through LOG_LOCAL7

### Configuration

#### Remote Syslog
```bash
SYSLOG_ENABLED=true
SYSLOG_NETWORK=tcp
SYSLOG_ADDRESS=logs.example.com:514
SYSLOG_TAG=aiserve-gpuproxy
SYSLOG_FACILITY=LOG_LOCAL0
```

#### Local Syslog (Auto-detect)
```bash
SYSLOG_ENABLED=true
# Will auto-detect /dev/log
```

#### Unix Socket
```bash
SYSLOG_ENABLED=true
SYSLOG_NETWORK=unix
SYSLOG_ADDRESS=/dev/log
```

#### File Logging
```bash
# Option 1: Using LOG_FILE
SYSLOG_ENABLED=true
LOG_FILE=/var/log/aiserve-gpuproxy.log

# Option 2: Using AISERVE_LOG_FILE environment variable
export AISERVE_LOG_FILE=/var/log/aiserve-gpuproxy.log
```

### Log Levels

#### Emergency (EMERG)
System is unusable - immediate action required
```go
logging.GetLogger().Emergency("Database connection lost")
```

#### Alert
Action must be taken immediately
```go
logging.GetLogger().Alert("Disk space critical: 95% full")
```

#### Critical (CRIT)
Critical conditions
```go
logging.GetLogger().Critical("GPU provider API unavailable")
```

#### Error
Error conditions
```go
logging.GetLogger().Error("Failed to create GPU instance")
```

#### Warning (WARN)
Warning conditions
```go
logging.GetLogger().Warning("Approaching spending limit: 90%")
```

#### Notice
Normal but significant condition
```go
logging.GetLogger().Notice("Guard rails limit updated")
```

#### Info
Informational messages
```go
logging.GetLogger().Info("Request completed successfully")
```

#### Debug
Debug-level messages
```go
logging.GetLogger().Debug("Cache hit for user data")
```

### Structured Logging Helpers

```go
// Log HTTP request
logging.LogRequest("POST", "/api/v1/gpu/instances", "192.168.1.100", 200, 150)

// Log error with context
logging.LogError("gpu-service", "Failed to create instance", err)

// Log info with context
logging.LogInfo("billing", "Payment processed successfully")

// Log warning
logging.LogWarning("guardrails", "Approaching 60min spending limit")

// Log debug
logging.LogDebug("cache", "Cache miss for user preferences")
```

### Example Output

Syslog format:
```
Jan 12 10:30:15 hostname aiserve-gpuproxy[1234]: [INFO] method=POST path=/api/v1/gpu/instances remote=192.168.1.100 status=200 duration=150ms
Jan 12 10:30:16 hostname aiserve-gpuproxy[1234]: [ERROR] component=gpu-service message=Failed to create instance error=connection timeout
```

File format:
```
[INFO] method=POST path=/api/v1/gpu/instances remote=192.168.1.100 status=200 duration=150ms
[ERROR] component=gpu-service message=Failed to create instance error=connection timeout
```

### Integration

The logging system is automatically integrated into all components:
- HTTP middleware logs all requests
- GPU service logs operations
- Billing service logs transactions
- Guard rails logs limit violations
- MCP/A2A/ACP servers log protocol messages

### Monitoring

View logs in real-time:

```bash
# Syslog
tail -f /var/log/syslog | grep aiserve-gpuproxy

# File
tail -f /var/log/aiserve-gpuproxy.log

# With log rotation
tail -f /var/log/aiserve-gpuproxy.log | grep ERROR
```

---

## Quick Start with All Features

### 1. Configure Environment
```bash
# Copy example config
cp .env.example .env

# Edit .env
vim .env
```

### 2. Enable Guard Rails
```env
GUARDRAILS_ENABLED=true
GUARDRAILS_MAX_60MIN_RATE=100.00
GUARDRAILS_MAX_1440MIN_RATE=1000.00
```

### 3. Enable Logging
```env
# Option 1: Syslog
SYSLOG_ENABLED=true
SYSLOG_ADDRESS=/dev/log

# Option 2: File
AISERVE_LOG_FILE=/var/log/aiserve-gpuproxy.log
```

### 4. Start Server
```bash
make build
./bin/aiserve-gpuproxy-admin migrate
./bin/aiserve-gpuproxy
```

### 5. Create API Key
```bash
./bin/aiserve-gpuproxy-admin create-user user@example.com password123 "John Doe"
./bin/aiserve-gpuproxy-admin create-apikey user@example.com "Production Key"
```

### 6. Test All Protocols

#### HTTP API
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/api/v1/gpu/instances
```

#### Guard Rails
```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/api/v1/guardrails/spending
```

#### MCP
```bash
./scripts/test-mcp.sh tools/list
```

#### A2A
```bash
curl -X POST -H "X-API-Key: $KEY" \
  http://localhost:8080/api/v1/a2a \
  -d '{"action":"agent.discover"}'
```

#### ACP
```bash
curl -X POST -H "X-API-Key: $KEY" \
  http://localhost:8080/api/v1/acp \
  -d '{"header":{"sender":"test","message_type":"query"},"payload":{"query":"capabilities"}}'
```

---

## Summary

AIServe GPU Proxy now includes:

1. **Guard Rails**: 17 time windows, real-time tracking, automatic blocking
2. **MCP Server**: 7 tools, 2 resources, Claude Desktop integration
3. **A2A Protocol**: Agent discovery, structured messaging, async support
4. **ACP Protocol**: 5 message types, 4 priority levels, conversation tracking
5. **Syslog Logging**: Remote/local syslog, file logging, 8 log levels

All features are production-ready and fully documented.
