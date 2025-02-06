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

// import (
// 	"context"
// 	"errors"
// 	"time"
//
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// 	"k8s.io/apimachinery/pkg/api/meta"
// 	"k8s.io/apimachinery/pkg/api/resource"
// 	"k8s.io/apimachinery/pkg/types"
// 	"k8s.io/utils/ptr"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/reconcile"
//
// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//
// 	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
// 	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
// )
//
// type fakeLoadIndexer struct {
// 	fn func(int32, map[string]autoscalerv1alpha1.WeightedPNormLoadIndexWeight, []common.MetricValue) ([]common.LoadIndex, error)
// }
//
// func (f *fakeLoadIndexer) Calculate(p int32, weights map[string]autoscalerv1alpha1.WeightedPNormLoadIndexWeight, values []common.MetricValue) ([]common.LoadIndex, error) {
// 	if f.fn != nil {
// 		return f.fn(p, weights, values)
// 	}
// 	return nil, errors.New("fake error in fakeLoadIndexer")
// }
//
// var _ = Describe("WeightedPNormLoadIndex Controller", func() {
// 	Context("When reconciling a resource", func() {
// 		ctx := context.Background()
//
// 		var sampleNamespace *objectContainer[*corev1.Namespace]
//
// 		var sampleNormalizer *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]
// 		var sampleNormalizerNotReady *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]
// 		var sampleNormalizerWithValues *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]
//
// 		var sampleLoadIndex *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]
// 		var sampleLoadIndexMalformedWeights *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]
// 		var sampleLoadIndexUsingNotReadyNormalizer *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]
// 		var sampleLoadIndexUsingNormalizerWithValues *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]
//
// 		BeforeEach(func() {
// 			By("Creating resources")
//
// 			sampleNamespace = newNamespaceWithRandomName()
// 			Expect(k8sClient.create(ctx, sampleNamespace.Generic())).To(Succeed())
// 			Expect(k8sClient.get(ctx, sampleNamespace.Generic())).To(Succeed())
//
// 			sampleNormalizer = NewObjectContainer(
// 				&autoscalerv1alpha1.RobustScalingNormalizer{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-normalizer",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: autoscalerv1alpha1.RobustScalingNormalizerSpec{
// 						NormalizerSpec: common.NormalizerSpec{
// 							MetricValuesProviderRef: &corev1.TypedLocalObjectReference{
// 								Kind: "N/A",
// 								Name: "N/A",
// 							},
// 						},
// 					},
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleNormalizerNotReady = NewObjectContainer(
// 				&autoscalerv1alpha1.RobustScalingNormalizer{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-normalizer-not-ready",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: sampleNormalizer.Object.Spec,
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 					meta.SetStatusCondition(
// 						&container.Object.Status.Conditions,
// 						metav1.Condition{
// 							Type:    StatusTypeReady,
// 							Status:  metav1.ConditionFalse,
// 							Reason:  "NotReadyForTest",
// 							Message: "Intentionally not ready for testing purposes",
// 						},
// 					)
// 					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleNormalizerWithValues = NewObjectContainer(
// 				&autoscalerv1alpha1.RobustScalingNormalizer{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-normalizer-with-values",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: sampleNormalizer.Object.Spec,
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.RobustScalingNormalizer]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 					container.Object.Status.Values = []common.MetricValue{
// 						{
// 							ID: "fake-metric",
// 							Shard: common.Shard{
// 								UID:  "fake-uid",
// 								ID:   "fake-id",
// 								Data: map[string]string{"fake-key": "fake-value"},
// 							},
// 							Query:        "fake-query",
// 							Value:        resource.MustParse("0"),
// 							DisplayValue: "0",
// 						},
// 					}
// 					meta.SetStatusCondition(
// 						&container.Object.Status.Conditions,
// 						metav1.Condition{
// 							Type:   StatusTypeReady,
// 							Status: metav1.ConditionTrue,
// 							Reason: StatusTypeReady,
// 						},
// 					)
// 					Expect(k8sClient.statusUpdate(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleLoadIndex = NewObjectContainer(
// 				&autoscalerv1alpha1.WeightedPNormLoadIndex{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-load-index",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: autoscalerv1alpha1.WeightedPNormLoadIndexSpec{
// 						LoadIndexerSpec: common.LoadIndexerSpec{
// 							MetricValuesProviderRef: &corev1.TypedLocalObjectReference{
// 								APIGroup: ptr.To("autoscaler.argoproj.io"),
// 								Kind:     "RobustScalingNormalizer",
// 								Name:     sampleNormalizer.Object.Name,
// 							},
// 						},
// 						P: 2,
// 						Weights: []autoscalerv1alpha1.WeightedPNormLoadIndexWeight{
// 							{
// 								ID:       "fake-metric",
// 								Weight:   resource.MustParse("1"),
// 								Negative: ptr.To(true),
// 							},
// 						},
// 					},
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleLoadIndexMalformedWeights = NewObjectContainer(
// 				&autoscalerv1alpha1.WeightedPNormLoadIndex{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-load-index-with-malformed-weights",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: sampleLoadIndex.Object.Spec,
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
// 					container.Object.Spec.Weights = append(
// 						container.Object.Spec.Weights,
// 						container.Object.Spec.Weights[0],
// 					)
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleLoadIndexUsingNotReadyNormalizer = NewObjectContainer(
// 				&autoscalerv1alpha1.WeightedPNormLoadIndex{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-load-index-using-not-ready-normalizer",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: sampleLoadIndex.Object.Spec,
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
// 					container.Object.Spec.MetricValuesProviderRef = &corev1.TypedLocalObjectReference{
// 						APIGroup: ptr.To("autoscaler.argoproj.io"),
// 						Kind:     "RobustScalingNormalizer",
// 						Name:     sampleNormalizerNotReady.Object.Name,
// 					}
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
//
// 			sampleLoadIndexUsingNormalizerWithValues = NewObjectContainer(
// 				&autoscalerv1alpha1.WeightedPNormLoadIndex{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      "sample-load-index-using-normalizer-with-values",
// 						Namespace: sampleNamespace.ObjectKey.Name,
// 					},
// 					Spec: sampleLoadIndex.Object.Spec,
// 				},
// 				func(container *objectContainer[*autoscalerv1alpha1.WeightedPNormLoadIndex]) {
// 					container.Object.Spec.MetricValuesProviderRef = &corev1.TypedLocalObjectReference{
// 						APIGroup: ptr.To("autoscaler.argoproj.io"),
// 						Kind:     "RobustScalingNormalizer",
// 						Name:     sampleNormalizerWithValues.Object.Name,
// 					}
// 					Expect(k8sClient.create(ctx, container.Generic())).To(Succeed())
// 					Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 				},
// 			)
// 		})
//
// 		AfterEach(func() {
// 			err := k8sClient.get(ctx, sampleNamespace.Generic())
// 			Expect(err).NotTo(HaveOccurred())
//
// 			By("Cleanup the test namespace")
// 			Expect(k8sClient.delete(ctx, sampleNamespace.Generic())).To(Succeed())
// 			sampleNamespace = nil
// 		})
//
// 		It("should successfully exit when the resource didn't exist", func() {
// 			By("Reconciling non existing resource")
// 			CheckExitingOnNonExistingResource(func() *WeightedPNormLoadIndexReconciler {
// 				return &WeightedPNormLoadIndexReconciler{
// 					Client: k8sClient,
// 					Scheme: k8sClient.Scheme(),
// 				}
// 			})
// 		})
//
// 		It("should handle error getting resource", func() {
// 			By("Failing to get resource")
// 			CheckFailureToGetResource(
// 				k8sClient,
// 				sampleLoadIndex,
// 				func(fClient *fakeClient) *WeightedPNormLoadIndexReconciler {
// 					return &WeightedPNormLoadIndexReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
// 		})
//
// 		It("should handle errors during reading weights", func() {
// 			By("Setting up reconciler")
//
// 			container := sampleLoadIndexMalformedWeights
//
// 			controllerReconciler := &WeightedPNormLoadIndexReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}
//
// 			By("Reconciling malformed resource with expectation to fail updating status")
// 			CheckFailureToUpdateStatus(
// 				k8sClient,
// 				container,
// 				func(fClient *fakeClient) *WeightedPNormLoadIndexReconciler {
// 					return &WeightedPNormLoadIndexReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling malformed resource")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("WeightsMalformed"))
// 			Expect(readyCondition.Message).To(Equal("duplicate weight definition for metric ID 'fake-metric'"))
// 		})
//
// 		It("should handle errors when metrics provider lookup fails", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleLoadIndex
//
// 			fClient := &fakeClient{
// 				Client: k8sClient,
// 			}
// 			fClient.
// 				WithGetFunction(sampleNormalizer.Generic(),
// 					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
// 						return errors.New("fake error getting the metrics provider")
// 					},
// 				)
//
// 			controllerReconciler := &WeightedPNormLoadIndexReconciler{
// 				Client: fClient,
// 				Scheme: fClient.Scheme(),
// 			}
//
// 			By("Reconciling with the expectation to fails to update the status")
// 			CheckFailureToUpdateStatus(
// 				fClient,
// 				sampleLoadIndex,
// 				func(fClient *fakeClient) *WeightedPNormLoadIndexReconciler {
// 					return &WeightedPNormLoadIndexReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("ErrorFindingMetricProvider"))
// 			Expect(readyCondition.Message).To(ContainSubstring("fake error getting the metrics provider"))
// 		})
//
// 		It("should exit if metrics provider is not ready", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleLoadIndexUsingNotReadyNormalizer
//
// 			controllerReconciler := &WeightedPNormLoadIndexReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 			}
//
// 			By("Reconciling resource with expectation to fail updating status")
// 			CheckFailureToUpdateStatus(
// 				k8sClient,
// 				container,
// 				func(fClient *fakeClient) *WeightedPNormLoadIndexReconciler {
// 					return &WeightedPNormLoadIndexReconciler{
// 						Client: fClient,
// 						Scheme: fClient.Scheme(),
// 					}
// 				},
// 			)
//
// 			By("Reconciling resource")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("MetricValuesProviderNotReady"))
// 			Expect(readyCondition.Message).To(ContainSubstring("Check the status of metric values provider"))
// 		})
//
// 		It("should successfully calculate load indexes", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleLoadIndexUsingNormalizerWithValues
//
// 			calculated := false
// 			controllerReconciler := &WeightedPNormLoadIndexReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 				LoadIndexer: &fakeLoadIndexer{
// 					fn: func(p int32, weights map[string]autoscalerv1alpha1.WeightedPNormLoadIndexWeight, values []common.MetricValue) ([]common.LoadIndex, error) {
// 						loadIndexes := []common.LoadIndex{}
// 						for _, value := range values {
// 							loadIndexes = append(loadIndexes, common.LoadIndex{
// 								Shard:        value.Shard,
// 								Value:        resource.MustParse("1"),
// 								DisplayValue: "1",
// 							})
// 						}
// 						calculated = true
// 						return loadIndexes, nil
// 					},
// 				},
// 			}
//
// 			By("Reconciling resource with expectation to fail to update status")
// 			CheckFailureToUpdateStatus(
// 				k8sClient,
// 				container,
// 				func(fClient *fakeClient) *WeightedPNormLoadIndexReconciler {
// 					return &WeightedPNormLoadIndexReconciler{
// 						Client:      fClient,
// 						Scheme:      fClient.Scheme(),
// 						LoadIndexer: controllerReconciler.LoadIndexer,
// 					}
// 				},
// 			)
//
// 			By("Reconciling resource")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).ToNot(HaveOccurred())
// 			Expect(calculated).To(BeTrue())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
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
// 			By("Checking load indexing results")
// 			shardsByUID := map[types.UID]common.Shard{}
// 			for _, metric := range sampleNormalizerWithValues.Object.Status.Values {
// 				shardsByUID[metric.Shard.UID] = metric.Shard
// 			}
// 			loadIndexes := container.Object.Status.Values
// 			Expect(loadIndexes).To(HaveLen(len(shardsByUID)))
// 			normalizedValuesByShardUID := map[types.UID]common.LoadIndex{}
// 			for _, val := range loadIndexes {
// 				normalizedValuesByShardUID[val.Shard.UID] = val
// 			}
// 			for shardUID, shard := range shardsByUID {
// 				val, ok := normalizedValuesByShardUID[shardUID]
// 				Expect(ok).To(BeTrue())
// 				Expect(val.Shard).To(Equal(shard))
// 				Expect(val.Value).To(Equal(resource.MustParse("1")))
// 				Expect(val.DisplayValue).To(Equal("1"))
// 			}
// 		})
//
// 		It("should handle errors during load indexes calculation", func() {
// 			By("Preparing reconciler")
//
// 			container := sampleLoadIndexUsingNormalizerWithValues
//
// 			controllerReconciler := &WeightedPNormLoadIndexReconciler{
// 				Client: k8sClient,
// 				Scheme: k8sClient.Scheme(),
// 				LoadIndexer: &fakeLoadIndexer{
// 					fn: func(p int32, weights map[string]autoscalerv1alpha1.WeightedPNormLoadIndexWeight, values []common.MetricValue) ([]common.LoadIndex, error) {
// 						return nil, errors.New("fake error")
// 					},
// 				},
// 			}
//
// 			By("Reconciling resource with expectation to fail to update status")
// 			CheckFailureToUpdateStatus(
// 				k8sClient,
// 				container,
// 				func(fClient *fakeClient) *WeightedPNormLoadIndexReconciler {
// 					return &WeightedPNormLoadIndexReconciler{
// 						Client:      fClient,
// 						Scheme:      fClient.Scheme(),
// 						LoadIndexer: controllerReconciler.LoadIndexer,
// 					}
// 				},
// 			)
//
// 			By("Reconciling resource")
// 			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
// 				NamespacedName: container.NamespacedName,
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 			Expect(result.Requeue).To(BeFalse())
//
// 			By("Checking conditions")
// 			Expect(k8sClient.get(ctx, container.Generic())).To(Succeed())
// 			readyCondition := meta.FindStatusCondition(
// 				container.Object.Status.Conditions,
// 				StatusTypeReady,
// 			)
// 			Expect(readyCondition).NotTo(BeNil())
// 			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
// 			Expect(readyCondition.Reason).To(Equal("LoadIndexCalculationError"))
// 			Expect(readyCondition.Message).To(ContainSubstring("fake error"))
// 		})
// 	})
// })
