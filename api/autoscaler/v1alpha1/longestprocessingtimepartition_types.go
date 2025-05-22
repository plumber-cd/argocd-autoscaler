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

// LongestProcessingTimePartitionSpec defines the desired state of LongestProcessingTimePartition.
type LongestProcessingTimePartitionSpec struct {
	common.PartitionerSpec `json:",inline"`

	// Optional: set the top ceiling of shards per replica.
	MaxShardsPerReplica *int32 `json:"maxShardsPerReplica,omitempty"`
}

// LongestProcessingTimePartitionStatus defines the observed state of LongestProcessingTimePartition.
type LongestProcessingTimePartitionStatus struct {
	common.PartitionProviderStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Current status based on Ready condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// LongestProcessingTimePartition is the Schema for the longestprocessingtimepartitions API.
type LongestProcessingTimePartition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LongestProcessingTimePartitionSpec   `json:"spec,omitempty"`
	Status LongestProcessingTimePartitionStatus `json:"status,omitempty"`
}

// GetConditions returns .Status.Conditions
func (p *LongestProcessingTimePartition) GetPartitionerSpec() *common.PartitionerSpec {
	return &p.Spec.PartitionerSpec
}

// GetReplicas returns .Status.Value
func (p *LongestProcessingTimePartition) GetPartitionProviderStatus() *common.PartitionProviderStatus {
	return &p.Status.PartitionProviderStatus
}

// +kubebuilder:object:root=true

// LongestProcessingTimePartitionList contains a list of LongestProcessingTimePartition.
type LongestProcessingTimePartitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LongestProcessingTimePartition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LongestProcessingTimePartition{}, &LongestProcessingTimePartitionList{})
}
