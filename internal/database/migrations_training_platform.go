package database

import (
	"context"
	"fmt"
)

// MigrateTrainingPlatform adds tables for the complete AI training & inference platform
// This includes: datasets, training_jobs, trained_models, inference_sessions, inference_requests, storage_usage
func (db *PostgresDB) MigrateTrainingPlatform() error {
	ctx := context.Background()

	queries := []string{
		// 1. Datasets - User training data management
		`CREATE TABLE IF NOT EXISTS datasets (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			description TEXT,

			-- Storage
			storage_provider VARCHAR(50) DEFAULT 'darkstorage',
			storage_path TEXT NOT NULL,
			size_bytes BIGINT NOT NULL DEFAULT 0,
			file_count INTEGER DEFAULT 0,

			-- Metadata
			dataset_type VARCHAR(50),
			format VARCHAR(50),
			splits JSONB,

			-- Costs
			storage_cost_per_month DECIMAL(10,4) DEFAULT 0.00,

			-- Status
			status VARCHAR(50) DEFAULT 'uploading',
			is_public BOOLEAN DEFAULT FALSE,
			tags TEXT[],

			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_datasets_user_id ON datasets(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_datasets_status ON datasets(status)`,
		`CREATE INDEX IF NOT EXISTS idx_datasets_storage_path ON datasets(storage_path)`,
		`CREATE INDEX IF NOT EXISTS idx_datasets_created_at ON datasets(created_at DESC)`,

		// 2. Training Jobs - GPU training orchestration
		`CREATE TABLE IF NOT EXISTS training_jobs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			dataset_id UUID REFERENCES datasets(id) ON DELETE SET NULL,

			-- Job Configuration
			name VARCHAR(255) NOT NULL,
			description TEXT,
			framework VARCHAR(50) NOT NULL,
			training_script_path TEXT,
			entrypoint VARCHAR(255),

			-- Compute Requirements
			gpu_type VARCHAR(100),
			gpu_count INTEGER DEFAULT 1,
			gpu_memory_gb INTEGER,
			cpu_count INTEGER DEFAULT 4,
			ram_gb INTEGER DEFAULT 32,
			storage_gb INTEGER DEFAULT 100,

			-- Hyperparameters
			hyperparameters JSONB,
			environment_vars JSONB,

			-- Execution
			provider VARCHAR(50),
			instance_id VARCHAR(255),
			start_time TIMESTAMP,
			end_time TIMESTAMP,
			duration_seconds INTEGER,

			-- Status & Progress
			status VARCHAR(50) DEFAULT 'queued',
			progress DECIMAL(5,2) DEFAULT 0.00,
			current_epoch INTEGER DEFAULT 0,
			total_epochs INTEGER,

			-- Output
			model_output_path TEXT,
			logs_path TEXT,
			metrics JSONB,

			-- Costs
			estimated_cost DECIMAL(10,4),
			actual_cost DECIMAL(10,4),
			gpu_cost_per_hour DECIMAL(10,4),

			-- Billing
			billing_id UUID REFERENCES billing_transactions(id),

			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_training_jobs_user_id ON training_jobs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_training_jobs_status ON training_jobs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_training_jobs_dataset_id ON training_jobs(dataset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_training_jobs_provider ON training_jobs(provider, instance_id)`,
		`CREATE INDEX IF NOT EXISTS idx_training_jobs_created_at ON training_jobs(created_at DESC)`,

		// 3. Trained Models - Model registry
		`CREATE TABLE IF NOT EXISTS trained_models (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			training_job_id UUID REFERENCES training_jobs(id) ON DELETE SET NULL,

			-- Model Identity
			name VARCHAR(255) NOT NULL,
			version VARCHAR(50) DEFAULT '1.0.0',
			description TEXT,

			-- Model Artifacts
			storage_provider VARCHAR(50) DEFAULT 'darkstorage',
			model_path TEXT NOT NULL,
			model_format VARCHAR(50) NOT NULL,
			size_bytes BIGINT NOT NULL DEFAULT 0,

			-- Model Metadata
			framework VARCHAR(50),
			framework_version VARCHAR(50),
			input_schema JSONB,
			output_schema JSONB,
			metrics JSONB,

			-- Hardware Requirements
			requires_gpu BOOLEAN DEFAULT FALSE,
			min_gpu_memory_gb INTEGER,
			min_ram_gb INTEGER,

			-- Deployment
			is_deployed BOOLEAN DEFAULT FALSE,
			deployment_endpoint TEXT,
			inference_count BIGINT DEFAULT 0,

			-- Costs
			storage_cost_per_month DECIMAL(10,4),
			inference_cost_per_1k DECIMAL(10,4) DEFAULT 0.10,

			-- Status
			status VARCHAR(50) DEFAULT 'ready',
			is_public BOOLEAN DEFAULT FALSE,
			tags TEXT[],

			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_trained_models_user_id ON trained_models(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trained_models_training_job_id ON trained_models(training_job_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trained_models_is_deployed ON trained_models(is_deployed)`,
		`CREATE INDEX IF NOT EXISTS idx_trained_models_status ON trained_models(status)`,
		`CREATE INDEX IF NOT EXISTS idx_trained_models_created_at ON trained_models(created_at DESC)`,

		// 4. Inference Sessions - Deployment tracking
		`CREATE TABLE IF NOT EXISTS inference_sessions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			model_id UUID NOT NULL REFERENCES trained_models(id) ON DELETE CASCADE,

			-- Deployment Type
			deployment_type VARCHAR(50) NOT NULL,

			-- Reserved Capacity
			reserved_gpu_type VARCHAR(100),
			reserved_instance_id VARCHAR(255),
			reserved_start_time TIMESTAMP,
			reserved_end_time TIMESTAMP,
			reserved_cost_per_hour DECIMAL(10,4),

			-- Usage Tracking
			total_requests BIGINT DEFAULT 0,
			total_inference_time_ms BIGINT DEFAULT 0,
			avg_latency_ms DECIMAL(10,2),

			-- Status
			status VARCHAR(50) DEFAULT 'active',

			-- Billing
			billing_id UUID REFERENCES billing_transactions(id),

			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_inference_sessions_user_id ON inference_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_inference_sessions_model_id ON inference_sessions(model_id)`,
		`CREATE INDEX IF NOT EXISTS idx_inference_sessions_status ON inference_sessions(status)`,

		// 5. Inference Requests - Per-request tracking
		`CREATE TABLE IF NOT EXISTS inference_requests (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			model_id UUID NOT NULL REFERENCES trained_models(id) ON DELETE CASCADE,
			session_id UUID REFERENCES inference_sessions(id) ON DELETE SET NULL,

			-- Request Details
			request_size_bytes INTEGER,
			response_size_bytes INTEGER,
			latency_ms INTEGER,

			-- GPU Usage
			gpu_used BOOLEAN DEFAULT FALSE,
			gpu_time_ms INTEGER,

			-- Status
			status VARCHAR(50) DEFAULT 'success',
			error_message TEXT,

			-- Billing
			cost DECIMAL(10,6),

			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_inference_requests_user_id ON inference_requests(user_id, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_inference_requests_model_id ON inference_requests(model_id)`,
		`CREATE INDEX IF NOT EXISTS idx_inference_requests_timestamp ON inference_requests(timestamp)`,

		// 6. Storage Usage - Detailed storage billing
		`CREATE TABLE IF NOT EXISTS storage_usage (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

			-- Storage Breakdown
			storage_type VARCHAR(50) NOT NULL,
			resource_id UUID,

			-- Usage Metrics
			size_bytes BIGINT NOT NULL,
			cost_per_gb_month DECIMAL(10,6) DEFAULT 0.10,

			-- Billing Period
			billing_month VARCHAR(7) NOT NULL,
			days_stored INTEGER DEFAULT 0,
			cost DECIMAL(10,4),

			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

			UNIQUE(user_id, storage_type, resource_id, billing_month)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_storage_usage_user_id ON storage_usage(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_storage_usage_billing_month ON storage_usage(billing_month)`,
		`CREATE INDEX IF NOT EXISTS idx_storage_usage_resource ON storage_usage(storage_type, resource_id)`,

		// 7. Training Job Queue - Redis-backed job queue metadata
		`CREATE TABLE IF NOT EXISTS training_job_queue (
			job_id UUID PRIMARY KEY REFERENCES training_jobs(id) ON DELETE CASCADE,
			priority INTEGER DEFAULT 5,
			retry_count INTEGER DEFAULT 0,
			max_retries INTEGER DEFAULT 3,
			queue_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			processing_started_at TIMESTAMP,
			next_retry_at TIMESTAMP,
			error_message TEXT
		)`,

		`CREATE INDEX IF NOT EXISTS idx_training_job_queue_priority ON training_job_queue(priority DESC, queue_time ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_training_job_queue_next_retry ON training_job_queue(next_retry_at) WHERE next_retry_at IS NOT NULL`,
	}

	for _, query := range queries {
		if _, err := db.Pool.Exec(ctx, query); err != nil {
			return fmt.Errorf("training platform migration failed: %w", err)
		}
	}

	return nil
}
