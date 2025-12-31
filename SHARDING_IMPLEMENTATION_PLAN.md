# KSM Scale Operator - Sharding Implementation Plan

## Executive Summary

This document outlines the implementation plan for adding **manual sharding** support to the KSM Scale Operator. Based on research, we will implement a **manual sharding strategy** using individual Pods with explicit shard configuration, avoiding auto-sharding due to its risks and limitations.

## Why Manual Sharding Over Auto-Sharding

### Auto-Sharding Risks:
1. **Deletion Risk**: Auto-sharding with StatefulSet can be accidentally deleted as a single resource
2. **Rollout Issues**: StatefulSet replaces pods one-by-one, causing metric collection gaps
3. **Downtime**: Each shard experiences downtime during rollouts
4. **Less Control**: Limited flexibility in managing individual shards

### Manual Sharding Benefits:
1. **Resilience**: Individual pods/deployments harder to accidentally delete all at once
2. **Granular Control**: Each shard can be managed independently
3. **Zero-Downtime Updates**: Can perform rolling updates without metric gaps
4. **Better Observability**: Easier to monitor and troubleshoot individual shards

## Technical Background

### How KSM Sharding Works

Kube-state-metrics uses a deterministic sharding algorithm:
1. Takes MD5 hash of Kubernetes object's UID
2. Performs modulo operation with total shard count
3. Each shard only exports metrics for objects matching its shard number

### Required Configuration

Each KSM instance needs:
- `--shard=N` (zero-indexed shard number)
- `--total-shards=M` (total number of shards)

Example: With 3 shards, you'd have:
- Pod 0: `--shard=0 --total-shards=3`
- Pod 1: `--shard=1 --total-shards=3`
- Pod 2: `--shard=2 --total-shards=3`

## Implementation Strategy

### Phase 1: Update CRD Structure

#### Current KSMScaleSpec:
```go
type KSMScaleSpec struct {
    NodesPerMonitor int32                `json:"nodesPerMonitor,omitempty"`
    MonitorImage    string               `json:"monitorImage,omitempty"`
    Namespace       string               `json:"namespace,omitempty"`
    Resources       ResourceRequirements `json:"resources,omitempty"`
}
```

#### Enhanced KSMScaleSpec with Sharding:
```go
type KSMScaleSpec struct {
    // Existing fields
    NodesPerMonitor int32                `json:"nodesPerMonitor,omitempty"`
    MonitorImage    string               `json:"monitorImage,omitempty"`
    Namespace       string               `json:"namespace,omitempty"`
    Resources       ResourceRequirements `json:"resources,omitempty"`
    
    // New sharding configuration
    Sharding        *ShardingConfig      `json:"sharding,omitempty"`
}

type ShardingConfig struct {
    // Enable sharding mode
    // +kubebuilder:default=false
    Enabled bool `json:"enabled"`
    
    // Sharding strategy: "manual" only for now
    // +kubebuilder:validation:Enum=manual
    // +kubebuilder:default="manual"
    Strategy string `json:"strategy,omitempty"`
    
    // Number of shards (overrides NodesPerMonitor calculation)
    // +kubebuilder:validation:Minimum=2
    // +kubebuilder:validation:Maximum=10
    ShardCount int32 `json:"shardCount,omitempty"`
    
    // Advanced configuration
    // +optional
    Advanced *AdvancedShardingConfig `json:"advanced,omitempty"`
}

type AdvancedShardingConfig struct {
    // Pod anti-affinity to spread shards across nodes
    // +kubebuilder:default=true
    SpreadAcrossNodes bool `json:"spreadAcrossNodes,omitempty"`
    
    // Add pod disruption budget for shards
    // +kubebuilder:default=true
    EnablePDB bool `json:"enablePDB,omitempty"`
    
    // Minimum available shards (for PDB)
    // +kubebuilder:default=1
    MinAvailable int32 `json:"minAvailable,omitempty"`
}
```

### Phase 2: Update Status Structure

```go
type KSMScaleStatus struct {
    // Existing fields
    CurrentReplicas int32       `json:"currentReplicas,omitempty"`
    DesiredReplicas int32       `json:"desiredReplicas,omitempty"`
    NodeCount       int32       `json:"nodeCount,omitempty"`
    Phase           string      `json:"phase,omitempty"`
    LastUpdated     metav1.Time `json:"lastUpdated,omitempty"`
    
    // New sharding status
    ShardingStatus  *ShardingStatus `json:"shardingStatus,omitempty"`
}

type ShardingStatus struct {
    TotalShards    int32             `json:"totalShards"`
    ActiveShards   int32             `json:"activeShards"`
    ShardHealth    []ShardHealth     `json:"shardHealth,omitempty"`
}

type ShardHealth struct {
    ShardID    int32  `json:"shardId"`
    PodName    string `json:"podName"`
    Ready      bool   `json:"ready"`
    LastSeen   metav1.Time `json:"lastSeen"`
}
```

