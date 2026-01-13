package fipa

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

// FIPAServer implements the FIPA Agent Communication Language (ACL)
type FIPAServer struct {
	gpuService     *gpu.Service
	billingService *billing.Service
	authService    *auth.Service
	guardRails     *middleware.GuardRails
}

// FIPAMessage represents a FIPA ACL message
type FIPAMessage struct {
	Performative string        `json:"performative"`
	Sender       AgentID       `json:"sender"`
	Receiver     []AgentID     `json:"receiver"`
	ReplyTo      []AgentID     `json:"reply-to,omitempty"`
	Content      interface{}   `json:"content"`
	Language     string        `json:"language,omitempty"`
	Encoding     string        `json:"encoding,omitempty"`
	Ontology     string        `json:"ontology,omitempty"`
	Protocol     string        `json:"protocol,omitempty"`
	ConversationID string      `json:"conversation-id,omitempty"`
	ReplyWith    string        `json:"reply-with,omitempty"`
	InReplyTo    string        `json:"in-reply-to,omitempty"`
	ReplyBy      *time.Time    `json:"reply-by,omitempty"`
}

// AgentID represents a FIPA agent identifier
type AgentID struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses,omitempty"`
}

// FIPA ACL Communicative Acts (Performatives)
const (
	// Passing information
	PerformativeInform      = "inform"
	PerformativeConfirm     = "confirm"
	PerformativeDisconfirm  = "disconfirm"
	PerformativeNotUnderstood = "not-understood"

	// Requesting information
	PerformativeQueryIf     = "query-if"
	PerformativeQueryRef    = "query-ref"
	PerformativeSubscribe   = "subscribe"

	// Requesting action
	PerformativeRequest     = "request"
	PerformativeRequestWhen = "request-when"
	PerformativeRequestWhenever = "request-whenever"

	// Negotiation
	PerformativePropose     = "propose"
	PerformativeAcceptProposal = "accept-proposal"
	PerformativeRejectProposal = "reject-proposal"
	PerformativeCFP         = "cfp" // Call for Proposal

	// Error handling
	PerformativeFailure     = "failure"
	PerformativeRefuse      = "refuse"

	// Agreement
	PerformativeAgree       = "agree"
	PerformativeCancel      = "cancel"
)

// FIPA Interaction Protocols
const (
	ProtocolRequest          = "fipa-request"
	ProtocolQueryRef         = "fipa-query"
	ProtocolContractNet      = "fipa-contract-net"
	ProtocolSubscribe        = "fipa-subscribe"
	ProtocolPropose          = "fipa-propose"
)

func NewFIPAServer(gpuSvc *gpu.Service, billingSvc *billing.Service, authSvc *auth.Service, gr *middleware.GuardRails) *FIPAServer {
	return &FIPAServer{
		gpuService:     gpuSvc,
		billingService: billingSvc,
		authService:    authSvc,
		guardRails:     gr,
	}
}

// HandleMessage processes a FIPA ACL message
func (s *FIPAServer) HandleMessage(ctx context.Context, msgBody []byte) ([]byte, error) {
	var msg FIPAMessage
	if err := json.Unmarshal(msgBody, &msg); err != nil {
		return s.errorResponse("", AgentID{}, "PARSE_ERROR", "Invalid FIPA ACL message format")
	}

	// Validate message
	if msg.Performative == "" {
		return s.errorResponse("", msg.Sender, "INVALID_MESSAGE", "Performative is required")
	}
	if msg.Sender.Name == "" {
		return s.errorResponse("", AgentID{}, "INVALID_MESSAGE", "Sender is required")
	}

	// Set defaults
	if msg.Language == "" {
		msg.Language = "JSON"
	}
	if msg.Encoding == "" {
		msg.Encoding = "UTF-8"
	}
	if msg.Ontology == "" {
		msg.Ontology = "gpu-proxy-ontology"
	}

	// Check authentication
	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		return s.errorResponse(msg.ReplyWith, msg.Sender, "UNAUTHORIZED", "Authentication required")
	}

	// Route based on performative
	switch msg.Performative {
	case PerformativeQueryRef:
		return s.handleQueryRef(ctx, msg)
	case PerformativeQueryIf:
		return s.handleQueryIf(ctx, msg)
	case PerformativeRequest:
		return s.handleRequest(ctx, msg)
	case PerformativeSubscribe:
		return s.handleSubscribe(ctx, msg)
	case PerformativePropose:
		return s.handlePropose(ctx, msg)
	case PerformativeCFP:
		return s.handleCFP(ctx, msg)
	case PerformativeAcceptProposal:
		return s.handleAcceptProposal(ctx, msg)
	case PerformativeRejectProposal:
		return s.handleRejectProposal(ctx, msg)
	default:
		return s.notUnderstoodResponse(msg.ReplyWith, msg.Sender,
			fmt.Sprintf("Unknown performative: %s", msg.Performative))
	}
}

