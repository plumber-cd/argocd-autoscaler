# argocd-autoscaler

# Problem

The focus of this project is - Application Controller.
Other tools from HPA to things like KEDA already exists that should enable anyone to scale other ArgoCD components.
Application Controller pose somewhat unique challenges that require a bespoke solution:

1. Controller "busyness" is not directly correlate to its CPU/memory consumption. There are queues and rate throttling involved.
1. A variety of directly incomparable metrics needs to be used (like - reconciliation rate and object count), creating the need for normalization.
1. Scaling is not linear process of adding/removing replicas - partitioning needs to be calculated and shards needs to be assigned to replicas.
1. Process to execute scaling intent and assign sharding is unique to Application Controller (modifying secrets and env vars on the App Controller).

ArgoCD Application Controller is only capable to shard based on destination clusters,
i.e. - all the apps destined to a particular cluster will always be reconciled by the same replica.
ArgoCD Application Controller has three sharding algorithms (at the moment of creation of this project).
The first two `legacy` and `round-robin` -
effectively are different ways to assign destination clusters to replicas without looking at any data.
The new (and still beta) `consistent-hashing` algorithm is trying to use data,
but it makes a (wrongful) assumption that all Applications do generate equal amount of work,
as it only tries to keep in balance the total amount of Applications per replica.

We need something better. And - also configurable, because everyone's scaling challenges are unique.

# Solution

Autoscaler will be operating in multiple phases, for each phase there could be multiple configurable implementations,
each implemented as its own API and a controller.

1. Shard Manager - finds shards aka clusters and manage shard assignments.
1. Poller - uses Shard Manager to find shards and poll metrics for them.
1. Normalizer - uses a Poller to make metrics from it comparable.
   Not required if polling a single metric or metrics are already normalized at the source.
1. Load Indexer - users either a Poller or a Normalizer to combine metrics from each shard into a single per-shard index.
1. Partitioner - uses a Load Indexer and calculates shards distribution balanced accordingly to their Load Indexes.
1. Evaluator - observes a Partitioner over time and makes decisions to apply when appropriate.
1. Scaler - monitors either Evaluator or a Partitioner, applies shard assignments to the Shard Manager, and scale replicas.

For initial implementation, we will pick one particular algorithm for each phase/controller.
The goal of the initial implementation is not to provide a best solution, but to provide a framework,
because everyone's definition of "best" will be unique accordingly to their setup.
With the input from the community, we can later extend with more implementations.

A quick disclaimer - Kubernetes doesn't like floats (because they are not treated equally across languages),
and they suggests to avoid them.
They, in fact, are preventing you from using them in `kubebuilder`, unless you enable use of "dangerous" types.
We use a lot of floats here in this project.
We'll follow their educated advice, and use `resource.Quantity` type to store floats.
It will be stored as a `string`,
and you may recognize those strings, as it is the same format that they are using for resource requests/limits.
Despite visual similarity (it would be saying `500m` to represent `0.5`, for example),
these values **DO NOT** have anything to do with CPU or memory.
It is just a string representation of a float with precision rules replicated among all languages.

## Common Data Types

### Shard

Shard represents a cluster managed by ArgoCD Application Controller.

- `.uid` - unique identifier of the shard (i.e. UID of the secret).
- `.id` - identifier of the shard (i.e. name of the secret), not guaranteed to be unique and mostly used for human readability in logs and metrics.
- `.namespace` - a namespace where this shard was found in.
- `.name` - a name of the shard as seen in the ArgoCD i.e. name of the destination cluster.
- `.server` - a URL of the destination server as seen in the ArgoCD for this shard.

### Metric Value

- `.id` - is a unique identifier of the metric.
- `.shard` - is a Shard object.
- `.query` - is a query used to fetch this value.
- `.value` - is a value of the metric.
- `.displayValue` - is a value of the metric formatted for display (may loose precision).

### Load Index

- `.shard` - is a Shard object.
- `.value` - is a value of the load index.
- `.displayValue` - is a value of the load index formatted for display (may loose precision).

### Replica

- `.id` - is a unique identifier of the replica (these are just indexes stored as strings).
- `.loadIndexes` - is a list of Load Index objects assigned to this replica (which in turn have their Shards inside).
- `.totalLoad` - is a sum of all Load Indexes assigned to this replica.
- `.totalLoadDisplay` - is a `.totalLoad` formatted for display (may loose precision).

