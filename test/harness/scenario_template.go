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

// ScenarioSeedFn is a signature of a function that seeds a resource.
type ScenarioSeedFn[K client.Object] func() K

// ScenarioTemplate is a template for a scenario.
// The seedFn must return a resource initialized with a name.
// Namespace will be added to it automatically during the execution.
// Seed resource might return an object with some additional data, and it will be preserved too.
// However, sometimes it be hard to prepare complex resource with dependencies without access to the Client or the ScenarioRun.
// Hydration functions can be used for that instead.
type ScenarioTemplate[K client.Object] struct {
	templateName string
	seedFn       ScenarioSeedFn[K]
}

// NewScenarioTemplate creates a template of a new scenario.
// The object, must be initialized with metav1.ObjectMeta.Name.
// The object may or may not be initialized with additional data.
func NewScenarioTemplate[K client.Object](
	name string,
	seedFn ScenarioSeedFn[K],
) *ScenarioTemplate[K] {
	return &ScenarioTemplate[K]{
		templateName: name,
		seedFn:       seedFn,
	}
}

func (s *ScenarioTemplate[K]) TemplateName() string {
	return s.templateName
}

func (s *ScenarioTemplate[K]) SeedFn() ScenarioSeedFn[K] {
	return s.seedFn
}

func (s *ScenarioTemplate[K]) Clone() *ScenarioTemplate[K] {
	return &ScenarioTemplate[K]{
		templateName: s.templateName,
		seedFn:       s.seedFn,
	}
}

// Hydrate hydrates the scenario with a base hydration function.
func (s *ScenarioTemplate[K]) Hydrate(
	name string,
	hydrationFn ScenarioHydrationFn[K],
) *ScenarioWithHydration[K] {
	return &ScenarioWithHydration[K]{
		ScenarioTemplate:   s,
		hydrationName:      name,
		hydrationFn:        hydrationFn,
		hydrationPatchesFn: []ScenarioHydrationFn[K]{},
	}
}

// HydrateWithContainer is a shortcut hydration function that instantiates the seed object as a container.
// Container will NOT be created in the cluster.
// Useful if the project to be customized with hydration patches before creation,
// or if the object must not exist for this check.
func (s *ScenarioTemplate[K]) HydrateWithContainer() *ScenarioWithHydration[K] {
	fn := func(run *ScenarioRun[K]) {
		Expect(run).NotTo(BeNil())
		Expect(run.SeedObject).NotTo(BeNil())
		Expect(run.SeedObject.GetName()).NotTo(BeEmpty())
		Expect(run.SeedObject.GetNamespace()).ToNot(BeEmpty())
		container := NewObjectContainer(run.Client.Scheme(), run.SeedObject)
		run.Container = container
	}
	return s.Hydrate("ephemeral container", fn)
}

// HydrateWithClientCreatingContainer is a shortcut hydration function that creates object in k8s after calling HydrateWithContainer.
func (s *ScenarioTemplate[K]) HydrateWithClientCreatingContainer() *ScenarioWithHydration[K] {
	containerFn := s.HydrateWithContainer().hydrationFn
	fn := func(run *ScenarioRun[K]) {
		Expect(run).NotTo(BeNil())
		Expect(run.Client).NotTo(BeNil())
		Expect(run.Container).NotTo(BeNil())
		Expect(run.Container.ClientObject()).NotTo(BeNil())
		Expect(run.Container.Object().GetName()).NotTo(BeEmpty())
		Expect(run.Container.Object().GetNamespace()).NotTo(BeNil())
		Expect(
			run.Client.CreateContainer(run.Context, run.Container.ClientObject()),
		).To(Succeed())
		Expect(
			run.Client.GetContainer(run.Context, run.Container.ClientObject()),
		).To(Succeed())
	}
	n := s.Hydrate("container", func(run *ScenarioRun[K]) {})
	n.hydrationFn = func(run *ScenarioRun[K]) {
		containerFn(run)
		fn(run)
	}
	return n
}