### Phase 3: Controller Implementation Changes

#### 3.1 Pod Naming Strategy

With sharding enabled, pods will be named:
```
kube-state-metrics-{ksmscale-name}-shard-{shard-id}
```

Example: `kube-state-metrics-production-shard-0`

#### 3.2 Modified Pod Creation Logic

```go
func (r *KSMScaleReconciler) createShardedMonitorPod(
    ctx context.Context, 
    ksmScale *monitorv1alpha1.KSMScale, 
    namespace string, 
    shardID int32,
    totalShards int32,
) error {
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: fmt.Sprintf("kube-state-metrics-%s-shard-%d", 
                ksmScale.Name, shardID),
            Namespace: namespace,
            Labels: map[string]string{
                "app.kubernetes.io/name":      "kube-state-metrics",
                "app.kubernetes.io/instance":  ksmScale.Name,
                "app.kubernetes.io/component": "exporter",
                "ksmscale.example.com":        ksmScale.Name,
                "shard-id":                    fmt.Sprintf("%d", shardID),
                "total-shards":                fmt.Sprintf("%d", totalShards),
                "sharding-enabled":            "true",
            },
        },
        Spec: corev1.PodSpec{
            ServiceAccountName: "kube-state-metrics",
            Containers: []corev1.Container{
                {
                    Name:  "kube-state-metrics",
                    Image: ksmScale.Spec.MonitorImage,
                    Args: []string{
                        "--host=0.0.0.0",
                        "--port=8080",
                        "--telemetry-host=0.0.0.0",
                        "--telemetry-port=8081",
                        fmt.Sprintf("--shard=%d", shardID),
                        fmt.Sprintf("--total-shards=%d", totalShards),
                    },
                    // ... rest of container spec
                },
            },
        },
    }
    
    // Add anti-affinity if configured
    if ksmScale.Spec.Sharding.Advanced != nil && 
       ksmScale.Spec.Sharding.Advanced.SpreadAcrossNodes {
        pod.Spec.Affinity = &corev1.Affinity{
            PodAntiAffinity: &corev1.PodAntiAffinity{
                PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
                    {
                        Weight: 100,
                        PodAffinityTerm: corev1.PodAffinityTerm{
                            LabelSelector: &metav1.LabelSelector{
                                MatchLabels: map[string]string{
                                    "ksmscale.example.com": ksmScale.Name,
                                    "sharding-enabled":     "true",
                                },
                            },
                            TopologyKey: "kubernetes.io/hostname",
                        },
                    },
                },
            },
        }
    }
    
    // Set owner reference and create
    if err := controllerutil.SetControllerReference(ksmScale, pod, r.Scheme); err != nil {
        return err
    }
    
    return r.Create(ctx, pod)
}
```

#### 3.3 Reconciliation Logic Updates

