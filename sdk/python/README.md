# AIServe.Farm Python SDK

Official Python client library for AIServe.Farm API.

## Installation

```bash
pip install aiserve
```

Or install from source:

```bash
git clone https://github.com/straticus1/aiserve-gpuproxyd.git
cd aiserve-gpuproxyd/sdk/python
pip install -e .
```

## Quick Start

```python
from aiserve import Client

# Create client
client = Client(
    base_url="https://api.aiserve.farm",
    api_key="your-api-key"
)

# List GPU instances
instances = client.gpu.list_instances(
    provider="vastai",
    min_vram=16
)

print(f"Found {len(instances)} instances")
```

## Documentation

See [API_REFERENCE.md](../../docs/API_REFERENCE.md) for complete API documentation.

## Examples

### Authentication

```python
from aiserve import Client

# Login with email/password
client = Client(base_url="https://api.aiserve.farm")
tokens = client.auth.login("user@example.com", "password")

# Use JWT token
client.set_token(tokens['access_token'])

# Or create API key for long-lived access
from datetime import datetime, timedelta

api_key = client.auth.create_api_key(
    name="production",
    expires_at=datetime.now() + timedelta(days=365)
)

# Use API key
client = Client(api_key=api_key['api_key'])
```

### GPU Management

```python
# List available GPUs
instances = client.gpu.list_instances(
    provider="all",
    min_vram=24,
    max_price=2.5,
    gpu_model="RTX 4090",
    location="US"
)

# Create single instance
contract = client.gpu.create_instance(
    provider="vastai",
    instance_id="instance_123",
    duration_hours=4,
    auto_renew=False
)

# Reserve multiple instances with load balancing
reservation = client.gpu.reserve_instances(
    count=4,
    filters={
        "min_vram": 24,
        "gpu_model": "RTX 4090",
        "location": "US"
    },
    config={
        "duration_hours": 8,
        "auto_renew": True
    }
)

# Destroy instance
client.gpu.destroy_instance("vastai", "instance_123")
```

### Model Serving

```python
# Upload model
with open("model.onnx", "rb") as f:
    model = client.models.upload(
        file=f,
        name="my_model",
        format="onnx",
        gpu_required=True
    )

# List models
models = client.models.list()

# Run inference
result = client.models.predict(
    model_id=model['model_id'],
    inputs={
        "features": [1.0, 2.0, 3.0, 4.0]
    }
)

print(f"Prediction: {result['outputs']} (latency: {result['latency_ms']:.2f}ms)")

# Delete model
client.models.delete(model['model_id'])
```

### Async Support

```python
from aiserve import AsyncClient
import asyncio

async def main():
    async with AsyncClient(api_key="your-api-key") as client:
        # All methods support async/await
        instances = await client.gpu.list_instances()

        # Concurrent operations
        tasks = [
            client.gpu.create_instance("vastai", f"instance_{i}")
            for i in range(4)
        ]
        results = await asyncio.gather(*tasks)

asyncio.run(main())
```

### Billing & Guardrails

```python
# Check spending status
spending = client.guardrails.get_spending()
print(f"Spent: ${spending['window_spent']:.2f} / ${spending['window_limit']:.2f}")

# Check if operation is allowed
check = client.guardrails.check_spending(estimated_cost=50.00)
if not check['allowed']:
    print("Spending limit would be exceeded")
    print(f"Violations: {check['violations']}")

# Record spending
client.guardrails.record_spending(amount=25.50)

# Get transaction history
transactions = client.billing.get_transactions()
```

### Load Balancing

```python
# Get current strategy
strategy = client.load_balancer.get_strategy()
print(f"Current strategy: {strategy}")

# Set strategy
client.load_balancer.set_strategy("least_connections")

# Get instance loads
loads = client.load_balancer.get_loads()
for instance_id, load in loads['loads'].items():
    print(f"{instance_id}: {load['connections']} connections ({load['load']:.2f} load)")
```

### Storage Quotas

```python
# Check quota status
quota = client.quota.get()
print(f"Storage: {quota['storage']['used_pct']:.1f}% used")
print(f"Uploads today: {quota['rate_limits']['uploads_last_day']}/{quota['rate_limits']['daily_limit']}")
```

