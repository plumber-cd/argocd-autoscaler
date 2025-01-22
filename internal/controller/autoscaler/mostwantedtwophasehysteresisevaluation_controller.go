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
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

var (
	knownPartitionProviders      []schema.GroupVersionKind
	knownPartitionProvidersMutex sync.Mutex
)

func RegisterPartitionProvider(gvk schema.GroupVersionKind) {
	knownPartitionProvidersMutex.Lock()
	defer knownPartitionProvidersMutex.Unlock()
	knownPartitionProviders = append(knownPartitionProviders, gvk)
}

func getPartitionProviders() []schema.GroupVersionKind {
	knownPartitionProvidersMutex.Lock()
	defer knownPartitionProvidersMutex.Unlock()
	_copy := make([]schema.GroupVersionKind, len(knownPartitionProviders))
	copy(_copy, knownPartitionProviders)
	return _copy
}

func init() {
	RegisterPartitionProvider(schema.GroupVersionKind{
		Group:   "autoscaler.argoproj.io",
		Version: "v1alpha1",
		Kind:    "LongestProcessingTimePartition",
	})
}

// MostWantedTwoPhaseHysteresisEvaluationReconciler reconciles a MostWantedTwoPhaseHysteresisEvaluation object
type MostWantedTwoPhaseHysteresisEvaluationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=mostwantedtwophasehysteresisevaluations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=mostwantedtwophasehysteresisevaluations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=mostwantedtwophasehysteresisevaluations/finalizers,verbs=update
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=longestprocessingtimepartitions,verbs=get;list;watch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *MostWantedTwoPhaseHysteresisEvaluationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Received reconcile request")

	evaluation := &autoscaler.MostWantedTwoPhaseHysteresisEvaluation{}
	if err := r.Get(ctx, req.NamespacedName, evaluation); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	partitionProvider, err := findByRef[PartitionProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		evaluation.Namespace,
		evaluation.Spec.PartitionProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find partition provider by ref")
		meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingPartitionProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, evaluation); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when partition provider is created
		return ctrl.Result{}, err
	}

	if !meta.IsStatusConditionPresentAndEqual((*partitionProvider).GetConditions(), typeReady, metav1.ConditionTrue) {
		meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
			Type:   typeReady,
			Status: metav1.ConditionFalse,
			Reason: "PartitionProviderNotReady",
			Message: fmt.Sprintf("Check the status of a partition provider %s (api=%s, kind=%s)",
				evaluation.Spec.PartitionProviderRef.Name,
				*evaluation.Spec.PartitionProviderRef.APIGroup,
				evaluation.Spec.PartitionProviderRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, evaluation); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when partition provider changes
		return ctrl.Result{}, nil
	}

	replicas := (*partitionProvider).GetReplicas()
	if len(replicas) == 0 {
		err := fmt.Errorf("No replicas found")
		log.Error(err, "No replicas found, fail")
		meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "NoReplicasFound",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, evaluation); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when partition provider changes
		return ctrl.Result{}, nil
	}

	// Maintain the history
	evaluation.Status.History = append(evaluation.Status.History,
		autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{
			Timestamp: metav1.Now(),
			Replicas:  replicas,
		})
	cleanHistory := []autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{}
	for _, record := range evaluation.Status.History {
		if record.Timestamp.Add(evaluation.Spec.StabilizationPeriod.Duration).After(time.Now()) {
			cleanHistory = append(cleanHistory, record)
		}
	}
	evaluation.Status.History = cleanHistory
	if len(evaluation.Status.History) < int(evaluation.Spec.MinimumSampleSize) {
		err := fmt.Errorf("Minimum sample size not reached")
		log.Error(err, "Minimum sample size not reached, waiting for another poll...")
		meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "MinimumSampleSizeNotReached",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, evaluation); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{RequeueAfter: evaluation.Spec.PollingPeriod.Duration}, nil
	}

	historyRecords := map[string]autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{}
	historyRecordsLastSeen := map[string]metav1.Time{}
	historyRecorsSeenTimes := map[string]int{}
	for _, record := range evaluation.Status.History {
		serializedRecord := r.SerializeHistoricalRecord(&record)
		if _, ok := historyRecordsLastSeen[serializedRecord]; !ok ||
			record.Timestamp.After(historyRecordsLastSeen[serializedRecord].Time) {
			historyRecords[serializedRecord] = record
			historyRecordsLastSeen[serializedRecord] = record.Timestamp
			historyRecorsSeenTimes[serializedRecord] = 0
		}
		historyRecorsSeenTimes[serializedRecord]++
	}
	topSeenRecord := autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{}
	maxSeenCount := 0
	for serializedRecord, seenTimes := range historyRecorsSeenTimes {
		if seenTimes > maxSeenCount {
			maxSeenCount = seenTimes
			topSeenRecord = historyRecords[serializedRecord]
		} else if seenTimes == maxSeenCount &&
			historyRecords[serializedRecord].Timestamp.After(topSeenRecord.Timestamp.Time) {
			topSeenRecord = historyRecords[serializedRecord]
		}
	}

	evaluation.Status.Projection = topSeenRecord.Replicas
	if evaluation.Status.LastEvaluationTimestamp == nil ||
		time.Since(evaluation.Status.LastEvaluationTimestamp.Time) >= evaluation.Spec.StabilizationPeriod.Duration {
		evaluation.Status.Replicas = topSeenRecord.Replicas
		evaluation.Status.LastEvaluationTimestamp = ptr.To(metav1.Now())
	}

	if !meta.IsStatusConditionPresentAndEqual(evaluation.Status.Conditions, typeAvailable, metav1.ConditionTrue) {
		meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
			Type:   typeAvailable,
			Status: metav1.ConditionTrue,
			Reason: "InitialEvaluationSuccessful",
		})
	}
	meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
		Type:   typeReady,
		Status: metav1.ConditionTrue,
		Reason: "EvaluationSuccessful",
	})
	if err := r.Status().Update(ctx, evaluation); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Re-queue for the next poll
	return ctrl.Result{RequeueAfter: evaluation.Spec.PollingPeriod.Duration}, nil
}

