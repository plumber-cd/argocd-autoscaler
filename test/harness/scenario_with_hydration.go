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
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScenarioHydrationFn is a signature of a function that hydrates resources for this ScenarioRun.
// It stores main resource as a Container on the run, but there could be additional resources created in the cluster too.
// The checks in this scenario should be able to find these additional resources on their own.
type ScenarioHydrationFn[K client.Object] func(*ScenarioRun[K])

// ScenarioWithHydration is a scenario instantiated from a template with a set of hydration functions.
// Hydration functions are used to prepare resources for the test, and create them in the cluster.
// Hydration functions are expected to store main resource as the Container on this ScenarioRun.
// Separation of base hydration function and patches allow to provide a reusable baseline for scenario reusability.
// Patches can be discarded separately from the base to be able to reuse the base with different set of patches to reproduce totally different scenario.
type ScenarioWithHydration[K client.Object] struct {
	*ScenarioTemplate[K]
	branchName         string
	hydrationName      string
	hydrationFn        ScenarioHydrationFn[K]
	hydrationPatches   []string
	hydrationPatchesFn []ScenarioHydrationFn[K]
}

func (s *ScenarioWithHydration[K]) Clone() *ScenarioWithHydration[K] {
	return &ScenarioWithHydration[K]{
		ScenarioTemplate:   s.ScenarioTemplate.Clone(),
		branchName:         s.branchName,
		hydrationName:      s.hydrationName,
		hydrationFn:        s.hydrationFn,
		hydrationPatches:   append([]string{}, s.hydrationPatches...),
		hydrationPatchesFn: append([]ScenarioHydrationFn[K]{}, s.hydrationPatchesFn...),
	}
}

// Hydrate adds hydration patch to this scenario.
func (s *ScenarioWithHydration[K]) Hydrate(
	name string,
	hydrationFn ScenarioHydrationFn[K],
) *ScenarioWithHydration[K] {
	s.hydrationPatchesFn = append(s.hydrationPatchesFn, hydrationFn)
	s.hydrationPatches = append(s.hydrationPatches, name)
	return s
}

// DeHydrate returns a scenario ScenarioTemplate ready to be re-hydrated with something else.
func (s *ScenarioWithHydration[K]) DeHydrate() *ScenarioTemplate[K] {
	return s.ScenarioTemplate.Clone()
}

// ResetHydrationPatches creates a new clone of this ScenarioWithHydration but without hydration patches.
func (s *ScenarioWithHydration[K]) ResetHydrationPatches() *ScenarioWithHydration[K] {
	clone := s.Clone()
	clone.hydrationPatches = []string{}
	clone.hydrationPatchesFn = []ScenarioHydrationFn[K]{}
	return clone
}

// WithFakeClient adds a fake client base function to the scenario.
// If fakeClientFn is nil - will use a clean client as fake client in the scenario.
// It means that no mocks are necessary for this scenario.
func (s *ScenarioWithHydration[K]) WithFakeClient(
	name string,
	fakeClientFn ScenarioFakeClientFn[K],
) *ScenarioWithFakeClient[K] {
	if fakeClientFn == nil {
		fakeClientFn = func(run *ScenarioRun[K]) {
			Expect(run).NotTo(BeNil())
			Expect(run.Client).NotTo(BeNil())
			run.FakeClient = &FakeClient{Client: run.Client}
		}
	}
	return &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s,
		fakeClientName:        name,
		fakeClientFn:          fakeClientFn,
		fakeClientPatches:     []string{},
		fakeClientPatchesFn:   []ScenarioFakeClientFn[K]{},
	}
}

// Branch allows to branch out a scenario as a copy for additional checks without the intention to extend it.
// Within the branch - scenario can continue like normally.
// The function returns the original scenario, so it can be chained further down by "forgetting" the branch timeline.
func (s *ScenarioWithHydration[K]) Branch(
	name string,
	branch func(*ScenarioWithHydration[K]),
) *ScenarioWithHydration[K] {
	clone := s.Clone()
	clone.branchName = name
	branch(clone)
	return s
}
