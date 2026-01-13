package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AIProxyConfig represents the complete AIProxy configuration
type AIProxyConfig struct {
	Node          NodeConfig          `yaml:"node" json:"node"`
	Server        AIProxyServerConfig `yaml:"server" json:"server"`
	Providers     ProvidersConfig     `yaml:"providers" json:"providers"`
	Routing       RoutingConfig       `yaml:"routing" json:"routing"`
	Budget        BudgetConfig        `yaml:"budget" json:"budget"`
	Observability ObservabilityConfig `yaml:"observability" json:"observability"`
	Security      SecurityConfig      `yaml:"security" json:"security"`
}

// NodeConfig defines node identity and mesh networking
type NodeConfig struct {
	ID     string     `yaml:"id" json:"id"`
	Name   string     `yaml:"name" json:"name"`
	Region string     `yaml:"region" json:"region"`
	Type   string     `yaml:"type" json:"type"` // gpu, edge, cloud, hybrid, standalone
	Mesh   MeshConfig `yaml:"mesh" json:"mesh"`
}

// MeshConfig defines mesh networking settings
type MeshConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	ListenAddr        string        `yaml:"listen_addr" json:"listen_addr"`
	Peers             []PeerConfig  `yaml:"peers" json:"peers"`
	ShareLoad         bool          `yaml:"share_load" json:"share_load"`
	PeerTimeout       time.Duration `yaml:"peer_timeout" json:"peer_timeout"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" json:"heartbeat_interval"`
}

// PeerConfig defines a mesh peer node
type PeerConfig struct {
	ID       string `yaml:"id" json:"id"`
	Addr     string `yaml:"addr" json:"addr"`
	Priority int    `yaml:"priority" json:"priority"`
}

