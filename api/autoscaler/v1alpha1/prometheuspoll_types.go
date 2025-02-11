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

// PrometheusMetric is a configuration for polling from Prometheus
type PrometheusMetric struct {

	// ID of this metric.
	// +kubebuilder:validation:Required
	ID string `json:"id,omitempty"`

	// Query is the query used to fetch the metric. It is a Go Template with Sprig functions binding.
	// A `.namespace`, `.name` and `.server` are available for the template.
	// +kubebuilder:validation:Required
	Query string `json:"query,omitempty"`

	// NoData if set will be used as the value when no data is available in Prometheus.
	// If not set - missing data will result in error.
	// There are metrics like argocd_app_k8s_request_total that might have a bug and not being reported for prolonged periods of time.
	NoData *resource.Quantity `json:"noData,omitempty"`
}

// PrometheusPollSpec defines the configuration for this poller.
type PrometheusPollSpec struct {
	common.PollerSpec `json:",inline"`

	// Period is the period of polling.
	// +kubebuilder:validation:Required
	Period metav1.Duration `json:"period,omitempty"`

	// Address is the address of the Prometheus server.
	// +kubebuilder:validation:Required
	Address string `json:"address,omitempty"`

	// Metrics is the list of metrics to poll from Prometheus.
	// +kubebuilder:validation:Required
	Metrics []PrometheusMetric `json:"metrics,omitempty"`
}

// PrometheusPollStatus defines the observed state of PrometheusPoll.
type PrometheusPollStatus struct {
	common.MetricValuesProviderStatus `json:",inline"`

	// LastPollingTime is the last time the polling was performed for this configuration.
	LastPollingTime *metav1.Time `json:"lastPollingTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Last Polled",type="date",JSONPath=".status.lastPollingTime",description="Since last poll"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status",description="Current status based on Ready condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time since creation"

// PrometheusPoll is the Schema for the prometheuspolls API.
type PrometheusPoll struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   PrometheusPollSpec   `json:"spec,omitempty"`
	Status PrometheusPollStatus `json:"status,omitempty"`
}

// GetConditions returns .Status.Conditions
func (p *PrometheusPoll) GetPollerSpec() *common.PollerSpec {
	return &p.Spec.PollerSpec
}

// GetValues returns .Status.Values
func (p *PrometheusPoll) GetMetricValuesProviderStatus() *common.MetricValuesProviderStatus {
	return &p.Status.MetricValuesProviderStatus
}

// +kubebuilder:object:root=true

// PrometheusPollList contains a list of PrometheusPoll.
type PrometheusPollList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrometheusPoll `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PrometheusPoll{}, &PrometheusPollList{})
}
