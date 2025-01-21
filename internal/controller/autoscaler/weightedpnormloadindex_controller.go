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
	"math"
	"strconv"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
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
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

var (
	knownMetricProviders      []schema.GroupVersionKind
	knownMetricProvidersMutex sync.Mutex
)

func RegisterMetricProvider(gvk schema.GroupVersionKind) {
	knownMetricProvidersMutex.Lock()
	defer knownMetricProvidersMutex.Unlock()
	knownMetricProviders = append(knownMetricProviders, gvk)
}

func getMetricProviders() []schema.GroupVersionKind {
	knownMetricProvidersMutex.Lock()
	defer knownMetricProvidersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownMetricProviders))
	copy(_copy, knownMetricProviders)
	return _copy
}

func init() {
	RegisterMetricProvider(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "PrometheusPoll",
	})
	RegisterMetricProvider(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "RobustScalingNormalizer",
	})
}

// WeightedPNormLoadIndexReconciler reconciles a WeightedPNormLoadIndex object
type WeightedPNormLoadIndexReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=weightedpnormloadindexes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=weightedpnormloadindexes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=weightedpnormloadindexes/finalizers,verbs=update
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=prometheuspolls,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=robustscalingnormalizers,verbs=get;list;watch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *WeightedPNormLoadIndexReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Received reconcile request")

	loadIndex := &autoscaler.WeightedPNormLoadIndex{}
	if err := r.Get(ctx, req.NamespacedName, loadIndex); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	weightsByID := map[string]autoscaler.WeightedPNormLoadIndexWeight{}
	for _, weight := range loadIndex.Spec.Weights {
		if _, exists := weightsByID[weight.ID]; exists {
			err := fmt.Errorf("duplicate weight definition for metric ID '%s'", weight.ID)
			log.Error(err, "Resource malformed")
			meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
				Type:    typeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "ResourceMalformed",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, loadIndex); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// Resource is malformed - re-queuing won't help
			return ctrl.Result{}, nil
		}
		weightsByID[weight.ID] = weight
	}

	metricProvider, err := findByRef[MetricProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		loadIndex.Namespace,
		loadIndex.Spec.MetricProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find metric provider by ref")
		meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingMetricProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, loadIndex); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when metric provider is created
		return ctrl.Result{}, err
	}

	if !meta.IsStatusConditionPresentAndEqual((*metricProvider).GetConditions(), typeReady, metav1.ConditionTrue) {
		meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
			Type:   typeReady,
			Status: metav1.ConditionFalse,
			Reason: "MetricProviderNotReady",
			Message: fmt.Sprintf("Check the status of a metric provider %s (api=%s, kind=%s)",
				loadIndex.Spec.MetricProviderRef.Name,
				*loadIndex.Spec.MetricProviderRef.APIGroup,
				loadIndex.Spec.MetricProviderRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, loadIndex); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when metric provider changes
		return ctrl.Result{}, nil
	}

	values := (*metricProvider).GetValues()

	if len(values) == 0 {
		err := fmt.Errorf("No metrics found")
		log.Error(err, "No metrics found, fail")
		meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoMetricsFound",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, loadIndex); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when metric provider changes
		return ctrl.Result{}, nil
	}

	metricsByShardByID := map[types.UID]map[string][]autoscaler.MetricValue{}
	metricsByShard := map[types.UID][]autoscaler.MetricValue{}
	for _, m := range values {
		if _, exists := metricsByShardByID[m.ShardUID]; !exists {
			metricsByShardByID[m.ShardUID] = map[string][]autoscaler.MetricValue{}
		}
		if _, exists := metricsByShardByID[m.ShardUID][m.ID]; !exists {
			metricsByShardByID[m.ShardUID][m.ID] = []autoscaler.MetricValue{}
		}
		metricsByShardByID[m.ShardUID][m.ID] = append(metricsByShardByID[m.ShardUID][m.ID], m)
		if _, exists := metricsByShard[m.ShardUID]; !exists {
			metricsByShard[m.ShardUID] = []autoscaler.MetricValue{}
		}
		metricsByShard[m.ShardUID] = append(metricsByShard[m.ShardUID], m)
	}
	for shardUID, shardMetricsByID := range metricsByShardByID {
		for metricID := range weightsByID {
			if values, exists := shardMetricsByID[metricID]; !exists || len(values) != 1 {
				err := fmt.Errorf("metric ID '%s' should exist exactly one time in shard %s", metricID, shardUID)
				log.Error(err, "Shard malformed")
				meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
					Type:    typeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "ShardMalformed",
					Message: err.Error(),
				})
				if err := r.Status().Update(ctx, loadIndex); err != nil {
					log.Error(err, "Failed to update resource status")
					return ctrl.Result{RequeueAfter: time.Second}, nil
				}
				// Wel will receive a new event when shard updates
				return ctrl.Result{}, nil
			}
		}
	}

	loadIndexes := []autoscaler.LoadIndex{}
	for shardUID, values := range metricsByShard {
		value, err := r.calculate(
			loadIndex.Spec.P,
			weightsByID,
			values,
		)
		if err != nil {
			log.Error(err, "Load Index calculation failed")
			meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
				Type:    typeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "CalculationError",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, loadIndex); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// Re-quering won't help if this is a math problem
			return ctrl.Result{}, nil
		}
		valueAsResource, err := resource.ParseQuantity(
			strconv.FormatFloat(value, 'f', -1, 32))
		loadIndexes = append(loadIndexes, autoscaler.LoadIndex{
			UID:   shardUID,
			Value: valueAsResource,
		})
	}

	loadIndex.Status.Values = loadIndexes

	if !meta.IsStatusConditionPresentAndEqual(loadIndex.Status.Conditions, typeAvailable, metav1.ConditionTrue) {
		meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
			Type:   typeAvailable,
			Status: metav1.ConditionTrue,
			Reason: "InitialCalculationSuccessful",
		})
	}
	meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
		Type:   typeReady,
		Status: metav1.ConditionTrue,
		Reason: "CalculationSuccessful",
	})
	if err := r.Status().Update(ctx, loadIndex); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// We don't need to do anything unless metric provider data changes
	return ctrl.Result{}, nil
}

