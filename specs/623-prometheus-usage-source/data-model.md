# Data Model: Prometheus Historical Cluster Usage

**Date**: 2026-10-04
**Spec**: [spec.md](spec.md)

Wire types stay finfocus-spec v0.7.5. This page is the fields this feature
reads and writes.

## Window

| Field | Where | Rule |
| --- | --- | --- |
| `From`, `To` | `engine.ClusterRequest` | Both zero: run-rate. Both set, `To` after `From`: historical. One set: error before `GetStats` |
| `start`, `end` | `GetStatsRequest` | Set only for a historical request, as `timestamppb` of `From` and `To` |
| `mode` | `GetStatsResponse` | `STATS_MODE_HISTORICAL` when the window was set. Anything else with a window is an error |
| `Period` | `engine.ClusterResult` | `monthly` for run-rate. `FormatWindow(From, To)` for historical (bounds and length) |

CLI parsing is `ParseTimeRange`: date-only is midnight UTC, `--to` defaults
to now, future and older-than-max-past are rejected, and the span must pass
`ValidateDateRange`.

## Historical usage row

Same subject keys as run-rate (`cluster`, `namespace`, `controller_kind`,
`controller`, `pod`, `node`, `label.<key>`, `kind`).

| Kind | Metrics | Unit | Amount |
| --- | --- | --- | --- |
| `workload` | `cpu_usage`, `mem_usage` only | `core-hours`, `GiB-hours` | Integral over `[From, To]`. No request metrics |
| `node` | `cpu_allocatable`, `mem_allocatable` | `core-hours`, `GiB-hours` | Integral of allocatable over the same window and the same 60s step |

A `(subject, metric)` pair appears once. Amounts are finite and non-negative.
The allocator already charges `max(request, usage)`; a missing request is 0,
so the charge is usage. CPU and memory share the window and the step, so
`node_split` weights stay a ratio.

## Priceable node

Same descriptor shape as run-rate: `id` equals the node subject, `tags.kind=node`,
`tags.capacity_type` is `spot` or `on-demand`, provider and type come from the
copied `NodeDescriptor` rules.

Identity order:

1. `kube_node_labels` and `kube_node_info` samples inside the window.
2. Live node object, only when recorded labels are missing and kubeconfig works.
3. Otherwise no priceable entry, and a warning prefixed `incomplete:`.

Fargate pod priceables are not produced. An EKS control-plane priceable is
produced only when the live client is connected and the API host matches the
existing EKS regex.

## Windowed price

`Engine.GetWindowCost` returns `[]CostResult` from `GetActualCostWithOptions`
with `SkipStateEstimate: true`.

| `CostResult` field | Historical use |
| --- | --- |
| `TotalCost` | The only amount copied onto `PricedResource.cost` and `PricedSummary.Monthly` |
| `Monthly` | Ignored. It is a projection of `TotalCost` |
| `Error` | Unpriced |
| `Currency` | Existing mixed-currency check |

`TotalCost <= 0`, a missing result, or `Error != nil` means `priced=false`,
cost 0, and a note. All nodes unpriced is fatal. `cost actual` does not set
`SkipStateEstimate`, so its state-based fallback stays.

## Incomplete signal

`GetStatsResponse` has warnings and no incomplete flag. A warning whose text
starts with `incomplete:` sets `ClusterResult.Incomplete`. Unknown-metric
warnings and the control-plane omission warning do not use that prefix.

A hole of more than one 60s step inside a series' own first sample and last
sample is `incomplete:`. A pod that starts after `From` is not, because the
hole test uses that series' own span.

## Cluster subject

| Store | Request `scope` | Result |
| --- | --- | --- |
| No `cluster` label | anything | One cluster. Subject `cluster` is `scope`, which may be empty |
| One `cluster` label value | empty or equal to that value | That value |
| One value | a different scope | `InvalidArgument` |
| Several values | empty or not one of them | `InvalidArgument`, message lists the values |
| Several values | one of them | Filter every query to that value |

## Checked hour example

One case is a PromQL string golden plus, where hours are claimed, an httptest
instant-vector fixture and the usage rows it must produce. The required cases
are counter reset, partial lifetime, a hole (incomplete), namespace filter,
label selector, and an empty window.

## What does not change

- Allocation policy document and digest.
- Run-rate row metrics and units (`core`, `GiB`).
- `ErrHistoricalUnsupported` when the user did not ask for a window and the
  source returned historical mode.
- JSON keys of `cost cluster` other than the values of `mode` and `period`.
- `registry.json`.
