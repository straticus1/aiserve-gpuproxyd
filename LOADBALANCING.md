# Load Balancing Guide

GPU Proxy includes advanced load balancing capabilities to distribute GPU workloads efficiently across providers.

## Supported Strategies

### 1. Round Robin
**Strategy:** `round_robin`

Distributes requests evenly across all available GPUs in a circular fashion.

**Use Cases:**
- Equal distribution of workload
- Simple setup with predictable patterns
- Testing and development

**Configuration:**
```env
LB_STRATEGY=round_robin
```

### 2. Equal Weighted
**Strategy:** `equal_weighted`

Distributes requests based on total connection count, favoring GPUs with fewer total connections.

**Use Cases:**
- Balancing long-running workloads
- Preventing overutilization of specific GPUs
- Fair distribution over time

**Configuration:**
```env
LB_STRATEGY=equal_weighted
```

### 3. Weighted Round Robin
**Strategy:** `weighted_round_robin`

Assigns weights based on GPU specifications (VRAM, price) and distributes accordingly.

**Weight Calculation:**
- 80GB+ VRAM: 3.0x weight
- 40GB+ VRAM: 2.0x weight
- 24GB+ VRAM: 1.5x weight
- Price < $1/hr: 1.2x multiplier

**Use Cases:**
- Heterogeneous GPU clusters
- Optimizing for performance vs cost
- Prioritizing powerful GPUs

**Configuration:**
```env
LB_STRATEGY=weighted_round_robin
```

### 4. Least Connections
**Strategy:** `least_connections`

Routes requests to the GPU with the fewest active connections.

**Use Cases:**
- Real-time workloads
- Variable request duration
- Maximizing throughput

**Configuration:**
```env
LB_STRATEGY=least_connections
```

### 5. Least Response Time
**Strategy:** `least_response_time`

Routes to the GPU with the lowest average response time.

**Use Cases:**
- Latency-sensitive applications
- Inference workloads
- Performance optimization

**Configuration:**
```env
LB_STRATEGY=least_response_time
```

## API Usage

### Get Current Strategy
```bash
curl -H "X-API-Key: YOUR_KEY" \
  http://localhost:8080/api/v1/loadbalancer/strategy
```

Response:
```json
{
  "strategy": "round_robin"
}
```

### Set Strategy
```bash
curl -X PUT \
  -H "X-API-Key: YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{"strategy": "least_connections"}' \
  http://localhost:8080/api/v1/loadbalancer/strategy
```

Response:
```json
{
  "strategy": "least_connections",
  "message": "Load balancing strategy updated"
}
```

### View Load Statistics
```bash
curl -H "X-API-Key: YOUR_KEY" \
  http://localhost:8080/api/v1/loadbalancer/loads
```

Response:
```json
{
  "strategy": "least_connections",
  "count": 5,
  "loads": {
    "vast-12345": {
      "instance_id": "vast-12345",
      "provider": "vast.ai",
      "active_connections": 3,
      "total_connections": 150,
      "avg_response_time": "250ms",
      "last_response_time": "245ms",
      "weight": 1.5,
      "last_used": "2024-01-12T10:30:00Z"
    }
  }
}
```

### Get Instance Load
```bash
curl -H "X-API-Key: YOUR_KEY" \
  "http://localhost:8080/api/v1/loadbalancer/load?instance_id=vast-12345"
```

## CLI Usage

### View Load Information

**All Load Info (Server + Provider):**
```bash
./bin/aiserve-gpuproxy-client -key YOUR_KEY load
```

**Server Load Only:**
```bash
./bin/aiserve-gpuproxy-client -key YOUR_KEY load server
```

Output:
```
Load Balancing Strategy: least_connections
Tracked Instances: 5

INSTANCE     PROVIDER  ACTIVE  TOTAL  AVG RT   WEIGHT
vast-12345   vast.ai   3       150    250ms    1.50
vast-67890   vast.ai   2       120    230ms    1.50
ionet-abc    io.net    1       80     180ms    2.00
```

**Provider Load Only:**
```bash
./bin/aiserve-gpuproxy-client -key YOUR_KEY load provider
```

Output:
```
Provider Load:
  vast.ai: 42 available instances
  io.net:  28 available instances
  Total:   70 instances
```

### Get/Set Strategy

**Get Current Strategy:**
```bash
./bin/aiserve-gpuproxy-client -key YOUR_KEY lb-strategy
```

**Set Strategy:**
```bash
./bin/aiserve-gpuproxy-client -key YOUR_KEY lb-strategy least_response_time
```

## GPU Reservation

Reserve multiple GPUs with automatic load balancing:

### Reserve with CLI
```bash
# Reserve 16 GPUs (max)
./bin/aiserve-gpuproxy-client -key YOUR_KEY reserve 16

# Reserve 4 GPUs
./bin/aiserve-gpuproxy-client -key YOUR_KEY reserve 4
```

Output:
```
Reserved 16 GPUs (requested: 16)
INSTANCE     CONTRACT  PROVIDER  GPU         VRAM  PRICE
vast-12345   54321     vast.ai   RTX 4090    24GB  $0.79
vast-67890   54322     vast.ai   RTX 4090    24GB  $0.79
ionet-abc    xyz789    io.net    A100        80GB  $1.99
...
```

