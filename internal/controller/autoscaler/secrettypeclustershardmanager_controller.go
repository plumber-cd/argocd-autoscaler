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
	"time"

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
}

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=secrettypeclustershardmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=secrettypeclustershardmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=secrettypeclustershardmanagers/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *SecretTypeClusterShardManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Received reconcile request")

	manager := &autoscaler.SecretTypeClusterShardManager{}
	if err := r.Get(ctx, req.NamespacedName, manager); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{RequeueAfter: time.Second}, err
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
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
		// Connection error? Let's try again
		return ctrl.Result{RequeueAfter: time.Second}, err
	}
	log.V(1).Info("Found secrets", "count", len(secrets.Items))

	replicasByUID := map[types.UID]common.Replica{}
	for _, replica := range manager.Spec.Replicas {
		for _, loadIndex := range replica.LoadIndexes {
			if _, ok := replicasByUID[loadIndex.Shard.UID]; ok {
				err := errors.New("duplicate replica found")
				log.Error(err, "Failed to list replicas")
				meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
					Type:    StatusTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "FailedToListReplicas",
					Message: err.Error(),
				})
				if err := r.Status().Update(ctx, manager); err != nil {
					log.Error(err, "Failed to update resource status")
					return ctrl.Result{RequeueAfter: time.Second}, err
				}
				// Resource is malformed, no point in retrying
				return ctrl.Result{}, nil
			}
			replicasByUID[loadIndex.Shard.UID] = replica
		}
	}

	shards := []common.Shard{}
	secretsToUpdate := []corev1.Secret{}
	for _, secret := range secrets.Items {
		shard := common.Shard{
			ID:  secret.Name,
			UID: secret.GetUID(),
			Data: map[string]string{
				"namespace": secret.Namespace,
				"name":      string(secret.Data["name"]),
				"server":    string(secret.Data["server"]),
			},
		}
		shards = append(shards, shard)

		actualReplicaBytes, actualReplicaSet := secret.Data["shard"]
		var actualReplica string
		if actualReplicaSet {
			actualReplica = string(actualReplicaBytes)
		}
		desiredReplica, desiredReplicaSet := replicasByUID[secret.GetUID()]
		if desiredReplicaSet && (!actualReplicaSet || desiredReplica.ID != actualReplica) {
			log.V(1).Info("Secret is outdated", "secret", secret.Name, "actual", actualReplica, "desired", desiredReplica.ID)
			secret.Data["shard"] = []byte(desiredReplica.ID)
			secretsToUpdate = append(secretsToUpdate, secret)
		}
	}

	for _, secret := range secretsToUpdate {
		if err := r.Update(ctx, &secret); err != nil {
			log.Error(err, "Failed to update secret", "secret", secret.Name)
			meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "FailedToUpdateSecret",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, manager); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, err
			}
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
	}

	manager.Status.Shards = shards
	manager.Status.Replicas = manager.Spec.Replicas
	meta.SetStatusCondition(&manager.Status.Conditions, metav1.Condition{
		Type:   StatusTypeReady,
		Status: metav1.ConditionTrue,
		Reason: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, manager); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{RequeueAfter: time.Second}, err
	}

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
