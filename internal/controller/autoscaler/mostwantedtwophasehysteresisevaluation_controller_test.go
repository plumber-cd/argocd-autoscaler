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
	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("MostWantedTwoPhaseHysteresisEvaluation Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		var sampleNamespace *objectContainer[*corev1.Namespace]

		var samplePartition *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]
		var samplePartitionNotReady *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]
		var samplePartitionReady *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]

		var sampleEvaluation *objectContainer[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]
		var sampleEvaluationUsingNotReadyPartition *objectContainer[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]
		var sampleEvaluationUsingReadyPartition *objectContainer[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]

		BeforeEach(func() {
			By("Creating resources")

			sampleNamespace = newNamespaceWithRandomName()
			Expect(k8sClient.create(ctx, sampleNamespace.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleNamespace.Generic())).To(Succeed())

			samplePartition = NewObjectContainer(
				&autoscalerv1alpha1.LongestProcessingTimePartition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-partition",
						Namespace: sampleNamespace.ObjectKey.Name,
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
				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePartitionNotReady = NewObjectContainer(
				&autoscalerv1alpha1.LongestProcessingTimePartition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-partition-not-ready",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePartition.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					meta.SetStatusCondition(
						&container.Object.Status.Conditions,
						metav1.Condition{
							Type:    StatusTypeReady,
							Status:  metav1.ConditionFalse,
							Reason:  "LoadIndexProviderNotReady",
							Message: "Purposefully not ready for testing",
						},
					)
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePartitionReady = NewObjectContainer(
				&autoscalerv1alpha1.LongestProcessingTimePartition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-partition-ready",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePartition.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					container.Object.Status.Replicas = common.ReplicaList{
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

			sampleEvaluation = NewObjectContainer(
				&autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-evaluation",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluationSpec{
						EvaluatorSpec: common.EvaluatorSpec{
							PartitionProviderRef: &corev1.TypedLocalObjectReference{
								APIGroup: ptr.To("autoscaler.argoproj.io"),
								Kind:     "LongestProcessingTimePartition",
								Name:     samplePartition.Object.Name,
							},
						},
						PollingPeriod:       metav1.Duration{Duration: time.Minute},
						StabilizationPeriod: metav1.Duration{Duration: time.Minute},
						MinimumSampleSize:   int32(10),
					},
				},
				func(container *objectContainer[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleEvaluationUsingNotReadyPartition = NewObjectContainer(
				&autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-evaluation-using-not-ready-partitioner",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleEvaluation.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
					container.Object.Spec.PartitionProviderRef.Name = samplePartitionNotReady.Object.Name
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleEvaluationUsingReadyPartition = NewObjectContainer(
				&autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-evaluation-using-ready-partitioner",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleEvaluation.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.MostWantedTwoPhaseHysteresisEvaluation]) {
					container.Object.Spec.PartitionProviderRef.Name = samplePartitionReady.Object.Name
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
			CheckExitingOnNonExistingResource(func() *MostWantedTwoPhaseHysteresisEvaluationReconciler {
				return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
			})
		})

		It("should handle error getting resource", func() {
			By("Failing to get resource")
			CheckFailureToGetResource(
				k8sClient,
				sampleEvaluation,
				func(fClient *fakeClient) *MostWantedTwoPhaseHysteresisEvaluationReconciler {
					return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should handle errors when partition provider lookup fails", func() {
			By("Preparing reconciler")

			container := sampleEvaluation

			fClient := &fakeClient{
				Client: k8sClient,
			}
			fClient.
				WithGetFunction(samplePartition.Generic(),
					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return errors.New("fake error getting partition provider")
					},
				)

			controllerReconciler := &MostWantedTwoPhaseHysteresisEvaluationReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
			}

			By("Reconciling with the expectation to fails to update the status")
			CheckFailureToUpdateStatus(
				fClient,
				sampleEvaluation,
				func(fClient *fakeClient) *MostWantedTwoPhaseHysteresisEvaluationReconciler {
					return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
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
			Expect(readyCondition.Reason).To(Equal("ErrorFindingPartitionProvider"))
			Expect(readyCondition.Message).To(ContainSubstring("fake error getting partition provider"))
		})

		It("should exit if partition provider is not ready", func() {
			By("Preparing reconciler")

			container := sampleEvaluationUsingNotReadyPartition

			controllerReconciler := &MostWantedTwoPhaseHysteresisEvaluationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling resource with expectation to fail updating status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *MostWantedTwoPhaseHysteresisEvaluationReconciler {
					return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
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
			Expect(readyCondition.Reason).To(Equal("PartitionProviderNotReady"))
			Expect(readyCondition.Message).To(ContainSubstring("Check the status of a partition provider"))
		})

		It("should bail out on minimum sample size not reached", func() {
			By("Preparing reconciler")

			container := sampleEvaluationUsingReadyPartition

			controllerReconciler := &MostWantedTwoPhaseHysteresisEvaluationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *MostWantedTwoPhaseHysteresisEvaluationReconciler {
					return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)

			for i := 0; i < int(container.Object.Spec.MinimumSampleSize-1); i++ {
				By("Reconciling resource sample #" + fmt.Sprintf("%d", i+1))
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: container.NamespacedName,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(container.Object.Spec.PollingPeriod.Duration))
				Expect(result.Requeue).To(BeFalse())

				By("Checking conditions")
				Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				readyCondition := meta.FindStatusCondition(
					container.Object.Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("MinimumSampleSizeNotReached"))
				Expect(readyCondition.Message).To(ContainSubstring("Minimum sample size not reached"))

				By("Checking history records")
				history := container.Object.Status.History
				Expect(history).To(HaveLen(i + 1))
				record := history[i]
				Expect(record.Replicas).To(Equal(samplePartitionReady.Object.Status.Replicas))
			}

			By("Reconciling resource sample #" + fmt.Sprintf("%d", container.Object.Spec.MinimumSampleSize) + " (min reached)")

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *MostWantedTwoPhaseHysteresisEvaluationReconciler {
					return &MostWantedTwoPhaseHysteresisEvaluationReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(container.Object.Spec.PollingPeriod.Duration))
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

			By("Checking history records")
			history := container.Object.Status.History
			Expect(history).To(HaveLen(int(container.Object.Spec.MinimumSampleSize)))
			record := history[int(container.Object.Spec.MinimumSampleSize)-1]
			Expect(record.Replicas).To(Equal(samplePartitionReady.Object.Status.Replicas))

			By("Checking evaluation results")
			Expect(container.Object.Status.LastEvaluationTimestamp.Time).To(BeTemporally("~", time.Now(), 2*time.Second))
			Expect(container.Object.Status.Projection).To(Equal(samplePartitionReady.Object.Status.Replicas))
			Expect(container.Object.Status.Replicas).To(Equal(samplePartitionReady.Object.Status.Replicas))

			By("Simulate stabilization period expiring")
			container.Object.Status.LastEvaluationTimestamp = &metav1.Time{
				Time: time.Now().Add(-container.Object.Spec.StabilizationPeriod.Duration).
					Add(-time.Second),
			}
			for i := range container.Object.Status.History {
				container.Object.Status.History[i].Timestamp = *container.Object.Status.LastEvaluationTimestamp
			}
			Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())

			for i := 0; i < int(container.Object.Spec.MinimumSampleSize-1); i++ {
				By("Reconciling resource sample (second pass) #" + fmt.Sprintf("%d", i+1))
				result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: container.NamespacedName,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(container.Object.Spec.PollingPeriod.Duration))
				Expect(result.Requeue).To(BeFalse())

				By("Checking conditions")
				Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				readyCondition := meta.FindStatusCondition(
					container.Object.Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("MinimumSampleSizeNotReached"))
				Expect(readyCondition.Message).To(ContainSubstring("Minimum sample size not reached"))

				By("Checking history records")
				history := container.Object.Status.History
				Expect(history).To(HaveLen(i + 1))
				record := history[i]
				Expect(record.Replicas).To(Equal(samplePartitionReady.Object.Status.Replicas))
			}
		})
	})
})
