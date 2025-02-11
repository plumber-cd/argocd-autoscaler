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
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/plumber-cd/argocd-autoscaler/test/harness"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

type fakeLoadIndexer struct {
	fn      func(int32, map[string]autoscalerv1alpha1.WeightedPNormLoadIndexWeight, []common.MetricValue) ([]common.LoadIndex, error)
	indexed bool
}

func (f *fakeLoadIndexer) Calculate(p int32, weights map[string]autoscalerv1alpha1.WeightedPNormLoadIndexWeight, values []common.MetricValue) ([]common.LoadIndex, error) {
	f.indexed = true
	if f.fn != nil {
		return f.fn(p, weights, values)
	}
	return nil, errors.New("fake error in fakeLoadIndexer")
}

var _ = Describe("WeightedPNormLoadIndex Controller", func() {
	var scenarioRun GenericScenarioRun

	var indexer *fakeLoadIndexer

	var collector = NewScenarioCollector[*autoscalerv1alpha1.WeightedPNormLoadIndex](
		func(fClient client.Client) *WeightedPNormLoadIndexReconciler {
			return &WeightedPNormLoadIndexReconciler{
				Client:      fClient,
				Scheme:      fClient.Scheme(),
				LoadIndexer: indexer,
			}
		},
	)

	NewScenarioTemplate(
		"not existing resource",
		func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.WeightedPNormLoadIndex{
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
		func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.WeightedPNormLoadIndex{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-weighted-p-norm-load-index",
						Namespace: run.Namespace().ObjectKey().Name,
					},
					Spec: autoscalerv1alpha1.WeightedPNormLoadIndexSpec{
						LoadIndexerSpec: common.LoadIndexerSpec{
							MetricValuesProviderRef: &corev1.TypedLocalObjectReference{
								Kind: "N/A",
								Name: "N/A",
							},
						},
						P: 3,
						Weights: []autoscalerv1alpha1.WeightedPNormLoadIndexWeight{
							{
								ID:     "fake-metric",
								Weight: resource.MustParse("1"),
							},
						},
					},
				},
			).Create()
			run.SetContainer(sampleNormalizer)
		},
	).
		Hydrate(
			"malformed duplicate weights",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
				sampleNormalizer := NewObjectContainer(
					run,
					&autoscalerv1alpha1.WeightedPNormLoadIndex{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "sample-weighted-p-norm-load-index",
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()
				sampleNormalizer.Object().Spec.Weights = append(
					sampleNormalizer.Object().Spec.Weights,
					sampleNormalizer.Object().Spec.Weights[0],
				)
				sampleNormalizer.Update()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle error during poll lookup",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
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
				Expect(readyCondition.Reason).To(Equal("WeightsMalformed"))
				Expect(readyCondition.Message).To(Equal("duplicate weight configuration for metric ID 'fake-metric'"))
			},
		).
		Commit(collector.Collect).
		RemoveLastHydration().
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle error during metric provider lookup",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
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
				Expect(readyCondition.Reason).To(Equal("ErrorFindingMetricProvider"))
				Expect(readyCondition.Message).To(ContainSubstring(`no matches for kind "N/A" in version ""`))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"sample metric values provider",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
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

				run.Container().Get().Object().Spec.MetricValuesProviderRef = &corev1.TypedLocalObjectReference{
					APIGroup: ptr.To(sampleNormalizer.GroupVersionKind().Group),
					Kind:     sampleNormalizer.GroupVersionKind().Kind,
					Name:     sampleNormalizer.ObjectKey().Name,
				}
				run.Container().Update()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"do nothing if metric values provider is not ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
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
			"metric values provider is ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
				sampleNormalizer := NewObjectContainer(
					run,
					&autoscalerv1alpha1.RobustScalingNormalizer{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Object().Spec.MetricValuesProviderRef.Name,
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()

				sampleNormalizer.Object().Status.Values = []common.MetricValue{
					{
						ID: "fake-metric",
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
					&sampleNormalizer.Object().Status.Conditions,
					metav1.Condition{
						Type:   StatusTypeReady,
						Status: metav1.ConditionTrue,
						Reason: StatusTypeReady,
					},
				)
				sampleNormalizer.StatusUpdate()
			},
		).
		Hydrate(
			"functioning load indexer",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
				indexer.fn = func(
					p int32,
					weights map[string]autoscalerv1alpha1.WeightedPNormLoadIndexWeight,
					values []common.MetricValue,
				) ([]common.LoadIndex, error) {
					loadIndexes := []common.LoadIndex{}
					for _, value := range values {
						loadIndexes = append(loadIndexes, common.LoadIndex{
							Shard:        value.Shard,
							Value:        resource.MustParse("1"),
							DisplayValue: "1",
						})
					}
					return loadIndexes, nil
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"load index successfully",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
				Expect(indexer.indexed).To(BeTrue())
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

				By("Checking load indexing results")
				sampleNormalizer := NewObjectContainer(
					run,
					&autoscalerv1alpha1.RobustScalingNormalizer{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Object().Spec.MetricValuesProviderRef.Name,
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()
				shardsByUID := map[types.UID]common.Shard{}
				for _, metric := range sampleNormalizer.Object().Status.Values {
					shardsByUID[metric.Shard.UID] = metric.Shard
				}
				loadIndexes := run.Container().Get().Object().Status.Values
				Expect(loadIndexes).To(HaveLen(len(shardsByUID)))
				normalizedValuesByShardUID := map[types.UID]common.LoadIndex{}
				for _, val := range loadIndexes {
					normalizedValuesByShardUID[val.Shard.UID] = val
				}
				for shardUID, shard := range shardsByUID {
					val, ok := normalizedValuesByShardUID[shardUID]
					Expect(ok).To(BeTrue())
					Expect(val.Shard).To(Equal(shard))
					Expect(val.Value).To(Equal(resource.MustParse("1")))
					Expect(val.DisplayValue).To(Equal("1"))
				}
			},
		).
		Commit(collector.Collect).
		RemoveLastHydration().
		Hydrate(
			"faulty normalizer",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
				indexer.fn = func(
					p int32,
					weights map[string]autoscalerv1alpha1.WeightedPNormLoadIndexWeight,
					values []common.MetricValue,
				) ([]common.LoadIndex, error) {
					return nil, errors.New("fake error")
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle errors",
			func(run *ScenarioRun[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
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
				Expect(readyCondition.Reason).To(Equal("LoadIndexCalculationError"))
				Expect(readyCondition.Message).To(ContainSubstring("fake error"))
			},
		).
		Commit(collector.Collect)

	BeforeEach(func() {
		indexer = &fakeLoadIndexer{}
		scenarioRun = collector.NewRun(ctx, k8sClient)
	})

	AfterEach(func() {
		collector.Cleanup(scenarioRun)
		scenarioRun = nil
		indexer = nil
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