func (s *FIPAServer) handleQueryRef(ctx context.Context, msg FIPAMessage) ([]byte, error) {
	content, ok := msg.Content.(map[string]interface{})
	if !ok {
		return s.failureResponse(msg.ReplyWith, msg.Sender, "Invalid content format")
	}

	query, _ := content["query"].(string)
	var result interface{}
	var err error

	switch query {
	case "gpu-instances":
		result, err = s.queryGPUInstances(ctx, content)
	case "spending-info":
		result, err = s.querySpendingInfo(ctx)
	case "billing-transactions":
		result, err = s.queryBillingTransactions(ctx)
	case "agent-description":
		result = s.getAgentDescription()
	default:
		return s.refuseResponse(msg.ReplyWith, msg.Sender,
			fmt.Sprintf("Unknown query: %s", query))
	}

	if err != nil {
		return s.failureResponse(msg.ReplyWith, msg.Sender, err.Error())
	}

	return s.informResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, result)
}

func (s *FIPAServer) handleQueryIf(ctx context.Context, msg FIPAMessage) ([]byte, error) {
	content, ok := msg.Content.(map[string]interface{})
	if !ok {
		return s.failureResponse(msg.ReplyWith, msg.Sender, "Invalid content format")
	}

	condition, _ := content["condition"].(string)
	var result bool
	var err error

	switch condition {
	case "spending-limit-exceeded":
		estimatedCost, _ := content["estimated_cost"].(float64)
		userID := middleware.GetUserID(ctx)
		info, queryErr := s.guardRails.CheckSpending(ctx, userID, estimatedCost)
		if queryErr != nil {
			return s.failureResponse(msg.ReplyWith, msg.Sender, queryErr.Error())
		}
		result = len(info.Violations) > 0
	case "gpu-available":
		provider, _ := content["provider"].(string)
		instances, queryErr := s.gpuService.ListInstances(ctx, gpu.Provider(provider))
		if queryErr != nil {
			return s.failureResponse(msg.ReplyWith, msg.Sender, queryErr.Error())
		}
		result = len(instances) > 0
	default:
		return s.refuseResponse(msg.ReplyWith, msg.Sender,
			fmt.Sprintf("Unknown condition: %s", condition))
	}

	if err != nil {
		return s.failureResponse(msg.ReplyWith, msg.Sender, err.Error())
	}

	if result {
		return s.confirmResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, content)
	}
	return s.disconfirmResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, content)
}

func (s *FIPAServer) handleRequest(ctx context.Context, msg FIPAMessage) ([]byte, error) {
	content, ok := msg.Content.(map[string]interface{})
	if !ok {
		return s.failureResponse(msg.ReplyWith, msg.Sender, "Invalid content format")
	}

	action, _ := content["action"].(string)
	var result interface{}
	var err error

	switch action {
	case "create-gpu-instance":
		result, err = s.actionCreateGPU(ctx, content)
	case "destroy-gpu-instance":
		result, err = s.actionDestroyGPU(ctx, content)
	case "record-spending":
		result, err = s.actionRecordSpending(ctx, content)
	default:
		return s.refuseResponse(msg.ReplyWith, msg.Sender,
			fmt.Sprintf("Unknown action: %s", action))
	}

	if err != nil {
		return s.failureResponse(msg.ReplyWith, msg.Sender, err.Error())
	}

	// First agree to the request
	_, _ = s.agreeResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, content)

	// Then inform of the result
	return s.informResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, result)
}

func (s *FIPAServer) handleSubscribe(ctx context.Context, msg FIPAMessage) ([]byte, error) {
	content, ok := msg.Content.(map[string]interface{})
	if !ok {
		return s.failureResponse(msg.ReplyWith, msg.Sender, "Invalid content format")
	}

	topic, _ := content["topic"].(string)

	result := map[string]interface{}{
		"subscribed": true,
		"topic":      topic,
		"agent":      "aiserve-gpuproxy",
	}

	return s.agreeResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, result)
}

func (s *FIPAServer) handlePropose(ctx context.Context, msg FIPAMessage) ([]byte, error) {
	// Accept the proposal
	result := map[string]interface{}{
		"proposal-accepted": true,
		"timestamp":         time.Now(),
	}

	return s.acceptProposalResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, result)
}

func (s *FIPAServer) handleCFP(ctx context.Context, msg FIPAMessage) ([]byte, error) {
	content, ok := msg.Content.(map[string]interface{})
	if !ok {
		return s.failureResponse(msg.ReplyWith, msg.Sender, "Invalid content format")
	}

	// Respond with a proposal
	proposal := map[string]interface{}{
		"service":     "gpu-proxy",
		"description": "GPU instance management and billing",
		"capabilities": s.getAgentDescription(),
		"terms":       content,
	}

	return s.proposeResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, proposal)
}

