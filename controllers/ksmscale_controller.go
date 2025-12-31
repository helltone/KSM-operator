package controllers

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	monitorv1alpha1 "github.com/example/ksm-scale-operator/api/v1alpha1"
)

// podInfo holds information about a pod and its shard assignment
type podInfo struct {
	pod     *corev1.Pod
	index   int32
	shardID int32
}

type KSMScaleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monitor.example.com,resources=ksmscales,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitor.example.com,resources=ksmscales/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitor.example.com,resources=ksmscales/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

func (r *KSMScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	ksmScale := &monitorv1alpha1.KSMScale{}
	err := r.Get(ctx, req.NamespacedName, ksmScale)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		logger.Error(err, "Failed to list nodes")
		return ctrl.Result{}, err
	}
	nodeCount := len(nodeList.Items)

	// Determine if sharding is enabled
	if ksmScale.Spec.Sharding != nil && ksmScale.Spec.Sharding.Enabled {
		return r.reconcileShardedMode(ctx, ksmScale, nodeCount)
	}
	
	// Fall back to existing non-sharded logic
	return r.reconcileNonShardedMode(ctx, ksmScale, nodeCount)
}

func (r *KSMScaleReconciler) getCurrentMonitorPods(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, namespace string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/name":   "kube-state-metrics",
			"ksmscale.example.com": ksmScale.Name,
		},
	}

	if err := r.List(ctx, podList, listOpts...); err != nil {
		return nil, err
	}

	var activePods []corev1.Pod
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp == nil {
			activePods = append(activePods, pod)
		}
	}

	return activePods, nil
}

