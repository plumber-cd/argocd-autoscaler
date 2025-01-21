package autoscaler

import (
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Poller is an interface that should be implemented by all pollers.
type Poller interface {
	// GetConditions returns a list of shards discovered by this discoverer.
	GetConditions() []metav1.Condition
	// GetValues returns a list of metric values discovered by this poller.
	GetValues() []autoscaler.MetricValue
}
