package router

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"aiserve-gpuproxyd/internal/config"
	"aiserve-gpuproxyd/internal/providers"
)

// Router handles intelligent routing of AI workloads across providers
type Router struct {
	config    *config.AIProxyConfig
	providers map[string]providers.Provider
	stats     *RouterStats
	mu        sync.RWMutex
}

// RouterStats tracks routing statistics
type RouterStats struct {
	TotalRequests    int64
	ProviderRequests map[string]int64
	ProviderErrors   map[string]int64
	ProviderLatency  map[string][]int
	TotalCost        float64
	mu               sync.RWMutex
}

// RoutingDecision represents the result of routing logic
type RoutingDecision struct {
	Provider     string
	Model        string
	Reason       string
	Alternatives []string
	EstimatedCost float64
}

// NewRouter creates a new router with configured providers
func NewRouter(cfg *config.AIProxyConfig) (*Router, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	r := &Router{
		config:    cfg,
		providers: make(map[string]providers.Provider),
		stats: &RouterStats{
			ProviderRequests: make(map[string]int64),
			ProviderErrors:   make(map[string]int64),
			ProviderLatency:  make(map[string][]int),
		},
	}

	// Initialize providers
	if err := r.initializeProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	return r, nil
}

// initializeProviders initializes all configured providers
func (r *Router) initializeProviders() error {
	// Initialize Cloudflare provider
	if r.config.Providers.Cloudflare != nil && r.config.Providers.Cloudflare.Enabled {
		cfProvider, err := providers.NewCloudflareProvider(r.config.Providers.Cloudflare)
		if err != nil {
			return fmt.Errorf("failed to initialize cloudflare provider: %w", err)
		}
		r.providers["cloudflare"] = cfProvider
	}

	// TODO: Initialize other providers (Local, OpenAI, Anthropic)

	if len(r.providers) == 0 {
		return fmt.Errorf("no providers initialized")
	}

	return nil
}

// Route selects the best provider for a request
func (r *Router) Route(ctx context.Context, req *providers.PredictRequest) (*RoutingDecision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	strategy := r.config.Routing.Strategy

	switch strategy {
	case "cost_optimized":
		return r.routeByCost(ctx, req)
	case "latency_optimized":
		return r.routeByLatency(ctx, req)
	case "availability":
		return r.routeByAvailability(ctx, req)
	case "round_robin":
		return r.routeRoundRobin(ctx, req)
	default:
		return r.routeByCost(ctx, req)
	}
}

// routeByCost selects the provider with lowest cost
func (r *Router) routeByCost(ctx context.Context, req *providers.PredictRequest) (*RoutingDecision, error) {
	type providerCost struct {
		name     string
		provider providers.Provider
		cost     float64
		priority int
	}

	candidates := []providerCost{}

	// Estimate tokens for cost calculation
	estimatedTokens := estimateRequestTokens(req)

	// Collect candidates
	for name, provider := range r.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}

		// Check if provider has the model
		models := provider.GetModels()
		hasModel := false
		for _, m := range models {
			if m == req.Model {
				hasModel = true
				break
			}
		}
		if !hasModel {
			continue
		}

		cost := provider.GetCostPer1kTokens(req.Model) * float64(estimatedTokens) / 1000.0
		candidates = append(candidates, providerCost{
			name:     name,
			provider: provider,
			cost:     cost,
			priority: provider.Priority(),
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available providers for model %s", req.Model)
	}

	// Sort by cost, then by priority
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].cost != candidates[j].cost {
			return candidates[i].cost < candidates[j].cost
		}
		return candidates[i].priority < candidates[j].priority
	})

	// Select best candidate
	best := candidates[0]
	alternatives := make([]string, 0, len(candidates)-1)
	for i := 1; i < len(candidates); i++ {
		alternatives = append(alternatives, candidates[i].name)
	}

	return &RoutingDecision{
		Provider:      best.name,
		Model:         req.Model,
		Reason:        fmt.Sprintf("lowest_cost (estimated: $%.6f)", best.cost),
		Alternatives:  alternatives,
		EstimatedCost: best.cost,
	}, nil
}

