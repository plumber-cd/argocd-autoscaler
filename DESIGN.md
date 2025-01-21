# argocd-autoscaler

# Problem

Although over time this project could expand in other ArgoCD controllers, initially the focus is Application Controller.

ArgoCD Application Controller is only capable to shard based on destination clusters,
i.e. all the apps destined to a particular cluster will always be reconciled by the same shard.
ArgoCD Application Controller has three sharding algorithms (at the moment of creating of this repository).
The first two `legacy` and `round-robin` -
effectively are different ways to assign destination clusters to shards without looking at any data.
The new (and still beta) `consistent-hashing` algorithm is trying to use data,
but it makes a wrongful assumption that all Applications do generate equal amount of reconciliation work,
which is simply not true.

We need something better. And also configurable.

# Solution

Autoscaler will be operating in multiple phases, for each phane there could be multiple configurable implementations:

1. Discovery - find shards aka clusters.
1. Poll - gather metrics from shards.
1. Normalize - make metrics comparable.
1. Load Index - combine metrics from each cluster into a single index.
1. Partition - distribute shards based accordingly each cluster index.
1. Evaluate - observe over time that the desired partition is the best choice.
1. Apply - assign shards to clusters in ArgoCD and restart controllers.

Each phase is a separate API and a controller to allow for extendability.

For initial implementation, we will pick one particular algorithm for each phase.

A quick disclaimer - Kubernetes doesn't like floats (because they are not the same across languages)
and suggests to avoid them.
They in fact prevent you from using them in `kubebuilder`, unless you enable use of "dangerous" types.
We use a lot of floats here.
We'll follow their educated advice, and use `resource.Quantity` type to store floats.
It will be stored as a `string` as a result,
and you may recognize it as it is the same thing they use for resource requests/limits.
Despite visual similarity (it would be saying `500m` to represent `0.5`),
these valued **DO NOT** have anything to do with CPU or memory.
It is just a string representation of a float with precision rules replicated among all languages.

## Discovery

First of all - we need to find our clusters. There are multiple ways to add clusters in ArgoCD,
I am using secrets labeled with `argocd.argoproj.io/secret-type: cluster` standard label.

### Secret Type Cluster

We would find all secrets with `argocd.argoproj.io/secret-type: cluster` label.
Shard ID would be the name of the secret, UID would be UID of the secret,
and for `.data` - we would use `.data.name` and `.data.server` from that very secret.

## Poll

An obvious choice is to just look at the metrics ArgoCD already export for Prometheus.
There is a good set that we can pull from Prometheus that would represent actual cluster load.

### Prometheus

- `sum(argocd_app_info{job="argocd-metrics"}) by (dest_server)`
- `sum(argocd_cluster_api_resource_objects{job="argocd-metrics"}) by (server)`
- `sum(argocd_cluster_api_resources{job="argocd-metrics"}) by (server)`
- `sum(increase(argocd_app_reconcile_count{job="argocd-metrics"}[1m])) by (dest_server)`
- `sum(increase(argocd_cluster_events_total{job="argocd-metrics"}[1m])) by (server)`
- `sum(increase(argocd_app_k8s_request_total{job="argocd-metrics"}[1m])) by (server)`
- `sum(increase(rest_client_requests_total{job="argocd-metrics"}[1m])) by (host)`

Note that in my installation, `server` label from `argocd_app_k8s_request_total` for some reason
reports as `https://100.64.0.1:443` for cluster that actual `.server` is set to `https://kubernetes.default.svc`.
Similarly `host` label from `rest_client_requests_total` is actually just a host instead of a URL for all clusters.
For `kubernetes.default.svc` it becomes `100.64.0.1:443`.
YMMW.
You may need to customize these queries for your setup.

There's probably more (for example - `argocd_kubectl_exec_pending` and `workqueue_depth` would be nice if they had `server` or `host` label),
and we can add more in future, but that's a good starting point.
Perhaps, pod cpu/memory consumption could also be included into the equation.

These queries are just for visual examination (easy to copy into Grafana).
The actual queries for the autoscaler need to be tweaked a little.

ArgoCD is not really a spiky workload, and scaling it often due to sharding implementation is not really a good idea.
So, we can probably use percentiles over large time frame.
What exact percentile and over what exact time frame would largely depend on the user, choice of metrics,
normalization and aggregation choices, as well as final consensus algorithm.

Let's tweak queries accordingly.

For gauges:

```
quantile_over_time(
  0.50,
  (
    sum by() (argocd_app_info{job="argocd-metrics",namespace="argocd",dest_server="https://kubernetes.default.svc"})
  )[24h:1m]
)
```

For counters:

```
quantile_over_time(
  0.50,
  (
    sum(increase(argocd_cluster_events_total{job="argocd-metrics",namespace="argocd",server="https://kubernetes.default.svc"}[1m]))
  )[24h:1m]
)
```

Combined, these metrics should give a good idea of how busy each cluster is, therefore - how much load it generates.

## Normalize

First, we need to normalize all our metrics (each withing its own dimension) so they are comparable to each other later.
We will be using Robust Scaling normalization for that.
As a result, we will get a representation of each metric as a
float number relative to median value which will be represented by 0.

### Robust Scaling

#### What It Is

Robust scaling is a method of **normalizing** different metrics so that they can be compared on a common scale without outliers skewing the results.
It uses robust statistics (median and interquartile range, often denoted IQR) rather than the mean and standard deviation.

#### How It Works

1. **Median and IQR (Interquartile Range):**
   - Find the median (`m`) of all metrics.
   - Calculate `IQR = Q3 - Q1` (we will use 75th percentile minus 25th percentile).
