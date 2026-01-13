package aiserve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultBaseURL = "https://api.aiserve.farm"
	DefaultTimeout = 30 * time.Second
	SDKVersion     = "1.0.1"
	UserAgent      = "aiserve-go-sdk/" + SDKVersion
)

// Client is the main AIServe API client
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	token      string
	apiKey     string

	// Services
	Auth         *AuthService
	GPU          *GPUService
	Models       *ModelsService
	Billing      *BillingService
	Guardrails   *GuardrailsService
	LoadBalancer *LoadBalancerService
	Quota        *QuotaService
}

// Config holds client configuration
type Config struct {
	BaseURL    string
	APIKey     string
	Token      string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewClient creates a new AIServe API client
func NewClient(config *Config) *Client {
	if config == nil {
		config = &Config{}
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		timeout := config.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	client := &Client{
		BaseURL:    baseURL,
		HTTPClient: httpClient,
		token:      config.Token,
		apiKey:     config.APIKey,
	}

	// Initialize services
	client.Auth = &AuthService{client: client}
	client.GPU = &GPUService{client: client}
	client.Models = &ModelsService{client: client}
	client.Billing = &BillingService{client: client}
	client.Guardrails = &GuardrailsService{client: client}
	client.LoadBalancer = &LoadBalancerService{client: client}
	client.Quota = &QuotaService{client: client}

	return client
}

// SetToken sets the JWT token for authentication
func (c *Client) SetToken(token string) {
	c.token = token
}

// SetAPIKey sets the API key for authentication
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// APIError represents an error response from the API
type APIError struct {
	Message    string                 `json:"error"`
	Code       string                 `json:"code,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	StatusCode int                    `json:"-"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("aiserve: %s (code: %s, status: %d)", e.Message, e.Code, e.StatusCode)
	}
	return fmt.Sprintf("aiserve: %s (status: %d)", e.Message, e.StatusCode)
}

// Request makes an HTTP request to the API
func (c *Client) Request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	// Authentication
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Execute request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, apiErr); err != nil {
			apiErr.Message = string(respBody)
		}
		return apiErr
	}

	// Parse response
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// RequestWithQuery makes an HTTP request with query parameters
func (c *Client) RequestWithQuery(ctx context.Context, method, path string, query url.Values, result interface{}) error {
	if len(query) > 0 {
		path = path + "?" + query.Encode()
	}
	return c.Request(ctx, method, path, nil, result)
}

// AuthService handles authentication operations
type AuthService struct {
	client *Client
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User   User   `json:"user"`
	Tokens Tokens `json:"tokens"`
}

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	req := &LoginRequest{
		Email:    email,
		Password: password,
	}
	var resp LoginResponse
	err := s.client.Request(ctx, "POST", "/api/v1/auth/login", req, &resp)
	return &resp, err
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type RegisterResponse struct {
	User User `json:"user"`
}

func (s *AuthService) Register(ctx context.Context, email, password, name string) (*RegisterResponse, error) {
	req := &RegisterRequest{
		Email:    email,
		Password: password,
		Name:     name,
	}
	var resp RegisterResponse
	err := s.client.Request(ctx, "POST", "/api/v1/auth/register", req, &resp)
	return &resp, err
}

