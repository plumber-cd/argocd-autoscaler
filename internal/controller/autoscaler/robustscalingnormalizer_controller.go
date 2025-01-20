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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
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
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoPollingConfiguration",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{}, err
		}
		// Re-queuing will change nothing - resource is malformed
		return ctrl.Result{}, nil
	}

	// First, change the availability condition from unknown to false -
	// once it is set to true at the very end, we won't touch it ever again.
	if meta.IsStatusConditionPresentAndEqual(normalizer.Status.Conditions, typeAvailable, metav1.ConditionUnknown) {
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:    typeAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  "InitialNormalization",
			Message: "This is the first poll",
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// TODO: normalize here
	normalizer.Status.Values = values
	if !meta.IsStatusConditionPresentAndEqual(normalizer.Status.Conditions, typeAvailable, metav1.ConditionTrue) {
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:   typeAvailable,
			Status: metav1.ConditionTrue,
			Reason: "InitialNormalizationSuccessful",
		})
	}
	meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
		Type:   typeReady,
		Status: metav1.ConditionTrue,
		Reason: "NormalizationSuccessful",
	})
	if err := r.Status().Update(ctx, normalizer); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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