2. **Scale Each Value (`x`):**
   `x_robust = (x - m) / IQR`
3. **Benefit:**
   - Less sensitive to extreme outliers than other methods (like min-max).
   - Keeps each metric’s range more comparable.

## Load Index

Now, we can use normalized metrics from each cluster to calculate its Load Index.
That way, we will be able to compare the load for partitioning.
We need to take into account that metrics are not equally important among each other.
For example, we are still looking at the number of apps,
but as we've established in the beginning - this metric is not really all that important.
Number of objects and API resources are probably a little more important, but also - not that much.
Number of reconciliations may or may not be important, depending on particular implementation and kinds of Applications.
What would be most important is rate of events, requests and numbers of connections to the destinations cluster.

With that in mind - we need an algorithm that would allow us to assign weight to each metric,
and prevents some metrics to completely overshadow all other.

### Weighted Positive "p-Norm"

#### What It Is

A **non-linear aggregator** that combines multiple normalized metrics (each with a different importance) into one "load index".
Rather than summing metrics directly, we apply a **p-norm** to reduce the effect of any single large metric.
We use positive variation as that would be easier to represent and use for charts later.

#### How It Works

1. **Inputs:**
   - Suppose you have `k` robust-scaled metrics:
     `x_1(S), x_2(S), ..., x_k(S)`.
   - Each metric `i` has an importance weight `w_i`.
   - Choose a `p` value (commonly 2 or 3) that controls how much large values stand out.

2. **Formula (p-Norm):**
   `LoadIndex(S) = ( Σ[i=1..k] [ w_i * ( x_i(S) )^p ] )^(1/p)`

3. **Interpretation:**
   - If `p = 1`, this reduces to a simple weighted sum.
   - If `p = 2`, it behaves like a weighted Euclidean norm (moderate emphasis on larger values).
   - Larger `p` values increase the influence of the largest metric but still combine the rest.

## Partitioning

Based on the load index, and keeping in mind that we want to scale horizontally - 
we will let the "heaviest" cluster to get an entire replica of the app controller to itself.
That will define the size of our buckets.
Then, we will do the partitioning with LPT algorithm.

### Longest Processing Time (LPT)

**Goal:** Assign shards (each with a computed load index) to replicas in a way that
attempts to minimize the makespan (the highest total load on any replica).

1. **Sort Shards** in descending order of load index.
2. **Iterate Over Shards** one at a time (from largest load to smallest):
   - Place each shard in the replica that has the **lowest current load** so far.
   - If there is no replicas where it could fit - create a new replica.
3. **Reasoning:**
   - LPT spreads out the largest tasks first, which often leads to more balanced usage of replicas.
   - It is a well-known heuristic in scheduling problems (and closely related to bin packing).

## Evaluate

Load index and partitioning results will never be consistent, as workloads and user activity changes all the time.

Re-balancing shards is an expensive and disruptive operation, so we all want to minimize that.
`consistent-hashing` sharding implementation in the app controller is trying to maintain status quo within +/-10%.

But - we don't want to just minimize re-balancing.
We want to minimize meaningless re-balancing but still embrace change when it's actually needed.

One idea was to use a Two-Phase Hysteresis.
But that would have a major flaw, as we don't want to fall behind
if pattern of the entire system at large indeed changed,
but we cannot determine how exactly because of the transient spikes.

So, we will use a variation of Two-Phase Hysteresis.

### Two-Phase Hysteresis "Most Wanted"

We will produce and store intended distribution of shards over a configurable amount of time.
That will allow us to "learn" the most "normal" configuration we need and ignore transient spikes.
In the end - we will apply a configuration that was desired the most amount of samples over that time window.

In the combination with this evaluation implementation,
we should probably be able to increase frequency and granularity of the sampling in the polling phase.

## Apply

We're assuming Application Controller is ran as STS and in `legacy` sharding mode.
This, we are making sure none of their re-balancing processes aren't getting in our way.

This controller, upon deciding that it is time to apply the intent,
will modify `argocd.argoproj.io/secret-type: cluster` secrets to set `.spec.shard`.

### Rollout Restart

Once cluster secrets are updated,
we want to change STS `.spec.replicas` and `ARGOCD_CONTROLLER_REPLICAS` environment variable on it accordingly.
This will restart replicas and force re-distribution. Hopefully.
If current STS spec did not change (same number of replicas with different shard distribution) -
we may need to trigger rolling restart ourselves.

### Scale X-0-Y

Sometimes, I've noticed issues with re-balancing via a normal rollout restart.
It may or may not be related to the fact that not at any point in time all the replicas have consensus on the sharding.
Old and new generation of replicas would have different `ARGOCD_CONTROLLER_REPLICAS` value and actively work on clusters.

One way to overcome this would be to scale STS to 0, wait, and scale it out to new desired value.
This is potentially distruptive operation, so a user should be able to configure a maintainence windows where that can be applied.

## Bonus - auto restarts

There seem to be various dead locks in the app controller -
for some examples you can check [my comment](https://github.com/argoproj/argo-cd/issues/15464#issuecomment-2587501293).

As a separate toggleable feature, a separate controller can detect that a cluster that still exists
as a `argocd.argoproj.io/secret-type: cluster` secret is not being observed in any of the shards.
Meaning it is currently "dead" with no activity.

In that case, controller can restart pods of the intended shard replica as well as the pod that reports this cluster in its metrics (if any).
Obviously the cluster can be actually dead, for various reasons...
So, there needs to be a backoff mechanism to prevent restarting app controllers just because one cluster is down.
