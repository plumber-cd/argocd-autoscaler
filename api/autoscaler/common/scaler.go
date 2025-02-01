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

import corev1 "k8s.io/api/core/v1"

type ScalerSpec struct {
	// PartitionProviderRef is the reference to the partition provider.
	// This is the source of partition layout to apply
	// +kubebuilder:validation:Required
	PartitionProviderRef *corev1.TypedLocalObjectReference `json:"partitionProviderRef,omitempty"`
	// ShardManagerRef is the reference to the shard manager.
	// This is a target to update the partition layout on.
	// +kubebuilder:validation:Required
	ShardManagerRef *corev1.TypedLocalObjectReference `json:"shardManagerRef,omitempty"`
	// ReplicaSetControllerRef is the reference to the replica set.
	// This is the controller of Application Controller RS to apply scaling.
	// +kubebuilder:validation:Required
	ReplicaSetControllerRef *corev1.TypedLocalObjectReference `json:"replicaSetControllerRef,omitempty"`
}

// +k8s:deepcopy-gen=false
type Scaler interface {
	GetScalerSpec() *ScalerSpec
}
