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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

var _ = Describe("SecretTypeClusterShardManager Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		var sampleNamespace *objectContainer[*corev1.Namespace]
		var sampleSecret *objectContainer[*corev1.Secret]
		var sampleShardManager *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
		var shardManagerWithDesiredReplicas *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
		var shardManagerWithMalformedDesiredReplicas *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]

		BeforeEach(func() {
			By("Creating resources")

			sampleNamespace = newNamespaceWithRandomName()
			Expect(k8sClient.create(ctx, sampleNamespace.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleNamespace.Generic())).To(Succeed())

			sampleSecret = NewObjectContainer(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-secret",
					Namespace: sampleNamespace.ObjectKey.Name,
					Labels: map[string]string{
						"mock-label": "mock",
					},
				},
				StringData: map[string]string{
					"name":   "mock-cluster",
					"server": "http://mock-cluster:8000",
				},
			}, func(container *objectContainer[*corev1.Secret]) {
				Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
				Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			})

			// We don't really need to reference it, the point of it existence is so that
			//  it DOESN'T come up anywhere because it wouldn't match the label selector
			_ = NewObjectContainer(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unlabeled-secret",
					Namespace: sampleNamespace.ObjectKey.Name,
					Labels: map[string]string{
						"mock-label": "mock-2",
					},
				},
				StringData: map[string]string{
					"name":   "mock-cluster-2",
					"server": "http://mock-cluster-2:8000",
				},
			}, func(container *objectContainer[*corev1.Secret]) {
				Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
				Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			})

			sampleShardManager = NewObjectContainer(&autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-shard-manager",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: autoscalerv1alpha1.SecretTypeClusterShardManagerSpec{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"mock-label": "mock",
						},
					},
				},
			})
			Expect(k8sClient.create(ctx, sampleShardManager.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleShardManager.Generic())).To(Succeed())

			shardManagerWithDesiredReplicas = NewObjectContainer(
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shard-manager-with-desired-replicas",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleShardManager.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
					container.Object.Spec.ShardManagerSpec = common.ShardManagerSpec{
						Replicas: common.ReplicaList{
							{
								ID: "0",
								LoadIndexes: []common.LoadIndex{
									{
										Shard: common.Shard{
											UID: sampleSecret.Object.GetUID(),
											ID:  sampleSecret.ObjectKey.Name,
											Data: map[string]string{
												"fake": "data",
											},
										},
										Value:        resource.MustParse("0"),
										DisplayValue: "0",
									},
								},
								TotalLoad:             resource.MustParse("0"),
								TotalLoadDisplayValue: "0",
							},
						},
					}
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			shardManagerWithMalformedDesiredReplicas = NewObjectContainer(
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "malformed-shard-manager",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: shardManagerWithDesiredReplicas.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
					container.Object.Spec.ShardManagerSpec.
						Replicas[0].LoadIndexes = append(
						container.Object.Spec.ShardManagerSpec.Replicas[0].LoadIndexes,
						container.Object.Spec.ShardManagerSpec.Replicas[0].LoadIndexes[0],
					)
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)
		})

		AfterEach(func() {
			err := k8sClient.get(ctx, sampleNamespace.Generic())
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the test namespace")
			Expect(k8sClient.delete(ctx, sampleNamespace.Generic())).To(Succeed())
			sampleNamespace = nil
		})

		It("should successfully exit when the resource didn't exist", func() {
			By("Reconciling non existing resource")
			CheckExitingOnNonExistingResource(func() *SecretTypeClusterShardManagerReconciler {
				return &SecretTypeClusterShardManagerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
			})
		})

		It("should handle error getting resource", func() {
			By("Failing to get resource")
			CheckFailureToGetResource(
				k8sClient,
				sampleShardManager,
				func(fClient *fakeClient) *SecretTypeClusterShardManagerReconciler {
					return &SecretTypeClusterShardManagerReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should handle errors on listing secret", func() {
			By("Reconciling resource")

			container := sampleShardManager

			fClient := &fakeClient{
				Client: k8sClient,
			}
			fClient.
				WithListFunction(&corev1.SecretList{},
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						return errors.NewBadRequest("fake error listing secrets")
					},
				)

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.ObjectKey,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake error listing secrets"))
			Expect(result.RequeueAfter).To(Equal(time.Second))

			By("Checking ready condition")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(container.Object.Status.Conditions, StatusTypeReady)
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("FailedToListSecrets"))
			Expect(readyCondition.Message).To(Equal("fake error listing secrets"))

			By("Reconciling resource again with expected failure to update the status")
			CheckFailureToUpdateStatus(
				fClient,
				container,
				func(fClient *fakeClient) *SecretTypeClusterShardManagerReconciler {
					return &SecretTypeClusterShardManagerReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should successfully discover shards", func() {
			By("Reconciling resource")

			container := sampleShardManager

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(container.Object.Status.Conditions, StatusTypeReady)
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

			By("Reconciling resource again with expected failure to update the status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *SecretTypeClusterShardManagerReconciler {
					return &SecretTypeClusterShardManagerReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should successfully update shards on secrets", func() {
			By("Reconciling with replicas set")

			container := shardManagerWithDesiredReplicas

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(container.Object.Status.Conditions, StatusTypeReady)
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

			By("Checking secret was assigned a shard")
			Expect(k8sClient.get(ctx, sampleSecret.Generic())).To(Succeed())
			Expect(sampleSecret.Object.Data["shard"]).To(Equal([]byte("0")))
		})

		It("should handle error updating the secret", func() {
			By("Reconciling with replicas set")

			container := shardManagerWithDesiredReplicas

			fClient := &fakeClient{
				Client: k8sClient,
			}
			fClient.
				WithUpdateFunction(sampleSecret.Generic(),
					func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return errors.NewBadRequest("fake error updating secret")
					},
				)

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake error updating secret"))
			Expect(result.RequeueAfter).To(Equal(time.Second))

			By("Checking ready condition")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(container.Object.Status.Conditions, StatusTypeReady)
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("FailedToUpdateSecret"))
			Expect(readyCondition.Message).To(Equal("fake error updating secret"))

			By("Reconciling resource again with expected failure to update the status")
			CheckFailureToUpdateStatus(
				fClient,
				container,
				func(fClient *fakeClient) *SecretTypeClusterShardManagerReconciler {
					return &SecretTypeClusterShardManagerReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should handle errors when desired replicas was malformed", func() {
			By("Reconciling with replicas set")

			container := shardManagerWithMalformedDesiredReplicas

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(container.Object.Status.Conditions, StatusTypeReady)
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("FailedToListReplicas"))
			Expect(readyCondition.Message).To(Equal("duplicate replica found"))

			By("Reconciling resource again with expected failure to update the status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *SecretTypeClusterShardManagerReconciler {
					return &SecretTypeClusterShardManagerReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})
	})
})
