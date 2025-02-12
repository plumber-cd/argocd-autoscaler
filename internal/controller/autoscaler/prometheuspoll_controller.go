/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package autoscaler

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	poller "github.com/plumber-cd/argocd-autoscaler/pollers/prometheus"
)

// PrometheusPollReconciler reconciles a PrometheusPoll object
type PrometheusPollReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Poller poller.Poller
}

var (
	prometheusPollValuesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "poll",
			Name:        "values",
			Help:        "Metrics polled by this poller",
			ConstLabels: prometheus.Labels{"poll_type": "prometheus"},
		},
		[]string{
			"poll_ref",
			"shard_uid",
			"shard_id",
			"shard_namespace",
			"shard_name",
			"shard_server",
			"metric_id",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(prometheusPollValuesGauge)
}

// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PrometheusPollReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("Received reconcile request")
	defer log.V(2).Info("Reconcile request completed")

	poll := &autoscaler.PrometheusPoll{}
	if err := r.Get(ctx, req.NamespacedName, poll); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	metricIDs := map[string]bool{}
	for _, metric := range poll.Spec.Metrics {
		log.V(2).Info("Reading metric configuration", "metric", metric.ID)
		if _, exists := metricIDs[metric.ID]; exists {
			err := fmt.Errorf("duplicate metric for ID '%s'", metric.ID)
			log.Error(err, "Malformed resource")
			meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "MalformedResource",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, poll); err != nil {
				log.V(1).Info("Failed to update resource status", "err", err)
				return ctrl.Result{}, err
			}
			// Re-queuing will change nothing - resource is malformed
			return ctrl.Result{}, nil
		}
		metricIDs[metric.ID] = true
	}
	log.V(1).Info("Metrics read", "count", len(poll.Spec.Metrics))

	shardsProvider, err := findByRef[common.ShardsProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		poll.Namespace,
		*poll.Spec.ShardManagerRef,
	)
	if err != nil {
		log.Error(err, "Failed to find shard manager by ref")
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingShardManager",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should receive an event if shard manager is created
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Shard manager found", "shardManager", poll.Spec.ShardManagerRef)

	if !meta.IsStatusConditionPresentAndEqual(shardsProvider.GetShardProviderStatus().Conditions, StatusTypeReady, metav1.ConditionTrue) {
		log.V(1).Info("Shard manager not ready", "shardManager", poll.Spec.ShardManagerRef)
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:   StatusTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "ShardManagerNotReady",
			Message: fmt.Sprintf("Check the status of shard manager %s (api=%s, kind=%s)",
				poll.Spec.ShardManagerRef.Name,
				*poll.Spec.ShardManagerRef.APIGroup,
				poll.Spec.ShardManagerRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when shard manager changes
		return ctrl.Result{}, nil
	}

	shards := shardsProvider.GetShardProviderStatus().Shards
	if poll.Status.LastPollingTime != nil {
		sinceLastPoll := time.Since(poll.Status.LastPollingTime.Time)
		log.V(2).Info("Reconciliation request for pre-existing poll", "sinceLastPoll", sinceLastPoll)

		if sinceLastPoll < poll.Spec.Period.Duration {
			log.V(2).Info("Not enough time has passed since last poll, checking if shards or metrics have changed")
			// Check if the list of shards has changed since the last poll
			previouslyObservedShards := []types.UID{}
			for _, metricValue := range poll.Status.Values {
				if !slices.Contains(previouslyObservedShards, metricValue.Shard.UID) {
					previouslyObservedShards = append(previouslyObservedShards, metricValue.Shard.UID)
				}
			}
			slices.Sort(previouslyObservedShards)

			currentlyObservedShards := []types.UID{}
			for _, shard := range shards {
				if !slices.Contains(currentlyObservedShards, shard.UID) {
					currentlyObservedShards = append(currentlyObservedShards, shard.UID)
				}
			}
			slices.Sort(currentlyObservedShards)

			previouslyObserverMetrics := []string{}
			for _, metricValue := range poll.Status.Values {
				if !slices.Contains(previouslyObserverMetrics, metricValue.ID) {
					previouslyObserverMetrics = append(previouslyObserverMetrics, metricValue.ID)
				}
			}
			slices.Sort(previouslyObserverMetrics)

			currentlyObservedMetrics := []string{}
			for _, metric := range poll.Spec.Metrics {
				if !slices.Contains(currentlyObservedMetrics, metric.ID) {
					currentlyObservedMetrics = append(currentlyObservedMetrics, metric.ID)
				}
			}
			slices.Sort(currentlyObservedMetrics)

			log.V(2).Info("Comparing shards and metrics",
				"previouslyObservedShards", previouslyObservedShards,
				"currentlyObservedShards", currentlyObservedShards,
				"previouslyObserverMetrics", previouslyObserverMetrics,
				"currentlyObservedMetrics", currentlyObservedMetrics)

			if slices.Equal(previouslyObservedShards, currentlyObservedShards) &&
				slices.Equal(previouslyObserverMetrics, currentlyObservedMetrics) {
				remainingWaitTime := poll.Spec.Period.Duration - sinceLastPoll
				log.V(1).Info("Not enough time has passed since last poll, queuing up for remaining time",
					"remaining", remainingWaitTime)
				return ctrl.Result{RequeueAfter: remainingWaitTime}, nil
			}
			log.V(2).Info("Shards or metrics have changed since last poll, re-queuing immediately")
		}
	}

	metrics, err := r.Poller.Poll(ctx, *poll, shards)
	if err != nil {
		log.Error(err, "Failed to poll metrics")
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "PollingError",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}
	log.V(1).Info("Polled metrics", "count", len(metrics))
	prometheusPollValuesGauge.DeletePartialMatch(prometheus.Labels{
		"poll_ref": req.NamespacedName.String(),
	})
	for _, metric := range metrics {
		log.V(2).Info("Metric polled", "metric", metric.ID, "value", metric.Value)
		prometheusPollValuesGauge.WithLabelValues(
			req.NamespacedName.String(),
			string(metric.Shard.UID),
			metric.Shard.ID,
			metric.Shard.Namespace,
			metric.Shard.Name,
			metric.Shard.Server,
			metric.ID,
		).Set(metric.Value.AsApproximateFloat64())
	}

	// Update the status with the new values
	poll.Status.Values = metrics
	poll.Status.LastPollingTime = &metav1.Time{Time: time.Now()}
	meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
		Type:   StatusTypeReady,
		Status: metav1.ConditionTrue,
		Reason: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, poll); err != nil {
		log.V(1).Info("Failed to update resource status", "err", err)
		return ctrl.Result{}, err
	}
	log.Info("Resource status updated", "values", len(metrics), "lastPollingTime", poll.Status.LastPollingTime)

	return ctrl.Result{RequeueAfter: poll.Spec.Period.Duration}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrometheusPollReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_prometheus_poll").
		For(
			&autoscaler.PrometheusPoll{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		)

	for _, gvk := range getShardManagers() {
		obj, err := mgr.GetScheme().New(gvk)
		if err != nil {
			return fmt.Errorf("failed to create object for GVK %v: %w", gvk, err)
		}

		clientObj, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("object for GVK %s does not implement client.Object", gvk.String())
		}

		c = c.Watches(
			clientObj,
			handler.EnqueueRequestsFromMapFunc(r.mapShardManager),
		)
	}

	c = c.
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		})
	return c.Complete(r)
}

func (r *PrometheusPollReconciler) mapShardManager(ctx context.Context, object client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	gvk, _, err := r.Scheme.ObjectKinds(object)
	if err != nil || len(gvk) == 0 {
		log.Log.Error(err, "Failed to determine GVK for object")
		return requests
	}
	namespace := object.GetNamespace()

	var polls autoscaler.PrometheusPollList
	if err := r.List(ctx, &polls, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list PrometheusPolls", "Namespace", namespace)
		return requests
	}

	for _, poll := range polls.Items {
		for _, _gvk := range gvk {
			if *poll.Spec.ShardManagerRef.APIGroup == _gvk.Group &&
				poll.Spec.ShardManagerRef.Kind == _gvk.Kind &&
				poll.Spec.ShardManagerRef.Name == object.GetName() {
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      poll.GetName(),
						Namespace: poll.GetNamespace(),
					},
				}
				requests = append(requests, req)
				break
			}
		}
	}

	return requests
}