### Reserve with API
```bash
curl -X POST \
  -H "X-API-Key: YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "count": 8,
    "filters": {
      "min_vram": 24,
      "max_price": 2.0
    },
    "config": {
      "image": "nvidia/cuda:12.0.0-base-ubuntu22.04"
    }
  }' \
  http://localhost:8080/api/v1/gpu/instances/reserve
```

Response:
```json
{
  "reserved": [
    {
      "instance_id": "vast-12345",
      "contract_id": "54321",
      "provider": "vast.ai",
      "gpu_model": "RTX 4090",
      "vram": 24,
      "price": 0.79
    }
  ],
  "count": 8,
  "requested": 8,
  "errors": []
}
```

## Load Balancing Limits

- **Reservation Limit:** 1-16 GPUs per request
- **Batch Creation:** Up to 8 GPUs per provider (16 total)
- **Strategy Changes:** Dynamic, no restart required
- **Tracking:** Automatic connection and response time tracking

## Best Practices

### 1. Strategy Selection

**For Inference Workloads:**
- Use `least_response_time` for latency-critical apps
- Use `least_connections` for variable request sizes
- Use `weighted_round_robin` for mixed GPU types

**For Training Workloads:**
- Use `weighted_round_robin` to prioritize powerful GPUs
- Use `equal_weighted` for fairness across GPUs
- Use `round_robin` for consistent workloads

### 2. Monitoring

Monitor load statistics regularly:
```bash
# Check every 10 seconds
watch -n 10 './bin/aiserve-gpuproxy-client -key $KEY load'
```

### 3. Dynamic Adjustment

Change strategy based on workload:
```bash
# Peak hours - optimize for latency
./bin/aiserve-gpuproxy-client -key $KEY lb-strategy least_response_time

# Off-peak - optimize for cost
./bin/aiserve-gpuproxy-client -key $KEY lb-strategy weighted_round_robin
```

### 4. Reservation Strategy

**Small Workloads (1-4 GPUs):**
```bash
./bin/aiserve-gpuproxy-client -key $KEY reserve 4
```

**Medium Workloads (5-8 GPUs):**
```bash
# Use batch creation for better control
curl -X POST ... /gpu/instances/batch
```

**Large Workloads (9-16 GPUs):**
```bash
# Reserve with filters for specific requirements
./bin/aiserve-gpuproxy-client -key $KEY reserve 16
```

## Integration Example

```python
import requests

class GPUProxyLB:
    def __init__(self, api_url, api_key):
        self.api_url = api_url
        self.headers = {"X-API-Key": api_key}

    def set_strategy(self, strategy):
        """Set load balancing strategy"""
        url = f"{self.api_url}/loadbalancer/strategy"
        response = requests.put(url, headers=self.headers,
                               json={"strategy": strategy})
        return response.json()

    def get_loads(self):
        """Get all instance loads"""
        url = f"{self.api_url}/loadbalancer/loads"
        response = requests.get(url, headers=self.headers)
        return response.json()

    def reserve_gpus(self, count, filters=None):
        """Reserve multiple GPUs with load balancing"""
        url = f"{self.api_url}/gpu/instances/reserve"
        payload = {
            "count": count,
            "filters": filters or {},
            "config": {"image": "nvidia/cuda:12.0.0-base"}
        }
        response = requests.post(url, headers=self.headers, json=payload)
        return response.json()

# Usage
lb = GPUProxyLB("http://localhost:8080/api/v1", "your-api-key")

# Set optimal strategy
lb.set_strategy("least_response_time")

# Reserve GPUs
result = lb.reserve_gpus(8, filters={"min_vram": 24})
print(f"Reserved {result['count']} GPUs")

# Monitor load
loads = lb.get_loads()
for instance_id, load in loads['loads'].items():
    print(f"{instance_id}: {load['active_connections']} active")
```

## Troubleshooting

### High Response Times

Check if specific instances are overloaded:
```bash
./bin/aiserve-gpuproxy-client -key $KEY load server
```

Switch to least connections:
```bash
./bin/aiserve-gpuproxy-client -key $KEY lb-strategy least_connections
```

### Uneven Distribution

Verify strategy:
```bash
./bin/aiserve-gpuproxy-client -key $KEY lb-strategy
```

Reset to round robin for testing:
```bash
./bin/aiserve-gpuproxy-client -key $KEY lb-strategy round_robin
```

### Reservation Failures

Check available instances:
```bash
./bin/aiserve-gpuproxy-client -key $KEY list | wc -l
```

Adjust reservation count or filters.

## Performance Tuning

### Response Time Optimization
```env
LB_STRATEGY=least_response_time
LB_ENABLED=true
```

### Connection Distribution
```env
LB_STRATEGY=least_connections
LB_ENABLED=true
```

### Cost Optimization
```env
LB_STRATEGY=weighted_round_robin
LB_ENABLED=true
```

## Metrics

The load balancer tracks:
- Active connections per instance
- Total connections (lifetime)
- Average response time
- Last response time
- Instance weight
- Last used timestamp

All metrics are accessible via the API and CLI.
