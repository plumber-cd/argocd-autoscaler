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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	. "github.com/plumber-cd/argocd-autoscaler/test/harness"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

type fakeNormalizer struct {
	fn         func(ctx context.Context, allMetrics []common.MetricValue) ([]common.MetricValue, error)
	normalized bool
}

func (f *fakeNormalizer) Normalize(ctx context.Context, allMetrics []common.MetricValue) ([]common.MetricValue, error) {
	f.normalized = true
	if f.fn != nil {
		return f.fn(ctx, allMetrics)
	}
	return nil, errors.New("fake normalizer not implemented")
}

var _ = Describe("PrometheusPoll Controller", func() {
	var scenarioRun GenericScenarioRun

	var normalizer *fakeNormalizer

	var collector = NewScenarioCollector[*autoscalerv1alpha1.RobustScalingNormalizer](
		func(fClient client.Client) *RobustScalingNormalizerReconciler {
			return &RobustScalingNormalizerReconciler{
				Client:     fClient,
				Scheme:     fClient.Scheme(),
				Normalizer: normalizer,
			}
		},
	)

	NewScenarioTemplate(
		"not existing resource",
		func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.RobustScalingNormalizer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "not-existent",
						Namespace: run.Namespace().ObjectKey().Name,
					},
				},
			)
			run.SetContainer(sampleNormalizer)
		},
	).
		BranchResourceNotFoundCheck(collector.Collect).
		BranchFailureToGetResourceCheck(collector.Collect)

	NewScenarioTemplate(
		"basic",
		func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.RobustScalingNormalizer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-robust-scaling-normalizer",
						Namespace: run.Namespace().ObjectKey().Name,
					},
					Spec: autoscalerv1alpha1.RobustScalingNormalizerSpec{
						NormalizerSpec: common.NormalizerSpec{
							MetricValuesProviderRef: &corev1.TypedLocalObjectReference{
								Kind: "N/A",
								Name: "N/A",
							},
						},
					},
				},
			).Create()
			run.SetContainer(sampleNormalizer)
		},
	).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle error during poll lookup",
			func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
				Expect(run.ReconcileError()).NotTo(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())

				By("Checking conditions")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("ErrorFindingMetricsProvider"))
				Expect(readyCondition.Message).To(ContainSubstring(`no matches for kind "N/A" in version ""`))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"sample poll",
			func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
				samplePoll := NewObjectContainer(
					run,
					&autoscalerv1alpha1.PrometheusPoll{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "sample-prometheus-poll",
							Namespace: run.Namespace().ObjectKey().Name,
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
				).Create()

				run.Container().Get().Object().Spec.NormalizerSpec = common.NormalizerSpec{
					MetricValuesProviderRef: &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To(samplePoll.GroupVersionKind().Group),
						Kind:     samplePoll.GroupVersionKind().Kind,
						Name:     samplePoll.ObjectKey().Name,
					},
				}
				run.Container().Update()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"do nothing if poll is not ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
				Expect(run.ReconcileError()).NotTo(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())

				By("Checking conditions")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("MetricValuesProviderNotReady"))
				Expect(readyCondition.Message).To(ContainSubstring("Check the status of metric values provider"))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"poll is ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
				samplePoll := NewObjectContainer(
					run,
					&autoscalerv1alpha1.PrometheusPoll{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "sample-prometheus-poll",
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()

				samplePoll.Object().Status.Values = []common.MetricValue{
					{
						ID: "fake-metric-value",
						Shard: common.Shard{
							UID:       "fake-uid",
							ID:        "fake-id",
							Namespace: run.Namespace().ObjectKey().Name,
							Name:      "fake-shard-name",
							Server:    "fake-shard-server",
						},
						Query:        "fake-query",
						Value:        resource.MustParse("0"),
						DisplayValue: "0",
					},
				}
				meta.SetStatusCondition(
					&samplePoll.Object().Status.Conditions,
					metav1.Condition{
						Type:   StatusTypeReady,
						Status: metav1.ConditionTrue,
						Reason: StatusTypeReady,
					},
				)
				samplePoll.StatusUpdate()
			},
		).
		Hydrate(
			"functioning normalizer",
			func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
				normalizer.fn = func(
					ctx context.Context,
					values []common.MetricValue,
				) ([]common.MetricValue, error) {
					normalizedValues := []common.MetricValue{}
					for _, value := range values {
						normalizedValues = append(normalizedValues, common.MetricValue{
							ID:           value.ID,
							Shard:        value.Shard,
							Query:        value.Query,
							Value:        resource.MustParse("0.5"),
							DisplayValue: "0.5",
						})
					}
					return normalizedValues, nil
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"normalize successfully",
			func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
				Expect(normalizer.normalized).To(BeTrue())
				Expect(run.ReconcileError()).ToNot(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())

				By("Checking conditions")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

				By("Checking normalization results")
				samplePoll := NewObjectContainer(
					run,
					&autoscalerv1alpha1.PrometheusPoll{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Object().Spec.NormalizerSpec.MetricValuesProviderRef.Name,
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()
				Expect(samplePoll.Object().Status.Values).ToNot(BeEmpty())
				metricsById := map[string]common.MetricValue{}
				shardsByUID := map[types.UID]common.Shard{}
				for _, metric := range samplePoll.Object().Status.Values {
					metricsById[metric.ID] = metric
					shardsByUID[metric.Shard.UID] = metric.Shard
				}
				normalizedValues := run.Container().Get().Object().Status.Values
				Expect(normalizedValues).To(HaveLen(len(samplePoll.Object().Status.Values)))
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
						Expect(val.Value.AsApproximateFloat64()).To(Equal(0.5))
						Expect(val.DisplayValue).To(Equal("0.5"))
					}
				}
			},
		).
		Commit(collector.Collect).
		RemoveLastHydration().
		Hydrate(
			"faulty normalizer",
			func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
				normalizer.fn = func(
					ctx context.Context,
					values []common.MetricValue,
				) ([]common.MetricValue, error) {
					return nil, errors.New("fake error")
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle errors",
			func(run *ScenarioRun[*autoscalerv1alpha1.RobustScalingNormalizer]) {
				Expect(run.ReconcileError()).NotTo(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())

				By("Checking conditions")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("ErrorDuringNormalization"))
				Expect(readyCondition.Message).To(ContainSubstring("fake error"))
			},
		).
		Commit(collector.Collect)

	BeforeEach(func() {
		normalizer = &fakeNormalizer{}
		scenarioRun = collector.NewRun(ctx, k8sClient)
	})

	AfterEach(func() {
		collector.Cleanup(scenarioRun)
		scenarioRun = nil
		normalizer = nil
	})

	for _, scenarioContext := range collector.All() {
		Context(scenarioContext.ContextStr, func() {
			for _, scenario := range scenarioContext.Its {
				It(scenario.ItStr, func() {
					scenario.ItFn(scenarioRun)
				})
			}
		})
	}
})
