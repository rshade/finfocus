# Research: Prometheus Historical Cluster Usage

**Date**: 2026-10-04
**Spec**: [spec.md](spec.md)

Each item is a choice the spec left to planning. None remain open.

## Window price field

- **Decision**: Historical allocation uses `CostResult.TotalCost`. Run-rate
  allocation keeps using `Monthly`.
- **Rationale**: `getActualCostFromPlugin` stores the plugin's period sum on
  `TotalCost`. `deriveActualCostWindow` then writes a monthly projection onto
  `Monthly` (`total * avgDaysPerMonth / days`, or hourly times 730). Pricing
  the cluster from `Monthly` would bill a projected month, which the spec
  forbids. `priceResources` today treats `Monthly <= 0` as unpriced; that
  rule stays on the run-rate path only.
- **Alternatives considered**: Reuse `priceResources` unchanged. Rejected
  because it reads the projection. Rename `priced[].monthly` in JSON.
  Rejected because the spec keeps the `cost cluster` document shape; the
  contract states that the key holds window spend when `mode` is
  `historical`.

## State-based estimate

- **Decision**: `ActualCostRequest` gains `SkipStateEstimate bool`, default
  false. `Engine.GetWindowCost` sets it true. `getActualCostForResource`
  skips both `tryStateBasedEstimation` calls when the flag is set.
- **Rationale**: A `$0` or empty actual answer currently falls through to
  hourly rate times uptime (`tryStateBasedEstimation` →
  `getHourlyRateForResource`). That is a projected price. Node descriptors
  usually lack a created timestamp, so the fallback often no-ops, but the
  spec requires that it cannot run. `cost actual` must keep the fallback.
- **Alternatives considered**: Call `GetActualCost` and discard results whose
  notes mention estimation. Rejected because the note text is not a contract.
  Strip created timestamps in the plugin only. Rejected because core would
  still estimate if a tag ever carried one.

## Spot note

- **Decision**: When `AllocateRequest.mode` is `STATS_MODE_HISTORICAL`,
  `allocateNode` does not set `spot node priced on-demand`. The cost number
  is unchanged.
- **Rationale**: The note is the run-rate policy for a missing spot price
  (`spot_nodes: on-demand-with-note`). Historical cost is already the bill
  core placed on `PricedResource.cost`. Core already copies the stats mode
  onto the allocate request.
- **Alternatives considered**: Strip the note in core after allocate.
  Rejected because the allocator owns the note. Change the policy default.
  Rejected because run-rate would lose the note.

## Hour integral

- **Decision**: Instant queries evaluated at the window end. Lookback is the
  window length in whole seconds (`<dur>s`). Subquery step is 60s. CPU uses
  `sum_over_time(rate(container_cpu_usage_seconds_total[5m])[<dur>:60s]) * 60 / 3600`
  (core-hours). Memory uses
  `sum_over_time(container_memory_working_set_bytes[<dur>:60s]) * 60 / 1024^3 / 3600`
  (GiB-hours). Node allocatable uses the same `sum_over_time` form on
  `kube_node_status_allocatable` (`resource="cpu"` in cores,
  `resource="memory"` in bytes). Pause containers (`container=""` and
  `container="POD"`) are excluded. A hole inside a series' own first-to-last
  span of more than one step produces an `incomplete:` warning. A series that
  starts late because the pod started late does not.
- **Rationale**: `increase()` over the whole window extrapolates across the
  range and can invent time the spec says not to invent. `sum_over_time` of
  a fixed step counts only samples that exist. `rate` handles counter resets.
  The same step on CPU and memory keeps the allocator's core-hour weights
  in ratio. kube-state-metrics is the source that still has a replaced node's
  labels after the node is gone.
- **Alternatives considered**: `increase()` divided by 3600. Rejected for
  extrapolation. Average gauge times the full window. Rejected because a
  deleted node would be billed for hours it did not exist. Live cAdvisor
  scrape only. Rejected because it cannot see a replaced node.

## Node identity

- **Decision**: Read `last_over_time(kube_node_labels[<dur>])` and
  `kube_node_info` (for `provider_id`). Map Prometheus label names with the
  kube-state-metrics sanitizer (non-alphanumeric to `_`, prefix `label_`)
  back onto the same Kubernetes label keys `NodeDescriptor` reads
  (`node.kubernetes.io/instance-type`, `topology.kubernetes.io/region`,
  `finfocus.dev/provider`, capacity-type labels, and the beta aliases).
  Build the priceable descriptor with a copy of those rules. If recorded
  labels are missing and the node still exists, list it through client-go
  with kubeconfig context equal to `scope`. If neither works, omit the
  priceable and warn `incomplete:`. Do not import `plugins/kubernetes`.
