package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ShardsProviderStatus struct {
	// Conditions
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Shards shards that are discovered.
	Shards []Shard `json:"shards,omitempty"`
}

// +k8s:deepcopy-gen=false
type ShardsProvider interface {
	GetStatus() ShardsProviderStatus
}
