package cuic

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

// CUICServer implements CUIC (QUIC-inspired Unified Inter-agent Communication) Protocol
type CUICServer struct {
	gpuService     *gpu.Service
	billingService *billing.Service
	authService    *auth.Service
	guardRails     *middleware.GuardRails
}

// CUICMessage represents a CUIC protocol message
type CUICMessage struct {
	StreamID       string                 `json:"stream_id"`
	MessageID      string                 `json:"message_id"`
	Version        string                 `json:"version"`
	Sender         string                 `json:"sender"`
	Receiver       string                 `json:"receiver"`
	MessageType    string                 `json:"message_type"`
	Priority       int                    `json:"priority"`
	Timestamp      time.Time              `json:"timestamp"`
	Payload        interface{}            `json:"payload"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CongestionHint string                 `json:"congestion_hint,omitempty"` // CUBIC-inspired congestion control hint
}

// CUICResponse represents a CUIC protocol response
type CUICResponse struct {
	StreamID       string                 `json:"stream_id"`
	MessageID      string                 `json:"message_id"`
	InReplyTo      string                 `json:"in_reply_to"`
	Version        string                 `json:"version"`
	Sender         string                 `json:"sender"`
	Receiver       string                 `json:"receiver"`
	Status         CUICStatus             `json:"status"`
	Payload        interface{}            `json:"payload"`
	Timestamp      time.Time              `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CongestionHint string                 `json:"congestion_hint,omitempty"`
}

// CUICStatus represents operation status
type CUICStatus struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Success   bool   `json:"success"`
	Retriable bool   `json:"retriable,omitempty"`
}

// Message types
const (
	MessageTypeStream      = "stream"      // Multiplexed stream message
	MessageTypeDatagram    = "datagram"    // Unreliable datagram
	MessageTypeRequest     = "request"     // Request-response pattern
	MessageTypeResponse    = "response"    // Response to request
	MessageTypeControl     = "control"     // Control flow message
	MessageTypeHeartbeat   = "heartbeat"   // Connection keepalive
)

// Priority levels (0-255, higher = more urgent)
const (
	PriorityLow      = 64
	PriorityNormal   = 128
	PriorityHigh     = 192
	PriorityCritical = 255
)

// Congestion hints (CUBIC-inspired)
const (
	CongestionNone     = "none"
	CongestionModerate = "moderate"
	CongestionSevere   = "severe"
)

func NewCUICServer(gpuSvc *gpu.Service, billingSvc *billing.Service, authSvc *auth.Service, gr *middleware.GuardRails) *CUICServer {
	return &CUICServer{
		gpuService:     gpuSvc,
		billingService: billingSvc,
		authService:    authSvc,
		guardRails:     gr,
	}
}

// HandleMessage processes a CUIC message
func (s *CUICServer) HandleMessage(ctx context.Context, msgBody []byte) ([]byte, error) {
	var msg CUICMessage
	if err := json.Unmarshal(msgBody, &msg); err != nil {
		return s.errorResponse("", "", "", "PARSE_ERROR", "Invalid message format", 400, false)
	}

	// Validate and set defaults
	if err := s.validateMessage(&msg); err != nil {
		return s.errorResponse(msg.StreamID, msg.MessageID, msg.Sender, "INVALID_MESSAGE", err.Error(), 400, false)
	}

	// Check authentication
	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return s.errorResponse(msg.StreamID, msg.MessageID, msg.Sender, "UNAUTHORIZED", "Authentication required", 401, false)
	}

	// Handle based on message type
	var response interface{}
	var err error
	var congestionHint string

	switch msg.MessageType {
	case MessageTypeRequest, MessageTypeStream:
		response, congestionHint, err = s.handleRequest(ctx, msg)
	case MessageTypeDatagram:
		response, congestionHint, err = s.handleDatagram(ctx, msg)
	case MessageTypeControl:
		response, congestionHint, err = s.handleControl(ctx, msg)
	case MessageTypeHeartbeat:
		response, congestionHint, err = s.handleHeartbeat(ctx, msg)
	default:
		return s.errorResponse(msg.StreamID, msg.MessageID, msg.Sender, "UNKNOWN_TYPE", fmt.Sprintf("Unknown message type: %s", msg.MessageType), 400, false)
	}

	if err != nil {
		// Determine if error is retriable
		retriable := s.isRetriable(err)
		return s.errorResponse(msg.StreamID, msg.MessageID, msg.Sender, "PROCESSING_ERROR", err.Error(), 500, retriable)
	}

	return s.successResponse(msg, response, 200, congestionHint)
}

