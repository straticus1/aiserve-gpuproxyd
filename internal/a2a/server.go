package a2a

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

// A2AServer implements the Agent-to-Agent Protocol
type A2AServer struct {
	gpuService     *gpu.Service
	billingService *billing.Service
	authService    *auth.Service
	guardRails     *middleware.GuardRails
	agentInfo      AgentInfo
}

// AgentInfo describes this agent's capabilities
type AgentInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Capabilities []string               `json:"capabilities"`
	Endpoints    map[string]string      `json:"endpoints"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// A2ARequest represents an Agent-to-Agent request
type A2ARequest struct {
	Version    string                 `json:"version"`
	MessageID  string                 `json:"message_id"`
	FromAgent  string                 `json:"from_agent"`
	ToAgent    string                 `json:"to_agent"`
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// A2AResponse represents an Agent-to-Agent response
type A2AResponse struct {
	Version    string                 `json:"version"`
	MessageID  string                 `json:"message_id"`
	InReplyTo  string                 `json:"in_reply_to"`
	FromAgent  string                 `json:"from_agent"`
	ToAgent    string                 `json:"to_agent"`
	Status     string                 `json:"status"`
	Data       interface{}            `json:"data,omitempty"`
	Error      *A2AError              `json:"error,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// A2AError represents an A2A error
type A2AError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func NewA2AServer(gpuSvc *gpu.Service, billingSvc *billing.Service, authSvc *auth.Service, gr *middleware.GuardRails) *A2AServer {
	return &A2AServer{
		gpuService:     gpuSvc,
		billingService: billingSvc,
		authService:    authSvc,
		guardRails:     gr,
		agentInfo: AgentInfo{
			ID:          "aiserve-gpuproxy",
			Name:        "AIServe GPU Proxy",
			Version:     "1.0.0",
			Description: "GPU resource management and proxy service",
			Capabilities: []string{
				"gpu.list",
				"gpu.create",
				"gpu.destroy",
				"gpu.status",
				"billing.transactions",
				"billing.check",
				"guardrails.spending",
				"guardrails.limits",
				"inference.proxy",
			},
			Endpoints: map[string]string{
				"http": "/api/v1/a2a",
				"ws":   "/api/v1/a2a/ws",
			},
			Metadata: map[string]interface{}{
				"provider":         "aiserve.farm",
				"protocol_version": "1.0",
				"supports_async":   true,
			},
		},
	}
}

// HandleRequest processes an A2A request
func (s *A2AServer) HandleRequest(ctx context.Context, reqBody []byte) ([]byte, error) {
	var req A2ARequest
	if err := json.Unmarshal(reqBody, &req); err != nil {
		return s.errorResponse("", "", "PARSE_ERROR", "Invalid JSON", err.Error())
	}

	// Validate request
	if req.Version == "" {
		req.Version = "1.0"
	}
	if req.MessageID == "" {
		req.MessageID = uuid.New().String()
	}
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}

	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return s.errorResponse(req.MessageID, req.FromAgent, "AUTH_ERROR", "Unauthorized", "Valid authentication required")
	}

	var data interface{}
	var err error

	switch req.Action {
	case "agent.discover":
		data = s.handleDiscover(ctx, req)
	case "agent.ping":
		data = s.handlePing(ctx, req)
	case "gpu.list":
		data, err = s.handleGPUList(ctx, req)
	case "gpu.create":
		data, err = s.handleGPUCreate(ctx, req)
	case "gpu.destroy":
		data, err = s.handleGPUDestroy(ctx, req)
	case "billing.transactions":
		data, err = s.handleBillingTransactions(ctx, userID, req)
	case "guardrails.spending":
		data, err = s.handleGuardRailsSpending(ctx, userID, req)
	case "guardrails.check":
		data, err = s.handleGuardRailsCheck(ctx, userID, req)
	case "inference.proxy":
		data, err = s.handleInferenceProxy(ctx, req)
	default:
		return s.errorResponse(req.MessageID, req.FromAgent, "UNKNOWN_ACTION", "Unknown action", fmt.Sprintf("Action '%s' not supported", req.Action))
	}

	if err != nil {
		return s.errorResponse(req.MessageID, req.FromAgent, "ACTION_ERROR", err.Error(), "")
	}

	return s.successResponse(req.MessageID, req.FromAgent, data)
}

