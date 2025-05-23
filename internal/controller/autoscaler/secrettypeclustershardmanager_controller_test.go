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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	. "github.com/plumber-cd/argocd-autoscaler/test/harness"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

var _ = Describe("SecretTypeClusterShardManager Controller", func() {
	var scenarioRun GenericScenarioRun

	var collector = NewScenarioCollector[*autoscalerv1alpha1.SecretTypeClusterShardManager](
		func(fClient client.Client) *SecretTypeClusterShardManagerReconciler {
			return &SecretTypeClusterShardManagerReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
			}
		},
	)

	NewScenarioTemplate(
		"not existing resource",
		func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
			sampleShardManager := NewObjectContainer(
				run,
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "not-existent",
						Namespace: run.Namespace().ObjectKey().Name,
					},
				},
			)
			run.SetContainer(sampleShardManager)
		},
	).
		BranchResourceNotFoundCheck(collector.Collect).
		BranchFailureToGetResourceCheck(collector.Collect)

	NewScenarioTemplate(
		"basic",
		func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
			sampleShardManager := NewObjectContainer(
				run,
				&autoscalerv1alpha1.SecretTypeClusterShardManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-shard-manager",
						Namespace: run.Namespace().ObjectKey().Name,
					},
					Spec: autoscalerv1alpha1.SecretTypeClusterShardManagerSpec{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"mock-label": "mock",
							},
						},
					},
				},
				func(
					run GenericScenarioRun,
					_ *ObjectContainer[*autoscalerv1alpha1.SecretTypeClusterShardManager],
				) {
					NewObjectContainer(
						run,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "sample-secret",
								Namespace: run.Namespace().Object().Name,
								Labels: map[string]string{
									"mock-label": "mock",
								},
							},
							StringData: map[string]string{
								"name":   "mock-cluster",
								"server": "http://mock-cluster:8000",
							},
						},
					).Create()

					NewObjectContainer(
						run,
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "unlabeled-secret",
								Namespace: run.Namespace().Object().Name,
								Labels: map[string]string{
									"mock-label": "mock-2",
								},
							},
							StringData: map[string]string{
								"name":   "mock-cluster-2",
								"server": "http://mock-cluster-2:8000",
							},
						},
					).Create()
				},
			).Create()
			run.SetContainer(sampleShardManager)
		},
	).
		Hydrate(
			"failure to list secrets",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				run.FakeClient().
					WithListFunction(&corev1.SecretList{},
						func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
							return errors.New("fake error listing secrets")
						},
					)
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle errors",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError()).To(HaveOccurred())
				Expect(run.ReconcileError().Error()).To(Equal("fake error listing secrets"))

				By("Checking ready condition")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("FailedToListSecrets"))
				Expect(readyCondition.Message).To(Equal("fake error listing secrets"))
			},
		).
		Commit(collector.Collect).
		DeHydrate().
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"export shards to the status",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError()).ToNot(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())

				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

				secretsListOptions := &client.ListOptions{
					Namespace: run.Namespace().ObjectKey().Name,
					LabelSelector: labels.SelectorFromSet(
						run.Container().Object().Spec.LabelSelector.MatchLabels,
					),
				}
				shardSecrets := &corev1.SecretList{}
				Expect(run.Client().List(ctx, shardSecrets, secretsListOptions)).To(Succeed())
				shardSecretsByUID := map[types.UID]corev1.Secret{}
				for _, secret := range shardSecrets.Items {
					shardSecretsByUID[secret.GetUID()] = secret
				}

				Expect(run.Container().Object().Status.Shards).To(HaveLen(len(shardSecretsByUID)))
				for _, shard := range run.Container().Object().Status.Shards {
					secret, ok := shardSecretsByUID[shard.UID]
					Expect(ok).To(BeTrue())
					Expect(shard.UID).To(Equal(secret.GetUID()))
					Expect(shard.ID).To(Equal(secret.Name))
					Expect(shard.Namespace).To(Equal(secret.Namespace))
					Expect(shard.Name).To(Equal(string(secret.Data["name"])))
					Expect(shard.Server).To(Equal(string(secret.Data["server"])))
				}
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"desired partitioning",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				secretsListOptions := &client.ListOptions{
					Namespace: run.Namespace().ObjectKey().Name,
					LabelSelector: labels.SelectorFromSet(
						run.Container().Object().Spec.LabelSelector.MatchLabels,
					),
				}
				shardSecrets := &corev1.SecretList{}
				Expect(run.Client().List(ctx, shardSecrets, secretsListOptions)).To(Succeed())
				Expect(shardSecrets.Items).ToNot(BeEmpty())

				run.Container().Object().Spec.ShardManagerSpec = common.ShardManagerSpec{
					Replicas: common.ReplicaList{},
				}

				for i, secret := range shardSecrets.Items {
					run.Container().Object().Spec.ShardManagerSpec.Replicas = append(
						run.Container().Object().Spec.ShardManagerSpec.Replicas,
						common.Replica{
							ID: int32(i),
							LoadIndexes: []common.LoadIndex{
								{
									Shard: common.Shard{
										UID:       secret.GetUID(),
										ID:        secret.Name,
										Namespace: secret.Namespace,
										Name:      string(secret.Data["name"]),
										Server:    string(secret.Data["server"]),
									},
									Value:        resource.MustParse("1"),
									DisplayValue: "1",
								},
							},
							TotalLoad:             resource.MustParse("1"),
							TotalLoadDisplayValue: "1",
						},
					)
				}

				run.Container().Update()
			},
		).
		Branch(
			"malformed desired partitioning",
			func(branch *Scenario[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				branch.Hydrate(
					"duplicated replicas",
					func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
						run.Container().Get().Object().Spec.ShardManagerSpec.Replicas = append(
							run.Container().Object().Spec.ShardManagerSpec.Replicas,
							run.Container().Object().Spec.ShardManagerSpec.Replicas[0],
						)
						run.Container().Update()
					},
				).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"handle errors",
						func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
							Expect(run.ReconcileError()).ToNot(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Duration(0)))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())

							By("Checking ready condition")
							readyCondition := meta.FindStatusCondition(
								run.Container().Get().Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("FailedToReadDesiredReplicaShard"))
							Expect(readyCondition.Message).To(Equal("duplicate replica found"))
						},
					).
					Commit(collector.Collect)
			},
		).
		Hydrate(
			"failing to update secrets",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				secretsListOptions := &client.ListOptions{
					Namespace: run.Namespace().ObjectKey().Name,
					LabelSelector: labels.SelectorFromSet(
						run.Container().Object().Spec.LabelSelector.MatchLabels,
					),
				}
				shardSecrets := &corev1.SecretList{}
				Expect(run.Client().List(ctx, shardSecrets, secretsListOptions)).To(Succeed())
				for _, secret := range shardSecrets.Items {
					container := NewObjectContainer(
						run,
						secret.DeepCopy(),
					).Get()
					run.FakeClient().WithUpdateFunction(container, func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return errors.New("fake error updating secret")
					})
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle errors",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError()).To(HaveOccurred())
				Expect(run.ReconcileError().Error()).To(Equal("fake error updating secret"))

				By("Checking ready condition")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("FailedToUpdateSecret"))
				Expect(readyCondition.Message).To(Equal("fake error updating secret"))
			},
		).
		Commit(collector.Collect).
		RemoveLastHydration().
		WithCheck(
			"successfully update shards on secrets",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError()).ToNot(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())

				By("Checking ready condition")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

				By("Checking secret was assigned a shard")
				secretsListOptions := &client.ListOptions{
					Namespace: run.Namespace().ObjectKey().Name,
					LabelSelector: labels.SelectorFromSet(
						run.Container().Object().Spec.LabelSelector.MatchLabels,
					),
				}
				shardSecrets := &corev1.SecretList{}
				Expect(run.Client().List(ctx, shardSecrets, secretsListOptions)).To(Succeed())
				shardSecretsByUID := map[types.UID]corev1.Secret{}
				for _, secret := range shardSecrets.Items {
					shardSecretsByUID[secret.GetUID()] = secret
				}
				for _, replica := range run.Container().Object().Spec.ShardManagerSpec.Replicas {
					secret, ok := shardSecretsByUID[replica.LoadIndexes[0].Shard.UID]
					Expect(ok).To(BeTrue())
					Expect(secret.Data["shard"]).To(Equal([]byte(fmt.Sprintf("%d", replica.ID))))
				}
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"failing to update secrets",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				run.FakeClient().
					WithUpdateFunction(run.Container(),
						func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
							return errors.New("fake error updating secret")
						},
					)
			},
		).
		WithCheck(
			"handle errors",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError()).To(HaveOccurred())
				Expect(run.ReconcileError().Error()).To(Equal("fake error updating secret"))

				By("Checking ready condition")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("FailedToUpdateSecret"))
				Expect(readyCondition.Message).To(Equal("fake error updating secret"))
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
