# GPU Reservations

The GPU Proxy supports advanced GPU reservation capabilities via both HTTP and gRPC APIs. This feature allows you to reserve multiple GPUs at once with automatic instance creation and intelligent load balancing.

## Overview

GPU reservations provide:
- **Bulk Operations**: Reserve 1-16 GPUs in a single request
- **Automatic Creation**: Instances are created automatically
- **Smart Selection**: Load balancer chooses optimal instances
- **Filtering**: Filter by VRAM, price, provider, and more
- **Partial Success**: Continue even if some instances fail
- **Contract Tracking**: Contract IDs returned in metadata

## Quick Start

### Via gRPC

```go
package main

import (
    "context"
    pb "github.com/aiserve/gpuproxy/proto"
    "google.golang.org/grpc"
)

func main() {
    conn, _ := grpc.Dial("localhost:9090", grpc.WithInsecure())
    client := pb.NewGPUProxyServiceClient(conn)

    // Login and get token (see examples for full auth flow)
    ctx := /* authenticated context */

    // Reserve 4 GPUs
    resp, err := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
        Count:    4,
        Provider: "vast.ai",
        MinVram:  16,
        MaxPrice: 2.0,
    })

    // Use reserved GPUs
    for _, gpu := range resp.ReservedInstances {
        contractID := gpu.Metadata["contract_id"]
        // Work with GPU...
    }
}
```

### Via HTTP

```bash
curl -X POST http://localhost:8080/api/v1/gpu/instances/reserve \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "count": 4,
    "filters": {
      "min_vram": 16,
      "max_price": 2.0,
      "provider": "vast.ai"
    }
  }'
```

## How It Works

### Reservation Flow

1. **List Instances**: Query available GPUs from providers
2. **Apply Filters**: Filter by your criteria (VRAM, price, etc.)
3. **Check Availability**: Verify enough instances exist
4. **Select Instances**: Use load balancer to pick optimal GPUs
5. **Create Instances**: Automatically create each selected instance
6. **Track Connections**: Update load balancer state
7. **Return Results**: Provide instance details and contract IDs

### Load Balancer Integration

The reservation system integrates with the load balancer to:
- **Select optimal instances** based on current strategy
- **Track connections** for future balancing decisions
- **Distribute load** across multiple providers
- **Prefer best performance** or lowest cost

#### Available Strategies

Set via gRPC:
```go
client.SetLoadBalancerStrategy(ctx, &pb.SetLoadBalancerStrategyRequest{
    Strategy: "least_connections",
})
```

Or via HTTP:
```bash
curl -X PUT http://localhost:8080/api/v1/loadbalancer/strategy \
  -H "X-API-Key: YOUR_API_KEY" \
  -d '{"strategy": "least_connections"}'
```

**Strategies**:
- `round_robin` - Even distribution
- `equal_weighted` - Balance by total connections
- `weighted_round_robin` - Prioritize by specs
- `least_connections` - Route to least busy (default)
- `least_response_time` - Route to fastest

## Filtering Options

### VRAM (GPU Memory)

Minimum VRAM in GB:
```go
req := &pb.ReserveGPUsRequest{
    MinVram: 40,  // A100 or better
}
```

Common values:
- `8` - RTX 3060 Ti
- `12` - RTX 3080 Ti
- `16` - RTX 4080
- `24` - RTX 3090, RTX 4090
- `40` - A100 40GB
- `80` - A100 80GB, H100

### Price

Maximum price per hour in USD:
```go
req := &pb.ReserveGPUsRequest{
    MaxPrice: 2.50,  // Max $2.50/hour
}
```

### Provider

Specific provider or all:
```go
req := &pb.ReserveGPUsRequest{
    Provider: "vast.ai",  // or "io.net" or "" for all
}
```

### Combined Filters

