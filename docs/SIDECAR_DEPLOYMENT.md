# AIServe.Farm Sidecar Deployment Guide

Run AIServe.Farm as a sidecar container alongside your application for local GPU access and model serving.

## What is Sidecar Deployment?

A sidecar is an auxiliary container that runs alongside your main application container, providing additional functionality without modifying your application code. AIServe.Farm as a sidecar provides:

- **Local GPU Access**: Direct access to GPU compute without network latency
- **Model Serving**: Run ML models locally with ONNX, PyTorch, TensorFlow support
- **API Gateway**: Unified interface to multiple GPU providers (vast.ai, io.net)
- **Cost Control**: Guardrails and spending limits built-in
- **Zero Code Changes**: Integrate via HTTP/gRPC/WebSocket APIs

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  Kubernetes Pod / Docker Compose Stack             │
│                                                     │
│  ┌──────────────────┐    ┌────────────────────┐   │
│  │                  │    │                    │   │
│  │  Your App        │───►│  AIServe Sidecar   │   │
│  │  (Any Language)  │    │  :8080             │   │
│  │                  │    │                    │   │
│  └──────────────────┘    └────────┬───────────┘   │
│                                   │               │
└───────────────────────────────────┼───────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
              ┌──────────┐    ┌──────────┐   ┌──────────┐
              │ Vast.AI  │    │  IO.net  │   │ Local GPU│
              │  GPUs    │    │   GPUs   │   │  (CUDA)  │
              └──────────┘    └──────────┘   └──────────┘
```

## Quick Start - Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  # Your application
  app:
    image: your-app:latest
    environment:
      - AISERVE_URL=http://aiserve:8080
      - AISERVE_API_KEY=${AISERVE_API_KEY}
    depends_on:
      - aiserve
    networks:
      - app-network

  # AIServe sidecar
  aiserve:
    image: ghcr.io/straticus1/aiserve-gpuproxyd:latest
    ports:
      - "8080:8080"  # HTTP API
      - "9090:9090"  # gRPC
    environment:
      - SERVER_HOST=0.0.0.0
      - SERVER_PORT=8080
      - GRPC_PORT=9090
      - DB_HOST=postgres
      - REDIS_HOST=redis
      - JWT_SECRET=${JWT_SECRET}
      - VASTAI_API_KEY=${VASTAI_API_KEY}
      - IONET_API_KEY=${IONET_API_KEY}
    volumes:
      - ./models:/app/models
      - ./data:/app/data
    networks:
      - app-network
    depends_on:
      - postgres
      - redis

  # Supporting services
  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=gpuproxy
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - app-network

  redis:
    image: redis:7-alpine
    networks:
      - app-network
    volumes:
      - redis-data:/data

volumes:
  postgres-data:
  redis-data:

networks:
  app-network:
    driver: bridge
```

Create `.env`:

```bash
# Security (generate these!)
JWT_SECRET=$(openssl rand -base64 64)
DB_PASSWORD=$(openssl rand -base64 32)

# GPU Provider Keys
VASTAI_API_KEY=your-vastai-api-key
IONET_API_KEY=your-ionet-api-key

# AIServe API Key (generated after first run)
AISERVE_API_KEY=your-aiserve-api-key
```

Start services:

```bash
docker-compose up -d
```

## Kubernetes Deployment

### Deployment with Sidecar

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  labels:
    app: myapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      # Your application container
      - name: app
        image: your-app:latest
        ports:
        - containerPort: 3000
        env:
        - name: AISERVE_URL
          value: "http://localhost:8080"
        - name: AISERVE_API_KEY
          valueFrom:
            secretKeyRef:
              name: aiserve-secrets
              key: api-key

      # AIServe sidecar container
      - name: aiserve
        image: ghcr.io/straticus1/aiserve-gpuproxyd:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: grpc
        env:
        - name: SERVER_HOST
          value: "0.0.0.0"
        - name: DB_HOST
          value: "postgres-service"
        - name: REDIS_HOST
          value: "redis-service"
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: aiserve-secrets
              key: jwt-secret
        - name: VASTAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: gpu-providers
              key: vastai-key
        - name: IONET_API_KEY
          valueFrom:
            secretKeyRef:
              name: gpu-providers
              key: ionet-key
        volumeMounts:
        - name: models
          mountPath: /app/models
        - name: data
          mountPath: /app/data
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5

      volumes:
      - name: models
        persistentVolumeClaim:
          claimName: aiserve-models
      - name: data
        persistentVolumeClaim:
          claimName: aiserve-data
