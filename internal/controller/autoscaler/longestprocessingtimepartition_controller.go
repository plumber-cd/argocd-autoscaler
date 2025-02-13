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
	"github.com/plumber-cd/argocd-autoscaler/partitioners/longestprocessingtime"
	"github.com/prometheus/client_golang/prometheus"
)

// LongestProcessingTimePartitionReconciler reconciles a LongestProcessingTimePartition object
type LongestProcessingTimePartitionReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Partitioner longestprocessingtime.Partitioner

	lastReconciled sync.Map
}

var (
	longestProcessingTimePartitionShardsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "partition",
			Name:        "shards",
			Help:        "Shards as partitioned",
			ConstLabels: prometheus.Labels{"partition_type": "longest_processing_time"},
		},
		[]string{
			"partition_ref",
			"shard_uid",
			"shard_id",
			"shard_namespace",
			"shard_name",
			"shard_server",
			"replica_id",
		},
	)
	longestProcessingTimePartitionReplicasTotalLoadGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "partition",
			Name:        "replicas_total_load",
			Help:        "Sum of the load indexes assigned to a replica",
			ConstLabels: prometheus.Labels{"partition_type": "longest_processing_time"},
		},
		[]string{
			"partition_ref",
			"replica_id",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(
		longestProcessingTimePartitionShardsGauge,
		longestProcessingTimePartitionReplicasTotalLoadGauge,
	)
}

// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=longestprocessingtimepartitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=longestprocessingtimepartitions/status,verbs=get;update;patch
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=longestprocessingtimepartitions/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *LongestProcessingTimePartitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	partition := &autoscaler.LongestProcessingTimePartition{}
	if err := r.Get(ctx, req.NamespacedName, partition); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	loadIndexProvider, err := findByRef[common.LoadIndexProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		partition.Namespace,
		*partition.Spec.LoadIndexProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find load index provider by ref")
		meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingLoadIndexProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, partition); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when load index provider is created
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Load index provider found", "loadIndexProvider", partition.Spec.LoadIndexProviderRef)

	if !meta.IsStatusConditionPresentAndEqual(loadIndexProvider.GetLoadIndexProviderStatus().Conditions, StatusTypeReady, metav1.ConditionTrue) {
		log.V(1).Info("Load index provider not ready", "loadIndexProvider", partition.Spec.LoadIndexProviderRef)
		meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
			Type:   StatusTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "LoadIndexProviderNotReady",
			Message: fmt.Sprintf("Check the status of a load index provider %s (api=%s, kind=%s)",
				partition.Spec.LoadIndexProviderRef.Name,
				*partition.Spec.LoadIndexProviderRef.APIGroup,
				partition.Spec.LoadIndexProviderRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, partition); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when load index provider changes
		return ctrl.Result{}, nil
	}

	loadIndexes := loadIndexProvider.GetLoadIndexProviderStatus().Values
	replicas, err := r.Partitioner.Partition(ctx, loadIndexes)
	if err != nil {
		log.Error(err, "Failure during partitioning")
		meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorDuringPartitioning",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, partition); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// If it's a math problem, we can't fix it by re-queuing
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Partitioned successfully", "replicas", len(replicas))
	longestProcessingTimePartitionShardsGauge.DeletePartialMatch(prometheus.Labels{
		"partition_ref": req.NamespacedName.String(),
	})
	longestProcessingTimePartitionReplicasTotalLoadGauge.DeletePartialMatch(prometheus.Labels{
		"partition_ref": req.NamespacedName.String(),
	})
	for _, replica := range replicas {
		for _, li := range replica.LoadIndexes {
			longestProcessingTimePartitionShardsGauge.WithLabelValues(
				req.NamespacedName.String(),
				string(li.Shard.UID),
				li.Shard.ID,
				li.Shard.Namespace,
				li.Shard.Name,
				li.Shard.Server,
				replica.ID,
			).Set(1)
		}
		longestProcessingTimePartitionReplicasTotalLoadGauge.WithLabelValues(
			req.NamespacedName.String(),
			replica.ID,
		).Set(replica.TotalLoad.AsApproximateFloat64())
	}

	partition.Status.Replicas = replicas
	meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
		Type:   StatusTypeReady,
		Status: metav1.ConditionTrue,
		Reason: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, partition); err != nil {
		log.V(1).Info("Failed to update resource status", "err", err)
		return ctrl.Result{}, err
	}
	log.Info("Resource status updated", "replicas", len(replicas))

	// We don't need to do anything unless load index provider data changes
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LongestProcessingTimePartitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_longers_processing_time_partition").
		For(
			&autoscalerv1alpha1.LongestProcessingTimePartition{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		)

	for _, gvk := range getLoadIndexProviders() {
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
			handler.EnqueueRequestsFromMapFunc(r.mapLoadIndexProvider),
		)
	}

	c = c.
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		})
	return c.Complete(r)
}

func (r *LongestProcessingTimePartitionReconciler) mapLoadIndexProvider(
	ctx context.Context, object client.Object) []reconcile.Request {

	requests := []reconcile.Request{}

	gvk, _, err := r.Scheme.ObjectKinds(object)
	if err != nil || len(gvk) == 0 {
		log.Log.Error(err, "Failed to determine GVK for object")
		return requests
	}
	namespace := object.GetNamespace()

	var partitions autoscaler.LongestProcessingTimePartitionList
	if err := r.List(ctx, &partitions, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list LongestProcessingTimePartition", "Namespace", namespace)
		return requests
	}

	for _, partition := range partitions.Items {
		for _, _gvk := range gvk {
			if *partition.Spec.LoadIndexProviderRef.APIGroup == _gvk.Group &&
				partition.Spec.LoadIndexProviderRef.Kind == _gvk.Kind &&
				partition.Spec.LoadIndexProviderRef.Name == object.GetName() {
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      partition.GetName(),
						Namespace: partition.GetNamespace(),
					},
				}
				requests = append(requests, req)
				break
			}
		}
	}

	return requests
}
