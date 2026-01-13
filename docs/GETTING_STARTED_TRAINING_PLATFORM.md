# Getting Started: AI Training & Inference Platform

## 🎉 What We Built

You now have the foundation for a **complete MLaaS (Machine Learning as a Service) platform** with three revenue streams:

1. **💾 Data Storage** - darkstorage.io integration (S3-compatible)
2. **🚀 GPU Training** - Rent GPUs, run training jobs, track costs
3. **⚡ Inference Serving** - Deploy models, pay-per-request or reserved capacity

## 📁 What's Been Created

### Database Schema (✅ Complete)
- **`datasets`** - User training data with storage tracking
- **`training_jobs`** - GPU training job orchestration
- **`trained_models`** - Model registry with versioning
- **`inference_sessions`** - Deployment tracking
- **`inference_requests`** - Per-request billing
- **`storage_usage`** - Storage cost breakdown

Location: `internal/database/migrations_training_platform.go`

### Go Models (✅ Complete)
- `Dataset`, `TrainingJob`, `TrainedModel`
- `InferenceSession`, `InferenceRequest`, `StorageUsage`
- All with proper JSON/DB tags

Location: `internal/models/training.go`

### Storage Client (✅ Complete)
- S3-compatible darkstorage.io client
- User-scoped paths (datasets/{user_id}/{dataset_id}/)
- Upload/download/delete operations
- Presigned URLs for secure access
- Storage usage calculation

Location: `internal/storage/darkstorage.go`

### Architecture Documentation (✅ Complete)
- Complete system design
- API endpoint specifications
- Pricing model
- Implementation roadmap

Location: `docs/AI_PLATFORM_ARCHITECTURE.md`

## 🚀 Quick Start

### Step 1: Run Database Migrations

```bash
# Build the admin tool
go build -o bin/admin ./cmd/admin

# Run existing migrations
./bin/admin migrate

# Add training platform tables (you'll need to add this to admin tool)
# We'll do this in Step 2
```

### Step 2: Add Migration Command to Admin Tool

Edit `cmd/admin/main.go` and add:

```go
case "migrate-training":
    if err := db.MigrateTrainingPlatform(); err != nil {
        log.Fatalf("Training platform migration failed: %v", err)
    }
    fmt.Println("Training platform migration completed successfully")
```

Then run:

```bash
./bin/admin migrate-training
```

### Step 3: Configure darkstorage.io Credentials

Add to `.env`:

```bash
# DarkStorage Configuration
DARKSTORAGE_ENDPOINT=https://darkstorage.io  # or your darkstorage endpoint
DARKSTORAGE_ACCESS_KEY=your-access-key-here
DARKSTORAGE_SECRET_KEY=your-secret-key-here
DARKSTORAGE_BUCKET=aiserve-datasets
```

### Step 4: Test Storage Client

Create a test script `cmd/test-storage/main.go`:

```go
package main

import (
    "context"
    "log"
    "strings"

    "github.com/aiserve/gpuproxy/internal/storage"
    "github.com/google/uuid"
)

func main() {
    // Initialize client
    client, err := storage.NewDarkStorageClient(&storage.DarkStorageConfig{
        Endpoint:        "https://darkstorage.io",
        AccessKeyID:     "your-key",
        SecretAccessKey: "your-secret",
        Bucket:          "aiserve-datasets",
        Region:          "us-east-1",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Test upload
    userID := uuid.New()
    datasetID := uuid.New()
    data := strings.NewReader("test,data,csv\n1,2,3\n")

    uri, size, err := client.UploadDataset(context.Background(), userID, datasetID, "test.csv", data, "text/csv")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Uploaded! URI: %s, Size: %d bytes", uri, size)
}
```

## 📋 Implementation Roadmap

### Phase 1: Core Infrastructure (Week 1) - 🚧 IN PROGRESS

**Already Done:**
- ✅ Database schema designed and created
- ✅ Go models defined
- ✅ Storage client implemented
- ✅ Architecture documented

**TODO:**
- [ ] Add migration command to admin tool
- [ ] Run migrations in development
- [ ] Test darkstorage.io integration
- [ ] Add darkstorage config to main config loader

### Phase 2: Dataset Management (Week 1-2)

Create `internal/api/dataset_handler.go`:

```go
package api

import (
    "net/http"
    "github.com/gorilla/mux"
)

type DatasetHandler struct {
    db      *database.PostgresDB
    storage *storage.DarkStorageClient
}

func (h *DatasetHandler) CreateDataset(w http.ResponseWriter, r *http.Request) {
    // POST /datasets
    // 1. Parse multipart form
    // 2. Extract user from context (auth middleware)
    // 3. Create dataset record in DB
    // 4. Upload files to darkstorage.io
    // 5. Update dataset size and file count
    // 6. Return dataset ID and URI
}

func (h *DatasetHandler) ListDatasets(w http.ResponseWriter, r *http.Request) {
    // GET /datasets
    // Return user's datasets with pagination
}

func (h *DatasetHandler) GetDataset(w http.ResponseWriter, r *http.Request) {
    // GET /datasets/{id}
    // Return dataset details + file list
}

func (h *DatasetHandler) DeleteDataset(w http.ResponseWriter, r *http.Request) {
    // DELETE /datasets/{id}
    // 1. Delete from darkstorage.io
    // 2. Delete from database
}
```

