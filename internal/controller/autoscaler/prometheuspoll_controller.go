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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

const (
	// typeAvailable represents the status of the resource reconciliation
	typeAvailable = "Available"
	// typeReady represents the status of the resourece
	typeReady = "Ready"
)

// PrometheusPollReconciler reconciles a PrometheusPoll object
type PrometheusPollReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls/finalizers,verbs=update

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
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: poll.Spec.InitialDelay.Duration}, nil
	}

	// TODO: check here if clusters were added/removed

	// Now we check if the last polling hasn't been expired yet, we then should re-queue for the remainder of its time
	if poll.Status.LastPollingTime != nil {
		sinceLastPoll := time.Since(poll.Status.LastPollingTime.Time)
		log.V(1).Info("Reconciliation request for pre-existing poll", "sinceLastPoll", sinceLastPoll)
		if sinceLastPoll < poll.Spec.Period.Duration {
			remainingWaitTime := poll.Spec.Period.Duration - sinceLastPoll
			log.V(1).Info("Not enough time has passed since last poll, queuing up for remaining time",
				"remaining", remainingWaitTime)
			return ctrl.Result{RequeueAfter: remainingWaitTime}, nil
		}
	}

	// Ok, at this point we are almost ready to poll.
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
			return ctrl.Result{}, err
		}
		// Re-queuing will change nothing - resource is malformed
		return ctrl.Result{}, nil
	}

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
func (r *PrometheusPollReconciler) SetupWithManager(mgr ctrl.Manager) error {
	secretPredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		labels := object.GetLabels()
		if labels == nil {
			return false
		}
		if val, ok := labels[common.LabelKeySecretType]; ok && val == common.LabelValueSecretTypeCluster {
			return true
		}
		return false
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&autoscalerv1alpha1.PrometheusPoll{}).
		Named("argocd_autoscaler_prometheuspoll").
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapClusterSecret),
			builder.WithPredicates(secretPredicate),
		).
		Owns(&corev1.Secret{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func (r *PrometheusPollReconciler) mapClusterSecret(ctx context.Context, object client.Object) []reconcile.Request {
	var requests []reconcile.Request
	namespace := object.GetNamespace()

	var polls autoscaler.PrometheusPollList
	if err := r.List(ctx, &polls, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list PrometheusPolls", "Namespace", namespace)
		return requests
	}

	for _, poll := range polls.Items {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      poll.GetName(),
				Namespace: poll.GetNamespace(),
			},
		}
		requests = append(requests, req)
	}

	return requests
}
