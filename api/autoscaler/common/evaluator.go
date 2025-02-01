package common

import corev1 "k8s.io/api/core/v1"

type EvaluatorSpec struct {
	// PartitionProviderRef is the reference to the partition provider.
	// +kubebuilder:validation:Required
	PartitionProviderRef *corev1.TypedLocalObjectReference `json:"partitionProviderRef,omitempty"`
}

// +k8s:deepcopy-gen=false
type Evaluator interface {
	GetSpec() EvaluatorSpec
}
