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

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectContainer is a simple wrapper over client.Object with easy and simple access to its NamespacedName and ObjectKey.
type ObjectContainer[K client.Object] struct {
	scheme         *runtime.Scheme
	gvk            schema.GroupVersionKind
	object         K
	objectKey      client.ObjectKey
	namespacedName types.NamespacedName
}

// NewObjectContainer creates a new ObjectContainer with the provided object.
// The object passed in must be initialized with metav1.ObjectMeta, where the NamespacedName and ObjectKey would inferred from.
// Optionally, the set of prep functions is taken in to prepare an object beyond the metadata (i.e. spec).
func NewObjectContainer[K client.Object](scheme *runtime.Scheme, obj K, prep ...func(*ObjectContainer[K])) *ObjectContainer[K] {

	gvks, _, err := scheme.ObjectKinds(obj)
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
		panic("Version is empty")
	}

	container := &ObjectContainer[K]{
		scheme: scheme,
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
		p(container)
	}
	return container
}

func CreateObjectContainer[K client.Object](ctx context.Context, client Client, obj K, prep ...func(*ObjectContainer[K])) *ObjectContainer[K] {
	Expect(obj).NotTo(BeNil())
	Expect(obj.GetName()).NotTo(BeEmpty())
	Expect(obj.GetNamespace()).NotTo(BeEmpty())
	container := NewObjectContainer(client.Scheme(), obj, prep...)
	Expect(client.CreateContainer(ctx, container.ClientObject())).To(Succeed())
	Expect(client.GetContainer(ctx, container.ClientObject())).To(Succeed())
	return container
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
func (c *ObjectContainer[K]) ClientObject() *ObjectContainer[client.Object] {
	return &ObjectContainer[client.Object]{
		object:         c.Object(),
		objectKey:      c.ObjectKey(),
		namespacedName: c.NamespacedName(),
	}
}

// DeepCopy returns a deep copy of the object wrapped in a new instance of ObjectContainer.
func (c *ObjectContainer[K]) DeepCopy() *ObjectContainer[K] {
	clone := c.Object().DeepCopyObject().(K)
	return NewObjectContainer(c.scheme, clone)
}
