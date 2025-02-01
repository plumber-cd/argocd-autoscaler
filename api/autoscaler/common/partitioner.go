package common

import corev1 "k8s.io/api/core/v1"

type PartitionerSpec struct {
	// LoadIndexProviderRef is the reference to the load index provider.
	// +kubebuilder:validation:Required
	LoadIndexProviderRef *corev1.TypedLocalObjectReference `json:"loadIndexProviderRef,omitempty"`
}

// +k8s:deepcopy-gen=false
type Partitioner interface {
	GetSpec() *PartitionerSpec
}
