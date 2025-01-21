package autoscaler

import (
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetricProvider is an interface that should be implemented by all metric providers.
type MetricProvider interface {
	// GetConditions returns a list of conditions of this provider.
	GetConditions() []metav1.Condition
	// GetValues returns a list of metric values discovered by this poller.
	GetValues() []autoscaler.MetricValue
}
