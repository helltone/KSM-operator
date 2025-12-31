# Sharding with Dynamic Replicas - Best of Both Worlds

## Concept Overview

Your proposed solution elegantly solves the scaling vs sharding conflict:

- **Fixed shard count**: Always have N shards (e.g., 3)
- **Dynamic pod count**: Scale pods based on cluster size
- **Minimum pods**: At least N pods (one per shard)
- **Round-robin distribution**: Additional pods are distributed across shards

### Example Scaling Pattern (3 shards)

| Nodes | Calculated Pods | Actual Pods | Shard Distribution |
|-------|----------------|-------------|-------------------|
| 10    | 1              | 3 (minimum) | Shard 0: Pod-0<br>Shard 1: Pod-1<br>Shard 2: Pod-2 |
| 45    | 3              | 3           | Shard 0: Pod-0<br>Shard 1: Pod-1<br>Shard 2: Pod-2 |
| 60    | 4              | 4           | Shard 0: Pod-0, Pod-3<br>Shard 1: Pod-1<br>Shard 2: Pod-2 |
| 75    | 5              | 5           | Shard 0: Pod-0, Pod-3<br>Shard 1: Pod-1, Pod-4<br>Shard 2: Pod-2 |
| 90    | 6              | 6           | Shard 0: Pod-0, Pod-3<br>Shard 1: Pod-1, Pod-4<br>Shard 2: Pod-2, Pod-5 |

## Architecture Benefits

### 1. Complete Metric Coverage
- All shards (0 to N-1) are always covered
- No missing metrics even in small clusters

### 2. Dynamic Scaling
- Pods scale with cluster size
- Additional capacity through replication, not re-sharding

### 3. Load Distribution
- Large clusters get multiple pods per shard
- Each pod in a shard processes the same objects (redundancy)
- Prometheus deduplication handles multiple sources

### 4. High Availability
- Multiple pods per shard provide redundancy
- If Pod-3 fails, Pod-0 still covers shard 0

## Implementation Design

### CRD Structure

```go
type KSMScaleSpec struct {
    // Existing fields
    NodesPerMonitor int32                `json:"nodesPerMonitor,omitempty"`
    MonitorImage    string               `json:"monitorImage,omitempty"`
    Namespace       string               `json:"namespace,omitempty"`
    Resources       ResourceRequirements `json:"resources,omitempty"`
    
    // Enhanced sharding configuration
    Sharding        *ShardingConfig      `json:"sharding,omitempty"`
}

type ShardingConfig struct {
    // Enable sharding mode
    // +kubebuilder:default=false
    Enabled bool `json:"enabled"`
    
    // Fixed number of shards (data partitions)
    // This defines how objects are distributed
    // +kubebuilder:validation:Minimum=2
    // +kubebuilder:validation:Maximum=10
    // +kubebuilder:default=3
    ShardCount int32 `json:"shardCount"`
    
    // Replication strategy for scaling
    ReplicationStrategy string `json:"replicationStrategy,omitempty"`
    // Options: "round-robin" (default), "least-loaded", "zone-aware"
    
    // Minimum replicas per shard (defaults to 1)
    // Total minimum pods = ShardCount * MinReplicasPerShard
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:default=1
    MinReplicasPerShard int32 `json:"minReplicasPerShard,omitempty"`
    
    // Maximum replicas per shard (0 = unlimited)
    // +kubebuilder:default=0
    MaxReplicasPerShard int32 `json:"maxReplicasPerShard,omitempty"`
}
```

### Enhanced Status

```go
type KSMScaleStatus struct {
    // Existing fields
    CurrentReplicas int32       `json:"currentReplicas,omitempty"`
    DesiredReplicas int32       `json:"desiredReplicas,omitempty"`
    NodeCount       int32       `json:"nodeCount,omitempty"`
    Phase           string      `json:"phase,omitempty"`
    LastUpdated     metav1.Time `json:"lastUpdated,omitempty"`
    
    // Enhanced sharding status
    ShardingStatus  *ShardingStatus `json:"shardingStatus,omitempty"`
}

type ShardingStatus struct {
    TotalShards       int32                    `json:"totalShards"`
    ShardDistribution map[string]ShardReplicas `json:"shardDistribution,omitempty"`
    // Example: {"0": {replicas: 2, pods: ["pod-0", "pod-3"]}}
}

type ShardReplicas struct {
    Replicas int32    `json:"replicas"`
    Pods     []string `json:"pods"`
    Ready    int32    `json:"ready"`
}
```

