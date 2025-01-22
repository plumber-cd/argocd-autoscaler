package autoscaler

import (
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PartitionProvider is an interface that should be implemented by all partition providers.
type PartitionProvider interface {
	// GetConditions returns a list of conditions of this provider.
	GetConditions() []metav1.Condition
	// GetReplicas returns a list of replicas with shard assignments produced by this provider.
	GetReplicas() []autoscaler.Replica
}