func (r *KSMScaleReconciler) createMonitorPod(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, namespace string, index int) error {
	// Set default image if not specified
	image := ksmScale.Spec.MonitorImage
	if image == "" {
		image = "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kube-state-metrics-%s-%d", ksmScale.Name, index),
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":     "kube-state-metrics",
				"app.kubernetes.io/instance": ksmScale.Name,
				"app.kubernetes.io/component": "exporter",
				"ksmscale.example.com":   ksmScale.Name,
				"index":                      fmt.Sprintf("%d", index),
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "kube-state-metrics",
			Containers: []corev1.Container{
				{
					Name:  "kube-state-metrics",
					Image: image,
					Args: []string{
						"--host=0.0.0.0",
						"--port=8080",
						"--telemetry-host=0.0.0.0",
						"--telemetry-port=8081",
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

	if ksmScale.Spec.Resources.Requests.CPU != "" || ksmScale.Spec.Resources.Requests.Memory != "" {
		pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{}
		if ksmScale.Spec.Resources.Requests.CPU != "" {
			pod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse(ksmScale.Spec.Resources.Requests.CPU)
		}
		if ksmScale.Spec.Resources.Requests.Memory != "" {
			pod.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse(ksmScale.Spec.Resources.Requests.Memory)
		}
	}

	if ksmScale.Spec.Resources.Limits.CPU != "" || ksmScale.Spec.Resources.Limits.Memory != "" {
		pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{}
		if ksmScale.Spec.Resources.Limits.CPU != "" {
			pod.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse(ksmScale.Spec.Resources.Limits.CPU)
		}
		if ksmScale.Spec.Resources.Limits.Memory != "" {
			pod.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory] = resource.MustParse(ksmScale.Spec.Resources.Limits.Memory)
		}
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

// reconcileNonShardedMode handles the original non-sharded scaling logic
func (r *KSMScaleReconciler) reconcileNonShardedMode(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, nodeCount int) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	nodesPerMonitor := int(ksmScale.Spec.NodesPerMonitor)
	if nodesPerMonitor == 0 {
		nodesPerMonitor = 15
	}

	desiredReplicas := int(math.Ceil(float64(nodeCount) / float64(nodesPerMonitor)))
	if desiredReplicas == 0 && nodeCount > 0 {
		desiredReplicas = 1
	}

	namespace := ksmScale.Spec.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}

	currentPods, err := r.getCurrentMonitorPods(ctx, ksmScale, namespace)
	if err != nil {
		logger.Error(err, "Failed to get current monitor pods")
		return ctrl.Result{}, err
	}

	currentReplicas := len(currentPods)

	logger.Info("Reconciling KSMScale (non-sharded)",
		"nodeCount", nodeCount,
		"desiredReplicas", desiredReplicas,
		"currentReplicas", currentReplicas)

	if currentReplicas < desiredReplicas {
		// Find which pod indexes are missing and create them
		existingIndexes := make(map[int]bool)
		for _, pod := range currentPods {
			if indexStr, ok := pod.Labels["index"]; ok {
				if index := parseInt(indexStr); index >= 0 {
					existingIndexes[index] = true
				}
			}
		}
		
		// Create missing pods starting from index 0
		podsCreated := 0
		for i := 0; i < desiredReplicas && podsCreated < (desiredReplicas-currentReplicas); i++ {
			if !existingIndexes[i] {
				if err := r.createMonitorPod(ctx, ksmScale, namespace, i); err != nil {
					logger.Error(err, "Failed to create monitor pod", "index", i)
					return ctrl.Result{}, err
				}
				logger.Info("Created monitor pod", "index", i)
				podsCreated++
			}
		}
	} else if currentReplicas > desiredReplicas {
		podsToDelete := currentPods[desiredReplicas:]
		for _, pod := range podsToDelete {
			if err := r.Delete(ctx, &pod); err != nil {
				logger.Error(err, "Failed to delete monitor pod", "pod", pod.Name)
				return ctrl.Result{}, err
			}
			logger.Info("Deleted monitor pod", "pod", pod.Name)
		}
	}

	// Update status
	ksmScale.Status.NodeCount = int32(nodeCount)
	ksmScale.Status.DesiredReplicas = int32(desiredReplicas)
	ksmScale.Status.CurrentReplicas = int32(currentReplicas)
	ksmScale.Status.LastUpdated = metav1.NewTime(time.Now())
	ksmScale.Status.ShardingStatus = nil // Clear sharding status for non-sharded mode
	
	if currentReplicas == desiredReplicas {
		ksmScale.Status.Phase = "Ready"
	} else {
		ksmScale.Status.Phase = "Scaling"
	}

	if err := r.Status().Update(ctx, ksmScale); err != nil {
		logger.Error(err, "Failed to update KSMScale status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// reconcileShardedMode handles sharding with dynamic replicas
func (r *KSMScaleReconciler) reconcileShardedMode(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, nodeCount int) (ctrl.Result, error) {
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
	
	minReplicasPerShard := ksmScale.Spec.Sharding.MinReplicasPerShard
	if minReplicasPerShard == 0 {
		minReplicasPerShard = 1
	}
	
	// Calculate desired pods (with minimum enforcement)
	calculatedPods := int32(math.Ceil(float64(nodeCount) / float64(nodesPerMonitor)))
	minPods := shardCount * minReplicasPerShard
	
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
	currentPods, err := r.getShardedPods(ctx, ksmScale, namespace)
	if err != nil {
		logger.Error(err, "Failed to get current sharded pods")
		return ctrl.Result{}, err
	}
	
	logger.Info("Reconciling KSMScale (sharded)",
		"nodeCount", nodeCount,
		"shardCount", shardCount,
		"desiredPods", desiredPods,
		"currentPods", len(currentPods))
	
	// Build current state map: podIndex -> (pod, shardID)
	
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
	var reconcileErrors []error
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
					reconcileErrors = append(reconcileErrors, err)
					continue
				}
				// Will be recreated in next reconciliation
			} else if !r.isShardConfigValid(existing.pod, assignedShard, shardCount) {
				logger.Info("Pod configuration outdated, recreating",
					"pod", existing.pod.Name)
					
				if err := r.Delete(ctx, existing.pod); err != nil {
					reconcileErrors = append(reconcileErrors, err)
					continue
				}
			}
		} else {
			// Create missing pod
			logger.Info("Creating sharded pod",
				"index", podIndex,
				"shard", assignedShard,
				"totalShards", shardCount)
				
			if err := r.createShardedPod(ctx, ksmScale, namespace, 
				podIndex, assignedShard, shardCount); err != nil {
				reconcileErrors = append(reconcileErrors, err)
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
				reconcileErrors = append(reconcileErrors, err)
			}
		}
	}
	
	// Create/update PodDisruptionBudget if enabled
	if ksmScale.Spec.Sharding.Distribution != nil && 
	   ksmScale.Spec.Sharding.Distribution.EnablePDB {
		if err := r.ensurePDB(ctx, ksmScale, namespace, shardCount); err != nil {
			logger.Error(err, "Failed to ensure PodDisruptionBudget")
			// Don't fail reconciliation for PDB errors
		}
	}
	
	// Update status with shard distribution
	if err := r.updateShardingStatus(ctx, ksmScale, nodeCount, desiredPods, shardCount, existingPods); err != nil {
		return ctrl.Result{}, err
	}
	
	// Determine requeue interval
	if len(reconcileErrors) > 0 || len(existingPods) != int(desiredPods) {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	
	return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}

// calculateShardDistribution distributes pods across shards using round-robin
func (r *KSMScaleReconciler) calculateShardDistribution(totalPods int32, shardCount int32) map[int32]int32 {
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

// getShardedPods returns pods that belong to sharded deployment
func (r *KSMScaleReconciler) getShardedPods(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, namespace string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/name": "kube-state-metrics",
			"ksmscale.example.com":   ksmScale.Name,
			"sharding-enabled":       "true",
		},
	}

	if err := r.List(ctx, podList, listOpts...); err != nil {
		return nil, err
	}

	var activePods []corev1.Pod
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp == nil {
			activePods = append(activePods, pod)
		}
	}

	return activePods, nil
}