// AIProxyServerConfig defines HTTP server settings specific to AIProxy
type AIProxyServerConfig struct {
	ListenAddr     string          `yaml:"listen_addr" json:"listen_addr"`
	ReadTimeout    time.Duration   `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout   time.Duration   `yaml:"write_timeout" json:"write_timeout"`
	MaxRequestSize string          `yaml:"max_request_size" json:"max_request_size"`
	Endpoints      []EndpointConfig `yaml:"endpoints" json:"endpoints"`
}

// EndpointConfig defines API endpoint configuration
type EndpointConfig struct {
	Path     string `yaml:"path" json:"path"`
	Protocol string `yaml:"protocol" json:"protocol"` // openai, native
	Enabled  bool   `yaml:"enabled" json:"enabled"`
}

// ProvidersConfig defines all provider configurations
type ProvidersConfig struct {
	Local      *LocalProviderConfig      `yaml:"local,omitempty" json:"local,omitempty"`
	Cloudflare *CloudflareProviderConfig `yaml:"cloudflare,omitempty" json:"cloudflare,omitempty"`
	OpenAI     *OpenAIProviderConfig     `yaml:"openai,omitempty" json:"openai,omitempty"`
	Anthropic  *AnthropicProviderConfig  `yaml:"anthropic,omitempty" json:"anthropic,omitempty"`
}

// LocalProviderConfig defines local GPU provider settings
type LocalProviderConfig struct {
	Type     string               `yaml:"type" json:"type"`
	Enabled  bool                 `yaml:"enabled" json:"enabled"`
	Priority int                  `yaml:"priority" json:"priority"`
	Runtimes []string             `yaml:"runtimes" json:"runtimes"`
	Models   []ModelConfig        `yaml:"models" json:"models"`
}

// CloudflareProviderConfig defines Cloudflare Workers AI settings
type CloudflareProviderConfig struct {
	Type        string                 `yaml:"type" json:"type"`
	Enabled     bool                   `yaml:"enabled" json:"enabled"`
	Priority    int                    `yaml:"priority" json:"priority"`
	Credentials CloudflareCredentials  `yaml:"credentials" json:"credentials"`
	Endpoint    string                 `yaml:"endpoint" json:"endpoint"`
	Models      []CloudflareModelConfig `yaml:"models" json:"models"`
}

// CloudflareCredentials stores Cloudflare API credentials
type CloudflareCredentials struct {
	AccountID string `yaml:"account_id" json:"account_id"`
	APIToken  string `yaml:"api_token" json:"api_token"`
}

// CloudflareModelConfig defines a Cloudflare model mapping
type CloudflareModelConfig struct {
	Name             string   `yaml:"name" json:"name"`
	CloudflareModel  string   `yaml:"cloudflare_model" json:"cloudflare_model"`
	Capabilities     []string `yaml:"capabilities" json:"capabilities"`
	CostPer1kTokens  float64  `yaml:"cost_per_1k_tokens,omitempty" json:"cost_per_1k_tokens,omitempty"`
	CostPerGeneration float64 `yaml:"cost_per_generation,omitempty" json:"cost_per_generation,omitempty"`
}

// OpenAIProviderConfig defines OpenAI provider settings
type OpenAIProviderConfig struct {
	Type        string            `yaml:"type" json:"type"`
	Enabled     bool              `yaml:"enabled" json:"enabled"`
	Priority    int               `yaml:"priority" json:"priority"`
	Credentials OpenAICredentials `yaml:"credentials" json:"credentials"`
	Endpoint    string            `yaml:"endpoint" json:"endpoint"`
	Models      []ModelConfig     `yaml:"models" json:"models"`
}

// OpenAICredentials stores OpenAI API credentials
type OpenAICredentials struct {
	APIKey string `yaml:"api_key" json:"api_key"`
}

// AnthropicProviderConfig defines Anthropic provider settings
type AnthropicProviderConfig struct {
	Type        string                `yaml:"type" json:"type"`
	Enabled     bool                  `yaml:"enabled" json:"enabled"`
	Priority    int                   `yaml:"priority" json:"priority"`
	Credentials AnthropicCredentials  `yaml:"credentials" json:"credentials"`
	Endpoint    string                `yaml:"endpoint" json:"endpoint"`
	Models      []ModelConfig         `yaml:"models" json:"models"`
}

// AnthropicCredentials stores Anthropic API credentials
type AnthropicCredentials struct {
	APIKey string `yaml:"api_key" json:"api_key"`
}

// ModelConfig defines a model configuration
type ModelConfig struct {
	Name            string   `yaml:"name" json:"name"`
	Path            string   `yaml:"path,omitempty" json:"path,omitempty"`
	Runtime         string   `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Capabilities    []string `yaml:"capabilities" json:"capabilities"`
	CostPer1kTokens float64  `yaml:"cost_per_1k_tokens" json:"cost_per_1k_tokens"`
	OpenAIModel     string   `yaml:"openai_model,omitempty" json:"openai_model,omitempty"`
	AnthropicModel  string   `yaml:"anthropic_model,omitempty" json:"anthropic_model,omitempty"`
}

// RoutingConfig defines routing behavior
type RoutingConfig struct {
	Strategy       string            `yaml:"strategy" json:"strategy"`
	Policies       []RoutingPolicy   `yaml:"policies" json:"policies"`
	Failover       FailoverConfig    `yaml:"failover" json:"failover"`
	LoadBalancing  LoadBalancingConfig `yaml:"load_balancing" json:"load_balancing"`
}

// RoutingPolicy defines a routing policy
type RoutingPolicy struct {
	Name  string        `yaml:"name" json:"name"`
	Type  string        `yaml:"type" json:"type"`
	Rules []RoutingRule `yaml:"rules" json:"rules"`
}

// RoutingRule defines a routing rule
type RoutingRule struct {
	If   string `yaml:"if,omitempty" json:"if,omitempty"`
	Then string `yaml:"then" json:"then"`
	Else string `yaml:"else,omitempty" json:"else,omitempty"`
}