func (s *FIPAServer) handleAcceptProposal(ctx context.Context, msg FIPAMessage) ([]byte, error) {
	result := map[string]interface{}{
		"acknowledged": true,
		"status":       "proposal-accepted",
	}

	return s.informResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, result)
}

func (s *FIPAServer) handleRejectProposal(ctx context.Context, msg FIPAMessage) ([]byte, error) {
	result := map[string]interface{}{
		"acknowledged": true,
		"status":       "proposal-rejected",
	}

	return s.informResponse(msg.ReplyWith, msg.Sender, msg.ConversationID, result)
}

// Query implementations

func (s *FIPAServer) queryGPUInstances(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *FIPAServer) querySpendingInfo(ctx context.Context) (interface{}, error) {
	userID := middleware.GetUserID(ctx)
	return s.guardRails.GetSpendingInfo(ctx, userID)
}

func (s *FIPAServer) queryBillingTransactions(ctx context.Context) (interface{}, error) {
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

func (s *FIPAServer) getAgentDescription() map[string]interface{} {
	return map[string]interface{}{
		"name":     "aiserve-gpuproxy",
		"type":     "gpu-resource-manager",
		"protocol": "FIPA-ACL",
		"ontologies": []string{"gpu-proxy-ontology"},
		"languages":  []string{"JSON", "XML"},
		"services": []string{
			"gpu-instance-management",
			"billing-management",
			"spending-control",
			"guard-rails",
		},
		"protocols": []string{
			ProtocolRequest,
			ProtocolQueryRef,
			ProtocolSubscribe,
			ProtocolPropose,
		},
	}
}

// Action implementations

func (s *FIPAServer) actionCreateGPU(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *FIPAServer) actionDestroyGPU(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

func (s *FIPAServer) actionRecordSpending(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

// Response helpers

func (s *FIPAServer) createResponse(performative, replyWith string, sender AgentID, conversationID string, content interface{}) ([]byte, error) {
	msg := FIPAMessage{
		Performative:   performative,
		Sender:         AgentID{Name: "aiserve-gpuproxy"},
		Receiver:       []AgentID{sender},
		Content:        content,
		Language:       "JSON",
		Encoding:       "UTF-8",
		Ontology:       "gpu-proxy-ontology",
		InReplyTo:      replyWith,
		ConversationID: conversationID,
	}
	return json.Marshal(msg)
}

func (s *FIPAServer) informResponse(replyWith string, sender AgentID, conversationID string, content interface{}) ([]byte, error) {
	return s.createResponse(PerformativeInform, replyWith, sender, conversationID, content)
}

func (s *FIPAServer) confirmResponse(replyWith string, sender AgentID, conversationID string, content interface{}) ([]byte, error) {
	return s.createResponse(PerformativeConfirm, replyWith, sender, conversationID, content)
}

func (s *FIPAServer) disconfirmResponse(replyWith string, sender AgentID, conversationID string, content interface{}) ([]byte, error) {
	return s.createResponse(PerformativeDisconfirm, replyWith, sender, conversationID, content)
}

func (s *FIPAServer) agreeResponse(replyWith string, sender AgentID, conversationID string, content interface{}) ([]byte, error) {
	return s.createResponse(PerformativeAgree, replyWith, sender, conversationID, content)
}

func (s *FIPAServer) refuseResponse(replyWith string, sender AgentID, reason string) ([]byte, error) {
	content := map[string]interface{}{
		"reason": reason,
	}
	return s.createResponse(PerformativeRefuse, replyWith, sender, "", content)
}

func (s *FIPAServer) failureResponse(replyWith string, sender AgentID, reason string) ([]byte, error) {
	content := map[string]interface{}{
		"reason": reason,
	}
	return s.createResponse(PerformativeFailure, replyWith, sender, "", content)
}

func (s *FIPAServer) notUnderstoodResponse(replyWith string, sender AgentID, reason string) ([]byte, error) {
	content := map[string]interface{}{
		"reason": reason,
	}
	return s.createResponse(PerformativeNotUnderstood, replyWith, sender, "", content)
}

func (s *FIPAServer) proposeResponse(replyWith string, sender AgentID, conversationID string, content interface{}) ([]byte, error) {
	return s.createResponse(PerformativePropose, replyWith, sender, conversationID, content)
}

func (s *FIPAServer) acceptProposalResponse(replyWith string, sender AgentID, conversationID string, content interface{}) ([]byte, error) {
	return s.createResponse(PerformativeAcceptProposal, replyWith, sender, conversationID, content)
}

func (s *FIPAServer) errorResponse(replyWith string, sender AgentID, errorCode, message string) ([]byte, error) {
	content := map[string]interface{}{
		"error":   errorCode,
		"message": message,
	}
	return s.createResponse(PerformativeFailure, replyWith, sender, "", content)
}
