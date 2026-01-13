# GPU Cloud Provider Helpers

Unified client libraries for managing GPU compute across multiple cloud providers.

## Provider Categories

### Budget CSPs (Cost-Optimized)
- **Vast.ai** - Spot GPU instances, lowest cost
- **IO.net** - Decentralized GPU network

### Major CSPs (Enterprise-Grade)
- **AWS** - SageMaker & EC2 GPU instances (p5, p4, g5, g4)
- **Oracle Cloud** - OCI GPU instances (A100, V100, A10)

## Usage

### Quick Start with CSP Manager

```go
package main

import (
    "context"
    "fmt"
    "github.com/aiserve/gpuproxy/helpers/common"
    "github.com/aiserve/gpuproxy/helpers/manager"
)

func main() {
    // Create unified manager
    mgr := manager.NewCSPManager(
        "vastai-api-key",
        "ionet-api-key",
        "us-east-1", "aws-key", "aws-secret", "",
        "oracle-tenancy", "oracle-user", "fingerprint", "private-key", "us-ashburn-1", "compartment-id",
    )

    // Prefer budget CSPs (97% cost savings)
    mgr.SetPreference(true)

    ctx := context.Background()

    // Reserve cheapest available H100
    instance, err := mgr.Reserve(ctx, common.ReservationRequest{
        PreferBudget:   true,
        AllowFallback:  true,
        GPUModel:       "H100",
        GPUCount:       1,
        MaxCostPerHour: 3.50,
        Image:          "pytorch/pytorch:latest",
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("Reserved %s GPU on %s for $%.2f/hr\n",
        instance.GPUModel, instance.Provider, instance.CostPerHour)

    // ... use the instance ...

    // Release when done
    err = mgr.Release(ctx, instance.Provider, instance.ID)
}
```

### Individual Provider Clients

#### Vast.ai (Budget CSP)

```go
import "github.com/aiserve/gpuproxy/helpers/vastai"

client := vastai.NewClient("your-api-key")

// List available GPUs
instances, err := client.List(ctx, common.ListOptions{
    GPUModel:     "H100",
    MaxCostPerHr: 2.50,
    Limit:        10,
})

// Reserve cheapest
instance, err := client.Reserve(ctx, common.ReservationRequest{
    GPUModel:       "H100",
    GPUCount:       1,
    MaxCostPerHour: 2.50,
    Image:          "nvidia/cuda:12.2.0-runtime-ubuntu22.04",
})

// Check status
status, err := client.Status(ctx, instance.ID)

// Release
err = client.Release(ctx, instance.ID)
```

#### IO.net (Budget CSP)

```go
import "github.com/aiserve/gpuproxy/helpers/ionet"

client := ionet.NewClient("your-api-key")

// List available devices
devices, err := client.List(ctx, common.ListOptions{
    GPUModel: "A100",
    Region:   "us-east",
})

// Reserve for 24 hours
instance, err := client.Reserve(ctx, common.ReservationRequest{
    GPUModel:   "A100",
    GPUCount:   4,
    Duration:   24 * time.Hour,
    Image:      "tensorflow/tensorflow:latest-gpu",
})

// Release
err = client.Release(ctx, instance.ID)
```

#### AWS (Major CSP)

```go
import "github.com/aiserve/gpuproxy/helpers/aws"

client := aws.NewClient("us-east-1", "access-key", "secret-key", "")

// List GPU instances (p5, p4, g5, g4)
instances, err := client.MockList(ctx, common.ListOptions{
    GPUModel: "H100",
    Region:   "us-east-1",
})

// Get pricing for instance type
price, err := client.GetPricing("p5.48xlarge") // $98.32/hr

// Available instance types
types := client.GetInstanceTypes()
// p5.48xlarge (8x H100 80GB)
// p4d.24xlarge (8x A100 40GB)
// g5.48xlarge (8x A10G 24GB)
```

#### Oracle Cloud (Major CSP)

```go
import "github.com/aiserve/gpuproxy/helpers/oracle"

client := oracle.NewClient(
    "tenancy-ocid",
    "user-ocid",
    "fingerprint",
    "private-key",
    "us-ashburn-1",
    "compartment-ocid",
)

// List GPU shapes
instances, err := client.MockList(ctx, common.ListOptions{
    GPUModel: "A100",
})

// Get pricing for shape
price, err := client.GetPricing("BM.GPU.A100-v2.8") // $29.60/hr

// Available shapes
shapes := client.GetShapes()
// BM.GPU.A100-v2.8 (8x A100 40GB)
// BM.GPU4.8 (8x V100 32GB)
// BM.GPU.A10.4 (4x A10 24GB)
```

## Cost Comparison

| Provider | GPU | Cost/Hour | Use Case |
|----------|-----|-----------|----------|
| Vast.ai | H100 80GB | $1.50-$2.50 | Development, training |
| IO.net | A100 40GB | $0.80-$1.50 | Batch inference |
| AWS p5 | H100 80GB | $98.32 | Production (8x GPUs) |
| AWS g5 | A10G 24GB | $1.00-$5.67 | Cost-optimized ML |
| Oracle | A100 40GB | $29.60 | Enterprise (8x GPUs) |

**Cost Savings**: Budget CSPs are **60-95% cheaper** than major CSPs