## Shard Manager

Manager finds shards (clusters), and exports findings in the `.status.shards` field.

If the `.spec.replicas` is defined, the manager will ensure that actual shards are assigned to replicas as desired.

### Secret Type Cluster

It would find all secrets with standard `argocd.argoproj.io/secret-type: cluster` label and export them as
`.status.shards` as a list of Shards.

`.id` would be `.metadata.name` of the secret, `.uid` would be `.metadata.uid` of the secret.
`.namespace` will be the namespace of the secret.
`.name` and `.server` would be extracted from the secret `.data.name` and `.data.server` fields.

To assign shards to replicas, the manager will be updating `.data.shard` on the secret.

## Poller

Poller is responsible for gathering metrics from each shard.
It uses Shard Manager referenced in `.spec.shardManagerRef` to discover the shards, and it can use data from the shard
to construct appropriate queries.

Poller exports resulting metrics into the `.status.values` as a list of Metric Value objects.

### Prometheus

An obvious choice is to just look at the metrics ArgoCD already exports for Prometheus.
There is a good set that we can pull from Prometheus that would represent actual cluster load.
Here's few examples:

- `sum(argocd_app_info{job="argocd-metrics"}) by (dest_server)`
- `sum(argocd_cluster_api_resource_objects{job="argocd-metrics"}) by (server)`
- `sum(argocd_cluster_api_resources{job="argocd-metrics"}) by (server)`
- `sum(increase(argocd_app_reconcile_count{job="argocd-metrics"}[1m])) by (dest_server)`
- `sum(increase(argocd_cluster_events_total{job="argocd-metrics"}[1m])) by (server)`
- `sum(increase(argocd_app_k8s_request_total{job="argocd-metrics"}[1m])) by (server)`
- `sum(increase(rest_client_requests_total{job="argocd-metrics"}[1m])) by (host)`

There's probably more.
For example - `argocd_kubectl_exec_pending` and `workqueue_depth` would be nice if they had `server` or `host` label.
Perhaps, pod cpu/memory should also be included.
But, above would be a good start.

These queries above are just for visual examination (easy to copy into Grafana).
The actual queries for the autoscaler needs to be tweaked a little.

ArgoCD is not really a spiky workload, and scaling it too often is not really a good idea.
Shards re-balancing is an expensive process that often causes disruptions.
So, we can probably use percentiles over large time frame, to smooth things out.
What exact percentile and over what exact time frame would largely depend on the user, choice of metrics,
normalization and aggregation choices, as well as final evaluation algorithm.

Let's tweak queries accordingly, just to make an example.

For gauges:

```
quantile_over_time(
    0.95,
    (
        sum(argocd_app_info{job="argocd-metrics",namespace="{{ .namespace }}",dest_server="{{ .shardServer }}"})
    )[1h:1m]
)
```

For counters:

```
quantile_over_time(
    0.95,
    (
        sum(increase(argocd_app_reconcile_count{job="argocd-metrics",namespace="{{ .namespace }}",dest_server="{{ .shardServer }}"}[1m]))
    )[1h:1m]
)
```

Also, note that in my installation, `server` label from `argocd_app_k8s_request_total` for some reason
reports as `https://100.64.0.1:443` for cluster that actual `.shardServer` is set to `https://kubernetes.default.svc`.
Similarly, `host` label from `rest_client_requests_total` is actually just a host instead of a URL for all clusters.
For `kubernetes.default.svc` it becomes `100.64.0.1:443`.
YMMW.
You may need to customize these queries for your setup.
For that purpose, Prometheus Poller uses Go Templates to receive queries from user as templates.
Go Templates are bound with Sprig set of functions: https://masterminds.github.io/sprig/.
See some examples in the [./config/default-scaling-strategy/poll.yaml](./config/02-default-scaling-strategy/02-poll.yaml).

Combined, these metrics should give a good idea of how busy each cluster is, therefore - how much load it generates.

## Normalizer

