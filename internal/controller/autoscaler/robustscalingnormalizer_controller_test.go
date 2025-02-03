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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

type fakeNormalizer struct {
	fn func(ctx context.Context, allMetrics []common.MetricValue) ([]common.MetricValue, error)
}

func (f *fakeNormalizer) Normalize(ctx context.Context, allMetrics []common.MetricValue) ([]common.MetricValue, error) {
	if f.fn != nil {
		return f.fn(ctx, allMetrics)
	}
	return nil, errors.New("fake normalizer not implemented")
}

var _ = Describe("RobustScalingNormalizer Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		var sampleNamespace *objectContainer[*corev1.Namespace]

		var samplePoll *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var samplePollNotReady *objectContainer[*autoscalerv1alpha1.PrometheusPoll]
		var samplePollWithValues *objectContainer[*autoscalerv1alpha1.PrometheusPoll]

		var sampleNormalizer *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]
		var sampleNormalizerUsingNotReadyPoll *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]
		var sampleNormalizerUsingPollWithValues *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]

		BeforeEach(func() {
			By("Creating resources")

			sampleNamespace = newNamespaceWithRandomName()
			Expect(k8sClient.create(ctx, sampleNamespace.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleNamespace.Generic())).To(Succeed())

			samplePoll = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: autoscalerv1alpha1.PrometheusPollSpec{
						PollerSpec: common.PollerSpec{
							ShardManagerRef: &corev1.TypedLocalObjectReference{
								Kind: "N/A",
								Name: "N/A",
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

			samplePollNotReady = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll-not-ready",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePoll.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					meta.SetStatusCondition(
						&container.Object.Status.Conditions,
						metav1.Condition{
							Type:    StatusTypeReady,
							Status:  metav1.ConditionFalse,
							Reason:  "NotReadyForTest",
							Message: "Intentionally not ready for testing purposes",
						},
					)
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePollWithValues = NewObjectContainer(
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-poll-with-values",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePoll.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.PrometheusPoll]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					container.Object.Status.Values = []common.MetricValue{
						{
							ID: "fake-metric-value",
							Shard: common.Shard{
								UID:  "fake-uid",
								ID:   "fake-id",
								Data: map[string]string{"fake-key": "fake-value"},
							},
							Query:        "fake-query",
							Value:        resource.MustParse("0"),
							DisplayValue: "0",
						},
					}
					meta.SetStatusCondition(
						&container.Object.Status.Conditions,
						metav1.Condition{
							Type:   StatusTypeReady,
							Status: metav1.ConditionTrue,
							Reason: StatusTypeReady,
						},
					)
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleNormalizer = NewObjectContainer(
				&autoscalerv1alpha1.RobustScalingNormalizer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-normalizer",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: autoscalerv1alpha1.RobustScalingNormalizerSpec{},
				},
				func(container *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]) {
					container.Object.Spec.MetricValuesProviderRef = &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("autoscaler.argoproj.io"),
						Kind:     "PrometheusPoll",
						Name:     samplePoll.ObjectKey.Name,
					}
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleNormalizerUsingNotReadyPoll = NewObjectContainer(
				&autoscalerv1alpha1.RobustScalingNormalizer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-normalizer-using-not-ready-poller",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleNormalizer.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]) {
					container.Object.Spec.MetricValuesProviderRef = &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("autoscaler.argoproj.io"),
						Kind:     "PrometheusPoll",
						Name:     samplePollNotReady.Object.Name,
					}
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleNormalizerUsingPollWithValues = NewObjectContainer(
				&autoscalerv1alpha1.RobustScalingNormalizer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-normalizer-using-poll-with-values",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleNormalizer.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]) {
					container.Object.Spec.MetricValuesProviderRef = &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("autoscaler.argoproj.io"),
						Kind:     "PrometheusPoll",
						Name:     samplePollWithValues.Object.Name,
					}
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
			CheckExitingOnNonExistingResource(func() *RobustScalingNormalizerReconciler {
				return &RobustScalingNormalizerReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
			})
		})

		It("should handle error getting resource", func() {
			By("Failing to get resource")
			CheckFailureToGetResource(
				k8sClient,
				sampleNormalizer,
				func(fClient *fakeClient) *RobustScalingNormalizerReconciler {
					return &RobustScalingNormalizerReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should handle errors when poller lookup fails", func() {
			By("Preparing reconciler")

			container := sampleNormalizer

			fClient := &fakeClient{
				Client: k8sClient,
			}
			fClient.
				WithGetFunction(samplePoll.Generic(),
					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return errors.New("fake error getting the poller")
					},
				)

			controllerReconciler := &RobustScalingNormalizerReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
			}

			By("Reconciling with the expectation to fails to update the status")
			CheckFailureToUpdateStatus(
				fClient,
				sampleNormalizer,
				func(fClient *fakeClient) *RobustScalingNormalizerReconciler {
					return &RobustScalingNormalizerReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)

			By("Reconciling")
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
			Expect(readyCondition.Reason).To(Equal("ErrorFindingMetricsProvider"))
			Expect(readyCondition.Message).To(ContainSubstring("fake error getting the poller"))
		})

		It("should exit if shard manager not ready", func() {
			By("Reconciling resource")

			container := sampleNormalizerUsingNotReadyPoll

			controllerReconciler := &RobustScalingNormalizerReconciler{
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
			Expect(readyCondition.Reason).To(Equal("MetricValuesProviderNotReady"))
			Expect(readyCondition.Message).To(ContainSubstring("Check the status of metric values provider"))
		})

		It("should handle errors updating status when poller not ready", func() {
			By("Reconciling resource")
			CheckFailureToUpdateStatus(
				k8sClient,
				sampleNormalizerUsingNotReadyPoll,
				func(fClient *fakeClient) *RobustScalingNormalizerReconciler {
					return &RobustScalingNormalizerReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should successfully normalize", func() {
			By("Preparing normalizer")

			container := sampleNormalizerUsingPollWithValues

			normalized := false
			controllerReconciler := &RobustScalingNormalizerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Normalizer: &fakeNormalizer{
					fn: func(ctx context.Context, values []common.MetricValue) ([]common.MetricValue, error) {
						normalizedValues := []common.MetricValue{}
						for _, value := range values {
							normalizedValues = append(normalizedValues, common.MetricValue{
								ID:           value.ID,
								Shard:        value.Shard,
								Query:        value.Query,
								Value:        resource.MustParse("1"),
								DisplayValue: "1",
							})
						}
						normalized = true
						return normalizedValues, nil
					},
				},
			}

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *RobustScalingNormalizerReconciler {
					return &RobustScalingNormalizerReconciler{
						Client:     fClient,
						Scheme:     fClient.Scheme(),
						Normalizer: controllerReconciler.Normalizer,
					}
				},
			)

			By("Reconciling resource")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(normalized).To(BeTrue())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			Expect(result.Requeue).To(BeFalse())

			By("Checking conditions")
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
			readyCondition := meta.FindStatusCondition(
				container.Object.Status.Conditions,
				StatusTypeReady,
			)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

			By("Checking normalization results")
			metricsById := map[string]common.MetricValue{}
			shardsByUID := map[types.UID]common.Shard{}
			for _, metric := range samplePollWithValues.Object.Status.Values {
				metricsById[metric.ID] = metric
				shardsByUID[metric.Shard.UID] = metric.Shard
			}
			normalizedValues := container.Object.Status.Values
			Expect(normalizedValues).To(HaveLen(len(samplePollWithValues.Object.Status.Values)))
			normalizedValuesByMetricIDAndShardUID := map[string]common.MetricValue{}
			for _, val := range normalizedValues {
				normalizedValuesByMetricIDAndShardUID[val.ID+":"+string(val.Shard.UID)] = val
			}
			for metricID, metric := range metricsById {
				for shardUID, shard := range shardsByUID {
					key := metricID + ":" + string(shardUID)
					val, ok := normalizedValuesByMetricIDAndShardUID[key]
					Expect(ok).To(BeTrue())
					Expect(val.ID).To(Equal(metric.ID))
					Expect(val.Shard).To(Equal(shard))
					Expect(val.Query).To(Equal(metric.Query))
					Expect(val.Value).To(Equal(resource.MustParse("1")))
					Expect(val.DisplayValue).To(Equal("1"))
				}
			}
		})

		It("should handle errors during normalizing", func() {
			By("Preparing normalizer")

			container := sampleNormalizerUsingPollWithValues

			controllerReconciler := &RobustScalingNormalizerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Normalizer: &fakeNormalizer{
					fn: func(ctx context.Context, values []common.MetricValue) ([]common.MetricValue, error) {
						return nil, errors.New("fake error")
					},
				},
			}

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *RobustScalingNormalizerReconciler {
					return &RobustScalingNormalizerReconciler{
						Client:     fClient,
						Scheme:     fClient.Scheme(),
						Normalizer: controllerReconciler.Normalizer,
					}
				},
			)

			By("Reconciling resource")
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
			Expect(readyCondition.Reason).To(Equal("ErrorDuringNormalization"))
			Expect(readyCondition.Message).To(ContainSubstring("fake error"))
		})
	})
})
