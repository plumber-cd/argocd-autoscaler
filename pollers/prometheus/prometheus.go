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

	autoscaler "github.com/plumber-cd/argocd-autoscaler/api/autoscaler/v1alpha1"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
)

type Poller struct{}

func (r *Poller) Poll(
	ctx context.Context,
	poll autoscaler.PrometheusPoll,
	shards []autoscaler.DiscoveredShard,
) ([]autoscaler.MetricValue, error) {

	log := log.FromContext(ctx).WithValues("poller", "prometheus")

	client, err := api.NewClient(api.Config{
		Address: poll.Spec.Address,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %w", err)
	}

	promAPI := pm.NewAPI(client)

	metrics := []autoscaler.MetricValue{}
	for _, metric := range poll.Spec.Metrics {
		for _, shard := range shards {
			tmplParams := map[string]interface{}{
				"id":        shard.ID,
				"namespace": poll.Namespace,
				"data":      shard.Data,
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

			valueAsResource, err := resource.ParseQuantity(
				strconv.FormatFloat(value, 'f', -1, 32))
			if err != nil {
				log.Error(err, "Failed to parse value as resource", "metric", metric.ID, "value", value)
				return nil, err
			}

			metrics = append(metrics, autoscaler.MetricValue{
				Poller:   "prometheus",
				ShardUID: shard.UID,
				ID:       metric.ID,
				Query:    query,
				Value:    valueAsResource,
			})
		}
	}

	return metrics, nil
}
