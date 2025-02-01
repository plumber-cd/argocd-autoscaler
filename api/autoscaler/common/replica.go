package common

import "k8s.io/apimachinery/pkg/api/resource"

// Replica is a representation of the replica for sharding
type Replica struct {
	// ID of the replica, starting from 0 and onward.
	ID string `json:"id,omitempty"`
	// LoadIndexes shards assigned to this replica wrapped into their load index.
	// +kubebuilder:validation:Required
	LoadIndexes []LoadIndex `json:"loadIndexes,omitempty"`
	// TotalLoad is the sum of all load indexes assigned to this replica.
	// +kubebuilder:validation:Required
	TotalLoad resource.Quantity `json:"totalLoad,omitempty"`
	// DisplayValue is the string representation of the total load without precision guarantee.
	// This is meaningless and exists purely for convenience of someone who is looking at the kubectl get output.
	// +kubebuilder:validation:Required
	TotalLoadDisplayValue string `json:"totalLoadDisplayValue,omitempty"`
}
