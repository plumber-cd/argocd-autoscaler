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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/plumber-cd/argocd-autoscaler/test/harness"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

type fakePartitioner struct {
	partitioned bool
	fn          func(ctx context.Context, shards []common.LoadIndex) (common.ReplicaList, error)
}

func (f *fakePartitioner) Partition(ctx context.Context, shards []common.LoadIndex) (common.ReplicaList, error) {
	f.partitioned = true
	if f.fn != nil {
		return f.fn(ctx, shards)
	}
	return nil, errors.New("fake partitioner not implemented")
}

var _ = Describe("LongestProcessingTimePartition Controller", func() {
	var scenarioRun GenericScenarioRun

	var partitioner *fakePartitioner

	var collector = NewScenarioCollector[*autoscalerv1alpha1.LongestProcessingTimePartition](
		func(fClient client.Client) *LongestProcessingTimePartitionReconciler {
			return &LongestProcessingTimePartitionReconciler{
				Client:      fClient,
				Scheme:      fClient.Scheme(),
				Partitioner: partitioner,
			}
		},
	)

	NewScenarioTemplate(
		"not existing resource",
		func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.LongestProcessingTimePartition{
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
		func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.LongestProcessingTimePartition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-longest-processing-time-partition",
						Namespace: run.Namespace().ObjectKey().Name,
					},
					Spec: autoscalerv1alpha1.LongestProcessingTimePartitionSpec{
						PartitionerSpec: common.PartitionerSpec{
							LoadIndexProviderRef: &corev1.TypedLocalObjectReference{
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
			"handle error during load index lookup",
			func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
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
				Expect(readyCondition.Reason).To(Equal("ErrorFindingLoadIndexProvider"))
				Expect(readyCondition.Message).To(ContainSubstring(`no matches for kind "N/A" in version ""`))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"sample load index provider",
			func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
				sampleLoadIndex := NewObjectContainer(
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
							P:       1,
							OffsetE: resource.MustParse("0.01"),
							Weights: []autoscalerv1alpha1.WeightedPNormLoadIndexWeight{
								{
									ID:     "fake-metric",
									Weight: resource.MustParse("1"),
								},
							},
						},
					},
				).Create()

				run.Container().Get().Object().Spec.LoadIndexProviderRef = &corev1.TypedLocalObjectReference{
					APIGroup: ptr.To(sampleLoadIndex.GroupVersionKind().Group),
					Kind:     sampleLoadIndex.GroupVersionKind().Kind,
					Name:     sampleLoadIndex.ObjectKey().Name,
				}
				run.Container().Update()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"do nothing if load index provider is not ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
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
				Expect(readyCondition.Reason).To(Equal("LoadIndexProviderNotReady"))
				Expect(readyCondition.Message).To(ContainSubstring("Check the status of a load index provider"))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"load index provider is ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
				sampleLoadIndex := NewObjectContainer(
					run,
					&autoscalerv1alpha1.WeightedPNormLoadIndex{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Object().Spec.LoadIndexProviderRef.Name,
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()

				sampleLoadIndex.Object().Status.Values = []common.LoadIndex{
					{
						Shard: common.Shard{
							UID:       "fake-shard-uid",
							ID:        "fake-shard-id",
							Namespace: run.Namespace().ObjectKey().Name,
							Name:      "fake-shard-name",
							Server:    "fake-server",
						},
						Value:        resource.MustParse("1"),
						DisplayValue: "1",
					},
				}
				meta.SetStatusCondition(
					&sampleLoadIndex.Object().Status.Conditions,
					metav1.Condition{
						Type:   StatusTypeReady,
						Status: metav1.ConditionTrue,
						Reason: StatusTypeReady,
					},
				)
				sampleLoadIndex.StatusUpdate()
			},
		).
		Hydrate(
			"functioning partitioner",
			func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
				partitioner.fn = func(
					ctx context.Context,
					shards []common.LoadIndex,
				) (common.ReplicaList, error) {
					replicas := common.ReplicaList{}
					for i, shard := range shards {
						replicas = append(replicas, common.Replica{
							ID:                    fmt.Sprintf("%d", i),
							LoadIndexes:           []common.LoadIndex{shard},
							TotalLoad:             shard.Value,
							TotalLoadDisplayValue: shard.DisplayValue,
						})
					}
					return replicas, nil
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"partition successfully",
			func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
				Expect(partitioner.partitioned).To(BeTrue())
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

				By("Checking partition results")
				sampleLoadIndex := NewObjectContainer(
					run,
					&autoscalerv1alpha1.WeightedPNormLoadIndex{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Object().Spec.LoadIndexProviderRef.Name,
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()
				shardsByUID := map[types.UID]common.LoadIndex{}
				for _, metric := range sampleLoadIndex.Object().Status.Values {
					shardsByUID[metric.Shard.UID] = metric
				}
				replicas := run.Container().Object().Status.Replicas
				Expect(replicas).To(HaveLen(len(shardsByUID)))
				replicasByShardUID := map[types.UID]common.Replica{}
				for _, replica := range replicas {
					Expect(replica.LoadIndexes).To(HaveLen(1))
					replicasByShardUID[replica.LoadIndexes[0].Shard.UID] = replica
				}
				for shardUID, shard := range shardsByUID {
					replica, ok := replicasByShardUID[shardUID]
					Expect(ok).To(BeTrue())
					Expect(replica.TotalLoad).To(Equal(shard.Value))
					Expect(replica.TotalLoadDisplayValue).To(Equal(shard.DisplayValue))
				}
			},
		).
		Commit(collector.Collect).
		RemoveLastHydration().
		Hydrate(
			"faulty partitioner",
			func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
				partitioner.fn = func(
					ctx context.Context,
					shards []common.LoadIndex,
				) (common.ReplicaList, error) {
					return nil, errors.New("fake error")
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle errors",
			func(run *ScenarioRun[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
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
				Expect(readyCondition.Reason).To(Equal("ErrorDuringPartitioning"))
				Expect(readyCondition.Message).To(ContainSubstring("fake error"))
			},
		).
		Commit(collector.Collect)

	BeforeEach(func() {
		partitioner = &fakePartitioner{}
		scenarioRun = collector.NewRun(ctx, k8sClient)
	})

	AfterEach(func() {
		collector.Cleanup(scenarioRun)
		scenarioRun = nil
		partitioner = nil
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
