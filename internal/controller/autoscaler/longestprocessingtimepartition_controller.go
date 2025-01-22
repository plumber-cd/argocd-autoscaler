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
	knownLoadIndexProviders      []schema.GroupVersionKind
	knownLoadIndexProvidersMutex sync.Mutex
)

func RegisterLoadIndexProvider(gvk schema.GroupVersionKind) {
	knownLoadIndexProvidersMutex.Lock()
	defer knownLoadIndexProvidersMutex.Unlock()
	knownLoadIndexProviders = append(knownLoadIndexProviders, gvk)
}

func getLoadIndexProviders() []schema.GroupVersionKind {
	knownLoadIndexProvidersMutex.Lock()
	defer knownLoadIndexProvidersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownLoadIndexProviders))
	copy(_copy, knownLoadIndexProviders)
	return _copy
}

func init() {
	RegisterLoadIndexProvider(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "WeightedPNormLoadIndex",
	})
}

// LongestProcessingTimePartitionReconciler reconciles a LongestProcessingTimePartition object
type LongestProcessingTimePartitionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=longestprocessingtimepartitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=longestprocessingtimepartitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=longestprocessingtimepartitions/finalizers,verbs=update
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=weightedpnormloadindexes,verbs=get;list;watch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *LongestProcessingTimePartitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Received reconcile request")

	partition := &autoscaler.LongestProcessingTimePartition{}
	if err := r.Get(ctx, req.NamespacedName, partition); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	loadIndexProvider, err := findByRef[LoadIndexProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		partition.Namespace,
		partition.Spec.LoadIndexProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find load index provider by ref")
		meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingLoadIndexProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, partition); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when load index provider is created
		return ctrl.Result{}, err
	}

	if !meta.IsStatusConditionPresentAndEqual((*loadIndexProvider).GetConditions(), typeReady, metav1.ConditionTrue) {
		meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
			Type:   typeReady,
			Status: metav1.ConditionFalse,
			Reason: "LoadIndexProviderNotReady",
			Message: fmt.Sprintf("Check the status of a load index provider %s (api=%s, kind=%s)",
				partition.Spec.LoadIndexProviderRef.Name,
				*partition.Spec.LoadIndexProviderRef.APIGroup,
				partition.Spec.LoadIndexProviderRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, partition); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when load index provider changes
		return ctrl.Result{}, nil
	}

	loadIndexes := (*loadIndexProvider).GetValues()

	if len(loadIndexes) == 0 {
		err := fmt.Errorf("No load indexes found")
		log.Error(err, "No load indexes found, fail")
		meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoLoadIndexFound",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, partition); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when load index provider changes
		return ctrl.Result{}, nil
	}

	replicas, err := r.LPTPartition(ctx, loadIndexes)
	if err != nil {
		log.Error(err, "Failure during partitioning")
		meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorDuringPartitioning",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, partition); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// If it's a math problem, we can't fix it by re-queuing
		return ctrl.Result{}, nil
	}

	partition.Status.Replicas = replicas
	if !meta.IsStatusConditionPresentAndEqual(partition.Status.Conditions, typeAvailable, metav1.ConditionTrue) {
		meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
			Type:   typeAvailable,
			Status: metav1.ConditionTrue,
			Reason: "InitialPartitioningSuccessful",
		})
	}
	meta.SetStatusCondition(&partition.Status.Conditions, metav1.Condition{
		Type:   typeReady,
		Status: metav1.ConditionTrue,
		Reason: "PartitioningSuccessful",
	})
	if err := r.Status().Update(ctx, partition); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// We don't need to do anything unless load index provider data changes
	return ctrl.Result{}, nil
}

func (r *LongestProcessingTimePartitionReconciler) LPTPartition(ctx context.Context,
	shards []autoscalerv1alpha1.LoadIndex) ([]autoscaler.Replica, error) {

	log := log.FromContext(ctx)

	replicas := []autoscaler.Replica{}

	if len(shards) == 0 {
		return replicas, nil
	}

	sort.Sort(LoadIndexesDesc(shards))
	bucketSize := shards[0].Value.AsApproximateFloat64()

	replicaCount := int32(0)
	for _, shard := range shards {
		// Find the replica with the least current load.
		minLoad := float64(0)
		selectedReplicaIndex := -1

		for i, replica := range replicas {
			if selectedReplicaIndex == -1 || replica.TotalLoad.AsApproximateFloat64() < minLoad {
				if replica.TotalLoad.AsApproximateFloat64()+shard.Value.AsApproximateFloat64() < bucketSize {
					selectedReplicaIndex = i
					minLoad = replica.TotalLoad.AsApproximateFloat64()
				}
			}
		}

		if selectedReplicaIndex < 0 {
			// Create a new replica for this shard.
			replicaCount++
			newReplica := autoscaler.Replica{
				ID:                    strconv.FormatInt(int64(replicaCount-1), 32),
				LoadIndexes:           []autoscaler.LoadIndex{shard},
				TotalLoad:             shard.Value,
				TotalLoadDisplayValue: strconv.FormatFloat(shard.Value.AsApproximateFloat64(), 'f', -1, 64),
			}
			replicas = append(replicas, newReplica)
		} else {
			// Assign shard to the selected replica.
			replicas[selectedReplicaIndex].LoadIndexes = append(replicas[selectedReplicaIndex].LoadIndexes, shard)
			totalLoad := replicas[selectedReplicaIndex].TotalLoad.AsApproximateFloat64()
			totalLoad += shard.Value.AsApproximateFloat64()
			totalLoadAsString := strconv.FormatFloat(totalLoad, 'f', -1, 64)
			replicas[selectedReplicaIndex].TotalLoadDisplayValue = totalLoadAsString
			totalLoadAsResource, err := resource.ParseQuantity(totalLoadAsString)
			if err != nil {
				log.Error(err, "Failed to parse total load as resource",
					"shard", shard.Shard.ID, "totalLoad", totalLoadAsResource)
				return nil, err
			}
			replicas[selectedReplicaIndex].TotalLoad = totalLoadAsResource
		}
	}

	return replicas, nil
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
