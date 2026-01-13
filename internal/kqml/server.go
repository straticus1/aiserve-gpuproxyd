package kqml

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/billing"
	"github.com/aiserve/gpuproxy/internal/gpu"
	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/google/uuid"
)

// KQMLServer implements the Knowledge Query and Manipulation Language protocol
type KQMLServer struct {
	gpuService     *gpu.Service
	billingService *billing.Service
	authService    *auth.Service
	guardRails     *middleware.GuardRails
}

// KQMLMessage represents a KQML message
type KQMLMessage struct {
	Performative string                 `json:"performative"`
	Sender       string                 `json:"sender"`
	Receiver     string                 `json:"receiver"`
	ReplyWith    string                 `json:"reply-with,omitempty"`
	InReplyTo    string                 `json:"in-reply-to,omitempty"`
	Language     string                 `json:"language,omitempty"`
	Ontology     string                 `json:"ontology,omitempty"`
	Content      map[string]interface{} `json:"content"`
}

// KQML Performatives (message types)
const (
	// Information performatives
	PerformativeAsk      = "ask"
	PerformativeAskOne   = "ask-one"
	PerformativeAskAll   = "ask-all"
	PerformativeTell     = "tell"
	PerformativeUntell   = "untell"
	PerformativeReply    = "reply"
	PerformativeSorry    = "sorry"

	// Action performatives
	PerformativeAchieve  = "achieve"
	PerformativeCancel   = "cancel"

	// Capability performatives
	PerformativeAdvertise = "advertise"
	PerformativeSubscribe = "subscribe"
	PerformativeMonitor   = "monitor"

	// Network performatives
	PerformativeRegister   = "register"
	PerformativeUnregister = "unregister"
	PerformativeForward    = "forward"
	PerformativeBroadcast  = "broadcast"

	// Meta performatives
	PerformativeError = "error"
)

func NewKQMLServer(gpuSvc *gpu.Service, billingSvc *billing.Service, authSvc *auth.Service, gr *middleware.GuardRails) *KQMLServer {
	return &KQMLServer{
		gpuService:     gpuSvc,
		billingService: billingSvc,
		authService:    authSvc,
		guardRails:     gr,
	}
}

// HandleMessage processes a KQML message
func (s *KQMLServer) HandleMessage(ctx context.Context, msgBody []byte) ([]byte, error) {
	var msg KQMLMessage

	// Try JSON format first
	if err := json.Unmarshal(msgBody, &msg); err != nil {
		// Try parsing KQML string format
		parsedMsg, parseErr := s.parseKQMLString(string(msgBody))
		if parseErr != nil {
			return s.errorResponse("", "PARSE_ERROR", "Invalid KQML message format")
		}
		msg = *parsedMsg
	}

	// Validate message
	if msg.Performative == "" {
		return s.errorResponse("", "INVALID_MESSAGE", "Performative is required")
	}
	if msg.Sender == "" {
		return s.errorResponse("", "INVALID_MESSAGE", "Sender is required")
	}

	// Check authentication
	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return s.errorResponse(msg.ReplyWith, "UNAUTHORIZED", "Authentication required")
	}

	// Route based on performative
	switch strings.ToLower(msg.Performative) {
	case PerformativeAsk, PerformativeAskOne, PerformativeAskAll:
		return s.handleAsk(ctx, msg)
	case PerformativeTell:
		return s.handleTell(ctx, msg)
	case PerformativeAchieve:
		return s.handleAchieve(ctx, msg)
	case PerformativeAdvertise:
		return s.handleAdvertise(ctx, msg)
	case PerformativeSubscribe:
		return s.handleSubscribe(ctx, msg)
	case PerformativeRegister:
		return s.handleRegister(ctx, msg)
	default:
		return s.errorResponse(msg.ReplyWith, "UNKNOWN_PERFORMATIVE",
			fmt.Sprintf("Unknown performative: %s", msg.Performative))
	}
}

func (s *KQMLServer) handleAsk(ctx context.Context, msg KQMLMessage) ([]byte, error) {
	query, ok := msg.Content["query"].(string)
	if !ok {
		return s.errorResponse(msg.ReplyWith, "INVALID_CONTENT", "Query is required")
	}

	var result interface{}
	var err error

	switch query {
	case "gpu-instances":
		result, err = s.queryGPUInstances(ctx, msg.Content)
	case "spending-info":
		result, err = s.querySpendingInfo(ctx)
	case "billing-transactions":
		result, err = s.queryBillingTransactions(ctx)
	case "capabilities":
		result = s.queryCapabilities()
	default:
		return s.errorResponse(msg.ReplyWith, "UNKNOWN_QUERY",
			fmt.Sprintf("Unknown query: %s", query))
	}

	if err != nil {
		return s.errorResponse(msg.ReplyWith, "QUERY_ERROR", err.Error())
	}

	return s.replyResponse(msg.ReplyWith, msg.Sender, result)
}

func (s *KQMLServer) handleTell(ctx context.Context, msg KQMLMessage) ([]byte, error) {
	action, ok := msg.Content["action"].(string)
	if !ok {
		return s.errorResponse(msg.ReplyWith, "INVALID_CONTENT", "Action is required")
	}

	var result interface{}
	var err error

	switch action {
	case "record-spending":
		result, err = s.actionRecordSpending(ctx, msg.Content)
	default:
		return s.errorResponse(msg.ReplyWith, "UNKNOWN_ACTION",
			fmt.Sprintf("Unknown action: %s", action))
	}

	if err != nil {
		return s.errorResponse(msg.ReplyWith, "ACTION_ERROR", err.Error())
	}

	return s.replyResponse(msg.ReplyWith, msg.Sender, result)
}

