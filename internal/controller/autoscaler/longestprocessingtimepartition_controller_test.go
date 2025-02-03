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

type fakePartitioner struct {
	fn func(ctx context.Context, shards []common.LoadIndex) (common.ReplicaList, error)
}

func (f *fakePartitioner) Partition(ctx context.Context, shards []common.LoadIndex) (common.ReplicaList, error) {
	if f.fn != nil {
		return f.fn(ctx, shards)
	}
	return nil, errors.New("fake partitioner not implemented")
}

var _ = Describe("LongestProcessingTimePartition Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		var sampleNamespace *objectContainer[*corev1.Namespace]

		var sampleLoadIndex *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]
		var sampleLoadIndexNotReady *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]
		var sampleLoadIndexWithValues *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]

		var samplePartition *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]
		var samplePartitionUsingNotReadyNormalizer *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]
		var samplePartitionUsingNormalizerWithValues *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]

		BeforeEach(func() {
			By("Creating resources")

			sampleNamespace = newNamespaceWithRandomName()
			Expect(k8sClient.create(ctx, sampleNamespace.Generic())).To(Succeed())
			Expect(k8sClient.get(ctx, sampleNamespace.Generic())).To(Succeed())

			sampleLoadIndex = NewObjectContainer(
				&autoscalerv1alpha1.WeightedPNormLoadIndex{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-load-index",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: autoscalerv1alpha1.WeightedPNormLoadIndexSpec{
						LoadIndexerSpec: common.LoadIndexerSpec{
							MetricValuesProviderRef: &corev1.TypedLocalObjectReference{
								Kind: "N/A",
								Name: "N/A",
							},
						},
						P: 2,
						Weights: []autoscalerv1alpha1.WeightedPNormLoadIndexWeight{
							{
								ID:       "fake-metric",
								Weight:   resource.MustParse("1"),
								Negative: ptr.To(true),
							},
						},
					},
				},
				func(container *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleLoadIndexNotReady = NewObjectContainer(
				&autoscalerv1alpha1.WeightedPNormLoadIndex{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-load-index-not-ready",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleLoadIndex.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					meta.SetStatusCondition(
						&container.Object.Status.Conditions,
						metav1.Condition{
							Type:    StatusTypeReady,
							Status:  metav1.ConditionFalse,
							Reason:  "NotReadyForTesting",
							Message: "Purposefully not ready for testing",
						},
					)
					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			sampleLoadIndexWithValues = NewObjectContainer(
				&autoscalerv1alpha1.WeightedPNormLoadIndex{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-load-index-with-values",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: sampleLoadIndex.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
					container.Object.Status.Values = []common.LoadIndex{
						{
							Shard: common.Shard{
								UID:  "fake-shard-uid",
								ID:   "fake-shard-id",
								Data: map[string]string{"fake-key": "fake-value"},
							},
							Value:        resource.MustParse("1"),
							DisplayValue: "1",
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

			samplePartition = NewObjectContainer(
				&autoscalerv1alpha1.LongestProcessingTimePartition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-partition",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: autoscalerv1alpha1.LongestProcessingTimePartitionSpec{
						PartitionerSpec: common.PartitionerSpec{
							LoadIndexProviderRef: &corev1.TypedLocalObjectReference{
								APIGroup: ptr.To("autoscaler.argoproj.io"),
								Kind:     "WeightedPNormLoadIndex",
								Name:     sampleLoadIndex.Object.Name,
							},
						},
					},
				},
				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePartitionUsingNotReadyNormalizer = NewObjectContainer(
				&autoscalerv1alpha1.LongestProcessingTimePartition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-partition-using-not-ready-normalizer",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePartition.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
					container.Object.Spec.PartitionerSpec.LoadIndexProviderRef.Name = sampleLoadIndexNotReady.Object.Name
					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
				},
			)

			samplePartitionUsingNormalizerWithValues = NewObjectContainer(
				&autoscalerv1alpha1.LongestProcessingTimePartition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-partition-using-normalizer-with-values",
						Namespace: sampleNamespace.ObjectKey.Name,
					},
					Spec: samplePartition.Object.Spec,
				},
				func(container *objectContainer[*autoscalerv1alpha1.LongestProcessingTimePartition]) {
					container.Object.Spec.PartitionerSpec.LoadIndexProviderRef.Name = sampleLoadIndexWithValues.Object.Name
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
			CheckExitingOnNonExistingResource(func() *WeightedPNormLoadIndexReconciler {
				return &WeightedPNormLoadIndexReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}
			})
		})

		It("should handle error getting resource", func() {
			By("Failing to get resource")
			CheckFailureToGetResource(
				k8sClient,
				samplePartition,
				func(fClient *fakeClient) *LongestProcessingTimePartitionReconciler {
					return &LongestProcessingTimePartitionReconciler{
						Client: fClient,
						Scheme: fClient.Scheme(),
					}
				},
			)
		})

		It("should handle errors when load index provider lookup fails", func() {
			By("Preparing reconciler")

			container := samplePartition

			fClient := &fakeClient{
				Client: k8sClient,
			}
			fClient.
				WithGetFunction(sampleLoadIndex.Generic(),
					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return errors.New("fake error getting load index provider")
					},
				)

			controllerReconciler := &LongestProcessingTimePartitionReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
			}

			By("Reconciling with the expectation to fails to update the status")
			CheckFailureToUpdateStatus(
				fClient,
				samplePartition,
				func(fClient *fakeClient) *LongestProcessingTimePartitionReconciler {
					return &LongestProcessingTimePartitionReconciler{
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
			Expect(readyCondition.Reason).To(Equal("ErrorFindingLoadIndexProvider"))
			Expect(readyCondition.Message).To(ContainSubstring("fake error getting load index provider"))
		})

		It("should exit if load index provider is not ready", func() {
			By("Preparing reconciler")

			container := samplePartitionUsingNotReadyNormalizer

			controllerReconciler := &LongestProcessingTimePartitionReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling resource with expectation to fail updating status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *LongestProcessingTimePartitionReconciler {
					return &LongestProcessingTimePartitionReconciler{
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
			Expect(readyCondition.Reason).To(Equal("LoadIndexProviderNotReady"))
			Expect(readyCondition.Message).To(ContainSubstring("Check the status of a load index provider"))
		})

		It("should successfully partition", func() {
			By("Preparing reconciler")

			container := samplePartitionUsingNormalizerWithValues

			partitioned := false
			controllerReconciler := &LongestProcessingTimePartitionReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Partitioner: &fakePartitioner{
					fn: func(ctx context.Context, shards []common.LoadIndex) (common.ReplicaList, error) {
						partitions := common.ReplicaList{}
						for i, shard := range shards {
							partitions = append(partitions, common.Replica{
								ID:                    fmt.Sprintf("replica-%d", i),
								LoadIndexes:           []common.LoadIndex{shard},
								TotalLoad:             shard.Value,
								TotalLoadDisplayValue: shard.DisplayValue,
							})
						}
						partitioned = true
						return partitions, nil
					},
				},
			}

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *LongestProcessingTimePartitionReconciler {
					return &LongestProcessingTimePartitionReconciler{
						Client:      fClient,
						Scheme:      fClient.Scheme(),
						Partitioner: controllerReconciler.Partitioner,
					}
				},
			)

			By("Reconciling resource")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: container.NamespacedName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(partitioned).To(BeTrue())
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

			By("Checking partition results")
			shardsByUID := map[types.UID]common.LoadIndex{}
			for _, metric := range sampleLoadIndexWithValues.Object.Status.Values {
				shardsByUID[metric.Shard.UID] = metric
			}
			replicas := container.Object.Status.Replicas
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
		})

		It("should handle errors during partitioning", func() {
			By("Preparing reconciler")

			container := samplePartitionUsingNormalizerWithValues

			controllerReconciler := &LongestProcessingTimePartitionReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Partitioner: &fakePartitioner{
					fn: func(ctx context.Context, shards []common.LoadIndex) (common.ReplicaList, error) {
						return nil, errors.New("fake error partitioning")
					},
				},
			}

			By("Reconciling resource with expectation to fail to update status")
			CheckFailureToUpdateStatus(
				k8sClient,
				container,
				func(fClient *fakeClient) *LongestProcessingTimePartitionReconciler {
					return &LongestProcessingTimePartitionReconciler{
						Client:      fClient,
						Scheme:      fClient.Scheme(),
						Partitioner: controllerReconciler.Partitioner,
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
			Expect(readyCondition.Reason).To(Equal("ErrorDuringPartitioning"))
			Expect(readyCondition.Message).To(ContainSubstring("fake error"))
		})
	})
})
