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
		var unlabeledSecret *objectContainer[*corev1.Secret]
		var sampleShardManager *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
		var shardManagerWithDesiredReplicas *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
		var shardManagerWithMalformedDesiredReplicas *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]

		BeforeEach(func() {
			By("Creating resources")

			sampleNamespace = newNamespaceWithRandomName()
			Expect(k8sClient.Create(ctx, sampleNamespace.Object)).To(Succeed())
			Expect(k8sClient.Get(ctx, sampleNamespace.ObjectKey, sampleNamespace.Object)).To(Succeed())

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
			})
			Expect(k8sClient.Create(ctx, sampleSecret.Object)).To(Succeed())
			Expect(k8sClient.Get(ctx, sampleSecret.ObjectKey, sampleSecret.Object)).To(Succeed())

			unlabeledSecret = NewObjectContainer(&corev1.Secret{
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
			})
			Expect(k8sClient.Create(ctx, unlabeledSecret.Object)).To(Succeed())
			Expect(k8sClient.Get(ctx, unlabeledSecret.ObjectKey, unlabeledSecret.Object)).To(Succeed())

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
			Expect(k8sClient.Create(ctx, sampleShardManager.Object)).To(Succeed())
			Expect(k8sClient.Get(ctx, sampleShardManager.ObjectKey, sampleShardManager.Object)).To(Succeed())

			shardManagerWithDesiredReplicas = NewObjectContainer(
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shard-manager-with-desired-replicas",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleShardManager.Object.Spec,
				},
			)
			shardManagerWithDesiredReplicas.Object.Spec.ShardManagerSpec = common.ShardManagerSpec{
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
			Expect(k8sClient.Create(ctx, shardManagerWithDesiredReplicas.Object)).To(Succeed())
			Expect(k8sClient.Get(
				ctx,
				shardManagerWithDesiredReplicas.ObjectKey,
				shardManagerWithDesiredReplicas.Object,
			)).To(Succeed())

			shardManagerWithMalformedDesiredReplicas = NewObjectContainer(
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "malformed-shard-manager",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: shardManagerWithDesiredReplicas.Object.Spec,
				},
			)
			shardManagerWithMalformedDesiredReplicas.Object.Spec.ShardManagerSpec.
				Replicas[0].LoadIndexes = append(
				shardManagerWithMalformedDesiredReplicas.Object.Spec.ShardManagerSpec.
					Replicas[0].LoadIndexes,
				shardManagerWithMalformedDesiredReplicas.Object.Spec.ShardManagerSpec.
					Replicas[0].LoadIndexes[0],
			)
			Expect(k8sClient.Create(ctx, shardManagerWithMalformedDesiredReplicas.Object)).To(Succeed())
			Expect(k8sClient.Get(ctx, shardManagerWithMalformedDesiredReplicas.ObjectKey,
				shardManagerWithMalformedDesiredReplicas.Object)).To(Succeed())
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, sampleNamespace.ObjectKey, sampleNamespace.Object)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the test namespace")
			Expect(k8sClient.Delete(ctx, sampleNamespace.Object)).To(Succeed())
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
				NamespacedName: sampleShardManager.ObjectKey,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake error listing secrets"))
			Expect(result.RequeueAfter).To(Equal(time.Second))

			By("Checking ready condition")
			Expect(k8sClient.Get(ctx, sampleShardManager.ObjectKey, sampleShardManager.Object)).
				To(Succeed())
			Expect(sampleShardManager.Object.Status.Conditions).
				To(HaveLen(1))
			Expect(sampleShardManager.Object.Status.Conditions[0].Type).
				To(Equal(StatusTypeReady))
			Expect(sampleShardManager.Object.Status.Conditions[0].Status).
				To(Equal(metav1.ConditionFalse))
			Expect(sampleShardManager.Object.Status.Conditions[0].Reason).
				To(Equal("FailedToListSecrets"))
			Expect(sampleShardManager.Object.Status.Conditions[0].Message).
				To(Equal("fake error listing secrets"))

			By("Reconciling resource again with expected failure to update the status")
			CheckFailureToUpdateStatus(
				sampleShardManager,
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

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: sampleShardManager.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			Expect(k8sClient.Get(ctx, sampleShardManager.ObjectKey, sampleShardManager.Object)).
				To(Succeed())
			Expect(sampleShardManager.Object.Status.Conditions).
				To(HaveLen(1))
			Expect(sampleShardManager.Object.Status.Conditions[0].Type).
				To(Equal(StatusTypeReady))
			Expect(sampleShardManager.Object.Status.Conditions[0].Status).
				To(Equal(metav1.ConditionTrue))
			Expect(sampleShardManager.Object.Status.Conditions[0].Reason).
				To(Equal(StatusTypeReady))

			By("Reconciling resource again with expected failure to update the status")
			CheckFailureToUpdateStatus(
				sampleShardManager,
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

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: shardManagerWithDesiredReplicas.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			Expect(
				k8sClient.Get(
					ctx,
					shardManagerWithDesiredReplicas.NamespacedName,
					shardManagerWithDesiredReplicas.Object,
				),
			).To(Succeed())
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions).
				To(HaveLen(1))
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions[0].Type).
				To(Equal(StatusTypeReady))
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions[0].Status).
				To(Equal(metav1.ConditionTrue))
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions[0].Reason).
				To(Equal(StatusTypeReady))

			By("Checking secret was assigned a shard")
			Expect(k8sClient.Get(ctx, sampleSecret.ObjectKey, sampleSecret.Object)).To(Succeed())
			Expect(sampleSecret.Object.Data["shard"]).To(Equal([]byte("0")))
		})

		It("should handle error updating the secret", func() {
			By("Reconciling with replicas set")

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
				NamespacedName: shardManagerWithDesiredReplicas.NamespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake error updating secret"))
			Expect(result.RequeueAfter).To(Equal(time.Second))

			By("Checking ready condition")
			Expect(
				k8sClient.Get(
					ctx,
					shardManagerWithDesiredReplicas.ObjectKey,
					shardManagerWithDesiredReplicas.Object,
				),
			).To(Succeed())
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions).
				To(HaveLen(1))
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions[0].Type).
				To(Equal(StatusTypeReady))
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions[0].Status).
				To(Equal(metav1.ConditionFalse))
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions[0].Reason).
				To(Equal("FailedToUpdateSecret"))
			Expect(shardManagerWithDesiredReplicas.Object.Status.Conditions[0].Message).
				To(Equal("fake error updating secret"))

			By("Reconciling resource again with expected failure to update the status")
			CheckFailureToUpdateStatus(
				shardManagerWithDesiredReplicas,
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

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: shardManagerWithMalformedDesiredReplicas.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			Expect(
				k8sClient.Get(
					ctx,
					shardManagerWithMalformedDesiredReplicas.ObjectKey,
					shardManagerWithMalformedDesiredReplicas.Object,
				),
			).To(Succeed())
			Expect(shardManagerWithMalformedDesiredReplicas.Object.Status.Conditions).
				To(HaveLen(1))
			Expect(shardManagerWithMalformedDesiredReplicas.Object.Status.Conditions[0].Type).
				To(Equal(StatusTypeReady))
			Expect(shardManagerWithMalformedDesiredReplicas.Object.Status.Conditions[0].Status).
				To(Equal(metav1.ConditionFalse))
			Expect(shardManagerWithMalformedDesiredReplicas.Object.Status.Conditions[0].Reason).
				To(Equal("FailedToListReplicas"))
			Expect(shardManagerWithMalformedDesiredReplicas.Object.Status.Conditions[0].Message).
				To(Equal("duplicate replica found"))

			By("Reconciling resource again with expected failure to update the status")
			CheckFailureToUpdateStatus(
				shardManagerWithMalformedDesiredReplicas,
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
