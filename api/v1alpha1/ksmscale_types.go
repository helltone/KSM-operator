package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true

type KSMScaleSpec struct {
	// Number of nodes per kube-state-metrics pod
	// +kubebuilder:validation:Minimum=9
	// +kubebuilder:default=15
	NodesPerMonitor int32 `json:"nodesPerMonitor,omitempty"`
	
	// Container image for kube-state-metrics
	// +kubebuilder:default="registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
	MonitorImage string `json:"monitorImage,omitempty"`
	
	// Target namespace for kube-state-metrics pods
	// +kubebuilder:default="monitoring"
	Namespace string `json:"namespace,omitempty"`
	
	// Resource requirements for kube-state-metrics pods
	Resources ResourceRequirements `json:"resources,omitempty"`
	
	// Sharding configuration for distributing metrics collection
	// +optional
	Sharding *ShardingConfig `json:"sharding,omitempty"`
}

// ShardingConfig defines sharding configuration for KSM pods
type ShardingConfig struct {
	// Enable sharding mode
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`
	
	// Fixed number of shards (data partitions)
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=3
	ShardCount int32 `json:"shardCount"`
	
	// Minimum replicas per shard (defaults to 1)
	// Total minimum pods = ShardCount * MinReplicasPerShard
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	MinReplicasPerShard int32 `json:"minReplicasPerShard,omitempty"`
	
	// Maximum replicas per shard (0 = unlimited)
	// +kubebuilder:default=0
	MaxReplicasPerShard int32 `json:"maxReplicasPerShard,omitempty"`
	
	// Distribution configuration for pod placement
	// +optional
	Distribution *DistributionConfig `json:"distribution,omitempty"`
}

// DistributionConfig defines how sharded pods are distributed
type DistributionConfig struct {
	// Spread shards across nodes
	// +kubebuilder:default=true
	SpreadAcrossNodes bool `json:"spreadAcrossNodes,omitempty"`
	
	// Enable pod disruption budget
	// +kubebuilder:default=true
	EnablePDB bool `json:"enablePDB,omitempty"`
	
	// Maximum unavailable shards during disruption
	// +kubebuilder:default=1
	MaxUnavailable int32 `json:"maxUnavailable,omitempty"`
}

type ResourceRequirements struct {
	Requests ResourceList `json:"requests,omitempty"`
	Limits   ResourceList `json:"limits,omitempty"`
}

type ResourceList struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type KSMScaleStatus struct {
	CurrentReplicas int32  `json:"currentReplicas,omitempty"`
	DesiredReplicas int32  `json:"desiredReplicas,omitempty"`
	NodeCount       int32  `json:"nodeCount,omitempty"`
	Phase           string `json:"phase,omitempty"`
	LastUpdated     metav1.Time `json:"lastUpdated,omitempty"`
	
	// Sharding status when sharding is enabled
	// +optional
	ShardingStatus *ShardingStatus `json:"shardingStatus,omitempty"`
}

// ShardingStatus provides detailed status about shard distribution
type ShardingStatus struct {
	// Total number of configured shards
	TotalShards int32 `json:"totalShards"`
	
	// Number of active shards (shards with at least one ready pod)
	ActiveShards int32 `json:"activeShards"`
	
	// Distribution of pods across shards
	// Map key is shard ID (0, 1, 2, etc.)
	// +optional
	ShardDistribution map[string]ShardReplicas `json:"shardDistribution,omitempty"`
	
	// Warning message if sharding configuration seems suboptimal
	// +optional
	Warning string `json:"warning,omitempty"`
}

// ShardReplicas describes the replicas for a specific shard
type ShardReplicas struct {
	// Number of total replicas for this shard
	Replicas int32 `json:"replicas"`
	
	// Number of ready replicas for this shard
	Ready int32 `json:"ready"`
	
	// Names of pods serving this shard
	// +optional
	Pods []string `json:"pods,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.nodeCount`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredReplicas`
// +kubebuilder:printcolumn:name="Current",type=integer,JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type KSMScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KSMScaleSpec   `json:"spec,omitempty"`
	Status KSMScaleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type KSMScaleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KSMScale `json:"items"`
}