// FailoverConfig defines failover behavior
type FailoverConfig struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`
	MaxRetries    int           `yaml:"max_retries" json:"max_retries"`
	RetryDelay    time.Duration `yaml:"retry_delay" json:"retry_delay"`
	FallbackChain []string      `yaml:"fallback_chain" json:"fallback_chain"`
}

// LoadBalancingConfig defines load balancing behavior
type LoadBalancingConfig struct {
	Enabled            bool    `yaml:"enabled" json:"enabled"`
	Strategy           string  `yaml:"strategy" json:"strategy"` // round_robin, least_loaded, weighted
	HealthAware        bool    `yaml:"health_aware" json:"health_aware"`
	UnhealthyThreshold float64 `yaml:"unhealthy_threshold" json:"unhealthy_threshold"`
}

// BudgetConfig defines budget and cost controls
type BudgetConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	DailyLimit   float64       `yaml:"daily_limit" json:"daily_limit"`
	MonthlyLimit float64       `yaml:"monthly_limit" json:"monthly_limit"`
	TrackCosts   bool          `yaml:"track_costs" json:"track_costs"`
	CostDB       string        `yaml:"cost_db" json:"cost_db"`
	Alerts       []BudgetAlert `yaml:"alerts" json:"alerts"`
}

// BudgetAlert defines a budget alert
type BudgetAlert struct {
	Threshold int    `yaml:"threshold" json:"threshold"` // percentage
	Action    string `yaml:"action" json:"action"`
}

// ObservabilityConfig defines observability settings
type ObservabilityConfig struct {
	Logging LoggingConfig `yaml:"logging" json:"logging"`
	Metrics MetricsConfig `yaml:"metrics" json:"metrics"`
	Tracing TracingConfig `yaml:"tracing" json:"tracing"`
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	Output string `yaml:"output" json:"output"`
}

// MetricsConfig defines metrics collection
type MetricsConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	PrometheusPort int      `yaml:"prometheus_port" json:"prometheus_port"`
	Collect        []string `yaml:"collect" json:"collect"`
}

// TracingConfig defines tracing settings
type TracingConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Provider string `yaml:"provider" json:"provider"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
}

// SecurityConfig defines security settings
type SecurityConfig struct {
	Auth         AuthConfig         `yaml:"auth" json:"auth"`
	RateLimiting RateLimitingConfig `yaml:"rate_limiting" json:"rate_limiting"`
	CORS         CORSConfig         `yaml:"cors" json:"cors"`
}

// AuthConfig defines authentication settings
type AuthConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Type    string     `yaml:"type" json:"type"` // api_key, jwt, oauth2
	APIKeys []APIKey   `yaml:"api_keys" json:"api_keys"`
}

// APIKey defines an API key configuration
type APIKey struct {
	Key       string `yaml:"key" json:"key"`
	Name      string `yaml:"name" json:"name"`
	RateLimit int    `yaml:"rate_limit" json:"rate_limit"`
}

// RateLimitingConfig defines rate limiting settings
type RateLimitingConfig struct {
	Enabled      bool `yaml:"enabled" json:"enabled"`
	DefaultLimit int  `yaml:"default_limit" json:"default_limit"`
	ByIP         bool `yaml:"by_ip" json:"by_ip"`
	ByAPIKey     bool `yaml:"by_api_key" json:"by_api_key"`
}

// CORSConfig defines CORS settings
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
}

