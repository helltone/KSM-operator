# Local Development Guide

This guide explains how to run the Kubernetes State Monitor Operator locally using kind (Kubernetes in Docker) to test kube-state-metrics scaling.

## Prerequisites

1. **Docker** - Required for running kind
2. **kind** - Local Kubernetes cluster tool
   ```bash
   # macOS
   brew install kind
   
   # Linux/Windows
   # See: https://kind.sigs.k8s.io/docs/user/quick-start/#installation
   ```
3. **kubectl** - Kubernetes CLI
   ```bash
   # macOS
   brew install kubectl
   ```
4. **Go 1.21+** - For building the operator

## Quick Start

The easiest way to get started:

```bash
# Complete setup: create cluster, build, deploy, and create sample resource
make dev

# Watch operator logs
make logs-kind
```

This will:
1. Create a 4-node kind cluster (1 control plane + 3 workers)
2. Build the operator Docker image
3. Load the image into kind
4. Deploy the operator
5. Create a sample StateMonitor resource

## Step-by-Step Setup

### 1. Create Kind Cluster

```bash
# Create a kind cluster with 4 nodes
make kind-cluster

# Or use the setup script
./scripts/kind-setup.sh
```

This creates a cluster with:
- 1 control plane node
- 3 worker nodes
- Cluster name: `state-monitor-dev`

### 2. Build and Load Operator Image

```bash
# Build Docker image and load into kind
make kind-load
```

This builds the operator with tag `state-monitor-operator:dev` and loads it into the kind cluster.

### 3. Deploy the Operator

```bash
# Deploy operator with development configuration
make deploy-kind
```

This will:
- Create the `monitoring` namespace
- Install CRDs
- Deploy RBAC resources (without IRSA annotations)
- Deploy the operator with `imagePullPolicy: Never` (uses local image)

### 4. Create a Sample StateMonitor

```bash
# Deploy development sample (2 nodes per monitor for testing)
make sample-dev
```

This creates a StateMonitor configured for local development:
- Ratio: 1 kube-state-metrics pod per 9 nodes (minimum allowed)
- Image: kube-state-metrics alpine version (lightweight for testing)  
- Minimal resource requirements
- Deployed to monitoring namespace for isolation

### 5. Verify Deployment

```bash
# Check operator is running
kubectl get pods -n monitoring

# Check StateMonitor status
kubectl get statemonitors

# Expected output with 4 nodes and 1:9 ratio = 1 kube-state-metrics pod
NAME      NODES   DESIRED   CURRENT   PHASE   AGE
dev-ksm   4       1         1         Ready   1m

# Check kube-state-metrics pods
kubectl get pods -n monitoring -l app.kubernetes.io/name=kube-state-metrics
```

## Development Workflow

### Making Code Changes

1. Make your code changes
2. Rebuild and redeploy:
   ```bash
   make kind-load deploy-kind
   ```

3. Check logs:
   ```bash
   make logs-kind
   ```

### Running Tests Locally

```bash
# Run unit tests
make test

# Run integration tests against kind cluster
kubectl config use-context kind-state-monitor-dev
make test-e2e
```

### Debugging

```bash
# Get operator logs
kubectl logs -n monitoring deployment/state-monitor-operator

# Describe StateMonitor
kubectl describe statemonitor dev-ksm

# Get events
kubectl get events -n monitoring --sort-by='.lastTimestamp'

# Port forward for kube-state-metrics  
kubectl port-forward -n monitoring service/kube-state-metrics 8080:8080
curl http://localhost:8080/metrics

# Port forward for operator metrics
kubectl port-forward -n monitoring deployment/state-monitor-operator 8080:8080
```

## Local Development Commands Reference

| Command | Description |
|---------|-------------|
| `make dev` | Complete setup (cluster + deploy + sample) |
| `make kind-cluster` | Create kind cluster |
| `make kind-delete` | Delete kind cluster |
| `make docker-build-kind` | Build Docker image for kind |
| `make kind-load` | Build and load image into kind |
| `make deploy-kind` | Deploy operator to kind |
| `make undeploy-kind` | Remove operator from kind |
| `make sample-dev` | Create sample StateMonitor |
| `make logs-kind` | Stream operator logs |
| `make clean-dev` | Clean up everything |

