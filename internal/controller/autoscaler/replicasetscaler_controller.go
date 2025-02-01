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
	"time"

	appsv1 "k8s.io/api/apps/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"

	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

type ReplicaSetReconcilerMode string

const (
	ReplicaSetReconcilerModeDefault        = ReplicaSetReconcilerMode("default")
	ReplicaSetReconcilerModeRolloutRestart = ReplicaSetReconcilerMode("rollout-restart")
	ReplicaSetReconcilerModeX0Y            = ReplicaSetReconcilerMode("x0y")
)

type ReplicaSetController struct {
	Kind        string
	Deployment  appsv1.Deployment
	StatefulSet appsv1.StatefulSet
}

// ReplicaSetScalerReconciler reconciles a ReplicaSetScaler object
type ReplicaSetScalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=replicasetscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=replicasetscalers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaler.argoproj.io,resources=replicasetscalers/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *ReplicaSetScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(1).Info("Received reconcile request")

	scaler := &autoscaler.ReplicaSetScaler{}
	if err := r.Get(ctx, req.NamespacedName, scaler); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	var mode ReplicaSetReconcilerMode
	if scaler.Spec.Mode.X0Y != nil {
		mode = ReplicaSetReconcilerModeX0Y
	}
	if scaler.Spec.Mode.Default != nil {
		if mode != "" {
			err := fmt.Errorf("Scaler spec is invalid - only one mode can be set")
			log.Error(err, "Validation error")
			meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "ErrorResourceMalformed",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, scaler); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// We should get a new event when spec changes
			return ctrl.Result{}, err
		}
		mode = ReplicaSetReconcilerModeDefault
	}
	if mode == "" {
		mode = ReplicaSetReconcilerModeDefault
	}

	partitionProvider, err := findByRef[common.PartitionProvider](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		scaler.Namespace,
		*scaler.Spec.PartitionProviderRef,
	)
	if err != nil {
		log.Error(err, "Failed to find partition provider by ref")
		meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingPartitionProvider",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, scaler); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when partition provider is created
		return ctrl.Result{}, err
	}

	if !meta.IsStatusConditionPresentAndEqual(partitionProvider.GetStatus().Conditions, StatusTypeReady, metav1.ConditionTrue) {
		meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
			Type:   StatusTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "PartitionProviderNotReady",
			Message: fmt.Sprintf("Check the status of partition provider %s (api=%s, kind=%s)",
				scaler.Spec.PartitionProviderRef.Name,
				*scaler.Spec.PartitionProviderRef.APIGroup,
				scaler.Spec.PartitionProviderRef.Kind,
			),
		})
		if err := r.Status().Update(ctx, scaler); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when partition provider changes
		return ctrl.Result{}, nil
	}

	shardManager, err := findByRef[common.ShardManager](
		ctx,
		r.Scheme,
		r.RESTMapper(),
		r.Client,
		scaler.Namespace,
		*scaler.Spec.ShardManagerRef,
	)
	if err != nil {
		log.Error(err, "Failed to find shard manager by ref")
		meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
			Type:    StatusTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ErrorFindingShardManager",
			Message: err.Error(),
		})
		if err := r.Status().Update(ctx, scaler); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should get a new event when shard manager is created
		return ctrl.Result{}, err
	}

	replicaSetController := ReplicaSetController{}
	switch scaler.Spec.ReplicaSetControllerRef.Kind {
	case "Deployment":
		fallthrough
	case "StatefulSet":
		replicaSetController.Kind = scaler.Spec.ReplicaSetControllerRef.Kind
	default:
		log.Error(err, "Unsupported ReplicaSetControllerRef.Kind", "kind", scaler.Spec.ReplicaSetControllerRef.Kind)
	}
	switch replicaSetController.Kind {
	case "Deployment":
		deploymentController, err := findByRef[appsv1.Deployment](
			ctx,
			r.Scheme,
			r.RESTMapper(),
			r.Client,
			scaler.Namespace,
			*scaler.Spec.PartitionProviderRef,
		)
		if err != nil {
			log.Error(err, "Failed to find Deployment by ref")
			meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "ErrorFindingDeployment",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, scaler); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// We should get a new event when deployment is created
			return ctrl.Result{}, err
		}
		replicaSetController.Deployment = deploymentController
	case "StatefulSet":
		statefulSetController, err := findByRef[appsv1.StatefulSet](
			ctx,
			r.Scheme,
			r.RESTMapper(),
			r.Client,
			scaler.Namespace,
			*scaler.Spec.PartitionProviderRef,
		)
		if err != nil {
			log.Error(err, "Failed to find StatefulSet by ref")
			meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "ErrorFindingStatefulSet",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, scaler); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// We should get a new event when statefulset is created
			return ctrl.Result{}, err
		}
		replicaSetController.StatefulSet = statefulSetController
	default:
		panic("unreachable")
	}

	// - If mode x0y - check the RS status and re-queue
	if mode == ReplicaSetReconcilerModeX0Y {
		// Check if RS is set to zero already, and if not - scale it to zero
		if r.GetRSControllerDesiredReplicas(replicaSetController) > 0 {
			if err := r.ScaleTo(ctx, replicaSetController, int32(0), false); err != nil {
				log.Error(err, "Failed to scale replica set controller to zero",
					"replicaSetControllerKind", scaler.Spec.ReplicaSetControllerRef.Kind,
					"replicaSetControllerName", scaler.Spec.ReplicaSetControllerRef.Name)
				meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
					Type:    StatusTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "ErrorScalingToZero",
					Message: err.Error(),
				})
				if err := r.Status().Update(ctx, scaler); err != nil {
					log.Error(err, "Failed to update resource status")
					return ctrl.Result{RequeueAfter: time.Second}, nil
				}
				// We should try again
				return ctrl.Result{RequeueAfter: time.Second}, err
			}
		}

		// Check if RS was already scaled to zero
		if r.GetRSControllerActualReplicas(replicaSetController) > 0 {
			meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
				Type:   StatusTypeReady,
				Status: metav1.ConditionFalse,
				Reason: "WaitingForRSControllerToScaleToZero",
			})
			if err := r.Status().Update(ctx, scaler); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// We should wait a little more
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	}

	// Check if shard manager desired state meets partition provider requirements
	if partitionProvider.GetStatus().Replicas.SerializeToString() != shardManager.GetSpec().Replicas.SerializeToString() {
		shardManager.GetSpec().Replicas = partitionProvider.GetStatus().Replicas
		if err := r.Client.Update(ctx, shardManager.GetClientObject()); err != nil {
			log.Error(err, "Failed to update shard manager desired state",
				"shardManagerKind", scaler.Spec.ShardManagerRef.Kind,
				"shardManagerName", scaler.Spec.ShardManagerRef.Name)
			meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "ErrorUpdatingShardManager",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, scaler); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// We should try again
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
	}

	// Check if shard manager actual state meets partition provider requirements
	if partitionProvider.GetStatus().Replicas.SerializeToString() != shardManager.GetStatus().Replicas.SerializeToString() {
		meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
			Type:   StatusTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "WaitingForShardManagerToApplyDesiredState",
		})
		if err := r.Status().Update(ctx, scaler); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should wait a little more
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// - If replica set condition is set - check the replica set, re-queue
	// - Apply change to replica set manager, set condition, re-queue

	// Check if RS is already scaled, and if not - apply the change
	desiredReplicas := int32(len(partitionProvider.GetStatus().Replicas))
	if r.GetRSControllerDesiredReplicas(replicaSetController) != desiredReplicas {
		restart := mode == ReplicaSetReconcilerModeDefault &&
			scaler.Spec.Mode.Default.RolloutRestart != nil &&
			*scaler.Spec.Mode.Default.RolloutRestart
		if err := r.ScaleTo(ctx, replicaSetController, desiredReplicas, restart); err != nil {
			log.Error(err, "Failed to scale replica set controller",
				"replicaSetControllerKind", scaler.Spec.ReplicaSetControllerRef.Kind,
				"replicaSetControllerName", scaler.Spec.ReplicaSetControllerRef.Name)
			meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "ErrorScaling",
				Message: err.Error(),
			})
			if err := r.Status().Update(ctx, scaler); err != nil {
				log.Error(err, "Failed to update resource status")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			// We should try again
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
	}

	// Check if RS was already scaled
	if r.GetRSControllerActualReplicas(replicaSetController) != desiredReplicas {
		meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
			Type:   StatusTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "WaitingForRSControllerToScale",
		})
		if err := r.Status().Update(ctx, scaler); err != nil {
			log.Error(err, "Failed to update resource status")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		// We should wait a little more
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
		Type:    StatusTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  StatusTypeReady,
		Message: StatusTypeReady,
	})
	if err := r.Status().Update(ctx, scaler); err != nil {
		log.Error(err, "Failed to update resource status")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ReplicaSetScalerReconciler) GetRSControllerDesiredReplicas(replicaSetController ReplicaSetController) int32 {
	switch replicaSetController.Kind {
	case "Deployment":
		return *replicaSetController.Deployment.Spec.Replicas
	case "StatefulSet":
		return *replicaSetController.StatefulSet.Spec.Replicas
	default:
		panic("unreachable")
	}
}

