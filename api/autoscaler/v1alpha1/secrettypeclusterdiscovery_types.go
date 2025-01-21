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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretTypeClusterDiscoverySpec defines the ArgoCD clusters discovery algorithm.
// It finds secrets with standard `argocd.argoproj.io/secret-type=cluster` label.
type SecretTypeClusterDiscoverySpec struct {
	// LabelSelector is a standard Kubernetes label selector that would override default ArgoCD standard label.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// SecretTypeClusterDiscoveryStatus defines the observed state of SecretTypeClusterDiscovery.
type SecretTypeClusterDiscoveryStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Shards shards that are discovered.
	Shards []Shard `json:"shards,omitempty"`
}

// GetConditions returns .Status.Conditions
func (d *SecretTypeClusterDiscovery) GetConditions() []metav1.Condition {
	return d.Status.Conditions
}

// GetShards returns .Status.Shards
func (d *SecretTypeClusterDiscovery) GetShards() []Shard {
	return d.Status.Shards
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Available",type="string",JSONPath=".status.conditions[?(@.type=='Available')].status",description="Current status based on Available condition"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Current status based on Ready condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// SecretTypeClusterDiscovery is the Schema for the secrettypeclusterdiscoveries API.
type SecretTypeClusterDiscovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   SecretTypeClusterDiscoverySpec   `json:"spec,omitempty"`
	Status SecretTypeClusterDiscoveryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecretTypeClusterDiscoveryList contains a list of SecretTypeClusterDiscovery.
type SecretTypeClusterDiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretTypeClusterDiscovery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretTypeClusterDiscovery{}, &SecretTypeClusterDiscoveryList{})
}
