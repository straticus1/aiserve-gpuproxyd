# gRPC API Documentation

AIServe GPU Proxy provides a high-performance gRPC API alongside its HTTP/REST API. gRPC offers strongly-typed contracts, bidirectional streaming, and superior performance for service-to-service communication.

## Quick Start

### Server Configuration

Set the gRPC port and optional TLS configuration in your `.env` file:

```env
GRPC_PORT=9090

# Optional: Enable TLS for production (leave empty for development)
GRPC_TLS_CERT=/path/to/server.crt
GRPC_TLS_KEY=/path/to/server.key
```

The gRPC server starts automatically alongside the HTTP server when you run `aiserve-gpuproxyd`.

#### TLS Configuration

For **development**, you can run the gRPC server in insecure mode by leaving `GRPC_TLS_CERT` and `GRPC_TLS_KEY` empty.

For **production**, generate TLS certificates:

```bash
# Self-signed certificate (for testing)
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes

# Let's Encrypt (for production)
certbot certonly --standalone -d your-domain.com
```

Then update `.env`:
```env
GRPC_TLS_CERT=/path/to/server.crt
GRPC_TLS_KEY=/path/to/server.key
```

### Authentication

All gRPC methods (except `Login` and `HealthCheck`) require authentication via metadata:

**Using JWT Token:**
```go
ctx := metadata.AppendToOutgoingContext(
    context.Background(),
    "authorization", "Bearer YOUR_JWT_TOKEN",
)
```

**Using API Key:**
```go
ctx := metadata.AppendToOutgoingContext(
    context.Background(),
    "x-api-key", "YOUR_API_KEY",
)
```

## Protocol Buffers Definition

The complete service definition is available in `proto/gpuproxy.proto`. All messages and services are defined there.

### Regenerating Protocol Buffers

If you modify `proto/gpuproxy.proto`, regenerate the Go code:

```bash
make proto
```

Or manually:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/gpuproxy.proto
```

## Available Services

### Authentication

#### Login
Authenticates a user and returns a JWT token.

**Request:**
```protobuf
message LoginRequest {
  string email = 1;
  string password = 2;
}
```

**Response:**
```protobuf
message LoginResponse {
  string token = 1;
  string user_id = 2;
  string email = 3;
}
```

**Example (Go):**
```go
resp, err := client.Login(ctx, &pb.LoginRequest{
    Email:    "user@example.com",
    Password: "password123",
})
token := resp.Token
```

#### CreateAPIKey
Creates a new API key for the authenticated user.

**Request:**
```protobuf
message CreateAPIKeyRequest {
  string name = 1;
  string token = 2; // JWT token for auth
}
```

**Response:**
```protobuf
message CreateAPIKeyResponse {
  string api_key = 1;
  string name = 2;
  int64 created_at = 3;
}
```

### GPU Management

#### ListGPUInstances
Lists available GPU instances with optional filtering.

**Request:**
```protobuf
message ListGPUInstancesRequest {
  string provider = 1;      // "vast.ai", "io.net", or "all"
  double min_vram = 2;       // Minimum VRAM in GB
  double max_price = 3;      // Maximum price per hour
  string gpu_model = 4;      // GPU model filter
}
```

**Response:**
```protobuf
message ListGPUInstancesResponse {
  repeated GPUInstance instances = 1;
  int32 total_count = 2;
}

message GPUInstance {
  string id = 1;
  string provider = 2;
  string status = 3;
  double price_per_hour = 4;
  int32 vram_gb = 5;
  string gpu_model = 6;
  int32 num_gpus = 7;
  string location = 8;
  map<string, string> metadata = 9;
}
```

**Example (Go):**
```go
resp, err := client.ListGPUInstances(ctx, &pb.ListGPUInstancesRequest{
    Provider: "all",
    MinVram:  16,
    MaxPrice: 2.5,
})

