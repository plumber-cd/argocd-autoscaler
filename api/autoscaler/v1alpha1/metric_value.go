package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// MetricValue is a resulting value for the metric.
type MetricValue struct {
	// OwnerRef is the reference to the owner of this metric.
	OwnerRef corev1.TypedLocalObjectReference `json:"ownerRef,omitempty"`
	// Poller is the identification of the poller that was used to fetch this metric.
	Poller string `json:"poller,omitempty"`
	// ID of this metric.
	ID string `json:"id,omitempty"`
	// Query is the query that was used to fetch this value.
	// It will be different for individual implementations.
	Query string `json:"query,omitempty"`
	// Value is the value of the metric.
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Value resource.Quantity `json:"value,omitempty"`
}
