#!/bin/bash
# AIProxy Quick Start Script
# Helps you configure and start AIProxy with Cloudflare Workers AI

set -e

echo "=========================================="
echo "  AIProxy Quick Start"
echo "=========================================="
echo ""

# Check if Cloudflare credentials are set
if [ -z "$CLOUDFLARE_ACCOUNT_ID" ]; then
    echo "❌ CLOUDFLARE_ACCOUNT_ID not set"
    echo ""
    echo "Please set your Cloudflare credentials:"
    echo "  export CLOUDFLARE_ACCOUNT_ID='your-account-id'"
    echo "  export CLOUDFLARE_API_TOKEN='your-api-token'"
    echo ""
    echo "Get them from: https://dash.cloudflare.com/"
    exit 1
fi

if [ -z "$CLOUDFLARE_API_TOKEN" ]; then
    echo "❌ CLOUDFLARE_API_TOKEN not set"
    echo ""
    echo "Please set your Cloudflare API token:"
    echo "  export CLOUDFLARE_API_TOKEN='your-api-token'"
    echo ""
    echo "Get it from: https://dash.cloudflare.com/"
    exit 1
fi

echo "✅ Cloudflare credentials found"
echo "   Account ID: $CLOUDFLARE_ACCOUNT_ID"
echo "   API Token: ${CLOUDFLARE_API_TOKEN:0:10}..."
echo ""

# Create config file
CONFIG_FILE="aiproxy.yaml"

if [ -f "$CONFIG_FILE" ]; then
    echo "⚠️  $CONFIG_FILE already exists"
    read -p "Overwrite? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Using existing $CONFIG_FILE"
    else
        rm "$CONFIG_FILE"
        echo "✅ Removed old $CONFIG_FILE"
    fi
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo "📝 Creating $CONFIG_FILE..."
    cat > "$CONFIG_FILE" << 'EOF'
node:
  id: "local-node"
  type: "standalone"

server:
  listen_addr: "0.0.0.0:8080"

providers:
  cloudflare:
    enabled: true
    priority: 1
    credentials:
      account_id: "${CLOUDFLARE_ACCOUNT_ID}"
      api_token: "${CLOUDFLARE_API_TOKEN}"
    models:
      - name: "llama-3.1-8b"
        cloudflare_model: "@cf/meta/llama-3.1-8b-instruct"
        capabilities: ["text-generation"]
        cost_per_1k_tokens: 0.001

      - name: "qwen-coder"
        cloudflare_model: "@cf/qwen/qwen2.5-coder-7b-instruct"
        capabilities: ["code-generation"]
        cost_per_1k_tokens: 0.001

routing:
  strategy: "cost_optimized"
  failover:
    enabled: true
    max_retries: 3

observability:
  logging:
    level: "info"
    format: "json"
EOF
    echo "✅ Created $CONFIG_FILE"
fi

echo ""
echo "=========================================="
echo "  Configuration Summary"
echo "=========================================="
echo "Listen address: 0.0.0.0:8080"
echo "Providers: Cloudflare Workers AI"
echo "Models:"
echo "  - llama-3.1-8b (text generation)"
echo "  - qwen-coder (code generation)"
echo "Routing: cost_optimized"
echo ""

# Build if needed
if [ ! -f "./aiserve-gpuproxyd" ]; then
    echo "🔨 Building aiserve-gpuproxyd..."
    go build -o aiserve-gpuproxyd ./cmd/server
    echo "✅ Build complete"
    echo ""
fi

echo "=========================================="
echo "  Starting AIProxy Server"
echo "=========================================="
echo ""
echo "Server will start on: http://localhost:8080"
echo ""
echo "Test with:"
echo "  curl http://localhost:8080/v1/chat/completions \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"model\": \"llama-3.1-8b\", \"messages\": [{\"role\": \"user\", \"content\": \"Hello!\"}]}'"
echo ""
echo "Or see docs/AIPROXY_GETTING_STARTED.md for more examples"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""
echo "=========================================="
echo ""

# Start server
./aiserve-gpuproxyd --aiproxy-config "$CONFIG_FILE"
