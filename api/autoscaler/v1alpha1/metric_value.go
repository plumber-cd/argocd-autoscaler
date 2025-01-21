package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

// MetricValue is a resulting value for the metric.
type MetricValue struct {
	// Poller is the identification of the poller that was used to fetch this metric.
	// +kubebuilder:validation:Required
	Poller string `json:"poller,omitempty"`
	// ShardUID uuid of this shard for this metric value.
	// It is used to uniquely identify the shard that was used to fetch this value.
	// When the discovery was external - this may be arbitrary string unique to that shard.
	// +kubebuilder:validation:Required
	ShardUID types.UID `json:"shardUID,omitempty"`
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
}