### Streaming Inference

```python
# WebSocket streaming
with client.models.stream_predict(model_id) as stream:
    # Send input
    stream.send({
        "inputs": {"prompt": "Hello world"}
    })

    # Receive outputs
    for result in stream:
        print(f"Received: {result['outputs']}")
```

## API Reference

### Client

```python
class Client:
    def __init__(
        self,
        base_url: str = "https://api.aiserve.farm",
        api_key: Optional[str] = None,
        token: Optional[str] = None,
        timeout: int = 30
    ):
        ...

    def set_token(self, token: str) -> None:
        ...

    def set_api_key(self, api_key: str) -> None:
        ...
```

### Authentication

```python
client.auth.login(email: str, password: str) -> Dict[str, Any]
client.auth.register(email: str, password: str, name: str) -> Dict[str, Any]
client.auth.create_api_key(name: str, expires_at: Optional[datetime] = None) -> Dict[str, Any]
```

### GPU Management

```python
client.gpu.list_instances(
    provider: str = "all",
    min_vram: Optional[int] = None,
    max_price: Optional[float] = None,
    gpu_model: Optional[str] = None,
    location: Optional[str] = None
) -> List[Dict[str, Any]]

client.gpu.create_instance(
    provider: str,
    instance_id: str,
    duration_hours: int = 1,
    auto_renew: bool = False
) -> Dict[str, Any]

client.gpu.destroy_instance(provider: str, instance_id: str) -> None

client.gpu.reserve_instances(
    count: int,
    filters: Optional[Dict] = None,
    config: Optional[Dict] = None
) -> Dict[str, Any]
```

### Model Serving

```python
client.models.upload(
    file: BinaryIO,
    name: Optional[str] = None,
    format: Optional[str] = None,
    gpu_required: bool = False
) -> Dict[str, Any]

client.models.list() -> List[Dict[str, Any]]
client.models.get(model_id: str) -> Dict[str, Any]
client.models.delete(model_id: str) -> None
client.models.predict(model_id: str, inputs: Dict, parameters: Optional[Dict] = None) -> Dict[str, Any]
```

### Error Handling

```python
from aiserve.exceptions import (
    AIServeError,
    AuthenticationError,
    RateLimitError,
    QuotaExceededError,
    SpendingLimitError
)

try:
    instances = client.gpu.list_instances()
except AuthenticationError as e:
    print(f"Authentication failed: {e}")
except RateLimitError as e:
    print(f"Rate limited: {e}")
except QuotaExceededError as e:
    print(f"Quota exceeded: {e}")
except AIServeError as e:
    print(f"API error: {e}")
```

## Advanced Usage

### Custom HTTP Client

```python
import requests
from aiserve import Client

session = requests.Session()
session.headers.update({"X-Custom-Header": "value"})

client = Client(
    api_key="your-api-key",
    session=session
)
```

### Retry Configuration

```python
from aiserve import Client

client = Client(
    api_key="your-api-key",
    max_retries=5,
    retry_backoff=2.0
)
```

### Logging

```python
import logging

# Enable debug logging
logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger('aiserve')
logger.setLevel(logging.DEBUG)
```

### Context Manager

```python
with Client(api_key="your-api-key") as client:
    instances = client.gpu.list_instances()
    # Client automatically closed after context
```

## Type Hints

The SDK is fully typed with Python type hints:

```python
from typing import List, Dict, Any, Optional
from aiserve import Client

def get_gpus(client: Client, min_vram: int) -> List[Dict[str, Any]]:
    return client.gpu.list_instances(min_vram=min_vram)
```

## Testing

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Run with coverage
pytest --cov=aiserve
```

## Requirements

- Python 3.8+
- requests >= 2.28.0
- aiohttp >= 3.8.0 (for async support)

## License

MIT License - see LICENSE file for details

## Support

- GitHub Issues: https://github.com/straticus1/aiserve-gpuproxyd/issues
- Documentation: https://aiserve.farm/docs
- Email: support@afterdarksys.com
