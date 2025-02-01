package common

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// LoadIndex is a representation of a shard with calculated load index to it.
type LoadIndex struct {
	// Shard is the shard that this load index is calculated for.
	// +kubebuilder:validation:Required
	Shard Shard `json:"shard,omitempty"`
	// Value is a value of this load index.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Value resource.Quantity `json:"value,omitempty"`
	// DisplayValue is the string representation of the without precision guarantee.
	// This is meaningless and exists purely for convenience of someone who is looking at the kubectl get output.
	// +kubebuilder:validation:Required
	DisplayValue string `json:"displayValue,omitempty"`
}

// LoadIndexesDesc implements sort.Interface for []LoadIndex based on the Value field in descending order.
type LoadIndexesDesc []LoadIndex

func (s LoadIndexesDesc) Len() int {
	return len(s)
}

func (s LoadIndexesDesc) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s LoadIndexesDesc) Less(i, j int) bool {
	return s[i].Value.AsApproximateFloat64() > s[j].Value.AsApproximateFloat64()
}
