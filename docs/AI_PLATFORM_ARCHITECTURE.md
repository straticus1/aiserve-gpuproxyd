# AI Platform Architecture - Training, Storage & Inference as a Service

## 🎯 Vision: Complete MLaaS Platform

**The Customer Journey:**
1. **Upload Training Data** → darkstorage.io (pay for storage)
2. **Submit Training Job** → rent GPU by the hour (pay for compute)
3. **Store Trained Models** → model registry (pay for storage)
4. **Deploy for Inference** → pay-per-request or reserved capacity
5. **Monitor & Scale** → real-time metrics, auto-scaling

**Revenue Streams:**
- 💾 **Data Storage** - $X/GB/month (darkstorage.io integration)
- 🚀 **GPU Training** - $X/GPU-hour (Vast.ai, io.net, OCI)
- 🧠 **Model Storage** - $X/GB/month (trained model artifacts)
- ⚡ **Inference** - $X/1000 requests OR $X/hour reserved
- 📊 **Premium Features** - Model versioning, A/B testing, auto-scaling

---

## 🏗️ System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     AI Platform Frontend                         │
│  (Web Dashboard + API + CLI + SDK)                              │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                    API Gateway Layer                             │
│  - Authentication (JWT/API Keys)                                 │
│  - Rate Limiting (Guard Rails)                                   │
│  - Request Routing                                               │
└─────────────────────────────────────────────────────────────────┘
                              ↓
        ┌────────────────────┴────────────────────┐
        ↓                    ↓                    ↓
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Training   │    │  Inference   │    │   Storage    │
│   Service    │    │   Service    │    │   Service    │
└──────────────┘    └──────────────┘    └──────────────┘
        ↓                    ↓                    ↓
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ Job Queue    │    │ Model Serve  │    │ darkstorage  │
│ (Redis)      │    │ (ONNX/etc)   │    │ .io (S3)     │
└──────────────┘    └──────────────┘    └──────────────┘
        ↓                    ↓
┌──────────────────────────────────────┐
│      GPU Orchestration Layer         │
│  - GPU Pool Management                │
│  - Provider Selection (Vast/io.net)  │
│  - Load Balancing                    │
└──────────────────────────────────────┘
                    ↓
        ┌───────────┴────────────┐
        ↓           ↓            ↓
