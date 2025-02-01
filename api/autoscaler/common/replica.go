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
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Replica is a representation of the replica for sharding
type Replica struct {
	// ID of the replica, starting from 0 and onward.
	ID string `json:"id,omitempty"`
	// LoadIndexes shards assigned to this replica wrapped into their load index.
	// +kubebuilder:validation:Required
	LoadIndexes []LoadIndex `json:"loadIndexes,omitempty"`
	// TotalLoad is the sum of all load indexes assigned to this replica.
	// +kubebuilder:validation:Required
	TotalLoad resource.Quantity `json:"totalLoad,omitempty"`
	// DisplayValue is the string representation of the total load without precision guarantee.
	// This is meaningless and exists purely for convenience of someone who is looking at the kubectl get output.
	// +kubebuilder:validation:Required
	TotalLoadDisplayValue string `json:"totalLoadDisplayValue,omitempty"`
}

type ReplicaList []Replica

// SerializeToString serializes list of replicas into predictable deterministic string.
// The format is: shardUID=replicaID,shardUID=replicaID,...
// The order of the shards is sorted.
// This is used to compare list of replicas when we need to know if two lists are equal.
// Regardless of the order of the replicas, the serialized string will be the same.
func (list ReplicaList) SerializeToString() string {

	m := map[string]string{}
	for _, replica := range list {
		for _, loadIndex := range replica.LoadIndexes {
			m[string(loadIndex.Shard.UID)] = replica.ID
		}
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var serializedItems []string
	for _, k := range keys {
		serializedItems = append(serializedItems, fmt.Sprintf("%s=%v", k, m[k]))
	}

	return strings.Join(serializedItems, ",")
}
