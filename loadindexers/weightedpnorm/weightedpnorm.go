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

package weightedpnorm

import (
	"fmt"
	"math"
	"strconv"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
)

type LoadIndexer interface {
	Calculate(int32, map[string]autoscaler.WeightedPNormLoadIndexWeight, []common.MetricValue) ([]common.LoadIndex, error)
}

type LoadIndexerImpl struct{}

func (li *LoadIndexerImpl) Calculate(
	p int32,
	weights map[string]autoscaler.WeightedPNormLoadIndexWeight,
	values []common.MetricValue,
) ([]common.LoadIndex, error) {
	metricsByShardByID := map[types.UID]map[string][]common.MetricValue{}
	metricsByShard := map[types.UID][]common.MetricValue{}
	shardsByUID := map[types.UID]common.Shard{}
	for _, m := range values {
		if _, exists := metricsByShardByID[m.Shard.UID]; !exists {
			metricsByShardByID[m.Shard.UID] = map[string][]common.MetricValue{}
		}
		if _, exists := metricsByShardByID[m.Shard.UID][m.ID]; !exists {
			metricsByShardByID[m.Shard.UID][m.ID] = []common.MetricValue{}
		}
		metricsByShardByID[m.Shard.UID][m.ID] = append(metricsByShardByID[m.Shard.UID][m.ID], m)
		if _, exists := metricsByShard[m.Shard.UID]; !exists {
			metricsByShard[m.Shard.UID] = []common.MetricValue{}
		}
		metricsByShard[m.Shard.UID] = append(metricsByShard[m.Shard.UID], m)
		if _, exists := shardsByUID[m.Shard.UID]; !exists {
			shardsByUID[m.Shard.UID] = m.Shard
		}
	}
	for shardUID, shardMetricsByID := range metricsByShardByID {
		for metricID := range weights {
			if values, exists := shardMetricsByID[metricID]; !exists || len(values) != 1 {
				return nil, fmt.Errorf("metric ID '%s' should exist exactly one time in shard %s", metricID, shardUID)
			}
		}
	}

	loadIndexes := []common.LoadIndex{}
	for shardUID, values := range metricsByShard {
		value := li.loadIndex(
			p,
			weights,
			values,
		)
		valueAsString := strconv.FormatFloat(value, 'f', -1, 64)
		valueAsResource, err := resource.ParseQuantity(valueAsString)
		if err != nil {
			return nil, err
		}
		loadIndexes = append(loadIndexes, common.LoadIndex{
			Shard:        shardsByUID[shardUID],
			Value:        valueAsResource,
			DisplayValue: valueAsString,
		})
	}

	return loadIndexes, nil
}

func (_ *LoadIndexerImpl) loadIndex(
	p int32,
	weights map[string]autoscaler.WeightedPNormLoadIndexWeight,
	values []common.MetricValue,
) float64 {
	sum := 0.0
	for _, m := range values {
		weight, ok := weights[m.ID]
		if !ok {
			continue
		}
		weightValue := weight.Weight.AsApproximateFloat64()
		value := m.Value.AsApproximateFloat64()
		if weight.Negative != nil && !*weight.Negative && value < 0 {
			value = float64(0)
		}
		sum += weightValue * math.Pow(math.Abs(value), float64(p))
	}

	return math.Pow(sum, float64(1)/float64(p))
}
