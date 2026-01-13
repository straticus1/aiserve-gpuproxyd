package langchain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/billing"
	"github.com/aiserve/gpuproxy/internal/gpu"
	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/google/uuid"
)

// LangChainServer implements the LangChain Agent Protocol
type LangChainServer struct {
	gpuService     *gpu.Service
	billingService *billing.Service
	authService    *auth.Service
	guardRails     *middleware.GuardRails
}

// AgentAction represents a LangChain agent action
type AgentAction struct {
	Tool      string                 `json:"tool"`
	ToolInput map[string]interface{} `json:"tool_input"`
	Log       string                 `json:"log,omitempty"`
}

// AgentFinish represents a completed agent execution
type AgentFinish struct {
	ReturnValues map[string]interface{} `json:"return_values"`
	Log          string                 `json:"log,omitempty"`
}

// AgentStep represents a step in agent execution
type AgentStep struct {
	Action      *AgentAction `json:"action,omitempty"`
	Observation string       `json:"observation,omitempty"`
}

// Tool represents a LangChain tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolMessage represents tool execution result
type ToolMessage struct {
	Tool    string      `json:"tool"`
	Content interface{} `json:"content"`
	Error   string      `json:"error,omitempty"`
}

// ChainRequest represents a LangChain request
type ChainRequest struct {
	Input       map[string]interface{} `json:"input"`
	ChatHistory []Message              `json:"chat_history,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChainResponse represents a LangChain response
type ChainResponse struct {
	Output      map[string]interface{} `json:"output"`
	Steps       []AgentStep            `json:"steps,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func NewLangChainServer(gpuSvc *gpu.Service, billingSvc *billing.Service, authSvc *auth.Service, gr *middleware.GuardRails) *LangChainServer {
	return &LangChainServer{
		gpuService:     gpuSvc,
		billingService: billingSvc,
		authService:    authSvc,
		guardRails:     gr,
	}
}

// GetTools returns available tools for LangChain
func (s *LangChainServer) GetTools() []Tool {
	return []Tool{
		{
			Name:        "list_gpu_instances",
			Description: "List available GPU instances from providers (vast.ai, io.net). Returns a list of GPU instances with specifications and pricing.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider to query: 'vast.ai', 'io.net', or 'all'",
						"enum":        []string{"vast.ai", "io.net", "all"},
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
			},
		},
		{
			Name:        "create_gpu_instance",
			Description: "Create a new GPU instance on the specified provider. Returns the created instance ID and status.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider: 'vast.ai' or 'io.net'",
						"enum":        []string{"vast.ai", "io.net"},
					},
					"instance_id": map[string]interface{}{
						"type":        "string",
						"description": "Instance ID from the provider",
					},
					"image": map[string]interface{}{
						"type":        "string",
						"description": "Docker image to use (optional)",
					},
				},
				"required": []string{"provider", "instance_id"},
			},
		},
		{
			Name:        "destroy_gpu_instance",
			Description: "Destroy an existing GPU instance. Returns confirmation of destruction.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider: 'vast.ai' or 'io.net'",
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
			Description: "Get current spending information across all guard rails time windows. Returns spending by time window and any violations.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "check_spending_limit",
			Description: "Check if a request with estimated cost would exceed spending limits. Returns whether the request is allowed and current spending.",
			Parameters: map[string]interface{}{
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
			Description: "Get billing transaction history for the authenticated user. Returns list of transactions.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "record_spending",
			Description: "Record spending amount for guard rails tracking. Returns confirmation.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"amount": map[string]interface{}{
						"type":        "number",
						"description": "Amount in USD to record",
					},
				},
				"required": []string{"amount"},
			},
		},
	}
}

// ExecuteTool executes a tool with the given inputs
func (s *LangChainServer) ExecuteTool(ctx context.Context, tool string, inputs map[string]interface{}) (*ToolMessage, error) {
	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return &ToolMessage{
			Tool:  tool,
			Error: "Authentication required",
		}, nil
	}

	var content interface{}
	var err error

	switch tool {
	case "list_gpu_instances":
		content, err = s.toolListGPUInstances(ctx, inputs)
	case "create_gpu_instance":
		content, err = s.toolCreateGPUInstance(ctx, inputs)
	case "destroy_gpu_instance":
		content, err = s.toolDestroyGPUInstance(ctx, inputs)
	case "get_spending_info":
		content, err = s.toolGetSpendingInfo(ctx, inputs)
	case "check_spending_limit":
		content, err = s.toolCheckSpendingLimit(ctx, inputs)
	case "get_billing_transactions":
		content, err = s.toolGetBillingTransactions(ctx, inputs)
	case "record_spending":
		content, err = s.toolRecordSpending(ctx, inputs)
	default:
		return &ToolMessage{
			Tool:  tool,
			Error: fmt.Sprintf("Unknown tool: %s", tool),
		}, nil
	}

	if err != nil {
		return &ToolMessage{
			Tool:  tool,
			Error: err.Error(),
		}, nil
	}

	return &ToolMessage{
		Tool:    tool,
		Content: content,
	}, nil
}