for _, instance := range resp.Instances {
    fmt.Printf("GPU: %s - $%.2f/hr\n", instance.GpuModel, instance.PricePerHour)
}
```

#### CreateGPUInstance
Creates a new GPU instance.

**Request:**
```protobuf
message CreateGPUInstanceRequest {
  string provider = 1;
  string instance_id = 2;
  string image = 3;
  map<string, string> env = 4;
  repeated int32 ports = 5;
}
```

**Example (Go):**
```go
resp, err := client.CreateGPUInstance(ctx, &pb.CreateGPUInstanceRequest{
    Provider:   "vast.ai",
    InstanceId: "12345",
    Image:      "nvidia/cuda:12.0.0-base-ubuntu22.04",
    Env: map[string]string{
        "MODEL": "llama-2-70b",
    },
    Ports: []int32{8080, 8081},
})
```

#### DestroyGPUInstance
Destroys a GPU instance.

**Request:**
```protobuf
message DestroyGPUInstanceRequest {
  string provider = 1;
  string instance_id = 2;
}
```

#### GetGPUInstance
Gets details of a specific GPU instance.

**Request:**
```protobuf
message GetGPUInstanceRequest {
  string provider = 1;
  string instance_id = 2;
}
```

### Proxy Requests

#### ProxyRequest (Unary)
Proxies a single request to a GPU instance.

**Request:**
```protobuf
message ProxyRequestMessage {
  string protocol = 1;      // "http", "https", "mcp", "openinference"
  string target_url = 2;
  string method = 3;        // HTTP method
  map<string, string> headers = 4;
  bytes body = 5;
  int32 timeout = 6;
}
```

**Response:**
```protobuf
message ProxyResponse {
  int32 status_code = 1;
  map<string, string> headers = 2;
  bytes body = 3;
  string error = 4;
}
```

**Example (Go):**
```go
resp, err := client.ProxyRequest(ctx, &pb.ProxyRequestMessage{
    Protocol:  "https",
    TargetUrl: "https://api.openai.com/v1/completions",
    Method:    "POST",
    Headers: map[string]string{
        "Content-Type": "application/json",
    },
    Body: []byte(`{"model":"gpt-4","prompt":"Hello"}`),
    Timeout: 30,
})

fmt.Printf("Status: %d, Body: %s\n", resp.StatusCode, string(resp.Body))
```

#### StreamProxyRequest (Bidirectional Streaming)
Streams multiple requests and responses for real-time inference.

**Example (Go):**
```go
stream, err := client.StreamProxyRequest(ctx)
if err != nil {
    log.Fatal(err)
}

// Send requests
go func() {
    for i := 0; i < 10; i++ {
        stream.Send(&pb.ProxyRequestMessage{
            Protocol:  "https",
            TargetUrl: "https://api.example.com/inference",
            Method:    "POST",
            Body:      []byte(fmt.Sprintf(`{"prompt":"Request %d"}`, i)),
        })
    }
    stream.CloseSend()
}()

// Receive responses
for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Response: %d - %s\n", resp.StatusCode, string(resp.Body))
}
```

### Billing

#### CreatePayment
Creates a new payment.

**Request:**
```protobuf
message CreatePaymentRequest {
  double amount = 1;
  string currency = 2;
  string provider = 3;        // "stripe", "crypto", "afterdark"
  string payment_method = 4;  // e.g., "card:num:expr:ccv"
}
```

#### GetTransactions
Retrieves transaction history.

**Request:**
```protobuf
message GetTransactionsRequest {
  int32 limit = 1;
  int32 offset = 2;
}
```

### Guard Rails

#### GetSpendingInfo
Retrieves spending information across all time windows.

**Response:**
```protobuf
message GetSpendingInfoResponse {
  repeated SpendingWindow windows = 1;
  bool guard_rails_enabled = 2;
  double total_spent_24h = 3;
}

