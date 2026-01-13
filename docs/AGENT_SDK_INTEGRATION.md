# Agent SDK Go Integration Guide

## Overview

Integrate Claude Agent SDK (agent-sdk-go) with GPU Proxy to enable autonomous AI agents that can orchestrate GPU operations, manage resources intelligently, and execute multi-step workflows.

## Why Agent SDK + GPU Proxy?

**Autonomous GPU Orchestration:**
- Agents automatically select optimal GPUs based on workload requirements
- Intelligent cost optimization across providers
- Self-healing infrastructure (auto-restart failed instances)
- Dynamic scaling based on queue depth
- Multi-step workflow execution without human intervention

**Use Cases:**
- **Auto-Scaling**: Agent monitors metrics and provisions/destroys GPUs automatically
- **Cost Optimization**: Agent analyzes usage patterns and recommends GPU tier changes
- **Batch Processing**: Agent manages queue → provision → process → cleanup lifecycle
- **Multi-Model Routing**: Agent routes requests to optimal GPU based on model requirements
- **Fault Tolerance**: Agent detects failures and automatically switches to backup GPUs

## Architecture

```
┌─────────────────┐
│  Claude Agent   │
│   (SDK Agent)   │
└────────┬────────┘
         │
         │ Uses Tools
         ▼
┌────────────────────────────────┐
│     GPU Proxy MCP Server       │
│  (Exposes GPU Ops as Tools)    │
└───────────┬────────────────────┘
            │
            │ Calls APIs
            ▼
┌────────────────────────────────┐
│      GPU Proxy REST API        │
│   /api/v1/gpu/*                │
└────────────────────────────────┘
```

## Implementation Steps

### Step 1: Install Dependencies

```bash
go get github.com/anthropics/agent-sdk-go
go get github.com/anthropics/mcp-golang
```

### Step 2: Create MCP Server for GPU Operations

