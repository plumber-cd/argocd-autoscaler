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
	// UID unique identifier of this shard.
	// There is multiple seemingly duplicative fields here, but this is the only one that is unique.
	// For example, when the shard is represented as a secret with type=cluster label,
	// the UID of the secret is a UID of the shard.
	// Meaning that it would change if the secret is re-created.
	// That's what is meant by "unique" in this context.
	// When the discovery was external - this may be arbitrary string unique to that shard.
	// +kubebuilder:validation:Required
	UID types.UID `json:"uid,omitempty"`
	// ID of this shard. It may or may not be unique, depending on the discoverer.
	// For a secret with type=cluster label, this would be the name of the secret.
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`
	// Namespace of this shard.
	// For a secret with type=cluster label, this would be the namespace of the secret.
	// If shard is managed externally - it is expected to be set to some value.
	// Same as the Application Controller is in - would be a logical choice.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace,omitempty"`
	// Name of this shard.
	// This must be the same as the name of this destination cluster as seen by Application Controller.
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// Server of this shard.
	// This must be the same as the server URL of this destination cluster as seen by Application Controller.
	// +kubebuilder:validation:Required
	Server string `json:"server,omitempty"`
}
