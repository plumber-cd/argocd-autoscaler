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

type fakePrometheusPoller struct {
	fn func(ctx context.Context, poll autoscalerv1alpha1.PrometheusPoll, shards []common.Shard) ([]common.MetricValue, error)
}

func (r *fakePrometheusPoller) Poll(
	ctx context.Context,
	poll autoscalerv1alpha1.PrometheusPoll,
	shards []common.Shard,
) ([]common.MetricValue, error) {
	if r.fn != nil {
		return r.fn(ctx, poll, shards)
	}
	return nil, errors.New("poller not implemented")
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
		var samplePollWithShards *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var samplePollRecent *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var samplePollRecentChangedMetrics *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var samplePollRecentChangedShards *objectContainer[*autoscalerv1alpha1.PrometheusPoll]

		BeforeEach(func() {
			By("Creating resources")

			sampleNamespace = newNamespaceWithRandomName()
			Expect(k8sClient.create(ctx, sampleNamespace.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleNamespace.Generic())).To(Succeed())

			sampleShardManager = NewObjectContainer(
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
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
				},
				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleNotReadyShardManager = NewObjectContainer(
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-not-ready-shard-manager",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleShardManager.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					meta.SetStatusCondition(&container.Object.Status.Conditions, metav1.Condition{
						Type:    StatusTypeReady,
						Status:  metav1.ConditionFalse,
						Reason:  "FakeNotReady",
						Message: "Not ready for test",
					})
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleShardManagerWithNoShards = NewObjectContainer(
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-shard-manager-with-zero-shards",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleShardManager.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					container.Object.Status.Shards = []common.Shard{}
					meta.SetStatusCondition(&container.Object.Status.Conditions, metav1.Condition{
						Type:    StatusTypeReady,
						Status:  metav1.ConditionTrue,
						Reason:  StatusTypeReady,
						Message: StatusTypeReady,
					})
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleShardManagerWithShards = NewObjectContainer(
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-shard-manager-with-shards",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleShardManager.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					container.Object.Status.Shards = []common.Shard{
						{
							UID: types.UID("fake-uid"),
							ID:  "fake-id",
							Data: map[string]string{
								"fake-key": "fake-value",
							},
						},
					}
					meta.SetStatusCondition(&container.Object.Status.Conditions, metav1.Condition{
						Type:    StatusTypeReady,
						Status:  metav1.ConditionTrue,
						Reason:  StatusTypeReady,
						Message: StatusTypeReady,
					})
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePoll = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
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
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleMalformedMetricsPoll = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-malformed-metrics-poll",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePoll.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					container.Object.Spec.Metrics = append(
						container.Object.Spec.Metrics,
						container.Object.Spec.Metrics[0],
					)
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleMalformedShardManagerRefPoll = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-malformed-shards-manager-ref-poll",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePoll.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					container.Object.Spec.ShardManagerRef = &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("autoscaler.argoproj.io"),
						Kind:     "SecretTypeClusterShardManager",
						Name:     "non-existing-shard-manager",
					}
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePollNotReadyShardManagerRef = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll-not-ready-shard-manager-ref",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePoll.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					container.Object.Spec.ShardManagerRef = &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("autoscaler.argoproj.io"),
						Kind:     "SecretTypeClusterShardManager",
						Name:     sampleNotReadyShardManager.Object.Name,
					}
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePollNoShards = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll-with-no-shards",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePoll.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					container.Object.Spec.ShardManagerRef = &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("autoscaler.argoproj.io"),
						Kind:     "SecretTypeClusterShardManager",
						Name:     sampleShardManagerWithNoShards.Object.Name,
					}
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePollWithShards = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll-with-shards",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePoll.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					container.Object.Spec.ShardManagerRef = &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("autoscaler.argoproj.io"),
						Kind:     "SecretTypeClusterShardManager",
						Name:     sampleShardManagerWithShards.Object.Name,
					}
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePollRecent = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll-recent",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePollWithShards.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					container.Object.Status.Values = []common.MetricValue{}
					for _, shard := range sampleShardManagerWithShards.Object.Status.Shards {
						for _, metric := range container.Object.Spec.Metrics {
							container.Object.Status.Values = append(
								container.Object.Status.Values,
								common.MetricValue{
									ID:           metric.ID,
									Shard:        shard,
									Query:        metric.Query,
									Value:        resource.MustParse("0"),
									DisplayValue: "0",
								})
						}
					}
					container.Object.Status.LastPollingTime = ptr.To(metav1.NewTime(
						time.Now().Add(-30 * time.Second),
					))
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePollRecentChangedMetrics = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll-recent-changed-metrics",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePollRecent.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					container.Object.Status = samplePollRecent.Object.Status
					for _, shard := range sampleShardManagerWithShards.Object.Status.Shards {
						for _, metric := range container.Object.Spec.Metrics {
							container.Object.Status.Values = append(
								container.Object.Status.Values,
								common.MetricValue{
									ID:           metric.ID + "_copy",
									Shard:        shard,
									Query:        metric.Query,
									Value:        resource.MustParse("0"),
									DisplayValue: "0",
								})
						}
					}
					container.Object.Status.LastPollingTime = ptr.To(metav1.NewTime(
						time.Now().Add(-30 * time.Second),
					))
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePollRecentChangedShards = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll-recent-changed-shards",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePollRecent.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					container.Object.Status = samplePollRecent.Object.Status
					for _, shard := range sampleShardManagerWithShards.Object.Status.Shards {
						copyShard := shard.DeepCopy()
						copyShard.UID = copyShard.UID + "-copy"
						copyShard.ID = copyShard.ID + "-copy"
						for _, metric := range container.Object.Spec.Metrics {
							container.Object.Status.Values = append(
								container.Object.Status.Values,
								common.MetricValue{
									ID:           metric.ID,
									Shard:        *copyShard,
									Query:        metric.Query,
									Value:        resource.MustParse("0"),
									DisplayValue: "0",
								})
						}
					}
					container.Object.Status.LastPollingTime = ptr.To(metav1.NewTime(
						time.Now().Add(-30 * time.Second),
					))
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
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
				k8sClient,
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

			container := sampleMalformedMetricsPoll

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				container.Object.Status.Conditions,
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
				k8sClient,
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

			container := sampleMalformedShardManagerRefPoll

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				container.Object.Status.Conditions,
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
				k8sClient,
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
			By("Reconciling resource")

			container := samplePollNotReadyShardManagerRef

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				container.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("ShardManagerNotReady"))
			Expect(readyCondition.Message).To(ContainSubstring("Check the status of shard manager"))
		})

		It("should handle errors updating status when shard manager not ready", func() {
			By("Reconciling resource")
			CheckFailureToUpdateStatus(
				k8sClient,
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
			By("Reconciling resource")

			container := samplePollNoShards

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				container.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("NoShardsFound"))
			Expect(readyCondition.Message).To(Equal("No shards found"))
		})

		It("should handle errors updating status when shard manager has no shards", func() {
			By("Reconciling resource")
			CheckFailureToUpdateStatus(
				k8sClient,
				samplePollNoShards,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should re-queue if was already recently polled", func() {
			By("Reconciling resource")

			container := samplePollRecent

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically("~", 30*time.Second, 2*time.Second))
			Expect(result.Requeue).To(BeFalse())
		})

		It("should poll now if was already recently polled but changed metrics", func() {
			By("Reconciling resource")

			container := samplePollRecentChangedMetrics

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Poller: &fakePrometheusPoller{},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("poller not implemented"))
		})

		It("should poll now if was already recently polled but changed shards", func() {
			By("Reconciling resource")

			container := samplePollRecentChangedShards

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Poller: &fakePrometheusPoller{},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("poller not implemented"))
		})

		It("should successfully poll", func() {
			By("Preparing poller")

			container := samplePollWithShards

			polled := false
			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Poller: &fakePrometheusPoller{
					fn: func(ctx context.Context, poll autoscalerv1alpha1.PrometheusPoll, shards []common.Shard) ([]common.MetricValue, error) {
						metricValues := []common.MetricValue{}
						for _, shard := range shards {
							for _, metric := range poll.Spec.Metrics {
								metricValues = append(
									metricValues,
									common.MetricValue{
										ID:           metric.ID,
										Shard:        shard,
										Query:        metric.Query,
										Value:        resource.MustParse("0"),
										DisplayValue: "0",
									},
								)
							}
						}
						polled = true
						return metricValues, nil
					},
				},
			}

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
						Poller: controllerReconciler.Poller,
					}
				},
			)

			By("Reconciling resource")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(polled).To(BeTrue())
			Expect(result.RequeueAfter).To(Equal(container.Object.Spec.Period.Duration))

			By("Checking conditions")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				container.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

			By("Checking polling results")
			metricsById := map[string]autoscalerv1alpha1.PrometheusMetric{}
			for _, metric := range container.Object.Spec.Metrics {
				metricsById[metric.ID] = metric
			}
			shardsByUID := map[types.UID]common.Shard{}
			for _, shard := range sampleShardManagerWithShards.Object.Status.Shards {
				shardsByUID[shard.UID] = shard
			}
			metricValues := container.Object.Status.Values
			Expect(metricValues).
				To(HaveLen(
					len(container.Object.Spec.Metrics) *
						len(sampleShardManagerWithShards.Object.Status.Shards),
				))
			valuesByMetricIDAndShardUID := map[string]common.MetricValue{}
			for _, val := range metricValues {
				valuesByMetricIDAndShardUID[val.ID+":"+string(val.Shard.UID)] = val
			}
			for metricID, metric := range metricsById {
				for shardUID, shard := range shardsByUID {
					key := metricID + ":" + string(shardUID)
					val, ok := valuesByMetricIDAndShardUID[key]
					Expect(ok).To(BeTrue())
					Expect(val.ID).To(Equal(metric.ID))
					Expect(val.Shard).To(Equal(shard))
					Expect(val.Query).To(Equal(metric.Query))
					Expect(val.Value).To(Equal(resource.MustParse("0")))
					Expect(val.DisplayValue).To(Equal("0"))
				}
			}
		})

		It("should handle errors during polling", func() {
			By("Preparing poller")

			container := samplePollWithShards

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Poller: &fakePrometheusPoller{
					fn: func(ctx context.Context, poll autoscalerv1alpha1.PrometheusPoll, shards []common.Shard) ([]common.MetricValue, error) {
						return nil, errors.New("fake error")
					},
				},
			}

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
						Poller: controllerReconciler.Poller,
					}
				},
			)

			By("Reconciling resource")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake error"))
			Expect(result.RequeueAfter).To(Equal(time.Second))

			By("Checking conditions")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				container.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("PollingError"))
			Expect(readyCondition.Message).To(ContainSubstring("fake error"))
		})

		It("should check results count against expectation", func() {
			By("Preparing poller")

			container := samplePollWithShards

			controllerReconciler := &PrometheusPollReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Poller: &fakePrometheusPoller{
					fn: func(ctx context.Context, poll autoscalerv1alpha1.PrometheusPoll, shards []common.Shard) ([]common.MetricValue, error) {
						return []common.MetricValue{}, nil
					},
				},
			}

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *PrometheusPollReconciler {
					return &PrometheusPollReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
						Poller: controllerReconciler.Poller,
					}
				},
			)

			By("Reconciling resource")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Expected poll results didn't match number of metrics * number of shards"))
			Expect(result.RequeueAfter).To(Equal(container.Object.Spec.Period.Duration))

			By("Checking conditions")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				container.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("ResultsCountMismatch"))
			Expect(readyCondition.Message).To(ContainSubstring("Expected poll results didn't match number of metrics * number of shards"))
		})
	})
})
