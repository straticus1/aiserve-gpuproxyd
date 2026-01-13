package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// MCP Server for GPU Proxy - exposes GPU operations as MCP tools
// Compatible with n8n MCP integration and agent-sdk-go

type MCPServer struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   *MCPError              `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewMCPServer() *MCPServer {
	baseURL := os.Getenv("GPU_PROXY_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	apiKey := os.Getenv("GPU_PROXY_API_KEY")

	return &MCPServer{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *MCPServer) callAPI(method, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, s.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	if s.apiKey != "" {
		req.Header.Set("X-API-Key", s.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("API error: %d - %v", resp.StatusCode, result)
	}

	return result, nil
}

func (s *MCPServer) handleRequest(ctx context.Context, req MCPRequest) MCPResponse {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "tools/list":
		resp.Result = s.listTools()

	case "tools/call":
		toolName := req.Params["name"].(string)
		args := req.Params["arguments"].(map[string]interface{})
		result, err := s.callTool(ctx, toolName, args)
		if err != nil {
			resp.Error = &MCPError{
				Code:    -32603,
				Message: err.Error(),
			}
		} else {
			resp.Result = result
		}

	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "gpuproxy-mcp",
				"version": "1.0.0",
			},
		}

	default:
		resp.Error = &MCPError{
			Code:    -32601,
			Message: "Method not found",
		}
	}

	return resp
}

func (s *MCPServer) listTools() map[string]interface{} {
	tools := []map[string]interface{}{
		{
			"name":        "provision_gpu",
			"description": "Provision a GPU instance based on preferences (H100, A100, etc.)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"gpu_model": map[string]interface{}{
						"type":        "string",
						"description": "GPU model (H100, H200, A100, V100, RTX 4090, etc.)",
					},
					"provider": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"vastai", "ionet"},
						"description": "GPU provider (default: vastai)",
					},
					"min_vram": map[string]interface{}{
						"type":        "number",
						"description": "Minimum VRAM in GB",
					},
					"max_price": map[string]interface{}{
						"type":        "number",
						"description": "Maximum price per hour in USD",
					},
				},
				"required": []string{"gpu_model"},
			},
		},
		{
			"name":        "list_gpu_instances",
			"description": "List all active GPU instances with status and usage",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "destroy_gpu_instance",
			"description": "Destroy a specific GPU instance to stop billing",
			"inputSchema": map[string]interface{}{
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
		},
		{
			"name":        "get_gpu_stats",
			"description": "Get comprehensive GPU usage statistics, costs, and performance metrics",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "get_health_status",
			"description": "Check health status of GPU Proxy and connected services (database, Redis, etc.)",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "get_available_gpus",
			"description": "Get list of available GPU models with specs (VRAM, price, compute capability)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"vendor": map[string]interface{}{
						"type": "string",
						"enum": []string{"NVIDIA", "AMD", "Intel", "Apple"},
					},
					"tier": map[string]interface{}{
						"type": "string",
						"enum": []string{"enterprise", "high_end", "mid_range", "budget"},
					},
					"min_vram": map[string]interface{}{
						"type": "number",
					},
					"max_price": map[string]interface{}{
						"type": "number",
					},
				},
			},
		},
		{
			"name":        "cleanup_idle_gpus",
			"description": "Find and destroy GPU instances idle for specified duration (cost optimization)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"idle_threshold_minutes": map[string]interface{}{
						"type":        "number",
						"description": "Minutes of idle time before cleanup (default: 30)",
						"default":     30,
					},
				},
			},
		},
		{
			"name":        "get_metrics",
			"description": "Get real-time Prometheus metrics (requests, GPU usage, database, cache, system)",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "batch_provision_gpus",
			"description": "Provision multiple GPU instances at once for high-throughput workloads",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"count": map[string]interface{}{
						"type":        "number",
						"description": "Number of instances to provision",
					},
					"gpu_model": map[string]interface{}{
						"type": "string",
					},
					"provider": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"count", "gpu_model"},
			},
		},
	}

	return map[string]interface{}{
		"tools": tools,
	}
}

func (s *MCPServer) callTool(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error) {
	switch name {
	case "provision_gpu":
		return s.provisionGPU(args)

	case "list_gpu_instances":
		return s.callAPI("GET", "/api/v1/gpu/instances", nil)

	case "destroy_gpu_instance":
		provider := args["provider"].(string)
		instanceID := args["instance_id"].(string)
		path := fmt.Sprintf("/api/v1/gpu/instances/%s/%s", provider, instanceID)
		return s.callAPI("DELETE", path, nil)

	case "get_gpu_stats":
		return s.callAPI("GET", "/stats", nil)

	case "get_health_status":
		return s.callAPI("GET", "/health", nil)

	case "get_available_gpus":
		query := ""
		if vendor, ok := args["vendor"].(string); ok {
			query += "?vendor=" + vendor
		}
		if tier, ok := args["tier"].(string); ok {
			if query == "" {
				query = "?"
			} else {
				query += "&"
			}
			query += "tier=" + tier
		}
		return s.callAPI("GET", "/api/v1/gpu/available"+query, nil)

	case "cleanup_idle_gpus":
		return s.cleanupIdleGPUs(args)

	case "get_metrics":
		return s.callAPI("GET", "/metrics", nil)

	case "batch_provision_gpus":
		return s.batchProvisionGPUs(args)

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *MCPServer) provisionGPU(args map[string]interface{}) (map[string]interface{}, error) {
	gpuModel := args["gpu_model"].(string)
	provider := "vastai"
	if p, ok := args["provider"].(string); ok {
		provider = p
	}

	body := map[string]interface{}{
		"preferred_gpus": []map[string]interface{}{
			{
				"model":    gpuModel,
				"priority": 1,
			},
		},
	}

	if minVRAM, ok := args["min_vram"].(float64); ok {
		body["constraints"] = map[string]interface{}{
			"min_vram": int(minVRAM),
		}
	}

	if maxPrice, ok := args["max_price"].(float64); ok {
		if constraints, ok := body["constraints"].(map[string]interface{}); ok {
			constraints["max_price_per_hour"] = maxPrice
		} else {
			body["constraints"] = map[string]interface{}{
				"max_price_per_hour": maxPrice,
			}
		}
	}

	return s.callAPI("POST", "/api/v1/gpu/instances/reserve", body)
}

func (s *MCPServer) cleanupIdleGPUs(args map[string]interface{}) (map[string]interface{}, error) {
	thresholdMinutes := 30.0
	if t, ok := args["idle_threshold_minutes"].(float64); ok {
		thresholdMinutes = t
	}

	instances, err := s.callAPI("GET", "/api/v1/gpu/instances", nil)
	if err != nil {
		return nil, err
	}

	cleanedUp := []map[string]interface{}{}
	now := time.Now()

	if instancesList, ok := instances["instances"].([]interface{}); ok {
		for _, inst := range instancesList {
			instance := inst.(map[string]interface{})
			lastUsedStr, ok := instance["last_used_at"].(string)
			if !ok {
				continue
			}

			lastUsed, err := time.Parse(time.RFC3339, lastUsedStr)
			if err != nil {
				continue
			}

			idleMinutes := now.Sub(lastUsed).Minutes()
			if idleMinutes > thresholdMinutes {
				provider := instance["provider"].(string)
				instanceID := instance["id"].(string)
				path := fmt.Sprintf("/api/v1/gpu/instances/%s/%s", provider, instanceID)

				_, err := s.callAPI("DELETE", path, nil)
				cleanedUp = append(cleanedUp, map[string]interface{}{
					"instance_id": instanceID,
					"provider":    provider,
					"idle_minutes": idleMinutes,
					"success":     err == nil,
				})
			}
		}
	}

	return map[string]interface{}{
		"cleaned_up_count": len(cleanedUp),
		"instances":        cleanedUp,
		"threshold_minutes": thresholdMinutes,
	}, nil
}

func (s *MCPServer) batchProvisionGPUs(args map[string]interface{}) (map[string]interface{}, error) {
	count := int(args["count"].(float64))
	gpuModel := args["gpu_model"].(string)
	provider := "vastai"
	if p, ok := args["provider"].(string); ok {
		provider = p
	}

	body := map[string]interface{}{
		"count":    count,
		"gpu_type": gpuModel,
		"provider": provider,
	}

	return s.callAPI("POST", "/api/v1/gpu/instances/batch", body)
}

func main() {
	server := NewMCPServer()

	log.Println("GPU Proxy MCP Server starting...")
	log.Printf("Connected to: %s", server.baseURL)

	// Use stdio for MCP protocol
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error decoding request: %v", err)
			continue
		}

		ctx := context.Background()
		resp := server.handleRequest(ctx, req)

		if err := encoder.Encode(resp); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
	}
}