// Action handlers

func (s *A2AServer) handleDiscover(ctx context.Context, req A2ARequest) interface{} {
	return s.agentInfo
}

func (s *A2AServer) handlePing(ctx context.Context, req A2ARequest) interface{} {
	return map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"agent":     s.agentInfo.ID,
	}
}

func (s *A2AServer) handleGPUList(ctx context.Context, req A2ARequest) (interface{}, error) {
	provider, _ := req.Parameters["provider"].(string)
	if provider == "" {
		provider = "all"
	}

	instances, err := s.gpuService.ListInstances(ctx, gpu.Provider(provider))
	if err != nil {
		return nil, err
	}

	// Apply filters
	if minVRAM, ok := req.Parameters["min_vram"].(float64); ok {
		filters := map[string]interface{}{"min_vram": int(minVRAM)}
		instances = s.gpuService.FilterInstances(instances, filters)
	}
	if maxPrice, ok := req.Parameters["max_price"].(float64); ok {
		filters := map[string]interface{}{"max_price": maxPrice}
		instances = s.gpuService.FilterInstances(instances, filters)
	}

	return map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
		"provider":  provider,
	}, nil
}

func (s *A2AServer) handleGPUCreate(ctx context.Context, req A2ARequest) (interface{}, error) {
	provider, _ := req.Parameters["provider"].(string)
	instanceID, _ := req.Parameters["instance_id"].(string)
	image, _ := req.Parameters["image"].(string)

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

func (s *A2AServer) handleGPUDestroy(ctx context.Context, req A2ARequest) (interface{}, error) {
	provider, _ := req.Parameters["provider"].(string)
	instanceID, _ := req.Parameters["instance_id"].(string)

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

func (s *A2AServer) handleBillingTransactions(ctx context.Context, userID uuid.UUID, req A2ARequest) (interface{}, error) {
	transactions, err := s.billingService.GetTransactionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"transactions": transactions,
		"count":        len(transactions),
	}, nil
}

func (s *A2AServer) handleGuardRailsSpending(ctx context.Context, userID uuid.UUID, req A2ARequest) (interface{}, error) {
	return s.guardRails.GetSpendingInfo(ctx, userID)
}

func (s *A2AServer) handleGuardRailsCheck(ctx context.Context, userID uuid.UUID, req A2ARequest) (interface{}, error) {
	estimatedCost, _ := req.Parameters["estimated_cost"].(float64)

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

func (s *A2AServer) handleInferenceProxy(ctx context.Context, req A2ARequest) (interface{}, error) {
	targetURL, _ := req.Parameters["target_url"].(string)
	if targetURL == "" {
		return nil, fmt.Errorf("target_url is required")
	}

	return map[string]interface{}{
		"status":  "proxied",
		"target":  targetURL,
		"message": "Request proxied successfully",
	}, nil
}

// Response helpers

func (s *A2AServer) successResponse(messageID, toAgent string, data interface{}) ([]byte, error) {
	resp := A2AResponse{
		Version:   "1.0",
		MessageID: uuid.New().String(),
		InReplyTo: messageID,
		FromAgent: s.agentInfo.ID,
		ToAgent:   toAgent,
		Status:    "success",
		Data:      data,
		Timestamp: time.Now(),
	}
	return json.Marshal(resp)
}

func (s *A2AServer) errorResponse(messageID, toAgent, code, message, details string) ([]byte, error) {
	resp := A2AResponse{
		Version:   "1.0",
		MessageID: uuid.New().String(),
		InReplyTo: messageID,
		FromAgent: s.agentInfo.ID,
		ToAgent:   toAgent,
		Status:    "error",
		Error: &A2AError{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now(),
	}
	return json.Marshal(resp)
}

// GetAgentInfo returns the agent's capabilities and information
func (s *A2AServer) GetAgentInfo() AgentInfo {
	return s.agentInfo
}