```go
req := &pb.ReserveGPUsRequest{
    Count:    8,
    Provider: "vast.ai",
    MinVram:  24,      // RTX 3090 or better
    MaxPrice: 1.50,    // Under $1.50/hour
}
```

## Response Format

### Success Response

```go
type ReserveGPUsResponse struct {
    ReservedInstances []*GPUInstance  // Details of reserved GPUs
    ReservedCount     int32            // Number successfully reserved
    Message           string           // Status message
}

type GPUInstance struct {
    Id            string              // Instance ID
    Provider      string              // vast.ai or io.net
    Status        string              // "reserved"
    PricePerHour  float64            // $/hour
    VramGb        int32              // VRAM in GB
    GpuModel      string             // e.g., "RTX 4090"
    NumGpus       int32              // Number of GPUs
    Location      string             // Datacenter location
    Metadata      map[string]string  // Includes contract_id
}
```

### Metadata Fields

- `contract_id` - Provider's contract/instance ID for management
- Additional provider-specific fields

## Error Handling

### gRPC Status Codes

```go
resp, err := client.ReserveGPUs(ctx, req)
if err != nil {
    st, _ := status.FromError(err)
    switch st.Code() {
    case codes.InvalidArgument:
        // Count not between 1-16
    case codes.FailedPrecondition:
        // Not enough instances available
    case codes.Internal:
        // Instance creation failed
    }
}
```

### HTTP Status Codes

- `201 Created` - Success
- `400 Bad Request` - Invalid count or not enough instances
- `500 Internal Server Error` - Creation failed

### Partial Success

Reservations continue even if some instances fail:

```go
resp, err := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
    Count: 10,
})

// Check if all succeeded
if resp.ReservedCount < 10 {
    log.Printf("Only reserved %d of 10 GPUs", resp.ReservedCount)
    log.Printf("Message: %s", resp.Message)
}

// Still use the ones that succeeded
for _, gpu := range resp.ReservedInstances {
    // Work with GPU...
}
```

## Cost Estimation

### Per-Hour Cost

```go
totalCost := 0.0
for _, gpu := range resp.ReservedInstances {
    totalCost += gpu.PricePerHour
}
fmt.Printf("Total: $%.2f/hour\n", totalCost)
```

### Daily/Weekly Estimates

```go
dailyCost := totalCost * 24
weeklyCost := totalCost * 24 * 7
monthlyCost := totalCost * 24 * 30

fmt.Printf("Daily: $%.2f\n", dailyCost)
fmt.Printf("Weekly: $%.2f\n", weeklyCost)
fmt.Printf("Monthly: $%.2f\n", monthlyCost)
```

## Best Practices

### 1. Check Availability First

```go
// List to see what's available
instances, _ := client.ListGPUInstances(ctx, &pb.ListGPUInstancesRequest{
    Provider: "all",
    MinVram:  24,
    MaxPrice: 2.0,
})

if instances.TotalCount < desiredCount {
    // Not enough available
    return
}

// Now reserve
resp, _ := client.ReserveGPUs(ctx, req)
```

### 2. Use Appropriate Filters

```go
// Too broad - may get expensive GPUs
req := &pb.ReserveGPUsRequest{
    Count: 10,
}

// Better - specify requirements
req := &pb.ReserveGPUsRequest{
    Count:    10,
    MinVram:  16,
    MaxPrice: 1.50,
    Provider: "vast.ai",
}
```

### 3. Handle Partial Success

```go
resp, err := client.ReserveGPUs(ctx, req)
if err != nil {
    return err
}

successCount := resp.ReservedCount
if successCount < req.Count {
    // Decide: use partial results or retry?
    if successCount == 0 {
        return errors.New("no GPUs reserved")
    }
    log.Printf("Partial success: %d/%d", successCount, req.Count)
}

// Continue with what we got
return processGPUs(resp.ReservedInstances)
```

### 4. Store Contract IDs

