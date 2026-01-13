package models

import (
	"time"

	"github.com/google/uuid"
)

// Dataset represents user training data
type Dataset struct {
	ID               uuid.UUID `json:"id" db:"id"`
	UserID           uuid.UUID `json:"user_id" db:"user_id"`
	Name             string    `json:"name" db:"name"`
	Description      string    `json:"description,omitempty" db:"description"`
	StorageProvider  string    `json:"storage_provider" db:"storage_provider"`
	StoragePath      string    `json:"storage_path" db:"storage_path"`
	SizeBytes        int64     `json:"size_bytes" db:"size_bytes"`
	FileCount        int       `json:"file_count" db:"file_count"`
	DatasetType      string    `json:"dataset_type,omitempty" db:"dataset_type"`
	Format           string    `json:"format,omitempty" db:"format"`
	Splits           string    `json:"splits,omitempty" db:"splits"` // JSONB stored as string
	StorageCostPerMonth float64 `json:"storage_cost_per_month" db:"storage_cost_per_month"`
	Status           string    `json:"status" db:"status"`
	IsPublic         bool      `json:"is_public" db:"is_public"`
	Tags             []string  `json:"tags,omitempty" db:"tags"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// TrainingJob represents a GPU training job
type TrainingJob struct {
	ID                 uuid.UUID  `json:"id" db:"id"`
	UserID             uuid.UUID  `json:"user_id" db:"user_id"`
	DatasetID          *uuid.UUID `json:"dataset_id,omitempty" db:"dataset_id"`
	Name               string     `json:"name" db:"name"`
	Description        string     `json:"description,omitempty" db:"description"`
	Framework          string     `json:"framework" db:"framework"`
	TrainingScriptPath string     `json:"training_script_path,omitempty" db:"training_script_path"`
	Entrypoint         string     `json:"entrypoint,omitempty" db:"entrypoint"`

	// Compute
	GPUType      string `json:"gpu_type,omitempty" db:"gpu_type"`
	GPUCount     int    `json:"gpu_count" db:"gpu_count"`
	GPUMemoryGB  int    `json:"gpu_memory_gb,omitempty" db:"gpu_memory_gb"`
	CPUCount     int    `json:"cpu_count" db:"cpu_count"`
	RAMGB        int    `json:"ram_gb" db:"ram_gb"`
	StorageGB    int    `json:"storage_gb" db:"storage_gb"`

	// Configuration (JSONB)
	Hyperparameters string `json:"hyperparameters,omitempty" db:"hyperparameters"`
	EnvironmentVars string `json:"environment_vars,omitempty" db:"environment_vars"`

	// Execution
	Provider        string     `json:"provider,omitempty" db:"provider"`
	InstanceID      string     `json:"instance_id,omitempty" db:"instance_id"`
	StartTime       *time.Time `json:"start_time,omitempty" db:"start_time"`
	EndTime         *time.Time `json:"end_time,omitempty" db:"end_time"`
	DurationSeconds int        `json:"duration_seconds" db:"duration_seconds"`

	// Progress
	Status       string  `json:"status" db:"status"`
	Progress     float64 `json:"progress" db:"progress"`
	CurrentEpoch int     `json:"current_epoch" db:"current_epoch"`
	TotalEpochs  int     `json:"total_epochs,omitempty" db:"total_epochs"`

	// Output
	ModelOutputPath string `json:"model_output_path,omitempty" db:"model_output_path"`
	LogsPath        string `json:"logs_path,omitempty" db:"logs_path"`
	Metrics         string `json:"metrics,omitempty" db:"metrics"` // JSONB

	// Costs
	EstimatedCost   float64    `json:"estimated_cost,omitempty" db:"estimated_cost"`
	ActualCost      float64    `json:"actual_cost,omitempty" db:"actual_cost"`
	GPUCostPerHour  float64    `json:"gpu_cost_per_hour,omitempty" db:"gpu_cost_per_hour"`
	BillingID       *uuid.UUID `json:"billing_id,omitempty" db:"billing_id"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TrainedModel represents a trained ML model
type TrainedModel struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	UserID          uuid.UUID  `json:"user_id" db:"user_id"`
	TrainingJobID   *uuid.UUID `json:"training_job_id,omitempty" db:"training_job_id"`
	Name            string     `json:"name" db:"name"`
	Version         string     `json:"version" db:"version"`
	Description     string     `json:"description,omitempty" db:"description"`

	// Storage
	StorageProvider string `json:"storage_provider" db:"storage_provider"`
	ModelPath       string `json:"model_path" db:"model_path"`
	ModelFormat     string `json:"model_format" db:"model_format"`
	SizeBytes       int64  `json:"size_bytes" db:"size_bytes"`

	// Metadata (JSONB)
	Framework        string `json:"framework,omitempty" db:"framework"`
	FrameworkVersion string `json:"framework_version,omitempty" db:"framework_version"`
	InputSchema      string `json:"input_schema,omitempty" db:"input_schema"`
	OutputSchema     string `json:"output_schema,omitempty" db:"output_schema"`
	Metrics          string `json:"metrics,omitempty" db:"metrics"`

	// Hardware Requirements
	RequiresGPU      bool `json:"requires_gpu" db:"requires_gpu"`
	MinGPUMemoryGB   int  `json:"min_gpu_memory_gb,omitempty" db:"min_gpu_memory_gb"`
	MinRAMGB         int  `json:"min_ram_gb,omitempty" db:"min_ram_gb"`

	// Deployment
	IsDeployed         bool   `json:"is_deployed" db:"is_deployed"`
	DeploymentEndpoint string `json:"deployment_endpoint,omitempty" db:"deployment_endpoint"`
	InferenceCount     int64  `json:"inference_count" db:"inference_count"`

	// Costs
	StorageCostPerMonth float64 `json:"storage_cost_per_month,omitempty" db:"storage_cost_per_month"`
	InferenceCostPer1k  float64 `json:"inference_cost_per_1k" db:"inference_cost_per_1k"`

	// Status
	Status   string   `json:"status" db:"status"`
	IsPublic bool     `json:"is_public" db:"is_public"`
	Tags     []string `json:"tags,omitempty" db:"tags"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// InferenceSession represents an active inference deployment
type InferenceSession struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	UserID         uuid.UUID  `json:"user_id" db:"user_id"`
	ModelID        uuid.UUID  `json:"model_id" db:"model_id"`
	DeploymentType string     `json:"deployment_type" db:"deployment_type"`

	// Reserved Capacity
	ReservedGPUType    string     `json:"reserved_gpu_type,omitempty" db:"reserved_gpu_type"`
	ReservedInstanceID string     `json:"reserved_instance_id,omitempty" db:"reserved_instance_id"`
	ReservedStartTime  *time.Time `json:"reserved_start_time,omitempty" db:"reserved_start_time"`
	ReservedEndTime    *time.Time `json:"reserved_end_time,omitempty" db:"reserved_end_time"`
	ReservedCostPerHour float64   `json:"reserved_cost_per_hour,omitempty" db:"reserved_cost_per_hour"`

	// Usage
	TotalRequests       int64   `json:"total_requests" db:"total_requests"`
	TotalInferenceTimeMs int64  `json:"total_inference_time_ms" db:"total_inference_time_ms"`
	AvgLatencyMs        float64 `json:"avg_latency_ms" db:"avg_latency_ms"`

	// Status
	Status    string     `json:"status" db:"status"`
	BillingID *uuid.UUID `json:"billing_id,omitempty" db:"billing_id"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// InferenceRequest represents a single inference request
type InferenceRequest struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	UserID            uuid.UUID  `json:"user_id" db:"user_id"`
	ModelID           uuid.UUID  `json:"model_id" db:"model_id"`
	SessionID         *uuid.UUID `json:"session_id,omitempty" db:"session_id"`
	RequestSizeBytes  int        `json:"request_size_bytes,omitempty" db:"request_size_bytes"`
	ResponseSizeBytes int        `json:"response_size_bytes,omitempty" db:"response_size_bytes"`
	LatencyMs         int        `json:"latency_ms" db:"latency_ms"`
	GPUUsed           bool       `json:"gpu_used" db:"gpu_used"`
	GPUTimeMs         int        `json:"gpu_time_ms,omitempty" db:"gpu_time_ms"`
	Status            string     `json:"status" db:"status"`
	ErrorMessage      string     `json:"error_message,omitempty" db:"error_message"`
	Cost              float64    `json:"cost" db:"cost"`
	Timestamp         time.Time  `json:"timestamp" db:"timestamp"`
}

