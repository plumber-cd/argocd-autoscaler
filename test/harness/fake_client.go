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
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetFn is the client.Client.Get function signature
type GetFn func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error

// ListFn is the client.Client.List function signature
type ListFn func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error

// UpdateFn is the client.Client.Update function signature
type UpdateFn func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error

// UpdateSubResourceFn is the client.Client.Status().Update function signature
type UpdateSubResourceFn func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error

// DeleteFn is the client.Client.Delete function signature
type DeleteFn func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error

// GenericObjectContainer is an interface for ObjectContainer that doesn't require to know its generic type.
type GenericObjectContainer interface {
	ClientObject() client.Object
	ObjectKey() client.ObjectKey
}

// FakeClient is a fake client.Client implementation.
// It allows to register custom Get, List, Update and Delete functions for specific objects.
type FakeClient struct {
	client.Client
	getFunctions          map[reflect.Type]map[string]GetFn
	listFunctions         map[reflect.Type]ListFn
	updateFunctions       map[reflect.Type]map[string]UpdateFn
	statusUpdateFunctions map[reflect.Type]map[string]UpdateSubResourceFn
	deleteFn              map[reflect.Type]map[string]DeleteFn
}

// NewFakeClient returns a new FakeClient instance.
func NewFakeClient(client client.Client) *FakeClient {
	return &FakeClient{
		Client: client,
	}
}

// FakeStatusWriter is a fake client.StatusWriter implementation.
// It allows to register custom Update functions for specific objects.
type FakeStatusWriter struct {
	client.StatusWriter
	parent *FakeClient
}

// WithGetFunction registers a custom Get function for the given object.
func (f *FakeClient) WithGetFunction(container GenericObjectContainer, fn GetFn) *FakeClient {
	if f.getFunctions == nil {
		f.getFunctions = make(map[reflect.Type]map[string]GetFn)
	}
	if f.getFunctions[reflect.TypeOf(container.ClientObject())] == nil {
		f.getFunctions[reflect.TypeOf(container.ClientObject())] = make(map[string]GetFn)
	}
	f.getFunctions[reflect.TypeOf(container.ClientObject())][container.ObjectKey().String()] = fn
	return f
}

// Get calls the custom Get function if it was registered for the given object.
// Otherwise, it calls the original client.Client.Get function.
func (f *FakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if f.getFunctions != nil {
		if functions, ok := f.getFunctions[reflect.TypeOf(obj)]; ok {
			if fn, ok := functions[key.String()]; ok {
				return fn(ctx, key, obj, opts...)
			}
		}
	}
	return f.Client.Get(ctx, key, obj, opts...)
}

// WithListFunction registers a custom List function for the given object list.
func (f *FakeClient) WithListFunction(list client.ObjectList, fn ListFn) *FakeClient {
	if f.listFunctions == nil {
		f.listFunctions = make(map[reflect.Type]ListFn)
	}
	f.listFunctions[reflect.TypeOf(list)] = fn
	return f
}

// List calls the custom List function if it was registered for the given object list.
// Otherwise, it calls the original client.Client.List function.
func (f *FakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if f.listFunctions != nil {
		if fn, ok := f.listFunctions[reflect.TypeOf(list)]; ok {
			return fn(ctx, list, opts...)
		}
	}
	return f.Client.List(ctx, list, opts...)
}

// WithUpdateFunction registers a custom Update function for the given object.
func (f *FakeClient) WithUpdateFunction(container GenericObjectContainer, fn UpdateFn) *FakeClient {
	if f.updateFunctions == nil {
		f.updateFunctions = make(map[reflect.Type]map[string]UpdateFn)
	}
	if f.updateFunctions[reflect.TypeOf(container.ClientObject())] == nil {
		f.updateFunctions[reflect.TypeOf(container.ClientObject())] = make(map[string]UpdateFn)
	}
	f.updateFunctions[reflect.TypeOf(container.ClientObject())][container.ObjectKey().String()] = fn
	return f
}

// Update calls the custom Update function if it was registered for the given object.
// Otherwise, it calls the original client.Client.Update function.
func (f *FakeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if f.updateFunctions != nil {
		if functions, ok := f.updateFunctions[reflect.TypeOf(obj)]; ok {
			if fn, ok := functions[client.ObjectKeyFromObject(obj).String()]; ok {
				return fn(ctx, obj, opts...)
			}
		}
	}
	return f.Client.Update(ctx, obj, opts...)
}

// Status returns a fake client.StatusWriter implementation.
func (f *FakeClient) Status() client.StatusWriter {
	return &FakeStatusWriter{
		StatusWriter: f.Client.Status(),
		parent:       f,
	}
}

// WithStatusUpdateFunction registers a custom Update function for the given object status.
func (f *FakeClient) WithStatusUpdateFunction(container GenericObjectContainer, fn UpdateSubResourceFn) *FakeClient {
	if f.statusUpdateFunctions == nil {
		f.statusUpdateFunctions = make(map[reflect.Type]map[string]UpdateSubResourceFn)
	}
	if f.statusUpdateFunctions[reflect.TypeOf(container.ClientObject())] == nil {
		f.statusUpdateFunctions[reflect.TypeOf(container.ClientObject())] = make(map[string]UpdateSubResourceFn)
	}
	f.statusUpdateFunctions[reflect.TypeOf(container.ClientObject())][container.ObjectKey().String()] = fn
	return f
}

// Update calls the custom Update function if it was registered for the given object status.
// Otherwise, it calls the original client.Client.Status().Update function.
func (s *FakeStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if s.parent.statusUpdateFunctions != nil {
		if functions, ok := s.parent.statusUpdateFunctions[reflect.TypeOf(obj)]; ok {
			if fn, ok := functions[client.ObjectKeyFromObject(obj).String()]; ok {
				return fn(ctx, obj, opts...)
			}
		}
	}
	return s.StatusWriter.Update(ctx, obj, opts...)
}

// WithDeleteFunction registers a custom Delete function for the given object.
func (f *FakeClient) WithDeleteFunction(container GenericObjectContainer, fn DeleteFn) *FakeClient {
	if f.deleteFn == nil {
		f.deleteFn = make(map[reflect.Type]map[string]DeleteFn)
	}
	if f.deleteFn[reflect.TypeOf(container.ClientObject())] == nil {
		f.deleteFn[reflect.TypeOf(container.ClientObject())] = make(map[string]DeleteFn)
	}
	f.deleteFn[reflect.TypeOf(container.ClientObject())][container.ObjectKey().String()] = fn
	return f
}

// Delete calls the custom Delete function if it was registered for the given object.
// Otherwise, it calls the original client.Client.Delete function.
func (f *FakeClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if f.deleteFn != nil {
		if functions, ok := f.deleteFn[reflect.TypeOf(obj)]; ok {
			if fn, ok := functions[client.ObjectKeyFromObject(obj).String()]; ok {
				return fn(ctx, obj, opts...)
			}
		}
	}
	return f.Client.Delete(ctx, obj, opts...)
}
