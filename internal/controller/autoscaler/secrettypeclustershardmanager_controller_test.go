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
	autoscalerv1alpha1 "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	"github.com/plumber-cd/argocd-autoscaler/test/harness"
	. "github.com/plumber-cd/argocd-autoscaler/test/harness"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("SecretTypeClusterShardManager Controller", func() {

	var testNamespace *ObjectContainer[*corev1.Namespace]

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
		func() *autoscalerv1alpha1.SecretTypeClusterShardManager {
			return &autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-existent",
				},
			}
		},
	).
		HydrateWithContainer().
		WithFakeClient(nil).
		BranchResourceNotFoundCheck(collector.Collect).
		BranchFailureToGetResourceCheck(collector.Collect)

	NewScenarioTemplate(
		"basic",
		func() *autoscalerv1alpha1.SecretTypeClusterShardManager {
			return &autoscalerv1alpha1.SecretTypeClusterShardManager{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sample-shard-manager",
				},
				Spec: autoscalerv1alpha1.SecretTypeClusterShardManagerSpec{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"mock-label": "mock",
						},
					},
				},
			}
		},
	).
		HydrateWithClientCreatingContainer().
		Hydrate(
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				_ = CreateObjectContainer(run.Context, *run.Client, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-secret",
						Namespace: run.Namespace.Object().Name,
						Labels: map[string]string{
							"mock-label": "mock",
						},
					},
					StringData: map[string]string{
						"name":   "mock-cluster",
						"server": "http://mock-cluster:8000",
					},
				})

				_ = harness.CreateObjectContainer(run.Context, *run.Client, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unlabeled-secret",
						Namespace: run.Namespace.Object().Name,
						Labels: map[string]string{
							"mock-label": "mock-2",
						},
					},
					StringData: map[string]string{
						"name":   "mock-cluster-2",
						"server": "http://mock-cluster-2:8000",
					},
				})
			},
		).
		WithFakeClient(nil).
		WithFakeClient(
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				run.FakeClient.
					WithListFunction(&corev1.SecretList{},
						func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
							return errors.New("fake error listing secrets")
						},
					)
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"should handle errors when failing to list secrets",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError).To(HaveOccurred())
				Expect(run.ReconcileError.Error()).To(Equal("fake error listing secrets"))
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Second))

				By("Checking ready condition")
				Expect(run.Client.GetContainer(ctx, run.Container.ClientObject())).To(Succeed())
				readyCondition := meta.FindStatusCondition(
					run.Container.Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("FailedToListSecrets"))
				Expect(readyCondition.Message).To(Equal("fake error listing secrets"))
			},
		).
		Commit(collector.Collect).
		ResetClientPatches().
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"should export shards to the status",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError).ToNot(HaveOccurred())
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult.Requeue).To(BeFalse())

				Expect(run.Client.GetContainer(ctx, run.Container.ClientObject())).To(Succeed())
				readyCondition := meta.FindStatusCondition(
					run.Container.Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

				secretsListOptions := &client.ListOptions{
					Namespace: testNamespace.ObjectKey().Name,
					LabelSelector: labels.SelectorFromSet(
						run.Container.Object().Spec.LabelSelector.MatchLabels,
					),
				}
				shardSecrets := &corev1.SecretList{}
				Expect(run.Client.List(ctx, shardSecrets, secretsListOptions)).To(Succeed())
				shardSecretsByUID := map[types.UID]corev1.Secret{}
				for _, secret := range shardSecrets.Items {
					shardSecretsByUID[secret.GetUID()] = secret
				}

				Expect(run.Container.Object().Status.Shards).To(HaveLen(len(shardSecretsByUID)))
				for _, shard := range run.Container.Object().Status.Shards {
					secret, ok := shardSecretsByUID[shard.UID]
					Expect(ok).To(BeTrue())
					Expect(shard.UID).To(Equal(secret.GetUID()))
					Expect(shard.ID).To(Equal(secret.Name))
					stringData := map[string]string{}
					for key, value := range secret.Data {
						stringData[key] = string(value)
					}
					stringData["namespace"] = secret.Namespace
					Expect(shard.Data).To(Equal(stringData))
				}
			},
		).
		Commit(collector.Collect).
		Hydrate(
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				secretsListOptions := &client.ListOptions{
					Namespace: testNamespace.ObjectKey().Name,
					LabelSelector: labels.SelectorFromSet(
						run.Container.Object().Spec.LabelSelector.MatchLabels,
					),
				}
				shardSecrets := &corev1.SecretList{}
				Expect(run.Client.List(ctx, shardSecrets, secretsListOptions)).To(Succeed())
				Expect(shardSecrets.Items).ToNot(BeEmpty())

				run.Container.Object().Spec.ShardManagerSpec = common.ShardManagerSpec{
					Replicas: common.ReplicaList{},
				}

				for i, secret := range shardSecrets.Items {
					run.Container.Object().Spec.ShardManagerSpec.Replicas = append(
						run.Container.Object().Spec.ShardManagerSpec.Replicas,
						common.Replica{
							ID: fmt.Sprintf("%d", i),
							LoadIndexes: []common.LoadIndex{
								{
									Shard: common.Shard{
										UID: secret.GetUID(),
										ID:  secret.Name,
										Data: map[string]string{
											"namespace": secret.Namespace,
											"name":      string(secret.Data["name"]),
											"server":    string(secret.Data["server"]),
										},
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

				Expect(run.Client.UpdateContainer(ctx, run.Container.ClientObject())).To(Succeed())
				Expect(run.Client.GetContainer(ctx, run.Container.ClientObject())).To(Succeed())
			},
		).
		WithFakeClient(nil).
		Branch(
			func(branch *ScenarioWithFakeClient[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				branch.Hydrate(
					func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
						run.Container.Object().Spec.ShardManagerSpec.Replicas = append(
							run.Container.Object().Spec.ShardManagerSpec.Replicas,
							run.Container.Object().Spec.ShardManagerSpec.Replicas[0],
						)
						Expect(run.Client.UpdateContainer(ctx, run.Container.ClientObject())).To(Succeed())
						Expect(run.Client.GetContainer(ctx, run.Container.ClientObject())).To(Succeed())
					},
				).
					WithFakeClient(nil).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"should handle errors when replicas are duplicated",
						func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
							Expect(run.ReconcileError).ToNot(HaveOccurred())
							Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Duration(0)))
							Expect(run.ReconcileResult.Requeue).To(BeFalse())

							By("Checking ready condition")
							Expect(run.Client.GetContainer(ctx, run.Container.ClientObject())).To(Succeed())
							readyCondition := meta.FindStatusCondition(
								run.Container.Object().Status.Conditions,
								StatusTypeReady,
							)
							Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
							Expect(readyCondition.Reason).To(Equal("FailedToListReplicas"))
							Expect(readyCondition.Message).To(Equal("duplicate replica found"))
						},
					).
					Commit(collector.Collect)
			},
		).
		WithCheck(
			"should successfully update shards on secrets",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError).ToNot(HaveOccurred())
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult.Requeue).To(BeFalse())

				By("Checking ready condition")
				Expect(run.Client.GetContainer(ctx, run.Container.ClientObject())).To(Succeed())
				readyCondition := meta.FindStatusCondition(
					run.Container.Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

				By("Checking secret was assigned a shard")
				secretsListOptions := &client.ListOptions{
					Namespace: testNamespace.ObjectKey().Name,
					LabelSelector: labels.SelectorFromSet(
						run.Container.Object().Spec.LabelSelector.MatchLabels,
					),
				}
				shardSecrets := &corev1.SecretList{}
				Expect(run.Client.List(ctx, shardSecrets, secretsListOptions)).To(Succeed())
				shardSecretsByUID := map[types.UID]corev1.Secret{}
				for _, secret := range shardSecrets.Items {
					shardSecretsByUID[secret.GetUID()] = secret
				}
				for _, replica := range run.Container.Object().Spec.ShardManagerSpec.Replicas {
					secret, ok := shardSecretsByUID[replica.LoadIndexes[0].Shard.UID]
					Expect(ok).To(BeTrue())
					Expect(secret.Data["shard"]).To(Equal([]byte(replica.ID)))
				}
			},
		).
		Commit(collector.Collect).
		WithFakeClient(
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				run.FakeClient.
					WithUpdateFunction(run.Container.ClientObject(),
						func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
							return errors.New("fake error updating secret")
						},
					)
			},
		).
		WithCheck(
			"should handle error updating the secret",
			func(run *ScenarioRun[*autoscalerv1alpha1.SecretTypeClusterShardManager]) {
				Expect(run.ReconcileError).To(HaveOccurred())
				Expect(run.ReconcileError.Error()).To(Equal("fake error updating secret"))
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Second))

				By("Checking ready condition")
				Expect(run.Client.GetContainer(ctx, run.Container.ClientObject())).To(Succeed())
				readyCondition := meta.FindStatusCondition(
					run.Container.Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("FailedToUpdateSecret"))
				Expect(readyCondition.Message).To(Equal("fake error updating secret"))
			},
		)

	BeforeEach(func() {
		testNamespace = CreateNamespaceWithRandomName(ctx, k8sClient)
	})

	AfterEach(func() {
		harness.DeleteNamespace(ctx, k8sClient, testNamespace)
		testNamespace = nil
	})

	Context("When reconciling a resource", func() {
		for _, scenario := range collector.All() {
			It(fmt.Sprintf("%s using template %q", scenario.CheckName, scenario.TemplateName), func() {
				scenario.It(ctx, k8sClient, testNamespace)
			})
		}
	})
})