// createShardedPod creates a new pod with sharding configuration
func (r *KSMScaleReconciler) createShardedPod(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, namespace string, podIndex int32, shardID int32, totalShards int32) error {
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
	
	// Apply resource requirements
	if ksmScale.Spec.Resources.Requests.CPU != "" || ksmScale.Spec.Resources.Requests.Memory != "" {
		pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{}
		if ksmScale.Spec.Resources.Requests.CPU != "" {
			pod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse(ksmScale.Spec.Resources.Requests.CPU)
		}
		if ksmScale.Spec.Resources.Requests.Memory != "" {
			pod.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse(ksmScale.Spec.Resources.Requests.Memory)
		}
	}
	
	if ksmScale.Spec.Resources.Limits.CPU != "" || ksmScale.Spec.Resources.Limits.Memory != "" {
		pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{}
		if ksmScale.Spec.Resources.Limits.CPU != "" {
			pod.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse(ksmScale.Spec.Resources.Limits.CPU)
		}
		if ksmScale.Spec.Resources.Limits.Memory != "" {
			pod.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory] = resource.MustParse(ksmScale.Spec.Resources.Limits.Memory)
		}
	}
	
	// Add anti-affinity to spread replicas of same shard if configured
	if ksmScale.Spec.Sharding.Distribution != nil && ksmScale.Spec.Sharding.Distribution.SpreadAcrossNodes {
		pod.Spec.Affinity = &corev1.Affinity{
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
		}
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

// isShardConfigValid checks if a pod has the correct shard configuration
func (r *KSMScaleReconciler) isShardConfigValid(pod *corev1.Pod, expectedShardID int32, expectedTotalShards int32) bool {
	// Check labels
	if shardLabel, ok := pod.Labels["shard-id"]; !ok || shardLabel != fmt.Sprintf("%d", expectedShardID) {
		return false
	}
	if totalShardsLabel, ok := pod.Labels["total-shards"]; !ok || totalShardsLabel != fmt.Sprintf("%d", expectedTotalShards) {
		return false
	}
	
	// Check container args
	if len(pod.Spec.Containers) == 0 {
		return false
	}
	
	container := pod.Spec.Containers[0]
	expectedShardArg := fmt.Sprintf("--shard=%d", expectedShardID)
	expectedTotalShardsArg := fmt.Sprintf("--total-shards=%d", expectedTotalShards)
	
	hasShardArg := false
	hasTotalShardsArg := false
	
	for _, arg := range container.Args {
		if arg == expectedShardArg {
			hasShardArg = true
		}
		if arg == expectedTotalShardsArg {
			hasTotalShardsArg = true
		}
	}
	
	return hasShardArg && hasTotalShardsArg
}

// ensurePDB creates or updates the PodDisruptionBudget for sharded pods
func (r *KSMScaleReconciler) ensurePDB(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, namespace string, shardCount int32) error {
	maxUnavailable := int32(1)
	if ksmScale.Spec.Sharding.Distribution != nil && ksmScale.Spec.Sharding.Distribution.MaxUnavailable > 0 {
		maxUnavailable = ksmScale.Spec.Sharding.Distribution.MaxUnavailable
	}
	
	// Ensure at least one shard remains available
	if maxUnavailable >= shardCount {
		maxUnavailable = shardCount - 1
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
	
	// Create or update PDB
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

// updateShardingStatus updates the status with detailed sharding information
func (r *KSMScaleReconciler) updateShardingStatus(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, nodeCount int, desiredPods int32, shardCount int32, existingPods map[int32]*podInfo) error {
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
				if r.isPodReady(info.pod) {
					replicas.Ready++
				}
			}
		}
		
		distribution[shardKey] = replicas
	}
	
	// Count active shards
	activeShards := int32(0)
	for shardID := int32(0); shardID < shardCount; shardID++ {
		shardKey := fmt.Sprintf("%d", shardID)
		if dist, ok := distribution[shardKey]; ok && dist.Ready > 0 {
			activeShards++
		}
	}
	
	// Evaluate sharding efficiency and generate warnings
	warning := r.evaluateShardingEfficiency(nodeCount, int(shardCount))
	
	// Update status
	ksmScale.Status.NodeCount = int32(nodeCount)
	ksmScale.Status.DesiredReplicas = desiredPods
	ksmScale.Status.CurrentReplicas = int32(len(existingPods))
	ksmScale.Status.LastUpdated = metav1.NewTime(time.Now())
	
	ksmScale.Status.ShardingStatus = &monitorv1alpha1.ShardingStatus{
		TotalShards:       shardCount,
		ActiveShards:      activeShards,
		ShardDistribution: distribution,
		Warning:           warning,
	}
	
	// Determine phase
	if activeShards < shardCount {
		ksmScale.Status.Phase = "Incomplete"
	} else if int32(len(existingPods)) != desiredPods {
		ksmScale.Status.Phase = "Scaling"
	} else {
		ksmScale.Status.Phase = "Ready"
	}
	
	return r.Status().Update(ctx, ksmScale)
}

