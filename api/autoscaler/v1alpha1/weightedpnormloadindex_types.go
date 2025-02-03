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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WeightedPNormLoadIndexWeight is the weight of the metric.
type WeightedPNormLoadIndexWeight struct {
	// ID of the metric.
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`

	// Weight of this metric.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Weight resource.Quantity `json:"weight,omitempty"`

	// Negative by default is true,
	// meaning that if there are negative input values (based on previous normalization),
	// they will be reducing the load index accordingly to their weight.
	// For example - Robust Scaling normalization results in 0 representing a median.
	// Depending on the original source of normalized values, this may or may not be desireable.
	// Set to false to assume replace all negative values with 0.
	// That will prevent load index reductions and it will only go up from positive values.
	// For Robust Scaling normalization, for example,
	// that would mean that only values bigger than the median would have any effect on the load index.
	Negative *bool `json:"negative,omitempty"`
}

// WeightedPNormLoadIndexSpec defines the desired state of WeightedPNormLoadIndex.
type WeightedPNormLoadIndexSpec struct {
	common.LoadIndexerSpec `json:",inline"`

	// P is the vaue of p in the p-norm
	// I.e. 1 for linear, 2 for euclidean, 3 for cubic etc.
	// +kubebuilder:validation:Required
	P int32 `json:"p,omitempty"`

	// Weigths is the list of metrics and their weights to use in this load index.
	// +kubebuilder:validation:Required
	Weights []WeightedPNormLoadIndexWeight `json:"weights,omitempty"`
}

// WeightedPNormLoadIndexStatus defines the observed state of WeightedPNormLoadIndex.
type WeightedPNormLoadIndexStatus struct {
	common.LoadIndexProviderStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=weightedpnormloadindexes
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Current status based on Ready condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// WeightedPNormLoadIndex is the Schema for the weightedpnormloadindexes API.
type WeightedPNormLoadIndex struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WeightedPNormLoadIndexSpec   `json:"spec,omitempty"`
	Status WeightedPNormLoadIndexStatus `json:"status,omitempty"`
}

// GetConditions returns .Status.Conditions
func (li *WeightedPNormLoadIndex) GetLoadIndexerSpec() *common.LoadIndexerSpec {
	return &li.Spec.LoadIndexerSpec
}

// GetValues returns .Status.Values
func (li *WeightedPNormLoadIndex) GetLoadIndexProviderStatus() *common.LoadIndexProviderStatus {
	return &li.Status.LoadIndexProviderStatus
}

// +kubebuilder:object:root=true

// WeightedPNormLoadIndexList contains a list of WeightedPNormLoadIndex.
type WeightedPNormLoadIndexList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WeightedPNormLoadIndex `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WeightedPNormLoadIndex{}, &WeightedPNormLoadIndexList{})
}
