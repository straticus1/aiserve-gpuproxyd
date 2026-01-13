# AIServe.Farm Node.js SDK

Official Node.js/TypeScript client library for AIServe.Farm API.

## Installation

```bash
npm install @aiserve/sdk
# or
yarn add @aiserve/sdk
# or
pnpm add @aiserve/sdk
```

## Quick Start

```typescript
import { AIServeClient } from '@aiserve/sdk';

// Create client
const client = new AIServeClient({
  baseUrl: 'https://api.aiserve.farm',
  apiKey: 'your-api-key'
});

// List GPU instances
const instances = await client.gpu.listInstances({
  provider: 'vastai',
  minVram: 16
});

console.log(`Found ${instances.length} instances`);
```

## Documentation

See [API_REFERENCE.md](../../docs/API_REFERENCE.md) for complete API documentation.

## Examples

### Authentication

```typescript
import { AIServeClient } from '@aiserve/sdk';

// Login with email/password
const client = new AIServeClient({
  baseUrl: 'https://api.aiserve.farm'
});

const { tokens } = await client.auth.login('user@example.com', 'password');

// Use JWT token
client.setToken(tokens.access_token);

// Or create API key for long-lived access
const apiKey = await client.auth.createApiKey({
  name: 'production',
  expiresAt: new Date(Date.now() + 365 * 24 * 60 * 60 * 1000)
});

// Use API key
const client2 = new AIServeClient({ apiKey: apiKey.api_key });
```

### GPU Management

```typescript
// List available GPUs
const instances = await client.gpu.listInstances({
  provider: 'all',
  minVram: 24,
  maxPrice: 2.5,
  gpuModel: 'RTX 4090',
  location: 'US'
});

// Create single instance
const contract = await client.gpu.createInstance('vastai', 'instance_123', {
  durationHours: 4,
  autoRenew: false
});

// Reserve multiple instances with load balancing
const reservation = await client.gpu.reserveInstances({
  count: 4,
  filters: {
    minVram: 24,
    gpuModel: 'RTX 4090',
    location: 'US'
  },
  config: {
    durationHours: 8,
    autoRenew: true
  }
});

// Destroy instance
await client.gpu.destroyInstance('vastai', 'instance_123');
```

### Model Serving

```typescript
import fs from 'fs';

// Upload model
const file = fs.createReadStream('model.onnx');
const model = await client.models.upload({
  file,
  name: 'my_model',
  format: 'onnx',
  gpuRequired: true
});

// List models
const models = await client.models.list();

// Run inference
const result = await client.models.predict(model.model_id, {
  inputs: {
    features: [1.0, 2.0, 3.0, 4.0]
  }
});

console.log(`Prediction: ${JSON.stringify(result.outputs)} (latency: ${result.latency_ms}ms)`);

// Delete model
await client.models.delete(model.model_id);
```

### Streaming Inference

```typescript
// WebSocket streaming
const stream = await client.models.streamPredict(modelId);

// Send input
await stream.send({
  inputs: { prompt: 'Hello world' }
});

// Receive outputs
for await (const result of stream) {
  console.log('Received:', result.outputs);
}

await stream.close();
```

### Billing & Guardrails

```typescript
// Check spending status
const spending = await client.guardrails.getSpending();
console.log(`Spent: $${spending.window_spent.toFixed(2)} / $${spending.window_limit.toFixed(2)}`);

// Check if operation is allowed
const check = await client.guardrails.checkSpending({ estimatedCost: 50.00 });
if (!check.allowed) {
  console.log('Spending limit would be exceeded');
  console.log('Violations:', check.violations);
}

// Record spending
await client.guardrails.recordSpending({ amount: 25.50 });

// Get transaction history
const transactions = await client.billing.getTransactions();
```

### Load Balancing

```typescript
// Get current strategy
const strategy = await client.loadBalancer.getStrategy();
console.log(`Current strategy: ${strategy}`);

// Set strategy
await client.loadBalancer.setStrategy('least_connections');

// Get instance loads
const loads = await client.loadBalancer.getLoads();
for (const [instanceId, load] of Object.entries(loads.loads)) {
  console.log(`${instanceId}: ${load.connections} connections (${load.load.toFixed(2)} load)`);
}
```

### Storage Quotas

```typescript
// Check quota status
const quota = await client.quota.get();
console.log(`Storage: ${quota.storage.used_pct.toFixed(1)}% used`);
console.log(`Uploads today: ${quota.rate_limits.uploads_last_day}/${quota.rate_limits.daily_limit}`);
```

## API Reference

### Client

```typescript
class AIServeClient {
  constructor(config: AIServeConfig);

  setToken(token: string): void;
  setApiKey(apiKey: string): void;

  auth: AuthService;
  gpu: GPUService;
  models: ModelsService;
  billing: BillingService;
  guardrails: GuardrailsService;
  loadBalancer: LoadBalancerService;
  quota: QuotaService;
}

interface AIServeConfig {
  baseUrl?: string;
  apiKey?: string;
  token?: string;
  timeout?: number;
}
```

### Authentication

```typescript
interface AuthService {
  login(email: string, password: string): Promise<LoginResponse>;
  register(email: string, password: string, name: string): Promise<RegisterResponse>;
  createApiKey(request: CreateApiKeyRequest): Promise<CreateApiKeyResponse>;
}

interface LoginResponse {
  user: User;
  tokens: Tokens;
}

interface Tokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}
```