func (s *KQMLServer) handleAchieve(ctx context.Context, msg KQMLMessage) ([]byte, error) {
	goal, ok := msg.Content["goal"].(string)
	if !ok {
		return s.errorResponse(msg.ReplyWith, "INVALID_CONTENT", "Goal is required")
	}

	var result interface{}
	var err error

	switch goal {
	case "create-gpu-instance":
		result, err = s.achieveCreateGPU(ctx, msg.Content)
	case "destroy-gpu-instance":
		result, err = s.achieveDestroyGPU(ctx, msg.Content)
	default:
		return s.errorResponse(msg.ReplyWith, "UNKNOWN_GOAL",
			fmt.Sprintf("Unknown goal: %s", goal))
	}

	if err != nil {
		return s.errorResponse(msg.ReplyWith, "GOAL_ERROR", err.Error())
	}

	return s.replyResponse(msg.ReplyWith, msg.Sender, result)
}

func (s *KQMLServer) handleAdvertise(ctx context.Context, msg KQMLMessage) ([]byte, error) {
	capabilities := s.queryCapabilities()
	return s.replyResponse(msg.ReplyWith, msg.Sender, capabilities)
}

func (s *KQMLServer) handleSubscribe(ctx context.Context, msg KQMLMessage) ([]byte, error) {
	topic, _ := msg.Content["topic"].(string)

	result := map[string]interface{}{
		"subscribed": true,
		"topic":      topic,
		"message":    "Subscription registered",
	}

	return s.replyResponse(msg.ReplyWith, msg.Sender, result)
}

func (s *KQMLServer) handleRegister(ctx context.Context, msg KQMLMessage) ([]byte, error) {
	result := map[string]interface{}{
		"registered": true,
		"agent":      msg.Sender,
		"timestamp":  time.Now(),
	}

	return s.replyResponse(msg.ReplyWith, msg.Sender, result)
}

// Query implementations

func (s *KQMLServer) queryGPUInstances(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *KQMLServer) querySpendingInfo(ctx context.Context) (interface{}, error) {
	userID := middleware.GetUserID(ctx)
	return s.guardRails.GetSpendingInfo(ctx, userID)
}

func (s *KQMLServer) queryBillingTransactions(ctx context.Context) (interface{}, error) {
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

func (s *KQMLServer) queryCapabilities() interface{} {
	return map[string]interface{}{
		"agent": "aiserve-gpuproxy",
		"protocol": "KQML",
		"performatives": []string{
			PerformativeAsk, PerformativeAskOne, PerformativeAskAll,
			PerformativeTell, PerformativeAchieve, PerformativeAdvertise,
			PerformativeSubscribe, PerformativeRegister,
		},
		"queries": []string{
			"gpu-instances", "spending-info", "billing-transactions", "capabilities",
		},
		"actions": []string{
			"record-spending",
		},
		"goals": []string{
			"create-gpu-instance", "destroy-gpu-instance",
		},
	}
}

// Action implementations

func (s *KQMLServer) actionRecordSpending(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	amount, ok := params["amount"].(float64)
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

// Goal implementations

func (s *KQMLServer) achieveCreateGPU(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *KQMLServer) achieveDestroyGPU(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

// Response helpers

func (s *KQMLServer) replyResponse(replyWith, receiver string, content interface{}) ([]byte, error) {
	msg := KQMLMessage{
		Performative: PerformativeReply,
		Sender:       "aiserve-gpuproxy",
		Receiver:     receiver,
		InReplyTo:    replyWith,
		Language:     "JSON",
		Ontology:     "gpu-proxy-v1",
		Content: map[string]interface{}{
			"result": content,
		},
	}
	return json.Marshal(msg)
}

func (s *KQMLServer) errorResponse(replyWith, errorCode, message string) ([]byte, error) {
	msg := KQMLMessage{
		Performative: PerformativeSorry,
		Sender:       "aiserve-gpuproxy",
		InReplyTo:    replyWith,
		Language:     "JSON",
		Content: map[string]interface{}{
			"error":   errorCode,
			"message": message,
		},
	}
	return json.Marshal(msg)
}

// parseKQMLString parses KQML string format into KQMLMessage
// Format: (performative :sender "agent" :receiver "server" :content (...))
func (s *KQMLServer) parseKQMLString(kqmlStr string) (*KQMLMessage, error) {
	kqmlStr = strings.TrimSpace(kqmlStr)
	if !strings.HasPrefix(kqmlStr, "(") || !strings.HasSuffix(kqmlStr, ")") {
		return nil, fmt.Errorf("invalid KQML format: must be enclosed in parentheses")
	}

	// This is a simplified parser - production would need a proper KQML parser
	// For now, we'll require JSON format for full functionality
	return nil, fmt.Errorf("KQML string format not fully implemented, please use JSON format")
}

// ToKQMLString converts message to KQML string format
func (msg *KQMLMessage) ToKQMLString() string {
	var parts []string
	parts = append(parts, msg.Performative)
	parts = append(parts, fmt.Sprintf(":sender %s", msg.Sender))
	if msg.Receiver != "" {
		parts = append(parts, fmt.Sprintf(":receiver %s", msg.Receiver))
	}
	if msg.ReplyWith != "" {
		parts = append(parts, fmt.Sprintf(":reply-with %s", msg.ReplyWith))
	}
	if msg.InReplyTo != "" {
		parts = append(parts, fmt.Sprintf(":in-reply-to %s", msg.InReplyTo))
	}
	if msg.Language != "" {
		parts = append(parts, fmt.Sprintf(":language %s", msg.Language))
	}
	if msg.Ontology != "" {
		parts = append(parts, fmt.Sprintf(":ontology %s", msg.Ontology))
	}

	return "(" + strings.Join(parts, " ") + ")"
}
