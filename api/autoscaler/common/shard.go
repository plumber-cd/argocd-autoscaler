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
	"k8s.io/apimachinery/pkg/types"
)

// Shard is a shard of something discovered by a discoverer.
// It is suitable to be used by Pollers.
// Attributes may be used in Go Templates for the poller if it supports that.
type Shard struct {
	// UID unique identifier of this shard so it can be uniquely identified in the list of shards.
	// The reason this is not baked into SourceRef is because actual resource may or may not exists.
	// When the discovery was external - this may be arbitrary string unique to that shard.
	// +kubebuilder:validation:Required
	UID types.UID `json:"uid,omitempty"`
	// ID of this shard. It may or may not be unique, depending on the discoverer.
	// It is expected to be used to populate Go Templates params for the poller (if the poller supports that).
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`
	// Data is a map that will be used to populate Go Templates params for the poller (if the poller supports that).
	// +kubebuilder:validation:Required
	Data map[string]string `json:"data,omitempty"`
}
