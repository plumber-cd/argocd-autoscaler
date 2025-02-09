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
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScenarioFakeClientFn is a signature of a function that adds a fake client to the ScenarioRun.
// This client may be modified to mock certain calls in order to reproduce this scenario.
// It is stored as FakeClient on the run.
type ScenarioFakeClientFn[K client.Object] func(*ScenarioRun[K])

// ScenarioWithFakeClient is a hydrated scenario with functions that can prepare a fake client.
// Fake client is used during the scenario to simulate certain conditions.
// It may or may not have mocks registered on it.
// Functions must store the client to the ScenarioRun as FakeClient.
// Separation of base fake client function and patches allow to provide a reusable baseline for scenario reusability.
// Patches can be discarded separately from the base to be able to reuse the base with different set of patches to reproduce totally different scenario.
type ScenarioWithFakeClient[K client.Object] struct {
	*ScenarioWithHydration[K]
	fakeClientName      string
	fakeClientFn        ScenarioFakeClientFn[K]
	fakeClientPatches   []string
	fakeClientPatchesFn []ScenarioFakeClientFn[K]
}

func (s *ScenarioWithFakeClient[K]) Clone() *ScenarioWithFakeClient[K] {
	return &ScenarioWithFakeClient[K]{
		ScenarioWithHydration: s.ScenarioWithHydration.Clone(),
		fakeClientName:        s.fakeClientName,
		fakeClientFn:          s.fakeClientFn,
		fakeClientPatches:     append([]string{}, s.fakeClientPatches...),
		fakeClientPatchesFn:   append([]ScenarioFakeClientFn[K]{}, s.fakeClientPatchesFn...),
	}
}

// Hydrate extends this scenario with additional hydration functions, without reset to the state without the client.
func (s *ScenarioWithFakeClient[K]) Hydrate(
	name string,
	hydrationFn ScenarioHydrationFn[K],
) *ScenarioWithFakeClient[K] {
	s.ScenarioWithHydration.Hydrate(name, hydrationFn)
	return s
}

// WithFakeClient adds fake client patch functions to the scenario.
func (s *ScenarioWithFakeClient[K]) WithFakeClient(
	name string,
	fakeClientFn ...ScenarioFakeClientFn[K],
) *ScenarioWithFakeClient[K] {
	s.fakeClientPatches = append(s.fakeClientPatches, name)
	s.fakeClientPatchesFn = append(s.fakeClientPatchesFn, fakeClientFn...)
	return s
}

// WithFakeClientThatFailsToGetResource adds additional mock to simulate failure to get the resource.
func (s *ScenarioWithFakeClient[K]) WithFakeClientThatFailsToGetResource() *ScenarioWithFakeClient[K] {
	fn := func(run *ScenarioRun[K]) {
		run.FakeClient.
			WithGetFunction(run.Container.ClientObject(),
				func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.NewBadRequest("fake error getting resource")
				},
			)
	}
	return s.WithFakeClient("failing to get the resource", fn)
}

// WithFakeClientThatFailsToUpdateStatus adds additional mock to simulate failure to update the status.
func (s *ScenarioWithFakeClient[K]) WithFakeClientThatFailsToUpdateStatus() *ScenarioWithFakeClient[K] {
	fn := func(run *ScenarioRun[K]) {
		Expect(run).NotTo(BeNil())
		Expect(run.FakeClient).NotTo(BeNil())
		run.FakeClient.
			WithStatusUpdateFunction(run.Container.ClientObject(),
				func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					return errors.NewBadRequest("fake error updating status")
				},
			)
	}
	return s.WithFakeClient("failing to update status", fn)
}

// RemoveClient returns HydratedScenario ready to be instantiated with another client.
func (s *ScenarioWithFakeClient[K]) RemoveClient() *ScenarioWithHydration[K] {
	return s.ScenarioWithHydration.Clone()
}

// ResetClientPatches returns a copy of ScenarioWithFakeClient but without the patches.
func (s *ScenarioWithFakeClient[K]) ResetClientPatches() *ScenarioWithFakeClient[K] {
	clone := s.Clone()
	clone.fakeClientPatches = []string{}
	clone.fakeClientPatchesFn = []ScenarioFakeClientFn[K]{}
	return clone
}

// WithCheck creates a ScenarioWithCheck.
func (s *ScenarioWithFakeClient[K]) WithCheck(
	checkName string,
	checkFn ScenarioCheckFn[K],
) *ScenarioWithCheck[K] {
	return &ScenarioWithCheck[K]{
		ScenarioWithFakeClient: s,
		checkName:              checkName,
		checkFn:                checkFn,
	}
}

// Branch allows to branch out a scenario as a copy for additional checks without the intention to extend it.
// Within the branch - scenario can continue like normally.
// The function returns the original scenario, so it can be chained further down by "forgetting" the branch timeline.
func (s *ScenarioWithFakeClient[K]) Branch(
	name string,
	branch func(*ScenarioWithFakeClient[K]),
) *ScenarioWithFakeClient[K] {
	clone := s.Clone()
	clone.branchName = name
	branch(clone)
	return s
}

// BranchResourceNotFoundCheck is a shortcut function that uses scenario client and a set of checks that are expecting clean exit with no re-queue.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *ScenarioWithFakeClient[K]) BranchResourceNotFoundCheck(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	s.Branch("resource not found", func(branch *ScenarioWithFakeClient[K]) {
		branch.
			WithCheck("successfully exit", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError).NotTo(HaveOccurred())
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Duration(0)))
				Expect(run.ReconcileResult.Requeue).To(BeFalse())
			}).
			Commit(callback)
	})
	return s
}

// BranchFailureToGetResourceCheck is a shortcut function that uses a WithClientFromParentThatFailsToGetResource() and a set of checks that are expecting a corresponding error from it.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *ScenarioWithFakeClient[K]) BranchFailureToGetResourceCheck(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	s.Branch("failure to get resource", func(branch *ScenarioWithFakeClient[K]) {
		branch.
			WithFakeClientThatFailsToGetResource().
			WithCheck("handle errors", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError).To(HaveOccurred())
				Expect(run.ReconcileError.Error()).To(Equal("fake error getting resource"))
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Second))
				Expect(run.ReconcileResult.Requeue).To(BeFalse())
			}).
			Commit(callback)
	})
	return s
}

// BranchFailureToUpdateStatusCheck is a shortcut function that uses a WithClientFromParentThatFailsToUpdateStatus() and a set of check that are expecting a corresponding error from it.
// Acts as a separate branch, does not modify the state, and allows to continue the scenario without impacting it.
func (s *ScenarioWithFakeClient[K]) BranchFailureToUpdateStatusCheck(callback ScenarioRegisterCallback[K]) *ScenarioWithFakeClient[K] {
	s.Branch("failure to update status", func(branch *ScenarioWithFakeClient[K]) {
		branch.
			WithFakeClientThatFailsToUpdateStatus().
			WithCheck("handle errors", func(run *ScenarioRun[K]) {
				Expect(run.ReconcileError).To(HaveOccurred())
				Expect(run.ReconcileError.Error()).To(Equal("fake error updating status"))
				Expect(run.ReconcileResult.RequeueAfter).To(Equal(time.Second))
			}).
			Commit(callback)
	})
	return s
}
