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

Autoscaler will be operating in multiple phases, for each phane there could be multiple configurable implementations,
each implemented as its own API and a controller.

1. Shard Manager - finds shards aka clusters, also manages their assignments using input from Evaluator.
1. Poller - uses Shard Manager to source shards and gathers metrics for them.
1. Normalizer - optionally, uses a Poller to make metrics on it comparable.
   Not required if polling a single metric or already normalized at the source.
1. Load Indexer - users either a Poller or a Normalizer to combine metrics from each shard into a single per-shard index.
1. Partitioner - uses a Load Indexter and finds shards distribution based accordingly to their Load Indexes.
1. Evaluator - observes a Partitioner over time and makes decisions to apply when appropriate.
1. Scaler - uses Shard Manager to scale Application Controllers accordingly, and restart them as needed.
1. Watch Dog - optionally, detects dead clusters and restarts Application Controllers as needed.

For initial implementation, we will pick one particular algorithm for each phase/controller.

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

## Shard Manager

Manager finds shards (clusters), which are consumed by downstream Pollers.
Between Managers and Pollers there is a simple contract - Manager exports shards as a list
with `.namespace`, `.shardID`, `.shardUID`, and `data`.
`.namespace` is the namespace of the manager itself.
`.shardID` is just a visual identification of that shard, primarily used for observability.
`.shardUID` is unique identifier of the shard by the manager.
`.data` is a map of arbitrary key and values that can be used by the poller (for example in templates for queries).

The manager will also watch Evaluator (if provided) or Partitioner (probably - not a good idea),
and when Evaluator is Ready - it will apply its distribution to `.data.shard` of the cluster secret.

Finally, the manager will export desired amount of replicas from upstream Scaler accordingly to the distribution.

### Secret Type Cluster

We would find all secrets with standard `argocd.argoproj.io/secret-type: cluster` label and identify them as shards.

`.shardID` would be `.metadata.name` of the secret, `.shardUID` would be `.metadata.uid` of the secret.

For `.data` - we would map `.data.name` and `.data.server` 1:1 from the secret itself,
additionally adding `.data.namespace` of the secret (although not sure how would it ever be different from `.namespace`,
but who knows).

To assign shards to replicas, the manager will be updating `.data.shard` on the secret.

## Poller

Poller is responsible for gathering metrics from each shard.
It uses Shard Manager reference to get the shards, as per the contract explained above.

Poller exports the list of metrics it polled together with the same shard object it received from the Shard Manager,
and optionaly with some additional information.

### Prometheus

An obvious choice is to just look at the metrics ArgoCD already export for Prometheus.
There is a good set that we can pull from Prometheus that would represent actual cluster load.

- `sum(argocd_app_info{job="argocd-metrics"}) by (dest_server)`
- `sum(argocd_cluster_api_resource_objects{job="argocd-metrics"}) by (server)`
- `sum(argocd_cluster_api_resources{job="argocd-metrics"}) by (server)`
- `sum(increase(argocd_app_reconcile_count{job="argocd-metrics"}[1m])) by (dest_server)`
- `sum(increase(argocd_cluster_events_total{job="argocd-metrics"}[1m])) by (server)`
- `sum(increase(argocd_app_k8s_request_total{job="argocd-metrics"}[1m])) by (server)`
- `sum(increase(rest_client_requests_total{job="argocd-metrics"}[1m])) by (host)`

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
      quantile_over_time(
        0.50,
        (
          sum by() (argocd_app_info{job="argocd-metrics",namespace="{{ .namespace }}",dest_server="{{ .data.server }}"})
        )[24h:1m]
      )
)
```

For counters:

```
      quantile_over_time(
        0.50,
        (
          sum(increase(argocd_app_reconcile_count{job="argocd-metrics",namespace="{{ .namespace }}",dest_server="{{ .data.server }}"}[1m]))
        )[24h:1m]
      )