## Supported GPUs

### Budget CSPs (Vast.ai, IO.net)
- H100 80GB - Latest flagship
- A100 40GB/80GB - ML workloads
- A40 48GB - Professional visualization
- RTX 4090 24GB - Consumer flagship
- RTX 3090 24GB - Previous gen

### Major CSPs (AWS, Oracle)
- H100 80GB (AWS p5, Oracle GM4)
- A100 40GB/80GB (AWS p4, Oracle A100-v2)
- V100 32GB (AWS p3, Oracle GPU4)
- A10G 24GB (AWS g5, Oracle A10)
- T4 16GB (AWS g4)

## Architecture Integration

These helpers integrate with the hybrid compute architecture:

```
┌─────────────────────────────────────────────────┐
│          CSP Manager (Unified Interface)        │
├─────────────────────────────────────────────────┤
│  Budget CSPs              │  Major CSPs         │
│  ├─ Vast.ai (500 max)     │  ├─ AWS (250 max)   │
│  └─ IO.net (500 max)      │  └─ Oracle (250 max)│
├─────────────────────────────────────────────────┤
│  Total Capacity: 1,000 GPUs + 200 TPUs         │
└─────────────────────────────────────────────────┘
```

### Port Allocation
- **2000-2500**: OpenRouter (Claude/GPT) - Teacher models
- **3000-5000**: GoLearn models (2k ports)
- **5001-8000**: GoMLX models (3k ports)
- **8001-11000**: Classic ML (3k ports)
- **11001-15000**: ONNX/PyTorch/TF (4k ports)

## Configuration

### Environment Variables

```bash
# Budget CSPs
export VASTAI_API_KEY="your-vastai-key"
export IONET_API_KEY="your-ionet-key"

# AWS
export AWS_REGION="us-east-1"
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"

# Oracle Cloud
export OCI_TENANCY_OCID="ocid1.tenancy.oc1..."
export OCI_USER_OCID="ocid1.user.oc1..."
export OCI_FINGERPRINT="aa:bb:cc:..."
export OCI_PRIVATE_KEY_PATH="/path/to/key.pem"
export OCI_REGION="us-ashburn-1"
export OCI_COMPARTMENT_OCID="ocid1.compartment.oc1..."
```

### Configuration File (TOML)

```toml
[compute.budget_csps]
vastai_api_key = "your-key"
ionet_api_key = "your-key"
max_vastai_gpus = 500
max_ionet_gpus = 500
prefer_budget = true

[compute.major_csps]
aws_region = "us-east-1"
aws_access_key = "your-key"
aws_secret_key = "your-secret"
oracle_region = "us-ashburn-1"
max_aws_gpus = 250
max_oracle_gpus = 250

[compute.preferences]
prefer_budget = true           # Try budget CSPs first
allow_fallback = true          # Fall back to major CSPs
max_cost_per_hour = 5.0       # Max $5/hr per GPU
auto_scale = true             # Auto-provision when needed
```

## CLI Tools

### List GPUs Across All Providers

```bash
go run helpers/cli/list.go --gpu H100 --max-cost 3.0
```

### Reserve GPU (Auto-Select Cheapest)

```bash
go run helpers/cli/reserve.go \
  --gpu H100 \
  --count 1 \
  --max-cost 2.50 \
  --prefer-budget \
  --image pytorch/pytorch:latest
```

### Check Instance Status

```bash
go run helpers/cli/status.go --provider vastai --id 12345
```

### Release Instance

```bash
go run helpers/cli/release.go --provider vastai --id 12345
```

## API Endpoints

The helpers integrate with the main GPU proxy API:

```bash
# List all available GPUs
GET /api/v1/compute/list?gpu=H100&max_cost=3.0

# Reserve GPU (auto-select provider)
POST /api/v1/compute/reserve
{
  "gpu_model": "H100",
  "gpu_count": 1,
  "max_cost_per_hour": 2.50,
  "prefer_budget": true,
  "allow_fallback": true,
  "image": "pytorch/pytorch:latest"
}

# Get instance status
GET /api/v1/compute/instances/{id}

# Release instance
DELETE /api/v1/compute/instances/{id}

# Get CSP statistics
GET /api/v1/compute/stats
```

## Testing

Mock data is available for testing without real API keys:

```go
// Use mock data for major CSPs
awsInstances, _ := awsClient.MockList(ctx, opts)
oracleInstances, _ := oracleClient.MockList(ctx, opts)
```

## Development Status

| Provider | Status | Implementation |
|----------|--------|----------------|
| Vast.ai | ✅ Ready | Full API integration |
| IO.net | ✅ Ready | Full API integration |
| AWS | ⚠️ Partial | Mock data only (SDK pending) |
| Oracle | ⚠️ Partial | Mock data only (SDK pending) |

## Next Steps

1. **AWS SDK Integration**: Implement actual EC2/SageMaker API calls
2. **Oracle SDK Integration**: Implement actual OCI compute API calls
3. **CLI Tools**: Build standalone CLI utilities
4. **Monitoring**: Add Prometheus metrics for reservation tracking
5. **Auto-Scaling**: Implement automatic GPU provisioning based on demand

## License

Part of aiserve-gpuproxyd - GPU Proxy Daemon for aiserve.farm platform
