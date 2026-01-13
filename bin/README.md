# GPU Proxy Binaries

This directory contains the compiled GPU Proxy binaries.

## Binaries

### aiserve-gpuproxyd
**Main GPU proxy server daemon**

Runs the API server that handles:
- GPU instance management (vast.ai, io.net)
- Load balancing with 5 strategies
- Authentication (JWT, API keys)
- Payment processing (Stripe, Crypto, AfterDark)
- WebSocket streaming
- Rate limiting and usage tracking

Start the server:
```bash
./aiserve-gpuproxyd
```

Developer/debug mode:
```bash
./aiserve-gpuproxyd -dv -dm
```

### aiserve-gpuproxy-client
**CLI client for API interaction**

Interact with the GPU Proxy API:
- List available GPUs
- Create/destroy instances
- Reserve multiple GPUs (1-16)
- View load statistics
- Manage load balancing strategy
- Proxy requests through GPU instances

Examples:
```bash
# List GPUs
./aiserve-gpuproxy-client -key YOUR_KEY list

# Reserve 8 GPUs
./aiserve-gpuproxy-client -key YOUR_KEY reserve 8

# View load
./aiserve-gpuproxy-client -key YOUR_KEY load

# Set load balancing strategy
./aiserve-gpuproxy-client -key YOUR_KEY lb-strategy least_connections
```

### aiserve-gpuproxy-admin
**Administrative utility**

Manage users, database, and system:
- Create/manage users
- Generate API keys
- View usage statistics
- Run database migrations
- System monitoring

Examples:
```bash
# Run migrations
./aiserve-gpuproxy-admin migrate

# Create user
./aiserve-gpuproxy-admin create-user admin@example.com pass123 "Admin"

# Make user admin
./aiserve-gpuproxy-admin make-admin admin@example.com

# View stats
./aiserve-gpuproxy-admin stats
```

## Building

To rebuild all binaries:
```bash
make build
```

To clean and rebuild:
```bash
make clean && make build
```