### Controller Implementation

```go
func (r *KSMScaleReconciler) reconcileShardedWithReplicas(
    ctx context.Context,
    ksmScale *monitorv1alpha1.KSMScale,
    nodeCount int,
) (ctrl.Result, error) {
    logger := log.FromContext(ctx)
    
    // Calculate desired pod count based on nodes
    nodesPerMonitor := int(ksmScale.Spec.NodesPerMonitor)
    if nodesPerMonitor == 0 {
        nodesPerMonitor = 15
    }
    
    shardCount := ksmScale.Spec.Sharding.ShardCount
    if shardCount == 0 {
        shardCount = 3
    }
    
    // Calculate desired pods (with minimum enforcement)
    calculatedPods := int32(math.Ceil(float64(nodeCount) / float64(nodesPerMonitor)))
    minPods := shardCount * ksmScale.Spec.Sharding.MinReplicasPerShard
    
    desiredPods := calculatedPods
    if desiredPods < minPods {
        desiredPods = minPods
        logger.Info("Enforcing minimum pods for complete shard coverage",
            "calculated", calculatedPods,
            "minimum", minPods,
            "shards", shardCount)
    }
    
    // Apply maximum limit if configured
    if ksmScale.Spec.Sharding.MaxReplicasPerShard > 0 {
        maxPods := shardCount * ksmScale.Spec.Sharding.MaxReplicasPerShard
        if desiredPods > maxPods {
            desiredPods = maxPods
            logger.Info("Capping at maximum replicas per shard",
                "calculated", calculatedPods,
                "maximum", maxPods)
        }
    }
    
    namespace := ksmScale.Spec.Namespace
    if namespace == "" {
        namespace = "monitoring"
    }
    
    // Get current pods
    currentPods, err := r.getMonitorPods(ctx, ksmScale, namespace)
    if err != nil {
        return ctrl.Result{}, err
    }
    
    // Build current state map: podIndex -> (pod, shardID)
    type podInfo struct {
        pod     *corev1.Pod
        index   int32
        shardID int32
    }
    
    existingPods := make(map[int32]*podInfo)
    for i := range currentPods {
        pod := &currentPods[i]
        if indexStr, ok := pod.Labels["pod-index"]; ok {
            if index := parseInt(indexStr); index >= 0 {
                info := &podInfo{
                    pod:   pod,
                    index: int32(index),
                }
                // Extract shard ID from pod
                if shardStr, ok := pod.Labels["shard-id"]; ok {
                    info.shardID = int32(parseInt(shardStr))
                }
                existingPods[int32(index)] = info
            }
        }
    }
    
    // Calculate shard distribution
    shardAssignments := r.calculateShardDistribution(desiredPods, shardCount)
    
    // Reconcile each pod position
    var errors []error
    for podIndex := int32(0); podIndex < desiredPods; podIndex++ {
        assignedShard := shardAssignments[podIndex]
        
        if existing, exists := existingPods[podIndex]; exists {
            // Verify pod has correct shard assignment
            if existing.shardID != assignedShard {
                logger.Info("Pod has wrong shard assignment, recreating",
                    "pod", existing.pod.Name,
                    "currentShard", existing.shardID,
                    "expectedShard", assignedShard)
                    
                if err := r.Delete(ctx, existing.pod); err != nil {
                    errors = append(errors, err)
                    continue
                }
                // Will be recreated in next reconciliation
            } else if !r.isPodConfigValid(existing.pod, assignedShard, shardCount) {
                logger.Info("Pod configuration outdated, recreating",
                    "pod", existing.pod.Name)
                    
                if err := r.Delete(ctx, existing.pod); err != nil {
                    errors = append(errors, err)
                    continue
                }
            }
        } else {
            // Create missing pod
            logger.Info("Creating pod",
                "index", podIndex,
                "shard", assignedShard,
                "totalShards", shardCount)
                
            if err := r.createReplicatedShardPod(ctx, ksmScale, namespace, 
                podIndex, assignedShard, shardCount); err != nil {
                errors = append(errors, err)
            }
        }
    }
    
    // Remove excess pods
    for podIndex, info := range existingPods {
        if podIndex >= desiredPods {
            logger.Info("Removing excess pod",
                "pod", info.pod.Name,
                "index", podIndex)
                
            if err := r.Delete(ctx, info.pod); err != nil {
                errors = append(errors, err)
            }
        }
    }
    
    // Update status with shard distribution
    if err := r.updateShardDistributionStatus(ctx, ksmScale, 
        nodeCount, desiredPods, shardCount, existingPods); err != nil {
        return ctrl.Result{}, err
    }
    
    // Determine requeue interval
    if len(errors) > 0 || len(existingPods) != int(desiredPods) {
        return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
    }
    
    return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}

func (r *KSMScaleReconciler) calculateShardDistribution(
    totalPods int32, 
    shardCount int32,
) map[int32]int32 {
    assignments := make(map[int32]int32)
    
    // Round-robin distribution
    // Pod 0 -> Shard 0
    // Pod 1 -> Shard 1  
    // Pod 2 -> Shard 2
    // Pod 3 -> Shard 0 (second replica)
    // Pod 4 -> Shard 1 (second replica)
    // etc.
    
    for podIndex := int32(0); podIndex < totalPods; podIndex++ {
        assignments[podIndex] = podIndex % shardCount
    }
    
    return assignments
}

func (r *KSMScaleReconciler) createReplicatedShardPod(
    ctx context.Context,
    ksmScale *monitorv1alpha1.KSMScale,
    namespace string,
    podIndex int32,
    shardID int32,
    totalShards int32,
) error {
    image := ksmScale.Spec.MonitorImage
    if image == "" {
        image = "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
    }
    
    // Calculate replica number for this shard
    replicaNum := podIndex / totalShards
    
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: fmt.Sprintf("kube-state-metrics-%s-%d", 
                ksmScale.Name, podIndex),
            Namespace: namespace,
            Labels: map[string]string{
                "app.kubernetes.io/name":      "kube-state-metrics",
                "app.kubernetes.io/instance":  ksmScale.Name,
                "app.kubernetes.io/component": "exporter",
                "ksmscale.example.com":        ksmScale.Name,
                "pod-index":                   fmt.Sprintf("%d", podIndex),
                "shard-id":                    fmt.Sprintf("%d", shardID),
                "shard-replica":               fmt.Sprintf("%d", replicaNum),
                "total-shards":                fmt.Sprintf("%d", totalShards),
                "sharding-enabled":            "true",
            },
            Annotations: map[string]string{
                "ksmscale.example.com/shard-assignment": fmt.Sprintf(
                    "Shard %d (Replica %d)", shardID, replicaNum),
            },
        },
        Spec: corev1.PodSpec{
            ServiceAccountName: "kube-state-metrics",
            
            // Add topology spread constraints for better distribution
            TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
                {
                    MaxSkew:           1,
                    TopologyKey:       "kubernetes.io/hostname",
                    WhenUnsatisfiable: corev1.DoNotSchedule,
                    LabelSelector: &metav1.LabelSelector{
                        MatchLabels: map[string]string{
                            "ksmscale.example.com": ksmScale.Name,
                            "shard-id":             fmt.Sprintf("%d", shardID),
                        },
                    },
                },
            },
            
            // Add anti-affinity to spread replicas of same shard
            Affinity: &corev1.Affinity{
                PodAntiAffinity: &corev1.PodAntiAffinity{
                    PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
                        {
                            Weight: 100,
                            PodAffinityTerm: corev1.PodAffinityTerm{
                                LabelSelector: &metav1.LabelSelector{
                                    MatchLabels: map[string]string{
                                        "ksmscale.example.com": ksmScale.Name,
                                        "shard-id":             fmt.Sprintf("%d", shardID),
                                    },
                                },
                                TopologyKey: "kubernetes.io/hostname",
                            },
                        },
                    },
                },
            },
            
            Containers: []corev1.Container{
                {
                    Name:  "kube-state-metrics",
                    Image: image,
                    Args: []string{
                        "--host=0.0.0.0",
                        "--port=8080",
                        "--telemetry-host=0.0.0.0",
                        "--telemetry-port=8081",
                        fmt.Sprintf("--shard=%d", shardID),
                        fmt.Sprintf("--total-shards=%d", totalShards),
                    },
                    Env: []corev1.EnvVar{
                        {
                            Name:  "SHARD_ID",
                            Value: fmt.Sprintf("%d", shardID),
                        },
                        {
                            Name:  "REPLICA_NUM", 
                            Value: fmt.Sprintf("%d", replicaNum),
                        },
                        {
                            Name: "POD_NAME",
                            ValueFrom: &corev1.EnvVarSource{
                                FieldRef: &corev1.ObjectFieldSelector{
                                    FieldPath: "metadata.name",
                                },
                            },
                        },
                    },
                    Ports: []corev1.ContainerPort{
                        {
                            Name:          "http-metrics",
                            ContainerPort: 8080,
                            Protocol:      corev1.ProtocolTCP,
                        },
                        {
                            Name:          "telemetry",
                            ContainerPort: 8081,
                            Protocol:      corev1.ProtocolTCP,
                        },
                    },
                    LivenessProbe: &corev1.Probe{
                        ProbeHandler: corev1.ProbeHandler{
                            HTTPGet: &corev1.HTTPGetAction{
                                Path: "/healthz",
                                Port: intstr.FromInt(8080),
                            },
                        },
                        InitialDelaySeconds: 5,
                        TimeoutSeconds:      5,
                    },
                    ReadinessProbe: &corev1.Probe{
                        ProbeHandler: corev1.ProbeHandler{
                            HTTPGet: &corev1.HTTPGetAction{
                                Path: "/",
                                Port: intstr.FromInt(8080),
                            },
                        },
                        InitialDelaySeconds: 5,
                        TimeoutSeconds:      5,
                    },
                    Resources: r.getResourceRequirements(ksmScale),
                    SecurityContext: &corev1.SecurityContext{
                        AllowPrivilegeEscalation: &[]bool{false}[0],
                        Capabilities: &corev1.Capabilities{
                            Drop: []corev1.Capability{"ALL"},
                        },
                        ReadOnlyRootFilesystem: &[]bool{true}[0],
                        RunAsNonRoot:          &[]bool{true}[0],
                        RunAsUser:             &[]int64{65534}[0],
                    },
                },
            },
        },
    }
    
    if err := controllerutil.SetControllerReference(ksmScale, pod, r.Scheme); err != nil {
        return err
    }
    
    if err := r.Create(ctx, pod); err != nil {
        if !errors.IsAlreadyExists(err) {
            return err
        }
    }
    
    return nil
}

func (r *KSMScaleReconciler) updateShardDistributionStatus(
    ctx context.Context,
    ksmScale *monitorv1alpha1.KSMScale,
    nodeCount int,
    desiredPods int32,
    shardCount int32,
    existingPods map[int32]*podInfo,
) error {
    // Build shard distribution map
    distribution := make(map[string]monitorv1alpha1.ShardReplicas)
    
    for shardID := int32(0); shardID < shardCount; shardID++ {
        shardKey := fmt.Sprintf("%d", shardID)
        replicas := monitorv1alpha1.ShardReplicas{
            Pods: []string{},
        }
        
        // Count pods for this shard
        for _, info := range existingPods {
            if info.shardID == shardID {
                replicas.Replicas++
                replicas.Pods = append(replicas.Pods, info.pod.Name)
                if isPodReady(info.pod) {
                    replicas.Ready++
                }
            }
        }
        
        distribution[shardKey] = replicas
    }
    
    // Update status
    ksmScale.Status.NodeCount = int32(nodeCount)
    ksmScale.Status.DesiredReplicas = desiredPods
    ksmScale.Status.CurrentReplicas = int32(len(existingPods))
    ksmScale.Status.LastUpdated = metav1.Now()
    
    ksmScale.Status.ShardingStatus = &monitorv1alpha1.ShardingStatus{
        TotalShards:       shardCount,
        ShardDistribution: distribution,
    }
    
    // Determine phase
    allShardsHaveReplicas := true
    for shardID := int32(0); shardID < shardCount; shardID++ {
        shardKey := fmt.Sprintf("%d", shardID)
        if dist, ok := distribution[shardKey]; !ok || dist.Replicas == 0 {
            allShardsHaveReplicas = false
            break
        }
    }
    
    if !allShardsHaveReplicas {
        ksmScale.Status.Phase = "Incomplete"
    } else if int32(len(existingPods)) != desiredPods {
        ksmScale.Status.Phase = "Scaling"
    } else {
        ksmScale.Status.Phase = "Ready"
    }
    
    return r.Status().Update(ctx, ksmScale)
}
```

