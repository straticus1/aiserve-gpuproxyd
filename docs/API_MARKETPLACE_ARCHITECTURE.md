# API Marketplace Platform Architecture

**Date**: 2026-01-13
**Platform**: APIProxy.app powered by KrakenD
**Purpose**: Universal API Gateway & Marketplace

## Overview

APIProxy.app is a multi-tenant API gateway and marketplace that allows:
1. After Dark Systems to resell all our APIs through one unified gateway
2. Third-party providers to list their APIs in our marketplace
3. Customers to access hundreds of APIs through a single authentication system
4. Us to monetize API traffic with flexible pricing models

## URL Structure

All APIs are accessed through a consistent hierarchical URL pattern:

```
https://apiproxy.app/api/{provider}/{service}/{endpoint}/{feature}/{options}
```

### URL Components

- **provider**: API provider (e.g., `aiserve`, `openai`, `stripe`, `darkapi`)
- **service**: High-level service category (e.g., `gpu`, `ml`, `chat`, `analysis`)
- **endpoint**: Specific API endpoint (e.g., `rent`, `inference`, `completions`)
- **feature**: Feature or model variant (e.g., `h100`, `yolo`, `gpt4`)
- **options**: Additional options or parameters (e.g., `spot`, `v8`, `stream`)

### Example URLs

**AIServe.Farm APIs:**
```
GET  /api/aiserve/gpu/list/available
POST /api/aiserve/gpu/rent/h100/spot
GET  /api/aiserve/gpu/status/{instance_id}
POST /api/aiserve/ml/inference/yolo/v8
POST /api/aiserve/ml/train/pytorch/distributed
GET  /api/aiserve/models/list/public
```

**Third-Party Proxied APIs:**
```
POST /api/openai/chat/completions/gpt4/stream
POST /api/anthropic/messages/claude/sonnet
POST /api/stripe/payments/charge/card
GET  /api/github/repos/search/code
POST /api/sendgrid/mail/send/transactional
```

**DarkAPI (Web Intelligence):**
```
GET  /api/darkapi/analysis/domain/whois/json
GET  /api/darkapi/analysis/ip/geolocation
POST /api/darkapi/scrape/website/structured
GET  /api/darkapi/ssl/certificate/{domain}
```

## System Components

### 1. KrakenD API Gateway

**Purpose**: High-performance API gateway handling all traffic

**Capabilities:**
- Request routing to backend services
- Rate limiting per API key
- Response caching
- API composition (merge multiple backend calls)
- Authentication/authorization
- Request transformation
- Response aggregation
- Circuit breaker pattern
- Logging and metrics

**Configuration:**
- Dynamic configuration loaded from PostgreSQL
- Hot-reload without downtime
- Per-provider rate limits
- Per-endpoint pricing rules

### 2. API Catalog Service

**Purpose**: Manages the API catalog and metadata

**Database Schema:**

```sql
-- API Providers
CREATE TABLE api_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    website_url VARCHAR(500),
    support_email VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',
    revenue_share_percent DECIMAL(5,2) DEFAULT 30.00,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- API Services
CREATE TABLE api_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES api_providers(id),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    base_url VARCHAR(500) NOT NULL,
    auth_type VARCHAR(50), -- bearer, api_key, oauth2, none
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider_id, slug)
);

-- API Endpoints
CREATE TABLE api_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID REFERENCES api_services(id),
    path VARCHAR(500) NOT NULL,
    method VARCHAR(10) NOT NULL,
    description TEXT,
    request_schema JSONB,
    response_schema JSONB,
    rate_limit_per_min INTEGER DEFAULT 60,
    base_price_per_call DECIMAL(10,6),
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- API Features (variants of endpoints)
CREATE TABLE api_features (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID REFERENCES api_endpoints(id),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    price_multiplier DECIMAL(5,2) DEFAULT 1.0,
    config JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- API Usage Tracking
CREATE TABLE api_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    endpoint_id UUID REFERENCES api_endpoints(id),
    feature_id UUID REFERENCES api_features(id),
    request_path VARCHAR(1000),
    request_method VARCHAR(10),
    status_code INTEGER,
    response_time_ms INTEGER,
    request_size_bytes INTEGER,
    response_size_bytes INTEGER,
    cost_usd DECIMAL(10,6),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_usage_user_created ON api_usage(user_id, created_at);
CREATE INDEX idx_usage_endpoint_created ON api_usage(endpoint_id, created_at);

-- API Keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255),
    scopes JSONB, -- Array of allowed provider/service combinations
    rate_limit_per_min INTEGER DEFAULT 60,
    monthly_spend_limit_usd DECIMAL(10,2),
    status VARCHAR(50) DEFAULT 'active',
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Provider Revenue Tracking
CREATE TABLE provider_revenue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES api_providers(id),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    total_calls BIGINT DEFAULT 0,
    gross_revenue_usd DECIMAL(12,2) DEFAULT 0,
    platform_fee_usd DECIMAL(12,2) DEFAULT 0,
    net_revenue_usd DECIMAL(12,2) DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider_id, period_start, period_end)
);
```

### 3. KrakenD Configuration Generator

**Purpose**: Dynamically generate KrakenD config from database

**Example KrakenD Config:**

