package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// MetricValue is a resulting value for the metric.
type MetricValue struct {
	// Poller is the identification of the poller that was used to fetch this metric.
	// +kubebuilder:validation:Required
	Poller string `json:"poller,omitempty"`
	// Shard is this shard for this metric value.
	// +kubebuilder:validation:Required
	Shard DiscoveredShard `json:"shard,omitempty"`
	// ID of this metric.
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`
	// Query is the query that was used to fetch this value.
	// It will be different for individual implementations.
	Query string `json:"query,omitempty"`
	// Value is the value of the metric.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Value resource.Quantity `json:"value,omitempty"`
	// DisplayValue is the string representation of the without precision guarantee.
	// This is meaningless and exists purely for convenience of someone who is looking at the kubectl get output.
	// +kubebuilder:validation:Required
	DisplayValue string `json:"displayValue,omitempty"`
}
