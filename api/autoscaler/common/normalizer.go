package common

import corev1 "k8s.io/api/core/v1"

type NormalizerSpec struct {
	// MetricValuesProviderRef is a reference to the metric values provider.
	// +kubebuilder:validation:Required
	MetricValuesProviderRef *corev1.TypedLocalObjectReference `json:"metricValuesProviderRef,omitempty"`
}

// +k8s:deepcopy-gen=false
type Normalizer interface {
	GetSpec() NormalizerSpec
}