```go
func (r *KSMScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // ... existing code ...
    
    // Determine if sharding is enabled
    if ksmScale.Spec.Sharding != nil && ksmScale.Spec.Sharding.Enabled {
        return r.reconcileSharded(ctx, ksmScale)
    }
    
    // Fall back to existing non-sharded logic
    return r.reconcileNonSharded(ctx, ksmScale)
}

func (r *KSMScaleReconciler) reconcileSharded(
    ctx context.Context, 
    ksmScale *monitorv1alpha1.KSMScale,
) (ctrl.Result, error) {
    logger := log.FromContext(ctx)
    
    // Get desired shard count
    desiredShards := ksmScale.Spec.Sharding.ShardCount
    if desiredShards == 0 {
        desiredShards = 3 // Default to 3 shards
    }
    
    namespace := ksmScale.Spec.Namespace
    if namespace == "" {
        namespace = "monitoring"
    }
    
    // Get current sharded pods
    currentPods, err := r.getShardedMonitorPods(ctx, ksmScale, namespace)
    if err != nil {
        return ctrl.Result{}, err
    }
    
    // Build shard map
    existingShards := make(map[int32]corev1.Pod)
    for _, pod := range currentPods {
        if shardIDStr, ok := pod.Labels["shard-id"]; ok {
            if shardID := parseInt(shardIDStr); shardID >= 0 {
                existingShards[int32(shardID)] = pod
            }
        }
    }
    
    // Reconcile each shard
    var shardHealth []monitorv1alpha1.ShardHealth
    activeShards := int32(0)
    
    for shardID := int32(0); shardID < desiredShards; shardID++ {
        if pod, exists := existingShards[shardID]; exists {
            // Shard exists, check health
            health := monitorv1alpha1.ShardHealth{
                ShardID:  shardID,
                PodName:  pod.Name,
                Ready:    isPodReady(&pod),
                LastSeen: metav1.Now(),
            }
            shardHealth = append(shardHealth, health)
            if health.Ready {
                activeShards++
            }
            
            // Verify shard configuration is correct
            if !r.isShardConfigCorrect(&pod, shardID, desiredShards) {
                logger.Info("Shard configuration mismatch, recreating", 
                    "shard", shardID, "pod", pod.Name)
                if err := r.Delete(ctx, &pod); err != nil {
                    return ctrl.Result{}, err
                }
                // Will be recreated in next reconciliation
            }
        } else {
            // Create missing shard
            logger.Info("Creating shard", "shardID", shardID)
            if err := r.createShardedMonitorPod(ctx, ksmScale, namespace, 
                shardID, desiredShards); err != nil {
                logger.Error(err, "Failed to create shard", "shardID", shardID)
                return ctrl.Result{}, err
            }
        }
    }
    
    // Delete excess shards
    for shardID, pod := range existingShards {
        if shardID >= desiredShards {
            logger.Info("Deleting excess shard", "shardID", shardID, "pod", pod.Name)
            if err := r.Delete(ctx, &pod); err != nil {
                return ctrl.Result{}, err
            }
        }
    }
    
    // Create or update PodDisruptionBudget if configured
    if ksmScale.Spec.Sharding.Advanced != nil && 
       ksmScale.Spec.Sharding.Advanced.EnablePDB {
        if err := r.reconcilePDB(ctx, ksmScale, namespace); err != nil {
            logger.Error(err, "Failed to reconcile PodDisruptionBudget")
            // Don't fail reconciliation for PDB errors
        }
    }
    
    // Update status
    ksmScale.Status.ShardingStatus = &monitorv1alpha1.ShardingStatus{
        TotalShards:  desiredShards,
        ActiveShards: activeShards,
        ShardHealth:  shardHealth,
    }
    
    // Update phase
    if activeShards == desiredShards {
        ksmScale.Status.Phase = "Ready"
    } else {
        ksmScale.Status.Phase = "Scaling"
    }
    
    ksmScale.Status.LastUpdated = metav1.Now()
    
    if err := r.Status().Update(ctx, ksmScale); err != nil {
        logger.Error(err, "Failed to update KSMScale status")
        return ctrl.Result{}, err
    }
    
    // Requeue more frequently during scaling, less frequently when stable
    if activeShards != desiredShards {
        return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
    }
    return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}
```

### Phase 4: Supporting Resources

#### 4.1 Service Configuration

Create a headless service for sharded pods:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kube-state-metrics-sharded
  namespace: monitoring
  labels:
    app.kubernetes.io/name: kube-state-metrics
spec:
  clusterIP: None  # Headless service
  selector:
    app.kubernetes.io/name: kube-state-metrics
    sharding-enabled: "true"
  ports:
  - name: http-metrics
    port: 8080
    targetPort: 8080
  - name: telemetry
    port: 8081
    targetPort: 8081
```

#### 4.2 PodDisruptionBudget

```go
func (r *KSMScaleReconciler) reconcilePDB(
    ctx context.Context,
    ksmScale *monitorv1alpha1.KSMScale,
    namespace string,
) error {
    minAvailable := ksmScale.Spec.Sharding.Advanced.MinAvailable
    if minAvailable == 0 {
        minAvailable = 1
    }
    
    pdb := &policyv1.PodDisruptionBudget{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("kube-state-metrics-%s-pdb", ksmScale.Name),
            Namespace: namespace,
        },
        Spec: policyv1.PodDisruptionBudgetSpec{
            MinAvailable: &intstr.IntOrString{
                Type:   intstr.Int,
                IntVal: minAvailable,
            },
            Selector: &metav1.LabelSelector{
                MatchLabels: map[string]string{
                    "ksmscale.example.com": ksmScale.Name,
                    "sharding-enabled":     "true",
                },
            },
        },
    }
    
    if err := controllerutil.SetControllerReference(ksmScale, pdb, r.Scheme); err != nil {
        return err
    }
    
    // Create or update PDB
    return r.createOrUpdate(ctx, pdb)
}
```

### Phase 5: Monitoring and Observability

#### 5.1 Prometheus Scraping Configuration

Each shard needs to be scraped individually. Example ServiceMonitor:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kube-state-metrics-sharded
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: kube-state-metrics
  endpoints:
  - port: http-metrics
    interval: 30s
    relabelings:
    - sourceLabels: [__meta_kubernetes_pod_label_shard_id]
      targetLabel: shard
    - sourceLabels: [__meta_kubernetes_pod_label_total_shards]
      targetLabel: total_shards
```

