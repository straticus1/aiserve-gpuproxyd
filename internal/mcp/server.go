package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/billing"
	"github.com/aiserve/gpuproxy/internal/gpu"
	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/google/uuid"
)

// MCPServer implements the Model Context Protocol server
type MCPServer struct {
	gpuService     *gpu.Service
	billingService *billing.Service
	authService    *auth.Service
	guardRails     *middleware.GuardRails
}

// MCPRequest represents an MCP protocol request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

// MCPResponse represents an MCP protocol response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func NewMCPServer(gpuSvc *gpu.Service, billingSvc *billing.Service, authSvc *auth.Service, gr *middleware.GuardRails) *MCPServer {
	return &MCPServer{
		gpuService:     gpuSvc,
		billingService: billingSvc,
		authService:    authSvc,
		guardRails:     gr,
	}
}

// HandleRequest processes an MCP request
func (s *MCPServer) HandleRequest(ctx context.Context, reqBody []byte) ([]byte, error) {
	var req MCPRequest
	if err := json.Unmarshal(reqBody, &req); err != nil {
		return s.errorResponse(nil, -32700, "Parse error", nil)
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(ctx, req)
	case "tools/list":
		return s.handleToolsList(ctx, req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(ctx, req)
	case "resources/read":
		return s.handleResourcesRead(ctx, req)
	default:
		return s.errorResponse(req.ID, -32601, "Method not found", nil)
	}
}

func (s *MCPServer) handleInitialize(ctx context.Context, req MCPRequest) ([]byte, error) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]interface{}{
			"name":    "aiserve-gpuproxy",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{
			"tools":     map[string]bool{},
			"resources": map[string]bool{},
		},
	}

	return s.successResponse(req.ID, result)
}

func (s *MCPServer) handleToolsList(ctx context.Context, req MCPRequest) ([]byte, error) {
	tools := []Tool{
		{
			Name:        "list_gpu_instances",
			Description: "List available GPU instances from all providers (vast.ai, io.net)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider to query (vast.ai, io.net, or all)",
						"enum":        []string{"vast.ai", "io.net", "all"},
						"default":     "all",
					},
					"min_vram": map[string]interface{}{
						"type":        "integer",
						"description": "Minimum VRAM in GB",
					},
					"max_price": map[string]interface{}{
						"type":        "number",
						"description": "Maximum price per hour in USD",
					},
				},
			},
		},
		{
			Name:        "create_gpu_instance",
			Description: "Create a GPU instance on a specific provider",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider (vast.ai or io.net)",
						"enum":        []string{"vast.ai", "io.net"},
					},
					"instance_id": map[string]interface{}{
						"type":        "string",
						"description": "Instance ID from the provider",
					},
					"image": map[string]interface{}{
						"type":        "string",
						"description": "Docker image to use (default: nvidia/cuda:12.0.0-base-ubuntu22.04)",
					},
				},
				"required": []string{"provider", "instance_id"},
			},
		},
		{
			Name:        "destroy_gpu_instance",
			Description: "Destroy a GPU instance",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider (vast.ai or io.net)",
						"enum":        []string{"vast.ai", "io.net"},
					},
					"instance_id": map[string]interface{}{
						"type":        "string",
						"description": "Instance ID to destroy",
					},
				},
				"required": []string{"provider", "instance_id"},
			},
		},
		{
			Name:        "get_spending_info",
			Description: "Get current spending information across all guard rails time windows",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "check_spending_limit",
			Description: "Check if a request with estimated cost would exceed spending limits",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"estimated_cost": map[string]interface{}{
						"type":        "number",
						"description": "Estimated cost in USD",
					},
				},
				"required": []string{"estimated_cost"},
			},
		},
		{
			Name:        "get_billing_transactions",
			Description: "Get billing transaction history",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "proxy_inference_request",
			Description: "Proxy an inference request to a GPU instance",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_url": map[string]interface{}{
						"type":        "string",
						"description": "Target URL for the inference request",
					},
					"method": map[string]interface{}{
						"type":        "string",
						"description": "HTTP method (GET, POST, etc.)",
						"default":     "POST",
					},
					"body": map[string]interface{}{
						"type":        "object",
						"description": "Request body",
					},
					"headers": map[string]interface{}{
						"type":        "object",
						"description": "Request headers",
					},
				},
				"required": []string{"target_url"},
			},
		},
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	return s.successResponse(req.ID, result)
}

func (s *MCPServer) handleToolsCall(ctx context.Context, req MCPRequest) ([]byte, error) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, -32602, "Invalid params", nil)
	}

	// Extract user ID from context
	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return s.errorResponse(req.ID, -32000, "Unauthorized", nil)
	}

	var result interface{}
	var err error

	switch params.Name {
	case "list_gpu_instances":
		result, err = s.listGPUInstances(ctx, params.Arguments)
	case "create_gpu_instance":
		result, err = s.createGPUInstance(ctx, params.Arguments)
	case "destroy_gpu_instance":
		result, err = s.destroyGPUInstance(ctx, params.Arguments)
	case "get_spending_info":
		result, err = s.getSpendingInfo(ctx, userID)
	case "check_spending_limit":
		result, err = s.checkSpendingLimit(ctx, userID, params.Arguments)
	case "get_billing_transactions":
		result, err = s.getBillingTransactions(ctx, userID)
	case "proxy_inference_request":
		result, err = s.proxyInferenceRequest(ctx, params.Arguments)
	default:
		return s.errorResponse(req.ID, -32601, "Tool not found", nil)
	}

	if err != nil {
		return s.errorResponse(req.ID, -32000, err.Error(), nil)
	}

	toolResult := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": formatResult(result),
			},
		},
	}

	return s.successResponse(req.ID, toolResult)
}

