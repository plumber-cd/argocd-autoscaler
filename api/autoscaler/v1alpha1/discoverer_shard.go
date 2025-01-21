package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// DiscovererShard is a shard of something discovered by a discoverer.
// It is suitable to be used by Pollers.
// Attributes may be used in Go Templates for the poller if it supports that.
type DiscovererShard struct {
	// SourceRef is a reference to the source object that was used to discover this shard.
	// It may be nil if discoverer was not using Kubernetes API to discover the shard.
	SourceRef *corev1.TypedLocalObjectReference `json:"sourceRef,omitempty"`
	// Data is a map that will be used to populate Go Templates params for the poller (if the poller supports that).
	// +kubebuilder:validation:Required
	Data map[string]string `json:"data,omitempty"`
}
