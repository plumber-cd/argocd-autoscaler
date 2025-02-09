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

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// GenericScenarioRun is used to operate without the knowledge of the generic type.
type GenericScenarioRun interface {
	Context() context.Context
	Client() client.Client
	Namespace() *ObjectContainer[*corev1.Namespace]
	FakeClient() *FakeClient
	GenericContainer() *GenericObjectContainer
	ReconcileResult() reconcile.Result
	ReconcileError() error
}

// ScenarioRun is a state of the scenario during the execution.
// Various functions use pointer to the run to communicate with each other.
type ScenarioRun[K client.Object] struct {
	// Context is a context for the scenario.
	ctx context.Context

	// Client is a clean client that can be used to perform operations against k8s.
	// This must be a clean client with no mocks.
	client client.Client

	// Namespace is a namespace object that is used to create resources in the cluster.
	namespace *ObjectContainer[*corev1.Namespace]

	// FakeClient is a fake client that can be used to mock certain calls.
	fakeClient *FakeClient

	// Container is a container that holds the main resource created during hydration.
	container *ObjectContainer[K]

	// ReconcileResult is a result of the reconciliation.
	reconcileResult reconcile.Result

	// ReconcileError is an error that occurred during the reconciliation.
	reconcileError error
}

func (r *ScenarioRun[K]) Context() context.Context {
	return r.ctx
}

func (r *ScenarioRun[K]) Client() client.Client {
	return r.client
}

func (r *ScenarioRun[K]) Namespace() *ObjectContainer[*corev1.Namespace] {
	return r.namespace
}

func (r *ScenarioRun[K]) FakeClient() *FakeClient {
	return r.fakeClient
}

func (r *ScenarioRun[K]) SetFakeClient(fakeClient *FakeClient) {
	r.fakeClient = fakeClient
}

func (r *ScenarioRun[K]) Container() *ObjectContainer[K] {
	return r.container
}

func (r *ScenarioRun[K]) SetContainer(container *ObjectContainer[K]) {
	r.container = container
}

func (r *ScenarioRun[K]) GenericContainer() *GenericObjectContainer {
	return r.container.run.GenericContainer()
}

func (r *ScenarioRun[K]) ReconcileResult() reconcile.Result {
	return r.reconcileResult
}

func (r *ScenarioRun[K]) ReconcileError() error {
	return r.reconcileError
}
