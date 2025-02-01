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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ShardsProviderStatus struct {
	// Conditions
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Shards shards that are discovered.
	Shards []Shard `json:"shards,omitempty"`
}

// +k8s:deepcopy-gen=false
type ShardsProvider interface {
	GetShardProviderStatus() *ShardsProviderStatus
}