### Phase 3: Training Jobs (Week 2-3)

**3.1: Job Queue (Redis)**

Install Redis client:
```bash
go get github.com/redis/go-redis/v9
```

Create `internal/training/queue.go`:

```go
package training

import (
    "context"
    "encoding/json"
    "github.com/redis/go-redis/v9"
    "github.com/google/uuid"
)

type JobQueue struct {
    redis *redis.Client
}

func (q *JobQueue) EnqueueJob(ctx context.Context, jobID uuid.UUID, priority int) error {
    // Add to Redis sorted set (score = priority)
    return q.redis.ZAdd(ctx, "training_jobs_queue", redis.Z{
        Score:  float64(priority),
        Member: jobID.String(),
    }).Err()
}

func (q *JobQueue) DequeueJob(ctx context.Context) (uuid.UUID, error) {
    // Pop highest priority job
    result, err := q.redis.ZPopMin(ctx, "training_jobs_queue", 1).Result()
    if err != nil {
        return uuid.Nil, err
    }

    if len(result) == 0 {
        return uuid.Nil, nil // Queue empty
    }

    return uuid.Parse(result[0].Member.(string))
}
```

**3.2: Training Orchestrator**

Create `internal/training/orchestrator.go`:

```go
package training

import (
    "context"
    "fmt"
    "github.com/aiserve/gpuproxy/internal/models"
    "github.com/aiserve/gpuproxy/helpers/vastai"
)

type Orchestrator struct {
    db        *database.PostgresDB
    queue     *JobQueue
    storage   *storage.DarkStorageClient
    vastAI    *vastai.Client
}

func (o *Orchestrator) ProcessJob(ctx context.Context, jobID uuid.UUID) error {
    // 1. Load job from database
    job, err := o.loadJob(ctx, jobID)
    if err != nil {
        return err
    }

    // 2. Update status to "provisioning"
    o.updateJobStatus(ctx, jobID, models.TrainingStatusProvisioning)

    // 3. Rent GPU from provider (Vast.ai, io.net, OCI)
    instance, err := o.rentGPU(ctx, job)
    if err != nil {
        o.updateJobStatus(ctx, jobID, models.TrainingStatusFailed)
        return err
    }

    // 4. Update job with instance ID
    o.updateJobInstance(ctx, jobID, instance.ID)

    // 5. Download training script from darkstorage
    script, err := o.storage.DownloadFileFromURI(ctx, job.TrainingScriptPath)
    if err != nil {
        return err
    }

    // 6. SSH to instance, upload script, start training
    o.updateJobStatus(ctx, jobID, models.TrainingStatusRunning)
    err = o.startTraining(ctx, instance, job, script)
    if err != nil {
        return err
    }

    // 7. Monitor training (in goroutine)
    go o.monitorTraining(ctx, jobID, instance.ID)

    return nil
}

func (o *Orchestrator) monitorTraining(ctx context.Context, jobID uuid.UUID, instanceID string) {
    // Poll training logs
    // Update progress in database
    // When complete:
    //   - Download trained model
    //   - Upload to darkstorage.io
    //   - Create trained_models record
    //   - Release GPU
    //   - Update job status to "completed"
}
```

**3.3: Training API**

Create `internal/api/training_handler.go`:

```go
func (h *TrainingHandler) SubmitJob(w http.ResponseWriter, r *http.Request) {
    // POST /training/jobs
    // 1. Parse request body
    // 2. Validate dataset exists
    // 3. Estimate cost
    // 4. Create training_jobs record
    // 5. Enqueue job
    // 6. Return job ID
}

func (h *TrainingHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
    // GET /training/jobs/{id}
    // Return job details + progress
}

func (h *TrainingHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
    // GET /training/jobs/{id}/logs
    // Stream logs via Server-Sent Events (SSE)
}
```

### Phase 4: Model Registry (Week 3-4)

Create `internal/api/model_handler.go` (extends existing model serve handler):

```go
func (h *ModelHandler) RegisterModel(w http.ResponseWriter, r *http.Request) {
    // POST /models
    // 1. Parse model metadata
    // 2. Create trained_models record
    // 3. Return model ID
}

func (h *ModelHandler) DeployModel(w http.ResponseWriter, r *http.Request) {
    // POST /models/{id}/deploy
    // 1. Load model from darkstorage.io
    // 2. Load into ONNX runtime (or appropriate runtime)
    // 3. Create inference_sessions record
    // 4. Return deployment endpoint
}
```

### Phase 5: ONNX Inference (Week 4)

Add ONNX runtime integration (already designed in previous conversation):

```bash
go get github.com/yalue/onnxruntime_go
```

Create `internal/ml/onnx_runtime.go` and integrate with existing RuntimeOrchestrator.

