package common

import corev1 "k8s.io/api/core/v1"

type ScalerSpec struct {
	// PartitionProviderRef is the reference to the partition provider.
	// This is the source of partition layout to apply
	// +kubebuilder:validation:Required
	PartitionProviderRef *corev1.TypedLocalObjectReference `json:"partitionProviderRef,omitempty"`
	// ShardManagerRef is the reference to the shard manager.
	// This is a target to update the partition layout on.
	// +kubebuilder:validation:Required
	ShardManagerRef *corev1.TypedLocalObjectReference `json:"shardManagerRef,omitempty"`
	// ReplicaSetControllerRef is the reference to the replica set.
	// This is the controller of Application Controller RS to apply scaling.
	// +kubebuilder:validation:Required
	ReplicaSetControllerRef *corev1.TypedLocalObjectReference `json:"replicaSetControllerRef,omitempty"`
}

// +k8s:deepcopy-gen=false
type Scaler interface {
	GetScalerSpec() *ScalerSpec
}
