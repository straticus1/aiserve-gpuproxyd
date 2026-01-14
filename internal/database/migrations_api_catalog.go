package database

// API Catalog migrations for APIProxy marketplace platform
const apiCatalogMigrations = `
-- =====================================================
-- API Marketplace Platform Schema
-- =====================================================

-- API Providers Table
CREATE TABLE IF NOT EXISTS api_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    website_url VARCHAR(500),
    support_email VARCHAR(255),
    logo_url VARCHAR(500),
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'inactive')),
    revenue_share_percent DECIMAL(5,2) DEFAULT 30.00 CHECK (revenue_share_percent >= 0 AND revenue_share_percent <= 100),
    is_first_party BOOLEAN DEFAULT FALSE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_api_providers_slug ON api_providers(slug);
CREATE INDEX idx_api_providers_status ON api_providers(status);

-- API Services Table
CREATE TABLE IF NOT EXISTS api_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES api_providers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    base_url VARCHAR(500) NOT NULL,
    auth_type VARCHAR(50) DEFAULT 'bearer' CHECK (auth_type IN ('bearer', 'api_key', 'oauth2', 'basic', 'none')),
    auth_config JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'beta', 'deprecated', 'inactive')),
    version VARCHAR(50) DEFAULT 'v1',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider_id, slug)
);

CREATE INDEX idx_api_services_provider ON api_services(provider_id);
CREATE INDEX idx_api_services_slug ON api_services(slug);
CREATE INDEX idx_api_services_category ON api_services(category);

-- API Endpoints Table
CREATE TABLE IF NOT EXISTS api_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES api_services(id) ON DELETE CASCADE,
    path VARCHAR(500) NOT NULL,
    method VARCHAR(10) NOT NULL CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS', 'HEAD')),
    description TEXT,
    summary VARCHAR(500),
    request_schema JSONB,
    response_schema JSONB,
    rate_limit_per_min INTEGER DEFAULT 60 CHECK (rate_limit_per_min > 0),
    rate_limit_per_hour INTEGER DEFAULT 3600,
    rate_limit_per_day INTEGER DEFAULT 100000,
    base_price_per_call DECIMAL(10,6) DEFAULT 0.001,
    requires_auth BOOLEAN DEFAULT TRUE,
    is_public BOOLEAN DEFAULT FALSE,
    cache_ttl_seconds INTEGER DEFAULT 0,
    timeout_seconds INTEGER DEFAULT 30,
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'beta', 'deprecated', 'inactive')),
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_api_endpoints_service ON api_endpoints(service_id);
CREATE INDEX idx_api_endpoints_path ON api_endpoints(path);
CREATE INDEX idx_api_endpoints_method ON api_endpoints(method);
CREATE INDEX idx_api_endpoints_status ON api_endpoints(status);

-- API Features (variants/models for endpoints)
CREATE TABLE IF NOT EXISTS api_features (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID NOT NULL REFERENCES api_endpoints(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    price_multiplier DECIMAL(5,2) DEFAULT 1.0 CHECK (price_multiplier >= 0),
    config JSONB DEFAULT '{}',
    is_default BOOLEAN DEFAULT FALSE,
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'beta', 'deprecated', 'inactive')),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_api_features_endpoint ON api_features(endpoint_id);
CREATE INDEX idx_api_features_slug ON api_features(slug);

-- API Keys Table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    key_prefix VARCHAR(20) NOT NULL,
    name VARCHAR(255),
    description TEXT,
    scopes JSONB DEFAULT '[]',
    rate_limit_per_min INTEGER DEFAULT 60,
    rate_limit_per_hour INTEGER DEFAULT 3600,
    rate_limit_per_day INTEGER DEFAULT 100000,
    monthly_spend_limit_usd DECIMAL(10,2),
    current_month_spend_usd DECIMAL(10,2) DEFAULT 0,
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'revoked')),
    last_used_at TIMESTAMP,
    last_used_ip VARCHAR(50),
    expires_at TIMESTAMP,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_status ON api_keys(status);
CREATE INDEX idx_api_keys_expires ON api_keys(expires_at) WHERE expires_at IS NOT NULL;

-- API Usage Tracking Table
CREATE TABLE IF NOT EXISTS api_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    endpoint_id UUID REFERENCES api_endpoints(id) ON DELETE SET NULL,
    feature_id UUID REFERENCES api_features(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES api_providers(id) ON DELETE SET NULL,
    request_path VARCHAR(1000),
    request_method VARCHAR(10),
    request_headers JSONB,
    request_body_size_bytes INTEGER,
    response_status_code INTEGER,
    response_time_ms INTEGER,
    response_body_size_bytes INTEGER,
    cache_hit BOOLEAN DEFAULT FALSE,
    cost_usd DECIMAL(10,6),
    provider_revenue_usd DECIMAL(10,6),
    platform_revenue_usd DECIMAL(10,6),
    error_message TEXT,
    client_ip VARCHAR(50),
    user_agent VARCHAR(500),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_api_usage_user_created ON api_usage(user_id, created_at DESC);
CREATE INDEX idx_api_usage_endpoint_created ON api_usage(endpoint_id, created_at DESC);
CREATE INDEX idx_api_usage_provider_created ON api_usage(provider_id, created_at DESC);
CREATE INDEX idx_api_usage_api_key ON api_usage(api_key_id, created_at DESC);
CREATE INDEX idx_api_usage_created ON api_usage(created_at DESC);

-- Provider Revenue Tracking Table
CREATE TABLE IF NOT EXISTS provider_revenue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES api_providers(id) ON DELETE CASCADE,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    total_calls BIGINT DEFAULT 0,
    successful_calls BIGINT DEFAULT 0,
    failed_calls BIGINT DEFAULT 0,
    total_response_time_ms BIGINT DEFAULT 0,
    gross_revenue_usd DECIMAL(12,2) DEFAULT 0,
    platform_fee_usd DECIMAL(12,2) DEFAULT 0,
    net_revenue_usd DECIMAL(12,2) DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'paid', 'disputed')),
    paid_at TIMESTAMP,
    payment_reference VARCHAR(255),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(provider_id, period_start, period_end)
);

CREATE INDEX idx_provider_revenue_provider ON provider_revenue(provider_id);
CREATE INDEX idx_provider_revenue_period ON provider_revenue(period_start, period_end);
CREATE INDEX idx_provider_revenue_status ON provider_revenue(status);

-- API Documentation Table
CREATE TABLE IF NOT EXISTS api_documentation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID NOT NULL REFERENCES api_endpoints(id) ON DELETE CASCADE,
    content_type VARCHAR(50) DEFAULT 'markdown' CHECK (content_type IN ('markdown', 'html', 'openapi')),
    content TEXT NOT NULL,
    examples JSONB DEFAULT '[]',
    version VARCHAR(50) DEFAULT '1.0',
    is_published BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_api_documentation_endpoint ON api_documentation(endpoint_id);

-- API Webhooks (for event notifications)
CREATE TABLE IF NOT EXISTS api_webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    url VARCHAR(500) NOT NULL,
    events TEXT[] NOT NULL,
    secret VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'paused', 'failed')),
    last_triggered_at TIMESTAMP,
    failure_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_api_webhooks_user ON api_webhooks(user_id);
CREATE INDEX idx_api_webhooks_status ON api_webhooks(status);

-- =====================================================
-- Sample Data: AIServe Provider & APIs
-- =====================================================

-- Insert AIServe.Farm as first-party provider
INSERT INTO api_providers (name, slug, description, website_url, support_email, is_first_party, revenue_share_percent, status)
VALUES
(
    'AIServe.Farm',
    'aiserve',
    'GPU Marketplace and AI Model Inference Platform',
    'https://aiserve.farm',
    'support@aiserve.farm',
    TRUE,
    0, -- 0% revenue share (our own platform)
    'active'
)
ON CONFLICT (slug) DO NOTHING;

-- Insert GPU Service
INSERT INTO api_services (provider_id, name, slug, description, category, base_url, auth_type, version, status)
SELECT
    id,
    'GPU Rental',
    'gpu',
    'Rent high-performance GPUs for AI/ML workloads',
    'Infrastructure',
    'https://api.aiserve.farm/v1/gpu',
    'bearer',
    'v1',
    'active'
FROM api_providers WHERE slug = 'aiserve'
ON CONFLICT (provider_id, slug) DO NOTHING;

-- Insert ML Inference Service
INSERT INTO api_services (provider_id, name, slug, description, category, base_url, auth_type, version, status)
SELECT
    id,
    'ML Inference',
    'ml',
    'Run inference on pre-trained AI models',
    'Machine Learning',
    'https://api.aiserve.farm/v1/ml',
    'bearer',
    'v1',
    'active'
FROM api_providers WHERE slug = 'aiserve'
ON CONFLICT (provider_id, slug) DO NOTHING;

-- Sample GPU Endpoints
INSERT INTO api_endpoints (service_id, path, method, summary, description, base_price_per_call, rate_limit_per_min, status)
SELECT
    s.id,
    '/list',
    'GET',
    'List available GPUs',
    'Get a list of all available GPUs with pricing and specifications',
    0.001,
    60,
    'active'
FROM api_services s
JOIN api_providers p ON s.provider_id = p.id
WHERE p.slug = 'aiserve' AND s.slug = 'gpu'
ON CONFLICT DO NOTHING;

-- =====================================================
-- Functions & Triggers
-- =====================================================

-- Update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at trigger to all tables
CREATE TRIGGER update_api_providers_updated_at BEFORE UPDATE ON api_providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_services_updated_at BEFORE UPDATE ON api_services
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_endpoints_updated_at BEFORE UPDATE ON api_endpoints
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_features_updated_at BEFORE UPDATE ON api_features
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`
