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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MostWantedTwoPhaseHysteresisEvaluationSpec defines the desired state of MostWantedTwoPhaseHysteresisEvaluation.
type MostWantedTwoPhaseHysteresisEvaluationSpec struct {
	// PartitionProviderRef is the reference to the resource that provides partitioning.
	// +kubebuilder:validation:Required
	PartitionProviderRef corev1.TypedLocalObjectReference `json:"paritionProviderRef,omitempty"`
	// PollingPeriod is the period for polling the partitioning.
	// +kubebuilder:validation:Required
	PollingPeriod metav1.Duration `json:"pollingPeriod,omitempty"`
	// StabilizationPeriod is the amount of time to wait before evaluating historical records.
	// +kubebuilder:validation:Required
	StabilizationPeriod metav1.Duration `json:"stabilizationPeriod,omitempty"`
	// MinimumSampleSize is the minimum number of samples to consider before evaluating.
	// +kubebuilder:validation:Required
	MinimumSampleSize int32 `json:"minimumSampleSize,omitempty"`
}

type MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord struct {
	// Timestamp is the time at which the record was created.
	// +kubebuilder:validation:Required
	Timestamp metav1.Time `json:"timestamp,omitempty"`
	// Replicas is the partition as it was seen at this moment in time.
	// +kubebuilder:validation:Required
	Replicas []Replica `json:"replicas,omitempty"`
}

// MostWantedTwoPhaseHysteresisEvaluationStatus defines the observed state of MostWantedTwoPhaseHysteresisEvaluation.
type MostWantedTwoPhaseHysteresisEvaluationStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// History is historical observations of the partitioning changes over time.
	History []MostWantedTwoPhaseHysteresisEvaluationStatusHistoricalRecord `json:"history,omitempty"`
	// Replicas is the current partitioning choice.
	Replicas []Replica `json:"replicas,omitempty"`
	// LastEvaluationTimestamp is the time at which the last evaluation was performed.
	LastEvaluationTimestamp *metav1.Time `json:"lastEvaluationTimestamp,omitempty"`
	// Projection shows what the partitioning choice would have been if evaluation was performed during last poll.
	Projection []Replica `json:"projection,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Last Evaluated",type="date",JSONPath=".status.lastEvaluationTimestamp",description="Since last poll"
// +kubebuilder:printcolumn:name="Available",type="string",JSONPath=".status.conditions[?(@.type=='Available')].status",description="Current status based on Available condition"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Current status based on Ready condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// MostWantedTwoPhaseHysteresisEvaluation is the Schema for the mostwantedtwophasehysteresisevaluations API.
type MostWantedTwoPhaseHysteresisEvaluation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MostWantedTwoPhaseHysteresisEvaluationSpec   `json:"spec,omitempty"`
	Status MostWantedTwoPhaseHysteresisEvaluationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MostWantedTwoPhaseHysteresisEvaluationList contains a list of MostWantedTwoPhaseHysteresisEvaluation.
type MostWantedTwoPhaseHysteresisEvaluationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MostWantedTwoPhaseHysteresisEvaluation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MostWantedTwoPhaseHysteresisEvaluation{}, &MostWantedTwoPhaseHysteresisEvaluationList{})
}
