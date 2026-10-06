# Contract: Prometheus `GetStats`

The plugin serves `UsageSourceService.GetStats` only. It does not implement
allocation. `GetPluginInfo` capabilities are exactly
`PLUGIN_CAPABILITY_USAGE_STATS`. `Supports` returns `Supported: false`.

## Request

| Request | Response |
| --- | --- |
| Both `start` and `end` unset, or only one set, or `end` not after `start` | `InvalidArgument`. Message says this source is historical only |
| Both set, `end` after `start` | `STATS_MODE_HISTORICAL` |

`scope` is the cluster name used to filter a `cluster` label, and the
kubeconfig context for the live-API fallback. Selector key `namespace` is a
namespace matcher. Every other selector key is an exact pod label match.
Unknown names in `metrics` are ignored with a warning that does not start
with `incomplete:`. This source's default metrics are `cpu_usage`,
`mem_usage`, `cpu_allocatable`, and `mem_allocatable`.

## Queries

Evaluation time is `end`. `<dur>` is `end-start` in whole seconds, rendered
as `<dur>s`. Step is `60s`. Matchers for namespace and, when a single cluster
was selected, `cluster`, are added inside the metric selector. Goldens in
`plugins/prometheus/promql` pin the full strings, including matchers.

CPU, summed by `namespace`, `pod`, `node`, in core-hours:

```text
sum by (namespace, pod, node) (
  sum_over_time(
    rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])
    [<dur>:60s]
  )
) * 60 / 3600
```

Memory, same grouping, in GiB-hours. `1073741824` is 1024 cubed:

```text
sum by (namespace, pod, node) (
  sum_over_time(
    container_memory_working_set_bytes{container!="",container!="POD"}[<dur>:60s]
  )
) * 60 / 1073741824 / 3600
```

Node CPU and memory allocatable, summed by `node`, same `sum_over_time`
shape, on `kube_node_status_allocatable{resource="cpu"}` (cores) and
`{resource="memory"}` (bytes, divided like the memory query).

Owner, pod info, pod labels, and node labels are
`last_over_time(<metric>[<dur>])` for `kube_pod_owner`,
`kube_replicaset_owner`, `kube_job_owner`, `kube_pod_info`,
`kube_pod_labels`, `kube_node_labels`, and `kube_node_info`. Joins happen in
Go, not in one PromQL expression. ReplicaSet joins to Deployment and Job
joins to CronJob the same way the run-rate collector walks owners. A missing
owner leaves `controller_kind` and `controller` empty. That warning does not
use the `incomplete:` prefix.

## Hours

- Counter reset: `rate` over 5m plus `sum_over_time` sums both sides. The
  fixture's core-hours equal the sum of the two segments, not the difference
  of the last and first counter values.
- Partial lifetime: samples that do not exist contribute nothing. A pod with
  samples for half the window produces half the core-hours of the same rate
  over the full window.
- Hole: if `count_over_time` of a series is more than one step short of that
  series' own first-to-last span, add `incomplete: <subject>: gap in <metric>`.
  A series whose first sample is simply after `start` is not a hole.
  The count is of 60s subquery steps, and each step looks back 5m, so a hole
  shorter than the lookback is not detected and the last value covers it
  (standard Prometheus semantics). A failed scrape or a vanished series
  writes a staleness marker, which ends the lookback, so those holes are
  still detected. The undetected case is mainly Prometheus itself stopped
  for under five minutes.
- Retention shorter than the window: if the earliest
  `kube_node_status_allocatable{resource="cpu"}` sample in the window is more
  than one step after `start`, add `incomplete: stored data starts at <time>,
  after the window start <start>; Prometheus retention may be shorter than
  the window`. No node series means no warning (an empty store stays valid).
- Empty window with the store up and no series: success, no workload rows.
  Node rows still appear when allocatable series exist.
- Store down, or URL unset outside the cluster: error before a successful
  empty response. The error names `FINFOCUS_PROMETHEUS_URL` when the URL is
  the problem. It never contains the bearer token.

## Node labels

Prometheus label `label_<sanitized>` maps back to the Kubernetes label.
Sanitized means every character outside `[A-Za-z0-9]` became `_`. The keys
that must round-trip are the ones `plugins/kubernetes/usage/nodes.go` reads:

- `node.kubernetes.io/instance-type` and `beta.kubernetes.io/instance-type`
- `topology.kubernetes.io/region` and `failure-domain.beta.kubernetes.io/region`
- `finfocus.dev/provider`
- `eks.amazonaws.com/capacityType` and `karpenter.sh/capacity-type`
- `eks.amazonaws.com/compute-type`

`provider_id` comes from `kube_node_info`. The descriptor rules (provider
from the label, else the providerID scheme; resource type token; spot versus
on-demand; `id` equal to the node name; `tags.cluster` and `tags.node` when
the cluster subject is non-empty) match `NodeDescriptor`. A node with no
recorded identity tries the live API once. Failure to identify is
`incomplete: node <name>: cannot determine provider, instance type, or region`.

## Address

| Process | URL |
| --- | --- |
| `FINFOCUS_PROMETHEUS_URL` set | That URL |
| Unset, and `KUBERNETES_SERVICE_HOST` set | `http://prometheus-operated.monitoring.svc:9090` |
| Unset, and not in-cluster | Error naming `FINFOCUS_PROMETHEUS_URL` |

`FINFOCUS_PROMETHEUS_BEARER_TOKEN`, when set, is sent as
`Authorization: Bearer`. It is not written to logs, errors, or fixtures.