func (r *WeightedPNormLoadIndexReconciler) calculate(
	p int32,
	weights map[string]autoscaler.WeightedPNormLoadIndexWeight,
	values []autoscaler.MetricValue,
) (float64, error) {

	sum := 0.0
	for _, m := range values {
		weight, ok := weights[m.ID]
		if !ok {
			return 0, fmt.Errorf("failed to find weight for metric ID '%s'", m.ID)
		}
		weightValue := weight.Weight.AsApproximateFloat64()
		value := m.Value.AsApproximateFloat64()
		if weight.Negative != nil && !*weight.Negative && value < 0 {
			value = float64(0)
		}
		sum += weightValue * math.Pow(value, float64(p))
	}

	return math.Pow(sum, float64(1)/float64(p)), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightedPNormLoadIndexReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_weighted_p_norm_load_index").
		For(
			&autoscalerv1alpha1.WeightedPNormLoadIndex{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		)

	for _, gvk := range getMetricProviders() {
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
			handler.EnqueueRequestsFromMapFunc(r.mapMetricProvider),
		)
	}

	c = c.
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		})
	return c.Complete(r)
}

func (r *WeightedPNormLoadIndexReconciler) mapMetricProvider(
	ctx context.Context, object client.Object) []reconcile.Request {

	requests := []reconcile.Request{}

	gvk, _, err := r.Scheme.ObjectKinds(object)
	if err != nil || len(gvk) == 0 {
		log.Log.Error(err, "Failed to determine GVK for object")
		return requests
	}
	namespace := object.GetNamespace()

	var loadIndexes autoscaler.WeightedPNormLoadIndexList
	if err := r.List(ctx, &loadIndexes, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list WeightedPNormLoadIndex", "Namespace", namespace)
		return requests
	}

	for _, loadIndex := range loadIndexes.Items {
		for _, _gvk := range gvk {
			if *loadIndex.Spec.MetricProviderRef.APIGroup == _gvk.Group &&
				loadIndex.Spec.MetricProviderRef.Kind == _gvk.Kind &&
				loadIndex.Spec.MetricProviderRef.Name == object.GetName() {
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      loadIndex.GetName(),
						Namespace: loadIndex.GetNamespace(),
					},
				}
				requests = append(requests, req)
				break
			}
		}
	}

	return requests
}
