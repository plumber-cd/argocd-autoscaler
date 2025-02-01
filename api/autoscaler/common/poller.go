package common

import (
	corev1 "k8s.io/api/core/v1"
)

type PollerSpec struct {
	// ShardManagerRef is a reference to the shard manager.
	// +kubebuilder:validation:Required
	ShardManagerRef *corev1.TypedLocalObjectReference `json:"shardManagerRef,omitempty"`
}

// +k8s:deepcopy-gen=false
type Poller interface {
	GetPollerSpec() *PollerSpec
}
