package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

// LoadIndex is a representation of a shared with calculated load index to it.
type LoadIndex struct {
	// UID unieue identifier of this shard so it can be uniquely identified in the list of shards.
	// +kubebuilder:validation:Required
	UID types.UID `json:"uid,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Value resource.Quantity `json:"value,omitempty"`
}
