// Package pollers provides pollers for different metric sources.
package pollers

import (
	"context"

	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

// Poller is the interface for polling metrics.
type Poller interface {
	// Poll polls the metrics.
	Poll(
		ctx context.Context,
		poll *autoscaler.Poll,
	) ([]*autoscaler.MetricValue, error)
}
