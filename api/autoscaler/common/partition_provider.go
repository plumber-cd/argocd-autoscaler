package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PartitionProviderStatus struct {
	// Conditions
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Replicas is the list of replicas partitioned.
	Replicas ReplicaList `json:"replicas,omitempty"`
}

// +k8s:deepcopy-gen=false
type PartitionProvider interface {
	GetPartitionProviderStatus() *PartitionProviderStatus
}
