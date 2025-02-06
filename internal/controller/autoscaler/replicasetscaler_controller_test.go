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

// import (
// 	"context"
// 	"errors"
// 	"time"
//
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// 	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
// 	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/reconcile"
//
// 	appsv1 "k8s.io/api/apps/v1"
// 	corev1 "k8s.io/api/core/v1"
// 	"k8s.io/apimachinery/pkg/api/meta"
// 	"k8s.io/apimachinery/pkg/api/resource"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/utils/ptr"
// )
//
// var _ = Describe("ReplicaSetScaler Controller", func() {
// 	Context("When reconciling a resource", func() {
// 		ctx := context.Background()
//
// 		var sampleNamespace *objectContainer[*corev1.Namespace]
//
// 		var samplePartition *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]
// 		var samplePartitionNotReady *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]
// 		var samplePartitionReady *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]
//
// 		var sampleShardManager *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
// 		var sampleShardManagerNotReady *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
// 		var sampleShardManagerReady *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
//
// 		var sampleReplicaSetController *objectContainer[*appsv1.StatefulSet]
//
// 		var sampleScaler *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]
// 		var sampleScalerMalformedWithMultipleModes *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]
// 		var sampleScalerUsingNotReadyPartitionProvider *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]
// 		var sampleScalerUsingNotReadyShardManager *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]
// 		var sampleScalerMalformedWithUnsupportedReplicaSetController *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]
// 		var sampleScalerReady *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]
//
// 		BeforeEach(func() {
// 			By("Creating resources")
//
// 			sampleNamespace = newNamespaceWithRandomName()
// 			Expect(k8sClient.create(ctx, sampleNamespace.Generic())).To(Succeed())
// 			Expect(k8sClient.get(ctx, sampleNamespace.Generic())).To(Succeed())
//
// 			samplePartition = NewObjectContainer(
// 				&autoscalerv1alpha1.LongestProcessingTimePartition{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-partition",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: autoscalerv1alpha1.LongestProcessingTimePartitionSpec{
// 						PartitionerSpec: common.PartitionerSpec{
// 							LoadIndexProviderRef: &corev1.TypedLocalObjectReference{
// 								Kind: "N/A",
// 								Name: "N/A",
// 							},
// 						},
// 					},
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			samplePartitionNotReady = NewObjectContainer(
// 				&autoscalerv1alpha1.LongestProcessingTimePartition{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-partition-not-ready",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *samplePartition.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 					meta.SetStatusCondition(
// 						&container.Object.Status.Conditions,
// 						metav1.Condition{
// 							Type:    StatusTypeReady,
// 							Status:  metav1.ConditionFalse,
// 							Reason:  "NotReadyForTest",
// 							Message: "Purposefully not ready for test",
// 						},
// 					)
// 					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			samplePartitionReady = NewObjectContainer(
// 				&autoscalerv1alpha1.LongestProcessingTimePartition{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-partition-ready",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *samplePartition.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 					meta.SetStatusCondition(
// 						&container.Object.Status.Conditions,
// 						metav1.Condition{
// 							Type:   StatusTypeReady,
// 							Status: metav1.ConditionTrue,
// 							Reason: StatusTypeReady,
// 						},
// 					)
// 					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleShardManager = NewObjectContainer(
// 				&autoscalerv1alpha1.SecretTypeClusterShardManager{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-shard-manager",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleShardManagerNotReady = NewObjectContainer(
// 				&autoscalerv1alpha1.SecretTypeClusterShardManager{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-shard-manager-not-ready",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *sampleShardManager.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 					meta.SetStatusCondition(
// 						&container.Object.Status.Conditions,
// 						metav1.Condition{
// 							Type:    StatusTypeReady,
// 							Status:  metav1.ConditionFalse,
// 							Reason:  "NotReadyForTest",
// 							Message: "Purposefully not ready for test",
// 						},
// 					)
// 					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleShardManagerReady = NewObjectContainer(
// 				&autoscalerv1alpha1.SecretTypeClusterShardManager{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-shard-manager-ready",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *sampleShardManager.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 					meta.SetStatusCondition(
// 						&container.Object.Status.Conditions,
// 						metav1.Condition{
// 							Type:   StatusTypeReady,
// 							Status: metav1.ConditionTrue,
// 							Reason: StatusTypeReady,
// 						},
// 					)
// 					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleReplicaSetController = NewObjectContainer(
// 				&appsv1.StatefulSet{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-sts",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: appsv1.StatefulSetSpec{
// 						Replicas: ptr.To(int32(1)),
// 						Selector: &metav1.LabelSelector{
// 							MatchLabels: map[string]string{
// 								"app": "sample-sts",
// 							},
// 						},
// 						Template: corev1.PodTemplateSpec{
// 							ObjectMeta: metav1.ObjectMeta{
// 								Labels: map[string]string{
// 									"app": "sample-sts",
// 								},
// 							},
// 							Spec: corev1.PodSpec{
// 								Containers: []corev1.Container{
// 									{
// 										Name: "argocd-application-controller",
// 										Env: []corev1.EnvVar{
// 											{
// 												Name:  "ARGOCD_CONTROLLER_REPLICAS",
// 												Value: "1",
// 											},
// 										},
// 									},
// 								},
// 							},
// 						},
// 					},
// 				},
// 				func(container *objectContainer[*appsv1.StatefulSet]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleScaler = NewObjectContainer(
// 				&autoscalerv1alpha1.ReplicaSetScaler{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-scaler",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: autoscalerv1alpha1.ReplicaSetScalerSpec{
// 						ScalerSpec: common.ScalerSpec{
// 							PartitionProviderRef: &corev1.TypedLocalObjectReference{
// 								APIGroup: ptr.To("autoscaler.argoproj.io"),
// 								Kind:     "LongestProcessingTimePartition",
// 								Name:     samplePartition.Object.Name,
// 							},
// 							ShardManagerRef: &corev1.TypedLocalObjectReference{
// 								APIGroup: ptr.To("autoscaler.argoproj.io"),
// 								Kind:     "SecretTypeClusterShardManager",
// 								Name:     sampleShardManager.Object.Name,
// 							},
// 							ReplicaSetControllerRef: &corev1.TypedLocalObjectReference{
// 								APIGroup: ptr.To("apps"),
// 								Kind:     "StatefulSet",
// 								Name:     sampleReplicaSetController.Object.Name,
// 							},
// 						},
// 						Mode: &autoscalerv1alpha1.ReplicaSetScalerSpecModes{
// 							Default: &autoscalerv1alpha1.ReplicaSetScalerSpecModeDefault{},
// 						},
// 					},
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleScalerMalformedWithMultipleModes = NewObjectContainer(
// 				&autoscalerv1alpha1.ReplicaSetScaler{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-scaler-malformed-with-multiple-modes",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *sampleScaler.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]) {
// 					container.Object.Spec.Mode.X0Y = &autoscalerv1alpha1.ReplicaSetScalerSpecModeX0Y{}
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleScalerUsingNotReadyPartitionProvider = NewObjectContainer(
// 				&autoscalerv1alpha1.ReplicaSetScaler{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-scaler-using-not-ready-partition-provider",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *sampleScaler.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]) {
// 					container.Object.Spec.PartitionProviderRef.Name = samplePartitionNotReady.Object.Name
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleScalerUsingNotReadyShardManager = NewObjectContainer(
// 				&autoscalerv1alpha1.ReplicaSetScaler{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-scaler-using-not-ready-shard-manager",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *sampleScaler.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]) {
// 					container.Object.Spec.PartitionProviderRef.Name = samplePartitionReady.Object.Name
// 					container.Object.Spec.ShardManagerRef.Name = sampleShardManagerNotReady.Object.Name
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleScalerMalformedWithUnsupportedReplicaSetController = NewObjectContainer(
// 				&autoscalerv1alpha1.ReplicaSetScaler{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-scaler-malformed-using-unsupported-replica-set-controller",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *sampleScaler.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]) {
// 					container.Object.Spec.PartitionProviderRef.Name = samplePartitionReady.Object.Name
// 					container.Object.Spec.ShardManagerRef.Name = sampleShardManagerReady.Object.Name
// 					container.Object.Spec.ReplicaSetControllerRef.Kind = "Deployment"
// 					container.Object.Spec.ReplicaSetControllerRef.Name = "does-not-matter"
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleScalerReady = NewObjectContainer(
// 				&autoscalerv1alpha1.ReplicaSetScaler{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-scaler-ready",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: *sampleScaler.Object.Spec.DeepCopy(),
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler]) {
// 					container.Object.Spec.PartitionProviderRef.Name = samplePartitionReady.Object.Name
// 					container.Object.Spec.ShardManagerRef.Name = sampleShardManagerReady.Object.Name
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
// 		})
//
// 		AfterEach(func() {
// 			err := k8sClient.get(ctx, sampleNamespace.Generic())
// 			Expect(err).NotTo(HaveOccurred())
//
// 			By("Cleanup the test namespace")
// 			Expect(k8sClient.delete(ctx, sampleNamespace.Generic())).To(Succeed())
// 			sampleNamespace = nil
// 		})
//
// 		It("should successfully exit when the resource didn't exist", func() {
// 			By("Reconciling non existing resource")
// 			CheckExitingOnNonExistingResource(func() *ReplicaSetScalerReconciler {
// 				return &ReplicaSetScalerReconciler{
// 					Client: k8sClient,
// 					Scheme: k8sClient.Scheme(),
// 				}
// 			})
// 		})
//
// 		It("should handle error getting resource", func() {
// 			NewScenario(
// 				ctx,
// 				sampleScaler,
// 				func(fClient client.Client) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			).WithCheck(
// 				"Failing to get resource",
// 				ClientThatFailsToGetResource(nil),
// 				CheckFailureToGetResource,
// 			).Check()
// 		})
//
// 		It("should fail on malformed duplicate modes", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleScalerMalformedWithMultipleModes
//
// 			controllerReconciler := &ReplicaSetScalerReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}
//
// 			By("Reconciling with the expectation to fails to update the status")
// 			CheckFailureToUpdateStatus(
// 				k8sClient,
// 				sampleScalerMalformedWithMultipleModes,
// 				func(fClient *fakeClient) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("ErrorResourceMalformedDuplicateModes"))
// 			Expect(readyCondition.Message).To(ContainSubstring("Scaler spec is invalid - only one mode can be set"))
// 		})
//
// 		It("should handle errors when partition provider lookup fails", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleScaler
//
// 			fClient := &fakeClient{
// 				Client: k8sClient,
// 			}
// 			fClient.
// 				WithGetFunction(samplePartition.Generic(),
// 					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
// 						return errors.New("fake error getting partition provider")
// 					},
// 				)
//
// 			controllerReconciler := &ReplicaSetScalerReconciler{
// 				Client: fClient,
// 				Scheme: fClient.Scheme(),
// 			}
//
// 			By("Reconciling with the expectation to fails to update the status")
// 			CheckFailureToUpdateStatus(
// 				fClient,
// 				sampleScaler,
// 				func(fClient *fakeClient) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("ErrorFindingPartitionProvider"))
// 			Expect(readyCondition.Message).To(ContainSubstring("fake error getting partition provider"))
// 		})
//
// 		It("should exit if partition provider is not ready", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleScalerUsingNotReadyPartitionProvider
//
// 			controllerReconciler := &ReplicaSetScalerReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}
//
// 			By("Reconciling resource with expectation to fail updating status")
// 			CheckFailureToUpdateStatus(
// 				k8sClient,
// 				container,
// 				func(fClient *fakeClient) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling resource")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("PartitionProviderNotReady"))
// 			Expect(readyCondition.Message).To(ContainSubstring("Check the status of a partition provider"))
// 		})
//
// 		It("should handle errors when shard manager lookup fails", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleScalerUsingNotReadyShardManager
//
// 			fClient := &fakeClient{
// 				Client: k8sClient,
// 			}
// 			fClient.
// 				WithGetFunction(sampleShardManagerNotReady.Generic(),
// 					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
// 						return errors.New("fake error getting shard manager")
// 					},
// 				)
//
// 			controllerReconciler := &ReplicaSetScalerReconciler{
// 				Client: fClient,
// 				Scheme: fClient.Scheme(),
// 			}
//
// 			By("Reconciling with the expectation to fails to update the status")
// 			CheckFailureToUpdateStatus(
// 				fClient,
// 				sampleScaler,
// 				func(fClient *fakeClient) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("ErrorFindingShardManager"))
// 			Expect(readyCondition.Message).To(ContainSubstring("fake error getting shard manager"))
// 		})
//
// 		It("should exit if shard manager is not ready", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleScalerUsingNotReadyShardManager
//
// 			controllerReconciler := &ReplicaSetScalerReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}
//
// 			By("Reconciling resource with expectation to fail updating status")
// 			CheckFailureToUpdateStatus(
// 				k8sClient,
// 				container,
// 				func(fClient *fakeClient) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling resource")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("ShardManagerNotReady"))
// 			Expect(readyCondition.Message).To(ContainSubstring("Check the status of a shard manager"))
// 		})
//
// 		It("should handle errors on unsupported replica set controller", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleScalerMalformedWithUnsupportedReplicaSetController
//
// 			controllerReconciler := &ReplicaSetScalerReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}
//
// 			By("Reconciling with the expectation to fails to update the status")
// 			CheckFailureToUpdateStatus(
// 				k8sClient,
// 				sampleScalerMalformedWithUnsupportedReplicaSetController,
// 				func(fClient *fakeClient) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("UnsupportedReplicaSetControllerKind"))
// 			Expect(readyCondition.Message).To(ContainSubstring("Check the ref for a replica set controller"))
// 		})
//
// 		It("should handle errors when sts lookup fails", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleScalerReady
//
// 			fClient := &fakeClient{
// 				Client: k8sClient,
// 			}
// 			fClient.
// 				WithGetFunction(sampleReplicaSetController.Generic(),
// 					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
// 						return errors.New("fake error getting sts")
// 					},
// 				)
//
// 			controllerReconciler := &ReplicaSetScalerReconciler{
// 				Client: fClient,
// 				Scheme: fClient.Scheme(),
// 			}
//
// 			By("Reconciling with the expectation to fails to update the status")
// 			CheckFailureToUpdateStatus(
// 				fClient,
// 				sampleScaler,
// 				func(fClient *fakeClient) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("ErrorFindingStatefulSet"))
// 			Expect(readyCondition.Message).To(ContainSubstring("fake error getting sts"))
// 		})
//
// 		It("should reconcile in default mode", func() {
// 			By("Preparing")
//
// 			container := sampleScalerReady
//
// 			expecterReplicas := common.ReplicaList{
// 				{
// 					ID: "0",
// 					LoadIndexes: []common.LoadIndex{
// 						{
// 							Shard: common.Shard{
// 								UID:  "shard-uid-0",
// 								ID:   "shard-0",
// 								Data: map[string]string{"key": "value"},
// 							},
// 							Value:        resource.MustParse("1"),
// 							DisplayValue: "1",
// 						},
// 					},
// 					TotalLoad:             resource.MustParse("1"),
// 					TotalLoadDisplayValue: "1",
// 				},
// 			}
// 			samplePartitionReady.Object.Status.Replicas = expecterReplicas
// 			Expect(k8sClient.statusUpdate(ctx, samplePartitionReady.Generic())).To(Succeed())
// 			Expect(k8sClient.get(ctx, samplePartitionReady.Generic())).To(Succeed())
//
// 			By("Reconciling with the expectation to fails to update the status")
// 			NewClientOperationCheck(
// 				ctx,
// 				container,
// 				func(fClient client.Client) *ReplicaSetScalerReconciler {
// 					return &ReplicaSetScalerReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			).WithStatusUpdateFailureCheck(
// 				ClientThatFailsToUpdateStatus(nil),
// 				CheckFailureToUpdateStatus,
// 			).WithCheck(
// 				func(*objectContainer[client.Object]) client.Client {
// 					return k8sClient
// 				},
// 				func(
// 					container *objectContainer[*autoscalerv1alpha1.ReplicaSetScaler],
// 					result reconcile.Result,
// 					err error,
// 				) {
// 					Expect(err).NotTo(HaveOccurred())
// 					Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 					Expect(result.Requeue).To(BeFalse())
//
// 					By("Checking conditions")
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 					readyCondition := meta.FindStatusCondition(
// 						container.Object.Status.Conditions,
// 						StatusTypeReady,
// 					)
// 					Expect(readyCondition).NotTo(BeNil())
// 					Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 					Expect(readyCondition.Reason).To(Equal("ErrorFindingStatefulSet"))
// 					Expect(readyCondition.Message).To(ContainSubstring("fake error getting sts"))
// 				},
// 			).Check()
// 		})
// 	})
// })