## Configuration Differences

### Production (EKS with IRSA)
- ServiceAccounts have IRSA annotations
- Uses production kube-state-metrics images
- Higher resource limits
- Deployed to kube-system namespace
- Standard 1:15 node ratio

### Local Development (kind)
- No IRSA annotations
- Uses local Docker images (`imagePullPolicy: Never`)
- Lower resource limits
- Uses kube-state-metrics alpine images
- Deployed to monitoring namespace for isolation
- Minimum 1:9 node ratio for testing

## Customizing for Development

### Adjust Node Count

Edit `config/kind/cluster.yaml` to add/remove nodes:

```yaml
nodes:
- role: control-plane
- role: worker
- role: worker  # Add more workers as needed
```

### Change Monitor Ratio

Edit `manifests/sample-statemonitor-dev.yaml`:

```yaml
spec:
  nodesPerMonitor: 10  # 1:10 ratio for testing scaling
```

### Use Different kube-state-metrics Version

```yaml
spec:
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.9.2"
```

## Troubleshooting

### Issue: Cluster already exists
```bash
make kind-delete
make kind-cluster
```

### Issue: Image not found
```bash
# Ensure operator image is loaded into kind
docker images | grep state-monitor-operator
make kind-load

# Check if kube-state-metrics image is available
kubectl describe pod -n monitoring -l app.kubernetes.io/name=kube-state-metrics
```

### Issue: Pods stuck in Pending
```bash
# Check pod events
kubectl describe pod -n monitoring <pod-name>

# Usually resource constraints - check node resources
kubectl top nodes
```

### Issue: CRD not found
```bash
# Apply CRDs manually
kubectl apply -f config/crd/bases/
```

### Issue: Permission denied
```bash
# Check RBAC for operator
kubectl auth can-i list nodes --as=system:serviceaccount:monitoring:state-monitor-operator
kubectl auth can-i create pods --as=system:serviceaccount:monitoring:state-monitor-operator -n monitoring

# Check RBAC for kube-state-metrics (if deployed to kube-system)
kubectl auth can-i list pods --as=system:serviceaccount:kube-system:kube-state-metrics
```

## Advanced Testing

### Simulate Node Scaling

```bash
# Add a node
docker run -d --name kind-worker4 --network kind kindest/node:v1.28.0
kind get nodes

# Remove a node  
docker stop kind-worker3
docker rm kind-worker3
```

### Test Multiple StateMonitors

```bash
# Create multiple monitors with different ratios
kubectl apply -f - <<EOF
apiVersion: monitor.example.com/v1alpha1
kind: StateMonitor
metadata:
  name: test-monitor-1
spec:
  nodesPerMonitor: 1
  monitorImage: "busybox:latest"
  namespace: monitoring
---
apiVersion: monitor.example.com/v1alpha1
kind: StateMonitor
metadata:
  name: test-monitor-2
spec:
  nodesPerMonitor: 3
  monitorImage: "busybox:latest"
  namespace: monitoring
EOF
```

## Clean Up

```bash
# Remove everything
make clean-dev

# Or manually:
make undeploy-kind
make kind-delete
```

## Tips for Development

1. **Use minimum ratios** (1:9) for easier scaling testing
2. **Watch logs** continuously: `make logs-kind`
3. **Use describe** liberally: `kubectl describe statemonitor`
4. **Check events**: `kubectl get events -n monitoring -w`
5. **Test edge cases**: 0 nodes, 1 node, many nodes
6. **Monitor metrics**: Access kube-state-metrics at http://localhost:8080/metrics
7. **Check health**: Verify liveness/readiness probes are working

## Next Steps

- Implement webhook validation
- Add Prometheus metrics scraping
- Test with chaos engineering tools
- Performance testing with many nodes