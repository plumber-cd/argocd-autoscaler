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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrometheusMetric is a configuration for polling from Prometheus
type PrometheusMetric struct {
	// ID of this metric.
	ID string `json:"id,omitempty"`
	// Query is the query used to fetch the metric. It is a Go Template with Sprig functions binding.
	// A `.data` of the cluster secret is passed inside as parameters.
	Query string `json:"query,omitempty"`
	// Value is the value of the metric.
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Value resource.Quantity `json:"value,omitempty"`
}

// PrometheusSource is a configuration for polling from Prometheus
type PrometheusSource struct {
	// Address is the address of the Prometheus server.
	Address string `json:"address,omitempty"`
	// Metrics is the list of metrics to poll from Prometheus.
	Metrics []PrometheusMetric `json:"metrics,omitempty"`
}

// PollSpec defines the configuration for this poller.
type PollSpec struct {
	// Period is the period of polling.
	Period metav1.Duration `json:"period,omitempty"`
	// PrometheusSource is the configuration for polling from Prometheus.
	PrometheusSource PrometheusSource `json:"prometheusSource,omitempty"`
}

// MetricValue is a resulting value from Prometheus
type MetricValue struct {
	// Poller is the identification of the poller that was used to fetch this metric.
	Poller string `json:"poller,omitempty"`
	// ID of this metric.
	ID string `json:"id,omitempty"`
	// Query is the query that was used to fetch this value.
	// It will be different for individual implementations.
	Query string `json:"query,omitempty"`
	// Value is the value of the metric.
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Value resource.Quantity `json:"value,omitempty"`
}

// PollStatus defines the observed state of Poll.
type PollStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Values of the metrics polled.
	// +kubebuilder:validation:Type:=number
	// +kubebuilder:validation:Format:=float
	Values []resource.Quantity `json:"values,omitempty"`
	// LastCalculatedTime is the last time the polling was performed for this configuration.
	LastPollingTime *metav1.Time `json:"lastPollingTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Poll is the Schema for the polls API.
type Poll struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PollSpec   `json:"spec,omitempty"`
	Status PollStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PollList contains a list of Poll.
type PollList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Poll `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Poll{}, &PollList{})
}
