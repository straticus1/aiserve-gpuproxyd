package providers

import (
	"context"
)

// Provider defines the interface that all AI providers must implement
type Provider interface {
	// Name returns the provider name
	Name() string

	// Type returns the provider type (local, cloudflare, openai, etc.)
	Type() string

	// IsAvailable checks if the provider is currently available
	IsAvailable(ctx context.Context) bool

	// GetModels returns the list of available models
	GetModels() []string

	// Predict performs inference with a model
	Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error)

	// Health returns the provider health status
	Health(ctx context.Context) ProviderHealth

	// Priority returns the provider priority (lower = higher priority)
	Priority() int

	// GetCostPer1kTokens returns the cost per 1k tokens for a model
	GetCostPer1kTokens(model string) float64
}

// PredictRequest represents a prediction request
type PredictRequest struct {
	Model       string      `json:"model"`
	Input       interface{} `json:"input"` // Can be string (prompt) or []Message (chat)
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Temperature float64     `json:"temperature,omitempty"`
	TopP        float64     `json:"top_p,omitempty"`
	Stop        []string    `json:"stop,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
}

// PredictResponse represents a prediction response
type PredictResponse struct {
	Output   interface{}      `json:"output"`
	Metadata ResponseMetadata `json:"metadata"`
}

// ResponseMetadata contains metadata about the response
type ResponseMetadata struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	LatencyMs    int     `json:"latency_ms"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost"`
	Currency     string  `json:"currency"`
	Cached       bool    `json:"cached,omitempty"`
}

// ProviderHealth represents the health status of a provider
type ProviderHealth struct {
	Provider  string `json:"provider"`
	Healthy   bool   `json:"healthy"`
	Available bool   `json:"available"`
	Latency   int    `json:"latency_ms"`
	Message   string `json:"message,omitempty"`
	ErrorRate float64 `json:"error_rate,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
