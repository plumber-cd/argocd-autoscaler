/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ReplicaSetScalerSpecModeDefault struct {
	RolloutRestart *bool `json:"rolloutRestart,omitempty"`
}

type ReplicaSetScalerSpecModeX0Y struct{}

type ReplicaSetScalerSpecModes struct {
	// Default is the default mode of the ReplicaSetScaler.
	// It first applies change to the Shard Manager.
	// Then, it applies change to the ReplicaSet Controller.
	// Lastly, if `.spec.mode.default.rolloutRestart` is true, it performs equivalent of `rollout restart` command.
	Default *ReplicaSetScalerSpecModeDefault `json:"default,omitempty"`

	// X0Y is the mode of the ReplicaSetScaler.
	// First, it scales down the ReplicaSet to 0 replicas.
	// Then, it applies change to the Shard Manager.
	// It waits for Shard Manager to apply configuration.
	// Lastly, it scales up the ReplicaSet to the desired replicas.
	X0Y *ReplicaSetScalerSpecModeX0Y `json:"x0y,omitempty"`
}

// ReplicaSetScalerSpec defines the desired state of ReplicaSetScaler.
type ReplicaSetScalerSpec struct {
	common.ScalerSpec `json:",inline"`

	// Mode is the mode of the ReplicaSetScaler
	Mode *ReplicaSetScalerSpecModes `json:"mode,omitempty"`
}

// ReplicaSetScalerStatus defines the observed state of ReplicaSetScaler.
type ReplicaSetScalerStatus struct {
	// Conditions
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Replicas is the list of replicas as it was applied to the Shard Manager.
	// Used for tracking whether the change was applied.
	Replicas common.ReplicaList `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Current status based on Ready condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// ReplicaSetScaler is the Schema for the replicasetscalers API.
type ReplicaSetScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicaSetScalerSpec   `json:"spec,omitempty"`
	Status ReplicaSetScalerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReplicaSetScalerList contains a list of ReplicaSetScaler.
type ReplicaSetScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicaSetScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReplicaSetScaler{}, &ReplicaSetScalerList{})
}
