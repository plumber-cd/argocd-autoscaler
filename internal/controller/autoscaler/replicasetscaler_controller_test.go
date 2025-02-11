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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

var _ = Describe("ReplicaSetScaler Controller", func() {
	var scenarioRun GenericScenarioRun

	var collector = NewScenarioCollector[*autoscalerv1alpha1.ReplicaSetScaler](
		func(fClient client.Client) *ReplicaSetScalerReconciler {
			return &ReplicaSetScalerReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
			}
		},
	)

	NewScenarioTemplate(
		"not existing resource",
		func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.ReplicaSetScaler{
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
		func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
			sampleNormalizer := NewObjectContainer(
				run,
				&autoscalerv1alpha1.ReplicaSetScaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-replica-set-scaler",
						Namespace: run.Namespace().ObjectKey().Name,
					},
					Spec: autoscalerv1alpha1.ReplicaSetScalerSpec{
						ScalerSpec: common.ScalerSpec{
							PartitionProviderRef: &corev1.TypedLocalObjectReference{
								Kind: "N/A",
								Name: "N/A",
							},
							ShardManagerRef: &corev1.TypedLocalObjectReference{
								Kind: "N/A",
								Name: "N/A",
							},
							ReplicaSetControllerRef: &corev1.TypedLocalObjectReference{
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
		Branch(
			"malformed duplicate modes",
			func(branch *Scenario[*autoscalerv1alpha1.ReplicaSetScaler]) {
				branch.Hydrate(
					"duplicate modes",
					func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
						run.Container().Object().Spec.Mode = &autoscalerv1alpha1.ReplicaSetScalerSpecModes{
							Default: &autoscalerv1alpha1.ReplicaSetScalerSpecModeDefault{},
							X0Y:     &autoscalerv1alpha1.ReplicaSetScalerSpecModeX0Y{},
						}
						run.Container().Update()
					},
				).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"handle errors",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
							Expect(readyCondition.Reason).To(Equal("ErrorResourceMalformedDuplicateModes"))
							Expect(readyCondition.Message).To(ContainSubstring("Scaler spec is invalid - only one mode can be set"))
						},
					).
					Commit(collector.Collect)
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle error during partition lookup",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
					{
						ID: "1",
						LoadIndexes: []common.LoadIndex{
							{
								Shard: common.Shard{
									UID:  types.UID("shard-1"),
									ID:   "shard-1",
									Data: map[string]string{"key": "value"},
								},
								Value:        resource.MustParse("1"),
								DisplayValue: "1",
							},
						},
						TotalLoad:             resource.MustParse("1"),
						TotalLoadDisplayValue: "1",
					},
					{
						ID: "2",
						LoadIndexes: []common.LoadIndex{
							{
								Shard: common.Shard{
									UID:  types.UID("shard-2"),
									ID:   "shard-2",
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
			"handle error during shard manager lookup",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
				Expect(readyCondition.Reason).To(Equal("ErrorFindingShardManager"))
				Expect(readyCondition.Message).To(ContainSubstring(`no matches for kind "N/A" in version ""`))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"sample shard manager",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
				sampleShardManager := NewObjectContainer(
					run,
					&autoscalerv1alpha1.SecretTypeClusterShardManager{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "sample-secret-type-cluster-shard-manager",
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Create()

				run.Container().Get().Object().Spec.ShardManagerRef = &corev1.TypedLocalObjectReference{
					APIGroup: ptr.To(sampleShardManager.GroupVersionKind().Group),
					Kind:     sampleShardManager.GroupVersionKind().Kind,
					Name:     sampleShardManager.ObjectKey().Name,
				}
				run.Container().Update()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"do nothing if shard manager is not ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
				Expect(readyCondition.Reason).To(Equal("ShardManagerNotReady"))
				Expect(readyCondition.Message).To(ContainSubstring("Check the status of a shard manager"))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"shard manager is ready",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
				sampleShardManager := NewObjectContainer(
					run,
					&autoscalerv1alpha1.SecretTypeClusterShardManager{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
							Namespace: run.Namespace().ObjectKey().Name,
						},
					},
				).Get()

				meta.SetStatusCondition(
					&sampleShardManager.Object().Status.Conditions,
					metav1.Condition{
						Type:   StatusTypeReady,
						Status: metav1.ConditionTrue,
						Reason: StatusTypeReady,
					},
				)
				sampleShardManager.StatusUpdate()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle error due to unknown replica set controller",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
				Expect(readyCondition.Reason).To(Equal("UnsupportedReplicaSetControllerKind"))
				Expect(readyCondition.Message).To(ContainSubstring(`Check the ref for a replica set controller`))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"replica set controller kind sts",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
				run.Container().Get().Object().Spec.ReplicaSetControllerRef.APIGroup = ptr.To("apps")
				run.Container().Get().Object().Spec.ReplicaSetControllerRef.Kind = "StatefulSet"
				run.Container().Update()
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle error on failure to lookup replica set controller",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
				Expect(readyCondition.Reason).To(Equal("ErrorFindingStatefulSet"))
				Expect(readyCondition.Message).To(ContainSubstring(`no matches for kind "StatefulSet" in version ""`))
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"sample sts",
			func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
				sampleSTS := NewObjectContainer(
					run,
					&appsv1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "sample-sts",
							Namespace: run.Namespace().ObjectKey().Name,
						},
						Spec: appsv1.StatefulSetSpec{
							Replicas: ptr.To(int32(25)),
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "sample-sts",
								},
							},
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										"app": "sample-sts",
									},
								},
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name: "argocd-application-controller",
											Env: []corev1.EnvVar{
												{
													Name:  "ARGOCD_CONTROLLER_REPLICAS",
													Value: "25",
												},
											},
										},
									},
								},
							},
						},
					},
				).Create()

				run.Container().Get().Object().Spec.ReplicaSetControllerRef = &corev1.TypedLocalObjectReference{
					APIGroup: ptr.To(sampleSTS.GroupVersionKind().Group),
					Kind:     sampleSTS.GroupVersionKind().Kind,
					Name:     sampleSTS.ObjectKey().Name,
				}
				run.Container().Update()

				sampleSTS.Object().Status.Replicas = *sampleSTS.Object().Spec.Replicas
				sampleSTS.StatusUpdate()
			},
		).
		Branch(
			"default mode",
			func(branch *Scenario[*autoscalerv1alpha1.ReplicaSetScaler]) {
				branch.
					Hydrate(
						"failing to update shard manager",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							shardManager := NewObjectContainer(
								run,
								&autoscalerv1alpha1.SecretTypeClusterShardManager{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							run.FakeClient().WithUpdateFunction(
								shardManager,
								func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
									return errors.New("fake failure to update shard manager")
								},
							)
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"failing to update shard manager",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).To(HaveOccurred())
							Expect(run.ReconcileError().Error()).To(ContainSubstring("fake failure to update shard manager"))

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("ErrorUpdatingShardManager"))
						},
					).
					Commit(collector.Collect).
					RemoveLastHydration().
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"update shard manager with the expected replicas",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).NotTo(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("WaitingForShardManagerToApplyDesiredState"))
						},
					).
					Commit(collector.Collect).
					Hydrate(
						"shard manager reconciled",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							samplePartition := NewObjectContainer(
								run,
								&autoscalerv1alpha1.LongestProcessingTimePartition{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							sampleShardManager := NewObjectContainer(
								run,
								&autoscalerv1alpha1.SecretTypeClusterShardManager{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							sampleShardManager.Object().Status.Replicas = samplePartition.Object().Status.Replicas
							sampleShardManager.StatusUpdate()
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"begin RS controller scaling",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).NotTo(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("BeginningRSControllerScaling"))

							By("Checking that we saved shard manager actual state")
							sampleShardManager := NewObjectContainer(
								run,
								&autoscalerv1alpha1.SecretTypeClusterShardManager{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							Expect(run.Container().Object().Status.Replicas).To(Equal(sampleShardManager.Object().Status.Replicas))
						},
					).
					Commit(collector.Collect).
					Hydrate(
						"in RS controller scaling phase",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							sampleShardManager := NewObjectContainer(
								run,
								&autoscalerv1alpha1.SecretTypeClusterShardManager{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							run.Container().Get().Object().Status.Replicas = sampleShardManager.Object().Status.Replicas
							run.Container().StatusUpdate()
						},
					).
					Hydrate(
						"failure to update sts",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							run.FakeClient().WithUpdateFunction(
								sampleSTS,
								func(
									ctx context.Context,
									obj client.Object,
									opts ...client.UpdateOption,
								) error {
									return errors.New("fake failure to update sts")
								},
							)
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"handle errors",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).To(HaveOccurred())
							Expect(run.ReconcileError().Error()).To(ContainSubstring("fake failure to update sts"))

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("ErrorScaling"))
						},
					).
					Commit(collector.Collect).
					RemoveLastHydration().
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"apply STS scaling",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).ToNot(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("WaitingForRSControllerToScale"))

							By("Checking STS")
							samplePartition := NewObjectContainer(
								run,
								&autoscalerv1alpha1.LongestProcessingTimePartition{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							desiredReplicas := len(samplePartition.Object().Status.Replicas)
							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							Expect(*sampleSTS.Object().Spec.Replicas).To(BeNumerically("==", desiredReplicas))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers).To(HaveLen(1))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env).To(HaveLen(1))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env[0].Name).
								To(Equal("ARGOCD_CONTROLLER_REPLICAS"))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env[0].Value).
								To(Equal(fmt.Sprintf("%d", desiredReplicas)))
						},
					).
					Commit(collector.Collect).
					Hydrate(
						"rollout restart is enabled",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							run.Container().Get().Object().Spec.Mode = &autoscalerv1alpha1.ReplicaSetScalerSpecModes{
								Default: &autoscalerv1alpha1.ReplicaSetScalerSpecModeDefault{
									RolloutRestart: ptr.To(true),
								},
							}
							run.Container().Update()
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"apply STS scaling and restart",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).ToNot(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("WaitingForRSControllerToScale"))

							By("Checking STS")
							samplePartition := NewObjectContainer(
								run,
								&autoscalerv1alpha1.LongestProcessingTimePartition{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							desiredReplicas := len(samplePartition.Object().Status.Replicas)
							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							Expect(*sampleSTS.Object().Spec.Replicas).To(BeNumerically("==", desiredReplicas))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers).To(HaveLen(1))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env).To(HaveLen(1))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env[0].Name).
								To(Equal("ARGOCD_CONTROLLER_REPLICAS"))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env[0].Value).
								To(Equal(fmt.Sprintf("%d", desiredReplicas)))

							restartAtAnnotation, ok := sampleSTS.Object().Spec.Template.Annotations["autoscaler.argoproj.io/restartedAt"]
							Expect(ok).To(BeTrue())
							restartAt, err := time.Parse(time.RFC3339, restartAtAnnotation)
							Expect(err).ToNot(HaveOccurred())
							Expect(time.Since(restartAt)).To(BeNumerically("<", time.Second))
						},
					).
					Commit(collector.Collect).
					RemoveLastHydration().
					Hydrate(
						"STS scaling completed",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							samplePartition := NewObjectContainer(
								run,
								&autoscalerv1alpha1.LongestProcessingTimePartition{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							desiredReplicas := len(samplePartition.Object().Status.Replicas)

							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							sampleSTS.Object().Status.Replicas = int32(desiredReplicas)
							sampleSTS.StatusUpdate()
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"finish reconciling",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
						},
					).
					Commit(collector.Collect)
			},
		).
		Branch(
			"X-0-Y mode",
			func(branch *Scenario[*autoscalerv1alpha1.ReplicaSetScaler]) {
				branch.
					Hydrate(
						"X-0-Y mode",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							run.Container().Get().Object().Spec.Mode = &autoscalerv1alpha1.ReplicaSetScalerSpecModes{
								X0Y: &autoscalerv1alpha1.ReplicaSetScalerSpecModeX0Y{},
							}
							run.Container().Update()
						},
					).
					Hydrate(
						"failure to update sts",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							run.FakeClient().WithUpdateFunction(
								sampleSTS,
								func(
									ctx context.Context,
									obj client.Object,
									opts ...client.UpdateOption,
								) error {
									return errors.New("fake failure to update sts")
								},
							)
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"handle errors",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).To(HaveOccurred())
							Expect(run.ReconcileError().Error()).To(ContainSubstring("fake failure to update sts"))

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("ErrorScalingToZero"))
						},
					).
					Commit(collector.Collect).
					RemoveLastHydration().
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"apply STS scaling to 0",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).ToNot(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("WaitingForRSControllerToScaleToZero"))

							By("Checking STS")
							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							Expect(*sampleSTS.Object().Spec.Replicas).To(BeNumerically("==", 0))
						},
					).
					Commit(collector.Collect).
					Hydrate(
						"STS scaling to 0 completed",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							sampleSTS.Object().Status.Replicas = 0
							sampleSTS.StatusUpdate()
						},
					).
					Hydrate(
						"failing to update shard manager",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							shardManager := NewObjectContainer(
								run,
								&autoscalerv1alpha1.SecretTypeClusterShardManager{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							run.FakeClient().WithUpdateFunction(
								shardManager,
								func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
									return errors.New("fake failure to update shard manager")
								},
							)
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"failing to update shard manager",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).To(HaveOccurred())
							Expect(run.ReconcileError().Error()).To(ContainSubstring("fake failure to update shard manager"))

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("ErrorUpdatingShardManager"))
						},
					).
					Commit(collector.Collect).
					RemoveLastHydration().
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"update shard manager with the expected replicas",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).NotTo(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("WaitingForShardManagerToApplyDesiredState"))
						},
					).
					Commit(collector.Collect).
					Hydrate(
						"shard manager reconciled",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							samplePartition := NewObjectContainer(
								run,
								&autoscalerv1alpha1.LongestProcessingTimePartition{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							sampleShardManager := NewObjectContainer(
								run,
								&autoscalerv1alpha1.SecretTypeClusterShardManager{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							sampleShardManager.Object().Status.Replicas = samplePartition.Object().Status.Replicas
							sampleShardManager.StatusUpdate()
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"begin RS controller scaling",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).NotTo(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("BeginningRSControllerScaling"))

							By("Checking that we saved shard manager actual state")
							sampleShardManager := NewObjectContainer(
								run,
								&autoscalerv1alpha1.SecretTypeClusterShardManager{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							Expect(run.Container().Object().Status.Replicas).To(Equal(sampleShardManager.Object().Status.Replicas))
						},
					).
					Commit(collector.Collect).
					Hydrate(
						"in RS controller scaling phase",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							sampleShardManager := NewObjectContainer(
								run,
								&autoscalerv1alpha1.SecretTypeClusterShardManager{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ShardManagerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							run.Container().Get().Object().Status.Replicas = sampleShardManager.Object().Status.Replicas
							run.Container().StatusUpdate()
						},
					).
					Hydrate(
						"failure to update sts",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							run.FakeClient().WithUpdateFunction(
								sampleSTS,
								func(
									ctx context.Context,
									obj client.Object,
									opts ...client.UpdateOption,
								) error {
									return errors.New("fake failure to update sts")
								},
							)
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"handle errors",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).To(HaveOccurred())
							Expect(run.ReconcileError().Error()).To(ContainSubstring("fake failure to update sts"))

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("ErrorScaling"))
						},
					).
					Commit(collector.Collect).
					RemoveLastHydration().
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"apply STS scaling",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							Expect(run.ReconcileError()).ToNot(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking conditions")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition).NotTo(BeNil())
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("WaitingForRSControllerToScale"))

							By("Checking STS")
							samplePartition := NewObjectContainer(
								run,
								&autoscalerv1alpha1.LongestProcessingTimePartition{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							desiredReplicas := len(samplePartition.Object().Status.Replicas)
							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							Expect(*sampleSTS.Object().Spec.Replicas).To(BeNumerically("==", desiredReplicas))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers).To(HaveLen(1))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env).To(HaveLen(1))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env[0].Name).
								To(Equal("ARGOCD_CONTROLLER_REPLICAS"))
							Expect(sampleSTS.Object().Spec.Template.Spec.Containers[0].Env[0].Value).
								To(Equal(fmt.Sprintf("%d", desiredReplicas)))
						},
					).
					Commit(collector.Collect).
					Hydrate(
						"STS scaling completed",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
							samplePartition := NewObjectContainer(
								run,
								&autoscalerv1alpha1.LongestProcessingTimePartition{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.PartitionProviderRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							desiredReplicas := len(samplePartition.Object().Status.Replicas)

							sampleSTS := NewObjectContainer(
								run,
								&appsv1.StatefulSet{
									ObjectMeta: metav1.ObjectMeta{
										Name:      run.Container().Get().Object().Spec.ReplicaSetControllerRef.Name,
										Namespace: run.Namespace().ObjectKey().Name,
									},
								},
							).Get()
							sampleSTS.Object().Status.Replicas = int32(desiredReplicas)
							sampleSTS.StatusUpdate()
						},
					).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"finish reconciling",
						func(run *ScenarioRun[*autoscalerv1alpha1.ReplicaSetScaler]) {
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
						},
					).
					Commit(collector.Collect)
			},
		)

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
