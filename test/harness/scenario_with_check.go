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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScenarioCheckFn is a signature of a function that checks the results of this ScenarioRun.
type ScenarioCheckFn[K client.Object] func(*ScenarioRun[K])

// ScenarioCheck is hydrated and has a fake client and check functions describe how to validate result upon execution.
type ScenarioCheck[K client.Object] struct {
	*Scenario[K]
	checkName string
	checkFn   ScenarioCheckFn[K]
}

// Commit registers a scenario in the collector by calling a callback.
// Returns a ScenarioWithFakeClient ready for another set of checks with additional mocks.
func (s *ScenarioCheck[K]) Commit(callback ScenarioRegisterCallback[K]) *Scenario[K] {
	callback(s)
	return s.Clone()
}