Create `cmd/mcp-server/main.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/anthropics/mcp-golang/mcp"
)

type GPUProxyMCP struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewGPUProxyMCP() *GPUProxyMCP {
	return &GPUProxyMCP{
		baseURL: os.Getenv("GPU_PROXY_URL"),
		apiKey:  os.Getenv("GPU_PROXY_API_KEY"),
		client:  &http.Client{},
	}
}

func (g *GPUProxyMCP) callAPI(method, path string, body interface{}) (map[string]interface{}, error) {
	// Implementation of API calls
	// ...
}

func main() {
	gp := NewGPUProxyMCP()

	server := mcp.NewServer(mcp.ServerOptions{
		Name:    "gpuproxy",
		Version: "1.0.0",
	})

	// Register tools
	server.AddTool(mcp.Tool{
		Name:        "provision_gpu",
		Description: "Provision a GPU instance based on preferences",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gpu_model": map[string]interface{}{
					"type":        "string",
					"description": "GPU model (H100, A100, etc.)",
				},
				"provider": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"vastai", "ionet"},
					"description": "GPU provider",
				},
				"duration": map[string]interface{}{
					"type":        "number",
					"description": "Duration in seconds (optional)",
				},
			},
			"required": []string{"gpu_model"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			gpuModel := args["gpu_model"].(string)
			provider := "vastai"
			if p, ok := args["provider"].(string); ok {
				provider = p
			}

			// Call GPU Proxy API
			result, err := gp.callAPI("POST", "/api/v1/gpu/instances/reserve", map[string]interface{}{
				"preferred_gpus": []map[string]interface{}{
					{
						"model":    gpuModel,
						"priority": 1,
					},
				},
				"provider": provider,
			})

			return result, err
		},
	})

	server.AddTool(mcp.Tool{
		Name:        "get_gpu_stats",
		Description: "Get GPU usage statistics and metrics",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return gp.callAPI("GET", "/api/v1/stats", nil)
		},
	})

	server.AddTool(mcp.Tool{
		Name:        "list_gpu_instances",
		Description: "List all active GPU instances",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return gp.callAPI("GET", "/api/v1/gpu/instances", nil)
		},
	})

	server.AddTool(mcp.Tool{
		Name:        "destroy_gpu_instance",
		Description: "Destroy a specific GPU instance",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"provider": map[string]interface{}{
					"type":        "string",
					"description": "Provider name (vastai, ionet)",
				},
				"instance_id": map[string]interface{}{
					"type":        "string",
					"description": "Instance identifier",
				},
			},
			"required": []string{"provider", "instance_id"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			provider := args["provider"].(string)
			instanceID := args["instance_id"].(string)
			path := fmt.Sprintf("/api/v1/gpu/instances/%s/%s", provider, instanceID)
			return gp.callAPI("DELETE", path, nil)
		},
	})

	server.AddTool(mcp.Tool{
		Name:        "cleanup_idle_gpus",
		Description: "Find and destroy GPU instances that have been idle for a specified duration",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"idle_threshold_minutes": map[string]interface{}{
					"type":        "number",
					"description": "Minutes of idle time before cleanup (default: 30)",
					"default":     30,
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			thresholdMinutes := 30.0
			if t, ok := args["idle_threshold_minutes"].(float64); ok {
				thresholdMinutes = t
			}

			// Get all instances
			instances, err := gp.callAPI("GET", "/api/v1/gpu/instances", nil)
			if err != nil {
				return nil, err
			}

			// Filter and cleanup idle instances
			cleanedUp := []map[string]interface{}{}
			// ... cleanup logic ...

			return map[string]interface{}{
				"cleaned_up_count": len(cleanedUp),
				"instances":        cleanedUp,
			}, nil
		},
	})

	server.AddTool(mcp.Tool{
		Name:        "get_health_status",
		Description: "Check health status of GPU Proxy services",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return gp.callAPI("GET", "/health", nil)
		},
	})

	server.AddTool(mcp.Tool{
		Name:        "get_available_gpus",
		Description: "Get list of available GPU models with specifications",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vendor": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"NVIDIA", "AMD", "Intel", "Apple"},
					"description": "Filter by GPU vendor",
				},
				"tier": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"enterprise", "high_end", "mid_range", "budget"},
					"description": "Filter by performance tier",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return gp.callAPI("GET", "/api/v1/gpu/available", args)
		},
	})

	log.Println("Starting GPU Proxy MCP Server on stdio...")
	if err := server.ServeStdio(); err != nil {
		log.Fatal(err)
	}
}
```

### Step 3: Create Agent with GPU Management Capabilities

Create `cmd/agent/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/anthropics/agent-sdk-go/agent"
	"github.com/anthropics/agent-sdk-go/mcp"
)

func main() {
	// Initialize MCP client for GPU Proxy
	mcpClient, err := mcp.NewClient(mcp.ClientOptions{
		Command: "./bin/mcp-server",  // Path to MCP server binary
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mcpClient.Close()

	// Create agent with GPU management instructions
	agentInstance := agent.New(agent.Options{
		APIKey: os.Getenv("ANTHROPIC_API_KEY"),
		Model:  "claude-sonnet-4-5-20250929",
		Instructions: `You are an autonomous GPU infrastructure manager for aiserve.farm.

Your responsibilities:
1. Monitor GPU usage and costs continuously
2. Provision GPUs when demand is high (>80% utilization)
3. Clean up idle GPUs (idle > 30 minutes) to save costs
4. Route AI workloads to optimal GPU types based on requirements
5. Maintain 99.9% uptime SLA
6. Keep daily costs under budget ($5000/day)

You have access to tools for:
- Provisioning GPUs (provision_gpu)
- Listing instances (list_gpu_instances)
- Destroying instances (destroy_gpu_instance)
- Getting statistics (get_gpu_stats)
- Cleaning up idle GPUs (cleanup_idle_gpus)
- Checking health (get_health_status)
- Finding available GPUs (get_available_gpus)

