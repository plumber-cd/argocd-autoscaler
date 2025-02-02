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
	"k8s.io/apimachinery/pkg/types"
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

		var namespaceName client.ObjectKey
		var namespacedName types.NamespacedName

		var sampleNamespace *corev1.Namespace
		var sampleSecret *corev1.Secret
		var unlabeledSecret *corev1.Secret
		var sampleShardManager *autoscalerv1alpha1.SecretTypeClusterShardManager
		var shardManagerWithDesiredReplicas *autoscalerv1alpha1.SecretTypeClusterShardManager
		var shardManagerWithMalformedDesiredReplicas *autoscalerv1alpha1.SecretTypeClusterShardManager

		checkFailureToUpdateStatus := func(namespacedName types.NamespacedName) {
			fakeClient := &fakeClient{
				Client: k8sClient,
			}
			fakeClient.
				WithStatusUpdateFunction(&autoscalerv1alpha1.SecretTypeClusterShardManager{},
					namespacedName,
					func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
						return errors.NewBadRequest("fake error updating status")
					},
				)

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: fakeClient,
				Scheme: fakeClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake error updating status"))
			Expect(result.RequeueAfter).To(Equal(time.Second))
		}

		BeforeEach(func() {
			By("Creating resources")

			namespaceName, namespacedName = newNamespace()

			sampleNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName.Name,
				},
			}
			Expect(k8sClient.Create(ctx, sampleNamespace)).To(Succeed())
			Expect(k8sClient.Get(ctx, namespaceName, sampleNamespace)).To(Succeed())

			sampleSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
					Labels: map[string]string{
						"mock-label": "mock",
					},
				},
				StringData: map[string]string{
					"name":   "mock-cluster",
					"server": "http://mock-cluster:8000",
				},
			}
			Expect(k8sClient.Create(ctx, sampleSecret)).To(Succeed())
			Expect(k8sClient.Get(ctx, namespacedName, sampleSecret)).To(Succeed())

			unlabeledSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unlabeled-secret",
					Namespace: namespacedName.Namespace,
					Labels: map[string]string{
						"mock-label": "mock-2",
					},
				},
				StringData: map[string]string{
					"name":   "mock-cluster-2",
					"server": "http://mock-cluster-2:8000",
				},
			}
			Expect(k8sClient.Create(ctx, unlabeledSecret)).To(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      unlabeledSecret.Name,
				Namespace: unlabeledSecret.Namespace,
			}, unlabeledSecret)).To(Succeed())

			sampleShardManager = &autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Spec: autoscalerv1alpha1.SecretTypeClusterShardManagerSpec{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"mock-label": "mock",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sampleShardManager)).To(Succeed())
			Expect(k8sClient.Get(ctx, namespacedName, sampleShardManager)).To(Succeed())

			shardManagerWithDesiredReplicas = &autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shard-manager-with-desired-replicas",
					Namespace: namespacedName.Namespace,
				},
				Spec: sampleShardManager.Spec,
			}
			shardManagerWithDesiredReplicas.Spec.ShardManagerSpec = common.ShardManagerSpec{
				Replicas: common.ReplicaList{
					{
						ID: "0",
						LoadIndexes: []common.LoadIndex{
							{
								Shard: common.Shard{
									UID: sampleSecret.GetUID(),
									ID:  sampleSecret.Name,
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
			Expect(k8sClient.Create(ctx, shardManagerWithDesiredReplicas)).To(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      shardManagerWithDesiredReplicas.Name,
				Namespace: shardManagerWithDesiredReplicas.Namespace,
			}, shardManagerWithDesiredReplicas)).To(Succeed())

			shardManagerWithMalformedDesiredReplicas = &autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "malformed-shard-manager",
					Namespace: namespacedName.Namespace,
				},
				Spec: shardManagerWithDesiredReplicas.Spec,
			}
			shardManagerWithMalformedDesiredReplicas.Spec.ShardManagerSpec.Replicas[0].LoadIndexes = append(
				shardManagerWithMalformedDesiredReplicas.Spec.ShardManagerSpec.Replicas[0].LoadIndexes,
				shardManagerWithMalformedDesiredReplicas.Spec.ShardManagerSpec.Replicas[0].LoadIndexes[0],
			)
			Expect(k8sClient.Create(ctx, shardManagerWithMalformedDesiredReplicas)).To(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      shardManagerWithMalformedDesiredReplicas.Name,
				Namespace: shardManagerWithMalformedDesiredReplicas.Namespace,
			}, shardManagerWithMalformedDesiredReplicas)).To(Succeed())
		})

		AfterEach(func() {
			namespace := &corev1.Namespace{}
			err := k8sClient.Get(ctx, namespaceName, namespace)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the test namespace")
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})

		It("should successfully exit when the resource didn't exist", func() {
			By("Reconciling non existing resource")
			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existing-resource",
					Namespace: "default",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())
		})

		It("should handle error getting resource", func() {
			By("Failing to get resource")

			fakeClient := &fakeClient{
				Client: k8sClient,
			}
			fakeClient.
				WithGetFunction(&autoscalerv1alpha1.SecretTypeClusterShardManager{},
					namespacedName,
					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return errors.NewBadRequest("fake error getting resource")
					},
				)

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: fakeClient,
				Scheme: fakeClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake error getting resource"))
			Expect(result.RequeueAfter).To(Equal(time.Second))
			Expect(result.Requeue).To(BeFalse())
		})

		It("should handle errors on listing secret", func() {
			By("Reconciling resource")

			fakeClient := &fakeClient{
				Client: k8sClient,
			}
			fakeClient.
				WithListFunction(&corev1.SecretList{},
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						return errors.NewBadRequest("fake error listing secrets")
					},
				)

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: fakeClient,
				Scheme: fakeClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake error listing secrets"))
			Expect(result.RequeueAfter).To(Equal(time.Second))

			By("Checking ready condition")
			resource := &autoscalerv1alpha1.SecretTypeClusterShardManager{}
			err = k8sClient.Get(ctx, namespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Conditions).To(HaveLen(1))
			Expect(resource.Status.Conditions[0].Type).To(Equal(StatusTypeReady))
			Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(resource.Status.Conditions[0].Reason).To(Equal("FailedToListSecrets"))
			Expect(resource.Status.Conditions[0].Message).To(Equal("fake error listing secrets"))

			By("Reconciling resource again with expected failure to update the status")
			checkFailureToUpdateStatus(namespacedName)
		})

		It("should successfully discover shards", func() {
			By("Reconciling resource")

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			resource := &autoscalerv1alpha1.SecretTypeClusterShardManager{}
			err = k8sClient.Get(ctx, namespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Conditions).To(HaveLen(1))
			Expect(resource.Status.Conditions[0].Type).To(Equal(StatusTypeReady))
			Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(resource.Status.Conditions[0].Reason).To(Equal(StatusTypeReady))

			By("Reconciling resource again with expected failure to update the status")
			checkFailureToUpdateStatus(namespacedName)
		})

		It("should successfully update shards on secrets", func() {
			By("Reconciling with replicas set")

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      shardManagerWithDesiredReplicas.Name,
					Namespace: shardManagerWithDesiredReplicas.Namespace,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			resource := &autoscalerv1alpha1.SecretTypeClusterShardManager{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      shardManagerWithDesiredReplicas.Name,
				Namespace: shardManagerWithDesiredReplicas.Namespace,
			}, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Conditions).To(HaveLen(1))
			Expect(resource.Status.Conditions[0].Type).To(Equal(StatusTypeReady))
			Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(resource.Status.Conditions[0].Reason).To(Equal(StatusTypeReady))

			By("Checking secret was assigned a shard")
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, namespacedName, secret)).To(Succeed())
			Expect(secret.Data["shard"]).To(Equal([]byte("0")))
		})

		It("should handle error updating the secret", func() {
			By("Reconciling with replicas set")

			fakeClient := &fakeClient{
				Client: k8sClient,
			}
			fakeClient.
				WithUpdateFunction(&corev1.Secret{},
					namespacedName,
					func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return errors.NewBadRequest("fake error updating secret")
					},
				)

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: fakeClient,
				Scheme: fakeClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      shardManagerWithDesiredReplicas.Name,
					Namespace: shardManagerWithDesiredReplicas.Namespace,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake error updating secret"))
			Expect(result.RequeueAfter).To(Equal(time.Second))

			By("Checking ready condition")
			shardManager := &autoscalerv1alpha1.SecretTypeClusterShardManager{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      shardManagerWithDesiredReplicas.Name,
				Namespace: shardManagerWithDesiredReplicas.Namespace,
			}, shardManager)).To(Succeed())
			Expect(shardManager.Status.Conditions).To(HaveLen(1))
			Expect(shardManager.Status.Conditions[0].Type).To(Equal(StatusTypeReady))
			Expect(shardManager.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(shardManager.Status.Conditions[0].Reason).To(Equal("FailedToUpdateSecret"))
			Expect(shardManager.Status.Conditions[0].Message).To(Equal("fake error updating secret"))

			By("Reconciling resource again with expected failure to update the status")
			checkFailureToUpdateStatus(types.NamespacedName{
				Name:      shardManagerWithDesiredReplicas.Name,
				Namespace: shardManagerWithDesiredReplicas.Namespace,
			})
		})

		It("should handle errors when desired replicas was malformed", func() {
			By("Reconciling with replicas set")

			controllerReconciler := &SecretTypeClusterShardManagerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      shardManagerWithMalformedDesiredReplicas.Name,
					Namespace: shardManagerWithMalformedDesiredReplicas.Namespace,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking ready condition")
			resource := &autoscalerv1alpha1.SecretTypeClusterShardManager{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      shardManagerWithMalformedDesiredReplicas.Name,
				Namespace: shardManagerWithMalformedDesiredReplicas.Namespace,
			}, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Conditions).To(HaveLen(1))
			Expect(resource.Status.Conditions[0].Type).To(Equal(StatusTypeReady))
			Expect(resource.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(resource.Status.Conditions[0].Reason).To(Equal("FailedToListReplicas"))
			Expect(resource.Status.Conditions[0].Message).To(Equal("duplicate replica found"))

			By("Reconciling resource again with expected failure to update the status")
			checkFailureToUpdateStatus(types.NamespacedName{
				Name:      shardManagerWithMalformedDesiredReplicas.Name,
				Namespace: shardManagerWithMalformedDesiredReplicas.Namespace,
			})
		})
	})
})
