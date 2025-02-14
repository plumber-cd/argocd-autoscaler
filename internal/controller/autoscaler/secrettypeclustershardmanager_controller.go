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
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argo "github.com/argoproj/argo-cd/v2/common"
	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"

	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

// SecretTypeClusterShardManagerReconciler reconciles a SecretTypeClusterShardManager object
type SecretTypeClusterShardManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	lastReconciled sync.Map
}

var (
	secretTypeClusterShardManagerDiscoveredShardsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "argocd_autoscaler",
			Subsystem:   "shard_manager",
			Name:        "discovered_shards",
			Help:        "Shards that are discovered by the shard manager",
			ConstLabels: prometheus.Labels{"shard_manager_type": "secret_type_cluster"},
		},
		[]string{
			"shard_manager_ref",
			"shard_uid",
			"shard_id",
			"shard_namespace",
			"shard_name",
			"shard_server",
			"replica_id",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(secretTypeClusterShardManagerDiscoveredShardsGauge)
}

// +kubebuilder:rbac:namespace=argocd-autoscaler,groups="",resources=secrets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=secrettypeclustershardmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=secrettypeclustershardmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:namespace=argocd-autoscaler,groups=autoscaler.argoproj.io,resources=secrettypeclustershardmanagers/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *SecretTypeClusterShardManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	manager := &autoscaler.SecretTypeClusterShardManager{}
	if err := r.Get(ctx, req.NamespacedName, manager); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	// Find all clusters
	secrets := &corev1.SecretList{}
	labelSelector := labels.Set(client.MatchingLabels{
		argo.LabelKeySecretType: argo.LabelValueSecretTypeCluster,
	}).AsSelector()
	if manager.Spec.LabelSelector != nil {
		labelSelector = labels.Set(manager.Spec.LabelSelector.MatchLabels).AsSelector()
	}
	if err := r.List(ctx, secrets, &client.ListOptions{
		Namespace:     manager.Namespace,
		LabelSelector: labelSelector,
	}); err != nil {
		log.Error(err, "Failed to list secrets")
		meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "FailedToListSecrets",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, manager); err != nil {
			log.V(1).Error(err, "Failed to update resource status")
			return ctrl.Result{}, err
		}
		// Connection error? Let's try again
		return ctrl.Result{}, err
	}
	log.V(1).Info("Found secrets", "count", len(secrets.Items))
	secretTypeClusterShardManagerDiscoveredShardsGauge.DeletePartialMatch(prometheus.Labels{
		"shard_manager_ref": req.NamespacedName.String(),
	})
	for _, secret := range secrets.Items {
		log.V(2).Info("Secret found", "secret", secret.Name)
		secretTypeClusterShardManagerDiscoveredShardsGauge.WithLabelValues(
			req.NamespacedName.String(),
			string(secret.GetUID()),
			secret.Name,
			secret.Namespace,
			string(secret.Data["name"]),
			string(secret.Data["server"]),
			string(secret.Data["shard"]),
		).Set(1)
	}

	replicasByUID := map[types.UID]common.Replica{}
	for _, replica := range manager.Spec.Replicas {
		log.V(2).Info("Reading desired replica", "replica", replica.ID)
		for _, loadIndex := range replica.LoadIndexes {
			log.V(2).Info("Reading desired replica shard", "replica", replica.ID, "shard", loadIndex.Shard.ID)
			if _, ok := replicasByUID[loadIndex.Shard.UID]; ok {
				err := errors.New("duplicate replica found")
				log.Error(err, "Failed to read desired replica shard",
					"replica", replica.ID, "shard", loadIndex.Shard.ID, "shardUID", loadIndex.Shard.UID)
				meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
					Type:    StatusTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "FailedToReadDesiredReplicaShard",
					Message: err.Error(),
				})
				if err := r.Status().Update(ctx, manager); err != nil {
					log.V(1).Info("Failed to update resource status", "err", err)
					return ctrl.Result{}, err
				}
				// Resource is malformed, no point in retrying
				return ctrl.Result{}, nil
			}
			replicasByUID[loadIndex.Shard.UID] = replica
		}
	}
	log.V(1).Info("Desired replicas read", "count", len(replicasByUID))

	shards := []common.Shard{}
	secretsToUpdate := []corev1.Secret{}
	for _, secret := range secrets.Items {
		shard := common.Shard{
			ID:        secret.Name,
			UID:       secret.GetUID(),
			Namespace: secret.Namespace,
			Name:      string(secret.Data["name"]),
			Server:    string(secret.Data["server"]),
		}
		log.V(2).Info("Reading shard", "shard", shard.ID)
		shards = append(shards, shard)

		actualReplicaBytes, actualReplicaSet := secret.Data["shard"]
		var actualReplica int
		if actualReplicaSet {
			_actualReplica, err := strconv.Atoi(string(actualReplicaBytes))
			if err != nil {
				log.Error(err, "Failed to read actual shard from the secret",
					"shard", string(actualReplicaBytes), "secret", secret.Name)
				meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
					Type:    StatusTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "FailedToReadShardFromSecret",
					Message: err.Error(),
				})
				if err := r.Status().Update(ctx, manager); err != nil {
					log.V(1).Info("Failed to update resource status", "err", err)
					return ctrl.Result{}, err
				}
				// Resource is malformed, no point in retrying
				return ctrl.Result{}, nil
			}
			actualReplica = _actualReplica
		}
		desiredReplica, desiredReplicaSet := replicasByUID[secret.GetUID()]
		if !desiredReplicaSet {
			log.V(2).Info("Shard had desired replica", "shard", shard.ID, "desired", desiredReplica.ID)
		}
		if desiredReplicaSet && (!actualReplicaSet || desiredReplica.ID != int32(actualReplica)) {
			log.V(1).Info("Secret is outdated", "secret", secret.Name, "actual", actualReplica, "desired", desiredReplica.ID)
			secret.Data["shard"] = []byte(fmt.Sprintf("%d", desiredReplica.ID))
			secretsToUpdate = append(secretsToUpdate, secret)
		}
	}
	log.V(1).Info("Shards read", "count", len(shards))

	for _, secret := range secretsToUpdate {
		log.V(2).Info("Updating secret",
			"secret", secret.Name, "namespace", secret.Namespace, "data", string(secret.Data["shard"]))
		if err := r.Update(ctx, &secret); err != nil {
			log.Error(err, "Failed to update secret", "secret", secret.Name)
			meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "FailedToUpdateSecret",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, manager); err != nil {
				log.V(1).Info("Failed to update resource status", "err", err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, err
		}
		log.Info("Secret updated",
			"secret", secret.Name, "namespace", secret.Namespace, "data", string(secret.Data["shard"]))
	}
	log.V(1).Info("Secrets updated", "count", len(secretsToUpdate))

	manager.Status.Shards = shards
	manager.Status.Replicas = manager.Spec.Replicas
	meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
		Type:   StatusTypeReady,
		Status: metav1.ConditionTrue,
		Reason: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, manager); err != nil {
		log.V(1).Info("Failed to update resource status", "err", err)
		return ctrl.Result{}, err
	}
	log.Info("Resource status updated", "shards", len(shards), "replicas", len(manager.Spec.Replicas))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretTypeClusterShardManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_secret_type_cluster_shard_manager").
		For(
			&autoscaler.SecretTypeClusterShardManager{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapClusterSecrets),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func (r *SecretTypeClusterShardManagerReconciler) mapClusterSecrets(
	ctx context.Context, object client.Object) []reconcile.Request {

	log := log.FromContext(ctx)

	requests := []reconcile.Request{}
	namespace := object.GetNamespace()

	var managers autoscaler.SecretTypeClusterShardManagerList
	if err := r.List(ctx, &managers, client.InNamespace(namespace)); err != nil {
		log.Error(err, "Failed to list Shard Managers", "Namespace", namespace)
		return requests
	}

	for _, manager := range managers.Items {
		labelSelector := labels.Set(client.MatchingLabels{
			argo.LabelKeySecretType: argo.LabelValueSecretTypeCluster,
		}).AsSelector()
		if manager.Spec.LabelSelector != nil {
			log.V(2).Info("This shard manager uses custom labelSelector", "shardManager", manager.Name,
				"shardManagerKind", object.GetObjectKind().GroupVersionKind().Kind)
			labelSelector = labels.Set(manager.Spec.LabelSelector.MatchLabels).AsSelector()
		}
		if labelSelector.Matches(labels.Set(object.GetLabels())) {
			log.V(2).Info("Enqueueing shard manager - match by labels", "shardManager", manager.Name,
				"shardManagerKind", object.GetObjectKind().GroupVersionKind().Kind)
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      manager.GetName(),
					Namespace: manager.GetNamespace(),
				},
			}
			requests = append(requests, req)
		}
	}

	return requests
}
