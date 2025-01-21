package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// LoadIndex is a representation of a shared with calculated load index to it.
type LoadIndex struct {
	// Shard is the shard that this load index is calculated for.
	// +kubebuilder:validation:Required
	Shard DiscoveredShard `json:"shard,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Value resource.Quantity `json:"value,omitempty"`
	// DisplayValue is the string representation of the without precision guarantee.
	// This is meaningless and exists purely for convenience of someone who is looking at the kubectl get output.
	// +kubebuilder:validation:Required
	DisplayValue string `json:"displayValue,omitempty"`
}
