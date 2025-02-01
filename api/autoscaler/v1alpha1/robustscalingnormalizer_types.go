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

// RobustScalingNormalizerSpec defines the desired state of RobustScalingNormalizer.
type RobustScalingNormalizerSpec struct {
	common.NormalizerSpec `json:",inline"`
}

// RobustScalingNormalizerStatus defines the observed state of RobustScalingNormalizer.
type RobustScalingNormalizerStatus struct {
	common.MetricValuesProviderStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Available",type="string",JSONPath=".status.conditions[?(@.type=='Available')].status",description="Current status based on Available condition"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Current status based on Ready condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// RobustScalingNormalizer is the Schema for the robustscalingnormalizers API.
type RobustScalingNormalizer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   RobustScalingNormalizerSpec   `json:"spec,omitempty"`
	Status RobustScalingNormalizerStatus `json:"status,omitempty"`
}

// GetConditions returns .Status.Conditions
func (p *RobustScalingNormalizer) GetNormalizerSpec() *common.NormalizerSpec {
	return &p.Spec.NormalizerSpec
}

// GetValues returns .Status.Values
func (p *RobustScalingNormalizer) GetMetricValuesProviderStatus() *common.MetricValuesProviderStatus {
	return &p.Status.MetricValuesProviderStatus
}

// +kubebuilder:object:root=true

// RobustScalingNormalizerList contains a list of RobustScalingNormalizer.
type RobustScalingNormalizerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RobustScalingNormalizer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RobustScalingNormalizer{}, &RobustScalingNormalizerList{})
}
