// Copyright 2025
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package robustscaling

import (
	"context"
	"math"
	"sort"
	"strconv"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Normalizer interface {
	Normalize(context.Context, *float64, []common.MetricValue) ([]common.MetricValue, error)
}

type NormalizerImpl struct{}

func (r *NormalizerImpl) Normalize(ctx context.Context,
	e *float64,
	allMetrics []common.MetricValue) ([]common.MetricValue, error) {

	log := log.FromContext(ctx)

	// First - we need to separate metrics by ID
	metricsByID := map[string][]common.MetricValue{}
	for _, metric := range allMetrics {
		if _, ok := metricsByID[metric.ID]; !ok {
			metricsByID[metric.ID] = []common.MetricValue{}
		}
		metricsByID[metric.ID] = append(metricsByID[metric.ID], metric)
	}

	// Now, normalize them within their group
	normalizedMetrics := []common.MetricValue{}
	for _, metrics := range metricsByID {
		all := []float64{}
		for _, metric := range metrics {
			all = append(all, metric.Value.AsApproximateFloat64())
		}

		sorted := make([]float64, len(all))
		copy(sorted, all)
		sort.Float64s(sorted)

		// Compute median
		median := r.computeMedian(sorted)

		var iqr float64
		if len(sorted) > 1 {
			// Compute Q1 and Q3
			q1, q3 := r.computeQuartiles(sorted)

			// Compute IQR
			iqr = q3 - q1
		} else {
			// Assume IQR 0 which will signal to the scaling function to also use 0 as scaled value
			// See comments below on why that is
			iqr = 0
		}

		// Scale each value
		for _, metric := range metrics {
			var normalized float64
			// If IQR was 0, either all values were identical or not enough data to compute IQR.
			// Either value we use, as long as it is the same value for all shards, it would signify no deviation.
			// Typically, you would use 0 which is exact median, which then later can scale based on the wage.
			if iqr == 0 {
				normalized = 0
			} else {
				normalized = (metric.Value.AsApproximateFloat64() - median) / iqr
			}
			normalizedAsString := strconv.FormatFloat(normalized, 'f', -1, 64)
			normalizedAsResource, err := resource.ParseQuantity(normalizedAsString)
			if err != nil {
				log.Error(err, "Failed to parse normalized value as resource", "normalized", normalized)
				return nil, err
			}
			normalizedMetrics = append(normalizedMetrics, common.MetricValue{
				ID:           metric.ID,
				Shard:        metric.Shard,
				Query:        metric.Query,
				Value:        normalizedAsResource,
				DisplayValue: normalizedAsString,
			})
		}
	}

	if e != nil {
		minValue := math.MaxFloat64
		maxValue := math.MaxFloat64 * -1
		for _, metric := range normalizedMetrics {
			if metric.Value.AsApproximateFloat64() < minValue {
				minValue = metric.Value.AsApproximateFloat64()
			}
			if metric.Value.AsApproximateFloat64() > maxValue {
				maxValue = metric.Value.AsApproximateFloat64()
			}
		}
		_e := *e
		offset := -(minValue) + _e*(maxValue-minValue)
		for i, metric := range normalizedMetrics {
			scaled := metric.Value.AsApproximateFloat64() + offset
			metricValueAsString := strconv.FormatFloat(scaled, 'f', -1, 64)
			normalizedMetrics[i].Value = resource.MustParse(metricValueAsString)
		}
	}

	return normalizedMetrics, nil
}

// computeMedian computes the median of a sorted slice of float64.
// Assumes the slice is already sorted.
func (r *NormalizerImpl) computeMedian(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		panic("empty slice")
	}
	middle := n / 2
	if n%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}
	return sorted[middle]
}

// computeQuartiles computes Q1 and Q3 of a sorted slice of float64.
// Assumes the slice is already sorted.
func (r *NormalizerImpl) computeQuartiles(sorted []float64) (q1, q3 float64) {
	n := len(sorted)

	// Q1: Median of the lower half
	lowerHalf := sorted[:n/2]
	q1 = r.computeMedian(lowerHalf)

	// Q3: Median of the upper half
	var upperHalf []float64
	if n%2 == 0 {
		upperHalf = sorted[n/2:]
	} else {
		upperHalf = sorted[n/2+1:]
	}
	q3 = r.computeMedian(upperHalf)

	return q1, q3
}