We need to normalize Metric Values from the Poller so they can be comparable to each other.
Various normalization algorithms exists that can scale metrics to comparable numbers.
For example, Robust Scaling algorithm will get a representation of each metric as a
float number relative to median value of all metrics within its group from all shards which will be represented by 0,
where positive numbers would mean "above median" and negative numbers will mean "below median".
Other algorithms can represent values in certain ranges, for example Min-Max algorithm would result in float values
ranging from 0 to 1.

Normalizer will use the Poller defined at `.spec.metricValuesProviderRef` and export same list of `.status.values` as list of Metric Values,
but with all values being normalized.

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

4. **Positive Offset:**
   - Output from this normalizer will be a set of values centered around 0, with negative values
     representing metrics below the median.
     Some implementations of the load index do not work well with negative values.
     Particularly, the only one that's currently implemented - Weighted Positive "p-Norm".
   - Additional option exists to make sure that there are no negative values by using offset to the right:
     `offset = -(min_value) + ε * (max_value - min_value))`.
    A good value for `ε` based on my practical tests seem to be `0.01`.
    A value of `0` will just upscale metrics so that the smallest one is at `0`, which is also an option,
    assuming there always would be other non-zero metrics from that same shard.
    I just wanted all non zero metrics to have non zero weights after normalization.

## Load Indexer

Load Indexer can either use Normalizer (or - directly the Poller) to source the metrics from `.spec.metricValuesProviderRef`.
Using Poller directly without normalization will produce bogus results where metrics with higher absolute values
completely overshadowing all other metrics.
Only do that if you either polling only one metric, or your metrics has already been normalized at the source.

Resulting Load Index may vary depending on the implementation, but the point is - it will be a single float value,
which can be used to compare shards between each other in terms of volume of load they each generate.
Resulting Load Index will be exported in the `.status.values` field as a list of Load Index objects.

Additionally, we need to take into account that the source metrics are not equally important among each other.
For example, we might still be looking at the number of apps,
but as we've established in the beginning - this metric is not really all that important.
Number of objects and API resources are probably a little more important, but also - not that much.
Number of reconciliations may or may not be important, depending on particular implementation and kinds of Applications.
What would be most important is the rate of events and requests to/from destinations clusters.

Or - I might be completely wrong for **your** particular use case.
Good thing that you can use whatever metrics and queries you want ;)

With that in mind - we need an algorithm that would allow us to assign weight to each metric,
and prevents some metrics to completely overshadow all others.

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
   - Tip: with Robust Scaling normalization with positive offset you may want to use `p = 1`,
     otherwise your index for largest replica will get so big that other replicas will be packed together too tight.
     Unless, again - that's exactly what you wanted.

## Partitioner

Partitioners are responsible for producing a distribution of shards to replicas.
Partitioner will use the Load Index of each shard, sourced from `.spec.loadIndexProviderRef`.
Result will be exported in `.status.replicas` as a list of Replica objects.

Individual algorithms can vary, but generally I am going into this with the following assumption -
we want to scale horizontally.
Which means in the ideal world we would not be needing this autoscaler, and we will just do one shard per one replica.
The problem is that - shards are not equal in load, and to be able to accommodate our biggest shard,
we need to give more resources to all replicas than they probably need.
With that in mind, we will use the "heaviest" shard to determine a maximum size of one replica,
so we can balance replicas among each other, and stack multiple shards together on less busier replicas.
That way, when we allocate resources to pods at STS level - it would make more sense.

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

Load Index and Partitioner will never be consistent, as workloads and user activity changes all the time.

Re-balancing shards is an expensive and disruptive operation, so we all want to minimize that.
`consistent-hashing` sharding implementation in the app controller, for example,
is trying to maintain status quo within +/-10%.

But - we don't want to just minimize re-balancing.
We want to minimize meaningless re-balancing but still embrace change when it's actually needed.

Evaluator will observe the Partitioner in `.spec.partitionProviderRef` over time,
and export only "stable" distribution plans as `.status.replicas`.

### "Most Wanted" Two-Phase Hysteresis

One idea was to use a Two-Phase Hysteresis, which is a well known algorithm, that would
produce the new partition repeatedly, and if it remains the same for X consecutive checks or amount of time - then apply it.
But that approach would result in a major flaw.
If status quo is already has fallen behind, but the new partitioning still can't get stable within the constraints,
then - we are stuck with the status quo.
One way to overcome this would be to reduce the threshold to apply - but that will cause too frequent re-balancing.

