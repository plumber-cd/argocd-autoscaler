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

var _ = Describe("MostWantedTwoPhaseHysteresisEvaluation Controller", func() {
	var scenarioRun GenericScenarioRun

	var collector = NewScenarioCollector[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation](
		func(fClient client.Client) *MostWantedTwoPhaseHysteresisEvaluationReconciler {
			return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
			}
		},
	)

	NewScenarioTemplate(
		"not existing resource",
		func(run *ScenarioRun[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation{
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
		func(run *ScenarioRun[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-most-wanted-two-phase-hysteresis-evaluation",
						Namespace: run.Namespace().ObjectKey().Name,
					},
					Spec: autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationSpec{
						EvaluatorSpec: common.EvaluatorSpec{
							PartitionProviderRef: &corev1.TypedLocalObjectReference{
								Kind: "N/A",
								Name: "N/A",
							},
						},
						PollingPeriod:       metav1.Duration{Duration: time.Minute},
						StabilizationPeriod: metav1.Duration{Duration: time.Hour},
						MinimumSampleSize:   5,
					},
				},
			).Create()
			run.SetContainer(sampleNormalizer)
		},
	).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle error during partition lookup",
			func(run *ScenarioRun[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
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
				Expect(readyCondition.Reason).To(Equal("ErrorFindingPartitionProvider"))
				Expect(readyCondition.Message).To(ContainSubstring(`no matches for kind "N/A" in version ""`))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"sample partition provider",
			func(run *ScenarioRun[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
				samplePartition := NewObjectContainer(
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

				run.Container().Get().Object().Spec.PartitionProviderRef = &corev1.TypedLocalObjectReference{
					APIGroup: ptr.To(samplePartition.GroupVersionKind().Group),
					Kind:     samplePartition.GroupVersionKind().Kind,
					Name:     samplePartition.ObjectKey().Name,
				}
				run.Container().Update()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"do nothing if partition provider is not ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
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
				Expect(readyCondition.Reason).To(Equal("PartitionProviderNotReady"))
				Expect(readyCondition.Message).To(ContainSubstring("Check the status of a partition provider"))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"partition provider is ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
				samplePartition := NewObjectContainer(
					run,
					&autoscalerv1alpha1.LongestProcessingTimePartition{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()

				samplePartition.Object().Status.Replicas = common.ReplicaList{
					{
						ID: "0",
						LoadIndexes: []common.LoadIndex{
							{
								Shard: common.Shard{
									UID:  types.UID("shard-0"),
									ID:   "shard-0",
									Data: map[string]string{"key": "value"},
								},
								Value:        resource.MustParse("1"),
								DisplayValue: "1",
							},
						},
						TotalLoad:             resource.MustParse("1"),
						TotalLoadDisplayValue: "1",
					},
				}
				meta.SetStatusCondition(
					&samplePartition.Object().Status.Conditions,
					metav1.Condition{
						Type:   StatusTypeReady,
						Status: metav1.ConditionTrue,
						Reason: StatusTypeReady,
					},
				)
				samplePartition.StatusUpdate()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"bail out on minimum sample size not reached",
			func(run *ScenarioRun[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
				samplePartition := NewObjectContainer(
					run,
					&autoscalerv1alpha1.LongestProcessingTimePartition{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				)
				for i := 0; i < int(run.Container().Object().Spec.MinimumSampleSize-1); i++ {
					Expect(run.ReconcileError()).ToNot(HaveOccurred())
					Expect(run.ReconcileResult().RequeueAfter).
						To(Equal(run.Container().Object().Spec.PollingPeriod.Duration))
					Expect(run.ReconcileResult().Requeue).To(BeFalse())

					By("Checking conditions")
					readyCondition := meta.FindStatusCondition(
						run.Container().Get().Object().Status.Conditions,
						StatusTypeReady,
					)
					Expect(readyCondition).NotTo(BeNil())
					Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
					Expect(readyCondition.Reason).To(Equal("MinimumSampleSizeNotReached"))
					Expect(readyCondition.Message).To(ContainSubstring("Minimum sample size not reached"))

					By("Checking history records")
					samplePartition.Get()
					history := run.Container().Object().Status.History
					Expect(history).To(HaveLen(i + 1))
					record := history[i]
					Expect(record.Replicas).To(Equal(samplePartition.Object().Status.Replicas))

					By("Reconciling resource sample #" + fmt.Sprintf("%d", i+2))
					run.Reconcile()
				}

				Expect(run.ReconcileError()).ToNot(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).
					To(Equal(run.Container().Object().Spec.PollingPeriod.Duration))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())

				By("Checking conditions")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

				By("Checking history records")
				samplePartition.Get()
				history := run.Container().Object().Status.History
				Expect(history).To(HaveLen(int(run.Container().Object().Spec.MinimumSampleSize)))
				record := history[int(run.Container().Object().Spec.MinimumSampleSize)-1]
				Expect(record.Replicas).To(Equal(samplePartition.Object().Status.Replicas))

				By("Checking evaluation results")
				Expect(run.Container().Object().Status.LastEvaluationTimestamp.Time).To(BeTemporally("~", time.Now(), 2*time.Second))
				Expect(run.Container().Object().Status.Projection).To(Equal(samplePartition.Object().Status.Replicas))
				Expect(run.Container().Object().Status.Replicas).To(Equal(samplePartition.Object().Status.Replicas))

				By("Simulate stabilization period expiring")
				run.Container().Object().Status.LastEvaluationTimestamp = &metav1.Time{
					Time: time.Now().Add(-run.Container().Object().Spec.StabilizationPeriod.Duration).
						Add(-time.Second),
				}
				for i := range run.Container().Object().Status.History {
					run.Container().Object().Status.History[i].Timestamp = *run.Container().Object().Status.LastEvaluationTimestamp
				}
				run.Container().StatusUpdate()

				for i := 0; i < int(run.Container().Object().Spec.MinimumSampleSize-1); i++ {
					By("Reconciling resource sample #" + fmt.Sprintf("%d", i+1))
					run.Reconcile()

					Expect(run.ReconcileError()).ToNot(HaveOccurred())
					Expect(run.ReconcileResult().RequeueAfter).
						To(Equal(run.Container().Object().Spec.PollingPeriod.Duration))
					Expect(run.ReconcileResult().Requeue).To(BeFalse())

					By("Checking conditions")
					readyCondition := meta.FindStatusCondition(
						run.Container().Get().Object().Status.Conditions,
						StatusTypeReady,
					)
					Expect(readyCondition).NotTo(BeNil())
					Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
					Expect(readyCondition.Reason).To(Equal("MinimumSampleSizeNotReached"))
					Expect(readyCondition.Message).To(ContainSubstring("Minimum sample size not reached"))

					By("Checking history records")
					samplePartition.Get()
					history := run.Container().Object().Status.History
					Expect(history).To(HaveLen(i + 1))
					record := history[i]
					Expect(record.Replicas).To(Equal(samplePartition.Object().Status.Replicas))
				}

				By("Reconciling resource sample #" + fmt.Sprintf("%d", run.Container().Object().Spec.MinimumSampleSize))
				run.Reconcile()

				Expect(run.ReconcileError()).ToNot(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).
					To(Equal(run.Container().Object().Spec.PollingPeriod.Duration))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())

				By("Checking conditions")
				readyCondition = meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

				By("Checking history records")
				samplePartition.Get()
				history = run.Container().Object().Status.History
				Expect(history).To(HaveLen(int(run.Container().Object().Spec.MinimumSampleSize)))
				record = history[int(run.Container().Object().Spec.MinimumSampleSize)-1]
				Expect(record.Replicas).To(Equal(samplePartition.Object().Status.Replicas))

				By("Checking evaluation results")
				Expect(run.Container().Object().Status.LastEvaluationTimestamp.Time).To(BeTemporally("~", time.Now(), 2*time.Second))
				Expect(run.Container().Object().Status.Projection).To(Equal(samplePartition.Object().Status.Replicas))
				Expect(run.Container().Object().Status.Replicas).To(Equal(samplePartition.Object().Status.Replicas))
			},
		).
		Commit(collector.Collect)

	// 	Context("When reconciling a resource", func() {
	// 		BeforeEach(func() {
	// 		It("should bail out on minimum sample size not reached", func() {
	// 			By("Preparing reconciler")
	//
	// 			container := sampleEvaluationUsingReadyPartition
	//
	// 			controllerReconciler := &MostWantedTwoPhaseHysteresisEvaluationReconciler{
	// 				Client: k8sClient,
	// 				Scheme: k8sClient.Scheme(),
	// 			}
	//
	// 			By("Reconciling resource with expectation to fail to update status")
	// 			CheckFailureToUpdateStatus(
	// 				k8sClient,
	// 				container,
	// 				func(fClient *fakeClient) *MostWantedTwoPhaseHysteresisEvaluationReconciler {
	// 					return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
	// 						Client: fClient,
	// 						Scheme: fClient.Scheme(),
	// 					}
	// 				},
	// 			)
	//
	// 			for i := 0; i < int(container.Object.Spec.MinimumSampleSize-1); i++ {
	// 				By("Reconciling resource sample #" + fmt.Sprintf("%d", i+1))
	// 				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
	// 					NamespacedName: container.NamespacedName,
	// 				})
	// 				Expect(err).ToNot(HaveOccurred())
	// 				Expect(result.RequeueAfter).To(Equal(container.Object.Spec.PollingPeriod.Duration))
	// 				Expect(result.Requeue).To(BeFalse())
	//
	// 				By("Checking conditions")
	// 				Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
	// 				readyCondition := meta.FindStatusCondition(
	// 					container.Object.Status.Conditions,
	// 					StatusTypeReady,
	// 				)
	// 				Expect(readyCondition).NotTo(BeNil())
	// 				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
	// 				Expect(readyCondition.Reason).To(Equal("MinimumSampleSizeNotReached"))
	// 				Expect(readyCondition.Message).To(ContainSubstring("Minimum sample size not reached"))
	//
	// 				By("Checking history records")
	// 				history := container.Object.Status.History
	// 				Expect(history).To(HaveLen(i + 1))
	// 				record := history[i]
	// 				Expect(record.Replicas).To(Equal(samplePartitionReady.Object.Status.Replicas))
	// 			}
	//
	// 			By("Reconciling resource sample #" + fmt.Sprintf("%d", container.Object.Spec.MinimumSampleSize) + " (min reached)")
	//
	// 			By("Reconciling resource with expectation to fail to update status")
	// 			CheckFailureToUpdateStatus(
	// 				k8sClient,
	// 				container,
	// 				func(fClient *fakeClient) *MostWantedTwoPhaseHysteresisEvaluationReconciler {
	// 					return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
	// 						Client: fClient,
	// 						Scheme: fClient.Scheme(),
	// 					}
	// 				},
	// 			)
	//
	// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
	// 				NamespacedName: container.NamespacedName,
	// 			})
	// 			Expect(err).ToNot(HaveOccurred())
	// 			Expect(result.RequeueAfter).To(Equal(container.Object.Spec.PollingPeriod.Duration))
	// 			Expect(result.Requeue).To(BeFalse())
	//
	// 			By("Checking conditions")
	// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
	// 			readyCondition := meta.FindStatusCondition(
	// 				container.Object.Status.Conditions,
	// 				StatusTypeReady,
	// 			)
	// 			Expect(readyCondition).NotTo(BeNil())
	// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
	// 			Expect(readyCondition.Reason).To(Equal(StatusTypeReady))
	//
	// 			By("Checking history records")
	// 			history := container.Object.Status.History
	// 			Expect(history).To(HaveLen(int(container.Object.Spec.MinimumSampleSize)))
	// 			record := history[int(container.Object.Spec.MinimumSampleSize)-1]
	// 			Expect(record.Replicas).To(Equal(samplePartitionReady.Object.Status.Replicas))
	//
	// 			By("Checking evaluation results")
	// 			Expect(container.Object.Status.LastEvaluationTimestamp.Time).To(BeTemporally("~", time.Now(), 2*time.Second))
	// 			Expect(container.Object.Status.Projection).To(Equal(samplePartitionReady.Object.Status.Replicas))
	// 			Expect(container.Object.Status.Replicas).To(Equal(samplePartitionReady.Object.Status.Replicas))
	//
	// 			By("Simulate stabilization period expiring")
	// 			container.Object.Status.LastEvaluationTimestamp = &metav1.Time{
	// 				Time: time.Now().Add(-container.Object.Spec.StabilizationPeriod.Duration).
	// 					Add(-time.Second),
	// 			}
	// 			for i := range container.Object.Status.History {
	// 				container.Object.Status.History[i].Timestamp = *container.Object.Status.LastEvaluationTimestamp
	// 			}
	// 			Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
	// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
	//
	// 			for i := 0; i < int(container.Object.Spec.MinimumSampleSize-1); i++ {
	// 				By("Reconciling resource sample (second pass) #" + fmt.Sprintf("%d", i+1))
	// 				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
	// 					NamespacedName: container.NamespacedName,
	// 				})
	// 				Expect(err).ToNot(HaveOccurred())
	// 				Expect(result.RequeueAfter).To(Equal(container.Object.Spec.PollingPeriod.Duration))
	// 				Expect(result.Requeue).To(BeFalse())
	//
	// 				By("Checking conditions")
	// 				Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
	// 				readyCondition := meta.FindStatusCondition(
	// 					container.Object.Status.Conditions,
	// 					StatusTypeReady,
	// 				)
	// 				Expect(readyCondition).NotTo(BeNil())
	// 				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
	// 				Expect(readyCondition.Reason).To(Equal("MinimumSampleSizeNotReached"))
	// 				Expect(readyCondition.Message).To(ContainSubstring("Minimum sample size not reached"))
	//
	// 				By("Checking history records")
	// 				history := container.Object.Status.History
	// 				Expect(history).To(HaveLen(i + 1))
	// 				record := history[i]
	// 				Expect(record.Replicas).To(Equal(samplePartitionReady.Object.Status.Replicas))
	// 			}
	// 		})
	// 	})

	BeforeEach(func() {
		scenarioRun = collector.NewRun(ctx, k8sClient)
	})

	AfterEach(func() {
		collector.Cleanup(scenarioRun)
		scenarioRun = nil
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
