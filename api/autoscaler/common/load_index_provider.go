package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LoadIndexProviderStatus struct {
	// Conditions
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Values is the list of load index values.
	Values []LoadIndex `json:"values,omitempty"`
}

// +k8s:deepcopy-gen=false
type LoadIndexProvider interface {
	GetStatus() *LoadIndexProviderStatus
}