### GPU Management

```typescript
interface GPUService {
  listInstances(options?: ListInstancesOptions): Promise<GPUInstance[]>;
  createInstance(provider: string, instanceId: string, config: InstanceConfig): Promise<CreateInstanceResponse>;
  destroyInstance(provider: string, instanceId: string): Promise<void>;
  reserveInstances(request: ReserveRequest): Promise<ReserveResponse>;
}

interface ListInstancesOptions {
  provider?: string;
  minVram?: number;
  maxPrice?: number;
  gpuModel?: string;
  location?: string;
}

interface GPUInstance {
  id: string;
  provider: string;
  gpu_model: string;
  vram_gb: number;
  price_per_hour: number;
  location: string;
  available: boolean;
}
```

### Model Serving

```typescript
interface ModelsService {
  upload(request: UploadModelRequest): Promise<UploadModelResponse>;
  list(): Promise<Model[]>;
  get(modelId: string): Promise<Model>;
  delete(modelId: string): Promise<void>;
  predict(modelId: string, request: PredictRequest): Promise<PredictResponse>;
  streamPredict(modelId: string): Promise<ModelStream>;
}

interface UploadModelRequest {
  file: ReadStream | Buffer;
  name?: string;
  format?: string;
  gpuRequired?: boolean;
}

interface PredictRequest {
  inputs: Record<string, any>;
  parameters?: Record<string, any>;
}

interface PredictResponse {
  model_id: string;
  outputs: Record<string, any>;
  latency_ms: number;
  metadata?: Record<string, any>;
}
```

### Error Handling

```typescript
import {
  AIServeError,
  AuthenticationError,
  RateLimitError,
  QuotaExceededError,
  SpendingLimitError
} from '@aiserve/sdk';

try {
  const instances = await client.gpu.listInstances();
} catch (error) {
  if (error instanceof AuthenticationError) {
    console.error('Authentication failed:', error.message);
  } else if (error instanceof RateLimitError) {
    console.error('Rate limited:', error.message);
  } else if (error instanceof QuotaExceededError) {
    console.error('Quota exceeded:', error.message);
  } else if (error instanceof AIServeError) {
    console.error('API error:', error.message, error.statusCode);
  }
}
```

## Advanced Usage

### Custom Axios Instance

```typescript
import axios from 'axios';
import { AIServeClient } from '@aiserve/sdk';

const axiosInstance = axios.create({
  timeout: 60000,
  headers: { 'X-Custom-Header': 'value' }
});

const client = new AIServeClient({
  apiKey: 'your-api-key',
  httpClient: axiosInstance
});
```

### Retry Configuration

```typescript
const client = new AIServeClient({
  apiKey: 'your-api-key',
  retries: 5,
  retryDelay: 1000,
  retryCondition: (error) => {
    return error.response?.status === 503;
  }
});
```

### Request Interceptors

```typescript
client.interceptors.request.use((config) => {
  console.log('Request:', config.method, config.url);
  return config;
});

client.interceptors.response.use((response) => {
  console.log('Response:', response.status);
  return response;
});
```

### Abort Controller

```typescript
const controller = new AbortController();

// Start request
const promise = client.gpu.listInstances({
  signal: controller.signal
});

// Cancel after 5 seconds
setTimeout(() => controller.abort(), 5000);

try {
  const instances = await promise;
} catch (error) {
  if (error.name === 'AbortError') {
    console.log('Request cancelled');
  }
}
```

## TypeScript Support

The SDK is written in TypeScript with full type definitions:

```typescript
import type {
  AIServeClient,
  GPUInstance,
  Model,
  PredictRequest,
  PredictResponse
} from '@aiserve/sdk';

async function runInference(
  client: AIServeClient,
  modelId: string,
  inputs: Record<string, any>
): Promise<PredictResponse> {
  return await client.models.predict(modelId, { inputs });
}
```

## Testing

```bash
# Install dependencies
npm install

# Run tests
npm test

# Run with coverage
npm run test:coverage

# Type check
npm run type-check
```

## ESM and CommonJS

The package supports both ESM and CommonJS:

```javascript
// ESM
import { AIServeClient } from '@aiserve/sdk';

// CommonJS
const { AIServeClient } = require('@aiserve/sdk');
```

## Browser Support

The SDK works in both Node.js and modern browsers:

```html
<script type="module">
  import { AIServeClient } from 'https://cdn.jsdelivr.net/npm/@aiserve/sdk/+esm';

  const client = new AIServeClient({ apiKey: 'your-api-key' });
  const instances = await client.gpu.listInstances();
</script>
```

## Requirements

- Node.js 16+
- TypeScript 4.5+ (for TypeScript users)

## Dependencies

- axios ^1.6.0
- ws ^8.14.0 (for WebSocket streaming)
- form-data ^4.0.0 (for file uploads)

## License

MIT License - see LICENSE file for details

## Support

- GitHub Issues: https://github.com/straticus1/aiserve-gpuproxyd/issues
- Documentation: https://aiserve.farm/docs
- Email: support@afterdarksys.com
- Discord: https://discord.gg/aiserve