// HandleChain processes a LangChain chain request
func (s *LangChainServer) HandleChain(ctx context.Context, reqBody []byte) ([]byte, error) {
	var req ChainRequest
	if err := json.Unmarshal(reqBody, &req); err != nil {
		return nil, fmt.Errorf("invalid request format: %w", err)
	}

	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Execute the requested action
	action, _ := req.Input["action"].(string)
	if action == "" {
		action = "execute"
	}

	var output map[string]interface{}
	var steps []AgentStep
	var err error

	switch action {
	case "get_tools":
		tools := s.GetTools()
		output = map[string]interface{}{
			"tools": tools,
		}
	case "execute":
		tool, _ := req.Input["tool"].(string)
		toolInput, _ := req.Input["tool_input"].(map[string]interface{})

		result, execErr := s.ExecuteTool(ctx, tool, toolInput)
		if execErr != nil {
			err = execErr
		}

		output = map[string]interface{}{
			"result": result,
		}

		steps = append(steps, AgentStep{
			Action: &AgentAction{
				Tool:      tool,
				ToolInput: toolInput,
				Log:       fmt.Sprintf("Executing %s", tool),
			},
			Observation: fmt.Sprintf("%v", result.Content),
		})
	default:
		err = fmt.Errorf("unknown action: %s", action)
	}

	if err != nil {
		return nil, err
	}

	response := ChainResponse{
		Output: output,
		Steps:  steps,
		Metadata: map[string]interface{}{
			"timestamp": time.Now(),
			"user_id":   userID.String(),
		},
	}

	return json.Marshal(response)
}

// Tool implementations

func (s *LangChainServer) toolListGPUInstances(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
	provider, _ := inputs["provider"].(string)
	if provider == "" {
		provider = "all"
	}

	instances, err := s.gpuService.ListInstances(ctx, gpu.Provider(provider))
	if err != nil {
		return nil, err
	}

	// Apply filters
	if minVRAM, ok := inputs["min_vram"].(float64); ok {
		filters := map[string]interface{}{"min_vram": int(minVRAM)}
		instances = s.gpuService.FilterInstances(instances, filters)
	}
	if maxPrice, ok := inputs["max_price"].(float64); ok {
		filters := map[string]interface{}{"max_price": maxPrice}
		instances = s.gpuService.FilterInstances(instances, filters)
	}

	return map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
		"provider":  provider,
	}, nil
}

func (s *LangChainServer) toolCreateGPUInstance(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
	provider, _ := inputs["provider"].(string)
	instanceID, _ := inputs["instance_id"].(string)
	image, _ := inputs["image"].(string)

	if provider == "" || instanceID == "" {
		return nil, fmt.Errorf("provider and instance_id are required")
	}

	config := make(map[string]interface{})
	if image != "" {
		config["image"] = image
	}

	createdID, err := s.gpuService.CreateInstance(ctx, gpu.Provider(provider), instanceID, config)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"instance_id": createdID,
		"provider":    provider,
		"status":      "created",
	}, nil
}

func (s *LangChainServer) toolDestroyGPUInstance(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
	provider, _ := inputs["provider"].(string)
	instanceID, _ := inputs["instance_id"].(string)

	if provider == "" || instanceID == "" {
		return nil, fmt.Errorf("provider and instance_id are required")
	}

	if err := s.gpuService.DestroyInstance(ctx, gpu.Provider(provider), instanceID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"instance_id": instanceID,
		"provider":    provider,
		"status":      "destroyed",
	}, nil
}

func (s *LangChainServer) toolGetSpendingInfo(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
	userID := middleware.GetUserID(ctx)
	return s.guardRails.GetSpendingInfo(ctx, userID)
}

func (s *LangChainServer) toolCheckSpendingLimit(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
	estimatedCost, _ := inputs["estimated_cost"].(float64)
	userID := middleware.GetUserID(ctx)

	info, err := s.guardRails.CheckSpending(ctx, userID, estimatedCost)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"allowed":    len(info.Violations) == 0,
		"violations": info.Violations,
		"spent":      info.WindowSpent,
	}, nil
}

func (s *LangChainServer) toolGetBillingTransactions(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
	userID := middleware.GetUserID(ctx)
	transactions, err := s.billingService.GetTransactionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"transactions": transactions,
		"count":        len(transactions),
	}, nil
}

func (s *LangChainServer) toolRecordSpending(ctx context.Context, inputs map[string]interface{}) (interface{}, error) {
	amount, ok := inputs["amount"].(float64)
	if !ok {
		return nil, fmt.Errorf("amount is required")
	}

	userID := middleware.GetUserID(ctx)
	if err := s.guardRails.RecordSpending(ctx, userID, amount); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"recorded": true,
		"amount":   amount,
	}, nil
}