// routeByLatency selects the provider with lowest latency
func (r *Router) routeByLatency(ctx context.Context, req *providers.PredictRequest) (*RoutingDecision, error) {
	type providerLatency struct {
		name         string
		provider     providers.Provider
		avgLatency   float64
		priority     int
	}

	r.stats.mu.RLock()
	defer r.stats.mu.RUnlock()

	candidates := []providerLatency{}

	// Collect candidates
	for name, provider := range r.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}

		// Check if provider has the model
		models := provider.GetModels()
		hasModel := false
		for _, m := range models {
			if m == req.Model {
				hasModel = true
				break
			}
		}
		if !hasModel {
			continue
		}

		// Calculate average latency
		avgLatency := 1000.0 // Default: 1 second
		if latencies, ok := r.stats.ProviderLatency[name]; ok && len(latencies) > 0 {
			sum := 0
			for _, l := range latencies {
				sum += l
			}
			avgLatency = float64(sum) / float64(len(latencies))
		}

		candidates = append(candidates, providerLatency{
			name:       name,
			provider:   provider,
			avgLatency: avgLatency,
			priority:   provider.Priority(),
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available providers for model %s", req.Model)
	}

	// Sort by latency, then by priority
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].avgLatency != candidates[j].avgLatency {
			return candidates[i].avgLatency < candidates[j].avgLatency
		}
		return candidates[i].priority < candidates[j].priority
	})

	// Select best candidate
	best := candidates[0]
	alternatives := make([]string, 0, len(candidates)-1)
	for i := 1; i < len(candidates); i++ {
		alternatives = append(alternatives, candidates[i].name)
	}

	return &RoutingDecision{
		Provider:     best.name,
		Model:        req.Model,
		Reason:       fmt.Sprintf("lowest_latency (avg: %.0fms)", best.avgLatency),
		Alternatives: alternatives,
	}, nil
}

// routeByAvailability selects first available provider
func (r *Router) routeByAvailability(ctx context.Context, req *providers.PredictRequest) (*RoutingDecision, error) {
	type providerPrio struct {
		name     string
		provider providers.Provider
		priority int
	}

	candidates := []providerPrio{}

	// Collect available candidates
	for name, provider := range r.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}

		// Check if provider has the model
		models := provider.GetModels()
		hasModel := false
		for _, m := range models {
			if m == req.Model {
				hasModel = true
				break
			}
		}
		if !hasModel {
			continue
		}

		candidates = append(candidates, providerPrio{
			name:     name,
			provider: provider,
			priority: provider.Priority(),
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available providers for model %s", req.Model)
	}

	// Sort by priority
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].priority < candidates[j].priority
	})

	best := candidates[0]
	alternatives := make([]string, 0, len(candidates)-1)
	for i := 1; i < len(candidates); i++ {
		alternatives = append(alternatives, candidates[i].name)
	}

	return &RoutingDecision{
		Provider:     best.name,
		Model:        req.Model,
		Reason:       "highest_priority_available",
		Alternatives: alternatives,
	}, nil
}

// routeRoundRobin implements round-robin routing
func (r *Router) routeRoundRobin(ctx context.Context, req *providers.PredictRequest) (*RoutingDecision, error) {
	// Simple round-robin based on request count
	r.stats.mu.RLock()
	defer r.stats.mu.RUnlock()

	var candidates []string
	for name, provider := range r.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}

		models := provider.GetModels()
		hasModel := false
		for _, m := range models {
			if m == req.Model {
				hasModel = true
				break
			}
		}
		if hasModel {
			candidates = append(candidates, name)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available providers for model %s", req.Model)
	}

	// Sort for deterministic ordering
	sort.Strings(candidates)

	// Select based on total request count
	index := int(r.stats.TotalRequests) % len(candidates)
	selected := candidates[index]

	alternatives := make([]string, 0, len(candidates)-1)
	for _, c := range candidates {
		if c != selected {
			alternatives = append(alternatives, c)
		}
	}

	return &RoutingDecision{
		Provider:     selected,
		Model:        req.Model,
		Reason:       "round_robin",
		Alternatives: alternatives,
	}, nil
}