## Prometheus Configuration

### Handling Duplicate Metrics

With multiple pods serving the same shard, Prometheus will receive duplicate metrics. Handle this with:

#### Option 1: Prometheus Deduplication (Recommended)

```yaml
# ServiceMonitor configuration
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
    # Add shard_id label
    - sourceLabels: [__meta_kubernetes_pod_label_shard_id]
      targetLabel: shard
    # Keep pod name for debugging
    - sourceLabels: [__meta_kubernetes_pod_name]
      targetLabel: pod
```

Then use Prometheus recording rules to deduplicate:

```yaml
groups:
- name: ksm_deduplication
  interval: 30s
  rules:
  # Deduplicate pod metrics by taking max across replicas
  - record: kube_pod_info_dedup
    expr: |
      max by (namespace, pod, uid, node, created_by_kind, created_by_name) (
        kube_pod_info
      )
  
  # Deduplicate node metrics
  - record: kube_node_info_dedup
    expr: |
      max by (node, kernel_version, os_image, container_runtime_version) (
        kube_node_info
      )
```

#### Option 2: Service-Level Load Balancing

Create services per shard:

```go
func (r *KSMScaleReconciler) ensureShardServices(
    ctx context.Context,
    ksmScale *monitorv1alpha1.KSMScale,
    namespace string,
    shardCount int32,
) error {
    for shardID := int32(0); shardID < shardCount; shardID++ {
        svc := &corev1.Service{
            ObjectMeta: metav1.ObjectMeta{
                Name: fmt.Sprintf("ksm-%s-shard-%d", ksmScale.Name, shardID),
                Namespace: namespace,
            },
            Spec: corev1.ServiceSpec{
                Selector: map[string]string{
                    "ksmscale.example.com": ksmScale.Name,
                    "shard-id": fmt.Sprintf("%d", shardID),
                },
                Ports: []corev1.ServicePort{
                    {
                        Name:       "metrics",
                        Port:       8080,
                        TargetPort: intstr.FromInt(8080),
                    },
                },
            },
        }
        
        if err := controllerutil.SetControllerReference(ksmScale, svc, r.Scheme); err != nil {
            return err
        }
        
        // Create or update service
        // ... implementation
    }
    return nil
}
```

