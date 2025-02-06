package harness

import (
	"context"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectContainer is a simple wrapper over client.Object with easy and simple access to its NamespacedName and ObjectKey.
type ObjectContainer[K client.Object] struct {
	object         K
	objectKey      client.ObjectKey
	namespacedName types.NamespacedName
}

// NewObjectContainer creates a new ObjectContainer with the provided object.
// The object passed in must be initialized with metav1.ObjectMeta, where the NamespacedName and ObjectKey would inferred from.
// Optionally, the set of prep functions is taken in to prepare an object beyond the metadata (i.e. spec).
func NewObjectContainer[K client.Object](obj K, prep ...func(*ObjectContainer[K])) *ObjectContainer[K] {
	container := &ObjectContainer[K]{
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
	container := NewObjectContainer(obj, prep...)
	Expect(client.CreateContainer(ctx, container.ClientObject())).To(Succeed())
	Expect(client.GetContainer(ctx, container.ClientObject())).To(Succeed())
	return container
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
	return NewObjectContainer(clone)
}
