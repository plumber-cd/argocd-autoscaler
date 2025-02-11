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
	"k8s.io/apimachinery/pkg/api/resource"
)

// LoadIndex is a representation of a shard with calculated load index to it.
type LoadIndex struct {
	// Shard is the shard that this load index is calculated for.
	// +kubebuilder:validation:Required
	Shard Shard `json:"shard,omitempty"`
	// Value is a value of this load index.
	// +kubebuilder:validation:Required
	Value resource.Quantity `json:"value,omitempty"`
	// DisplayValue is the string representation of the without precision guarantee.
	// This is meaningless and exists purely for convenience of someone who is looking at the kubectl get output.
	// +kubebuilder:validation:Required
	DisplayValue string `json:"displayValue,omitempty"`
}

// LoadIndexesDesc implements sort.Interface for []LoadIndex based on the Value field in descending order.
type LoadIndexesDesc []LoadIndex

func (s LoadIndexesDesc) Len() int {
	return len(s)
}

func (s LoadIndexesDesc) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s LoadIndexesDesc) Less(i, j int) bool {
	return s[i].Value.AsApproximateFloat64() > s[j].Value.AsApproximateFloat64()
}
