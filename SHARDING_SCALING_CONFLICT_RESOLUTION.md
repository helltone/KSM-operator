# Sharding vs. Dynamic Scaling: Conflict Resolution

## The Problem

You correctly identified a fundamental conflict in the sharding design:

1. **Sharding requires** all shards (0 to N-1) to be running with the same `--total-shards=N` configuration
2. **Dynamic scaling wants** to adjust pod count based on node count (e.g., 1 pod per 15 nodes)
3. **Conflict**: What happens when we need 1 pod but have 3 shards configured?

### Problematic Scenarios

#### Scenario 1: Less Pods Than Shards
- **Cluster size**: 10 nodes
- **Calculated pods needed**: 1 (at 15 nodes per pod)
- **Configured shards**: 3
- **Problem**: Running only 1 pod with `--shard=0 --total-shards=3` means 2/3 of objects have NO metrics!

#### Scenario 2: Mismatched Shard Configuration
- **Pod 1**: Running with `--total-shards=3`
- **Scale down**: Now need only 1 pod
- **Pod 1 reconfigured**: `--total-shards=1`
- **Problem**: Complete reconfiguration required, metrics disruption

#### Scenario 3: Partial Shard Coverage
- Running shard 0 and 1 out of 3 total shards
- Missing shard 2 means ~33% of metrics are not collected
- Prometheus queries return incomplete data

## Solution Options

### Option 1: Fixed Pods When Sharding Enabled (Recommended)

**Concept**: When sharding is enabled, the number of pods equals the shard count, ignoring the node-based calculation.

```go
type ShardingConfig struct {
    Enabled bool `json:"enabled"`
    
    // When set, this becomes the fixed number of pods
    // Ignores NodesPerMonitor calculation
    ShardCount int32 `json:"shardCount"`
    
    // Minimum shards to maintain even if calculated pods < shardCount
    // +kubebuilder:validation:Minimum=2
    // +kubebuilder:default=3
    MinimumShards int32 `json:"minimumShards,omitempty"`
}
```

**Reconciliation Logic**:
```go
func (r *KSMScaleReconciler) calculateDesiredPods(
    ksmScale *monitorv1alpha1.KSMScale,
    nodeCount int,
) int32 {
    if ksmScale.Spec.Sharding != nil && ksmScale.Spec.Sharding.Enabled {
        // With sharding, pod count = shard count (fixed)
        return ksmScale.Spec.Sharding.ShardCount
    }
    
    // Without sharding, use dynamic calculation
    nodesPerMonitor := int(ksmScale.Spec.NodesPerMonitor)
    if nodesPerMonitor == 0 {
        nodesPerMonitor = 15
    }
    
    desiredPods := int32(math.Ceil(float64(nodeCount) / float64(nodesPerMonitor)))
    if desiredPods == 0 && nodeCount > 0 {
        desiredPods = 1
    }
    
    return desiredPods
}
```

**Pros**:
- Simple and predictable
- Guarantees complete metric coverage
- No complex shard reconfiguration

**Cons**:
- Loses dynamic scaling benefit
- May over-provision for small clusters
- May under-provision for large clusters

### Option 2: Dynamic Sharding with Minimum Guarantee

**Concept**: Allow shard count to change, but enforce minimum pods = configured shards.

```go
type ShardingConfig struct {
    Enabled bool `json:"enabled"`
    
    // Base shard count (minimum pods)
    BaseShardCount int32 `json:"baseShardCount"`
    
    // Allow dynamic scaling above base
    AllowDynamicExpansion bool `json:"allowDynamicExpansion"`
    
    // If true, shards expand but never contract below base
    PreventContraction bool `json:"preventContraction"`
}
```

**Implementation**:
```go
func (r *KSMScaleReconciler) calculateShardsAndPods(
    ksmScale *monitorv1alpha1.KSMScale,
    nodeCount int,
) (pods int32, shards int32) {
    baseShards := ksmScale.Spec.Sharding.BaseShardCount
    if baseShards == 0 {
        baseShards = 3
    }
    
    // Calculate desired pods based on nodes
    calculatedPods := int32(math.Ceil(float64(nodeCount) / float64(ksmScale.Spec.NodesPerMonitor)))
    
    if !ksmScale.Spec.Sharding.AllowDynamicExpansion {
        // Fixed sharding
        return baseShards, baseShards
    }
    
    // Dynamic sharding with minimum
    if calculatedPods < baseShards {
        return baseShards, baseShards
    }
    
    // Scale shards with pods
    return calculatedPods, calculatedPods
}
```

**Challenge**: When shards change, ALL pods need reconfiguration!

### Option 3: Hybrid Approach - Shard Groups (Most Flexible)

