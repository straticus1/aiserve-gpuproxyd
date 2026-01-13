#!/bin/bash

# Test script for MCP server
# Usage: ./scripts/test-mcp.sh <method> [tool_name] [arguments]

API_KEY="${API_KEY:-your-api-key-here}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
METHOD="${1:-initialize}"
TOOL_NAME="${2:-}"
ARGUMENTS="${3:-{}}"

case "$METHOD" in
  initialize)
    curl -X POST "${BASE_URL}/api/v1/mcp" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: ${API_KEY}" \
      -d '{
        "jsonrpc": "2.0",
        "method": "initialize",
        "params": {},
        "id": 1
      }' | jq .
    ;;

  tools/list)
    curl -X POST "${BASE_URL}/api/v1/mcp" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: ${API_KEY}" \
      -d '{
        "jsonrpc": "2.0",
        "method": "tools/list",
        "params": {},
        "id": 2
      }' | jq .
    ;;

  tools/call)
    if [ -z "$TOOL_NAME" ]; then
      echo "Error: tool name required"
      echo "Usage: $0 tools/call <tool_name> [arguments]"
      exit 1
    fi

    curl -X POST "${BASE_URL}/api/v1/mcp" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: ${API_KEY}" \
      -d "{
        \"jsonrpc\": \"2.0\",
        \"method\": \"tools/call\",
        \"params\": {
          \"name\": \"${TOOL_NAME}\",
          \"arguments\": ${ARGUMENTS}
        },
        \"id\": 3
      }" | jq .
    ;;

  resources/list)
    curl -X POST "${BASE_URL}/api/v1/mcp" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: ${API_KEY}" \
      -d '{
        "jsonrpc": "2.0",
        "method": "resources/list",
        "params": {},
        "id": 4
      }' | jq .
    ;;

  resources/read)
    if [ -z "$TOOL_NAME" ]; then
      echo "Error: resource URI required"
      echo "Usage: $0 resources/read <uri>"
      exit 1
    fi

    curl -X POST "${BASE_URL}/api/v1/mcp" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: ${API_KEY}" \
      -d "{
        \"jsonrpc\": \"2.0\",
        \"method\": \"resources/read\",
        \"params\": {
          \"uri\": \"${TOOL_NAME}\"
        },
        \"id\": 5
      }" | jq .
    ;;

  *)
    echo "Unknown method: $METHOD"
    echo "Available methods:"
    echo "  initialize"
    echo "  tools/list"
    echo "  tools/call <tool_name> [arguments]"
    echo "  resources/list"
    echo "  resources/read <uri>"
    exit 1
    ;;
esac