```

Also, note that in my installation, `server` label from `argocd_app_k8s_request_total` for some reason
reports as `https://100.64.0.1:443` for cluster that actual `.data.server` is set to `https://kubernetes.default.svc`.
Similarly `host` label from `rest_client_requests_total` is actually just a host instead of a URL for all clusters.
For `kubernetes.default.svc` it becomes `100.64.0.1:443`.
YMMW.
You may need to customize these queries for your setup.
For that purpose, Prometheus Poller uses Go Templates to receive queries from user as templates.
Go Templates are bound with Sprig set of functions: https://masterminds.github.io/sprig/.
See some examples in the (./config/samples/autoscaler_v1alpha1_prometheuspoll.yaml).

Combined, these metrics should give a good idea of how busy each cluster is, therefore - how much load it generates.

## Normalizer

We need to normalize all our metrics (each withing its own dimension) so they are comparable to each other later.
Various normalization implementations can scale metrics to comparable numbers.
For example, Robust Scaling algorithm will get a representation of each metric as a
float number relative to median value of all metrics within that group from all shards which will be represented by 0,
where positive numbers would mean "above median" and negative numbers will mean "below median".
Other algorithms can represent values in certain ranges, for example Min-Max algorithm would result in float values
ranging from 0 to 1.

Normalizer will use the Poller and export same list but with values normalized.

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

## Load Indexer

Load Indexer can either use the Poller directly (if there is a single group of metrics or if the metrics are already
normalized at the source) or the Normalizer to source the metrics.
Using Poller directly without normalization will produce bogus results with metric with higher absolute values
completely overshadowing all other metrics.

Resulting Load Index may vary depending on the implementation, but the point is - it will be a single float value,
which can be used to compare shards between each other and understand which one generates how much load relatively
to each other.

We need to take into account that the source metrics are not equally important among each other.
For example, we are still looking at the number of apps,
but as we've established in the beginning - this metric is not really all that important.
Number of objects and API resources are probably a little more important, but also - not that much.
Number of reconciliations may or may not be important, depending on particular implementation and kinds of Applications.
What would be most important is rate of events and requests to/from the destinations cluster.

Or - I might be completely wrong for **your** particular use case.
Good thing that you can use whatever metrics and queries you want.

With that in mind - we need an algorithm that would allow us to assign weight to each metric,
and prevents some metrics to completely overshadow all other.

### Weighted Positive "p-Norm"

#### What It Is

A **non-linear aggregator** that combines multiple normalized metrics
(each with a different importance) into one "load index".
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

## Partitioner

Partitioners are responsible for producing a distribution of shards to replicas.
Partitioner will use the Load Index of each shard.

Individual algorithms can vary, but generally I am going into this with the following assumption -
we want to scale horizontally.
Which means in the ideal world we would not be needing this autoscaler, and we will just do one shard per one replica.
The problme is that shards are not equal in load, and to accomodate biggest shard, we need to give a lot of resources
to all replicas, while other shards/replicas may totally not need that much.
With that in mind, we will use the "heaviest" shard to determine a maximum size of one replica, so we can equalize
replicas among each other, and then - a uniform resource allocation to the entire StatefulSet would be more meaningful.

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

## Evaluator

Load index and partitioning results will never be consistent, as workloads and user activity changes all the time.

Re-balancing shards is an expensive and disruptive operation, so we all want to minimize that.
`consistent-hashing` sharding implementation in the app controller, for example,
is trying to maintain status quo within +/-10%.

But - we don't want to just minimize re-balancing.
We want to minimize meaningless re-balancing but still embrace change when it's actually needed.

Evaluator will observe the Partitioner over time and export only the "stable" distributions for Shard Manager to pick up.

### "Most Wanted" Two-Phase Hysteresis

One idea was to use a Two-Phase Hysteresis, which is a well known algorighm that would
produce the new partition repeatedly, and if it remains the same for X consecutive checks or amount of time, then apply.
But that as it is would result in a major flaw.
If status quo is already fallen behind, but the new partitioning still can't get stable within the constraints,
then - we are stuck with the status quo.
One way to overcome this would be to reduce the threshold to apply - but that will cause too frequent re-balancing.

