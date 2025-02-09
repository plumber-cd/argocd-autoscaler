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

type fakePrometheusPoller struct {
	fn     func(ctx context.Context, poll autoscalerv1alpha1.PrometheusPoll, shards []common.Shard) ([]common.MetricValue, error)
	polled bool
}

func (r *fakePrometheusPoller) Poll(
	ctx context.Context,
	poll autoscalerv1alpha1.PrometheusPoll,
	shards []common.Shard,
) ([]common.MetricValue, error) {
	r.polled = true
	if r.fn != nil {
		return r.fn(ctx, poll, shards)
	}
	return nil, errors.New("poller not implemented")
}

var _ = Describe("PrometheusPoll Controller", func() {
	var scenarioRun GenericScenarioRun

	var poller *fakePrometheusPoller

	var collector = NewScenarioCollector[*autoscalerv1alpha1.PrometheusPoll](
		func(fClient client.Client) *PrometheusPollReconciler {
			return &PrometheusPollReconciler{
				Client: fClient,
				Scheme: fClient.Scheme(),
				Poller: poller,
			}
		},
	)

	NewScenarioTemplate(
		"not existing resource",
		func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
			samplePoll := NewObjectContainer(
				run,
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "not-existent",
						Namespace: run.Namespace().ObjectKey().Name,
					},
				},
			)
			run.SetContainer(samplePoll)
		},
	).
		BranchResourceNotFoundCheck(collector.Collect).
		BranchFailureToGetResourceCheck(collector.Collect)

	NewScenarioTemplate(
		"basic",
		func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
			samplePoll := NewObjectContainer(
				run,
				&autoscalerv1alpha1.PrometheusPoll{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sample-prometheus-poll",
						Namespace: run.Namespace().ObjectKey().Name,
					},
					Spec: autoscalerv1alpha1.PrometheusPollSpec{
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
				func(
					run GenericScenarioRun,
					container *ObjectContainer[*autoscalerv1alpha1.PrometheusPoll],
				) {
					sampleShardManager := NewObjectContainer(
						run,
						&autoscalerv1alpha1.SecretTypeClusterShardManager{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "sample-shard-manager",
								Namespace: run.Namespace().ObjectKey().Name,
							},
						},
					).Create()

					container.Object().Spec.PollerSpec = common.PollerSpec{
						ShardManagerRef: &corev1.TypedLocalObjectReference{
							APIGroup: ptr.To(sampleShardManager.GroupVersionKind().Group),
							Kind:     sampleShardManager.GroupVersionKind().Kind,
							Name:     sampleShardManager.ObjectKey().Name,
						},
					}
				},
			).Create()
			run.SetContainer(samplePoll)
		},
	).
		Branch(
			"malformed",
			func(branch *Scenario[*autoscalerv1alpha1.PrometheusPoll]) {
				branch.Hydrate(
					"duplicate metric",
					func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
						run.Container().Get().Object().Spec.Metrics = append(
							run.Container().Object().Spec.Metrics,
							run.Container().Object().Spec.Metrics[0],
						)
						run.Container().Update()
					},
				).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"handle errors",
						func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
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
							Expect(readyCondition.Reason).To(Equal("MalformedResource"))
							Expect(readyCondition.Message).To(Equal("duplicate metric for ID 'fake-test'"))
						},
					).
					Commit(collector.Collect)
			},
		).
		Branch(
			"failing to get shard manager",
			func(branch *Scenario[*autoscalerv1alpha1.PrometheusPoll]) {
				branch.Hydrate(
					"pointing to non-existing shard manager",
					func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
						run.Container().Get().Object().Spec.PollerSpec.ShardManagerRef.Name = "non-existing-shard-manager"
						run.Container().Update()
					},
				).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"handle errors",
						func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
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
							Expect(readyCondition.Message).To(ContainSubstring("\"non-existing-shard-manager\" not found"))
						},
					).
					Commit(collector.Collect)
			},
		).
		Branch(
			"shard manager not ready",
			func(branch *Scenario[*autoscalerv1alpha1.PrometheusPoll]) {
				branch.Hydrate(
					"shard manager not ready",
					func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
						shardManager := NewObjectContainer(
							run,
							&autoscalerv1alpha1.SecretTypeClusterShardManager{
								ObjectMeta: metav1.ObjectMeta{
									Name:      run.Container().Object().Spec.PollerSpec.ShardManagerRef.Name,
									Namespace: run.Namespace().Object().Name,
								},
							},
						).Get()
						meta.SetStatusCondition(&shardManager.Object().Status.Conditions, metav1.Condition{
							Type:    StatusTypeReady,
							Status:  metav1.ConditionFalse,
							Reason:  "FakeNotReady",
							Message: "Not ready for test",
						})
						shardManager.StatusUpdate()
					},
				).
					BranchFailureToUpdateStatusCheck(collector.Collect).
					WithCheck(
						"update status and exit",
						func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
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
							Expect(readyCondition.Message).To(ContainSubstring("Check the status of shard manager"))
						},
					).
					Commit(collector.Collect)
			},
		).
		Hydrate(
			"add shards to shard manager",
			func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
				shardManager := NewObjectContainer(
					run,
					&autoscalerv1alpha1.SecretTypeClusterShardManager{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Object().Spec.PollerSpec.ShardManagerRef.Name,
							Namespace: run.Namespace().Object().Name,
						},
					},
				).Get()
				shardManager.Object().Status.Shards = []common.Shard{
					{
						UID: types.UID("fake-shard-uid"),
						ID:  "fake-shard-id",
						Data: map[string]string{
							"fake-key": "fake-value",
						},
					},
				}
				meta.SetStatusCondition(&shardManager.Object().Status.Conditions, metav1.Condition{
					Type:    StatusTypeReady,
					Status:  metav1.ConditionTrue,
					Reason:  StatusTypeReady,
					Message: StatusTypeReady,
				})
				shardManager.StatusUpdate()
			},
		).
		Branch(
			"recently polled",
			func(branch *Scenario[*autoscalerv1alpha1.PrometheusPoll]) {
				branch.Hydrate(
					"status with recent poll",
					func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
						shardManager := NewObjectContainer(
							run,
							&autoscalerv1alpha1.SecretTypeClusterShardManager{
								ObjectMeta: metav1.ObjectMeta{
									Name:      run.Container().Object().Spec.PollerSpec.ShardManagerRef.Name,
									Namespace: run.Namespace().Object().Name,
								},
							},
						).Get()

						run.Container().Get().Object().Status.Values = []common.MetricValue{}
						for _, shard := range shardManager.Object().Status.Shards {
							for _, metric := range run.Container().Object().Spec.Metrics {
								run.Container().Object().Status.Values = append(
									run.Container().Object().Status.Values,
									common.MetricValue{
										ID:           metric.ID,
										Shard:        shard,
										Query:        metric.Query,
										Value:        resource.MustParse("0"),
										DisplayValue: "0",
									})
							}
						}
						run.Container().Object().Status.LastPollingTime = ptr.To(metav1.NewTime(
							time.Now().Add(-30 * time.Second),
						))
						run.Container().StatusUpdate()
					},
				).
					WithCheck(
						"re-queue for the remainder of the period",
						func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
							Expect(run.ReconcileError()).NotTo(HaveOccurred())
							Expect(run.ReconcileResult().RequeueAfter).To(BeNumerically("~", 30*time.Second, 2*time.Second))
							Expect(run.ReconcileResult().Requeue).To(BeFalse())
						},
					).
					Commit(collector.Collect).
					Branch(
						"recently polled but changed shard manager",
						func(branch *Scenario[*autoscalerv1alpha1.PrometheusPoll]) {
							branch.Hydrate(
								"changed shard manager replicas",
								func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
									shardManager := NewObjectContainer(
										run,
										&autoscalerv1alpha1.SecretTypeClusterShardManager{
											ObjectMeta: metav1.ObjectMeta{
												Name:      run.Container().Object().Spec.PollerSpec.ShardManagerRef.Name,
												Namespace: run.Namespace().Object().Name,
											},
										},
									).Get()
									shardManager.Object().Status.Shards = append(shardManager.Object().Status.Shards, common.Shard{
										UID: types.UID("fake-shard-uid-2"),
										ID:  "fake-shard-id-2",
										Data: map[string]string{
											"fake-key": "fake-value",
										},
									})
									shardManager.StatusUpdate()
								},
							).
								WithCheck(
									"poll immediately",
									func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
										Expect(run.ReconcileError()).To(HaveOccurred())
										Expect(run.ReconcileError().Error()).To(ContainSubstring("poller not implemented"))
									},
								).
								Commit(collector.Collect)
						},
					).
					Branch(
						"recently polled but changed metrics",
						func(branch *Scenario[*autoscalerv1alpha1.PrometheusPoll]) {
							branch.Hydrate(
								"changed metrics",
								func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
									run.Container().Get().Object().Spec.Metrics = append(
										run.Container().Object().Spec.Metrics,
										autoscalerv1alpha1.PrometheusMetric{
											ID:     "fake-test-2",
											Query:  "fake-query-2",
											NoData: ptr.To(resource.MustParse("0")),
										},
									)
									run.Container().Update()
								},
							).
								WithCheck(
									"poll immediately",
									func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
										Expect(run.ReconcileError()).To(HaveOccurred())
										Expect(run.ReconcileError().Error()).To(ContainSubstring("poller not implemented"))
									},
								).
								Commit(collector.Collect)
						},
					)
			},
		).
		Hydrate(
			"functioning poller",
			func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
				poller.fn = func(
					ctx context.Context,
					poll autoscalerv1alpha1.PrometheusPoll,
					shards []common.Shard,
				) ([]common.MetricValue, error) {
					metricValues := []common.MetricValue{}
					for _, shard := range shards {
						for _, metric := range poll.Spec.Metrics {
							metricValues = append(
								metricValues,
								common.MetricValue{
									ID:           metric.ID,
									Shard:        shard,
									Query:        metric.Query,
									Value:        resource.MustParse("1"),
									DisplayValue: "1",
								},
							)
						}
					}
					return metricValues, nil
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"poll successfully",
			func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
				Expect(poller.polled).To(BeTrue())
				Expect(run.ReconcileError()).ToNot(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(run.Container().Object().Spec.Period.Duration))

				By("Checking conditions")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(StatusTypeReady))

				shardManager := NewObjectContainer(
					run,
					&autoscalerv1alpha1.SecretTypeClusterShardManager{
						ObjectMeta: metav1.ObjectMeta{
							Name:      run.Container().Object().Spec.PollerSpec.ShardManagerRef.Name,
							Namespace: run.Namespace().Object().Name,
						},
					},
				).Get()

				By("Checking polling results")
				metricsById := map[string]autoscalerv1alpha1.PrometheusMetric{}
				for _, metric := range run.Container().Object().Spec.Metrics {
					metricsById[metric.ID] = metric
				}
				shardsByUID := map[types.UID]common.Shard{}
				for _, shard := range shardManager.Object().Status.Shards {
					shardsByUID[shard.UID] = shard
				}
				metricValues := run.Container().Object().Status.Values
				Expect(metricValues).
					To(HaveLen(
						len(run.Container().Object().Spec.Metrics) *
							len(shardManager.Object().Status.Shards),
					))
				valuesByMetricIDAndShardUID := map[string]common.MetricValue{}
				for _, val := range metricValues {
					valuesByMetricIDAndShardUID[val.ID+":"+string(val.Shard.UID)] = val
				}
				for metricID, metric := range metricsById {
					for shardUID, shard := range shardsByUID {
						key := metricID + ":" + string(shardUID)
						val, ok := valuesByMetricIDAndShardUID[key]
						Expect(ok).To(BeTrue())
						Expect(val.ID).To(Equal(metric.ID))
						Expect(val.Shard).To(Equal(shard))
						Expect(val.Query).To(Equal(metric.Query))
						Expect(val.Value).To(Equal(resource.MustParse("1")))
						Expect(val.DisplayValue).To(Equal("1"))
					}
				}
			},
		).
		Commit(collector.Collect).
		Hydrate(
			"failing poller",
			func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
				poller.fn = func(
					ctx context.Context,
					poll autoscalerv1alpha1.PrometheusPoll,
					shards []common.Shard,
				) ([]common.MetricValue, error) {
					return nil, errors.New("fake error")
				}
			},
		).
		BranchFailureToUpdateStatusCheck(collector.Collect).
		WithCheck(
			"handle polling error",
			func(run *ScenarioRun[*autoscalerv1alpha1.PrometheusPoll]) {
				Expect(poller.polled).To(BeTrue())
				Expect(run.ReconcileError()).To(HaveOccurred())
				Expect(run.ReconcileError().Error()).To(ContainSubstring("fake error"))
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Second))

				By("Checking conditions")
				readyCondition := meta.FindStatusCondition(
					run.Container().Get().Object().Status.Conditions,
					StatusTypeReady,
				)
				Expect(readyCondition).NotTo(BeNil())
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				Expect(readyCondition.Reason).To(Equal("PollingError"))
				Expect(readyCondition.Message).To(ContainSubstring("fake error"))
			},
		).
		Commit(collector.Collect)

	BeforeEach(func() {
		poller = &fakePrometheusPoller{}
		scenarioRun = collector.NewRun(ctx, k8sClient)
	})

	AfterEach(func() {
		collector.Cleanup(scenarioRun)
		scenarioRun = nil
		poller = nil
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
