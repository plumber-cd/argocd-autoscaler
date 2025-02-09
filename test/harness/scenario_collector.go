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
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ScenarioReconcilerFn is a signature of a function that returns a reconciler.
type ScenarioReconcilerFn[R reconcile.TypedReconciler[reconcile.Request]] func(client.Client) R

// ScenarioRegisterCallback is a signature of a callback function to register this scenario to the collector.
type ScenarioRegisterCallback[K client.Object] func(*ScenarioWithCheck[K])

// ScenarioItFn is a signature of a function that is called from ginkgo to run a scenario.
type ScenarioItFn func(context.Context, *Client, *ObjectContainer[*corev1.Namespace])

// ScenarioIt is a wrapper for ginkgo.
// It represents executable unit of code for the It function in ginkgo.
type ScenarioIt struct {
	ItStr string
	ItFn  ScenarioItFn
}

// ScenarioContext is a wrapper for ginkgo.
// It represents executable unit of code for the Context function in ginkgo.
type ScenarioContext struct {
	ContextStr string
	Its        []*ScenarioIt
}

// ScenarioCollector is a collector to instruct ginkgo.
// The idea is that the collector is instantiated once per the ginkgo context.
// Scenarios are getting registered to the collector during configuration phase using ScenarioRegisterCallback.
// In the runtime phase, registered scenarios are executed using their ScenarioItFn functions.
type ScenarioCollector[K client.Object, R reconcile.TypedReconciler[reconcile.Request]] struct {
	reconcilerFn ScenarioReconcilerFn[R]
	checks       []*ScenarioWithCheck[K]
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
func (c *ScenarioCollector[K, R]) Collect(check *ScenarioWithCheck[K]) {
	c.checks = append(c.checks, check)
}

// All is a function that returns all scenarios that was registered as a list of ScenarioIt.
func (c *ScenarioCollector[K, R]) All() []ScenarioContext {
	contexts := map[string]*ScenarioContext{}
	orderedContexts := []string{}
	for _, check := range c.checks {
		contextStr := fmt.Sprintf(
			"using %s template with %s hydration using %s client",
			check.templateName,
			strings.Join(append([]string{check.hydrationName}, check.hydrationPatches...), " + "),
			check.fakeClientName,
		)
		if len(check.fakeClientPatches) > 0 {
			contextStr += fmt.Sprintf(" that is %s", strings.Join(check.fakeClientPatches, " and "))
		}

		if _, ok := contexts[contextStr]; !ok {
			contexts[contextStr] = &ScenarioContext{
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
		scenarioIt := &ScenarioIt{
			ItStr: caseStr,
			ItFn: func(ctx context.Context, client *Client, ns *ObjectContainer[*corev1.Namespace]) {
				By("Creating scenario run")
				run := &ScenarioRun[K]{
					Context:   ctx,
					Client:    client,
					Namespace: ns,
				}
				By("Seeding object for template " + check.templateName)
				run.SeedObject = check.seedFn()
				run.SeedObject.SetNamespace(run.Namespace.Object().Name)
				By("Hydrating " + check.hydrationName)
				check.hydrationFn(run)
				for i, fn := range check.hydrationPatchesFn {
					By("Patching hydration with " + check.hydrationPatches[i])
					fn(run)
				}
				By("Creating fake client " + check.fakeClientName)
				check.fakeClientFn(run)
				for i, fn := range check.fakeClientPatchesFn {
					By("Patching fake client with " + check.fakeClientPatches[i])
					fn(run)
				}
				By("Creating reconciler")
				reconciler := c.reconcilerFn(run.FakeClient)
				By("Reconciling")
				result, err := reconciler.Reconcile(run.Context, reconcile.Request{
					NamespacedName: run.Container.NamespacedName(),
				})
				run.ReconcileResult = result
				run.ReconcileError = err
				By("Checking " + check.checkName)
				check.checkFn(run)
				// By("Simulating error")
				// Fail("Simulating error")
			},
		}
		scenarioContext.Its = append(scenarioContext.Its, scenarioIt)
	}
	contextsList := make([]ScenarioContext, len(orderedContexts))
	for i, contextStr := range orderedContexts {
		contextsList[i] = *contexts[contextStr]
	}
	return contextsList
}