```

### Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aiserve-secrets
type: Opaque
stringData:
  jwt-secret: <base64-encoded-secret>
  api-key: <your-api-key>
---
apiVersion: v1
kind: Secret
metadata:
  name: gpu-providers
type: Opaque
stringData:
  vastai-key: <your-vastai-key>
  ionet-key: <your-ionet-key>
```

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: myapp-service
spec:
  selector:
    app: myapp
  ports:
  - name: http
    port: 80
    targetPort: 3000
  - name: aiserve-http
    port: 8080
    targetPort: 8080
  - name: aiserve-grpc
    port: 9090
    targetPort: 9090
```

## Client Integration

### Go Application

```go
package main

import (
    "log"
    "os"

    "github.com/straticus1/aiserve-gpuproxyd/sdk/go/aiserve"
)

func main() {
    // Connect to sidecar
    client := aiserve.NewClient(&aiserve.Config{
        BaseURL: os.Getenv("AISERVE_URL"),
        APIKey:  os.Getenv("AISERVE_API_KEY"),
    })

    // Use GPU instances
    instances, err := client.GPU.ListInstances(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found %d GPUs via sidecar", len(instances))
}
```

### Python Application

```python
import os
from aiserve import Client

# Connect to sidecar
client = Client(
    base_url=os.environ['AISERVE_URL'],
    api_key=os.environ['AISERVE_API_KEY']
)

# Run model inference
result = client.models.predict(
    model_id='my-model',
    inputs={'features': [1, 2, 3, 4]}
)
print(f"Result: {result['outputs']}")
```

### Node.js Application

```javascript
import { AIServeClient } from '@aiserve/sdk';

// Connect to sidecar
const client = new AIServeClient({
  baseUrl: process.env.AISERVE_URL,
  apiKey: process.env.AISERVE_API_KEY
});

// Reserve GPUs
const reservation = await client.gpu.reserveInstances({
  count: 4,
  filters: { minVram: 24 }
});

console.log(`Reserved ${reservation.count} GPUs`);
```

### cURL / Any Language

```bash
# The sidecar exposes standard HTTP/REST API
export AISERVE_URL="http://localhost:8080"
export API_KEY="your-api-key"

# List GPU instances
curl -X GET "$AISERVE_URL/api/v1/gpu/instances" \
  -H "Authorization: Bearer $API_KEY"

# Run inference
curl -X POST "$AISERVE_URL/api/v1/models/model-id/predict" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"features": [1.0, 2.0, 3.0]}}'
```

## Configuration

### Environment Variables

```bash
# Server
SERVER_HOST=0.0.0.0                # Listen on all interfaces for sidecar
SERVER_PORT=8080
GRPC_PORT=9090

# Database (use service names in K8s/Docker Compose)
DB_HOST=postgres
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=<password>
DB_NAME=gpuproxy

# Redis (use service names)
REDIS_HOST=redis
REDIS_PORT=6379

# Security
JWT_SECRET=<generate-with-openssl-rand>

# GPU Providers
VASTAI_API_KEY=<your-key>
IONET_API_KEY=<your-key>

# Storage (if using model uploads)
MODEL_STORAGE_PATH=/app/models
```

### Resource Requirements

**Minimum:**
- CPU: 500m (0.5 cores)
- Memory: 1Gi
- Storage: 10Gi (for models)

**Recommended:**
- CPU: 1000m (1 core)
- Memory: 2Gi
- Storage: 50Gi

**High Load:**
- CPU: 2000m (2 cores)
- Memory: 4Gi
- Storage: 100Gi

## Benefits of Sidecar Pattern

### 1. Zero Application Changes
Your app remains unchanged - just add HTTP API calls to `localhost:8080`.

### 2. Language Agnostic
Works with any language that can make HTTP requests (Go, Python, Node.js, Java, Ruby, etc.).

### 3. Local Network Communication
Sidecar runs in same pod/network - sub-millisecond latency.

### 4. Resource Isolation
AIServe has its own resource limits and doesn't interfere with your app.

### 5. Independent Scaling
Scale AIServe sidecars independently from your application replicas.

### 6. Shared Storage
Models and data stored in shared volumes accessible by both containers.

## Advanced Patterns

### Load Balancer Sidecar

Use one AIServe sidecar to serve multiple app replicas:

```yaml
# Separate deployment for shared AIServe
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aiserve-pool
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: aiserve
        image: ghcr.io/straticus1/aiserve-gpuproxyd:latest
        # ... config ...