```json
{
  "version": 3,
  "endpoints": [
    {
      "endpoint": "/api/aiserve/gpu/list/available",
      "method": "GET",
      "backend": [
        {
          "url_pattern": "/api/v1/gpu/list?status=available",
          "host": ["https://api.aiserve.farm"],
          "method": "GET"
        }
      ],
      "extra_config": {
        "auth/validator": {
          "alg": "RS256",
          "roles": ["user", "admin"]
        },
        "qos/ratelimit/router": {
          "max_rate": 60,
          "client_max_rate": 10
        }
      }
    },
    {
      "endpoint": "/api/openai/chat/completions/{model}/{mode}",
      "method": "POST",
      "backend": [
        {
          "url_pattern": "/v1/chat/completions",
          "host": ["https://api.openai.com"],
          "method": "POST",
          "extra_config": {
            "modifier/martian": {
              "header.Modifier": {
                "scope": ["request"],
                "name": "Authorization",
                "value": "Bearer ${env:OPENAI_API_KEY}"
              }
            }
          }
        }
      ]
    }
  ]
}
```

### 4. Billing & Usage Service

**Purpose**: Track usage and calculate costs

**Features:**
- Real-time usage metering
- Cost calculation per API call
- Monthly billing aggregation
- Provider revenue splits
- Usage alerts and limits

### 5. API Marketplace Frontend

**Purpose**: Web interface for browsing APIs

**Pages:**
- `/` - Homepage with featured APIs
- `/endpoints` - Full API catalog with search/filter
- `/api/{provider}` - Provider detail page
- `/api/{provider}/{service}` - Service documentation
- `/dashboard` - User dashboard with usage stats
- `/providers/signup` - Provider onboarding
- `/docs` - API documentation

## Revenue Model

### For After Dark Systems APIs:
- 100% revenue (our own APIs)
- Examples: AIServe.Farm, DarkAPI, HostScience, WebScience

### For Third-Party Providers:
- 70% to provider, 30% to platform (configurable)
- We handle billing, authentication, rate limiting
- Providers get analytics and revenue reports

### For Proxied External APIs (OpenAI, etc.):
- Markup pricing (e.g., 10-20% over cost)
- Simplifies billing for customers
- Single invoice for all API usage

## Deployment Architecture

```
┌─────────────────────────────────────────────────┐
│                  Internet                       │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│           Cloudflare (CDN + DDoS)               │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│       Kubernetes Ingress (nginx)                │
│       Load Balancer: 129.80.158.147            │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│         KrakenD Gateway (Replicated)            │
│         - Request routing                       │
│         - Rate limiting                         │
│         - Authentication                        │
│         - Response caching                      │
└────┬──────────┬──────────┬──────────┬───────────┘
     │          │          │          │
     ▼          ▼          ▼          ▼
┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
│AIServe  │ │ OpenAI  │ │ DarkAPI │ │ Stripe  │
│Backend  │ │   API   │ │ Backend │ │   API   │
└─────────┘ └─────────┘ └─────────┘ └─────────┘
```

## Implementation Phases

### Phase 1: Foundation (Week 1)
- [x] Set up PostgreSQL database with API catalog schema
- [ ] Deploy KrakenD to Kubernetes
- [ ] Create API catalog management UI
- [ ] Implement API key authentication

### Phase 2: AIServe Integration (Week 2)
- [ ] Migrate all AIServe.Farm APIs to KrakenD
- [ ] Set up GPU rental API endpoints
- [ ] Configure ML inference API routes
- [ ] Add model training API endpoints

### Phase 3: External API Proxying (Week 3)
- [ ] Add OpenAI API proxying
- [ ] Add Anthropic Claude API
- [ ] Add Stripe payment API
- [ ] Set up credential management

### Phase 4: Marketplace Features (Week 4)
- [ ] Build provider onboarding flow
- [ ] Create API documentation generator
- [ ] Implement usage analytics dashboard
- [ ] Set up automated billing

### Phase 5: Advanced Features (Week 5+)
- [ ] API composition (combine multiple APIs)
- [ ] WebSocket support
- [ ] GraphQL gateway
- [ ] API versioning system
- [ ] A/B testing for API responses

## Security Considerations

1. **Authentication:**
   - API keys with JWT tokens
   - OAuth2 for user authentication
   - Rate limiting per key

2. **Authorization:**
   - Scope-based access control
   - Provider-level permissions
   - Endpoint-level restrictions

3. **Data Protection:**
   - TLS/HTTPS everywhere
   - No sensitive data in logs
   - Encrypted API keys in database

4. **DDoS Protection:**
   - Cloudflare in front
   - KrakenD rate limiting
   - Circuit breakers for backends

## Monitoring & Observability

- **Metrics:** Prometheus + Grafana
- **Logs:** Centralized logging (ELK or Loki)
- **Tracing:** Distributed tracing with Jaeger
- **Alerts:** PagerDuty for critical issues

## Cost Structure

### Infrastructure Costs:
- KrakenD pods: ~$50/month (2 replicas)
- PostgreSQL: ~$100/month (managed RDS)
- Redis cache: ~$30/month
- Monitoring: ~$50/month

### External API Costs:
- OpenAI: Pass-through + 15% markup
- Anthropic: Pass-through + 15% markup
- Other APIs: Variable based on usage

### Revenue Potential:
- 1000 users × $50/month = $50,000/month
- Provider fees (30% of third-party revenue)
- Premium features (analytics, higher limits)

## Next Steps

1. Create database schema and migrations
2. Set up KrakenD in Kubernetes
3. Build API catalog management interface
4. Migrate first AIServe APIs
5. Launch beta with select customers

---

**Contact**: ryan@afterdarksys.com
**Platform**: Oracle Kubernetes Engine
**Region**: us-ashburn-1
