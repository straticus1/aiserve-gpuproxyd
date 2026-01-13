package compute

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ComputeProvider represents different compute providers
type ComputeProvider string

const (
	ProviderVastAI     ComputeProvider = "vastai"
	ProviderIONet      ComputeProvider = "ionet"
	ProviderOpenRouter ComputeProvider = "openrouter"
	ProviderLocal      ComputeProvider = "local"
)

// ComputeType represents the type of compute resource
type ComputeType string

const (
	ComputeGPU ComputeType = "gpu"
	ComputeTPU ComputeType = "tpu"
	ComputeCPU ComputeType = "cpu"
)

// ReservationRequest represents a request for compute resources
type ReservationRequest struct {
	Provider     ComputeProvider `json:"provider"`
	ComputeType  ComputeType     `json:"compute_type"`
	GPUModel     string          `json:"gpu_model,omitempty"`     // e.g., "H100", "A100"
	TPUVersion   string          `json:"tpu_version,omitempty"`   // e.g., "v4", "v5e"
	Count        int             `json:"count"`                   // Number of resources
	Duration     time.Duration   `json:"duration"`                // How long to reserve
	Region       string          `json:"region,omitempty"`        // Preferred region
	MinVRAM      int             `json:"min_vram,omitempty"`      // Minimum VRAM in GB
	MaxCostPerHr float64         `json:"max_cost_per_hr"`         // Maximum cost per hour
	Priority     int             `json:"priority"`                // 1-10, higher = more important
	Labels       map[string]string `json:"labels,omitempty"`      // Custom labels

	// Hybrid mode options
	EnableHybrid     bool              `json:"enable_hybrid"`      // Use multiple providers
	FallbackProviders []ComputeProvider `json:"fallback_providers"` // Fallback order
}