// SerializeHistoricalRecord serializes historical record into a string.
// The format is: shardUID=replicaID,shardUID=replicaID,...
// The order of the shards is sorted.
// This is used to compare historical records.
// Regardless of the order of the replicas in the record, the serialized string will be the same.
func (r *MostWantedTwoPhaseHysteresisEvaluationReconciler) SerializeHistoricalRecord(
	record *autoscaler.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord) string {

	m := map[string]string{}
	for _, replica := range record.Replicas {
		for _, loadIndex := range replica.LoadIndexes {
			m[string(loadIndex.Shard.UID)] = replica.ID
		}
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var serializedItems []string
	for _, k := range keys {
		serializedItems = append(serializedItems, fmt.Sprintf("%s=%v", k, m[k]))
	}

	return strings.Join(serializedItems, ",")
}

// SetupWithManager sets up the controller with the Manager.
func (r *MostWantedTwoPhaseHysteresisEvaluationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_most_wanted_two_phase_hysteresis_evaluation").
		For(
			&autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		)

	for _, gvk := range getPartitionProviders() {
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
			handler.EnqueueRequestsFromMapFunc(r.mapPartitionProvider),
		)
	}

	c = c.
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		})
	return c.Complete(r)
}

func (r *MostWantedTwoPhaseHysteresisEvaluationReconciler) mapPartitionProvider(
	ctx context.Context, object client.Object) []reconcile.Request {

	requests := []reconcile.Request{}

	gvk, _, err := r.Scheme.ObjectKinds(object)
	if err != nil || len(gvk) == 0 {
		log.Log.Error(err, "Failed to determine GVK for object")
		return requests
	}
	namespace := object.GetNamespace()

	var evaluations autoscaler.MostWantedTwoPhaseHysteresisEvaluationList
	if err := r.List(ctx, &evaluations, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list MostWantedTwoPhaseHysteresisEvaluation", "Namespace", namespace)
		return requests
	}

	for _, partition := range evaluations.Items {
		for _, _gvk := range gvk {
			if *partition.Spec.PartitionProviderRef.APIGroup == _gvk.Group &&
				partition.Spec.PartitionProviderRef.Kind == _gvk.Kind &&
				partition.Spec.PartitionProviderRef.Name == object.GetName() {
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
