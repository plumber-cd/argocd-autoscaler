package autoscaler

import (
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

// Discoverer is an interface that should be implemented by all discoverers.
type Discoverer interface {
	// GetShards returns a list of shards discovered by this discoverer.
	GetShards() []autoscaler.DiscoveredShard
}
