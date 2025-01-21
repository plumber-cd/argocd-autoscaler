package v1alpha1

import (
	"k8s.io/apimachinery/pkg/types"
)

// Shard is a shard of something discovered by a discoverer.
// It is suitable to be used by Pollers.
// Attributes may be used in Go Templates for the poller if it supports that.
type Shard struct {
	// ID of this shard. It may or may not be unique, depending on the discoverer.
	// It is expected to be used to populate Go Templates params for the poller (if the poller supports that).
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`
	// UID unieue identifier of this shard so it can be uniquely identified in the list of shards.
	// The reason this is not baked into SourceRef is because actual resource may or may not exists.
	// When the discovery was external - this may be arbitrary string unique to that shard.
	// +kubebuilder:validation:Required
	UID types.UID `json:"uid,omitempty"`
	// Data is a map that will be used to populate Go Templates params for the poller (if the poller supports that).
	// +kubebuilder:validation:Required
	Data map[string]string `json:"data,omitempty"`
}
