package acp

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

// ACPServer implements the Agent Communications Protocol
type ACPServer struct {
	gpuService     *gpu.Service
	billingService *billing.Service
	authService    *auth.Service
	guardRails     *middleware.GuardRails
}

// ACPMessage represents a standardized agent communication message
type ACPMessage struct {
	Header  ACPHeader   `json:"header"`
	Payload interface{} `json:"payload"`
}

// ACPHeader contains message metadata
type ACPHeader struct {
	Version       string            `json:"version"`
	MessageID     string            `json:"message_id"`
	ConversationID string           `json:"conversation_id,omitempty"`
	Sender        string            `json:"sender"`
	Recipient     string            `json:"recipient"`
	MessageType   string            `json:"message_type"`
	Timestamp     time.Time         `json:"timestamp"`
	Priority      string            `json:"priority,omitempty"`
	TTL           int               `json:"ttl,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ACPResponse represents an ACP response
type ACPResponse struct {
	Header  ACPHeader   `json:"header"`
	Payload interface{} `json:"payload"`
	Status  ACPStatus   `json:"status"`
}

// ACPStatus represents the status of an operation
type ACPStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// Message types
const (
	MessageTypeRequest      = "request"
	MessageTypeResponse     = "response"
	MessageTypeNotification = "notification"
	MessageTypeQuery        = "query"
	MessageTypeCommand      = "command"
	MessageTypeEvent        = "event"
)

// Priority levels
const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

func NewACPServer(gpuSvc *gpu.Service, billingSvc *billing.Service, authSvc *auth.Service, gr *middleware.GuardRails) *ACPServer {
	return &ACPServer{
		gpuService:     gpuSvc,
		billingService: billingSvc,
		authService:    authSvc,
		guardRails:     gr,
	}
}

// HandleMessage processes an ACP message
func (s *ACPServer) HandleMessage(ctx context.Context, msgBody []byte) ([]byte, error) {
	var msg ACPMessage
	if err := json.Unmarshal(msgBody, &msg); err != nil {
		return s.errorResponse("", "", "PARSE_ERROR", "Invalid message format", 400)
	}

	// Validate header
	if err := s.validateHeader(&msg.Header); err != nil {
		return s.errorResponse(msg.Header.MessageID, msg.Header.Sender, "INVALID_HEADER", err.Error(), 400)
	}

	// Check authentication
	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return s.errorResponse(msg.Header.MessageID, msg.Header.Sender, "UNAUTHORIZED", "Authentication required", 401)
	}

	// Route based on message type
	var response interface{}
	var err error

	switch msg.Header.MessageType {
	case MessageTypeRequest, MessageTypeCommand:
		response, err = s.handleCommand(ctx, msg)
	case MessageTypeQuery:
		response, err = s.handleQuery(ctx, msg)
	case MessageTypeNotification:
		response, err = s.handleNotification(ctx, msg)
	case MessageTypeEvent:
		response, err = s.handleEvent(ctx, msg)
	default:
		return s.errorResponse(msg.Header.MessageID, msg.Header.Sender, "UNKNOWN_TYPE", fmt.Sprintf("Unknown message type: %s", msg.Header.MessageType), 400)
	}

	if err != nil {
		return s.errorResponse(msg.Header.MessageID, msg.Header.Sender, "PROCESSING_ERROR", err.Error(), 500)
	}

	return s.successResponse(msg.Header, response, 200)
}

func (s *ACPServer) validateHeader(header *ACPHeader) error {
	if header.Version == "" {
		header.Version = "1.0"
	}
	if header.MessageID == "" {
		header.MessageID = uuid.New().String()
	}
	if header.Timestamp.IsZero() {
		header.Timestamp = time.Now()
	}
	if header.Sender == "" {
		return fmt.Errorf("sender is required")
	}
	if header.MessageType == "" {
		return fmt.Errorf("message_type is required")
	}
	return nil
}

func (s *ACPServer) handleCommand(ctx context.Context, msg ACPMessage) (interface{}, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid payload format")
	}

	command, _ := payload["command"].(string)
	parameters, _ := payload["parameters"].(map[string]interface{})

	switch command {
	case "gpu.list":
		return s.commandGPUList(ctx, parameters)
	case "gpu.create":
		return s.commandGPUCreate(ctx, parameters)
	case "gpu.destroy":
		return s.commandGPUDestroy(ctx, parameters)
	case "billing.query":
		return s.commandBillingQuery(ctx, parameters)
	case "guardrails.check":
		return s.commandGuardRailsCheck(ctx, parameters)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

func (s *ACPServer) handleQuery(ctx context.Context, msg ACPMessage) (interface{}, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid payload format")
	}

	query, _ := payload["query"].(string)
	parameters, _ := payload["parameters"].(map[string]interface{})

	switch query {
	case "gpu.availability":
		return s.commandGPUList(ctx, parameters)
	case "spending.current":
		return s.querySpendingCurrent(ctx)
	case "capabilities":
		return s.queryCapabilities(ctx)
	default:
		return nil, fmt.Errorf("unknown query: %s", query)
	}
}

func (s *ACPServer) handleNotification(ctx context.Context, msg ACPMessage) (interface{}, error) {
	return map[string]interface{}{
		"acknowledged": true,
		"message_id":   msg.Header.MessageID,
	}, nil
}

func (s *ACPServer) handleEvent(ctx context.Context, msg ACPMessage) (interface{}, error) {
	return map[string]interface{}{
		"processed": true,
		"message_id": msg.Header.MessageID,
	}, nil
}

// Command implementations

func (s *ACPServer) commandGPUList(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	provider, _ := params["provider"].(string)
	if provider == "" {
		provider = "all"
	}

	instances, err := s.gpuService.ListInstances(ctx, gpu.Provider(provider))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
	}, nil
}

func (s *ACPServer) commandGPUCreate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	provider, _ := params["provider"].(string)
	instanceID, _ := params["instance_id"].(string)
	image, _ := params["image"].(string)

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

func (s *ACPServer) commandGPUDestroy(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	provider, _ := params["provider"].(string)
	instanceID, _ := params["instance_id"].(string)

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

func (s *ACPServer) commandBillingQuery(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *ACPServer) commandGuardRailsCheck(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	userID := middleware.GetUserID(ctx)
	estimatedCost, _ := params["estimated_cost"].(float64)

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

func (s *ACPServer) querySpendingCurrent(ctx context.Context) (interface{}, error) {
	userID := middleware.GetUserID(ctx)
	return s.guardRails.GetSpendingInfo(ctx, userID)
}

func (s *ACPServer) queryCapabilities(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"agent": "aiserve-gpuproxy",
		"capabilities": []string{
			"gpu.list",
			"gpu.create",
			"gpu.destroy",
			"billing.query",
			"guardrails.check",
			"guardrails.spending",
		},
		"protocols": []string{
			"MCP",
			"A2A",
			"ACP",
		},
		"version": "1.0.0",
	}, nil
}

// Response helpers

func (s *ACPServer) successResponse(requestHeader ACPHeader, data interface{}, code int) ([]byte, error) {
	responseHeader := ACPHeader{
		Version:        "1.0",
		MessageID:      uuid.New().String(),
		ConversationID: requestHeader.ConversationID,
		Sender:         "aiserve-gpuproxy",
		Recipient:      requestHeader.Sender,
		MessageType:    MessageTypeResponse,
		Timestamp:      time.Now(),
	}

	resp := ACPResponse{
		Header:  responseHeader,
		Payload: data,
		Status: ACPStatus{
			Code:    code,
			Message: "success",
			Success: true,
		},
	}

	return json.Marshal(resp)
}

func (s *ACPServer) errorResponse(messageID, recipient, errorCode, message string, statusCode int) ([]byte, error) {
	header := ACPHeader{
		Version:     "1.0",
		MessageID:   uuid.New().String(),
		Sender:      "aiserve-gpuproxy",
		Recipient:   recipient,
		MessageType: MessageTypeResponse,
		Timestamp:   time.Now(),
	}

	resp := ACPResponse{
		Header: header,
		Payload: map[string]interface{}{
			"error":      errorCode,
			"details":    message,
			"message_id": messageID,
		},
		Status: ACPStatus{
			Code:    statusCode,
			Message: message,
			Success: false,
		},
	}

	return json.Marshal(resp)
}