### Phase 6: Billing Integration (Week 5)

Extend existing `internal/billing/service.go`:

```go
func (s *Service) ChargeTrainingJob(ctx context.Context, job *models.TrainingJob) error {
    // Calculate cost: GPU hours * rate
    cost := float64(job.DurationSeconds) / 3600.0 * job.GPUCostPerHour

    // Create billing transaction
    tx, err := s.CreateTransaction(ctx, job.UserID, cost, "USD", ProviderAfterDark, "gpu_training")
    if err != nil {
        return err
    }

    // Link to job
    job.BillingID = &tx.ID
    job.ActualCost = cost

    // Update job
    return s.updateJob(ctx, job)
}

func (s *Service) ChargeInference(ctx context.Context, modelID uuid.UUID, requestCount int64) error {
    // Charge per 1000 requests
    model, err := s.loadModel(ctx, modelID)
    if err != nil {
        return err
    }

    cost := float64(requestCount) / 1000.0 * model.InferenceCostPer1k

    return s.CreateTransaction(ctx, model.UserID, cost, "USD", ProviderAfterDark, "inference")
}

func (s *Service) ChargeStorage(ctx context.Context, userID uuid.UUID, month string) error {
    // Calculate monthly storage costs
    // datasets: $0.10/GB/month
    // models: $0.15/GB/month
}
```

## 🎯 MVP Feature Set (2 Weeks)

### Must-Have Features:
1. ✅ Database schema
2. ✅ Storage client
3. [ ] Dataset upload API
4. [ ] Training job submission
5. [ ] GPU rental (Vast.ai integration)
6. [ ] Training monitoring
7. [ ] Model registration
8. [ ] ONNX inference
9. [ ] Basic billing

### Nice-to-Have (Later):
- Model versioning
- A/B testing
- Auto-scaling
- Spot instance training
- Distributed training
- Web dashboard
- CLI tool

## 💰 Revenue Projections

**Conservative Estimates:**

**Assumptions:**
- 100 active users
- Average 10 training jobs/month per user
- Average 1 hour GPU time per job
- Average 100K inference requests/month per user

**Monthly Revenue:**
```
Training:   100 users × 10 jobs × 1 hour × $1.00/hr = $1,000
Storage:    100 users × 5GB × $0.10/GB            = $50
Inference:  100 users × 100K requests × $0.10/1K  = $1,000
                                         TOTAL:    $2,050/month
```

**At Scale (1,000 users):**
```
Training:   $10,000/month
Storage:    $500/month
Inference:  $10,000/month
            TOTAL: $20,500/month
```

**Profit Margins:**
- Training: 30% margin = $6,150/month at 1K users
- Storage: 50% margin = $250/month
- Inference: 60% margin = $6,000/month
- **Total Profit: $12,400/month at 1,000 users**

## 🔧 Dependencies to Add

```bash
# Storage (S3)
go get github.com/aws/aws-sdk-go-v2/aws
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/service/s3

# Job Queue (Redis)
go get github.com/redis/go-redis/v9

# ONNX Runtime
go get github.com/yalue/onnxruntime_go

# SSH (for GPU instance management)
go get golang.org/x/crypto/ssh
```

## 📊 Monitoring Dashboard Ideas

**User Dashboard Sections:**
1. **Overview**
   - Current month spending
   - Active training jobs
   - Deployed models
   - Storage usage

2. **Datasets**
   - Upload dataset
   - Manage datasets
   - View dataset files

3. **Training**
   - Submit new job
   - Monitor running jobs
   - View training history
   - Download trained models

4. **Models**
   - Model registry
   - Deploy model
   - Test inference
   - View metrics

5. **Billing**
   - Usage breakdown
   - Cost estimates
   - Payment methods
   - Billing history

## 🚀 Next Steps

1. **Run migrations** - Add training platform tables
2. **Test storage** - Verify darkstorage.io integration
3. **Build dataset API** - Upload/list/delete datasets
4. **Implement job queue** - Redis-based training queue
5. **GPU integration** - Connect to Vast.ai/io.net
6. **ONNX runtime** - Add inference support
7. **Billing integration** - Track and charge usage
8. **Build dashboard** - Simple web UI

## 💡 Marketing Angle

**Positioning:** "Train Your AI Models for 1/10th the Cost of AWS"

**Key Selling Points:**
- 🚀 **70% cheaper than AWS SageMaker** (rented GPUs)
- ⚡ **5-minute setup** - Upload dataset, submit job, get trained model
- 💰 **Pay-as-you-go** - No minimum spend, no complex pricing
- 🔐 **Private & secure** - Your data, your models, your darkstorage
- 🌐 **Any framework** - PyTorch, TensorFlow, scikit-learn via ONNX
- 📊 **Real-time monitoring** - Track training progress, costs, metrics

**Target Customers:**
- Solo AI engineers building side projects
- Startups running MVP experiments
- Researchers with limited budgets
- Enterprises optimizing AI costs

Ready to build this? Let's start with the dataset upload API! 🚀