## Example Configurations

### Small Cluster with Sharding (Minimum 3 Pods)

```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: small-sharded
spec:
  nodesPerMonitor: 15  # Would calculate to 1 pod for 15 nodes
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
  namespace: monitoring
  
  sharding:
    enabled: true
    shardCount: 3           # 3 shards
    minReplicasPerShard: 1  # Minimum 3 pods total (1 per shard)
    # Result: 3 pods minimum even for small cluster
```

### Medium Cluster with Scaling

```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: medium-sharded
spec:
  nodesPerMonitor: 15  # 75 nodes / 15 = 5 pods
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
  namespace: monitoring
  
  sharding:
    enabled: true
    shardCount: 3           # 3 shards
    minReplicasPerShard: 1  # Minimum 3 pods
    # Result: 5 pods distributed as:
    # Shard 0: 2 pods
    # Shard 1: 2 pods  
    # Shard 2: 1 pod
```

### Large Cluster with Limits

```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: large-sharded
spec:
  nodesPerMonitor: 15  # 300 nodes / 15 = 20 pods
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
  namespace: monitoring
  
  sharding:
    enabled: true
    shardCount: 5           # 5 shards
    minReplicasPerShard: 1  # Minimum 5 pods
    maxReplicasPerShard: 5  # Maximum 25 pods total (5 per shard)
    # Result: 20 pods distributed as:
    # Each shard gets 4 pods
```

