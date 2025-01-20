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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/argoproj/argo-cd/v2/common"

	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	"github.com/plumber-cd/argocd-autoscaler/pollers/prometheus"
)

// RobustScalingNormalizerReconciler reconciles a RobustScalingNormalizer object
type RobustScalingNormalizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=robustscalingnormalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=robustscalingnormalizers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=robustscalingnormalizers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *RobustScalingNormalizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Received reconcile request")

	normalizer := &autoscaler.RobustScalingNormalizer{}
	if err := r.Get(ctx, req.NamespacedName, normalizer); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	values := []autoscaler.MetricValue{}

	switch normalizer.Spec.PollerRef.Kind {

	case "PrometheusPoll":

		poll := &autoscaler.PrometheusPoll{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      normalizer.Spec.PollerRef.Name,
			Namespace: normalizer.Namespace,
		}, poll); err != nil {
			err := fmt.Errorf("%s didn't exist %s/%s",
				normalizer.Spec.PollerRef.Kind, normalizer.Namespace, normalizer.Name)
			log.Error(err, "PrometheusPoll not found")
			meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
				Type:    typeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "PrometheusPollNotFound",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, normalizer); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		values = poll.Status.Values

	default:

		err := fmt.Errorf("Unknown poller Kind=%s in %s/%s",
			normalizer.Spec.PollerRef.Kind, normalizer.Namespace, normalizer.Name)
		log.Error(err, "Unknown poller Kind")
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "UnknownPollerKind",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil

	}

	// If the resource malformed - bail out early
	if len(values) == 0 {
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
			return ctrl.Result{}, err
		}
		// Re-queuing will change nothing - resource is malformed
		return ctrl.Result{}, nil
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
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Find all clusters
	clusters := &corev1.SecretList{}
	if err := r.List(ctx, clusters, &client.ListOptions{
		Namespace: poll.Namespace,
		LabelSelector: labels.Set(client.MatchingLabels{
			common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
		}).AsSelector(),
	}); err != nil {
		log.Error(err, "Failed to list clusters")
		meta.SetStatusCondition(&poll.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoPollingConfiguration",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, poll); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}
	log.V(1).Info("Found clusters", "count", len(clusters.Items))

	if poll.Status.LastPollingTime != nil {

		sinceLastPoll := time.Since(poll.Status.LastPollingTime.Time)
		log.V(1).Info("Reconciliation request for pre-existing poll", "sinceLastPoll", sinceLastPoll)

		if sinceLastPoll < poll.Spec.Period.Duration {

			// Check if the list of clusters has changed since the last poll
			previouslyObservedClusters := []string{}
			for _, metricValue := range poll.Status.Values {
				refStr := fmt.Sprintf("%s/%s",
					metricValue.OwnerRef.Kind,
					metricValue.OwnerRef.Name)
				if !slices.Contains(previouslyObservedClusters, refStr) {
					previouslyObservedClusters = append(previouslyObservedClusters, refStr)
				}
			}
			slices.Sort(previouslyObservedClusters)
			currentlyObservedClusters := []string{}
			for _, cluster := range clusters.Items {
				refStr := fmt.Sprintf("%s/%s",
					cluster.Kind,
					cluster.Name)
				if !slices.Contains(currentlyObservedClusters, refStr) {
					currentlyObservedClusters = append(currentlyObservedClusters, refStr)
				}
			}
			slices.Sort(currentlyObservedClusters)
			if slices.Equal(previouslyObservedClusters, currentlyObservedClusters) {
				remainingWaitTime := poll.Spec.Period.Duration - sinceLastPoll
				log.V(1).Info("Not enough time has passed since last poll, queuing up for remaining time",
					"remaining", remainingWaitTime)
				return ctrl.Result{RequeueAfter: remainingWaitTime}, nil
			}
			log.V(1).Info("Clusters have changed since last poll, re-queuing for immediate poll")
		}
	}

	poller := prometheus.Poller{}
	metrics, err := poller.Poll(ctx, *poll, clusters.Items)
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
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
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
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: poll.Spec.Period.Duration}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RobustScalingNormalizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pollerPredicate := predicate.NewPredicateFuncs(func(_ client.Object) bool {
		return true
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&autoscalerv1alpha1.RobustScalingNormalizer{}).
		Named("argocd_autoscaler_rubustscalingnormalizer").
		Watches(
			&autoscaler.PrometheusPoll{},
			handler.EnqueueRequestsFromMapFunc(r.mapPrometheusPolls),
			builder.WithPredicates(pollerPredicate),
		).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func (r *RobustScalingNormalizerReconciler) mapPrometheusPolls(
	ctx context.Context, object client.Object) []reconcile.Request {

	requests := []reconcile.Request{}
	namespace := object.GetNamespace()

	var normalizers autoscaler.RobustScalingNormalizerList
	if err := r.List(ctx, &normalizers, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list RobustScalingNormalizer", "Namespace", namespace)
		return requests
	}

	for _, normalizer := range normalizers.Items {
		if normalizer.Spec.PollerRef.APIGroup == ptr.To(object.GetObjectKind().GroupVersionKind().Group) &&
			normalizer.Spec.PollerRef.Kind == object.GetObjectKind().GroupVersionKind().Kind &&
			normalizer.Spec.PollerRef.Name == object.GetName() {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      normalizer.GetName(),
					Namespace: normalizer.GetNamespace(),
				},
			}
			requests = append(requests, req)
		}
	}

	return requests
}
