package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"aiserve-gpuproxyd/internal/config"
)

// CloudflareProvider implements the Provider interface for Cloudflare Workers AI
type CloudflareProvider struct {
	config     *config.CloudflareProviderConfig
	httpClient *http.Client
	baseURL    string
}

// CloudflareRequest represents a request to Cloudflare Workers AI
type CloudflareRequest struct {
	Prompt      string                 `json:"prompt,omitempty"`
	Messages    []CloudflareMessage    `json:"messages,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Raw         bool                   `json:"raw,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

// CloudflareMessage represents a message in a chat completion request
type CloudflareMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CloudflareResponse represents a response from Cloudflare Workers AI
type CloudflareResponse struct {
	Success  bool                   `json:"success"`
	Errors   []CloudflareError      `json:"errors"`
	Messages []string               `json:"messages"`
	Result   CloudflareResult       `json:"result"`
}

// CloudflareError represents an error from Cloudflare API
type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CloudflareResult represents the result of a Cloudflare AI inference
type CloudflareResult struct {
	Response string                 `json:"response,omitempty"`
	Text     string                 `json:"text,omitempty"`
	Choices  []CloudflareChoice     `json:"choices,omitempty"`
	Usage    *CloudflareUsage       `json:"usage,omitempty"`
	Extra    map[string]interface{} `json:"-"`
}

// CloudflareChoice represents a completion choice
type CloudflareChoice struct {
	Index   int               `json:"index"`
	Message CloudflareMessage `json:"message"`
}

// CloudflareUsage represents token usage information
type CloudflareUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewCloudflareProvider creates a new Cloudflare Workers AI provider
func NewCloudflareProvider(cfg *config.CloudflareProviderConfig) (*CloudflareProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cloudflare config is required")
	}

	if cfg.Credentials.AccountID == "" {
		return nil, fmt.Errorf("cloudflare account ID is required")
	}

	if cfg.Credentials.APIToken == "" {
		return nil, fmt.Errorf("cloudflare API token is required")
	}

	return &CloudflareProvider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL: fmt.Sprintf("%s/accounts/%s/ai/run",
			cfg.Endpoint,
			cfg.Credentials.AccountID,
		),
	}, nil
}

// Name returns the provider name
func (p *CloudflareProvider) Name() string {
	return "cloudflare"
}

// Type returns the provider type
func (p *CloudflareProvider) Type() string {
	return "cloudflare"
}

// IsAvailable checks if the provider is available
func (p *CloudflareProvider) IsAvailable(ctx context.Context) bool {
	// Simple health check - try to list models or ping endpoint
	return true // For now, assume available if configured
}

// GetModels returns the list of available models
func (p *CloudflareProvider) GetModels() []string {
	models := make([]string, 0, len(p.config.Models))
	for _, model := range p.config.Models {
		models = append(models, model.Name)
	}
	return models
}

// Predict performs inference with a model
func (p *CloudflareProvider) Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error) {
	// Find the model configuration
	var modelConfig *config.CloudflareModelConfig
	for i, m := range p.config.Models {
		if m.Name == req.Model {
			modelConfig = &p.config.Models[i]
			break
		}
	}

	if modelConfig == nil {
		return nil, fmt.Errorf("model %s not found in cloudflare provider configuration", req.Model)
	}

	// Build the request
	cfReq := CloudflareRequest{
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}

	// Convert input format
	if messages, ok := req.Input.([]interface{}); ok {
		// Chat completion format
		cfReq.Messages = make([]CloudflareMessage, 0, len(messages))
		for _, msg := range messages {
			if m, ok := msg.(map[string]interface{}); ok {
				cfReq.Messages = append(cfReq.Messages, CloudflareMessage{
					Role:    fmt.Sprintf("%v", m["role"]),
					Content: fmt.Sprintf("%v", m["content"]),
				})
			}
		}
	} else if prompt, ok := req.Input.(string); ok {
		// Simple prompt format
		cfReq.Prompt = prompt
	} else {
		return nil, fmt.Errorf("unsupported input format")
	}

	// Make the API call
	url := fmt.Sprintf("%s/%s", p.baseURL, modelConfig.CloudflareModel)

	startTime := time.Now()
	response, err := p.makeRequest(ctx, url, cfReq)
	if err != nil {
		return nil, fmt.Errorf("cloudflare API request failed: %w", err)
	}
	latency := time.Since(startTime)

	// Extract output text
	var output string
	if response.Result.Response != "" {
		output = response.Result.Response
	} else if response.Result.Text != "" {
		output = response.Result.Text
	} else if len(response.Result.Choices) > 0 {
		output = response.Result.Choices[0].Message.Content
	} else {
		return nil, fmt.Errorf("no output in cloudflare response")
	}

	// Calculate tokens and cost
	var inputTokens, outputTokens int
	if response.Result.Usage != nil {
		inputTokens = response.Result.Usage.PromptTokens
		outputTokens = response.Result.Usage.CompletionTokens
	} else {
		// Estimate tokens if not provided
		inputTokens = estimateTokens(req.Input)
		outputTokens = estimateTokens(output)
	}

	totalTokens := inputTokens + outputTokens
	cost := float64(totalTokens) / 1000.0 * modelConfig.CostPer1kTokens

	return &PredictResponse{
		Output: output,
		Metadata: ResponseMetadata{
			Provider:     p.Name(),
			Model:        req.Model,
			LatencyMs:    int(latency.Milliseconds()),
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  totalTokens,
			Cost:         cost,
			Currency:     "USD",
		},
	}, nil
}

// makeRequest makes an HTTP request to Cloudflare API
func (p *CloudflareProvider) makeRequest(ctx context.Context, url string, req CloudflareRequest) (*CloudflareResponse, error) {
	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Authorization", "Bearer "+p.config.Credentials.APIToken)
	httpReq.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var cfResp CloudflareResponse
	if err := json.Unmarshal(respBody, &cfResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for errors
	if !cfResp.Success {
		if len(cfResp.Errors) > 0 {
			return nil, fmt.Errorf("cloudflare API error: %s", cfResp.Errors[0].Message)
		}
		return nil, fmt.Errorf("cloudflare API request unsuccessful")
	}

	return &cfResp, nil
}

// Health returns the provider health status
func (p *CloudflareProvider) Health(ctx context.Context) ProviderHealth {
	health := ProviderHealth{
		Provider:  p.Name(),
		Healthy:   true,
		Available: true,
		Latency:   0,
	}

	// Simple availability check
	if !p.IsAvailable(ctx) {
		health.Healthy = false
		health.Available = false
		health.Message = "Provider unavailable"
	}

	return health
}

// Priority returns the provider priority
func (p *CloudflareProvider) Priority() int {
	return p.config.Priority
}

// GetCostPer1kTokens returns the cost per 1k tokens for a model
func (p *CloudflareProvider) GetCostPer1kTokens(model string) float64 {
	for _, m := range p.config.Models {
		if m.Name == model {
			return m.CostPer1kTokens
		}
	}
	return 0.0
}

// estimateTokens estimates the number of tokens in text (rough approximation)
func estimateTokens(input interface{}) int {
	var text string
	switch v := input.(type) {
	case string:
		text = v
	case []interface{}:
		// For message arrays, concatenate all content
		for _, msg := range v {
			if m, ok := msg.(map[string]interface{}); ok {
				if content, ok := m["content"].(string); ok {
					text += content + " "
				}
			}
		}
	default:
		return 0
	}

	// Rough estimate: 1 token ≈ 4 characters
	return len(text) / 4
}
