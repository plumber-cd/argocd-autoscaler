// Copyright 2025
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package harness

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ScenarioReconcilerFn is a signature of a function that returns a reconciler.
type ScenarioReconcilerFn[R reconcile.TypedReconciler[reconcile.Request]] func(client.Client) R

// ScenarioRegisterCallback is a signature of a callback function to register this scenario to the collector.
type ScenarioRegisterCallback[K client.Object] func(*ScenarioCheck[K])

// ScenarioItFn is a signature of a function that is called from ginkgo to run a scenario.
type ScenarioItFn[K client.Object] func(GenericScenarioRun)

// ScenarioIt is a wrapper for ginkgo.
// It represents executable unit of code for the It function in ginkgo.
type ScenarioIt[K client.Object] struct {
	ItStr string
	ItFn  ScenarioItFn[K]
}

// ScenarioContext is a wrapper for ginkgo.
// It represents executable unit of code for the Context function in ginkgo.
type ScenarioContext[K client.Object] struct {
	ContextStr string
	Its        []*ScenarioIt[K]
}

// ScenarioCollector is a collector to instruct ginkgo.
// The idea is that the collector is instantiated once per the ginkgo context.
// Scenarios are getting registered to the collector during configuration phase using ScenarioRegisterCallback.
// In the runtime phase, registered scenarios are executed using their ScenarioItFn functions.
type ScenarioCollector[K client.Object, R reconcile.TypedReconciler[reconcile.Request]] struct {
	reconcilerFn ScenarioReconcilerFn[R]
	checks       []*ScenarioCheck[K]
}

// NewScenarioCollector creates a new scenario collector.
func NewScenarioCollector[K client.Object, R reconcile.TypedReconciler[reconcile.Request]](
	reconcilerFn ScenarioReconcilerFn[R],
) *ScenarioCollector[K, R] {
	return &ScenarioCollector[K, R]{
		reconcilerFn: reconcilerFn,
	}
}

// Collect is a callback function to register a scenario in the collector.
func (c *ScenarioCollector[K, R]) NewRun(ctx context.Context, k8sClient client.Client) *ScenarioRun[K] {
	By("Creating a new run")
	Expect(ctx).NotTo(BeNil())
	Expect(k8sClient).NotTo(BeNil())
	run := &ScenarioRun[K]{
		ctx:        ctx,
		client:     k8sClient,
		fakeClient: NewFakeClient(k8sClient),
	}
	By("Creating a test namespace")
	name := "test-" + string(uuid.NewUUID())
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
	}
	run.namespace = NewObjectContainer(run, namespace).Create()
	return run
}

func (c *ScenarioCollector[K, R]) Cleanup(run GenericScenarioRun) {
	By("Deleting the test namespace")
	run.Namespace().Get().Delete()
}

// Collect is a callback function to register a scenario in the collector.
func (c *ScenarioCollector[K, R]) Collect(check *ScenarioCheck[K]) {
	c.checks = append(c.checks, check)
}

// All is a function that returns all scenarios that was registered as a list of ScenarioIt.
func (c *ScenarioCollector[K, R]) All() []ScenarioContext[K] {
	contexts := map[string]*ScenarioContext[K]{}
	orderedContexts := []string{}
	for _, check := range c.checks {
		contextStr := fmt.Sprintf("using %s scenario", check.templateName)
		if len(check.hydrations) > 0 {
			contextStr += fmt.Sprintf(" and %s patches", strings.Join(check.hydrations, " + "))
		}

		if _, ok := contexts[contextStr]; !ok {
			contexts[contextStr] = &ScenarioContext[K]{
				ContextStr: contextStr,
			}
			orderedContexts = append(orderedContexts, contextStr)
		}
		scenarioContext := contexts[contextStr]

		caseStr := fmt.Sprintf(
			"should %s",
			check.checkName,
		)
		if check.branchName != "" {
			caseStr = fmt.Sprintf(
				"when %s %s",
				check.branchName,
				caseStr,
			)
		}
		scenarioIt := &ScenarioIt[K]{
			ItStr: caseStr,
			ItFn: func(scenarioRun GenericScenarioRun) {
				run, ok := scenarioRun.(*ScenarioRun[K])
				Expect(ok).To(BeTrue())
				By("Seeding scenario using " + check.templateName)
				check.seedFn(run)
				for i, fn := range check.hydrationsFn {
					By("Patching with " + check.hydrations[i])
					fn(run)
				}
				By("Creating reconciler")
				reconciler := c.reconcilerFn(run.fakeClient)
				Expect(reconciler).NotTo(BeNil())
				By("Reconciling")
				result, err := reconciler.Reconcile(run.ctx, reconcile.Request{
					NamespacedName: run.container.NamespacedName(),
				})
				run.reconcileResult = result
				run.reconcileError = err
				By("Checking " + check.checkName)
				check.checkFn(run)
				// By("Simulating error")
				// Fail("Simulating error")
			},
		}
		scenarioContext.Its = append(scenarioContext.Its, scenarioIt)
	}
	contextsList := make([]ScenarioContext[K], len(orderedContexts))
	for i, contextStr := range orderedContexts {
		contextsList[i] = *contexts[contextStr]
	}
	return contextsList
}