func (r *ReplicaSetScalerReconciler) GetRSControllerActualReplicas(replicaSetController ReplicaSetController) int32 {
	switch replicaSetController.Kind {
	case "Deployment":
		return replicaSetController.Deployment.Status.Replicas
	case "StatefulSet":
		return replicaSetController.StatefulSet.Status.Replicas
	default:
		panic("unreachable")
	}
}

func (r *ReplicaSetScalerReconciler) ScaleTo(ctx context.Context, replicaSetController ReplicaSetController, replicas int32, restart bool) error {
	var obj client.Object
	switch replicaSetController.Kind {
	case "Deployment":
		replicaSetController.Deployment.Spec.Replicas = ptr.To(replicas)
		if restart {
			replicaSetController.Deployment.Spec.Template.Annotations["autoscaler.argoproj.io/restartedAt"] = fmt.Sprintf("%d", time.Now().Unix())
		}
		obj = &replicaSetController.Deployment
	case "StatefulSet":
		replicaSetController.StatefulSet.Spec.Replicas = ptr.To(replicas)
		for containerIndex, container := range replicaSetController.StatefulSet.Spec.Template.Spec.Containers {
			// TODO: can the name of the container be different?
			// We might need to expose this as a user input in the API
			if container.Name != "argocd-application-controller" {
				continue
			}

			for envIndex, env := range container.Env {
				if env.Name != "ARGOCD_CONTROLLER_REPLICAS" {
					continue
				}

				replicaSetController.StatefulSet.Spec.Template.Spec.Containers[containerIndex].Env[envIndex].Value = fmt.Sprintf("%d", replicas)
			}
		}
		if restart {
			replicaSetController.StatefulSet.Spec.Template.Annotations["autoscaler.argoproj.io/restartedAt"] = fmt.Sprintf("%d", time.Now().Unix())
		}
		obj = &replicaSetController.StatefulSet
	default:
		panic("unreachable")
	}
	return r.Client.Update(ctx, obj)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplicaSetScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c := ctrl.NewControllerManagedBy(mgr).
		Named("argocd_autoscaler_replica_set_scaler").
		For(
			&autoscaler.ReplicaSetScaler{},
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

	for _, gvk := range getShardManagers() {
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
			handler.EnqueueRequestsFromMapFunc(r.mapShardManager),
		)
	}

	for _, gvk := range getReplicaSetControllers() {
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
			handler.EnqueueRequestsFromMapFunc(r.mapReplicaSetController),
		)
	}

	c = c.
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		})
	return c.Complete(r)
}

