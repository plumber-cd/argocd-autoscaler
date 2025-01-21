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
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	"github.com/plumber-cd/argocd-autoscaler/pollers/prometheus"
)

var (
	knownDiscoverers      []schema.GroupVersionKind
	knownDiscoverersMutex sync.Mutex
)

func RegisterDiscoverer(gvk schema.GroupVersionKind) {
	knownDiscoverersMutex.Lock()
	defer knownDiscoverersMutex.Unlock()
	knownDiscoverers = append(knownDiscoverers, gvk)
}

func getDiscoverers() []schema.GroupVersionKind {
	knownDiscoverersMutex.Lock()
	defer knownDiscoverersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownDiscoverers))
	copy(_copy, knownDiscoverers)
	return _copy
}

func init() {
	RegisterDiscoverer(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "SecretTypeClusterDiscovery",
	})
}

// PrometheusPollReconciler reconciles a PrometheusPoll object
type PrometheusPollReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls/finalizers,verbs=update
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=secrettypeclusterdiscoveries,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PrometheusPollReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Received reconcile request")

	poll := &autoscaler.PrometheusPoll{}
	if err := r.Get(ctx, req.NamespacedName, poll); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	// After initial creation of the new poll or individual cluster,
	// we need to wait a little for some metrics to become available.
	// We will check if this resource has no conditions and re-queue it for initial cooldown parameter.
	if len(poll.Status.Conditions) == 0 {
		log.Info("First reconciliation request, re-queuing for initial delay",
			"cooldown", poll.Spec.InitialDelay)
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    typeAvailable,
			Status:  metav1.ConditionUnknown,
			Reason:  "InitialDelay",
			Message: "Initial delay before initial reconciliation...",
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: poll.Spec.InitialDelay.Duration}, nil
	}

	// If the resource malformed - bail out early
	if len(poll.Spec.Metrics) == 0 {
		err := fmt.Errorf("No polling configuration present")
		log.Error(err, "No polling configuration present, fail")
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoPollingConfiguration",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// Re-queuing will change nothing - resource is malformed
		return ctrl.Result{}, nil
	}

	metricIDs := map[string]bool{}
	for _, metric := range poll.Spec.Metrics {
		if _, exists := metricIDs[metric.ID]; exists {
			err := fmt.Errorf("duplicate metric for ID '%s'", metric.ID)
			log.Error(err, "No polling configuration present, fail")
			meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
				Type:    typeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "NoPollingConfiguration",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, poll); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// Re-queuing will change nothing - resource is malformed
			return ctrl.Result{}, nil
		}
		metricIDs[metric.ID] = true
	}

	// First, change the availability condition from unknown to false -
	// once it is set to true at the very end, we won't touch it ever again.
	if meta.IsStatusConditionPresentAndEqual(poll.Status.Conditions, typeAvailable, metav1.ConditionUnknown) {
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    typeAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  "InitialPoll",
			Message: "This is the first poll",
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	discoverer, err := findByRef[Discoverer](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		poll.Namespace,
		poll.Spec.DiscovererRef,
	)
	if err != nil {
		log.Error(err, "Failed to find discoverer by ref")
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingDiscoverer",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should receive an event if discoverer is created
		return ctrl.Result{}, err
	}

	if !meta.IsStatusConditionPresentAndEqual((*discoverer).GetConditions(), typeReady, metav1.ConditionTrue) {
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:   typeReady,
			Status: metav1.ConditionFalse,
			Reason: "DiscoveredNotReady",
			Message: fmt.Sprintf("Check the status of discoverer %s (api=%s, kind=%s)",
				poll.Spec.DiscovererRef.Name,
				*poll.Spec.DiscovererRef.APIGroup,
				poll.Spec.DiscovererRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when discoverer changes
		return ctrl.Result{}, nil
	}

	shards := (*discoverer).GetShards()
	if len(shards) == 0 {
		err := fmt.Errorf("No shards found")
		log.Error(err, "No shards found, fail")
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoShardsFound",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when discovery changes
		return ctrl.Result{}, nil
	}

	if poll.Status.LastPollingTime != nil {

		sinceLastPoll := time.Since(poll.Status.LastPollingTime.Time)
		log.V(1).Info("Reconciliation request for pre-existing poll", "sinceLastPoll", sinceLastPoll)

		if sinceLastPoll < poll.Spec.Period.Duration {

			// Check if the list of shards has changed since the last poll
			previouslyObservedShards := []types.UID{}
			for _, metricValue := range poll.Status.Values {
				if !slices.Contains(previouslyObservedShards, metricValue.ShardUID) {
					previouslyObservedShards = append(previouslyObservedShards, metricValue.ShardUID)
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

			if slices.Equal(previouslyObservedShards, currentlyObservedShards) {
				remainingWaitTime := poll.Spec.Period.Duration - sinceLastPoll
				log.V(1).Info("Not enough time has passed since last poll, queuing up for remaining time",
					"remaining", remainingWaitTime)
				return ctrl.Result{RequeueAfter: remainingWaitTime}, nil
			}
			log.V(1).Info("Shards have changed since last poll, re-queuing for immediate poll")
		}
	}

	poller := prometheus.Poller{}
	metrics, err := poller.Poll(ctx, *poll, shards)
	if err != nil {
		log.Error(err, "Failed to poll metrics")
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "PollingError",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: time.Second}, err
	}

	if len(metrics) == 0 {
		err := fmt.Errorf("No metrics found")
		log.Error(err, "No metrics found, fail")
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoMetricsFound",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: poll.Spec.Period.Duration}, nil
	}

	log.V(1).Info("Polled metrics", "count", len(metrics))

	// Update the status with the new values
	poll.Status.Values = metrics
	poll.Status.LastPollingTime = &metav1.Time{Time: time.Now()}
	if !meta.IsStatusConditionPresentAndEqual(poll.Status.Conditions, typeAvailable, metav1.ConditionTrue) {
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:   typeAvailable,
			Status: metav1.ConditionTrue,
			Reason: "InitialPollSuccessful",
		})
	}
	meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
		Type:   typeReady,
		Status: metav1.ConditionTrue,
		Reason: "PollingSuccessful",
	})
	if err := r.Status().Update(ctx, poll); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

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

	for _, gvk := range getDiscoverers() {
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
			handler.EnqueueRequestsFromMapFunc(r.mapDiscoverer),
		)
	}

	c = c.
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		})
	return c.Complete(r)
}

func (r *PrometheusPollReconciler) mapDiscoverer(ctx context.Context, object client.Object) []reconcile.Request {
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
			if *poll.Spec.DiscovererRef.APIGroup == _gvk.Group &&
				poll.Spec.DiscovererRef.Kind == _gvk.Kind &&
				poll.Spec.DiscovererRef.Name == object.GetName() {
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
