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
	"sort"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
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

	metricValuesProvider, err := findByRef[common.MetricValuesProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		normalizer.Namespace,
		*normalizer.Spec.MetricValuesProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find poller by ref")
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingMetricsProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when metrics provider is created
		return ctrl.Result{}, err
	}

	if !meta.IsStatusConditionPresentAndEqual(metricValuesProvider.GetMetricValuesProviderStatus().Conditions, StatusTypeReady, metav1.ConditionTrue) {
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
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when poller changes
		return ctrl.Result{}, nil
	}

	values := metricValuesProvider.GetMetricValuesProviderStatus().Values
	if len(values) == 0 {
		err := fmt.Errorf("No metrics found")
		log.Error(err, "No metrics found, fail")
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoMetricsFound",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when poller changes
		return ctrl.Result{}, nil
	}

	normalizedValues, err := r.normalize(ctx, values)
	if err != nil {
		log.Error(err, "Error during normalization")
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorDuringNormalization",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, normalizer); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// If this is a math problem - re-queuing won't help
		return ctrl.Result{}, nil
	}

	normalizer.Status.Values = normalizedValues
	if !meta.IsStatusConditionPresentAndEqual(normalizer.Status.Conditions, StatusTypeAvailable, metav1.ConditionTrue) {
		meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
			Type:   StatusTypeAvailable,
			Status: metav1.ConditionTrue,
			Reason: StatusTypeAvailable,
		})
	}
	meta.SetStatusCondition(&normalizer.Status.Conditions, metav1.Condition{
		Type:   StatusTypeReady,
		Status: metav1.ConditionTrue,
		Reason: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, normalizer); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// We don't need to do anything unless poller data changes
	return ctrl.Result{}, nil
}

func (r *RobustScalingNormalizerReconciler) normalize(ctx context.Context,
	allMetrics []common.MetricValue) ([]common.MetricValue, error) {

	log := log.FromContext(ctx)

	// First - we need to separate metrics by ID
	metricsByID := map[string][]common.MetricValue{}
	for _, metric := range allMetrics {
		if _, ok := metricsByID[metric.ID]; !ok {
			metricsByID[metric.ID] = []common.MetricValue{}
		}
		metricsByID[metric.ID] = append(metricsByID[metric.ID], metric)
	}

	// Now, normalize them within their group
	normalizedMetrics := []common.MetricValue{}
	for _, metrics := range metricsByID {
		all := []float64{}
		for _, metric := range metrics {
			all = append(all, metric.Value.AsApproximateFloat64())
		}

		sorted := make([]float64, len(all))
		copy(sorted, all)
		sort.Float64s(sorted)

		// Compute median
		median := computeMedian(sorted)

		var iqr float64
		if len(sorted) > 1 {
			// Compute Q1 and Q3
			q1, q3 := computeQuartiles(sorted)

			// Compute IQR
			iqr = q3 - q1
		} else {
			// Assume IQR 0 which will signal to the scaling function to also use 0 as scaled value
			// See comments below on why that is
			iqr = 0
		}

		// Scale each value
		for _, metric := range metrics {
			var normalized float64
			// If IQR was 0, either all values were identical or not enough data to compute IQR.
			// Either value we use, as long as it is the same value for all shards, it would signify no deviation.
			// Typically, you would use 0 which is exact median, which then later can scale based on the wage.
			if iqr == 0 {
				normalized = 0
			} else {
				normalized = (metric.Value.AsApproximateFloat64() - median) / iqr
			}
			normalizedAsString := strconv.FormatFloat(normalized, 'f', -1, 64)
			normalizedAsResource, err := resource.ParseQuantity(normalizedAsString)
			if err != nil {
				log.Error(err, "Failed to parse normalized value as resource", "normalized", normalized)
				return nil, err
			}
			normalizedMetrics = append(normalizedMetrics, common.MetricValue{
				ID:           metric.ID,
				Shard:        metric.Shard,
				Query:        metric.Query,
				Value:        normalizedAsResource,
				DisplayValue: normalizedAsString,
			})
		}
	}

	return normalizedMetrics, nil
}

// computeMedian computes the median of a sorted slice of float64.
// Assumes the slice is already sorted.
func computeMedian(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		panic("empty slice")
	}
	middle := n / 2
	if n%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}
	return sorted[middle]
}

// computeQuartiles computes Q1 and Q3 of a sorted slice of float64.
// Assumes the slice is already sorted.
func computeQuartiles(sorted []float64) (q1, q3 float64) {
	n := len(sorted)

	// Q1: Median of the lower half
	lowerHalf := sorted[:n/2]
	q1 = computeMedian(lowerHalf)

	// Q3: Median of the upper half
	var upperHalf []float64
	if n%2 == 0 {
		upperHalf = sorted[n/2:]
	} else {
		upperHalf = sorted[n/2+1:]
	}
	q3 = computeMedian(upperHalf)

	return q1, q3
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