**Concept**: Combine sharding with replication. Each "shard group" handles a portion of objects, and within each group, pods are replicated.

```go
type ShardingConfig struct {
    Enabled bool `json:"enabled"`
    
    // Number of logical shards (data partitions)
    ShardGroups int32 `json:"shardGroups"`
    
    // Scaling strategy within groups
    ScalingStrategy string `json:"scalingStrategy"` // "fixed" or "dynamic"
    
    // For dynamic: min replicas per shard group
    MinReplicasPerGroup int32 `json:"minReplicasPerGroup,omitempty"`
    
    // For dynamic: nodes per replica within a group
    NodesPerReplica int32 `json:"nodesPerReplica,omitempty"`
}
```

**Architecture**:
```
Shard Group 0 (handles objects with hash % 3 == 0):
  - Pod 0-0: --shard=0 --total-shards=3
  - Pod 0-1: --shard=0 --total-shards=3 (replica)

Shard Group 1 (handles objects with hash % 3 == 1):
  - Pod 1-0: --shard=1 --total-shards=3
  - Pod 1-1: --shard=1 --total-shards=3 (replica)

Shard Group 2 (handles objects with hash % 3 == 2):
  - Pod 2-0: --shard=2 --total-shards=3
```

**Note**: This requires careful service/endpoint configuration for Prometheus scraping to avoid duplicate metrics.

### Option 4: Smart Sharding with Incomplete Coverage Acceptance

**Concept**: Allow incomplete shard coverage with clear status reporting.

```go
type ShardingConfig struct {
    Enabled bool `json:"enabled"`
    
    // Maximum shards (defines the sharding space)
    MaxShards int32 `json:"maxShards"`
    
    // Accept incomplete coverage below this node count
    AcceptIncompleteBelowNodes int32 `json:"acceptIncompleteBelowNodes"`
    
    // Minimum coverage percentage required
    MinCoveragePercent int32 `json:"minCoveragePercent"`
}
```

**Status Reporting**:
```go
type ShardingStatus struct {
    ConfiguredShards  int32   `json:"configuredShards"`
    ActiveShards     int32   `json:"activeShards"`
    CoveragePercent  float64 `json:"coveragePercent"`
    MissingShards    []int32 `json:"missingShards,omitempty"`
    Warning          string  `json:"warning,omitempty"`
}
```

## Recommended Implementation: Option 1 with Enhancements

Based on the analysis, I recommend **Option 1 (Fixed Pods)** with these enhancements:

### Enhanced CRD Structure

```go
type KSMScaleSpec struct {
    // Existing fields
    NodesPerMonitor int32                `json:"nodesPerMonitor,omitempty"`
    MonitorImage    string               `json:"monitorImage,omitempty"`
    Namespace       string               `json:"namespace,omitempty"`
    Resources       ResourceRequirements `json:"resources,omitempty"`
    
    // Sharding configuration
    Sharding        *ShardingConfig      `json:"sharding,omitempty"`
}

type ShardingConfig struct {
    // Enable sharding mode
    Enabled bool `json:"enabled"`
    
    // REQUIRED when enabled: Fixed number of shards/pods
    // This overrides NodesPerMonitor calculation
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Minimum=2
    // +kubebuilder:validation:Maximum=10
    ShardCount int32 `json:"shardCount"`
    
    // Recommended node ranges for shard counts
    // (For user guidance, not enforced)
    // 2 shards: 15-50 nodes
    // 3 shards: 50-100 nodes
    // 5 shards: 100-200 nodes
    // 10 shards: 200+ nodes
    
    // Strategy for pod distribution
    Distribution DistributionConfig `json:"distribution,omitempty"`
}

type DistributionConfig struct {
    // Spread shards across availability zones
    SpreadAcrossZones bool `json:"spreadAcrossZones,omitempty"`
    
    // Spread shards across nodes
    SpreadAcrossNodes bool `json:"spreadAcrossNodes,omitempty"`
    
    // Enforce pod disruption budget
    // +kubebuilder:default=true
    EnablePDB bool `json:"enablePDB,omitempty"`
    
    // Maximum unavailable shards during disruption
    // +kubebuilder:default=1
    MaxUnavailable int32 `json:"maxUnavailable,omitempty"`
}
```

### Controller Implementation

