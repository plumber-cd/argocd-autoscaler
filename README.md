# argocd-autoscaler

This controller can automatically partition shards (destination kubernetes clusters) to ArgoCD Application Controllers,
determine how many App Controller replicas are needed for that partitioning, and scale App Controllers accordingly.

There are three levels of resolution that I can explain how this works and how to use it.

## TL;DR

Level one aka TL;DR version: it will look at prometheus metrics from App Controllers,
determine load index of each destination cluster,
and partition clusters to replicas in the most efficient way.
And, scale the App Controllers accordingly, too. You can install it using kustomize in three simple steps:

1. Grab CRDs from [./config/crd](./config/crd)
1. Grab autoscaler from [./config/default](./config/default)
1. Grab our default opinionated scaling strategy from [./config/default-scaling-strategy](./config/default-scaling-strategy)

Sample `kustomization.yaml` that you may want to use:

```yaml
namespace: argocd
resources:
- github.com/plumber-cd/argocd-autoscaler/config/crd?ref=vX.X.X
- github.com/plumber-cd/argocd-autoscaler/config/default?ref=vX.X.X
- github.com/plumber-cd/argocd-autoscaler/config/default-scaling-strategy?ref=vX.X.X
patches:
- target:
    kind: Deployment
    name: argocd-autoscaler
  patch: |-
    - op: add
      path: /spec/template/spec/containers/0/resources
      value:
        resources:
          requests:
            cpu: 10m
            memory: 64Mi
          limits:
            memory: 128Mi
```

Note that it does only partitioning and horizontal scaling.
It is still on you to appropriately scale App Controllers CPU/mem based on how the load is getting distributed.
This autoscaler aims to keep all replicas utilized equally, so - usual scaling tools would work just fine.

## Advanced configuration

### Customize scaling strategy

You can use [./config/defaul-scaling-strategy](./config/default-scaling-strategy) as a starting point.
The things you will likely want to customize are `poll.yaml` and `load-index.yaml` and `evaluation.yaml`.

The `poll.yaml` uses `PrometheusPoll` and controls what queries are made to Prometheus.
Based on your preference, you may adjust what works for you.
Note that queries must return a single value.

The `load-index.yaml` uses `WeightedPNormLoadIndex` to calculate load index using normalized polling results with weights.
Here, you may want to adjust how much each individual metric contributes to the result.

The `evaluation.yaml` uses `MostWantedTwoPhaseHysteresisEvaluation` to observe and promote partitioning.
Here, you may want to customize how long should it be observing before electing and applying a re-shuffle.

Customization recommendations based on defaults are as follows.

Generally speaking, you may want to follow default opinionated `quantile_over_time` sampling and maybe only modify
what are quantiles sampled on and their weights.
Otherwise - you want to keep in mind to keep sampling at high enough quantile and for longer ranges.
This helps to "remember" spikes for longer,
and have them reflected in the load index later.
You will receive a mostly flat load index, and that's what you are aiming for.
Otherwise, spikes will not inform overall partitioning decisions in the evaluation phase,
and (provided that at idle all shards are at about the same rate of utilization overall) - you will most likely
end up with partitioning of one shard per one replica.
Which is totally an option if this is literally all you care about - to make it automatically schedule new replicas
for new clusters.

It may be tempting to remove evaluation piece and use load index directly at the scaling,
if you want to make scaling more reactive and instantaneous.
Which is fine if you are aiming for one shard = one replica,
but doing that otherwise may result in App Controller restarts every poll cycle and instability.

There is a middle ground which is to still use evaluation,
use `max` over a minimal possible polling period to get latest values in queries,
and dial down evaluation stabilization period to much shorter value that you are comfortable with how often it can restart.

You can use [./grafana/argocd-autoscaler.json](./grafana/argocd-autoscaler.json) dashboard to aid you in figuring out optimal values for you.
You can deploy everything but `scaler.yaml`, which will essentially make it a dry-run.
You'll see in Grafana what it would want to do under current scaling strategy without it actually doing anything.
For that, of course, you'd need to deploy a `ServiceMonitor` (see below).

Lastly, you may want to customize `scaler.yaml` to adjust how it applies changes to the STS.
For that - refer to [this section of the design document](./DESIGN.md#replica-set-default) for more information.

### Monitor Autoscaler with Prometheus

Instead of `github.com/plumber-cd/argocd-autoscaler/config/default`
you can use `github.com/plumber-cd/argocd-autoscaler/config/default-with-monitoring`.
That deploys additional metrics service and a service monitor.
Here's a sample `kustomization.yaml` that you may want to use:

```yaml
namespace: argocd
resources:
- github.com/plumber-cd/argocd-autoscaler/config/crd?ref=vX.X.X
- github.com/plumber-cd/argocd-autoscaler/config/default-with-monitoring?ref=vX.X.X
- github.com/plumber-cd/argocd-autoscaler/config/default-scaling-strategy?ref=vX.X.X
patches:
- target:
    kind: Deployment
    name: argocd-autoscaler
  path: argocd-application-controller-remove-replicas-patch.yaml
  patch: |-
    - op: add
      path: /spec/template/spec/containers/0/resources
      value:
        resources:
          requests:
            cpu: 10m
            memory: 64Mi
          limits:
            memory: 128Mi
- target:
    kind: NetworkPolicy
    name: argocd-autoscaler-allow-metrics-traffic
  patch: |-
    - op: replace
      path: /spec/ingress/0/from/0/namespaceSelector/matchLabels
      value:
        kubernetes.io/metadata.name: <namespace where your prometheus lives>
- target:
    kind: ServiceMonitor
    name: argocd-autoscaler
  path: argocd-autoscaler-metrics-network-policy-patch.yaml
  patch: |-
    - op: add
      path: /metadata/labels
      value:
        <your prometheus selector label>: <your prometheus selector label value>
```

At [./grafana/argocd-autoscaler.json](./grafana/argocd-autoscaler.json) you may find a dashboard that I am using.

There are also other dashboards generated by `kubebuilder` - runtime telemetry is good but custom metrics one is useless.

### Secured Autoscaler with Prometheus

Generally speaking, for securing communications between prometheus and `/metrics` endpoint, typically means:

1. Network Policy that allows only Prometheus to access the `/metrics` endpoint.
1. Put `/metrics` endpoint behind authentication.
1. Apply TLS encryption for `/metrics` endpoint.

Network policy is already included in the previous example. For the rest - you do need additional RBAC and CertManager.

Once you make sure these prerequisites are deployed, you may want to use [./config/default-secured](./config/default-secured).
This will apply authentication and TLS to the `/metrics` endpoint,
and it will also modify ServiceMonitor to inform Prometheus how to securely scrape it.

## Hardcore

I have a pretty detailed document at [./DESIGN.md](./DESIGN.md) that explains all the implementation details.
It can help you to customize everything and anything or hopefully even contribute more ideas that we can implement.
At the current stage of the project, we really aim to provide a framework rather than a end result solution.
Although we do include default opinionated scaling strategy - only you can define a strategy that would work best for yourself.
