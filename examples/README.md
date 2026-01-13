# gRPC Examples

This directory contains example code demonstrating how to use the GPU Proxy gRPC API.

## Prerequisites

1. **Running Server**: Ensure aiserve-gpuproxyd is running with gRPC enabled
2. **Credentials**: You need valid login credentials
3. **Go Dependencies**: Install required packages

```bash
go mod download
```

## Examples

### GPU Reservation Example

**File**: `grpc_reserve_gpus.go`

This example demonstrates the complete workflow for reserving GPUs via gRPC:
1. Login and get JWT token
2. List available GPU instances with filters
3. Reserve multiple GPUs with automatic creation
4. Check load balancer status
5. View transaction history

**Usage**:

```bash
# Set credentials
export GPU_EMAIL="user@example.com"
export GPU_PASSWORD="your-password"

# Optional: Set server address (defaults to localhost:9090)
export GPU_GRPC_ADDR="localhost:9090"

# Run the example
go run examples/grpc_reserve_gpus.go
```

**Expected Output**:

```
=== Logging in ===
Logged in as: user@example.com
User ID: 123e4567-e89b-12d3-a456-426614174000

=== Listing Available GPUs ===
Found 42 instances matching criteria:
  1. vast.ai - RTX 4090 ($1.20/hr, 24GB VRAM, US-West)
  2. vast.ai - RTX 3090 ($0.80/hr, 24GB VRAM, EU-Central)
  3. io.net - A100 ($2.50/hr, 40GB VRAM, US-East)
  ... and 39 more

=== Reserving 2 GPUs ===
Successfully reserved 2 GPU(s)
Successfully reserved 2 GPUs:

GPU 1:
  Instance ID:  vast-12345
  Provider:     vast.ai
  GPU Model:    RTX 4090
  VRAM:         24GB
  GPUs:         1
  Location:     US-West
  Price/Hour:   $1.20
  Status:       reserved
  Contract ID:  contract-abc123

GPU 2:
  Instance ID:  vast-67890
  Provider:     vast.ai
  GPU Model:    RTX 3090
  VRAM:         24GB
  GPUs:         1
  Location:     EU-Central
  Price/Hour:   $0.80
  Status:       reserved
  Contract ID:  contract-def456

Total cost: $2.00/hour
Estimated daily cost: $48.00
Estimated weekly cost: $336.00

=== Load Balancer Status ===
Current strategy: least_connections
Tracked instances: 2

=== Recent Transactions ===
Showing 5 of 12 total transactions:
  1. completed - $50.00 USD (txn-id-1)
  2. completed - $25.00 USD (txn-id-2)
  3. pending - $2.00 USD (txn-id-3)
  4. completed - $100.00 USD (txn-id-4)
  5. completed - $30.00 USD (txn-id-5)

=== Reservation Complete ===
Your GPUs are now reserved and ready to use!
Total reserved: 2 GPUs
Total cost: $2.00/hour
```

## Features Demonstrated

### Authentication
- Login with email/password
- JWT token generation
- Using tokens in gRPC metadata

### GPU Management
- Listing instances with filters (VRAM, price)
- Reserving multiple GPUs at once
- Automatic instance creation
- Load balancer integration

### Billing & Monitoring
- Transaction history retrieval
- Cost calculation and estimation
- Load balancer status checking

## Advanced Usage

### Custom Filters

Reserve specific GPU types:

```go
reserved, err := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
    Count:    8,
    Provider: "vast.ai",
    MinVram:  40,      // A100 or better
    MaxPrice: 3.0,     // Max $3/hour
})
```

### Error Handling

```go
reserved, err := client.ReserveGPUs(ctx, req)
if err != nil {
    if st, ok := status.FromError(err); ok {
        switch st.Code() {
        case codes.InvalidArgument:
            log.Printf("Invalid request: %s", st.Message())
        case codes.FailedPrecondition:
            log.Printf("Not enough GPUs available: %s", st.Message())
        case codes.Internal:
            log.Printf("Server error: %s", st.Message())
        default:
            log.Printf("Unknown error: %s", st.Message())
        }
    }
    return
}
```

### Using API Keys Instead of JWT

```go
ctx := metadata.AppendToOutgoingContext(
    context.Background(),
    "x-api-key", "your-api-key-here",
)

instances, err := client.ListGPUInstances(ctx, req)
```

### Partial Success Handling

```go
reserved, err := client.ReserveGPUs(ctx, req)
if err != nil {
    log.Fatal(err)
}

// Check message for partial failures
if reserved.ReservedCount < req.Count {
    log.Printf("Warning: Only reserved %d of %d requested GPUs",
        reserved.ReservedCount, req.Count)
    log.Printf("Message: %s", reserved.Message)
}

// Still process successfully reserved instances
for _, inst := range reserved.ReservedInstances {
    // Use the GPU...
}
```

## Provider-Specific Notes

### vast.ai
- Instance IDs prefixed with "vast-"
- Supports on-demand and spot instances
- Automatic datacenter selection

### io.net
- Instance IDs prefixed with "ionet-"
- Enterprise-grade reliability
- Global distribution

### Multi-Provider (provider="")
- Searches across all providers
- Uses load balancer for optimal selection
- Best price/performance ratio

## Best Practices

1. **Always check reserved count**: Handle partial success cases
2. **Store contract IDs**: Available in instance metadata
3. **Monitor costs**: Calculate estimated costs before reserving
4. **Use filters**: Specify exact requirements to avoid over-provisioning
5. **Check load balancer**: Verify strategy matches your use case
6. **Handle errors gracefully**: Check gRPC status codes

## Troubleshooting

### Connection Refused
```bash
# Check if gRPC server is running
netstat -an | grep 9090

# Or use grpcurl
grpcurl -plaintext localhost:9090 list
```

### Authentication Failed
```bash
# Verify credentials
echo $GPU_EMAIL
echo $GPU_PASSWORD

# Test login
grpcurl -plaintext \
  -d '{"email":"'$GPU_EMAIL'","password":"'$GPU_PASSWORD'"}' \
  localhost:9090 gpuproxy.GPUProxyService/Login
```

### Not Enough GPUs
```bash
# Check available instances first
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{"provider":"all","min_vram":16}' \
  localhost:9090 gpuproxy.GPUProxyService/ListGPUInstances
```

## Additional Resources

- [gRPC API Documentation](../GRPC.md)
- [Protocol Buffer Definitions](../proto/gpuproxy.proto)
- [Main README](../README.md)
- [Load Balancing Guide](../LOADBALANCING.md)