```go
func (r *KSMScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := log.FromContext(ctx)
    
    // Fetch KSMScale resource
    ksmScale := &monitorv1alpha1.KSMScale{}
    if err := r.Get(ctx, req.NamespacedName, ksmScale); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // Count nodes for status reporting
    nodeList := &corev1.NodeList{}
    if err := r.List(ctx, nodeList); err != nil {
        return ctrl.Result{}, err
    }
    nodeCount := len(nodeList.Items)
    
    // Determine operational mode
    if ksmScale.Spec.Sharding != nil && ksmScale.Spec.Sharding.Enabled {
        return r.reconcileShardedMode(ctx, ksmScale, nodeCount)
    }
    
    return r.reconcileDynamicMode(ctx, ksmScale, nodeCount)
}

func (r *KSMScaleReconciler) reconcileShardedMode(
    ctx context.Context,
    ksmScale *monitorv1alpha1.KSMScale,
    nodeCount int,
) (ctrl.Result, error) {
    logger := log.FromContext(ctx)
    
    // In sharded mode, pod count = shard count (fixed)
    requiredPods := ksmScale.Spec.Sharding.ShardCount
    
    logger.Info("Reconciling sharded mode",
        "nodeCount", nodeCount,
        "fixedShards", requiredPods,
        "effectiveNodesPerShard", nodeCount/int(requiredPods))
    
    // Warn if shard count seems inappropriate for cluster size
    r.evaluateShardingEfficiency(ctx, ksmScale, nodeCount, requiredPods)
    
    namespace := ksmScale.Spec.Namespace
    if namespace == "" {
        namespace = "monitoring"
    }
    
    // Get current pods
    currentPods, err := r.getShardedPods(ctx, ksmScale, namespace)
    if err != nil {
        return ctrl.Result{}, err
    }
    
    // Ensure all shards are running
    existingShards := make(map[int32]*corev1.Pod)
    for i := range currentPods {
        pod := &currentPods[i]
        if shardID, err := r.getShardID(pod); err == nil {
            existingShards[shardID] = pod
        }
    }
    
    // Create/update all required shards
    var errors []error
    for shardID := int32(0); shardID < requiredPods; shardID++ {
        if existingPod, exists := existingShards[shardID]; exists {
            // Verify configuration is correct
            if !r.isShardConfigValid(existingPod, shardID, requiredPods) {
                logger.Info("Updating shard configuration",
                    "shard", shardID,
                    "pod", existingPod.Name)
                if err := r.Delete(ctx, existingPod); err != nil {
                    errors = append(errors, err)
                    continue
                }
                // Will be recreated in next reconciliation
            }
        } else {
            // Create missing shard
            logger.Info("Creating shard", "shardID", shardID, "totalShards", requiredPods)
            if err := r.createShardPod(ctx, ksmScale, namespace, shardID, requiredPods); err != nil {
                errors = append(errors, err)
            }
        }
    }
    
    // Remove any excess pods (shards beyond required count)
    for shardID, pod := range existingShards {
        if shardID >= requiredPods {
            logger.Info("Removing excess shard", "shardID", shardID)
            if err := r.Delete(ctx, pod); err != nil {
                errors = append(errors, err)
            }
        }
    }
    
    // Create/update PodDisruptionBudget
    if ksmScale.Spec.Sharding.Distribution.EnablePDB {
        if err := r.ensurePDB(ctx, ksmScale, namespace, requiredPods); err != nil {
            logger.Error(err, "Failed to ensure PodDisruptionBudget")
            // Don't fail reconciliation for PDB errors
        }
    }
    
    // Update status
    if err := r.updateShardingStatus(ctx, ksmScale, nodeCount, requiredPods, existingShards); err != nil {
        return ctrl.Result{}, err
    }
    
    // Determine requeue interval
    if len(errors) > 0 {
        // Errors occurred, retry sooner
        return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
    }
    
    if len(existingShards) != int(requiredPods) {
        // Still scaling, check frequently
        return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
    }
    
    // Stable state, check less frequently
    return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *KSMScaleReconciler) evaluateShardingEfficiency(
    ctx context.Context,
    ksmScale *monitorv1alpha1.KSMScale,
    nodeCount int,
    shardCount int32,
) {
    logger := log.FromContext(ctx)
    
    nodesPerShard := float64(nodeCount) / float64(shardCount)
    
    // Define efficiency thresholds
    type shardRecommendation struct {
        minNodes int
        maxNodes int
        shards   int32
    }
    
    recommendations := []shardRecommendation{
        {0, 30, 1},      // Non-sharded for small clusters
        {30, 60, 2},
        {60, 120, 3},
        {120, 200, 5},
        {200, 400, 7},
        {400, 1000, 10},
    }
    
    var recommended int32 = 1
    for _, rec := range recommendations {
        if nodeCount >= rec.minNodes && nodeCount < rec.maxNodes {
            recommended = rec.shards
            break
        }
    }
    
    if shardCount != recommended {
        logger.Info("Sharding efficiency warning",
            "currentNodes", nodeCount,
            "configuredShards", shardCount,
            "recommendedShards", recommended,
            "nodesPerShard", nodesPerShard)
        
        // Update status with recommendation
        if ksmScale.Status.ShardingStatus == nil {
            ksmScale.Status.ShardingStatus = &monitorv1alpha1.ShardingStatus{}
        }
        
        if nodesPerShard < 10 {
            ksmScale.Status.ShardingStatus.Warning = fmt.Sprintf(
                "Over-sharded: %d shards for %d nodes (%.1f nodes/shard). Consider reducing to %d shards.",
                shardCount, nodeCount, nodesPerShard, recommended)
        } else if nodesPerShard > 50 {
            ksmScale.Status.ShardingStatus.Warning = fmt.Sprintf(
                "Under-sharded: %d shards for %d nodes (%.1f nodes/shard). Consider increasing to %d shards.",
                shardCount, nodeCount, nodesPerShard, recommended)
        }
    }
}

func (r *KSMScaleReconciler) ensurePDB(
    ctx context.Context,
    ksmScale *monitorv1alpha1.KSMScale,
    namespace string,
    totalShards int32,
) error {
    maxUnavailable := ksmScale.Spec.Sharding.Distribution.MaxUnavailable
    if maxUnavailable == 0 {
        maxUnavailable = 1
    }
    
    // Ensure at least one shard remains available
    if maxUnavailable >= totalShards {
        maxUnavailable = totalShards - 1
        if maxUnavailable < 1 {
            maxUnavailable = 1
        }
    }
    
    pdb := &policyv1.PodDisruptionBudget{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("ksm-%s-shards", ksmScale.Name),
            Namespace: namespace,
        },
        Spec: policyv1.PodDisruptionBudgetSpec{
            MaxUnavailable: &intstr.IntOrString{
                Type:   intstr.Int,
                IntVal: maxUnavailable,
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
    
    // Create or update
    existing := &policyv1.PodDisruptionBudget{}
    err := r.Get(ctx, client.ObjectKeyFromObject(pdb), existing)
    if err != nil {
        if errors.IsNotFound(err) {
            return r.Create(ctx, pdb)
        }
        return err
    }
    
    existing.Spec = pdb.Spec
    return r.Update(ctx, existing)
}
```

