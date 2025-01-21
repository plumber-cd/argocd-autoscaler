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

	"github.com/argoproj/argo-cd/v2/common"

	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

// SecretTypeClusterDiscoveryReconciler reconciles a SecretTypeClusterDiscovery object
type SecretTypeClusterDiscoveryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=secrettypeclusterdiscoveries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=secrettypeclusterdiscoveries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=secrettypeclusterdiscoveries/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *SecretTypeClusterDiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Received reconcile request")

	discovery := &autoscaler.SecretTypeClusterDiscovery{}
	if err := r.Get(ctx, req.NamespacedName, discovery); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	// First, change the availability condition from unknown to false -
	// once it is set to true at the very end, we won't touch it ever again.
	if meta.IsStatusConditionPresentAndEqual(discovery.Status.Conditions, typeAvailable, metav1.ConditionUnknown) {
		meta.SetStatusCondition(&discovery.Status.Conditions, metav1.Condition{
			Type:    typeAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  "InitialDiscovery",
			Message: "First time discovery",
		})
		if err := r.Status().Update(ctx, discovery); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Find all clusters
	clusters := &corev1.SecretList{}
	labelSelector := labels.Set(client.MatchingLabels{
		common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
	}).AsSelector()
	if discovery.Spec.LabelSelector != nil {
		labelSelector = labels.Set(discovery.Spec.LabelSelector.MatchLabels).AsSelector()
	}
	if err := r.List(ctx, clusters, &client.ListOptions{
		Namespace:     discovery.Namespace,
		LabelSelector: labelSelector,
	}); err != nil {
		log.Error(err, "Failed to list clusters")
		meta.SetStatusCondition(&discovery.Status.Conditions, metav1.Condition{
			Type:    typeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "FailedToListClusters",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, discovery); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	log.V(1).Info("Found clusters", "count", len(clusters.Items))

	shards := []autoscaler.Shard{}
	for _, cluster := range clusters.Items {
		shard := autoscaler.Shard{
			ID:  cluster.Name,
			UID: cluster.GetUID(),
			Data: map[string]string{
				"name":   string(cluster.Data["name"]),
				"server": string(cluster.Data["server"]),
			},
		}
		shards = append(shards, shard)
	}

	discovery.Status.Shards = shards
	if !meta.IsStatusConditionPresentAndEqual(discovery.Status.Conditions, typeAvailable, metav1.ConditionTrue) {
		meta.SetStatusCondition(&discovery.Status.Conditions, metav1.Condition{
			Type:   typeAvailable,
			Status: metav1.ConditionTrue,
			Reason: "InitialDiscoverySuccessful",
		})
	}
	meta.SetStatusCondition(&discovery.Status.Conditions, metav1.Condition{
		Type:   typeReady,
		Status: metav1.ConditionTrue,
		Reason: "DiscoverySuccessful",
	})
	if err := r.Status().Update(ctx, discovery); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretTypeClusterDiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_secret_type_cluster_discovery").
		For(
			&autoscaler.SecretTypeClusterDiscovery{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapClusterSecret),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func (r *SecretTypeClusterDiscoveryReconciler) mapClusterSecret(
	ctx context.Context, object client.Object) []reconcile.Request {

	log := log.FromContext(ctx)

	requests := []reconcile.Request{}
	namespace := object.GetNamespace()

	var discoveries autoscaler.SecretTypeClusterDiscoveryList
	if err := r.List(ctx, &discoveries, client.InNamespace(namespace)); err != nil {
		log.Error(err, "Failed to list discoverers", "Namespace", namespace)
		return requests
	}

	for _, discovery := range discoveries.Items {
		labelSelector := labels.Set(client.MatchingLabels{
			common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
		}).AsSelector()
		if discovery.Spec.LabelSelector != nil {
			log.V(2).Info("This discoverer uses custom labelSelector", "discoveryName", discovery.Name,
				"discoveryKind", object.GetObjectKind().GroupVersionKind().Kind)
			labelSelector = labels.Set(discovery.Spec.LabelSelector.MatchLabels).AsSelector()
		}
		if labelSelector.Matches(labels.Set(object.GetLabels())) {
			log.V(2).Info("Enqueueing discovery - match by labels", "discoveryName", discovery.Name,
				"discoveryKind", object.GetObjectKind().GroupVersionKind().Kind)
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      discovery.GetName(),
					Namespace: discovery.GetNamespace(),
				},
			}
			requests = append(requests, req)
		}
	}

	return requests
}