func (s *CUICServer) validateMessage(msg *CUICMessage) error {
	if msg.Version == "" {
		msg.Version = "1.0"
	}
	if msg.MessageID == "" {
		msg.MessageID = uuid.New().String()
	}
	if msg.StreamID == "" {
		msg.StreamID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Priority == 0 {
		msg.Priority = PriorityNormal
	}
	if msg.Sender == "" {
		return fmt.Errorf("sender is required")
	}
	if msg.MessageType == "" {
		return fmt.Errorf("message_type is required")
	}
	return nil
}

func (s *CUICServer) handleRequest(ctx context.Context, msg CUICMessage) (interface{}, string, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, CongestionNone, fmt.Errorf("invalid payload format")
	}

	operation, _ := payload["operation"].(string)
	parameters, _ := payload["parameters"].(map[string]interface{})

	// Simulate congestion detection based on system load
	congestionHint := s.detectCongestion(ctx)

	switch operation {
	case "gpu.list":
		data, err := s.operationGPUList(ctx, parameters)
		return data, congestionHint, err
	case "gpu.create":
		data, err := s.operationGPUCreate(ctx, parameters)
		return data, congestionHint, err
	case "gpu.destroy":
		data, err := s.operationGPUDestroy(ctx, parameters)
		return data, congestionHint, err
	case "gpu.status":
		data, err := s.operationGPUStatus(ctx, parameters)
		return data, congestionHint, err
	case "billing.query":
		data, err := s.operationBillingQuery(ctx, parameters)
		return data, congestionHint, err
	case "guardrails.check":
		data, err := s.operationGuardRailsCheck(ctx, parameters)
		return data, congestionHint, err
	case "stream.info":
		data, err := s.operationStreamInfo(ctx, msg.StreamID)
		return data, congestionHint, err
	default:
		return nil, congestionHint, fmt.Errorf("unknown operation: %s", operation)
	}
}

func (s *CUICServer) handleDatagram(ctx context.Context, msg CUICMessage) (interface{}, string, error) {
	// Datagram messages are fire-and-forget
	return map[string]interface{}{
		"acknowledged": true,
		"message_id":   msg.MessageID,
		"stream_id":    msg.StreamID,
	}, CongestionNone, nil
}

func (s *CUICServer) handleControl(ctx context.Context, msg CUICMessage) (interface{}, string, error) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return nil, CongestionNone, fmt.Errorf("invalid payload format")
	}

	controlType, _ := payload["control_type"].(string)

	switch controlType {
	case "stream.open":
		return map[string]interface{}{
			"stream_id": msg.StreamID,
			"status":    "open",
			"max_data":  1048576, // 1MB window
		}, CongestionNone, nil
	case "stream.close":
		return map[string]interface{}{
			"stream_id": msg.StreamID,
			"status":    "closed",
		}, CongestionNone, nil
	case "flow.control":
		return map[string]interface{}{
			"stream_id":    msg.StreamID,
			"window_size":  1048576,
			"max_streams":  100,
			"congestion":   s.detectCongestion(ctx),
		}, s.detectCongestion(ctx), nil
	default:
		return nil, CongestionNone, fmt.Errorf("unknown control type: %s", controlType)
	}
}

func (s *CUICServer) handleHeartbeat(ctx context.Context, msg CUICMessage) (interface{}, string, error) {
	return map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now(),
		"stream_id": msg.StreamID,
		"latency":   time.Since(msg.Timestamp).Milliseconds(),
	}, CongestionNone, nil
}

// Operation implementations