---
# Your app without sidecar
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 10
  template:
    spec:
      containers:
      - name: app
        env:
        - name: AISERVE_URL
          value: "http://aiserve-service:8080"
```

### GPU Node Affinity

Run AIServe sidecar on GPU nodes:

```yaml
spec:
  template:
    spec:
      nodeSelector:
        accelerator: nvidia-tesla-t4
      containers:
      - name: aiserve
        resources:
          limits:
            nvidia.com/gpu: 1
```

### Init Container Pattern

Pre-load models before app starts:

```yaml
spec:
  template:
    spec:
      initContainers:
      - name: model-loader
        image: busybox
        command:
        - sh
        - -c
        - |
          wget -O /models/model.onnx https://storage.com/model.onnx
        volumeMounts:
        - name: models
          mountPath: /models
      containers:
      - name: aiserve
        # ... uses pre-loaded models ...
```

## Monitoring

### Prometheus Metrics

```yaml
apiVersion: v1
kind: Service
metadata:
  name: aiserve-metrics
  labels:
    app: aiserve
spec:
  ports:
  - name: metrics
    port: 8080
    targetPort: 8080
  selector:
    app: aiserve
  type: ClusterIP
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: aiserve
spec:
  selector:
    matchLabels:
      app: aiserve
  endpoints:
  - port: metrics
    path: /metrics
```

### Health Checks

```bash
# Liveness probe
curl http://localhost:8080/health

# Readiness probe
curl http://localhost:8080/health

# Detailed monitor
curl http://localhost:8080/monitor
```

## Troubleshooting

### Sidecar Not Starting

```bash
# Check logs
kubectl logs <pod-name> -c aiserve

# Common issues:
# - Missing secrets (JWT_SECRET, DB_PASSWORD)
# - Database not ready (add initContainer or wait script)
# - Port conflicts (check 8080, 9090 availability)
```

### Connection Refused

```bash
# Verify sidecar is listening
kubectl exec <pod-name> -c aiserve -- netstat -tuln | grep 8080

# Check from app container
kubectl exec <pod-name> -c app -- curl http://localhost:8080/health
```

### High Memory Usage

```bash
# Limit model cache
- name: MAX_LOADED_MODELS
  value: "10"

# Enable model unloading
- name: MODEL_IDLE_TIMEOUT
  value: "30m"
```

## Security Best Practices

1. **Never expose sidecar externally** - Use ClusterIP service type
2. **Use Kubernetes Secrets** for API keys and credentials
3. **Enable network policies** to restrict sidecar access
4. **Set resource limits** to prevent resource exhaustion
5. **Use least-privilege service accounts**
6. **Enable authentication** on all API endpoints
7. **Rotate API keys regularly**
8. **Monitor spending** via guardrails

## Performance Tuning

```yaml
# Connection pooling
- name: DB_MAX_CONNS
  value: "50"
- name: DB_MIN_CONNS
  value: "10"

# Redis performance
- name: REDIS_POOL_SIZE
  value: "20"

# Request limits
- name: MAX_CONCURRENT_REQUESTS
  value: "100"
```

## Examples Repository

Complete sidecar examples available at:
- Docker Compose: `/examples/docker-compose/`
- Kubernetes: `/examples/kubernetes/`
- Helm Chart: `/examples/helm/`

## Support

- Documentation: https://aiserve.farm/docs/sidecar
- GitHub Issues: https://github.com/straticus1/aiserve-gpuproxyd/issues
- Discord: https://discord.gg/aiserve
- Email: support@afterdarksys.com

---

**Last Updated:** 2026-01-13
**Platform:** AIServe.Farm by AfterDark Systems (ADS)
