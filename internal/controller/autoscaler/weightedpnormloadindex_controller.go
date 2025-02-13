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
	"sync"
	"time"

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
	"github.com/plumber-cd/argocd-autoscaler/loadindexers/weightedpnorm"
	"github.com/prometheus/client_golang/prometheus"
)

// WeightedPNormLoadIndexReconciler reconciles a WeightedPNormLoadIndex object
type WeightedPNormLoadIndexReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	LoadIndexer weightedpnorm.LoadIndexer

	lastReconciled sync.Map
}

var (
	weightedPNormLoadIndexValuesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "load_index",
			Name:        "values",
			Help:        "Load Index values",
			ConstLabels: prometheus.Labels{"load_index_type": "weighted_p_norm"},
		},
		[]string{
			"load_index_ref",
			"shard_uid",
			"shard_id",
			"shard_namespace",
			"shard_name",
			"shard_server",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(weightedPNormLoadIndexValuesGauge)
}

// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=weightedpnormloadindexes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=weightedpnormloadindexes/status,verbs=get;update;patch
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=weightedpnormloadindexes/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *WeightedPNormLoadIndexReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("Received reconcile request")
	defer log.V(2).Info("Reconcile request completed")

	if lastTimeRaw, exists := r.lastReconciled.Load(req.NamespacedName.String()); exists {
		lastTime := lastTimeRaw.(time.Time)
		if time.Since(lastTime) < GlobalRateLimit {
			log.V(2).Info("Rate limiting", "since", time.Since(lastTime))
			return ctrl.Result{RequeueAfter: GlobalRateLimit - time.Since(lastTime)}, nil
		}
	}
	r.lastReconciled.Store(req.NamespacedName.String(), time.Now())

	loadIndex := &autoscaler.WeightedPNormLoadIndex{}
	if err := r.Get(ctx, req.NamespacedName, loadIndex); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	weightsByID := map[string]autoscaler.WeightedPNormLoadIndexWeight{}
	for _, weight := range loadIndex.Spec.Weights {
		log.V(2).Info("Reading weights configuration", "metric", weight.ID)
		if _, exists := weightsByID[weight.ID]; exists {
			err := fmt.Errorf("duplicate weight configuration for metric ID '%s'", weight.ID)
			log.Error(err, "Resource malformed")
			meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "WeightsMalformed",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, loadIndex); err != nil {
				log.V(1).Info("Failed to update resource status", "err", err)
				return ctrl.Result{}, err
			}
			// Resource is malformed - re-queuing won't help
			return ctrl.Result{}, nil
		}
		weightsByID[weight.ID] = weight
	}
	log.V(1).Info("Weights configuration read", "count", len(weightsByID))

	metricValuesProvider, err := findByRef[common.MetricValuesProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		loadIndex.Namespace,
		*loadIndex.Spec.MetricValuesProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find metric provider by ref")
		meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingMetricProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, loadIndex); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when metric provider is created
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Metric values provider found",
		"metricValuesProvider", loadIndex.Spec.MetricValuesProviderRef)

	if !meta.IsStatusConditionPresentAndEqual(
		metricValuesProvider.GetMetricValuesProviderStatus().Conditions, StatusTypeReady, metav1.ConditionTrue) {
		log.V(1).Info("Metric values provider not ready",
			"metricValuesProvider", loadIndex.Spec.MetricValuesProviderRef)
		meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
			Type:   StatusTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "MetricValuesProviderNotReady",
			Message: fmt.Sprintf("Check the status of metric values provider %s (api=%s, kind=%s)",
				loadIndex.Spec.MetricValuesProviderRef.Name,
				*loadIndex.Spec.MetricValuesProviderRef.APIGroup,
				loadIndex.Spec.MetricValuesProviderRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, loadIndex); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when metric provider changes
		return ctrl.Result{}, nil
	}

	values := metricValuesProvider.GetMetricValuesProviderStatus().Values
	loadIndexes, err := r.LoadIndexer.Calculate(
		loadIndex.Spec.P,
		loadIndex.Spec.OffsetE.AsApproximateFloat64(),
		weightsByID,
		values,
	)
	if err != nil {
		log.Error(err, "Load Index calculation failed")
		meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "LoadIndexCalculationError",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, loadIndex); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// re-queuing won't help if this is a math problem
		// It could also be malformed data from the upstream, but net result is the same
		return ctrl.Result{}, nil
	}
	log.Info("Load indexes calculated", "count", len(loadIndexes))
	weightedPNormLoadIndexValuesGauge.DeletePartialMatch(prometheus.Labels{
		"load_index_ref": req.NamespacedName.String(),
	})
	for _, li := range loadIndexes {
		log.V(2).Info("Load Index", "value", li.Value)
		weightedPNormLoadIndexValuesGauge.WithLabelValues(
			req.NamespacedName.String(),
			string(li.Shard.UID),
			li.Shard.ID,
			li.Shard.Namespace,
			li.Shard.Name,
			li.Shard.Server,
		).Set(li.Value.AsApproximateFloat64())
	}

	loadIndex.Status.Values = loadIndexes
	meta.SetStatusCondition(&loadIndex.Status.Conditions, metav1.Condition{
		Type:   StatusTypeReady,
		Status: metav1.ConditionTrue,
		Reason: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, loadIndex); err != nil {
		log.V(1).Info("Failed to update resource status", "err", err)
		return ctrl.Result{}, err
	}
	log.Info("Resource status updated", "indexes", len(loadIndexes))

	// We don't need to do anything unless metric provider data changes
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightedPNormLoadIndexReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_weighted_p_norm_load_index").
		For(
			&autoscalerv1alpha1.WeightedPNormLoadIndex{},
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
			if *loadIndex.Spec.MetricValuesProviderRef.APIGroup == _gvk.Group &&
				loadIndex.Spec.MetricValuesProviderRef.Kind == _gvk.Kind &&
				loadIndex.Spec.MetricValuesProviderRef.Name == object.GetName() {
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
