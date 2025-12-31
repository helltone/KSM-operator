# KSM Scale Operator

A Kubernetes operator that automatically scales kube-state-metrics pods based on cluster node count, maintaining a configurable ratio of nodes per kube-state-metrics pod.

## Features

- Automatic scaling of kube-state-metrics pods based on node count
- Configurable node-to-pod ratio (default: 1 pod per 15 nodes)
- Production-ready kube-state-metrics configuration with health checks
- IRSA support for EKS clusters
- Minimal RBAC permissions
- Standard Kubernetes labels and security context

## Quick Start

### Prerequisites

- Kubernetes cluster (1.24+) or kind for local development
- kubectl configured
- Go 1.21+ (for local development)
- Docker or Podman (for building images)

### Local Development (kind)

For local development without IRSA:

```bash
# Quick setup with kind
make dev

# Watch operator logs
make logs-kind
```

See [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) for detailed local development instructions.

### Production Installation (EKS)

1. Clone the repository:
```bash
git clone <repository-url>
cd ksm-scale-operator
```

2. Configure IRSA (if using EKS):
   - Update `config/production/service_account.yaml` with your IAM role ARNs
   - Ensure the IAM roles have minimal required permissions

3. Install the operator:
```bash
make deploy
```

4. Create a KSMScale resource:
```bash
kubectl apply -f manifests/sample-ksmscale.yaml
```

4. Verify the operator is running:
```bash
kubectl get pods -n monitoring
kubectl get ksmscales
```

## Configuration

Edit the `manifests/sample-ksmscale.yaml` file to customize:

- `nodesPerMonitor`: Number of nodes per kube-state-metrics pod (minimum: 9)
- `monitorImage`: kube-state-metrics container image (default: registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1)
- `namespace`: Target namespace for kube-state-metrics pods (default: kube-system)
- `resources`: CPU and memory limits/requests

## Development

### Building

```bash
make build
```

### Running Locally

```bash
make run
```

### Building Docker Image

```bash
make docker-build
```

### Running Tests

```bash
make test
```

## RBAC Configuration

The operator requires specific RBAC permissions to function correctly:

### Operator Permissions
- **Cluster-wide**: Read access to nodes for scaling decisions
- **Namespace-specific**: Full control over kube-state-metrics pods in the configured namespace
- **CRD Management**: Full access to KSMScale custom resources

### kube-state-metrics Permissions
The kube-state-metrics pods require read access to various Kubernetes resources to expose metrics:
- Core resources: pods, nodes, services, configmaps, secrets, etc.
- Apps resources: deployments, daemonsets, statefulsets, replicasets
- Batch resources: jobs, cronjobs
- Networking resources: ingresses, networkpolicies
- Storage resources: storageclasses, volumeattachments

### Environment-Specific Configuration

#### Local Development (kind)
- Uses standard Kubernetes ServiceAccounts without IRSA
- RBAC is automatically applied via `make dev`
- Service accounts created in `monitoring` namespace

#### Production (EKS with IRSA)
- Configure IAM roles for both operator and kube-state-metrics
- Update `config/production/service_account.yaml` with your IAM role ARNs
- IAM roles should have minimal permissions following the principle of least privilege
- RBAC ClusterRoles/ClusterRoleBindings provide Kubernetes-level permissions

## Documentation

For detailed documentation, see [OPERATOR_DOCUMENTATION.md](OPERATOR_DOCUMENTATION.md)

## License

Apache 2.0