// Predict routes and executes a prediction request
func (r *Router) Predict(ctx context.Context, req *providers.PredictRequest) (*providers.PredictResponse, *RoutingDecision, error) {
	// Route the request
	decision, err := r.Route(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("routing failed: %w", err)
	}

	// Get the provider
	provider, ok := r.providers[decision.Provider]
	if !ok {
		return nil, decision, fmt.Errorf("provider %s not found", decision.Provider)
	}

	// Execute with failover
	var lastErr error
	fallbackChain := r.config.Routing.Failover.FallbackChain
	if !r.config.Routing.Failover.Enabled || len(fallbackChain) == 0 {
		fallbackChain = []string{decision.Provider}
	}

	for attempt := 0; attempt < r.config.Routing.Failover.MaxRetries; attempt++ {
		for _, providerName := range fallbackChain {
			provider, ok := r.providers[providerName]
			if !ok || !provider.IsAvailable(ctx) {
				continue
			}

			// Track request
			r.recordRequest(providerName)

			// Execute request
			startTime := time.Now()
			resp, err := provider.Predict(ctx, req)
			latency := time.Since(startTime)

			if err != nil {
				lastErr = err
				r.recordError(providerName)
				continue
			}

			// Record success
			r.recordLatency(providerName, int(latency.Milliseconds()))
			r.recordCost(resp.Metadata.Cost)

			return resp, decision, nil
		}

		// Wait before retry
		if attempt < r.config.Routing.Failover.MaxRetries-1 {
			time.Sleep(r.config.Routing.Failover.RetryDelay)
		}
	}

	return nil, decision, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// GetProvider returns a provider by name
func (r *Router) GetProvider(name string) (providers.Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// GetProviders returns all providers
func (r *Router) GetProviders() map[string]providers.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers
}

// GetStats returns routing statistics
func (r *Router) GetStats() *RouterStats {
	r.stats.mu.RLock()
	defer r.stats.mu.RUnlock()

	// Create a copy
	stats := &RouterStats{
		TotalRequests:    r.stats.TotalRequests,
		TotalCost:        r.stats.TotalCost,
		ProviderRequests: make(map[string]int64),
		ProviderErrors:   make(map[string]int64),
		ProviderLatency:  make(map[string][]int),
	}

	for k, v := range r.stats.ProviderRequests {
		stats.ProviderRequests[k] = v
	}
	for k, v := range r.stats.ProviderErrors {
		stats.ProviderErrors[k] = v
	}
	for k, v := range r.stats.ProviderLatency {
		stats.ProviderLatency[k] = append([]int{}, v...)
	}

	return stats
}

// recordRequest increments request counters
func (r *Router) recordRequest(provider string) {
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()
	r.stats.TotalRequests++
	r.stats.ProviderRequests[provider]++
}

// recordError increments error counter
func (r *Router) recordError(provider string) {
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()
	r.stats.ProviderErrors[provider]++
}

// recordLatency records latency measurement
func (r *Router) recordLatency(provider string, latencyMs int) {
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()

	latencies := r.stats.ProviderLatency[provider]
	latencies = append(latencies, latencyMs)

	// Keep only last 100 measurements
	if len(latencies) > 100 {
		latencies = latencies[len(latencies)-100:]
	}

	r.stats.ProviderLatency[provider] = latencies
}

// recordCost adds to total cost
func (r *Router) recordCost(cost float64) {
	r.stats.mu.Lock()
	defer r.stats.mu.Unlock()
	r.stats.TotalCost += cost
}

// estimateRequestTokens estimates the number of tokens in a request
func estimateRequestTokens(req *providers.PredictRequest) int {
	// Default estimate
	tokens := 100

	// Add max_tokens if specified
	if req.MaxTokens > 0 {
		tokens += req.MaxTokens
	} else {
		tokens += 500 // Default response size estimate
	}

	// Try to estimate input size
	switch input := req.Input.(type) {
	case string:
		tokens += len(input) / 4 // Rough estimate: 1 token ≈ 4 chars
	case []interface{}:
		for _, item := range input {
			if msg, ok := item.(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					tokens += len(content) / 4
				}
			}
		}
	}

	return tokens
}