// evaluateShardingEfficiency provides recommendations for shard configuration
func (r *KSMScaleReconciler) evaluateShardingEfficiency(nodeCount int, shardCount int) string {
	nodesPerShard := float64(nodeCount) / float64(shardCount)
	
	// Define efficiency thresholds
	type shardRecommendation struct {
		minNodes int
		maxNodes int
		shards   int
	}
	
	recommendations := []shardRecommendation{
		{0, 30, 1},      // Non-sharded for small clusters
		{30, 60, 2},
		{60, 120, 3},
		{120, 200, 5},
		{200, 400, 7},
		{400, 1000, 10},
	}
	
	var recommended int = 1
	for _, rec := range recommendations {
		if nodeCount >= rec.minNodes && nodeCount < rec.maxNodes {
			recommended = rec.shards
			break
		}
	}
	
	if shardCount != recommended {
		if nodesPerShard < 10 {
			return fmt.Sprintf(
				"Over-sharded: %d shards for %d nodes (%.1f nodes/shard). Consider reducing to %d shards.",
				shardCount, nodeCount, nodesPerShard, recommended)
		} else if nodesPerShard > 50 {
			return fmt.Sprintf(
				"Under-sharded: %d shards for %d nodes (%.1f nodes/shard). Consider increasing to %d shards.",
				shardCount, nodeCount, nodesPerShard, recommended)
		}
	}
	
	return ""
}

// isPodReady checks if a pod is ready
func (r *KSMScaleReconciler) isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *KSMScaleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1alpha1.KSMScale{}).
		Owns(&corev1.Pod{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Complete(r)
}

func parseInt(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return -1
}