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

// MetricValue is a resulting value for the metric.
type MetricValue struct {
	// ID of this metric.
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`
	// Shard is this shard for this metric value.
	// +kubebuilder:validation:Required
	Shard Shard `json:"shard,omitempty"`
	// Query is the query that was used to fetch this value.
	// It will be different for individual implementations.
	Query string `json:"query,omitempty"`
	// Value is the value of the metric.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Value resource.Quantity `json:"value,omitempty"`
	// DisplayValue is the string representation of the without precision guarantee.
	// This is meaningless and exists purely for convenience of someone who is looking at the kubectl get output.
	// +kubebuilder:validation:Required
	DisplayValue string `json:"displayValue,omitempty"`
}