┌──────────┐  ┌──────────┐  ┌──────────┐
│ Vast.ai  │  │ io.net   │  │   OCI    │
│  GPUs    │  │  GPUs    │  │  GPUs    │
└──────────┘  └──────────┘  └──────────┘
```

---

## 📊 Database Schema Extensions

### New Tables Needed:

#### 1. `datasets` - User Training Data
```sql
CREATE TABLE datasets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Storage
    storage_provider VARCHAR(50) DEFAULT 'darkstorage',  -- darkstorage, s3, gcs
    storage_path TEXT NOT NULL,                          -- darkstorage://bucket/user_id/dataset_id/
    size_bytes BIGINT NOT NULL,
    file_count INTEGER DEFAULT 0,

    -- Metadata
    dataset_type VARCHAR(50),                            -- image, text, tabular, audio, video
    format VARCHAR(50),                                  -- csv, parquet, jsonl, tfrecord, custom
    splits JSONB,                                        -- {"train": 0.8, "val": 0.1, "test": 0.1}

    -- Costs
    storage_cost_per_month DECIMAL(10,4) DEFAULT 0.00,  -- Calculated cost

    -- Status
    status VARCHAR(50) DEFAULT 'uploading',              -- uploading, ready, processing, error
    is_public BOOLEAN DEFAULT FALSE,
    tags TEXT[],

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_datasets_user_id ON datasets(user_id);
CREATE INDEX idx_datasets_status ON datasets(status);
CREATE INDEX idx_datasets_storage_path ON datasets(storage_path);
```

#### 2. `training_jobs` - GPU Training Orchestration
```sql
CREATE TABLE training_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dataset_id UUID REFERENCES datasets(id) ON DELETE SET NULL,

    -- Job Configuration
    name VARCHAR(255) NOT NULL,
    description TEXT,
    framework VARCHAR(50) NOT NULL,                      -- pytorch, tensorflow, sklearn, custom
    training_script_path TEXT,                           -- S3/darkstorage path to training script
    entrypoint VARCHAR(255),                             -- main.py, train.sh

    -- Compute Requirements
    gpu_type VARCHAR(100),                               -- H100, A100, RTX4090, etc.
    gpu_count INTEGER DEFAULT 1,
    gpu_memory_gb INTEGER,
    cpu_count INTEGER DEFAULT 4,
    ram_gb INTEGER DEFAULT 32,
    storage_gb INTEGER DEFAULT 100,

    -- Hyperparameters (JSON)
    hyperparameters JSONB,                               -- {"lr": 0.001, "batch_size": 32, ...}
    environment_vars JSONB,                              -- {"PYTORCH_VERSION": "2.0", ...}

    -- Execution
    provider VARCHAR(50),                                -- vastai, ionet, oci
    instance_id VARCHAR(255),                            -- Provider's instance ID
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    duration_seconds INTEGER,

    -- Status & Progress
    status VARCHAR(50) DEFAULT 'queued',                 -- queued, provisioning, running, completed, failed, cancelled
    progress DECIMAL(5,2) DEFAULT 0.00,                 -- 0.00 to 100.00
    current_epoch INTEGER DEFAULT 0,
    total_epochs INTEGER,

    -- Output
    model_output_path TEXT,                              -- darkstorage://bucket/models/job_id/
    logs_path TEXT,                                      -- darkstorage://bucket/logs/job_id/
    metrics JSONB,                                       -- {"loss": [0.5, 0.3, ...], "accuracy": [...]}

    -- Costs
    estimated_cost DECIMAL(10,4),
    actual_cost DECIMAL(10,4),
    gpu_cost_per_hour DECIMAL(10,4),

    -- Billing
    billing_id UUID REFERENCES billing_transactions(id),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_training_jobs_user_id ON training_jobs(user_id);
CREATE INDEX idx_training_jobs_status ON training_jobs(status);
CREATE INDEX idx_training_jobs_dataset_id ON training_jobs(dataset_id);
CREATE INDEX idx_training_jobs_provider ON training_jobs(provider, instance_id);
```

#### 3. `trained_models` - Model Registry
```sql
CREATE TABLE trained_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    training_job_id UUID REFERENCES training_jobs(id) ON DELETE SET NULL,

    -- Model Identity
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) DEFAULT '1.0.0',
    description TEXT,

    -- Model Artifacts
    storage_provider VARCHAR(50) DEFAULT 'darkstorage',
    model_path TEXT NOT NULL,                            -- darkstorage://bucket/models/model_id/
    model_format VARCHAR(50) NOT NULL,                   -- onnx, pytorch, tensorflow, pickle, etc.
    size_bytes BIGINT NOT NULL,

    -- Model Metadata
    framework VARCHAR(50),                               -- pytorch, tensorflow, sklearn
    framework_version VARCHAR(50),
    input_schema JSONB,                                  -- {"inputs": [{"name": "image", "shape": [...]}]}
    output_schema JSONB,
    metrics JSONB,                                       -- {"accuracy": 0.95, "f1": 0.92}

    -- Hardware Requirements
    requires_gpu BOOLEAN DEFAULT FALSE,
    min_gpu_memory_gb INTEGER,
    min_ram_gb INTEGER,

    -- Deployment
    is_deployed BOOLEAN DEFAULT FALSE,
    deployment_endpoint TEXT,                            -- /serve/models/{model_id}/predict
    inference_count BIGINT DEFAULT 0,

    -- Costs
    storage_cost_per_month DECIMAL(10,4),
    inference_cost_per_1k DECIMAL(10,4) DEFAULT 0.10,   -- $0.10 per 1000 requests

    -- Status
    status VARCHAR(50) DEFAULT 'ready',                  -- ready, deploying, deployed, error
    is_public BOOLEAN DEFAULT FALSE,
    tags TEXT[],

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_trained_models_user_id ON trained_models(user_id);
CREATE INDEX idx_trained_models_training_job_id ON trained_models(training_job_id);
CREATE INDEX idx_trained_models_is_deployed ON trained_models(is_deployed);
CREATE INDEX idx_trained_models_status ON trained_models(status);
```

#### 4. `inference_sessions` - Inference Deployments
```sql
CREATE TABLE inference_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_id UUID NOT NULL REFERENCES trained_models(id) ON DELETE CASCADE,

    -- Deployment Type
    deployment_type VARCHAR(50) NOT NULL,                -- on_demand, reserved, serverless

    -- Reserved Capacity (if reserved)
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
    status VARCHAR(50) DEFAULT 'active',                 -- active, paused, terminated

    -- Billing
    billing_id UUID REFERENCES billing_transactions(id),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_inference_sessions_user_id ON inference_sessions(user_id);