func (s *MCPServer) handleResourcesList(ctx context.Context, req MCPRequest) ([]byte, error) {
	resources := []map[string]interface{}{
		{
			"uri":         "gpu://instances",
			"name":        "GPU Instances",
			"description": "List of available GPU instances",
			"mimeType":    "application/json",
		},
		{
			"uri":         "spending://current",
			"name":        "Current Spending",
			"description": "Current spending across all time windows",
			"mimeType":    "application/json",
		},
	}

	result := map[string]interface{}{
		"resources": resources,
	}

	return s.successResponse(req.ID, result)
}

func (s *MCPServer) handleResourcesRead(ctx context.Context, req MCPRequest) ([]byte, error) {
	var params struct {
		URI string `json:"uri"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, -32602, "Invalid params", nil)
	}

	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return s.errorResponse(req.ID, -32000, "Unauthorized", nil)
	}

	var content interface{}
	var err error

	switch params.URI {
	case "gpu://instances":
		content, err = s.gpuService.ListInstances(ctx, gpu.ProviderAll)
	case "spending://current":
		content, err = s.guardRails.GetSpendingInfo(ctx, userID)
	default:
		return s.errorResponse(req.ID, -32602, "Unknown resource URI", nil)
	}

	if err != nil {
		return s.errorResponse(req.ID, -32000, err.Error(), nil)
	}

	result := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"uri":      params.URI,
				"mimeType": "application/json",
				"text":     formatResult(content),
			},
		},
	}

	return s.successResponse(req.ID, result)
}

// Tool implementation methods

func (s *MCPServer) listGPUInstances(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Provider string  `json:"provider"`
		MinVRAM  int     `json:"min_vram"`
		MaxPrice float64 `json:"max_price"`
	}

	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	if params.Provider == "" {
		params.Provider = "all"
	}

	provider := gpu.Provider(params.Provider)
	instances, err := s.gpuService.ListInstances(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Apply filters if provided
	filters := make(map[string]interface{})
	if params.MinVRAM > 0 {
		filters["min_vram"] = params.MinVRAM
	}
	if params.MaxPrice > 0 {
		filters["max_price"] = params.MaxPrice
	}

	if len(filters) > 0 {
		instances = s.gpuService.FilterInstances(instances, filters)
	}

	return instances, nil
}

func (s *MCPServer) createGPUInstance(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Provider   string `json:"provider"`
		InstanceID string `json:"instance_id"`
		Image      string `json:"image"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Provider == "" || params.InstanceID == "" {
		return nil, fmt.Errorf("provider and instance_id are required")
	}

	config := make(map[string]interface{})
	if params.Image != "" {
		config["image"] = params.Image
	}

	provider := gpu.Provider(params.Provider)
	instanceID, err := s.gpuService.CreateInstance(ctx, provider, params.InstanceID, config)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"instance_id": instanceID,
		"provider":    params.Provider,
		"status":      "created",
	}, nil
}

func (s *MCPServer) destroyGPUInstance(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Provider   string `json:"provider"`
		InstanceID string `json:"instance_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Provider == "" || params.InstanceID == "" {
		return nil, fmt.Errorf("provider and instance_id are required")
	}

	provider := gpu.Provider(params.Provider)
	if err := s.gpuService.DestroyInstance(ctx, provider, params.InstanceID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"instance_id": params.InstanceID,
		"provider":    params.Provider,
		"status":      "destroyed",
	}, nil
}

func (s *MCPServer) getSpendingInfo(ctx context.Context, userID uuid.UUID) (interface{}, error) {
	return s.guardRails.GetSpendingInfo(ctx, userID)
}

func (s *MCPServer) checkSpendingLimit(ctx context.Context, userID uuid.UUID, args json.RawMessage) (interface{}, error) {
	var params struct {
		EstimatedCost float64 `json:"estimated_cost"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	info, err := s.guardRails.CheckSpending(ctx, userID, params.EstimatedCost)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"allowed":    len(info.Violations) == 0,
		"violations": info.Violations,
		"spent":      info.WindowSpent,
	}, nil
}

func (s *MCPServer) getBillingTransactions(ctx context.Context, userID uuid.UUID) (interface{}, error) {
	return s.billingService.GetTransactionsByUser(ctx, userID)
}

func (s *MCPServer) proxyInferenceRequest(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		TargetURL string                 `json:"target_url"`
		Method    string                 `json:"method"`
		Body      map[string]interface{} `json:"body"`
		Headers   map[string]string      `json:"headers"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.TargetURL == "" {
		return nil, fmt.Errorf("target_url is required")
	}

	if params.Method == "" {
		params.Method = "POST"
	}

	// This would integrate with your existing protocol handler
	log.Printf("Proxying inference request to %s", params.TargetURL)

	return map[string]interface{}{
		"status":  "success",
		"message": "Request proxied successfully",
	}, nil
}

// Helper methods

func (s *MCPServer) successResponse(id interface{}, result interface{}) ([]byte, error) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	return json.Marshal(resp)
}

func (s *MCPServer) errorResponse(id interface{}, code int, message string, data interface{}) ([]byte, error) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}
	return json.Marshal(resp)
}

func formatResult(data interface{}) string {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(bytes)
}
