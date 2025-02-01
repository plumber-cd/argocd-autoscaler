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
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretTypeClusterShardManagerSpec defines the ArgoCD clusters discovery algorithm.
// It finds secrets with standard `argocd.argoproj.io/secret-type=cluster` label.
type SecretTypeClusterShardManagerSpec struct {
	common.ShardManagerSpec `json:",inline"`

	// LabelSelector use to override default ArgoCD `argocd.argoproj.io/secret-type=cluster` label selector.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// SecretTypeClusterShardManagerStatus defines the observed state of SecretTypeClusterShardManager.
type SecretTypeClusterShardManagerStatus struct {
	common.ShardManagerStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SecretTypeClusterShardManager is the Schema for the secrettypeclustershardmanagers API.
type SecretTypeClusterShardManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretTypeClusterShardManagerSpec   `json:"spec,omitempty"`
	Status SecretTypeClusterShardManagerStatus `json:"status,omitempty"`
}

func (s *SecretTypeClusterShardManager) GetShardManagerSpec() *common.ShardManagerSpec {
	return &s.Spec.ShardManagerSpec
}

func (s *SecretTypeClusterShardManager) GetShardManagerStatus() *common.ShardManagerStatus {
	return &s.Status.ShardManagerStatus
}

func (s *SecretTypeClusterShardManager) GetShardProviderStatus() *common.ShardsProviderStatus {
	return &s.Status.ShardsProviderStatus
}

func (s *SecretTypeClusterShardManager) GetClientObject() client.Object {
	return s
}

// +kubebuilder:object:root=true

// SecretTypeClusterShardManagerList contains a list of SecretTypeClusterShardManager.
type SecretTypeClusterShardManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretTypeClusterShardManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretTypeClusterShardManager{}, &SecretTypeClusterShardManagerList{})
}