```go
type Reservation struct {
    InstanceID string
    ContractID string
    Provider   string
    CreatedAt  time.Time
}

reservations := []Reservation{}
for _, gpu := range resp.ReservedInstances {
    reservations = append(reservations, Reservation{
        InstanceID: gpu.Id,
        ContractID: gpu.Metadata["contract_id"],
        Provider:   gpu.Provider,
        CreatedAt:  time.Now(),
    })
}

// Store in database for later cleanup
db.SaveReservations(reservations)
```

### 5. Clean Up When Done

```go
// Destroy instances when no longer needed
for _, reservation := range reservations {
    client.DestroyGPUInstance(ctx, &pb.DestroyGPUInstanceRequest{
        Provider:   reservation.Provider,
        InstanceId: reservation.ContractID,
    })
}
```

## Use Cases

### Machine Learning Training

Reserve multiple GPUs for distributed training:

```go
// Reserve 8x A100 GPUs for training
resp, _ := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
    Count:    8,
    MinVram:  40,  // A100
    MaxPrice: 3.0,
    Provider: "vast.ai",
})

// Configure distributed training across GPUs
for i, gpu := range resp.ReservedInstances {
    rank := i
    // Set up training on GPU at rank i
}
```

### Batch Inference

Reserve GPUs for parallel inference:

```go
// Reserve 4 GPUs for batch processing
resp, _ := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
    Count:    4,
    MinVram:  24,
    MaxPrice: 1.50,
})

// Distribute inference batches
batches := splitWork(dataset, len(resp.ReservedInstances))
for i, gpu := range resp.ReservedInstances {
    go processInference(gpu, batches[i])
}
```

### Development/Testing

Reserve cost-effective GPUs for development:

```go
// Reserve 1-2 cheaper GPUs for testing
resp, _ := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
    Count:    1,
    MinVram:  8,
    MaxPrice: 0.50,  // Under $0.50/hour
})
```

## Monitoring

### Check Reservation Status

```go
// Get load balancer info
loadInfo, _ := client.GetLoadInfo(ctx, &pb.GetLoadInfoRequest{
    Type: "all",
})

fmt.Printf("Strategy: %s\n", loadInfo.CurrentStrategy)
fmt.Printf("Active connections: %d\n", len(loadInfo.ServerLoad))

for _, instance := range loadInfo.ServerLoad {
    fmt.Printf("  %s: %d connections, %.2fms avg\n",
        instance.InstanceId,
        instance.Connections,
        instance.AvgResponseTimeMs)
}
```

### View Costs

```go
// Get recent transactions
txns, _ := client.GetTransactions(ctx, &pb.GetTransactionsRequest{
    Limit: 10,
})

totalSpent := 0.0
for _, txn := range txns.Transactions {
    if txn.Status == "completed" {
        totalSpent += txn.Amount
    }
}
fmt.Printf("Recent spending: $%.2f\n", totalSpent)
```

## Limits

- **Maximum GPUs**: 16 per request
- **Minimum GPUs**: 1 per request
- **Concurrent Reservations**: Unlimited (subject to provider limits)
- **Provider Limits**: Varies by provider and account tier

## Advanced Features

### Per-Provider API Keys

The system supports multiple API keys for each provider, allowing:
- Increased rate limits
- Multi-account support
- Failover between accounts
- Region-specific keys

Configure in `.env`:
```env
VASTAI_API_KEY=key1,key2,key3
IONET_API_KEY=key1,key2
```

### Priority Models

Coming soon - reserve GPUs with priority levels:
- `high` - Guaranteed allocation
- `medium` - Best-effort allocation
- `low` - Use spare capacity only

### GPU Locking

Coming soon - lock GPUs for exclusive access:
- Prevent other users from accessing
- Guaranteed resources
- Premium pricing

## See Also

- [gRPC API Documentation](GRPC.md)
- [Load Balancing Guide](LOADBALANCING.md)
- [Examples](examples/README.md)
- [Main README](README.md)