type CreateAPIKeyRequest struct {
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

type CreateAPIKeyResponse struct {
	APIKey    string    `json:"api_key"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

func (s *AuthService) CreateAPIKey(ctx context.Context, req *CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	var resp CreateAPIKeyResponse
	err := s.client.Request(ctx, "POST", "/api/v1/auth/apikey", req, &resp)
	return &resp, err
}

// GPUService handles GPU operations
type GPUService struct {
	client *Client
}

type ListInstancesOptions struct {
	Provider string
	MinVRAM   int
	MaxPrice  float64
	GPUModel  string
	Location  string
}

type GPUInstance struct {
	ID           string  `json:"id"`
	Provider     string  `json:"provider"`
	GPUModel     string  `json:"gpu_model"`
	VRAMGB       int     `json:"vram_gb"`
	PricePerHour float64 `json:"price_per_hour"`
	Location     string  `json:"location"`
	Available    bool    `json:"available"`
}

type ListInstancesResponse struct {
	Instances []GPUInstance `json:"instances"`
	Count     int           `json:"count"`
	Provider  string        `json:"provider"`
}

func (s *GPUService) ListInstances(ctx context.Context, opts *ListInstancesOptions) ([]GPUInstance, error) {
	query := url.Values{}
	if opts != nil {
		if opts.Provider != "" {
			query.Set("provider", opts.Provider)
		}
		if opts.MinVRAM > 0 {
			query.Set("min_vram", fmt.Sprintf("%d", opts.MinVRAM))
		}
		if opts.MaxPrice > 0 {
			query.Set("max_price", fmt.Sprintf("%.2f", opts.MaxPrice))
		}
		if opts.GPUModel != "" {
			query.Set("gpu_model", opts.GPUModel)
		}
		if opts.Location != "" {
			query.Set("location", opts.Location)
		}
	}

	var resp ListInstancesResponse
	err := s.client.RequestWithQuery(ctx, "GET", "/api/v1/gpu/instances", query, &resp)
	return resp.Instances, err
}

type InstanceConfig struct {
	DurationHours int  `json:"duration_hours"`
	AutoRenew     bool `json:"auto_renew"`
}

type CreateInstanceResponse struct {
	ContractID string    `json:"contract_id"`
	Provider   string    `json:"provider"`
	InstanceID string    `json:"instance_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *GPUService) CreateInstance(ctx context.Context, provider, instanceID string, config *InstanceConfig) (*CreateInstanceResponse, error) {
	var resp CreateInstanceResponse
	path := fmt.Sprintf("/api/v1/gpu/instances/%s/%s", provider, instanceID)
	err := s.client.Request(ctx, "POST", path, config, &resp)
	return &resp, err
}

func (s *GPUService) DestroyInstance(ctx context.Context, provider, instanceID string) error {
	path := fmt.Sprintf("/api/v1/gpu/instances/%s/%s", provider, instanceID)
	return s.client.Request(ctx, "DELETE", path, nil, nil)
}

type GPUFilters struct {
	MinVRAM   int     `json:"min_vram,omitempty"`
	GPUModel  string  `json:"gpu_model,omitempty"`
	Location  string  `json:"location,omitempty"`
	MaxPrice  float64 `json:"max_price,omitempty"`
}

type ReserveRequest struct {
	Count   int            `json:"count"`
	Filters *GPUFilters    `json:"filters,omitempty"`
	Config  *InstanceConfig `json:"config,omitempty"`
}

type ReservationInstance struct {
	InstanceID string `json:"instance_id"`
	Provider   string `json:"provider"`
	ContractID string `json:"contract_id"`
}

type ReserveResponse struct {
	Reserved  []ReservationInstance `json:"reserved"`
	Count     int                   `json:"count"`
	Requested int                   `json:"requested"`
	Errors    []string              `json:"errors"`
}

func (s *GPUService) ReserveInstances(ctx context.Context, req *ReserveRequest) (*ReserveResponse, error) {
	var resp ReserveResponse
	err := s.client.Request(ctx, "POST", "/api/v1/gpu/instances/reserve", req, &resp)
	return &resp, err
}

// ModelsService handles model operations
type ModelsService struct {
	client *Client
}

type Model struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Format         string    `json:"format"`
	Version        string    `json:"version"`
	Framework      string    `json:"framework"`
	Status         string    `json:"status"`
	Replicas       int       `json:"replicas"`
	TotalRequests  int64     `json:"total_requests"`
	AverageLatency float64   `json:"average_latency"`
	ErrorRate      float64   `json:"error_rate"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ListModelsResponse struct {
	Models []Model `json:"models"`
	Count  int     `json:"count"`
}

func (s *ModelsService) List(ctx context.Context) ([]Model, error) {
	var resp ListModelsResponse
	err := s.client.Request(ctx, "GET", "/api/v1/models", nil, &resp)
	return resp.Models, err
}

func (s *ModelsService) Get(ctx context.Context, modelID string) (*Model, error) {
	var model Model
	path := fmt.Sprintf("/api/v1/models/%s", modelID)
	err := s.client.Request(ctx, "GET", path, nil, &model)
	return &model, err
}

func (s *ModelsService) Delete(ctx context.Context, modelID string) error {
	path := fmt.Sprintf("/api/v1/models/%s", modelID)
	return s.client.Request(ctx, "DELETE", path, nil, nil)
}

type PredictRequest struct {
	Inputs     map[string]interface{} `json:"inputs"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type PredictResponse struct {
	ModelID   string                 `json:"model_id"`
	Outputs   map[string]interface{} `json:"outputs"`
	LatencyMs float64                `json:"latency_ms"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (s *ModelsService) Predict(ctx context.Context, modelID string, req *PredictRequest) (*PredictResponse, error) {
	var resp PredictResponse
	path := fmt.Sprintf("/api/v1/models/%s/predict", modelID)
	err := s.client.Request(ctx, "POST", path, req, &resp)
	return &resp, err
}

// BillingService handles billing operations
type BillingService struct {
	client *Client
}

type Transaction struct {
	ID        string    `json:"id"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type GetTransactionsResponse struct {
	Transactions []Transaction `json:"transactions"`
	Count        int           `json:"count"`
}

func (s *BillingService) GetTransactions(ctx context.Context) ([]Transaction, error) {
	var resp GetTransactionsResponse
	err := s.client.Request(ctx, "GET", "/api/v1/billing/transactions", nil, &resp)
	return resp.Transactions, err
}

// GuardrailsService handles spending guardrails
type GuardrailsService struct {
	client *Client
}

type SpendingInfo struct {
	WindowSpent float64  `json:"window_spent"`
	WindowLimit float64  `json:"window_limit"`
	WindowName  string   `json:"window_name"`
	Percentage  float64  `json:"percentage_used"`
	Violations  []string `json:"violations"`
}

func (s *GuardrailsService) GetSpending(ctx context.Context) (*SpendingInfo, error) {
	var info SpendingInfo
	err := s.client.Request(ctx, "GET", "/api/v1/guardrails/spending", nil, &info)
	return &info, err
}

type RecordSpendingRequest struct {
	Amount float64 `json:"amount"`
}

func (s *GuardrailsService) RecordSpending(ctx context.Context, amount float64) error {
	req := &RecordSpendingRequest{Amount: amount}
	return s.client.Request(ctx, "POST", "/api/v1/guardrails/spending/record", req, nil)
}

type CheckSpendingRequest struct {
	EstimatedCost float64 `json:"estimated_cost"`
}

type CheckSpendingResponse struct {
	Allowed    bool     `json:"allowed"`
	Spent      float64  `json:"spent"`
	Limit      float64  `json:"limit"`
	Violations []string `json:"violations,omitempty"`
}

func (s *GuardrailsService) CheckSpending(ctx context.Context, estimatedCost float64) (*CheckSpendingResponse, error) {
	req := &CheckSpendingRequest{EstimatedCost: estimatedCost}
	var resp CheckSpendingResponse
	err := s.client.Request(ctx, "POST", "/api/v1/guardrails/spending/check", req, &resp)
	return &resp, err
}

// LoadBalancerService handles load balancing operations
type LoadBalancerService struct {
	client *Client
}

const (
	StrategyRoundRobin         = "round_robin"
	StrategyEqualWeighted      = "equal_weighted"
	StrategyWeightedRoundRobin = "weighted_round_robin"
	StrategyLeastConnections   = "least_connections"
	StrategyLeastResponseTime  = "least_response_time"
)

type GetStrategyResponse struct {
	Strategy string `json:"strategy"`
}

func (s *LoadBalancerService) GetStrategy(ctx context.Context) (string, error) {
	var resp GetStrategyResponse
	err := s.client.Request(ctx, "GET", "/api/v1/loadbalancer/strategy", nil, &resp)
	return resp.Strategy, err
}

type SetStrategyRequest struct {
	Strategy string `json:"strategy"`
}

func (s *LoadBalancerService) SetStrategy(ctx context.Context, strategy string) error {
	req := &SetStrategyRequest{Strategy: strategy}
	return s.client.Request(ctx, "PUT", "/api/v1/loadbalancer/strategy", req, nil)
}

type InstanceLoad struct {
	Connections    int     `json:"connections"`
	Load           float64 `json:"load"`
	ResponseTimeMs float64 `json:"response_time_ms"`
}

type GetLoadsResponse struct {
	Strategy string                  `json:"strategy"`
	Loads    map[string]InstanceLoad `json:"loads"`
	Count    int                     `json:"count"`
}

func (s *LoadBalancerService) GetLoads(ctx context.Context) (*GetLoadsResponse, error) {
	var resp GetLoadsResponse
	err := s.client.Request(ctx, "GET", "/api/v1/loadbalancer/loads", nil, &resp)
	return &resp, err
}

// QuotaService handles storage quota operations
type QuotaService struct {
	client *Client
}

type QuotaInfo struct {
	UserID     string                 `json:"user_id"`
	Storage    StorageQuota           `json:"storage"`
	FileSize   FileQuota              `json:"file_size"`
	RateLimits RateLimitInfo          `json:"rate_limits"`
}

type StorageQuota struct {
	UsedBytes  int64   `json:"used_bytes"`
	LimitBytes int64   `json:"limit_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

type FileQuota struct {
	MaxBytes int64 `json:"max_bytes"`
}

type RateLimitInfo struct {
	UploadsLastHour int `json:"uploads_last_hour"`
	HourlyLimit     int `json:"hourly_limit"`
	UploadsLastDay  int `json:"uploads_last_day"`
	DailyLimit      int `json:"daily_limit"`
}

func (s *QuotaService) Get(ctx context.Context) (*QuotaInfo, error) {
	var info QuotaInfo
	err := s.client.Request(ctx, "GET", "/api/v1/quota", nil, &info)
	return &info, err
}