CREATE INDEX idx_inference_sessions_model_id ON inference_sessions(model_id);
CREATE INDEX idx_inference_sessions_status ON inference_sessions(status);
```

#### 5. `inference_requests` - Per-Request Tracking
```sql
CREATE TABLE inference_requests (
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
    status VARCHAR(50) DEFAULT 'success',                -- success, error, timeout
    error_message TEXT,

    -- Billing
    cost DECIMAL(10,6),                                  -- Micro-transaction cost

    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Partitioned by month for efficiency
CREATE INDEX idx_inference_requests_user_id ON inference_requests(user_id, timestamp DESC);
CREATE INDEX idx_inference_requests_model_id ON inference_requests(model_id);
CREATE INDEX idx_inference_requests_timestamp ON inference_requests(timestamp);
```

#### 6. `storage_usage` - Detailed Storage Billing
```sql
CREATE TABLE storage_usage (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Storage Breakdown
    storage_type VARCHAR(50) NOT NULL,                   -- dataset, model, logs, artifacts
    resource_id UUID,                                    -- ID of dataset/model/etc

    -- Usage Metrics
    size_bytes BIGINT NOT NULL,
    cost_per_gb_month DECIMAL(10,6) DEFAULT 0.10,       -- $0.10/GB/month

    -- Billing Period
    billing_month VARCHAR(7) NOT NULL,                   -- "2026-01"
    days_stored INTEGER DEFAULT 0,
    cost DECIMAL(10,4),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(user_id, storage_type, resource_id, billing_month)
);

CREATE INDEX idx_storage_usage_user_id ON storage_usage(user_id);
CREATE INDEX idx_storage_usage_billing_month ON storage_usage(billing_month);
```

---

## 🎮 API Endpoints

### Dataset Management

```
POST   /datasets                    # Upload/register dataset
GET    /datasets                    # List user's datasets
GET    /datasets/{id}               # Get dataset details
PUT    /datasets/{id}               # Update dataset metadata
DELETE /datasets/{id}               # Delete dataset
POST   /datasets/{id}/upload        # Upload files to dataset
GET    /datasets/{id}/files         # List files in dataset
GET    /datasets/{id}/stats         # Get dataset statistics
```

### Training Jobs

```
POST   /training/jobs               # Submit training job
GET    /training/jobs               # List user's training jobs
GET    /training/jobs/{id}          # Get job status
PUT    /training/jobs/{id}          # Update job (pause/resume)
DELETE /training/jobs/{id}          # Cancel job
GET    /training/jobs/{id}/logs     # Stream training logs
GET    /training/jobs/{id}/metrics  # Get training metrics
POST   /training/jobs/{id}/retry    # Retry failed job
```

### Model Registry

```
POST   /models                      # Register trained model
GET    /models                      # List user's models
GET    /models/{id}                 # Get model details
PUT    /models/{id}                 # Update model metadata
DELETE /models/{id}                 # Delete model
POST   /models/{id}/deploy          # Deploy model for inference
POST   /models/{id}/undeploy        # Undeploy model
GET    /models/{id}/versions        # List model versions
POST   /models/{id}/download        # Download model artifacts
```

### Inference

```
POST   /inference/sessions          # Create inference session (reserved capacity)
GET    /inference/sessions          # List active sessions
DELETE /inference/sessions/{id}     # Terminate session

POST   /inference/predict           # On-demand inference (pay-per-request)
POST   /models/{id}/predict         # Inference on specific model
POST   /models/{id}/batch-predict   # Batch inference
```

### Billing & Usage

```
GET    /billing/usage               # Get current month usage
GET    /billing/usage/training      # Training costs breakdown
GET    /billing/usage/inference     # Inference costs breakdown
GET    /billing/usage/storage       # Storage costs breakdown
GET    /billing/estimate            # Estimate costs for planned job
POST   /billing/topup               # Add credits
GET    /billing/history             # Billing history
```

---

## 💰 Pricing Model

### Storage Pricing (darkstorage.io)

```
Dataset Storage:    $0.10/GB/month
Model Storage:      $0.15/GB/month  (higher priority, faster access)
Logs/Artifacts:     $0.05/GB/month
```

### Training Pricing (GPU Rental)

```
Budget Tier:
  RTX 3060:         $0.30/hour
  GTX 1080 Ti:      $0.25/hour

Mid-Range Tier:
  RTX 4090:         $0.80/hour
  RTX 3090:         $0.60/hour

High-End Tier:
  A100 (40GB):      $2.50/hour
  A100 (80GB):      $3.50/hour

Enterprise Tier:
  H100:             $5.00/hour
  H200:             $6.00/hour
```

### Inference Pricing

**On-Demand (Pay-per-Request):**
```
CPU Inference:      $0.10/1000 requests
GPU Inference:      $0.50/1000 requests
Batch Inference:    $0.25/1000 requests  (20% discount)
```

**Reserved Capacity (Pay-per-Hour):**
```
CPU Instance:       $0.10/hour  (unlimited requests)
GPU Instance:       $1.00/hour  (unlimited requests)
Multi-GPU:          $2.50/hour  (4x GPUs, load balanced)
```

### Premium Features

```
Model Versioning:   $5/month
A/B Testing:        $10/month
Auto-Scaling:       $20/month
Private Storage:    $50/month  (dedicated darkstorage bucket)
```

---

## 🔧 Implementation Priority

### Phase 1: Foundation (Week 1-2)
- ✅ Database schema migrations
- ✅ Dataset upload API (darkstorage.io integration)
- ✅ Storage billing tracking
- ✅ Basic dataset management UI

### Phase 2: Training (Week 3-4)
- ✅ Training job submission API
- ✅ Redis job queue (Bull/BullMQ equivalent for Go)
- ✅ GPU orchestration (Vast.ai/io.net integration)
- ✅ Training job monitoring
- ✅ Cost tracking per job

### Phase 3: Model Registry (Week 5-6)
- ✅ Model upload/registration
- ✅ Model versioning
- ✅ Model metadata management
- ✅ Model storage billing

### Phase 4: Inference (Week 7-8)
- ✅ ONNX Runtime integration (already designed)
- ✅ On-demand inference API
- ✅ Reserved capacity management
- ✅ Inference request tracking
- ✅ Per-request billing

### Phase 5: Polish & Scale (Week 9-10)
- ✅ Dashboard UI (model performance, costs)
- ✅ Auto-scaling inference
- ✅ A/B testing framework
- ✅ Optimization recommendations
- ✅ Multi-model deployment

---

## 🚀 Quick Win: MVP Features

**Minimum Viable Product (2 weeks):**

1. **Dataset Upload** → User uploads CSV/images to darkstorage.io
2. **Training Job** → User submits PyTorch training script
3. **GPU Rental** → Platform rents Vast.ai GPU, runs training
4. **Model Export** → Training outputs ONNX model to darkstorage.io
5. **Inference Deploy** → User deploys model for predictions
6. **Billing** → Track GPU hours, storage, inference requests

**User Flow:**
```bash
# Upload dataset
curl -F "file=@train.csv" https://api.aiserve.com/datasets

# Submit training job
curl -X POST https://api.aiserve.com/training/jobs \
  -d '{
    "dataset_id": "123-456",
    "framework": "pytorch",
    "training_script": "darkstorage://scripts/train.py",
    "gpu_type": "RTX4090",
    "hyperparameters": {"epochs": 10, "lr": 0.001}
  }'