## Monitoring and Observability

### Key Metrics

```promql
# Pods per shard
count by (shard_id) (
  up{job="kube-state-metrics"}
)

# Check shard coverage
count(
  count by (shard_id) (up{job="kube-state-metrics"})
) == <expected_shard_count>

# Metrics per shard (after deduplication)
sum by (shard) (
  rate(kube_state_metrics_scraped_total[5m])
)

# Load balance check
stddev by () (
  count by (shard_id) (
    kube_pod_info
  )
) < 1000  # Threshold for imbalance
```

### Alert Rules

```yaml
groups:
- name: ksm-shard-replicas
  rules:
  - alert: KSMShardUncovered
    expr: |
      count(
        count by (shard_id) (up{job="kube-state-metrics"} == 1)
      ) < ksmscale_configured_shards
    for: 5m
    annotations:
      summary: "KSM shard {{ $labels.shard_id }} has no active replicas"
      
  - alert: KSMShardImbalanced
    expr: |
      max(
        count by (shard_id) (up{job="kube-state-metrics"})
      ) - 
      min(
        count by (shard_id) (up{job="kube-state-metrics"})
      ) > 2
    for: 15m
    annotations:
      summary: "KSM shard replicas are imbalanced"
```

## Migration Path

### From Non-Sharded to Sharded with Replicas