// StorageUsage tracks storage costs
type StorageUsage struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	StorageType   string     `json:"storage_type" db:"storage_type"`
	ResourceID    *uuid.UUID `json:"resource_id,omitempty" db:"resource_id"`
	SizeBytes     int64      `json:"size_bytes" db:"size_bytes"`
	CostPerGBMonth float64   `json:"cost_per_gb_month" db:"cost_per_gb_month"`
	BillingMonth  string     `json:"billing_month" db:"billing_month"`
	DaysStored    int        `json:"days_stored" db:"days_stored"`
	Cost          float64    `json:"cost" db:"cost"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// TrainingJobQueue represents job queue metadata
type TrainingJobQueue struct {
	JobID               uuid.UUID  `json:"job_id" db:"job_id"`
	Priority            int        `json:"priority" db:"priority"`
	RetryCount          int        `json:"retry_count" db:"retry_count"`
	MaxRetries          int        `json:"max_retries" db:"max_retries"`
	QueueTime           time.Time  `json:"queue_time" db:"queue_time"`
	ProcessingStartedAt *time.Time `json:"processing_started_at,omitempty" db:"processing_started_at"`
	NextRetryAt         *time.Time `json:"next_retry_at,omitempty" db:"next_retry_at"`
	ErrorMessage        string     `json:"error_message,omitempty" db:"error_message"`
}

// Training job statuses
const (
	TrainingStatusQueued       = "queued"
	TrainingStatusProvisioning = "provisioning"
	TrainingStatusRunning      = "running"
	TrainingStatusCompleted    = "completed"
	TrainingStatusFailed       = "failed"
	TrainingStatusCancelled    = "cancelled"
)

// Deployment types
const (
	DeploymentTypeOnDemand   = "on_demand"
	DeploymentTypeReserved   = "reserved"
	DeploymentTypeServerless = "serverless"
)

// Dataset statuses
const (
	DatasetStatusUploading  = "uploading"
	DatasetStatusReady      = "ready"
	DatasetStatusProcessing = "processing"
	DatasetStatusError      = "error"
)

// Model statuses
const (
	ModelStatusReady     = "ready"
	ModelStatusDeploying = "deploying"
	ModelStatusDeployed  = "deployed"
	ModelStatusError     = "error"
)
