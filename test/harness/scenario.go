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
	"errors"
	"time"

	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScenarioSeedFn is a signature of a function that seeds a resource.
type ScenarioSeedFn[K client.Object] func(run *ScenarioRun[K])

// ScenarioHydrationFn is a signature of a function that hydrates resources and fake client for this ScenarioRun.
// The client may be modified to mock certain calls in order to reproduce this scenario.
// At least one hydration function must populate run.FakeClient.
type ScenarioHydrationFn[K client.Object] func(*ScenarioRun[K])

// Scenario is a scenario instantiated from a template with a fake client function and a set of hydration functions.
// Fake client function is used to instantiate a fake client, possibly with mocks.
// Hydration functions are used to prepare resources for the test, and create them in the cluster.
type Scenario[K client.Object] struct {
	templateName string
	seedFn       ScenarioSeedFn[K]

	branchName string

	hydrations   []string
	hydrationsFn []ScenarioHydrationFn[K]
}

// NewScenarioTemplate creates a template of a new scenario.
func NewScenarioTemplate[K client.Object](
	name string,
	seedFn ScenarioSeedFn[K],
) *Scenario[K] {
	return &Scenario[K]{
		templateName: name,
		seedFn:       seedFn,

		hydrations:   []string{},
		hydrationsFn: []ScenarioHydrationFn[K]{},
	}
}

func (s *Scenario[K]) Clone() *Scenario[K] {
	return &Scenario[K]{
		templateName: s.templateName,
		seedFn:       s.seedFn,

		branchName: s.branchName,

		hydrations:   append([]string{}, s.hydrations...),
		hydrationsFn: append([]ScenarioHydrationFn[K]{}, s.hydrationsFn...),
	}
}

// Hydrate adds hydration patch to this scenario.
func (s *Scenario[K]) Hydrate(
	name string,
	hydrationFn ScenarioHydrationFn[K],
) *Scenario[K] {
	s.hydrations = append(s.hydrations, name)
	s.hydrationsFn = append(s.hydrationsFn, hydrationFn)
	return s
}

// DeHydrate returns a clone of this Scenario ready to be re-hydrated with something else.
func (s *Scenario[K]) DeHydrate() *Scenario[K] {
	clone := s.Clone()
	clone.hydrations = []string{}
	clone.hydrationsFn = []ScenarioHydrationFn[K]{}
	return clone
}

// RemoveLastHydration removes one hydration from the scenario that was added last.
func (s *Scenario[K]) RemoveLastHydration() *Scenario[K] {
	s.hydrations = s.hydrations[:len(s.hydrations)-1]
	s.hydrationsFn = s.hydrationsFn[:len(s.hydrationsFn)-1]
	return s
}

// WithFakeClientThatFailsToGetResource adds additional mock to simulate failure to get the resource.
func (s *Scenario[K]) WithFakeClientThatFailsToGetResource() *Scenario[K] {
	fn := func(run *ScenarioRun[K]) {
		run.FakeClient().
			WithGetFunction(run.Container(),
				func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.New("fake error getting resource")
				},
			)
	}
	return s.Hydrate("failing to get the resource", fn)
}

// WithFakeClientThatFailsToUpdateStatus adds additional mock to simulate failure to update the status.
func (s *Scenario[K]) WithFakeClientThatFailsToUpdateStatus() *Scenario[K] {
	fn := func(run *ScenarioRun[K]) {
		run.FakeClient().
			WithStatusUpdateFunction(run.Container(),
				func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					return errors.New("fake error updating status")
				},
			)
	}
	return s.Hydrate("failing to update status", fn)
}

// WithCheck creates a ScenarioCheck.
func (s *Scenario[K]) WithCheck(
	checkName string,
	checkFn ScenarioCheckFn[K],
) *ScenarioCheck[K] {
	return &ScenarioCheck[K]{
		Scenario:  s,
		checkName: checkName,
		checkFn:   checkFn,
	}
}

// Branch allows to branch out a scenario as a copy for additional checks without the intention to extend it.
// Within the branch - scenario can continue like normally.
// The function returns the original scenario, so it can be chained further down by "forgetting" the branch timeline.
func (s *Scenario[K]) Branch(
	name string,
	branch func(*Scenario[K]),
) *Scenario[K] {
	clone := s.Clone()
	clone.branchName = name
	branch(clone)
	return s
}

// BranchResourceNotFoundCheck is a shortcut function that uses fake client
// and a set of checks that are expecting clean exit with no re-queue.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *Scenario[K]) BranchResourceNotFoundCheck(callback ScenarioRegisterCallback[K]) *Scenario[K] {
	s.Branch("resource not found", func(branch *Scenario[K]) {
		branch.
			WithCheck("successfully exit", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError()).NotTo(HaveOccurred())
				Expect(run.ReconcileResult().RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult().Requeue).To(BeFalse())
			}).
			Commit(callback)
	})
	return s
}

// BranchFailureToGetResourceCheck is a shortcut function that uses a WithClientFromParentThatFailsToGetResource()
// and a set of checks that are expecting a corresponding error from it.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *Scenario[K]) BranchFailureToGetResourceCheck(callback ScenarioRegisterCallback[K]) *Scenario[K] {
	s.Branch("failure to get resource", func(branch *Scenario[K]) {
		branch.
			WithFakeClientThatFailsToGetResource().
			WithCheck("handle errors", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError()).To(HaveOccurred())
				Expect(run.ReconcileError().Error()).To(Equal("fake error getting resource"))
			}).
			Commit(callback)
	})
	return s
}

// BranchFailureToUpdateStatusCheck is a shortcut function that uses a WithClientFromParentThatFailsToUpdateStatus()
// and a set of check that are expecting a corresponding error from it.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *Scenario[K]) BranchFailureToUpdateStatusCheck(callback ScenarioRegisterCallback[K]) *Scenario[K] {
	s.Branch("failure to update status", func(branch *Scenario[K]) {
		branch.
			WithFakeClientThatFailsToUpdateStatus().
			WithCheck("handle errors", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError()).To(HaveOccurred())
				Expect(run.ReconcileError().Error()).To(Equal("fake error updating status"))
			}).
			Commit(callback)
	})
	return s
}