// Reservation represents a reserved compute resource
type Reservation struct {
	ID           string          `json:"id"`
	Provider     ComputeProvider `json:"provider"`
	ComputeType  ComputeType     `json:"compute_type"`
	InstanceID   string          `json:"instance_id"`    // Provider's instance ID
	Status       string          `json:"status"`         // pending, active, terminated
	StartTime    time.Time       `json:"start_time"`
	EndTime      time.Time       `json:"end_time"`
	CostPerHr    float64         `json:"cost_per_hr"`
	TotalCost    float64         `json:"total_cost"`

	// Connection info
	Endpoint     string          `json:"endpoint"`       // IP:Port or URL
	Port         int             `json:"port"`           // Assigned port (2000-15000)
	Protocol     string          `json:"protocol"`       // http, grpc, custom

	// Metadata
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// ReservationClient manages compute reservations across providers
type ReservationClient struct {
	mu sync.RWMutex

	// Provider clients
	vastAIClient     *VastAIClient
	ionetClient      *IONetClient
	openRouterClient *OpenRouterClient

	// Resource tracking
	reservations     map[string]*Reservation  // reservation_id -> Reservation
	activeGPUs       int                      // Current active GPUs
	activeTPUs       int                      // Current active TPUs

	// Limits
	maxGPUs          int                      // Max concurrent GPUs (1000 default)
	maxTPUs          int                      // Max concurrent TPUs (200 default)
	maxVastAIGPUs    int                      // Max Vast.ai GPUs (500 default)
	maxIONetGPUs     int                      // Max IO.net GPUs (500 default)

	// Port management (2000-15000)
	portAllocator    *PortAllocator
}

// NewReservationClient creates a new reservation client
func NewReservationClient(vastAIKey, ionetKey, openRouterKey string) *ReservationClient {
	return &ReservationClient{
		vastAIClient:     NewVastAIClient(vastAIKey),
		ionetClient:      NewIONetClient(ionetKey),
		openRouterClient: NewOpenRouterClient(openRouterKey),
		reservations:     make(map[string]*Reservation),
		maxGPUs:          1000,
		maxTPUs:          200,
		maxVastAIGPUs:    500,
		maxIONetGPUs:     500,
		portAllocator:    NewPortAllocator(2000, 15000),
	}
}

// Reserve requests compute resources with intelligent provider selection
func (c *ReservationClient) Reserve(ctx context.Context, req *ReservationRequest) (*Reservation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check capacity limits
	if req.ComputeType == ComputeGPU && c.activeGPUs+req.Count > c.maxGPUs {
		return nil, fmt.Errorf("GPU capacity exceeded: current=%d, requested=%d, max=%d",
			c.activeGPUs, req.Count, c.maxGPUs)
	}
	if req.ComputeType == ComputeTPU && c.activeTPUs+req.Count > c.maxTPUs {
		return nil, fmt.Errorf("TPU capacity exceeded: current=%d, requested=%d, max=%d",
			c.activeTPUs, req.Count, c.maxTPUs)
	}

	// Select provider based on request
	var reservation *Reservation
	var err error

	switch req.Provider {
	case ProviderVastAI:
		reservation, err = c.reserveVastAI(ctx, req)
	case ProviderIONet:
		reservation, err = c.reserveIONet(ctx, req)
	case ProviderOpenRouter:
		reservation, err = c.reserveOpenRouter(ctx, req)
	case ProviderLocal:
		reservation, err = c.reserveLocal(ctx, req)
	default:
		// Auto-select best provider
		reservation, err = c.autoSelectProvider(ctx, req)
	}

	if err != nil {
		// Try fallback providers if hybrid mode enabled
		if req.EnableHybrid && len(req.FallbackProviders) > 0 {
			for _, fallbackProvider := range req.FallbackProviders {
				req.Provider = fallbackProvider
				reservation, err = c.Reserve(ctx, req)
				if err == nil {
					break
				}
			}
		}

		if err != nil {
			return nil, fmt.Errorf("failed to reserve compute: %w", err)
		}
	}

	// Allocate port
	port, err := c.portAllocator.Allocate()
	if err != nil {
		// Cleanup reservation
		c.Release(ctx, reservation.ID)
		return nil, fmt.Errorf("failed to allocate port: %w", err)
	}
	reservation.Port = port

	// Track reservation
	c.reservations[reservation.ID] = reservation
	if req.ComputeType == ComputeGPU {
		c.activeGPUs += req.Count
	} else if req.ComputeType == ComputeTPU {
		c.activeTPUs += req.Count
	}

	return reservation, nil
}

// Release terminates a reservation
func (c *ReservationClient) Release(ctx context.Context, reservationID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	reservation, exists := c.reservations[reservationID]
	if !exists {
		return fmt.Errorf("reservation not found: %s", reservationID)
	}

	// Release with appropriate provider
	var err error
	switch reservation.Provider {
	case ProviderVastAI:
		err = c.vastAIClient.DestroyInstance(ctx, reservation.InstanceID)
	case ProviderIONet:
		err = c.ionetClient.TerminateInstance(ctx, reservation.InstanceID)
	case ProviderOpenRouter:
		// OpenRouter doesn't need instance cleanup
		err = nil
	case ProviderLocal:
		err = c.releaseLocal(ctx, reservation.InstanceID)
	}

	if err != nil {
		return fmt.Errorf("failed to release reservation: %w", err)
	}

	// Free port
	c.portAllocator.Free(reservation.Port)

	// Update tracking
	if reservation.ComputeType == ComputeGPU {
		c.activeGPUs--
	} else if reservation.ComputeType == ComputeTPU {
		c.activeTPUs--
	}

	reservation.Status = "terminated"
	reservation.EndTime = time.Now()

	return nil
}

// autoSelectProvider intelligently selects the best provider
func (c *ReservationClient) autoSelectProvider(ctx context.Context, req *ReservationRequest) (*Reservation, error) {
	// For OpenRouter models, always use OpenRouter
	if req.Labels != nil && req.Labels["model_type"] == "openrouter" {
		return c.reserveOpenRouter(ctx, req)
	}

	// For custom models, prefer local if available, else Vast.ai
	if req.Labels != nil && req.Labels["model_type"] == "custom" {
		// Try local first
		if reservation, err := c.reserveLocal(ctx, req); err == nil {
			return reservation, nil
		}
		// Fall back to Vast.ai
		return c.reserveVastAI(ctx, req)
	}

	// For general GPU requests, balance between Vast.ai and IO.net
	vastAILoad := float64(c.activeGPUs) / float64(c.maxVastAIGPUs)
	ionetLoad := float64(c.activeGPUs) / float64(c.maxIONetGPUs)

	if vastAILoad < ionetLoad {
		return c.reserveVastAI(ctx, req)
	}
	return c.reserveIONet(ctx, req)
}

// reserveVastAI reserves compute from Vast.ai
func (c *ReservationClient) reserveVastAI(ctx context.Context, req *ReservationRequest) (*Reservation, error) {
	// TODO: Implement Vast.ai specific reservation logic
	return nil, fmt.Errorf("Vast.ai reservation not yet implemented")
}

// reserveIONet reserves compute from IO.net
func (c *ReservationClient) reserveIONet(ctx context.Context, req *ReservationRequest) (*Reservation, error) {
	// TODO: Implement IO.net specific reservation logic
	return nil, fmt.Errorf("IO.net reservation not yet implemented")
}

// reserveOpenRouter reserves compute from OpenRouter
func (c *ReservationClient) reserveOpenRouter(ctx context.Context, req *ReservationRequest) (*Reservation, error) {
	// OpenRouter doesn't provision instances, just provides model access
	instanceID := fmt.Sprintf("openrouter-%d", time.Now().Unix())

	reservation := &Reservation{
		ID:          fmt.Sprintf("res-%d", time.Now().UnixNano()),
		Provider:    ProviderOpenRouter,
		ComputeType: ComputeGPU, // OpenRouter uses GPUs behind the scenes
		InstanceID:  instanceID,
		Status:      "active",
		StartTime:   time.Now(),
		Endpoint:    "https://openrouter.ai/api/v1",
		Protocol:    "http",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	return reservation, nil
}

// reserveLocal reserves local compute resources
func (c *ReservationClient) reserveLocal(ctx context.Context, req *ReservationRequest) (*Reservation, error) {
	// TODO: Implement local GPU/TPU reservation
	return nil, fmt.Errorf("local reservation not yet implemented")
}

// releaseLocal releases local compute resources
func (c *ReservationClient) releaseLocal(ctx context.Context, instanceID string) error {
	// TODO: Implement local resource cleanup
	return nil
}

// GetReservation retrieves a reservation by ID
func (c *ReservationClient) GetReservation(reservationID string) (*Reservation, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	reservation, exists := c.reservations[reservationID]
	if !exists {
		return nil, fmt.Errorf("reservation not found: %s", reservationID)
	}

	return reservation, nil
}

// ListReservations returns all active reservations
func (c *ReservationClient) ListReservations() []*Reservation {
	c.mu.RLock()
	defer c.mu.RUnlock()

	reservations := make([]*Reservation, 0, len(c.reservations))
	for _, res := range c.reservations {
		if res.Status == "active" {
			reservations = append(reservations, res)
		}
	}

	return reservations
}

// GetCapacity returns current capacity usage
func (c *ReservationClient) GetCapacity() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"gpus": map[string]int{
			"active":  c.activeGPUs,
			"max":     c.maxGPUs,
			"available": c.maxGPUs - c.activeGPUs,
		},
		"tpus": map[string]int{
			"active":  c.activeTPUs,
			"max":     c.maxTPUs,
			"available": c.maxTPUs - c.activeTPUs,
		},
		"providers": map[string]interface{}{
			"vastai": map[string]int{
				"max": c.maxVastAIGPUs,
			},
			"ionet": map[string]int{
				"max": c.maxIONetGPUs,
			},
		},
	}
}