So, we will use a custom variation of Two-Phase Hysteresis, that I don't know if it exists, but I called it "Most Wanted".

We will produce and store intended distribution of shards over a configurable amount of time.
That will allow us to "learn" the most "normal" configuration over time and ignore transient spikes.
In the end - we will use a configuration that was desired the most amount of samples over that time window.
This will guarantee that we will use an up to date configuration once every predetermined time window.
It may not be the most efficient accordingly to the latest data, but it's reflecting well the most recent history,
without risking to fall behind too much.

There is an edge case that needs to be explained in a little more details.
The `MostWantedTwoPhaseHysteresisEvaluation` CRD only stores hashes of the recent history for repeated evaluation.
It cannot store full distribution plans as it would quickly get out of etcd object size limits.
Therefore, there could be a scenario when the most-wanted but old record is about to be erased,
while the new projected winner is not the current partitioner output.
Thus - the current projection is effectively unknown (only its hash and last seen time and total seen count is known).
When that happens - evaluator will enter a not-ready state.
Eventually, either another partitioning plan will surpass this currently unknown projection, or -
partitioner will give us another sample with the current projection details that evaluator will be able to remember.

## Scaler

Using input from `.spec.partitionProviderRef` - Scaler can ensure that intention becomes reality.
To do that, it would be updating `.spec.replicas` on the `.spec.shardManagerRef`, as well as apply necessary changes to the `.spec.replicaSetControllerRef`.

### Replica Set Default

Implementations may vary, but for initial "default" implementation I am going into this with following assumptions.

We're assuming Application Controller is ran as STS and in `legacy` sharding mode.
Thus, we are making sure none of their baked-in dynamic re-balancing aren't getting in our way.

What happens then upon creating a new cluster but without a shard assignment -
it will be assigned to __some__ replica and the process will eventually get to Scaler through all phases.

This default implementation will do the following:

- Change `.spec.replicas` on the STS/Deployment.
- Change `ARGOCD_CONTROLLER_REPLICAS` environment variable on the STS.
- Change `.spec.replicas` on the Shard Manager.

Order of operations will vary on `.spec.mode`.

#### Default

If `.spec.mode` is set to `default: {}`, which is default, the Scaler will first apply changes to the Shard Manager,
wait, then - to the replica controller, and do nothing else.
If `.spec.mode.default.rolloutRestart` is set to `true` - additional annotation value will be added to the
RS template, equivalent of what `kubectl` does on `rollout restart` command.
It will trigger all pods to recycle even if `ARGOCD_CONTROLLER_REPLICAS` did not change.

This mode is exactly what would happen if you manually adjust number of replicas yourself.
So, that's why it's default. However, if my assumptions are correct, it is not going to work.
Or, not going to work well?
See, I am suspecting that it may cause problems, because at some points in time,
there would be replicas with different values in `ARGOCD_CONTROLLER_REPLICAS`,
thus - possibly making two or more replicas to think that they own the same shard.
I don't really know how it works or what would happen.
You can check my comments for more details on kinds of issues that I personally faced here:
https://github.com/argoproj/argo-cd/issues/15464#issuecomment-2587501293.
Also, it may very well be only the case with `consistent-hashing` algorithm that I was trying to use at a time.
Input from community is welcome on this subject.
I'm not even sure `ARGOCD_CONTROLLER_REPLICAS` is even in use with `legacy` sharding mode at Application Controller.
So, maybe I was overthinking it all?
In the meantime I personally am going to adopt a more reliable (in my opinion) mechanism below that I call X-0-Y scaling.

#### X-0-Y

If `.spec.mode` is set to `x0y: {}`, the Scaler first will scale down RS owner to 0 and wait until it's ready.
Then, it will apply changes to the Shard Manager, and wait.
Lastly, it will apply `ARGOCD_CONTROLLER_REPLICAS` and `spec.replicas` to the RS owner, which would scale them back out.

This mechanism effectively guarantees that at no point in time there would be two or more replicas with different
value in `ARGOCD_CONTROLLER_REPLICAS` environment variable.
At the cost of a brief downtime, which honestly, likely no one will ever even notice.
