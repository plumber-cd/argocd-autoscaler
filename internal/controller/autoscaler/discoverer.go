package autoscaler

import (
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Discoverer is an interface that should be implemented by all discoverers.
type Discoverer interface {
	// GetConditions returns a list of conditions for this dicsoverer.
	GetConditions() []metav1.Condition
	// GetShards returns a list of shards discovered by this discoverer.
	GetShards() []autoscaler.Shard
}
