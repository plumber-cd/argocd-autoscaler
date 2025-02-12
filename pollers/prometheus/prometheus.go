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

// Package prometheus provides a Prometheus poller.
package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"

	pm "github.com/prometheus/client_golang/api/prometheus/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/plumber-cd/argocd-autoscaler/api/autoscaler/common"
	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
)

type Poller interface {
	Poll(ctx context.Context, poll autoscaler.PrometheusPoll, shards []common.Shard) ([]common.MetricValue, error)
}

type PollerImpl struct{}

func (r *PollerImpl) Poll(
	ctx context.Context,
	poll autoscaler.PrometheusPoll,
	shards []common.Shard,
) ([]common.MetricValue, error) {

	log := log.FromContext(ctx).WithValues("poller", "prometheus")

	client, err := api.NewClient(api.Config{
		Address: poll.Spec.Address,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %w", err)
	}

	promAPI := pm.NewAPI(client)

	metrics := []common.MetricValue{}
	for _, metric := range poll.Spec.Metrics {
		for _, shard := range shards {
			tmplParams := map[string]interface{}{
				"id":        shard.ID,
				"uid":       shard.UID,
				"namespace": shard.Namespace,
				"name":      shard.Name,
				"server":    shard.Server,
			}

			tmpl, err := template.New("query_" + metric.ID).Funcs(sprig.FuncMap()).Parse(metric.Query)
			if err != nil {
				log.Error(err, "Failed to parse Prometheus query as Go template", "metric", metric.ID)
				return nil, err
			}

			var queryBuffer bytes.Buffer
			if err := tmpl.Execute(&queryBuffer, tmplParams); err != nil {
				log.Error(err, "Failed to execute Prometheus query template", "metric", metric.ID)
				return nil, err
			}

			query := queryBuffer.String()
			log.V(2).Info("Executing Prometheus query", "metric", metric.ID, "query", query)

			result, warnings, err := promAPI.Query(ctx, query, time.Now())
			if err != nil {
				log.Error(err, "Failed to query Prometheus", "metric", metric.ID, "query", query)
				return nil, fmt.Errorf("error querying Prometheus: %w, query:\n%s", err, query)
			}
			if len(warnings) > 0 {
				log.Error(nil, "Prometheus query warnings", "warnings", warnings, "metric", metric.ID, "query", query)
			}

			vectorVal, ok := result.(model.Vector)
			if !ok {
				log.Error(nil, "Unexpected result type", "result", result, "metric", metric.ID, "query", query)
				return nil, fmt.Errorf("unexpected result type: %T, query:\n%s", result, query)
			}

			var value float64
			if len(vectorVal) == 0 {
				if metric.NoData == nil {
					log.Error(nil, "Result from Prometheus had no data", "metric", metric.ID, "query", query)
					return nil, fmt.Errorf("Result from Prometheus had no data, query:\n%s", query)
				}
				value = metric.NoData.AsApproximateFloat64()
			} else if len(vectorVal) != 1 {
				// We require users to use promql that returns a single value ready for normalization
				log.Error(nil, "Expected exactly 1 sample", "result", vectorVal, "metric", metric, "query", query)
				return nil, fmt.Errorf("Expecter exaxtly 1 sample, got: %d, query:\n%s", len(vectorVal), query)
			} else {
				sample := vectorVal[0]
				value = float64(sample.Value)
			}

			log.V(2).Info("Prometheus query response", "metric", metric.ID, "value", value, "query", query)

			valueAsString := strconv.FormatFloat(value, 'f', -1, 64)
			valueAsResource, err := resource.ParseQuantity(valueAsString)
			if err != nil {
				log.Error(err, "Failed to parse value as resource", "metric", metric.ID, "value", value)
				return nil, err
			}

			metrics = append(metrics, common.MetricValue{
				ID:           metric.ID,
				Shard:        shard,
				Query:        query,
				Value:        valueAsResource,
				DisplayValue: valueAsString,
			})
		}
	}

	return metrics, nil
}
