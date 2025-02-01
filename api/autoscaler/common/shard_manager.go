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

import "sigs.k8s.io/controller-runtime/pkg/client"

type ShardManagerSpec struct {
	// Replicas is the list of replicas with shard assignments.
	Replicas ReplicaList `json:"replicas,omitempty"`
}

type ShardManagerStatus struct {
	ShardsProviderStatus `json:",inline"`
	// Replicas is the list of replicas that was last successfully applied by the manager.
	Replicas ReplicaList `json:"replicas,omitempty"`
}

// +k8s:deepcopy-gen=false
type ShardManager interface {
	GetShardManagerSpec() *ShardManagerSpec
	GetShardManagerStatus() *ShardManagerStatus
	GetShardManagerClientObject() client.Object
}
