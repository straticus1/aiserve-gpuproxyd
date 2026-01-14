#!/bin/bash
# deploy.sh - Build and deploy AIServe.Farm website to OCI Kubernetes
set -e

# Configuration
REGISTRY="us-ashburn-1.ocir.io"
NAMESPACE="idd2oizp8xvc"
REPO="web3dns/aiserve-farm"
IMAGE="${REGISTRY}/${NAMESPACE}/${REPO}"
TAG="${1:-latest}"
FULL_IMAGE="${IMAGE}:${TAG}"

echo "=== AIServe.Farm Kubernetes Deployment ==="
echo ""
echo "Image: ${FULL_IMAGE}"
echo "Namespace: default"
echo ""

# Step 1: Build Docker image
echo "[1/4] Building Docker image..."
cd "$(dirname "$0")/.."

docker build -f k8s/Dockerfile.web -t "${FULL_IMAGE}" .

if [ $? -ne 0 ]; then
    echo "Error: Docker build failed"
    exit 1
fi

echo "  Image built successfully"
echo ""

# Step 2: Login to OCI Registry
echo "[2/4] Logging in to OCI Registry..."

# Check if already logged in
if ! docker info 2>/dev/null | grep -q "Registry: ${REGISTRY}"; then
    echo "  Attempting OCI registry login..."

    # Try to get credentials from OCI CLI
    if command -v oci &> /dev/null; then
        AUTH_TOKEN=$(oci iam auth-token list --user-id "$(oci iam user list --query 'data[0].id' --raw-output)" --query 'data[0]."token"' --raw-output 2>/dev/null || echo "")

        if [ -n "$AUTH_TOKEN" ]; then
            OCI_USER=$(oci iam user list --query 'data[0].name' --raw-output)
            echo "$AUTH_TOKEN" | docker login -u "${NAMESPACE}/${OCI_USER}" --password-stdin "${REGISTRY}"
        fi
    fi

    # If OCI CLI login failed, prompt for manual login
    if [ $? -ne 0 ]; then
        echo "  Please login to OCI Registry manually:"
        echo "  docker login ${REGISTRY}"
        echo "  Username format: ${NAMESPACE}/<your-oci-username>"
        echo "  Password: Your OCI auth token"
        exit 1
    fi
fi

echo "  Logged in successfully"
echo ""

# Step 3: Push to registry
echo "[3/4] Pushing image to registry..."
docker push "${FULL_IMAGE}"

if [ $? -ne 0 ]; then
    echo "Error: Docker push failed"
    exit 1
fi

echo "  Image pushed successfully"
echo ""

# Step 4: Deploy to Kubernetes
echo "[4/4] Deploying to Kubernetes..."

# Apply the deployment
kubectl apply -f k8s/deployment.yaml

if [ $? -ne 0 ]; then
    echo "Error: Kubernetes deployment failed"
    exit 1
fi

echo "  Deployment applied successfully"
echo ""

# Wait for rollout
echo "Waiting for rollout to complete..."
kubectl rollout status deployment/modeltrack -n default --timeout=300s

if [ $? -ne 0 ]; then
    echo "Error: Rollout failed or timed out"
    echo ""
    echo "Check pod status with:"
    echo "  kubectl get pods -n default | grep modeltrack"
    echo ""
    echo "Check logs with:"
    echo "  kubectl logs -n default -l app=modeltrack --tail=50"
    exit 1
fi

echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Website URL: https://aiserve.farm"
echo ""
echo "Check status:"
echo "  kubectl get pods -n default | grep modeltrack"
echo "  kubectl get svc modeltrack -n default"
echo "  kubectl get ingress aiserve-farm-ingress -n default"
echo ""
echo "View logs:"
echo "  kubectl logs -n default -l app=modeltrack --tail=50 -f"
echo ""
