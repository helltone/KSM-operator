# Kubernetes State Monitor Operator Documentation

## Overview

The Kubernetes State Monitor Operator is a custom Kubernetes controller that automatically manages the deployment and scaling of Kubernetes State Monitor pods based on the number of nodes in your cluster. It maintains a configurable ratio of monitor pods to cluster nodes, ensuring optimal monitoring coverage as your cluster scales.

## Architecture

### Core Components

1. **Custom Resource Definition (CRD)**: `StateMonitor`
   - Defines the desired state for monitoring configuration
   - Cluster-scoped resource for global monitoring management

2. **Controller**: Reconciliation loop that:
   - Watches for changes in node count
   - Calculates required monitor pod replicas
   - Creates or deletes monitor pods to maintain desired ratio

3. **Monitor Pods**: State monitoring workloads managed by the operator

## How It Works

### Reconciliation Process

1. **Node Discovery**
   - The operator continuously monitors the cluster's node count
   - Lists all nodes using Kubernetes API with read-only permissions

2. **Replica Calculation**
   - Formula: `desiredReplicas = ceil(nodeCount / nodesPerMonitor)`
   - Default ratio: 1 monitor pod per 15 nodes
   - Minimum: 1 pod if any nodes exist
   - Example scenarios:
     - 10 nodes → 1 monitor pod
     - 15 nodes → 1 monitor pod
     - 16 nodes → 2 monitor pods
     - 30 nodes → 2 monitor pods
     - 31 nodes → 3 monitor pods

3. **Pod Management**
   - **Scale Up**: Creates new pods when node count increases
   - **Scale Down**: Deletes excess pods when node count decreases
   - Pods are created with unique names: `state-monitor-{cr-name}-{index}`
   - All pods are labeled for easy identification and management

4. **Status Updates**
   - Updates StateMonitor status with:
     - Current node count
     - Desired replica count
     - Current replica count
     - Operational phase (Ready/Scaling)
     - Last update timestamp

### Reconciliation Triggers

- Node added or removed from cluster
- StateMonitor CR created, updated, or deleted
- Periodic reconciliation (every 60 seconds)
- Monitor pod creation or deletion

## Security Model

### IRSA (IAM Roles for Service Accounts) on EKS

The operator uses AWS IRSA for secure, minimal-privilege access:

1. **Operator Service Account** (`state-monitor-operator`)
   - Annotated with IAM role ARN
   - Permissions:
     - Read-only access to Kubernetes nodes (cluster-wide)
     - Full management of pods in `monitoring` namespace only
     - Management of StateMonitor CRs

2. **Monitor Pod Service Account** (`state-monitor`)
   - Separate service account for monitor pods
   - Can have different IAM permissions as needed

### RBAC Configuration

#### Cluster-Level Permissions
```yaml
- Read nodes (get, list, watch)
- Manage StateMonitor CRs (all verbs)
```

#### Namespace-Level Permissions (monitoring namespace only)
```yaml
- Manage pods (get, list, watch, create, update, patch, delete)
```

## Configuration

### StateMonitor Custom Resource

```yaml
apiVersion: monitor.example.com/v1alpha1
kind: StateMonitor
metadata:
  name: default
spec:
  nodesPerMonitor: 15              # Nodes per monitor pod (default: 15)
  monitorImage: "monitor:latest"   # Container image for monitor pods
  namespace: monitoring             # Target namespace (default: monitoring)
  resources:                        # Resource requirements for pods
    requests:
      cpu: "100m"
      memory: "150Mi"
    limits:
      cpu: "200m"
      memory: "256Mi"
```

### Environment Variables in Monitor Pods

Each monitor pod receives:
- `MONITOR_INDEX`: Unique index of the pod
- `NODES_PER_MONITOR`: Configured ratio value
- `NODE_NAME`: Name of the node where pod is scheduled

## Deployment

### Prerequisites

1. Kubernetes cluster (EKS recommended for IRSA)
2. `monitoring` namespace created
3. IAM roles configured (for EKS)
4. Required permissions to create CRDs and cluster roles