### Example Configurations

#### Small Cluster (20 nodes)
```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: small-cluster-ksm
spec:
  # For small clusters, use dynamic scaling without sharding
  nodesPerMonitor: 30
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
  namespace: monitoring
```

#### Medium Cluster (75 nodes)
```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: medium-cluster-ksm
spec:
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
  namespace: monitoring
  
  sharding:
    enabled: true
    shardCount: 3  # Fixed at 3 shards (25 nodes per shard)
    distribution:
      spreadAcrossNodes: true
      enablePDB: true
      maxUnavailable: 1  # Allow 1 shard down at a time
```

#### Large Cluster (300 nodes)
```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: large-cluster-ksm
spec:
  monitorImage: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
  namespace: monitoring
  
  sharding:
    enabled: true
    shardCount: 7  # Fixed at 7 shards (~43 nodes per shard)
    distribution:
      spreadAcrossZones: true
      spreadAcrossNodes: true
      enablePDB: true
      maxUnavailable: 2  # Allow 2 shards down at a time
```

## Migration Guide

### Switching from Dynamic to Sharded Mode

1. **Evaluate Current State**
   ```bash
   kubectl get ksmscale <name> -o yaml
   # Note current pod count and node count
   ```

2. **Choose Appropriate Shard Count**
   - 30-60 nodes: 2 shards
   - 60-120 nodes: 3 shards
   - 120-200 nodes: 5 shards

3. **Update Configuration**
   ```yaml
   spec:
     sharding:
       enabled: true
       shardCount: 3  # Based on cluster size
   ```

4. **Monitor Transition**
   ```bash
   # Watch pods being created/updated
   kubectl get pods -l ksmscale.example.com=<name> -w
   
   # Check shard configuration
   kubectl get pods -l ksmscale.example.com=<name> -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].args}{"\n"}{end}'
   ```

5. **Verify Complete Coverage**
   ```promql
   # All shards reporting metrics
   count(up{job="kube-state-metrics"} == 1) == <shard_count>
   ```

## Conclusion

The recommended approach is to use **fixed pod count equal to shard count** when sharding is enabled. This ensures:

1. **Complete metric coverage** - All shards always running
2. **Predictable behavior** - No complex reconfiguration
3. **Clear mental model** - Sharding = fixed deployment size
4. **Operational safety** - No risk of missing metrics

Users must make an explicit choice:
- **Dynamic scaling** (traditional mode) for smaller, variable clusters
- **Fixed sharding** for larger, stable clusters requiring predictable performance

The implementation includes warnings when shard count seems inappropriate for cluster size, helping operators make informed decisions.