package common

import "time"

// ProviderType categorizes cloud providers
type ProviderType string

const (
	// Budget CSPs (Cost-optimized, spot instances)
	ProviderBudgetVastAI ProviderType = "vastai"
	ProviderBudgetIONet  ProviderType = "ionet"

	// Major CSPs (Enterprise-grade, SLA guarantees)
	ProviderMajorAWS    ProviderType = "aws"
	ProviderMajorOracle ProviderType = "oracle"
	ProviderMajorGCP    ProviderType = "gcp"
	ProviderMajorAzure  ProviderType = "azure"
)

// GPUInstance represents a GPU compute instance across any provider
type GPUInstance struct {
	// Universal fields
	ID           string       `json:"id"`
	Provider     ProviderType `json:"provider"`
	Status       string       `json:"status"`  // "running", "stopped", "terminated"

	// Hardware specs
	GPUModel     string       `json:"gpu_model"`     // "H100", "A100", "V100"
	GPUCount     int          `json:"gpu_count"`
	VRAM         int          `json:"vram_gb"`
	CPUCores     int          `json:"cpu_cores"`
	RAM          int          `json:"ram_gb"`
	Disk         int          `json:"disk_gb"`

	// Network
	PublicIP     string       `json:"public_ip"`
	PrivateIP    string       `json:"private_ip"`
	SSHPort      int          `json:"ssh_port"`
	SSHKey       string       `json:"ssh_key"`

	// Location
	Region       string       `json:"region"`
	Datacenter   string       `json:"datacenter"`

	// Pricing
	CostPerHour  float64      `json:"cost_per_hour"`
	TotalCost    float64      `json:"total_cost"`

	// Timing
	CreatedAt    time.Time    `json:"created_at"`
	StartedAt    time.Time    `json:"started_at"`
	TerminatedAt *time.Time   `json:"terminated_at,omitempty"`

	// Provider-specific data
	ProviderData map[string]interface{} `json:"provider_data,omitempty"`
}

// ReservationRequest represents a request to reserve GPU compute
type ReservationRequest struct {
	// Provider selection
	Provider         ProviderType `json:"provider"`
	PreferBudget     bool         `json:"prefer_budget"`      // Prefer budget CSPs
	AllowFallback    bool         `json:"allow_fallback"`     // Fallback to other providers

	// Hardware requirements
	GPUModel         string       `json:"gpu_model"`          // "H100", "A100", "any"
	GPUCount         int          `json:"gpu_count"`
	MinVRAM          int          `json:"min_vram_gb"`
	MinCPUCores      int          `json:"min_cpu_cores"`
	MinRAM           int          `json:"min_ram_gb"`
	MinDisk          int          `json:"min_disk_gb"`

	// Location preferences
	PreferredRegion  string       `json:"preferred_region"`   // "us-east-1", "eu-west-1"
	RequireRegion    bool         `json:"require_region"`     // Strict region requirement

	// Budget constraints
	MaxCostPerHour   float64      `json:"max_cost_per_hour"`
	MaxTotalCost     float64      `json:"max_total_cost"`

	// Duration
	Duration         time.Duration `json:"duration"`           // How long to reserve
	SpotInstance     bool         `json:"spot_instance"`      // Use spot/preemptible

	// Image/Container
	Image            string       `json:"image"`              // Docker image or AMI
	Env              map[string]string `json:"env"`           // Environment variables

	// Metadata
	Labels           map[string]string `json:"labels"`
}

// ListOptions provides filters for listing instances
type ListOptions struct {
	Provider     ProviderType `json:"provider,omitempty"`
	Status       string       `json:"status,omitempty"`
	GPUModel     string       `json:"gpu_model,omitempty"`
	Region       string       `json:"region,omitempty"`
	MinCostPerHr float64      `json:"min_cost_per_hour,omitempty"`
	MaxCostPerHr float64      `json:"max_cost_per_hour,omitempty"`
	Limit        int          `json:"limit,omitempty"`
	Offset       int          `json:"offset,omitempty"`
}

// ProviderStats represents statistics for a provider
type ProviderStats struct {
	Provider         ProviderType `json:"provider"`
	TotalInstances   int          `json:"total_instances"`
	RunningInstances int          `json:"running_instances"`
	TotalCost        float64      `json:"total_cost"`
	AvgCostPerHour   float64      `json:"avg_cost_per_hour"`
	TotalVRAM        int          `json:"total_vram_gb"`
	Uptime           time.Duration `json:"uptime"`
}

// ProviderCapabilities describes what a provider can do
type ProviderCapabilities struct {
	Provider         ProviderType `json:"provider"`
	Type             string       `json:"type"`          // "budget" or "major"

	// Features
	SupportsSpot     bool         `json:"supports_spot"`
	SupportsReserved bool         `json:"supports_reserved"`
	SupportsSLA      bool         `json:"supports_sla"`
	SupportsGPUDirect bool        `json:"supports_gpu_direct"`

	// Available GPUs
	AvailableGPUs    []string     `json:"available_gpus"`

	// Regions
	Regions          []string     `json:"regions"`

	// Pricing
	MinCostPerHour   float64      `json:"min_cost_per_hour"`
	AvgCostPerHour   float64      `json:"avg_cost_per_hour"`

	// Limits
	MaxGPUsPerInstance int        `json:"max_gpus_per_instance"`
	MaxInstances     int          `json:"max_instances"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}