- **Rationale**: Recorded labels survive node replacement. The live API does
  not. Importing the kubernetes module would tie two release tags together
  and pull its projected-cost code into this plugin.
- **Alternatives considered**: Live API only. Rejected by the spec for nodes
  that are gone. A shared Go module for `NodeDescriptor`. Rejected as extra
  scope; the copy is one function and the plan names the file it must match.

## Cluster selection

- **Decision**: If series carry a `cluster` label and more than one value is
  present, `scope` must name one of them or `GetStats` returns
  `InvalidArgument` listing the values. One value is that cluster. No label
  means the store is one cluster and the subject `cluster` is `scope` (empty
  if the request did not set it).
- **Rationale**: Matches the spec rule that clusters are never summed
  together. Prometheus external labels and kube-state-metrics
  `--metric-labels` are both expressed as a `cluster` label.
- **Alternatives considered**: Always require `scope`. Rejected because a
  dedicated store has nothing to disambiguate. Match kube context against
  the Prometheus URL. Rejected because those strings are not the same.

## Control plane and Fargate

- **Decision**: Emit an EKS control-plane priceable only when the live API
  fallback is connected and the API server host matches the existing EKS host
  regex. Otherwise one warning, not `incomplete:`. Do not emit Fargate pod
  priceables. Pods whose node name starts with `fargate-` already get the
  allocator's existing note.
- **Rationale**: The spec does not add Fargate prices or a new control-plane
  discovery path. The run-rate regex and the `fargate-` note already exist.
- **Alternatives considered**: Query CloudWatch or a control-plane metric.
  Rejected as a new pricing source.

## Plugin address and capabilities

- **Decision**: URL from `FINFOCUS_PROMETHEUS_URL`. In-cluster default, only
  when `KUBERNETES_SERVICE_HOST` is set, is
  `http://prometheus-operated.monitoring.svc:9090`. Token from
  `FINFOCUS_PROMETHEUS_BEARER_TOKEN`, never logged. Capability list is
  exactly `PLUGIN_CAPABILITY_USAGE_STATS`. `Supports` is always false.
- **Rationale**: `inferCapabilities` always adds projected, actual, pricing,
  and estimate. `WithCapabilities` replaces that list (`sdk.go` uses
  `info.Capabilities` when non-empty). Advertising actual cost would make
  this usage plugin compete with aws-public. The Operator service name is
  the only stable in-cluster default; kube-prometheus-stack's Service name
  is chart-specific and is set with the variable.
- **Alternatives considered**: Discover Services named `prometheus` in any
  namespace. Rejected because that requires cluster-wide RBAC for a guess.
  Declare actual cost as well and return unimplemented. Rejected because
  hosts would still route to it.

## Kind fixture

- **Decision**: The deterministic hours are the plugin httptest fixtures.
  The kind job additionally starts Prometheus with remote-write enabled,
  writes one fixed series fixture, and runs the real CLI against a
  `FINFOCUS_HOME` that contains the kubernetes allocator, the prometheus
  plugin, and `test/e2e/kind/fixedcost` (returns `TotalCost` from the
  environment). aws-public is not installed there.
- **Rationale**: Live scrapes over a few minutes do not produce a stable
  resource-hour total. aws-public actual cost needs a cloud bill and often
  returns `$0`, which this path treats as unpriced. The run-rate kind test
  keeps aws-public and is unchanged.
- **Alternatives considered**: Wait for real scrapes and assert only
  conservation. Rejected because the spec asks for the fixture's
  resource-hours. Point the kind test at httptest. Rejected because the spec
  asks for Prometheus in the kind cluster.

## Registry

- **Decision**: Wire release-please for `plugins/prometheus` (tag
  `prometheus-v0.1.0` once released). Do not edit `registry.json` in this
  feature.
- **Rationale**: The spec gates the install entry on a real release tag.
  Release-please config does not publish that tag by itself. Kubernetes
  followed the same split.
- **Alternatives considered**: Add the registry entry immediately. Rejected
  because install would look for a tag that does not exist.