### Installation Steps

1. **Create namespace**:
   ```bash
   kubectl create namespace monitoring
   ```

2. **Install CRD**:
   ```bash
   kubectl apply -f config/crd/bases/monitor.example.com_statemonitors.yaml
   ```

3. **Configure IRSA** (EKS only):
   - Update service account annotations with your IAM role ARNs
   - Create IAM roles with minimal required permissions

4. **Deploy RBAC**:
   ```bash
   kubectl apply -f config/rbac/
   ```

5. **Deploy operator**:
   ```bash
   kubectl apply -f config/manager/manager.yaml
   ```

6. **Create StateMonitor CR**:
   ```bash
   kubectl apply -f manifests/sample-statemonitor.yaml
   ```

## Monitoring and Observability

### Metrics

The operator exposes metrics on port 8080:
- Controller reconciliation metrics
- Node count metrics
- Pod replica metrics
- Error counters

### Health Checks

- **Liveness probe**: `/healthz` on port 8081
- **Readiness probe**: `/readyz` on port 8081

### Status Information

Check operator status:
```bash
kubectl get statemonitors
```

Example output:
```
NAME      NODES   DESIRED   CURRENT   PHASE   AGE
default   45      3         3         Ready   10m
```

## Operational Scenarios

### Cluster Scale-Up

When nodes are added:
1. Operator detects increased node count
2. Recalculates required replicas
3. Creates additional monitor pods if needed
4. Updates status to reflect new state

### Cluster Scale-Down

When nodes are removed:
1. Operator detects decreased node count
2. Recalculates required replicas
3. Deletes excess monitor pods gracefully
4. Updates status to reflect new state

### Operator Recovery

If the operator pod crashes:
1. Kubernetes restarts the operator
2. On startup, operator reconciles current state
3. Adjusts monitor pods to match desired state
4. Resumes normal operation

### Monitor Pod Failure

If a monitor pod fails:
1. Operator detects missing pod in next reconciliation
2. Creates replacement pod automatically
3. Maintains desired replica count

## Troubleshooting

### Common Issues

1. **Pods not scaling**
   - Check operator logs: `kubectl logs -n monitoring deployment/state-monitor-operator`
   - Verify RBAC permissions
   - Check node visibility: `kubectl get nodes`

2. **IRSA not working**
   - Verify service account annotations
   - Check IAM role trust relationships
   - Validate OIDC provider configuration

3. **Monitor pods failing**
   - Check image availability
   - Verify resource limits are appropriate
   - Review pod events: `kubectl describe pod -n monitoring <pod-name>`

### Debug Commands

```bash
# Check operator logs
kubectl logs -n monitoring deployment/state-monitor-operator -f

# View StateMonitor details
kubectl describe statemonitor default

# List all monitor pods
kubectl get pods -n monitoring -l app=state-monitor

# Check operator metrics
kubectl port-forward -n monitoring deployment/state-monitor-operator 8080:8080
curl http://localhost:8080/metrics
```

## Best Practices

1. **Resource Planning**
   - Set appropriate resource limits based on monitoring workload
   - Consider node capacity when setting ratios

2. **High Availability**
   - Run operator with leader election enabled
   - Use pod disruption budgets for monitor pods

3. **Monitoring**
   - Set up alerts for operator health
   - Monitor reconciliation errors
   - Track scaling events

4. **Security**
   - Use minimal IAM permissions
   - Regularly rotate service account tokens
   - Enable audit logging

## Limitations

1. **Approximate Ratios**: Due to ceiling function, actual ratio may be slightly lower than configured
2. **Namespace Bound**: Monitor pods only created in specified namespace
3. **Single Cluster**: Operator manages pods within single cluster only
4. **Pod-Based**: Uses pods directly, not deployments or statefulsets

## Future Enhancements

Potential improvements:
- Support for DaemonSets for exact node coverage
- Multi-namespace support
- Custom scheduling strategies
- Integration with HPA for dynamic ratio adjustment
- Webhook validation for CR changes
- Support for multiple monitor types