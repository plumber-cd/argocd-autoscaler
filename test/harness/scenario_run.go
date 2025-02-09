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

// ScenarioRun is a state of the scenario during the execution.
// Various functions use pointer to the run to communicate with each other.
type ScenarioRun[K client.Object] struct {
	// Context is a context for the scenario.
	Context context.Context

	// Client is a clean client that can be used to perform operations against k8s.
	// This must be a clean client with no mocks.
	Client *Client

	// Namespace is a namespace object that is used to create resources in the cluster.
	Namespace *ObjectContainer[*corev1.Namespace]

	// FakeClient is a fake client that can be used to mock certain calls.
	FakeClient *FakeClient

	// SeedObject is a resource that was seeded by the seed function.
	SeedObject K

	// Container is a container that holds the main resource created during hydration.
	Container *ObjectContainer[K]

	// ReconcileResult is a result of the reconciliation.
	ReconcileResult reconcile.Result

	// ReconcileError is an error that occurred during the reconciliation.
	ReconcileError error
}