message SpendingWindow {
  string window = 1;      // e.g., "5min", "60min"
  double spent = 2;
  double limit = 3;
  double remaining = 4;
  bool exceeded = 5;
}
```

#### CheckSpendingLimit
Checks if a spending amount is allowed.

**Request:**
```protobuf
message CheckSpendingLimitRequest {
  double amount = 1;
}
```

**Response:**
```protobuf
message CheckSpendingLimitResponse {
  bool allowed = 1;
  string reason = 2;
  repeated SpendingWindow would_exceed = 3;
}
```

### Load Balancing

#### SetLoadBalancerStrategy
Sets the load balancing strategy.

**Request:**
```protobuf
message SetLoadBalancerStrategyRequest {
  string strategy = 1;  // "round_robin", "least_connections", etc.
}
```

**Available Strategies:**
- `round_robin` - Distribute evenly across all GPUs
- `equal_weighted` - Balance based on total connections
- `weighted_round_robin` - Prioritize by GPU specs
- `least_connections` - Route to GPU with fewest connections
- `least_response_time` - Route to fastest GPU

#### GetLoadInfo
Retrieves current load information.

**Request:**
```protobuf
message GetLoadInfoRequest {
  string type = 1;  // "server", "provider", or "all"
}
```

#### ReserveGPUs
Reserves multiple GPUs (1-16) with automatic load balancing and creation.

This method:
1. Lists available instances matching filters
2. Uses load balancer to select optimal instances
3. Creates instances automatically
4. Tracks connections for load balancing
5. Returns contract IDs in metadata

**Request:**
```protobuf
message ReserveGPUsRequest {
  int32 count = 1;        // 1-16 GPUs
  string provider = 2;    // "vast.ai", "io.net", or "" for all
  double min_vram = 3;    // Minimum VRAM in GB
  double max_price = 4;   // Maximum price per hour
}
```

**Response:**
```protobuf
message ReserveGPUsResponse {
  repeated GPUInstance reserved_instances = 1;
  int32 reserved_count = 2;
  string message = 3;
}
```

**Example (Go):**
```go
resp, err := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
    Count:    4,
    Provider: "vast.ai",
    MinVram:  16,
    MaxPrice: 2.0,
})

if err != nil {
    log.Fatal(err)
}

fmt.Printf("Reserved %d GPUs:\n", resp.ReservedCount)
for _, inst := range resp.ReservedInstances {
    contractID := inst.Metadata["contract_id"]
    fmt.Printf("  - %s: %s (Contract: %s)\n",
        inst.GpuModel, inst.Id, contractID)
}
```

**Error Handling:**
- `InvalidArgument` - Count not between 1-16
- `FailedPrecondition` - Not enough instances available
- `Internal` - Instance creation failed

**Notes:**
- Automatically creates instances (no separate create call needed)
- Uses load balancer strategy for optimal selection
- Returns partial success if some instances fail
- Contract IDs stored in instance metadata
- Tracks connections for future load balancing decisions

### Health Check

#### HealthCheck
Performs a health check (no authentication required).

**Response:**
```protobuf
message HealthCheckResponse {
  string status = 1;
  int64 timestamp = 2;
  map<string, string> details = 3;
}
```

## Client Examples

### Go Client (Complete Example)

```go
package main

import (
    "context"
    "log"
    "time"

    pb "github.com/aiserve/gpuproxy/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/metadata"
)