// LoadAIProxyConfig loads AIProxy configuration from a YAML or JSON file
func LoadAIProxyConfig(path string) (*AIProxyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	config := &AIProxyConfig{}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal([]byte(expanded), config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal([]byte(expanded), config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format: %s (use .yaml or .json)", ext)
	}

	// Set defaults
	config.setDefaults()

	// Validate
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// setDefaults sets default values for optional fields
func (c *AIProxyConfig) setDefaults() {
	// Node defaults
	if c.Node.Type == "" {
		c.Node.Type = "standalone"
	}

	// Server defaults
	if c.Server.ListenAddr == "" {
		c.Server.ListenAddr = "0.0.0.0:8080"
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 120 * time.Second
	}

	// Default endpoints if none specified
	if len(c.Server.Endpoints) == 0 {
		c.Server.Endpoints = []EndpointConfig{
			{Path: "/v1/chat/completions", Protocol: "openai", Enabled: true},
			{Path: "/v1/embeddings", Protocol: "openai", Enabled: true},
			{Path: "/v1/models", Protocol: "openai", Enabled: true},
			{Path: "/aiproxy/predict", Protocol: "native", Enabled: true},
		}
	}

	// Routing defaults
	if c.Routing.Strategy == "" {
		c.Routing.Strategy = "cost_optimized"
	}
	if c.Routing.Failover.RetryDelay == 0 {
		c.Routing.Failover.RetryDelay = 1 * time.Second
	}
	if c.Routing.Failover.MaxRetries == 0 {
		c.Routing.Failover.MaxRetries = 3
	}

	// Observability defaults
	if c.Observability.Logging.Level == "" {
		c.Observability.Logging.Level = "info"
	}
	if c.Observability.Logging.Format == "" {
		c.Observability.Logging.Format = "json"
	}
	if c.Observability.Logging.Output == "" {
		c.Observability.Logging.Output = "stdout"
	}

	// Cloudflare endpoint default
	if c.Providers.Cloudflare != nil && c.Providers.Cloudflare.Endpoint == "" {
		c.Providers.Cloudflare.Endpoint = "https://api.cloudflare.com/client/v4"
	}

	// OpenAI endpoint default
	if c.Providers.OpenAI != nil && c.Providers.OpenAI.Endpoint == "" {
		c.Providers.OpenAI.Endpoint = "https://api.openai.com/v1"
	}

	// Anthropic endpoint default
	if c.Providers.Anthropic != nil && c.Providers.Anthropic.Endpoint == "" {
		c.Providers.Anthropic.Endpoint = "https://api.anthropic.com/v1"
	}
}

// Validate validates the configuration
func (c *AIProxyConfig) Validate() error {
	// Validate node
	if c.Node.ID == "" {
		return fmt.Errorf("node.id is required")
	}

	// Validate at least one provider is enabled
	hasProvider := false
	if c.Providers.Local != nil && c.Providers.Local.Enabled {
		hasProvider = true
	}
	if c.Providers.Cloudflare != nil && c.Providers.Cloudflare.Enabled {
		hasProvider = true
		// Validate Cloudflare credentials
		if c.Providers.Cloudflare.Credentials.AccountID == "" {
			return fmt.Errorf("cloudflare.credentials.account_id is required when cloudflare provider is enabled")
		}
		if c.Providers.Cloudflare.Credentials.APIToken == "" {
			return fmt.Errorf("cloudflare.credentials.api_token is required when cloudflare provider is enabled")
		}
	}
	if c.Providers.OpenAI != nil && c.Providers.OpenAI.Enabled {
		hasProvider = true
		if c.Providers.OpenAI.Credentials.APIKey == "" {
			return fmt.Errorf("openai.credentials.api_key is required when openai provider is enabled")
		}
	}
	if c.Providers.Anthropic != nil && c.Providers.Anthropic.Enabled {
		hasProvider = true
		if c.Providers.Anthropic.Credentials.APIKey == "" {
			return fmt.Errorf("anthropic.credentials.api_key is required when anthropic provider is enabled")
		}
	}

	if !hasProvider {
		return fmt.Errorf("at least one provider must be enabled")
	}

	return nil
}

// GetEnabledProviders returns a list of enabled provider names
func (c *AIProxyConfig) GetEnabledProviders() []string {
	providers := []string{}
	if c.Providers.Local != nil && c.Providers.Local.Enabled {
		providers = append(providers, "local")
	}
	if c.Providers.Cloudflare != nil && c.Providers.Cloudflare.Enabled {
		providers = append(providers, "cloudflare")
	}
	if c.Providers.OpenAI != nil && c.Providers.OpenAI.Enabled {
		providers = append(providers, "openai")
	}
	if c.Providers.Anthropic != nil && c.Providers.Anthropic.Enabled {
		providers = append(providers, "anthropic")
	}
	return providers
}
