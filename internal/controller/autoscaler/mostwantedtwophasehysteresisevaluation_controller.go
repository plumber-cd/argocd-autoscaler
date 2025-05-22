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
	"strconv"
	"sync"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
)

// MostWantedTwoPhaseHysteresisEvaluationReconciler reconciles a MostWantedTwoPhaseHysteresisEvaluation object
type MostWantedTwoPhaseHysteresisEvaluationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	lastReconciled sync.Map
}

var (
	mostWantedTwoPhaseHysteresisEvaluationProjectedShardsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "evaluation",
			Name:        "projected_shards",
			Help:        "Projected shards partitioning",
			ConstLabels: prometheus.Labels{"evaluation_type": "most_wanted_two_phase_hysteresis"},
		},
		[]string{
			"evaluation_ref",
			"shard_uid",
			"shard_id",
			"shard_namespace",
			"shard_name",
			"shard_server",
			"replica_id",
		},
	)
	mostWantedTwoPhaseHysteresisEvaluationProjectedReplicasTotalLoadGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "evaluation",
			Name:        "projected_replicas_total_load",
			Help:        "Projected sum of the load indexes assigned to a replica",
			ConstLabels: prometheus.Labels{"evaluation_type": "most_wanted_two_phase_hysteresis"},
		},
		[]string{
			"evaluation_ref",
			"replica_id",
		},
	)
	mostWantedTwoPhaseHysteresisEvaluationShardsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "evaluation",
			Name:        "shards",
			Help:        "Shards partitioning",
			ConstLabels: prometheus.Labels{"evaluation_type": "most_wanted_two_phase_hysteresis"},
		},
		[]string{
			"evaluation_ref",
			"shard_uid",
			"shard_id",
			"shard_namespace",
			"shard_name",
			"shard_server",
			"replica_id",
		},
	)
	mostWantedTwoPhaseHysteresisEvaluationReplicasTotalLoadGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "evaluation",
			Name:        "replicas_total_load",
			Help:        "Sum of the load indexes assigned to a replica",
			ConstLabels: prometheus.Labels{"evaluation_type": "most_wanted_two_phase_hysteresis"},
		},
		[]string{
			"evaluation_ref",
			"replica_id",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(
		mostWantedTwoPhaseHysteresisEvaluationProjectedShardsGauge,
		mostWantedTwoPhaseHysteresisEvaluationProjectedReplicasTotalLoadGauge,
		mostWantedTwoPhaseHysteresisEvaluationShardsGauge,
		mostWantedTwoPhaseHysteresisEvaluationReplicasTotalLoadGauge,
	)
}

// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=mostwantedtwophasehysteresisevaluations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=mostwantedtwophasehysteresisevaluations/status,verbs=get;update;patch
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=mostwantedtwophasehysteresisevaluations/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
//
//nolint:gocyclo
func (r *MostWantedTwoPhaseHysteresisEvaluationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	evaluation := &autoscaler.MostWantedTwoPhaseHysteresisEvaluation{}
	if err := r.Get(ctx, req.NamespacedName, evaluation); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	partitionProvider, err := findByRef[common.PartitionProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		evaluation.Namespace,
		*evaluation.Spec.PartitionProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find partition provider by ref")
		meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingPartitionProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, evaluation); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when partition provider is created
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Partition provider found", "partitionProviderRef", evaluation.Spec.PartitionProviderRef)

	if !meta.IsStatusConditionPresentAndEqual(partitionProvider.GetPartitionProviderStatus().Conditions, StatusTypeReady, metav1.ConditionTrue) {
		log.V(1).Info("Partition provider not ready", "partitionProviderRef", evaluation.Spec.PartitionProviderRef)
		meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
			Type:   StatusTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "PartitionProviderNotReady",
			Message: fmt.Sprintf("Check the status of a partition provider %s (api=%s, kind=%s)",
				evaluation.Spec.PartitionProviderRef.Name,
				*evaluation.Spec.PartitionProviderRef.APIGroup,
				evaluation.Spec.PartitionProviderRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, evaluation); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// We should get a new event when partition provider changes
		return ctrl.Result{}, nil
	}

	currentDesiredPartition := partitionProvider.GetPartitionProviderStatus().Replicas
	log.V(2).Info("Currently reported replicas by partition provider", "count", len(currentDesiredPartition))

	// First, we check if current desired partition was ever seen before.
	// If we find a record of it - we bump the counter and refresh last seen timestamp.
	{
		seenBefore := false
		for i, record := range evaluation.Status.History {
			if record.ReplicasHash == Sha256(currentDesiredPartition.SerializeToString()) {
				seenBefore = true
				evaluation.Status.History[i].SeenTimes++
				evaluation.Status.History[i].Timestamp = metav1.Now()
				break
			}
		}

		// If currently desired partition was never seen before, we create a new history record for it.
		if !seenBefore {
			evaluation.Status.History = append(evaluation.Status.History,
				autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{
					Timestamp:    metav1.Now(),
					ReplicasHash: Sha256(currentDesiredPartition.SerializeToString()),
					SeenTimes:    1,
				},
			)
		}
	}

	// Maintaining the history records:
	// Remove all records that last seen outside of the stabilization period.
	{
		cleanHistory := []autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{}
		for _, record := range evaluation.Status.History {
			if record.Timestamp.Add(evaluation.Spec.StabilizationPeriod.Duration).After(time.Now()) {
				cleanHistory = append(cleanHistory, record)
			}
		}
		historyWasTrimmed := len(cleanHistory) != len(evaluation.Status.History)
		if historyWasTrimmed {
			log.V(1).Info("Trim the history", "from", len(evaluation.Status.History), "to", len(cleanHistory))
			evaluation.Status.History = cleanHistory
		}
	}

	// Noticing each history record
	historyRecordsByHash := map[string]autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{}
	historyRecordsByHashLastSeen := map[string]metav1.Time{}
	historyRecorsByHashSeenTimes := map[string]int32{}
	for _, record := range evaluation.Status.History {
		hash := record.ReplicasHash
		log.V(2).Info("Noticing record", "record", hash)
		if _, ok := historyRecordsByHashLastSeen[hash]; !ok {
			historyRecordsByHash[hash] = record
			historyRecordsByHashLastSeen[hash] = record.Timestamp
			historyRecorsByHashSeenTimes[hash] = record.SeenTimes
		} else if record.Timestamp.After(historyRecordsByHashLastSeen[hash].Time) {
			historyRecordsByHashLastSeen[hash] = record.Timestamp
			historyRecorsByHashSeenTimes[hash] += record.SeenTimes
		} else {
			historyRecorsByHashSeenTimes[hash] += record.SeenTimes
		}
	}

	// Determining the top seen record
	// Tie breaker is the last seen record
	topSeenRecord := autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{}
	maxSeenCount := int32(0)
	for hash, seenTimes := range historyRecorsByHashSeenTimes {
		log.V(2).Info("Evaluating records", "record", hash, "seenTimes", seenTimes)
		if seenTimes > maxSeenCount {
			maxSeenCount = seenTimes
			topSeenRecord = historyRecordsByHash[hash]
		} else if seenTimes == maxSeenCount &&
			historyRecordsByHash[hash].Timestamp.After(topSeenRecord.Timestamp.Time) {
			log.V(2).Info("Tie breaker", "left", topSeenRecord.ReplicasHash, "right", hash)
			topSeenRecord = historyRecordsByHash[hash]
		}
	}

	// If current desired partition is determined to be top seen partition, update current projection to reflect that.
	// Current projection is used to track what the partition is on the off chance it wins the election later.
	log.V(2).Info("Top seen record", "record", topSeenRecord.ReplicasHash)
	if topSeenRecord.ReplicasHash == Sha256(currentDesiredPartition.SerializeToString()) {
		log.V(1).Info("A new election projection updated", "record", topSeenRecord.ReplicasHash)
		evaluation.Status.Projection = currentDesiredPartition
	}

	// At any moment we only know what is the currently applied partition and projected winner.
	// We do not store every partition details, because that blows up etcd limits.
	// This should be all we need - currently elected partition is the top seen one by the status quote,
	// until some other variant becomes the new projected leader.
	// If current top seen record somehow is neither, this is a bug. We can't do anything about it.
	// In which case - we bail out until some other partition wins the race and become known as projected leader.
	if topSeenRecord.ReplicasHash != Sha256(evaluation.Status.Projection.SerializeToString()) {
		err := fmt.Errorf("top seen record is neither current projection nor current partition")
		log.Error(err, "Failed to evaluate")
		meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "AwaitingNewProjection",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, evaluation); err != nil {
			log.V(1).Info("Failed to update resource status", "err", err)
			return ctrl.Result{}, err
		}
		// In this case re-queuing will change nothing
		return ctrl.Result{}, nil
	}

	// Prometheus records
	{
		mostWantedTwoPhaseHysteresisEvaluationProjectedShardsGauge.DeletePartialMatch(prometheus.Labels{
			"evaluation_ref": req.NamespacedName.String(),
		})
		mostWantedTwoPhaseHysteresisEvaluationProjectedReplicasTotalLoadGauge.DeletePartialMatch(prometheus.Labels{
			"evaluation_ref": req.NamespacedName.String(),
		})
		for _, replica := range evaluation.Status.Projection {
			for _, li := range replica.LoadIndexes {
				mostWantedTwoPhaseHysteresisEvaluationProjectedShardsGauge.WithLabelValues(
					req.NamespacedName.String(),
					string(li.Shard.UID),
					li.Shard.ID,
					li.Shard.Namespace,
					li.Shard.Name,
					li.Shard.Server,
					strconv.Itoa(int(replica.ID)),
				).Set(1)
			}
			mostWantedTwoPhaseHysteresisEvaluationProjectedReplicasTotalLoadGauge.WithLabelValues(
				req.NamespacedName.String(),
				strconv.Itoa(int(replica.ID)),
			).Set(replica.TotalLoad.AsApproximateFloat64())
		}
	}

	// If last evaluation already expired (or this is a first evaluation) - apply current projection as the new leader.
	// Also - wipe the history to prevent past counters from influencing future elections.
	if evaluation.Status.LastEvaluationTimestamp == nil ||
		time.Since(evaluation.Status.LastEvaluationTimestamp.Time) >= evaluation.Spec.StabilizationPeriod.Duration {
		evaluation.Status.Replicas = evaluation.Status.Projection
		evaluation.Status.LastEvaluationTimestamp = ptr.To(metav1.Now())
		evaluation.Status.History = []autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord{}
		log.Info("New partitioning has won",
			"replicas", len(evaluation.Status.Replicas), "lastEvaluationTimestamp", evaluation.Status.LastEvaluationTimestamp)
	}

	// Prometheus records
	{
		mostWantedTwoPhaseHysteresisEvaluationShardsGauge.DeletePartialMatch(prometheus.Labels{
			"evaluation_ref": req.NamespacedName.String(),
		})
		mostWantedTwoPhaseHysteresisEvaluationReplicasTotalLoadGauge.DeletePartialMatch(prometheus.Labels{
			"evaluation_ref": req.NamespacedName.String(),
		})
		for _, replica := range evaluation.Status.Replicas {
			for _, li := range replica.LoadIndexes {
				mostWantedTwoPhaseHysteresisEvaluationShardsGauge.WithLabelValues(
					req.NamespacedName.String(),
					string(li.Shard.UID),
					li.Shard.ID,
					li.Shard.Namespace,
					li.Shard.Name,
					li.Shard.Server,
					strconv.Itoa(int(replica.ID)),
				).Set(1)
			}
			mostWantedTwoPhaseHysteresisEvaluationReplicasTotalLoadGauge.WithLabelValues(
				req.NamespacedName.String(),
				strconv.Itoa(int(replica.ID)),
			).Set(replica.TotalLoad.AsApproximateFloat64())
		}
	}

	meta.SetStatusCondition(&evaluation.Status.Conditions, metav1.Condition{
		Type:   StatusTypeReady,
		Status: metav1.ConditionTrue,
		Reason: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, evaluation); err != nil {
		log.V(1).Info("Failed to update resource status", "err", err)
		return ctrl.Result{}, err
	}
	log.Info("Resource status updated", "projection", len(evaluation.Status.Projection))

	// We should get a reconciliation request on poller changes
	return ctrl.Result{}, nil
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
