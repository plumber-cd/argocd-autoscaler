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

package utils

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// Parse Prometheus metrics into a structured map
func ParsePrometheusMetrics(metricsText string) (map[string]*dto.MetricFamily, error) {
	var parser expfmt.TextParser
	return parser.TextToMetricFamilies(bytes.NewReader([]byte(metricsText)))
}

// Compare expected vs actual Prometheus metrics, returning a **clean debug output**
func ComparePrometheusMetrics(actualMetrics, expectedMetrics map[string]*dto.MetricFamily) (bool, string) {
	var messages []string
	match := true

	// Check for missing and mismatched metrics
	for name, expectedFamily := range expectedMetrics {
		actualFamily, found := actualMetrics[name]
		if !found {
			messages = append(messages, fmt.Sprintf("❌ Missing Prometheus metric: %s", name))
			match = false
			continue
		}

		for _, expectedMetric := range expectedFamily.Metric {
			matchFound := false

			for _, actualMetric := range actualFamily.Metric {
				if PrometheusMetricMatches(expectedMetric, actualMetric) {
					matchFound = true
					break
				}
			}

			if !matchFound {
				messages = append(messages, fmt.Sprintf("⚠️ Mismatch in Prometheus metric: %s", name))
				messages = append(messages, FormatPrometheusMetricComparison(expectedMetric, actualFamily.Metric))
				match = false
			}
		}
	}

	if match {
		return true, ""
	}

	return false, "\n" + FormatPrometheusFailureMessage(messages)
}

// Check if two Prometheus metrics match (ignoring label order)
func PrometheusMetricMatches(expected, actual *dto.Metric) bool {
	expectedLabels := PrometheusLabelsToMap(expected.Label)
	actualLabels := PrometheusLabelsToMap(actual.Label)

	for key, expectedValue := range expectedLabels {
		if expectedValue == "*" {
			continue // Wildcard: Accept any value
		}
		if actualLabels[key] != expectedValue {
			return false
		}
	}

	if expected.Counter != nil && actual.Counter != nil {
		if *expected.Counter.Value != *actual.Counter.Value {
			return false
		}
	}

	return true
}

// Convert Prometheus LabelPair list to a map for easier comparison
func PrometheusLabelsToMap(pairs []*dto.LabelPair) map[string]string {
	m := make(map[string]string)
	for _, p := range pairs {
		m[p.GetName()] = p.GetValue()
	}
	return m
}

// Format Prometheus metric comparison for debugging
func FormatPrometheusMetricComparison(expected *dto.Metric, actualMetrics []*dto.Metric) string {
	expectedLabels := PrometheusLabelsToMap(expected.Label)
	expectedValue := expected.Counter.GetValue()

	var output string
	output += fmt.Sprintf("   🔹 Expected: %s %v\n", FormatPrometheusLabels(expectedLabels), expectedValue)

	for _, actual := range actualMetrics {
		actualLabels := PrometheusLabelsToMap(actual.Label)
		actualValue := actual.Counter.GetValue()

		labelMismatch := expectedLabels
		valueMismatch := expectedValue != actualValue

		if labelMismatch != nil || valueMismatch {
			output += fmt.Sprintf("   ❌ Actual:   %s %v\n", FormatPrometheusLabels(actualLabels), actualValue)
			if valueMismatch {
				output += fmt.Sprintf("      ↳ ❗ Value mismatch: expected %.2f, got %.2f\n", expectedValue, actualValue)
			}
			if labelMismatch != nil {
				// nolint:lll
				output += fmt.Sprintf("      ↳ 🏷  Label mismatch: expected %s, got %s\n", FormatPrometheusLabels(expectedLabels), FormatPrometheusLabels(actualLabels))
			}
		}
	}

	return output
}

// Format Prometheus labels into a readable string
func FormatPrometheusLabels(labels map[string]string) string {
	pairs := make([]string, len(labels))
	for k, v := range labels {
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, k, v))
	}
	return "{" + fmt.Sprintf("%s", pairs) + "}"
}

// Format a human-readable Prometheus failure message
func FormatPrometheusFailureMessage(messages []string) string {
	return "❌ Prometheus Metric Assertion Failed:\n" + fmt.Sprintf("%s", messages)
}

// **Final Gomega Matcher for Prometheus Metrics**
func MatchPrometheusMetrics(expectedMetrics map[string]*dto.MetricFamily) gomega.OmegaMatcher {
	return gomega.WithTransform(func(actual string) (bool, error) {
		actualMetrics, err := ParsePrometheusMetrics(actual)
		if err != nil {
			return false, fmt.Errorf("❌ Failed to parse actual Prometheus metrics: %w", err)
		}
		match, failureMessage := ComparePrometheusMetrics(actualMetrics, expectedMetrics)
		if !match {
			return false, errors.New(failureMessage)
		}
		return true, nil
	}, gomega.BeTrue())
}

// CreatePrometheusMetric constructs a Prometheus metric with labels and a value
func CreatePrometheusMetric(name string, labels map[string]string, value float64) *dto.MetricFamily {
	mf := &dto.MetricFamily{
		Name: &name,
		Type: dto.MetricType_COUNTER.Enum(), // Change type if needed
		Metric: []*dto.Metric{
			{
				Label:   convertPrometheusLabels(labels),
				Counter: &dto.Counter{Value: &value},
			},
		},
	}
	return mf
}

// Convert label map to Prometheus LabelPair format
func convertPrometheusLabels(labels map[string]string) []*dto.LabelPair {
	pairs := make([]*dto.LabelPair, len(labels))
	for k, v := range labels {
		labelName, labelValue := k, v
		pairs = append(pairs, &dto.LabelPair{
			Name:  &labelName,
			Value: &labelValue,
		})
	}
	return pairs
}

// ExpectedPrometheusMetricsMap converts multiple metrics into a testable expected map
func ExpectedPrometheusMetricsMap(metrics ...*dto.MetricFamily) map[string]*dto.MetricFamily {
	expected := make(map[string]*dto.MetricFamily)
	for _, mf := range metrics {
		expected[*mf.Name] = mf
	}
	return expected
}