Always explain your reasoning before taking action.
Prioritize cost efficiency while maintaining performance.`,
		MCPServers: []mcp.Client{mcpClient},
	})

	// Example: Auto-scaling workflow
	ctx := context.Background()

	// Check stats and make decisions
	response, err := agentInstance.Run(ctx, agent.Message{
		Role: "user",
		Content: `Check the current GPU statistics. If we have more than 100 requests in flight
		and active GPU instances are less than 10, provision 2 additional H100 GPUs.
		Also check for any idle GPUs (idle > 30 min) and clean them up to save costs.`,
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Agent Response:\n%s\n", response.Content)

	// The agent will:
	// 1. Call get_gpu_stats to check metrics
	// 2. Analyze if scaling is needed
	// 3. Call provision_gpu if needed
	// 4. Call cleanup_idle_gpus
	// 5. Report results
}
```

### Step 4: Create Autonomous Agent Loop

Create `cmd/agent-loop/main.go`:

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/anthropics/agent-sdk-go/agent"
)

func main() {
	agentInstance := initializeAgent() // From Step 3

	// Run agent loop every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Println("Starting autonomous GPU management agent...")

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

			response, err := agentInstance.Run(ctx, agent.Message{
				Role: "user",
				Content: `Perform your routine GPU infrastructure check:
				1. Get current statistics
				2. Check if auto-scaling is needed
				3. Clean up idle GPUs
				4. Verify health status
				5. Report any anomalies or cost concerns`,
			})

			cancel()

			if err != nil {
				log.Printf("Agent error: %v", err)
				continue
			}

			log.Printf("Agent Action Report:\n%s\n", response.Content)
		}
	}
}
```

## Example Workflows

### 1. Cost-Aware Auto-Scaling

```go
message := agent.Message{
	Role: "user",
	Content: `We have a $5000 daily budget. Current time is 3 PM.
	Check GPU stats. If we're trending to exceed budget, scale down by:
	1. Destroying non-critical GPU instances
	2. Routing new requests to cheaper GPUs (A100 instead of H100)
	3. Queueing low-priority requests

	If we're under 50% budget and have high queue depth, scale up with H100s.`,
}
```

The agent will:
- Call `get_gpu_stats` to check costs
- Analyze budget trajectory
- Call `destroy_gpu_instance` for scale-down
- Call `provision_gpu` for scale-up
- Provide detailed reasoning

### 2. Intelligent Workload Routing

```go
message := agent.Message{
	Role: "user",
	Content: `New AI inference request:
	Model: LLaMA-70B
	Input: 2048 tokens
	Priority: High

	Find the optimal GPU for this workload considering:
	- VRAM requirements (minimum 40GB)
	- Cost per hour
	- Current availability
	- Queue depth

	Provision if needed, or queue if no capacity.`,
}
```

The agent will:
- Call `get_available_gpus` to find suitable GPUs
- Analyze VRAM requirements
- Check cost vs priority
- Call `list_gpu_instances` to check availability
- Make provisioning decision
- Return routing recommendation

### 3. Fault-Tolerant Operation

```go
message := agent.Message{
	Role: "user",
	Content: `GPU instance vast-ai-12345 has failed health checks 3 times.
	Implement failover:
	1. Destroy the failed instance
	2. Provision replacement on different provider (IONet)
	3. Migrate pending workloads
	4. Update routing tables
	5. Send alert to ops team`,
}
```

### 4. Batch Processing Pipeline

```go
message := agent.Message{
	Role: "user",
	Content: `We have 1000 images to process with Stable Diffusion.
	Estimated time: 5 hours with 1 GPU.

	Create optimal batch processing plan:
	1. Determine how many RTX 4090 GPUs to provision
	2. Split workload across GPUs
	3. Monitor progress every 30 min
	4. Handle failures with retries
	5. Clean up GPUs when done
	6. Total cost must be < $100`,
}
```

## Advanced Patterns

### Multi-Agent Orchestration

```go
// Cost Optimizer Agent
costAgent := agent.New(agent.Options{
	Instructions: "Optimize GPU costs while maintaining SLA...",
	// ...
})

