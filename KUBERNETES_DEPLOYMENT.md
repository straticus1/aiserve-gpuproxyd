# AIServe.Farm Kubernetes Deployment Guide

**Date**: 2026-01-13
**Status**: Ready for Deployment

## Discovery Summary

aiserve.farm (129.80.158.147) **IS NOT** a standalone OCI compute instance. It's running as a **containerized workload in Oracle Kubernetes Engine (OKE)**.

This explains why:
- SSH connections to 129.80.158.147 failed (it's a load balancer IP, not a compute instance)
- The IP didn't show up in `oci compute instance list` (not a VM)
- Direct server access wasn't possible (no SSH daemon on the LB)

## Current Production Environment

### Kubernetes Resources

**Namespace**: `default`

**Deployment**: `modeltrack`
```yaml
- Name: modeltrack
- Replicas: 2 pods
- Image: us-ashburn-1.ocir.io/idd2oizp8xvc/web3dns/modeltrack:latest
- Container Port: 3000
- Resources:
    Requests: 100m CPU, 128Mi memory
    Limits: 500m CPU, 512Mi memory
- Health Checks: HTTP GET / on port 3000
```

**Service**: `modeltrack`
```yaml
- Type: ClusterIP
- Cluster IP: 10.96.134.196
- Port: 80 → 3000 (targetPort)
- Selector: app=modeltrack
```

**Ingress**: `aiserve-farm-ingress`
```yaml
- Class: nginx
- Hosts:
    - aiserve.farm
    - www.aiserve.farm
- Backend Service: modeltrack:80
- TLS:
    Secret: aiserve-farm-tls (managed by cert-manager)
    Issuer: letsencrypt-prod
- Load Balancer IP: 129.80.158.147
```

### Pod Status
```bash
$ kubectl get pods -n default | grep modeltrack
modeltrack-586c6c5f8-jrdmq   1/1  Running  0  11d
modeltrack-586c6c5f8-mv774   1/1  Running  0  11d
```

### OKE Cluster Information

Based on `/Users/ryan/development/oci-observability/terraform/`:
- **VCN CIDR**: 10.0.0.0/16
- **Subnets**:
  - API Endpoint: 10.0.0.0/24 (public)
  - Worker Nodes: 10.0.1.0/24 (private)
  - Load Balancer: 10.0.2.0/24 (public)
- **Kubernetes Version**: v1.28.2
- **Node Pool**: 3 worker nodes (VM.Standard.E4.Flex)
- **Pod Network**: 10.244.0.0/16
- **Service Network**: 10.96.0.0/16

## New Deployment Architecture

### Files Created

1. **k8s/Dockerfile.web** - Nginx-based container for static website
   - Base: `nginx:alpine`
   - Port: 8080 (non-root)
   - Health Check: `/health` endpoint
   - Security: Runs as nginx user (UID 101)

2. **k8s/nginx.conf** - Nginx configuration
   - Serves static files from /usr/share/nginx/html
   - SPA routing with `try_files`
   - Security headers
   - Asset caching (1 year for static files)

3. **k8s/deployment.yaml** - Kubernetes manifests
   - Deployment with 2 replicas
   - Rolling update strategy (maxSurge: 1, maxUnavailable: 0)
   - Health probes on /health endpoint
   - SecurityContext for non-root execution

4. **k8s/deploy.sh** - Automated deployment script
   - Builds Docker image
   - Logs in to OCI Registry
   - Pushes image to us-ashburn-1.ocir.io
   - Applies k8s manifests
   - Waits for rollout completion

### Image Registry

- **Registry**: us-ashburn-1.ocir.io
- **Namespace**: idd2oizp8xvc
- **Repository**: web3dns/aiserve-farm
- **Full Image**: `us-ashburn-1.ocir.io/idd2oizp8xvc/web3dns/aiserve-farm:latest`

### Key Changes from Old to New

| Aspect | Old (modeltrack) | New (aiserve-farm) |
|--------|------------------|---------------------|
| Base Image | Node.js app | nginx:alpine |
| Port | 3000 | 8080 |
| Type | Dynamic SSR | Static SPA |
| User | root | nginx (UID 101) |
| Health Check | GET / | GET /health |
| Image Name | modeltrack | aiserve-farm |

## Deployment Steps

### Prerequisites

1. **Docker** - Must be running and authenticated to OCI Registry
2. **kubectl** - Configured with access to OKE cluster
3. **OCI CLI** - For registry authentication (optional, manual login works too)

### Deploy New Website

```bash
cd /Users/ryan/development/aiserve-gpuproxyd

# Build, push, and deploy in one command
./k8s/deploy.sh

# Or specify a tag
./k8s/deploy.sh v1.0.0
```

### Manual Deployment Steps

If the automated script doesn't work:

```bash
# 1. Build image
docker build -f k8s/Dockerfile.web -t us-ashburn-1.ocir.io/idd2oizp8xvc/web3dns/aiserve-farm:latest .

# 2. Login to OCI Registry
docker login us-ashburn-1.ocir.io
# Username: idd2oizp8xvc/<your-oci-username>
# Password: <your-oci-auth-token>

# 3. Push image
docker push us-ashburn-1.ocir.io/idd2oizp8xvc/web3dns/aiserve-farm:latest

# 4. Apply k8s manifests
kubectl apply -f k8s/deployment.yaml

# 5. Watch rollout
kubectl rollout status deployment/modeltrack -n default
```

### Rollback to Previous Version

```bash
# Rollback deployment
kubectl rollout undo deployment/modeltrack -n default

# Check rollout status
kubectl rollout status deployment/modeltrack -n default
```

## Verification

### Check Deployment Status

```bash
# Pods
kubectl get pods -n default -l app=modeltrack

# Deployment
kubectl get deployment modeltrack -n default

# Service
kubectl get svc modeltrack -n default

# Ingress
kubectl get ingress aiserve-farm-ingress -n default

# Logs
kubectl logs -n default -l app=modeltrack --tail=50 -f
```

### Test Website

```bash
# Check health endpoint (from inside cluster)
kubectl run -it --rm test --image=curlimages/curl --restart=Never -- \
  curl http://modeltrack.default.svc.cluster.local/health

# Check external access
curl -I https://aiserve.farm

# Check website content
curl https://aiserve.farm | grep -i "AIServe"
```

### Expected Results

- **HTTP Status**: 200 OK
- **Content-Type**: text/html
- **Title**: Contains "AIServe.Farm"
- **SSL**: Valid Let's Encrypt certificate
- **Response Time**: < 500ms

## Troubleshooting

### Image Pull Errors

```bash
# Check image pull secret
kubectl get secrets -n default | grep regcred

# If missing, create it:
kubectl create secret docker-registry regcred \
  --docker-server=us-ashburn-1.ocir.io \
  --docker-username=idd2oizp8xvc/<your-username> \
  --docker-password=<your-auth-token> \
  -n default

# Update deployment to use secret
kubectl patch deployment modeltrack -n default \
  -p '{"spec":{"template":{"spec":{"imagePullSecrets":[{"name":"regcred"}]}}}}'
```

### Pods Not Starting

```bash
# Describe pod for events
kubectl describe pod -n default -l app=modeltrack

# Check logs
kubectl logs -n default -l app=modeltrack --all-containers --tail=100

# Common issues:
# 1. Port binding conflicts (use 8080, not 80)
# 2. Permission denied (run as nginx user)
# 3. Health check failures (ensure /health endpoint exists)
```

### Ingress Not Working

```bash
# Check ingress controller
kubectl get pods -n ingress-nginx

# Check ingress configuration
kubectl describe ingress aiserve-farm-ingress -n default

# Check load balancer
kubectl get svc -n ingress-nginx

# Test from inside cluster
kubectl run -it --rm test --image=curlimages/curl --restart=Never -- \
  curl -H "Host: aiserve.farm" http://modeltrack.default.svc.cluster.local/
```

### SSL Certificate Issues

```bash
# Check certificate
kubectl get certificate -n default

# Check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager --tail=100

# Check certificate details
kubectl describe certificate aiserve-farm-tls -n default

# Force certificate renewal
kubectl delete certificate aiserve-farm-tls -n default
# Cert-manager will automatically recreate it
```

## Security Considerations

### Non-Root Execution

The new container runs as the `nginx` user (UID 101), not root. This follows Kubernetes security best practices.

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 101
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
```

### Network Policies

Consider adding network policies to restrict pod-to-pod communication:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: modeltrack-netpol
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: modeltrack
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
```

## Monitoring

### Metrics

The deployment exports metrics for monitoring:

```bash
# CPU usage
kubectl top pod -n default -l app=modeltrack

# Memory usage
kubectl top pod -n default -l app=modeltrack

# Request rate (via Prometheus)
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Then query: rate(nginx_http_requests_total{namespace="default",pod=~"modeltrack.*"}[5m])
```

### Logs

Logs are automatically collected by the cluster logging system (if configured):

```bash
# Recent logs
kubectl logs -n default -l app=modeltrack --tail=100

# Follow logs
kubectl logs -n default -l app=modeltrack -f

# Logs from all replicas
kubectl logs -n default -l app=modeltrack --all-containers --prefix
```

## Next Steps

1. ✅ Created Dockerfile and k8s manifests
2. ⏳ Build and push Docker image to OCI Registry
3. ⏳ Deploy to Kubernetes cluster
4. ⏳ Verify deployment at https://aiserve.farm
5. Monitor for issues and performance
6. Set up alerts for downtime/errors
7. Configure backup/disaster recovery

## Related Documentation

- [DEPLOYMENT_STATUS.md](./DEPLOYMENT_STATUS.md) - Overall deployment status
- [OCI_NETWORK_CONFIGURATION.md](./OCI_NETWORK_CONFIGURATION.md) - OCI network troubleshooting (obsolete now that we know it's k8s)
- [oci-observability Terraform](../oci-observability/terraform/) - OKE cluster infrastructure

## Contact

**DevOps Team**: ryan@afterdarksys.com
**Platform**: Oracle Cloud Infrastructure (OCI)
**Region**: us-ashburn-1
**Cluster**: observability-oke-production
