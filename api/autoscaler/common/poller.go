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

package common

import (
	corev1 "k8s.io/api/core/v1"
)

type PollerSpec struct {
	// ShardManagerRef is a reference to the shard manager.
	// +kubebuilder:validation:Required
	ShardManagerRef *corev1.TypedLocalObjectReference `json:"shardManagerRef,omitempty"`
}

// +k8s:deepcopy-gen=false
type Poller interface {
	GetPollerSpec() *PollerSpec
}