// Performance Optimizer Agent
perfAgent := agent.New(agent.Options{
	Instructions: "Maximize throughput and minimize latency...",
	// ...
})

// Coordinator Agent
coordinator := agent.New(agent.Options{
	Instructions: `You coordinate between cost optimizer and performance optimizer.
	Balance their recommendations to achieve optimal cost/performance ratio.`,
	Agents: []agent.Agent{costAgent, perfAgent},
})
```

### Learning from Historical Data

```go
message := agent.Message{
	Role: "user",
	Content: `Analyze the last 7 days of GPU usage statistics.
	Identify patterns:
	1. Peak usage hours
	2. Most cost-effective GPU types
	3. Average request duration by model
	4. Cost per inference by GPU type

	Recommend:
	1. Reserved instances for base load
	2. Auto-scaling rules
	3. GPU preference updates`,
}
```

## Integration with Existing Protocols

### A2A Protocol

```go
// Agent responds to A2A messages
server.AddHandler("a2a:request", func(msg A2AMessage) {
	agentInstance.Run(ctx, agent.Message{
		Role:    "user",
		Content: fmt.Sprintf("Handle A2A request: %v", msg),
	})
})
```

### MCP Protocol

Already integrated via MCP server!

### CUIC Protocol

```go
// Agent publishes capabilities via CUIC
cuicServer.RegisterCapability(cuic.Capability{
	Type: "gpu-provisioning",
	Description: "Autonomous GPU infrastructure management",
	Agent: agentInstance,
})
```

## Deployment

### Docker Compose

```yaml
services:
  agent-manager:
    build: ./cmd/agent-loop
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - GPU_PROXY_URL=http://gpuproxy:8080
      - GPU_PROXY_API_KEY=${GPU_PROXY_API_KEY}
    depends_on:
      - gpuproxy
      - mcp-server

  mcp-server:
    build: ./cmd/mcp-server
    environment:
      - GPU_PROXY_URL=http://gpuproxy:8080
      - GPU_PROXY_API_KEY=${GPU_PROXY_API_KEY}
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gpu-agent-manager
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: agent
        image: gpuproxy/agent-manager:latest
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: anthropic-creds
              key: api-key
```

## Monitoring & Observability

Agent actions are automatically logged:

```json
{
  "timestamp": "2026-01-13T12:34:56Z",
  "agent_id": "gpu-manager-001",
  "action": "provision_gpu",
  "reasoning": "Queue depth exceeded 100, need 2 more H100s",
  "result": "success",
  "cost_impact": "$6.00/hour"
}
```

## Security Considerations

1. **API Key Management**: Store in secrets, never in code
2. **Rate Limiting**: Prevent agent from making too many API calls
3. **Budget Limits**: Hard cap on daily spending
4. **Approval Gates**: Require human approval for large expenses
5. **Audit Logging**: Log all agent decisions and actions

## Cost Estimation

**Agent SDK Costs:**
- Claude API calls: ~$0.01-0.10 per decision
- Running every 5 minutes: ~288 calls/day = $2.88-28.80/day

**GPU Cost Savings:**
- Idle cleanup: Save 20-30% on unused GPUs
- Optimal routing: Save 10-15% by using right GPU for workload
- Auto-scaling: Reduce over-provisioning by 40%

**ROI:** Typical savings of $500-2000/day for infrastructure > $5000/day

## Next Steps

1. Implement MCP server (`cmd/mcp-server`)
2. Create basic agent (`cmd/agent`)
3. Test with single workflow
4. Add autonomous loop (`cmd/agent-loop`)
5. Deploy to production with monitoring
6. Iterate on agent instructions based on performance

## Resources

- Agent SDK Go: https://github.com/anthropics/agent-sdk-go
- MCP Protocol: https://modelcontextprotocol.io
- GPU Proxy API: `/docs/API.md`
- aiserve.farm Platform: https://aiserve.farm

## Support

For questions or issues:
- GitHub: https://github.com/aiserve/gpuproxy
- Discord: https://discord.gg/aiserve
- Email: support@aiserve.farm
