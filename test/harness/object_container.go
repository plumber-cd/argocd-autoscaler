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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectContainer is a wrapper over client.Object with easy and simple access to its NamespacedName and ObjectKey.
// It stores its own context and client, so the object can be manipulated with to re-create specific testing scenario.
// The client is never a fake client.
type ObjectContainer[K client.Object] struct {
	run            GenericScenarioRun
	gvk            schema.GroupVersionKind
	object         K
	objectKey      client.ObjectKey
	namespacedName types.NamespacedName
}

// NewObjectContainer creates a new ObjectContainer with the provided object.
// The object passed in must be initialized with metav1.ObjectMeta,
// where the NamespacedName and ObjectKey would inferred from.
// Optionally, the set of prep functions is taken in to prepare an object beyond the metadata (i.e. spec).
// This can use unprepared client, including client.Client. It does not have to be the one used in a ScenarioRun.
// NOTE that this function does NOT create a resource in the cluster. The client is only used for schema and RESTMapper.
// Client is, however, saved for later use for Create/Get/Update/UpdateStatus/Delete methods.
func NewObjectContainer[K client.Object](
	run GenericScenarioRun,
	obj K,
	prep ...func(GenericScenarioRun, *ObjectContainer[K]),
) *ObjectContainer[K] {
	gvks, _, err := run.Client().Scheme().ObjectKinds(obj)
	if err != nil {
		panic(err)
	}

	if len(gvks) < 1 {
		panic("no GVKs found")
	}

	if len(gvks) > 1 {
		panic("multiple GVKs found")
	}

	gvk := gvks[0]

	if gvk.Version == "" {
		gk := schema.GroupKind{
			Group: gvk.Group,
			Kind:  gvk.Kind,
		}

		mapping, err := run.Client().RESTMapper().RESTMapping(gk, "")
		if err != nil {
			panic("Version is empty")
		}
		gvk.Version = mapping.GroupVersionKind.Version
	}

	container := &ObjectContainer[K]{
		run:    run,
		gvk:    gvk,
		object: obj,
		objectKey: client.ObjectKey{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
		namespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
	}
	for _, p := range prep {
		p(run, container)
	}
	return container
}

// Create wraps client.Client.Create method with this ObjectContainer.
func (c *ObjectContainer[K]) Create() *ObjectContainer[K] {
	Expect(c.run.Client().Create(c.run.Context(), c.Object())).To(Succeed())
	return c.Get()
}

// Get wraps client.Client.Get method with this ObjectContainer.
func (c *ObjectContainer[K]) Get() *ObjectContainer[K] {
	Expect(c.run.Client().Get(c.run.Context(), c.ObjectKey(), c.Object())).To(Succeed())
	return c
}

// UpdateContainer wraps client.Client.Update method with with ObjectContainer.
func (c *ObjectContainer[K]) Update() *ObjectContainer[K] {
	Expect(c.run.Client().Update(c.run.Context(), c.Object())).To(Succeed())
	return c.Get()
}

// StatusUpdateContainer wraps client.Client.Status().Update method with this ObjectContainer.
func (c *ObjectContainer[K]) StatusUpdate() *ObjectContainer[K] {
	Expect(c.run.Client().Status().Update(c.run.Context(), c.Object())).To(Succeed())
	return c.Get()
}

// DeleteContainer wraps client.Client.Delete method with this ObjectContainer.
func (c *ObjectContainer[K]) Delete() *ObjectContainer[K] {
	Expect(c.run.Client().Delete(c.run.Context(), c.Object())).To(Succeed())
	return c
}

func (c *ObjectContainer[K]) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

// Object returns the object.
func (c *ObjectContainer[K]) Object() K {
	return c.object
}

// ObjectKey returns the object key.
func (c *ObjectContainer[K]) ObjectKey() client.ObjectKey {
	return c.objectKey
}

// NamespacedName returns the namespaced name.
func (c *ObjectContainer[K]) NamespacedName() types.NamespacedName {
	return c.namespacedName
}

// ClientObject returns non-generic ObjectContainer with client.Object type.
// Might be useful to avoid casting when client function is expecting client.Object.
func (c *ObjectContainer[K]) ClientObject() client.Object {
	return c.Object()
}

// DeepCopy returns a deep copy of the object wrapped in a new instance of ObjectContainer.
func (c *ObjectContainer[K]) DeepCopy() *ObjectContainer[K] {
	clone := c.Object().DeepCopyObject().(K)
	return NewObjectContainer(c.run, clone)
}