func main() {
    // Connect to server
    conn, err := grpc.Dial(
        "localhost:9090",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    client := pb.NewGPUProxyServiceClient(conn)

    // 1. Login
    loginResp, err := client.Login(context.Background(), &pb.LoginRequest{
        Email:    "user@example.com",
        Password: "password123",
    })
    if err != nil {
        log.Fatalf("Login failed: %v", err)
    }
    log.Printf("Logged in as %s", loginResp.Email)

    // 2. Create authenticated context
    ctx := metadata.AppendToOutgoingContext(
        context.Background(),
        "authorization", "Bearer "+loginResp.Token,
    )

    // 3. List GPU instances
    instances, err := client.ListGPUInstances(ctx, &pb.ListGPUInstancesRequest{
        Provider: "all",
        MinVram:  16,
        MaxPrice: 3.0,
    })
    if err != nil {
        log.Fatalf("Failed to list instances: %v", err)
    }

    log.Printf("Found %d instances:", instances.TotalCount)
    for _, inst := range instances.Instances {
        log.Printf("  - %s: %s ($%.2f/hr, %dGB VRAM)",
            inst.Provider, inst.GpuModel, inst.PricePerHour, inst.VramGb)
    }

    // 4. Create GPU instance
    if len(instances.Instances) > 0 {
        firstInstance := instances.Instances[0]
        createResp, err := client.CreateGPUInstance(ctx, &pb.CreateGPUInstanceRequest{
            Provider:   firstInstance.Provider,
            InstanceId: firstInstance.Id,
            Image:      "nvidia/cuda:12.0.0-base-ubuntu22.04",
        })
        if err != nil {
            log.Printf("Create failed: %v", err)
        } else {
            log.Printf("Created instance: %s", createResp.Message)
        }
    }

    // 5. Check spending
    spending, err := client.GetSpendingInfo(ctx, &pb.GetSpendingInfoRequest{})
    if err != nil {
        log.Printf("Failed to get spending: %v", err)
    } else {
        log.Printf("Total spent (24h): $%.2f", spending.TotalSpent_24H)
        for _, window := range spending.Windows {
            log.Printf("  %s: $%.2f / $%.2f", window.Window, window.Spent, window.Limit)
        }
    }

    // 6. Health check
    health, err := client.HealthCheck(context.Background(), &pb.HealthCheckRequest{})
    if err != nil {
        log.Printf("Health check failed: %v", err)
    } else {
        log.Printf("Server status: %s", health.Status)
    }
}
```

### Python Client

First, generate Python protobuf code:

```bash
python -m grpc_tools.protoc -I./proto \
    --python_out=./python_client \
    --grpc_python_out=./python_client \
    proto/gpuproxy.proto
```

**Example:**

```python
import grpc
from proto import gpuproxy_pb2, gpuproxy_pb2_grpc

def main():
    # Connect to server
    channel = grpc.insecure_channel('localhost:9090')
    stub = gpuproxy_pb2_grpc.GPUProxyServiceStub(channel)

    # Login
    login_response = stub.Login(gpuproxy_pb2.LoginRequest(
        email="user@example.com",
        password="password123"
    ))
    print(f"Logged in: {login_response.email}")

    # Create authenticated metadata
    metadata = [('authorization', f'Bearer {login_response.token}')]

    # List GPU instances
    instances_response = stub.ListGPUInstances(
        gpuproxy_pb2.ListGPUInstancesRequest(
            provider="all",
            min_vram=16,
            max_price=3.0
        ),
        metadata=metadata
    )

    print(f"Found {instances_response.total_count} instances:")
    for instance in instances_response.instances:
        print(f"  - {instance.provider}: {instance.gpu_model} "
              f"(${instance.price_per_hour:.2f}/hr, {instance.vram_gb}GB)")

    # Get spending info
    spending = stub.GetSpendingInfo(
        gpuproxy_pb2.GetSpendingInfoRequest(),
        metadata=metadata
    )
    print(f"Total spent (24h): ${spending.total_spent_24h:.2f}")

if __name__ == '__main__':
    main()
```

### Node.js/TypeScript Client

Install dependencies:

```bash
npm install @grpc/grpc-js @grpc/proto-loader
```

**Example:**

```typescript
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';

const PROTO_PATH = './proto/gpuproxy.proto';

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

const gpuproxy = grpc.loadPackageDefinition(packageDefinition).gpuproxy;

async function main() {
  const client = new gpuproxy.GPUProxyService(
    'localhost:9090',
    grpc.credentials.createInsecure()
  );

  // Login
  const loginPromise = new Promise((resolve, reject) => {
    client.Login(
      { email: 'user@example.com', password: 'password123' },
      (err, response) => {
        if (err) reject(err);
        else resolve(response);
      }
    );
  });

  const loginResp = await loginPromise;
  console.log('Logged in:', loginResp.email);

  // Create metadata with token
  const metadata = new grpc.Metadata();
  metadata.add('authorization', `Bearer ${loginResp.token}`);

  // List instances
  const instancesPromise = new Promise((resolve, reject) => {
    client.ListGPUInstances(
      { provider: 'all', min_vram: 16 },
      metadata,
      (err, response) => {
        if (err) reject(err);
        else resolve(response);
      }
    );
  });

  const instances = await instancesPromise;
  console.log(`Found ${instances.total_count} instances`);
}

main().catch(console.error);
```

## Error Handling

gRPC uses standard status codes. Common errors:

- `UNAUTHENTICATED` (16) - Missing or invalid authentication
- `PERMISSION_DENIED` (7) - Insufficient permissions
- `INVALID_ARGUMENT` (3) - Invalid request parameters
- `NOT_FOUND` (5) - Resource not found
- `INTERNAL` (13) - Internal server error

**Go Example:**

```go
resp, err := client.ListGPUInstances(ctx, req)
if err != nil {
    if st, ok := status.FromError(err); ok {
        switch st.Code() {
        case codes.Unauthenticated:
            log.Println("Authentication required")
        case codes.InvalidArgument:
            log.Printf("Invalid request: %s", st.Message())
        default:
            log.Printf("Error: %s", st.Message())
        }
    }
    return
}
```

## Performance Tips

1. **Connection Pooling**: Reuse gRPC connections across requests
2. **Streaming**: Use streaming RPCs for high-throughput scenarios
3. **Compression**: Enable gRPC compression for large payloads
4. **Timeouts**: Set appropriate timeouts on context

**Example with timeouts:**

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

resp, err := client.ListGPUInstances(ctx, req)
```

## Security

### TLS/SSL Configuration

The gRPC server supports TLS encryption out of the box. Configure via environment variables:

**Server Configuration:**

```env
# In .env file
GRPC_TLS_CERT=/path/to/server.crt
GRPC_TLS_KEY=/path/to/server.key
```

The server will automatically use TLS if both cert and key are provided. Leave empty for insecure development mode.

**Client Connection with TLS:**

For **insecure** connections (development only):

```go
conn, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
```

For **secure** connections with self-signed certificate:

```go
import "google.golang.org/grpc/credentials"

creds, err := credentials.NewClientTLSFromFile("server.crt", "")
if err != nil {
    log.Fatal(err)
}

conn, err := grpc.Dial("gpuproxy.example.com:9090",
    grpc.WithTransportCredentials(creds))
```

For **secure** connections with system CA (Let's Encrypt):

```go
creds := credentials.NewClientTLSFromCert(nil, "")
conn, err := grpc.Dial("gpuproxy.example.com:9090",
    grpc.WithTransportCredentials(creds))
```

**Python Client with TLS:**

```python
import grpc

# Insecure (development)
channel = grpc.insecure_channel('localhost:9090')

# Secure with certificate
with open('server.crt', 'rb') as f:
    creds = grpc.ssl_channel_credentials(f.read())
channel = grpc.secure_channel('gpuproxy.example.com:9090', creds)
```

**Node.js Client with TLS:**

```javascript
const grpc = require('@grpc/grpc-js');
const fs = require('fs');

// Insecure (development)
const channel = new grpc.Channel(
  'localhost:9090',
  grpc.credentials.createInsecure()
);

// Secure with certificate
const cert = fs.readFileSync('server.crt');
const creds = grpc.credentials.createSsl(cert);
const channel = new grpc.Channel(
  'gpuproxy.example.com:9090',
  creds
);
```

## Troubleshooting

### Connection Refused

Check that the gRPC server is running:

```bash
netstat -an | grep 9090
```

Or use grpcurl to test:

```bash
grpcurl -plaintext localhost:9090 list
```

### Authentication Errors

Ensure you're passing the correct metadata:

```go
// Correct
ctx := metadata.AppendToOutgoingContext(ctx,
    "authorization", "Bearer "+token)

// Also correct
ctx := metadata.AppendToOutgoingContext(ctx,
    "x-api-key", apiKey)
```

### Proto Generation Issues

If you get import errors, ensure protoc plugins are installed:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### TLS Certificate Errors

**"x509: certificate signed by unknown authority"**

This means the client doesn't trust the server's certificate. Solutions:

```go
// For self-signed certs, load the certificate file
creds, err := credentials.NewClientTLSFromFile("server.crt", "")

// Or skip verification (INSECURE - development only)
config := &tls.Config{InsecureSkipVerify: true}
creds := credentials.NewTLS(config)
```

**"transport: authentication handshake failed"**

This usually means:
1. Server expects TLS but client is using insecure connection
2. Server is insecure but client expects TLS

Check your server logs to see if TLS is enabled:
```
gRPC server starting with TLS enabled       # TLS is on
gRPC server starting WITHOUT TLS (insecure) # TLS is off
```

Match your client configuration to the server mode.

**Testing TLS with grpcurl:**

```bash
# Without TLS
grpcurl -plaintext localhost:9090 list

# With TLS (self-signed cert)
grpcurl -cacert server.crt localhost:9090 list

# With TLS (system CA)
grpcurl localhost:9090 list
```

## Additional Resources

- [gRPC Official Documentation](https://grpc.io/docs/)
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers)
- [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
- [gRPC Python Quick Start](https://grpc.io/docs/languages/python/quickstart/)