1. **Start with current deployment**
   ```bash
   kubectl get ksmscale my-ksm -o yaml > backup.yaml
   ```

2. **Add sharding configuration**
   ```yaml
   spec:
     sharding:
       enabled: true
       shardCount: 3
       minReplicasPerShard: 1
   ```

3. **Monitor rollout**
   ```bash
   # Watch pods being created
   kubectl get pods -l ksmscale.example.com=my-ksm -w
   
   # Verify shard assignments
   kubectl get pods -l ksmscale.example.com=my-ksm \
     -o custom-columns=NAME:.metadata.name,SHARD:.metadata.labels.shard-id,INDEX:.metadata.labels.pod-index
   ```

4. **Validate complete coverage**
   ```bash
   # Should see 3 different shard IDs
   kubectl get pods -l ksmscale.example.com=my-ksm \
     -o jsonpath='{.items[*].metadata.labels.shard-id}' | tr ' ' '\n' | sort -u
   ```

## Advantages of This Approach

1. **Best of Both Worlds**: Fixed sharding for predictability, dynamic scaling for efficiency
2. **No Metric Gaps**: All shards always covered
3. **Gradual Scaling**: Smooth scaling as cluster grows
4. **High Availability**: Natural redundancy through replication
5. **Load Distribution**: Round-robin ensures even distribution
6. **Operational Simplicity**: No complex resharding on scale events
7. **Resource Efficiency**: Scale pods based on actual need while maintaining coverage

## Considerations

### Resource Usage
- Multiple pods for same shard means redundant processing
- Each pod still processes all K8s objects (filters by hash)
- Network traffic multiplied by replica count
- Prometheus must handle deduplication

### Scheduling
- Use topology spread constraints to distribute replicas
- Anti-affinity prevents replicas on same node
- Consider zone-aware placement for HA

### Cost-Benefit Analysis
- **Small clusters** (<30 nodes): May have 3 pods instead of 1 (3x resources)
- **Medium clusters** (30-100 nodes): Optimal resource usage
- **Large clusters** (100+ nodes): Better than non-sharded single instance

## Conclusion

This implementation provides:
- **Complete metric coverage** at all scales
- **Dynamic scaling** based on cluster size
- **High availability** through replication
- **Operational safety** with predictable behavior
- **Smooth migration** from existing deployments

The round-robin distribution ensures even load distribution while maintaining the simplicity of fixed sharding. This is the optimal solution for production Kubernetes clusters of varying sizes.