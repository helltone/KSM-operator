package utils

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitorv1alpha1 "github.com/example/ksm-scale-operator/api/v1alpha1"
)

// CreateTestNodes creates a specified number of test nodes
func CreateTestNodes(count int) []*corev1.Node {
	nodes := make([]*corev1.Node, count)
	for i := 0; i < count; i++ {
		nodes[i] = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("test-node-%d", i+1),
				Labels: map[string]string{
					"node-role.kubernetes.io/worker": "",
				},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
	}
	return nodes
}

// CreateTestKSMScale creates a test KSMScale resource
func CreateTestKSMScale(name string, nodesPerMonitor int32, namespace string) *monitorv1alpha1.KSMScale {
	if namespace == "" {
		namespace = "monitoring"
	}
	if nodesPerMonitor == 0 {
		nodesPerMonitor = 15
	}

	return &monitorv1alpha1.KSMScale{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: monitorv1alpha1.KSMScaleSpec{
			NodesPerMonitor: nodesPerMonitor,
			MonitorImage:    "test-image:latest",
			Namespace:       namespace,
			Resources: monitorv1alpha1.ResourceRequirements{
				Requests: monitorv1alpha1.ResourceList{
					CPU:    "10m",
					Memory: "32Mi",
				},
				Limits: monitorv1alpha1.ResourceList{
					CPU:    "50m",
					Memory: "64Mi",
				},
			},
		},
	}
}

// WaitForKSMScaleReady waits for a KSMScale to reach Ready phase
func WaitForKSMScaleReady(ctx context.Context, c client.Client, name string, timeout time.Duration) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	timeoutChan := time.After(timeout)
	
	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("timeout waiting for KSMScale %s to become ready", name)
		case <-ticker.C:
			var ksmScale monitorv1alpha1.KSMScale
			err := c.Get(ctx, types.NamespacedName{Name: name}, &ksmScale)
			if err != nil {
				continue
			}
			if ksmScale.Status.Phase == "Ready" {
				return nil
			}
		}
	}
}

// CreateTestKSMScaleWithSharding creates a KSMScale with sharding configuration
func CreateTestKSMScaleWithSharding(name string, shardCount int32, nodesPerMonitor int32) *monitorv1alpha1.KSMScale {
	ksmScale := CreateTestKSMScale(name, nodesPerMonitor, "monitoring")
	ksmScale.Spec.Sharding = &monitorv1alpha1.ShardingConfig{
		Enabled:             true,
		ShardCount:          shardCount,
		MinReplicasPerShard: 1,
		MaxReplicasPerShard: 4,
		Distribution: &monitorv1alpha1.DistributionConfig{
			SpreadAcrossNodes: true,
			EnablePDB:         true,
			MaxUnavailable:    1,
		},
	}
	return ksmScale
}

// GetKSMScalePods retrieves all pods for a KSMScale resource
func GetKSMScalePods(ctx context.Context, c client.Client, ksmScale *monitorv1alpha1.KSMScale) ([]corev1.Pod, error) {
	var podList corev1.PodList
	
	labelSelector := client.MatchingLabels{
		"app": "kube-state-metrics",
		"managed-by": "ksm-scale-operator",
		"instance": ksmScale.Name,
	}
	
	listOpts := []client.ListOption{
		client.InNamespace(ksmScale.Spec.Namespace),
		labelSelector,
	}
	
	if err := c.List(ctx, &podList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	
	return podList.Items, nil
}

// ValidateShardDistribution checks if pods are properly distributed across shards
func ValidateShardDistribution(pods []corev1.Pod, expectedShardCount int32) error {
	shardCounts := make(map[string]int)
	
	for _, pod := range pods {
		if shardID, ok := pod.Labels["shard-id"]; ok {
			shardCounts[shardID]++
		} else {
			return fmt.Errorf("pod %s missing shard-id label", pod.Name)
		}
	}
	
	// Check that we have the expected number of shards
	if len(shardCounts) > int(expectedShardCount) {
		return fmt.Errorf("found %d shards, expected max %d", len(shardCounts), expectedShardCount)
	}
	
	// Each active shard should have at least one pod
	for shardID, count := range shardCounts {
		if count == 0 {
			return fmt.Errorf("shard %s has no pods", shardID)
		}
	}
	
	return nil
}

// CountPodsPerShard returns a map of shard ID to pod count
func CountPodsPerShard(pods []corev1.Pod) map[string]int {
	shardCounts := make(map[string]int)
	
	for _, pod := range pods {
		if shardID, ok := pod.Labels["shard-id"]; ok {
			shardCounts[shardID]++
		}
	}
	
	return shardCounts
}

// GetReadyPods filters pods that are in Ready state
func GetReadyPods(pods []corev1.Pod) []corev1.Pod {
	var readyPods []corev1.Pod
	
	for _, pod := range pods {
		if isPodReady(&pod) {
			readyPods = append(readyPods, pod)
		}
	}
	
	return readyPods
}

// isPodReady checks if a pod is ready
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	
	return false
}