#### 5.2 Health Check Queries

Prometheus queries to monitor shard health:

```promql
# Check if all shards are up
count(up{job="kube-state-metrics"}) == <expected_shard_count>

# Check metrics distribution across shards
sum by (shard) (kube_pod_info)

# Alert on missing shards
absent(up{job="kube-state-metrics", shard="0"})
```

## Implementation Timeline

### Week 1-2: Foundation
- [ ] Update CRD with sharding configuration
- [ ] Generate new CRD manifests
- [ ] Update types and deepcopy code

### Week 3-4: Core Logic
- [ ] Implement sharded pod creation
- [ ] Implement reconcileSharded function
- [ ] Add shard health tracking

### Week 5: Supporting Features
- [ ] Add PodDisruptionBudget support
- [ ] Implement pod anti-affinity
- [ ] Create headless service

### Week 6: Testing & Documentation
- [ ] Unit tests for sharding logic
- [ ] Integration tests
- [ ] Update documentation
- [ ] Create example manifests

## Example Usage

### Basic Sharding Configuration

```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: production-ksm-sharded
spec:
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
  namespace: monitoring
  
  sharding:
    enabled: true
    strategy: manual
    shardCount: 3
    advanced:
      spreadAcrossNodes: true
      enablePDB: true
      minAvailable: 2
  
  resources:
    requests:
      cpu: "100m"
      memory: "150Mi"
    limits:
      cpu: "200m"
      memory: "300Mi"
```

### Non-Sharded (Backward Compatible)

```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: development-ksm
spec:
  nodesPerMonitor: 30
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
  namespace: monitoring
  # No sharding section - uses legacy scaling
```

## Monitoring & Alerts

### Key Metrics to Track

1. **Shard Availability**
   - All shards running: `ksmscale_shards_active / ksmscale_shards_total == 1`
   - Individual shard health

2. **Metric Distribution**
   - Metrics per shard should be roughly equal
   - No shard should have 0 metrics

3. **Resource Usage**
   - Memory usage per shard
   - CPU usage per shard
   - Network traffic per shard

### Alert Rules

```yaml
groups:
- name: ksm-sharding
  rules:
  - alert: KSMShardDown
    expr: ksmscale_shards_active < ksmscale_shards_total
    for: 5m
    annotations:
      summary: "KSM shard is down"
      
  - alert: KSMShardImbalance
    expr: |
      max(kube_pod_info) by (shard) / 
      min(kube_pod_info) by (shard) > 2
    for: 15m
    annotations:
      summary: "KSM shards have imbalanced load"
```

## Migration Strategy

### From Non-Sharded to Sharded

1. Deploy new KSMScale resource with sharding enabled
2. Verify all shards are healthy
3. Update Prometheus scrape configs
4. Validate metrics collection
5. Delete old non-sharded deployment

### Rollback Plan

1. Keep non-sharded configuration as backup
2. If issues arise, disable sharding in CRD
3. Operator will automatically revert to non-sharded mode
4. Clean up sharded pods

## Testing Strategy

### Unit Tests
- Shard calculation logic
- Pod creation with correct flags
- Health status tracking
- PDB creation

### Integration Tests
- Deploy with 3 shards
- Verify each shard gets correct configuration
- Test scaling up/down shards
- Test pod failure recovery

### Load Tests
- Verify metric distribution
- Monitor resource usage
- Test with large clusters (1000+ nodes)

## Security Considerations

1. **RBAC**: No additional RBAC needed beyond existing pod management
2. **Network Policies**: Each shard needs same network access as before
3. **Pod Security**: Maintain existing security context

## Future Enhancements

1. **Auto-Scaling Shards**: Automatically adjust shard count based on cluster size
2. **Shard Weights**: Allow uneven distribution for heterogeneous workloads
3. **Resource-Based Sharding**: Shard based on resource types instead of UIDs
4. **Cross-Shard Coordination**: Add leader election for shard coordination

## Conclusion

This manual sharding implementation provides:
- **Safety**: Individual pods prevent accidental mass deletion
- **Control**: Granular management of each shard
- **Reliability**: Better failure isolation
- **Observability**: Easier monitoring and debugging

The implementation maintains backward compatibility while adding powerful sharding capabilities for large-scale deployments.