# Check status
curl https://api.aiserve.com/training/jobs/789

# Deploy model
curl -X POST https://api.aiserve.com/models/abc-def/deploy

# Run inference
curl -X POST https://api.aiserve.com/models/abc-def/predict \
  -d '{"input": [1.0, 2.0, 3.0]}'
```

---

## 🎯 Success Metrics

**Customer Metrics:**
- Training jobs completed/day
- Average GPU utilization
- Inference requests/second
- Customer retention rate
- Average revenue per user (ARPU)

**Platform Metrics:**
- GPU pool utilization
- Training job success rate
- Average inference latency
- Storage usage growth
- Profit margin per service

**Target Economics:**
- Training: 30% margin (rent GPU at $2/hr, charge $2.60/hr)
- Storage: 50% margin (darkstorage cost + 50%)
- Inference: 60% margin (CPU inference, high volume)

---

## 🔐 Security & Isolation

**Data Isolation:**
- User datasets stored in separate darkstorage.io buckets
- Training jobs run in isolated containers/VMs
- Models stored with user-scoped access control
- Inference sessions isolated per user

**Access Control:**
- JWT/API key authentication on all endpoints
- Row-level security (user_id checks)
- Rate limiting per user tier
- Audit logs for all operations

**Compliance:**
- GDPR-compliant data deletion
- SOC 2 Type II readiness
- Encrypted storage (darkstorage.io)
- Encrypted training data transfer

---

## 📊 Monitoring & Observability

**Metrics to Track:**
- Training job queue depth
- GPU utilization per provider
- Inference latency p50/p95/p99
- Storage usage trends
- Cost per user/service
- Error rates

**Alerting:**
- Training job failures
- GPU rental failures
- Inference latency spikes
- Cost overruns (Guard Rails)
- Storage quota exceeded

---

## 🎨 UI/Dashboard Mockup

**Dashboard Sections:**
1. **Overview** - Current costs, active jobs, storage usage
2. **Datasets** - Upload, manage, visualize datasets
3. **Training** - Submit jobs, monitor progress, view logs
4. **Models** - Registry, deploy, version management
5. **Inference** - Test models, view metrics, scale
6. **Billing** - Usage breakdown, cost estimates, history

**Key Visualizations:**
- Training cost over time (chart)
- GPU utilization heatmap
- Inference latency histogram
- Storage growth trend
- Cost breakdown pie chart

---

## 🔮 Future Enhancements

**Advanced Features:**
- Distributed training (multi-GPU, multi-node)
- AutoML (hyperparameter tuning)
- Model optimization (quantization, pruning)
- Edge deployment (TensorFlow Lite, ONNX Mobile)
- Federated learning
- Model monitoring (drift detection)
- Custom docker container support
- Spot instance training (lower cost, interruptible)

**Marketplace:**
- Pre-trained models marketplace
- Public datasets marketplace
- Training script templates
- Model fine-tuning as a service

---

## 💡 Competitive Advantages

**vs AWS SageMaker:**
- ✅ 50-70% lower cost (rented GPUs)
- ✅ Simpler pricing (no hidden fees)
- ✅ Pay-per-use (no minimum spend)

**vs Google Vertex AI:**
- ✅ Faster time-to-train (pre-warmed GPU pool)
- ✅ More GPU options (Vast.ai + io.net)
- ✅ No vendor lock-in (ONNX standard)

**vs Replicate/Banana:**
- ✅ Full training pipeline (not just inference)
- ✅ User-owned data (darkstorage.io)
- ✅ Custom training scripts

---

## 🎯 Go-to-Market Strategy

**Target Customers:**
1. **Indie ML Engineers** - Personal projects, low budget
2. **Startups** - MVP training, cost-sensitive
3. **Researchers** - One-off experiments
4. **Enterprises** - Overflow capacity, cost optimization

**Pricing Tiers:**
- **Free Tier** - 1GB storage, 1 GPU-hour/month, 1000 inference requests
- **Starter** - $20/month - 10GB storage, $20 training credits
- **Pro** - $100/month - 100GB storage, $100 training credits, premium features
- **Enterprise** - Custom pricing, SLA, dedicated support

---

This is the blueprint. Ready to build it?
