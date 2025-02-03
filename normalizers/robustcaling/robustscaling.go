package robustscaling

import (
	"context"
	"sort"
	"strconv"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Normalizer interface {
	Normalize(ctx context.Context, allMetrics []common.MetricValue) ([]common.MetricValue, error)
}

type NormalizerImpl struct{}

func (r *NormalizerImpl) Normalize(ctx context.Context,
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