So, we will use a custom variation of Two-Phase Hysteresis, that I don't know if it exists, but I called it "Most Wanted".

We will produce and store intended distribution of shards over a configurable amount of time.
That will allow us to "learn" the most "normal" configuration over time and ignore transient spikes.
In the end - we will apply a configuration that was desired the most amount of samples over that time window.
This will guarantee that we will apply an up to date configuration once every predetermined time window.
It may not be the most efficient accordingly to the latest data, but it's reflecting well the most recent history,
without risking to fall behind too much.

In combination with this evaluation implementation,
we should probably be able to increase frequency and granularity of the sampling in the polling phase.
But that mostly depend on the polling parameters.
Increasing poll frequency if our queries get percentiles over 24h - wouldn't change the outcome.
Maybe a good idea would be to experiment with hybrid poll with queries for both historical and current metrics,
with appropriate weights for each.
That way the load index would represent both historical pattern as well as current load.

## Scaler

Implementations may vary, but for initial implementation(s) I am going into this with following assumptions.

We're assuming Application Controller is ran as STS and in `legacy` sharding mode.
Thus, we are making sure none of their baked-in dynamic re-balancing aren't getting in our way.

What happens upon creating a new cluster but without a shard assignment -
it will be assigned to __some__ replica and it will begin reconciliations immediately.

Upon completing all prior phases and using input from either Shards Manager, Partitioner or Evaluator -
scaler can apply corresponding shard assignments.
Choice of upstream provider for replicas to shards distribution is crucial, for example by using Shards Manager -
you would guarantee that the StatefulSet is scaled only after successful shards assignment.

### Stateful Set Default

This default implementation will do two things:
change `.spec.replicas` of the STS and a value of `ARGOCD_CONTROLLER_REPLICAS` environment variable.

Upon Stateful Set becoming Ready, so will the Scaler.

Below is completely optional and based on my observations.
Perhaps, some community input here would be much appreciated.

There are a number of issues circulating on GitHub resulting in stuck clusters and never ending apps refreshes.
I personally never tried to use `legacy` sharding algorithm, so maybe it won't be a problem here.
With `consistent-hashing` I ran into this problem multiple times a day. With `round-robin` it was a weekly occurence.
This issue is probably related to the fact that during a normal scale up/down, there would be different replicas
with different values in `ARGOCD_CONTROLLER_REPLICAS`.
So then some replicas might probably try to manage the same cluster at the same time, or some clusters may not
be managed at all.
Honestly, I have no idea, and this is mostly a speculation.
You can check my comment for more details here: https://github.com/argoproj/argo-cd/issues/15464#issuecomment-2587501293

So, this scaler can optionally "force" correct re-distribution in two possible ways.

#### Rollout Restart

One would be to simply to initiate `rollout restart` (after waiting for the STS update to finish first, of course).
Depends on how many of my assumptions from above are true, and what kinds of issues you observe in your environment,
this may or may not work for you.

#### X-0-Y

This one would be more interesting.
It would be my personal choice, although some may find this a little too disruptive.
For me - it's ok based on my Evaluator set to 7 days and the fact that number of clusters in my environment don't
change all that much.
It's amount of apps and user activity across namespaces that does (in my environment).

As Shards Manager effectively is listening to Scaler for instructions, the Scaler can choose at which time to make that
information available.
First, Scaler can scale STS down to 0 thus make sure no replicas are up.
Then, and only then, can it export new replica/shard distribution for the Shards Manager to pick up.
Finally and only after Shards Manager reports success - we can scale out STS to its new desired replica count.

This effectively guarantees that at no point in time there would be two or more replicas with different
value in `ARGOCD_CONTROLLER_REPLICAS` environment variable.
At the cost of a brief downtime, which honestly, likely no one will ever even notice.
