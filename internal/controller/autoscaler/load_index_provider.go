package autoscaler

import (
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LoadIndexProvider is an interface that should be implemented by all load index providers.
type LoadIndexProvider interface {
	// GetConditions returns a list of conditions of this provider.
	GetConditions() []metav1.Condition
	// GetValues returns a list of load index values calculated by this provider.
	GetValues() []autoscaler.LoadIndex
}
