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
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	robustscaling "github.com/plumber-cd/argocd-autoscaler/normalizers/robustscaling"
	"github.com/prometheus/client_golang/prometheus"
)

// RobustScalingNormalizerReconciler reconciles a RobustScalingNormalizer object
type RobustScalingNormalizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Normalizer robustscaling.Normalizer
}

var (
	robustScalingNormalizerValuesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "normalizer",
			Name:        "values",
			Help:        "Metrics normalized by this normalizer",
			ConstLabels: prometheus.Labels{"normalizer_type": "robust_scaling"},
		},
		[]string{
			"normalizer_ref",
			"shard_id",
			"metric_id",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(robustScalingNormalizerValuesGauge)
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
	log.V(2).Info("Received reconcile request")
	defer log.V(2).Info("Reconcile request completed")

	normalizer := &autoscaler.RobustScalingNormalizer{}
	if err := r.Get(ctx, req.NamespacedName, normalizer); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	metricValuesProvider, err := findByRef[common.MetricValuesProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		normalizer.Namespace,
		*normalizer.Spec.MetricValuesProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find metric provider by ref")
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingMetricsProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when metrics provider is created
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Metric values provider found", "metricValuesProvider", normalizer.Spec.MetricValuesProviderRef)

	if !meta.IsStatusConditionPresentAndEqual(
		metricValuesProvider.GetMetricValuesProviderStatus().Conditions, StatusTypeReady, metav1.ConditionTrue) {
		log.V(1).Info("Metric values provider not ready", "metricValuesProvider", normalizer.Spec.MetricValuesProviderRef)
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:   StatusTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "MetricValuesProviderNotReady",
			Message: fmt.Sprintf("Check the status of metric values provider %s (api=%s, kind=%s)",
				normalizer.Spec.MetricValuesProviderRef.Name,
				*normalizer.Spec.MetricValuesProviderRef.APIGroup,
				normalizer.Spec.MetricValuesProviderRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when poller changes
		return ctrl.Result{}, nil
	}

	values := metricValuesProvider.GetMetricValuesProviderStatus().Values
	normalizedValues, err := r.Normalizer.Normalize(ctx, values)
	if err != nil {
		log.Error(err, "Error during normalization")
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorDuringNormalization",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// If this is a math problem - re-queuing won't help
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Metrics normalized", "count", len(normalizedValues))
	robustScalingNormalizerValuesGauge.DeletePartialMatch(prometheus.Labels{
		"normalizer_ref": req.NamespacedName.String(),
	})
	for _, metric := range normalizedValues {
		log.V(2).Info("Metric normalized", "metric", metric.ID, "value", metric.Value)
		robustScalingNormalizerValuesGauge.WithLabelValues(
			req.NamespacedName.String(),
			metric.Shard.ID,
			metric.ID,
		).Set(metric.Value.AsApproximateFloat64())
	}

	normalizer.Status.Values = normalizedValues
	meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
		Type:   StatusTypeReady,
		Status: metav1.ConditionTrue,
		Reason: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, normalizer); err != nil {
		log.V(1).Info("Failed to update resource status", "err", err)
		return ctrl.Result{}, err
	}
	log.Info("Resource status updated", "values", len(normalizedValues))

	// We don't need to do anything unless poller data changes
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RobustScalingNormalizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_rubust_scaling_normalizer").
		For(
			&autoscalerv1alpha1.RobustScalingNormalizer{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		)

	for _, gvk := range getMetricValuesProviders() {
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
			handler.EnqueueRequestsFromMapFunc(r.mapPoller),
		)
	}

	c = c.
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		})
	return c.Complete(r)
}

func (r *RobustScalingNormalizerReconciler) mapPoller(
	ctx context.Context, object client.Object) []reconcile.Request {

	requests := []reconcile.Request{}

	gvk, _, err := r.Scheme.ObjectKinds(object)
	if err != nil || len(gvk) == 0 {
		log.Log.Error(err, "Failed to determine GVK for object")
		return requests
	}
	namespace := object.GetNamespace()

	var normalizers autoscaler.RobustScalingNormalizerList
	if err := r.List(ctx, &normalizers, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list RobustScalingNormalizer", "Namespace", namespace)
		return requests
	}

	for _, normalizer := range normalizers.Items {
		for _, _gvk := range gvk {
			if *normalizer.Spec.MetricValuesProviderRef.APIGroup == _gvk.Group &&
				normalizer.Spec.MetricValuesProviderRef.Kind == _gvk.Kind &&
				normalizer.Spec.MetricValuesProviderRef.Name == object.GetName() {
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      normalizer.GetName(),
						Namespace: normalizer.GetNamespace(),
					},
				}
				requests = append(requests, req)
				break
			}
		}
	}

	return requests
}