func (s *CUICServer) operationGPUList(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	provider, _ := params["provider"].(string)
	if provider == "" {
		provider = "all"
	}

	instances, err := s.gpuService.ListInstances(ctx, gpu.Provider(provider))
	if err != nil {
		return nil, err
	}

	// Apply filters
	if minVRAM, ok := params["min_vram"].(float64); ok {
		filters := map[string]interface{}{"min_vram": int(minVRAM)}
		instances = s.gpuService.FilterInstances(instances, filters)
	}
	if maxPrice, ok := params["max_price"].(float64); ok {
		filters := map[string]interface{}{"max_price": maxPrice}
		instances = s.gpuService.FilterInstances(instances, filters)
	}

	return map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
		"provider":  provider,
	}, nil
}

func (s *CUICServer) operationGPUCreate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *CUICServer) operationGPUDestroy(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *CUICServer) operationGPUStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	provider, _ := params["provider"].(string)
	instanceID, _ := params["instance_id"].(string)

	if provider == "" || instanceID == "" {
		return nil, fmt.Errorf("provider and instance_id are required")
	}

	// Get instance status
	instances, err := s.gpuService.ListInstances(ctx, gpu.Provider(provider))
	if err != nil {
		return nil, err
	}

	for _, inst := range instances {
		if inst.ID == instanceID {
			status := "available"
			if !inst.Available {
				status = "unavailable"
			}
			return map[string]interface{}{
				"instance_id": instanceID,
				"provider":    provider,
				"status":      status,
				"gpu_name":    inst.GPUName,
				"vram_gb":     inst.VRAM,
				"available":   inst.Available,
			}, nil
		}
	}

	return nil, fmt.Errorf("instance not found: %s", instanceID)
}

func (s *CUICServer) operationBillingQuery(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *CUICServer) operationGuardRailsCheck(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *CUICServer) operationStreamInfo(ctx context.Context, streamID string) (interface{}, error) {
	return map[string]interface{}{
		"stream_id":   streamID,
		"status":      "active",
		"window_size": 1048576,
		"protocol":    "CUIC/1.0",
	}, nil
}

// Helper functions

func (s *CUICServer) detectCongestion(ctx context.Context) string {
	// Simplified congestion detection
	// In a real implementation, this would monitor system metrics
	return CongestionNone
}

func (s *CUICServer) isRetriable(err error) bool {
	// Determine if error is retriable based on error type
	errStr := err.Error()
	retriableErrors := []string{"timeout", "unavailable", "connection", "temporary"}
	for _, r := range retriableErrors {
		if contains(errStr, r) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Response helpers

func (s *CUICServer) successResponse(msg CUICMessage, data interface{}, code int, congestionHint string) ([]byte, error) {
	resp := CUICResponse{
		StreamID:  msg.StreamID,
		MessageID: uuid.New().String(),
		InReplyTo: msg.MessageID,
		Version:   msg.Version,
		Sender:    "aiserve-gpuproxy",
		Receiver:  msg.Sender,
		Status: CUICStatus{
			Code:    code,
			Message: "success",
			Success: true,
		},
		Payload:        data,
		Timestamp:      time.Now(),
		CongestionHint: congestionHint,
	}

	return json.Marshal(resp)
}

func (s *CUICServer) errorResponse(streamID, messageID, receiver, errorCode, message string, statusCode int, retriable bool) ([]byte, error) {
	if streamID == "" {
		streamID = uuid.New().String()
	}

	resp := CUICResponse{
		StreamID:  streamID,
		MessageID: uuid.New().String(),
		InReplyTo: messageID,
		Version:   "1.0",
		Sender:    "aiserve-gpuproxy",
		Receiver:  receiver,
		Status: CUICStatus{
			Code:      statusCode,
			Message:   message,
			Success:   false,
			Retriable: retriable,
		},
		Payload: map[string]interface{}{
			"error":      errorCode,
			"details":    message,
			"message_id": messageID,
			"stream_id":  streamID,
		},
		Timestamp:      time.Now(),
		CongestionHint: CongestionNone,
	}

	return json.Marshal(resp)
}
