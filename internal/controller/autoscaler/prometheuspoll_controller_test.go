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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

type fakePrometheusPoller struct{}

func (r *fakePrometheusPoller) Poll(
	ctx context.Context,
	poll autoscalerv1alpha1.PrometheusPoll,
	shards []common.Shard,
) ([]common.MetricValue, error) {
	return nil, errors.New("fake error")
}

var _ = Describe("PrometheusPoll Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		var sampleNamespace *objectContainer[*corev1.Namespace]

		var sampleShardManager *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
		var sampleNotReadyShardManager *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
		var sampleShardManagerWithNoShards *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]
		var sampleShardManagerWithShards *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]

		var samplePoll *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var samplePollNotReadyShardManagerRef *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var sampleMalformedMetricsPoll *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var sampleMalformedShardManagerRefPoll *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var samplePollNoShards *objectContainer[*autoscalerv1alpha1.PrometheusPoll]

		BeforeEach(func() {
			By("Creating resources")

			sampleNamespace = newNamespaceWithRandomName()
			Expect(k8sClient.create(ctx, sampleNamespace.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleNamespace.Generic())).To(Succeed())

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

			sampleNotReadyShardManager = NewObjectContainer(&autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-not-ready-shard-manager",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: sampleShardManager.Object.Spec,
			})
			Expect(k8sClient.create(ctx, sampleNotReadyShardManager.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleNotReadyShardManager.Generic())).To(Succeed())
			meta.SetStatusCondition(&sampleNotReadyShardManager.Object.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "FakeNotReady",
				Message: "Not ready for test",
			})
			Expect(k8sClient.statusUpdate(ctx, sampleNotReadyShardManager.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleNotReadyShardManager.Generic())).To(Succeed())

			sampleShardManagerWithNoShards = NewObjectContainer(&autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-shard-manager-with-zero-shards",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: sampleShardManager.Object.Spec,
			})
			Expect(k8sClient.create(ctx, sampleShardManagerWithNoShards.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleShardManagerWithNoShards.Generic())).To(Succeed())
			sampleShardManagerWithNoShards.Object.Status.Shards = []common.Shard{}
			meta.SetStatusCondition(&sampleShardManagerWithNoShards.Object.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionTrue,
				Reason:  StatusTypeReady,
				Message: StatusTypeReady,
			})
			Expect(k8sClient.statusUpdate(ctx, sampleShardManagerWithNoShards.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleShardManagerWithNoShards.Generic())).To(Succeed())

			sampleShardManagerWithShards = NewObjectContainer(&autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-shard-manager-with-shards",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: sampleShardManager.Object.Spec,
			})
			Expect(k8sClient.create(ctx, sampleShardManagerWithShards.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleShardManagerWithShards.Generic())).To(Succeed())
			sampleShardManagerWithShards.Object.Status.Shards = []common.Shard{
				{
					UID: types.UID("fake-uid"),
					ID:  "fake-id",
					Data: map[string]string{
						"fake-key": "fake-value",
					},
				},
			}
			meta.SetStatusCondition(&sampleShardManagerWithShards.Object.Status.Conditions, metav1.Condition{
				Type:    StatusTypeReady,
				Status:  metav1.ConditionTrue,
				Reason:  StatusTypeReady,
				Message: StatusTypeReady,
			})
			Expect(k8sClient.statusUpdate(ctx, sampleShardManagerWithShards.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleShardManagerWithShards.Generic())).To(Succeed())

			samplePoll = NewObjectContainer(&autoscalerv1alpha1.PrometheusPoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-poll",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: autoscalerv1alpha1.PrometheusPollSpec{
					PollerSpec: common.PollerSpec{
						ShardManagerRef: &corev1.TypedLocalObjectReference{
							APIGroup: ptr.To("autoscaler.argoproj.io"),
							Kind:     "SecretTypeClusterShardManager",
							Name:     sampleShardManager.ObjectKey.Name,
						},
					},
					Period:  metav1.Duration{Duration: time.Minute},
					Address: "http://prometheus:9090",
					Metrics: []autoscalerv1alpha1.PrometheusMetric{
						{
							ID:     "fake-test",
							Query:  "fake-query",
							NoData: ptr.To(resource.MustParse("0")),
						},
					},
				},
			})
			Expect(k8sClient.create(ctx, samplePoll.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, samplePoll.Generic())).To(Succeed())

			sampleMalformedMetricsPoll = NewObjectContainer(&autoscalerv1alpha1.PrometheusPoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-malformed-metrics-poll",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: samplePoll.Object.Spec,
			})
			sampleMalformedMetricsPoll.Object.Spec.Metrics = append(
				sampleMalformedMetricsPoll.Object.Spec.Metrics,
				sampleMalformedMetricsPoll.Object.Spec.Metrics[0],
			)
			Expect(k8sClient.create(ctx, sampleMalformedMetricsPoll.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleMalformedMetricsPoll.Generic())).To(Succeed())

			sampleMalformedShardManagerRefPoll = NewObjectContainer(&autoscalerv1alpha1.PrometheusPoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-malformed-shards-manager-ref-poll",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: samplePoll.Object.Spec,
			})
			sampleMalformedShardManagerRefPoll.Object.Spec.ShardManagerRef = &corev1.TypedLocalObjectReference{
				APIGroup: ptr.To("autoscaler.argoproj.io"),
				Kind:     "SecretTypeClusterShardManager",
				Name:     "non-existing-shard-manager",
			}
			Expect(k8sClient.create(ctx, sampleMalformedShardManagerRefPoll.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleMalformedShardManagerRefPoll.Generic())).To(Succeed())

			samplePollNotReadyShardManagerRef = NewObjectContainer(&autoscalerv1alpha1.PrometheusPoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-poll-not-ready-shard-manager-ref",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: samplePoll.Object.Spec,
			})
			samplePollNotReadyShardManagerRef.Object.Spec.ShardManagerRef = &corev1.TypedLocalObjectReference{
				APIGroup: ptr.To("autoscaler.argoproj.io"),
				Kind:     "SecretTypeClusterShardManager",
				Name:     sampleNotReadyShardManager.Object.Name,
			}
			Expect(k8sClient.create(ctx, samplePollNotReadyShardManagerRef.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, samplePollNotReadyShardManagerRef.Generic())).To(Succeed())

			samplePollNoShards = NewObjectContainer(&autoscalerv1alpha1.PrometheusPoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-poll-with-no-shards",
					Namespace: sampleNamespace.ObjectKey.Name,
				},
				Spec: samplePoll.Object.Spec,
			})
			samplePollNoShards.Object.Spec.ShardManagerRef = &corev1.TypedLocalObjectReference{
				APIGroup: ptr.To("autoscaler.argoproj.io"),
				Kind:     "SecretTypeClusterShardManager",
				Name:     sampleShardManagerWithNoShards.Object.Name,
			}
			Expect(k8sClient.create(ctx, samplePollNoShards.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, samplePollNoShards.Generic())).To(Succeed())
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
			CheckExitingOnNonExistingResource(func() *PrometheusPollReconciler {
				return &PrometheusPollReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
			})
		})

		It("should handle error getting resource", func() {
			By("Failing to get resource")
			CheckFailureToGetResource(
				samplePoll,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should handle errors during reading metrics queries", func() {
			By("Reconciling malformed resource")

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: sampleMalformedMetricsPoll.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, sampleMalformedMetricsPoll.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				sampleMalformedMetricsPoll.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("MalformedResource"))
			Expect(readyCondition.Message).To(Equal("duplicate metric for ID 'fake-test'"))
		})

		It("should handle errors updating status during errors during reading metrics queries", func() {
			By("Reconciling malformed resource")
			CheckFailureToUpdateStatus(
				sampleMalformedMetricsPoll,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should handle errors when shard manager lookup fails", func() {
			By("Reconciling malformed resource")

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: sampleMalformedShardManagerRefPoll.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, sampleMalformedShardManagerRefPoll.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				sampleMalformedShardManagerRefPoll.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("ErrorFindingShardManager"))
			Expect(readyCondition.Message).To(ContainSubstring("\"non-existing-shard-manager\" not found"))
		})

		It("should handle errors updating status during errors when shard manager lookup fails", func() {
			By("Reconciling malformed resource")
			CheckFailureToUpdateStatus(
				sampleMalformedShardManagerRefPoll,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should exit if shard manager not ready", func() {
			By("Reconciling malformed resource")

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: samplePollNotReadyShardManagerRef.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, samplePollNotReadyShardManagerRef.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				samplePollNotReadyShardManagerRef.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("ShardManagerNotReady"))
			Expect(readyCondition.Message).To(ContainSubstring("Check the status of shard manager"))
		})

		It("should handle errors updating status when shard manager not ready", func() {
			By("Reconciling malformed resource")
			CheckFailureToUpdateStatus(
				samplePollNotReadyShardManagerRef,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should exit if shard manager has no shards", func() {
			By("Reconciling malformed resource")

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: samplePollNoShards.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, samplePollNoShards.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				samplePollNoShards.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("NoShardsFound"))
			Expect(readyCondition.Message).To(Equal("No shards found"))
		})

		It("should handle errors updating status when shard manager has no shards", func() {
			By("Reconciling malformed resource")
			CheckFailureToUpdateStatus(
				samplePollNoShards,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})
	})
})