func (r *ReplicaSetScalerReconciler) mapPartitionProvider(
	ctx context.Context, object client.Object) []reconcile.Request {

	requests := []reconcile.Request{}

	gvk, _, err := r.Scheme.ObjectKinds(object)
	if err != nil || len(gvk) == 0 {
		log.Log.Error(err, "Failed to determine GVK for object")
		return requests
	}
	namespace := object.GetNamespace()

	var scalers autoscaler.ReplicaSetScalerList
	if err := r.List(ctx, &scalers, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list ReplicaSetScalerList", "Namespace", namespace)
		return requests
	}

	for _, scaler := range scalers.Items {
		for _, _gvk := range gvk {
			if *scaler.Spec.PartitionProviderRef.APIGroup == _gvk.Group &&
				scaler.Spec.PartitionProviderRef.Kind == _gvk.Kind &&
				scaler.Spec.PartitionProviderRef.Name == object.GetName() {
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      scaler.GetName(),
						Namespace: scaler.GetNamespace(),
					},
				}
				requests = append(requests, req)
				break
			}
		}
	}

	return requests
}

func (r *ReplicaSetScalerReconciler) mapShardManager(
	ctx context.Context, object client.Object) []reconcile.Request {

	requests := []reconcile.Request{}

	gvk, _, err := r.Scheme.ObjectKinds(object)
	if err != nil || len(gvk) == 0 {
		log.Log.Error(err, "Failed to determine GVK for object")
		return requests
	}
	namespace := object.GetNamespace()

	var scalers autoscaler.ReplicaSetScalerList
	if err := r.List(ctx, &scalers, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list ReplicaSetScalerList", "Namespace", namespace)
		return requests
	}

	for _, scaler := range scalers.Items {
		for _, _gvk := range gvk {
			if *scaler.Spec.ShardManagerRef.APIGroup == _gvk.Group &&
				scaler.Spec.ShardManagerRef.Kind == _gvk.Kind &&
				scaler.Spec.ShardManagerRef.Name == object.GetName() {
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      scaler.GetName(),
						Namespace: scaler.GetNamespace(),
					},
				}
				requests = append(requests, req)
				break
			}
		}
	}

	return requests
}

func (r *ReplicaSetScalerReconciler) mapReplicaSetController(
	ctx context.Context, object client.Object) []reconcile.Request {

	requests := []reconcile.Request{}

	gvk, _, err := r.Scheme.ObjectKinds(object)
	if err != nil || len(gvk) == 0 {
		log.Log.Error(err, "Failed to determine GVK for object")
		return requests
	}
	namespace := object.GetNamespace()

	var scalers autoscaler.ReplicaSetScalerList
	if err := r.List(ctx, &scalers, client.InNamespace(namespace)); err != nil {
		log.Log.Error(err, "Failed to list ReplicaSetScalerList", "Namespace", namespace)
		return requests
	}

	for _, scaler := range scalers.Items {
		for _, _gvk := range gvk {
			if *scaler.Spec.ReplicaSetControllerRef.APIGroup == _gvk.Group &&
				scaler.Spec.ReplicaSetControllerRef.Kind == _gvk.Kind &&
				scaler.Spec.ReplicaSetControllerRef.Name == object.GetName() {
				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      scaler.GetName(),
						Namespace: scaler.GetNamespace(),
					},
				}
				requests = append(requests, req)
				break
			}
		}
	}

	return requests
}
