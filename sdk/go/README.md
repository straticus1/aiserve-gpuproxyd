# AIServe.Farm Go SDK

Official Go client library for AIServe.Farm API.

## Installation

```bash
go get github.com/straticus1/aiserve-gpuproxyd/sdk/go/aiserve
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/straticus1/aiserve-gpuproxyd/sdk/go/aiserve"
)

func main() {
    // Create client
    client := aiserve.NewClient(&aiserve.Config{
        BaseURL: "https://api.aiserve.farm",
        APIKey:  "your-api-key",
    })

    // List GPU instances
    instances, err := client.GPU.ListInstances(context.Background(), &aiserve.ListInstancesOptions{
        Provider: "vastai",
        MinVRAM:  16,
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d instances\n", len(instances))
}
```

## Documentation

See [API_REFERENCE.md](../../docs/API_REFERENCE.md) for complete API documentation.

## Examples

### Authentication

```go
// Login with email/password
client := aiserve.NewClient(&aiserve.Config{
    BaseURL: "https://api.aiserve.farm",
})

tokens, err := client.Auth.Login(ctx, "user@example.com", "password")
if err != nil {
    log.Fatal(err)
}

// Use JWT token
client.SetToken(tokens.AccessToken)

// Or create API key for long-lived access
apiKey, err := client.Auth.CreateAPIKey(ctx, &aiserve.CreateAPIKeyRequest{
    Name:      "production",
    ExpiresAt: time.Now().AddDate(1, 0, 0),
})
```

### GPU Management

```go
// List available GPUs
instances, err := client.GPU.ListInstances(ctx, &aiserve.ListInstancesOptions{
    Provider:  "all",
    MinVRAM:    24,
    MaxPrice:   2.5,
    GPUModel:   "RTX 4090",
    Location:   "US",
})

// Create single instance
contract, err := client.GPU.CreateInstance(ctx, "vastai", "instance_123", &aiserve.InstanceConfig{
    DurationHours: 4,
    AutoRenew:     false,
})

// Reserve multiple instances with load balancing
reservation, err := client.GPU.ReserveInstances(ctx, &aiserve.ReserveRequest{
    Count: 4,
    Filters: &aiserve.GPUFilters{
        MinVRAM:   24,
        GPUModel:  "RTX 4090",
        Location:  "US",
    },
})

// Destroy instance
err = client.GPU.DestroyInstance(ctx, "vastai", "instance_123")
```

### Model Serving

```go
// Upload model
file, _ := os.Open("model.onnx")
defer file.Close()

model, err := client.Models.Upload(ctx, &aiserve.UploadModelRequest{
    File:        file,
    Name:        "my_model",
    Format:      "onnx",
    GPURequired: true,
})

// List models
models, err := client.Models.List(ctx)

// Run inference
result, err := client.Models.Predict(ctx, model.ID, &aiserve.PredictRequest{
    Inputs: map[string]interface{}{
        "features": []float64{1.0, 2.0, 3.0, 4.0},
    },
})

fmt.Printf("Prediction: %v (latency: %.2fms)\n", result.Outputs, result.LatencyMs)

// Delete model
err = client.Models.Delete(ctx, model.ID)
```

### Billing & Guardrails

```go
// Check spending status
spending, err := client.Guardrails.GetSpending(ctx)
fmt.Printf("Spent: $%.2f / $%.2f\n", spending.WindowSpent, spending.WindowLimit)

// Check if operation is allowed
allowed, err := client.Guardrails.CheckSpending(ctx, 50.00)
if !allowed {
    log.Fatal("Spending limit would be exceeded")
}

// Record spending
err = client.Guardrails.RecordSpending(ctx, 25.50)

// Get transaction history
transactions, err := client.Billing.GetTransactions(ctx)
```

### Load Balancing

```go
// Get current strategy
strategy, err := client.LoadBalancer.GetStrategy(ctx)

// Set strategy
err = client.LoadBalancer.SetStrategy(ctx, aiserve.StrategyLeastConnections)

// Get instance loads
loads, err := client.LoadBalancer.GetLoads(ctx)
for instanceID, load := range loads.Loads {
    fmt.Printf("%s: %d connections (%.2f load)\n",
        instanceID, load.Connections, load.Load)
}
```

### Storage Quotas

```go
// Check quota status
quota, err := client.Quota.Get(ctx)
fmt.Printf("Storage: %.1f%% used\n", quota.Storage.UsedPct)
fmt.Printf("Uploads today: %d/%d\n",
    quota.RateLimits.UploadsLastDay,
    quota.RateLimits.DailyLimit)
```

## API Reference

### Client

```go
type Client struct {
    Auth         *AuthService
    GPU          *GPUService
    Models       *ModelsService
    Billing      *BillingService
    Guardrails   *GuardrailsService
    LoadBalancer *LoadBalancerService
    Quota        *QuotaService
}

func NewClient(config *Config) *Client
func (c *Client) SetToken(token string)
func (c *Client) SetAPIKey(apiKey string)
```

### Configuration

```go
type Config struct {
    BaseURL    string        // API base URL
    APIKey     string        // API key (optional if using JWT)
    HTTPClient *http.Client  // Custom HTTP client (optional)
    Timeout    time.Duration // Request timeout (default: 30s)
}
```

### Error Handling

```go
if err != nil {
    if apiErr, ok := err.(*aiserve.APIError); ok {
        fmt.Printf("API Error: %s (code: %s, status: %d)\n",
            apiErr.Message, apiErr.Code, apiErr.StatusCode)
    }
}
```

## Advanced Usage

### Custom HTTP Client

```go
httpClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:       100,
        IdleConnTimeout:    90 * time.Second,
        DisableCompression: true,
    },
}

client := aiserve.NewClient(&aiserve.Config{
    BaseURL:    "https://api.aiserve.farm",
    APIKey:     "your-api-key",
    HTTPClient: httpClient,
})
```

### Context Usage

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

instances, err := client.GPU.ListInstances(ctx, nil)

// With cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(5 * time.Second)
    cancel() // Cancel after 5 seconds
}()

result, err := client.Models.Predict(ctx, modelID, request)
```

### Streaming Inference

```go
// WebSocket streaming
stream, err := client.Models.StreamPredict(ctx, modelID)
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

// Send input
err = stream.Send(&aiserve.PredictRequest{
    Inputs: inputs,
})

// Receive output
for {
    result, err := stream.Receive()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Received: %v\n", result.Outputs)
}
```

## Testing

```bash
go test ./...
```

## License

MIT License - see LICENSE file for details

## Support

- GitHub Issues: https://github.com/straticus1/aiserve-gpuproxyd/issues
- Documentation: https://aiserve.farm/docs
- Email: support@afterdarksys.com
