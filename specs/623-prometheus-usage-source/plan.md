# Implementation Plan: Prometheus Historical Cluster Usage

**Branch**: `623-prometheus-usage-source` | **Date**: 2026-10-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/623-prometheus-usage-source/spec.md`

## Summary

Add a Prometheus usage plugin and a historical branch on `finfocus cost cluster`.
With `--from`/`--to`, core asks that plugin for integrated `cpu_usage` and
`mem_usage`, prices each node with actual spend over the same window
(`CostResult.TotalCost`, state-based projection disabled), and reuses the
kubernetes allocator. With no window, the command stays the run-rate path
(`GetProjectedCostWithErrors`, `Monthly`). The plugin is its own module. The
registry entry waits until `prometheus-v0.1.0` exists.

## Technical Context

**Language/Version**: Go 1.27.1 (see `go.mod`)
**Primary Dependencies**: finfocus-spec v0.7.5 (historical stats mode, `UnitCoreHours` / `UnitGiBHours` already released), Cobra, gRPC pluginsdk, `github.com/prometheus/client_golang` query API (plugin module only), client-go (live node-identity fallback only), kind + kubectl (existing E2E), testify
**Storage**: N/A (no new persistent state)
**Testing**: `go test` with testify; golden PromQL strings; httptest fixtures for hour totals; engine fakes for the window pricer; kind E2E (`test/e2e`, tag `e2e_kind`) with a fixed-cost plugin and a remote-written Prometheus fixture
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI binary with a nested gRPC plugin module
**Performance Goals**: A handful of instant queries per cluster report (one per metric family), not one query per pod. The existing plugin RPC deadline bounds a slow store
**Constraints**: Plugin must not import `github.com/rshade/finfocus/internal` or `pkg`. No finfocus-spec change. Historical amounts are `TotalCost`, never `Monthly` and never the state-based hourly estimate. `$0` stays unpriced. Conservation stays in core at 1e-6. Bearer token never logged. Run-rate output unchanged
**Scale/Scope**: One new nested module, a window branch in `internal/engine/cluster.go`, `--from`/`--to` on the existing command, allocator spot-note exception, kind job extension. No registry.json edit

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Prometheus is a usage plugin. Core only
  passes the window through and prices with the existing actual-cost engine.
  Allocation stays in `plugins/kubernetes`.
- [x] **Test-Driven Development**: Query goldens, httptest hour fixtures, and
  engine window tests are written before the behavior they pin. Plugin
  coverage at least 80%. The new pricing branch at least 95%. No TUI change
  (tabwriter footer only).
- [x] **Cross-Platform Compatibility**: Pure Go. The kind job stays Linux CI.
  No platform-specific code.
- [x] **Documentation Integrity**: Update
  `docs/src/content/docs/guides/cluster-costs.md`, add
  `plugins/prometheus/README.md`, and add the historical gotcha to CLAUDE.md
  in the same change as the code. `make docs-lint` is part of verification.
- [x] **Protocol Stability**: Consumes finfocus-spec v0.7.5 fields that already
  exist (`GetStatsRequest.start`/`end`, `STATS_MODE_HISTORICAL`, usage metric
  constants). No proto edit.
- [x] **Implementation Completeness**: The windowed command, the plugin, and
  the kind proof are finished behavior. The registry entry is an explicit
  non-goal until `prometheus-v0.1.0` exists (same shape as the kubernetes
  registry gate), not a stub in the plugin.
- [x] **Persistence Model**: No new store. Prometheus is an external read.
- [x] **Quality Gates**: `make test`, `make lint` (including the new module
  and `check-plugin-boundaries`), and `make docs-lint` before completion.
- [x] **Multi-Repo Coordination**: No finfocus-spec or aws-public change. Kind
  does not call a cloud bill. The plugin module pins the same finfocus-spec
  version as the root module.

**Violations Requiring Justification**: None.

Post-design re-check: still pass. `SkipStateEstimate` defaults off, so
`cost actual` keeps today's state-based fallback. The historical branch is
the only caller that sets it.

## Project Structure

### Documentation (this feature)

```text
specs/623-prometheus-usage-source/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cost-cluster-window.md
│   └── prometheus-stats.md
└── tasks.md             # /speckit.tasks, not this command
```

### Source Code (repository root)

```text
plugins/prometheus/                 # nested module, usage source only
├── go.mod                          # finfocus-spec v0.7.5, client_golang, client-go
├── cmd/main.go
├── plugin.go                       # GetStats, Supports=false, USAGE_STATS only
├── config.go                       # FINFOCUS_PROMETHEUS_URL, bearer token
├── promql/                         # query strings + goldens
├── collect/                        # HTTP API, rows, gap warnings
├── identity/                       # kube-state-metrics labels; live API fallback
└── README.md
internal/engine/cluster.go          # window branch, GetWindowCost, TotalCost
internal/engine/types.go            # ActualCostRequest.SkipStateEstimate
internal/cli/cost_cluster.go        # --from / --to via ParseTimeRange
internal/cli/cost_cluster_render.go # historical footer
plugins/kubernetes/allocate/        # skip spot on-demand note when mode is historical
Makefile                            # test/lint/build/boundary for the new module
release-please-config.json          # package stanza + root exclude-path
.release-please-manifest.json       # plugins/prometheus initial 0.1.0
test/e2e/kind/                      # Prometheus install + remote-write fixture
test/e2e/cluster_kind_hist_test.go  # tag e2e_kind
docs/src/content/docs/guides/cluster-costs.md
```

**Structure Decision**: Same nested-module pattern as `plugins/kubernetes`
and `plugins/jev`. Core does not speak PromQL. The plugin does not allocate
and does not import the kubernetes module (label rules are copied; the
source of truth stays `plugins/kubernetes/usage/nodes.go`).

## Design Notes

### Core pipeline

`ClusterRequest` gains `From` and `To time.Time`. Both zero means run-rate.
Exactly one non-zero is an error before `GetStats`.

`ResourcePricer` gains `GetWindowCost(ctx, resources, from, to) ([]CostResult, error)`.
`*Engine.GetWindowCost` calls `GetActualCostWithOptions` with
`SkipStateEstimate: true`. `GetActualCost` (the public method `cost actual`
uses) does not set the flag.

`priceResources` is unchanged and still treats `Monthly <= 0` as unpriced.
The historical branch reads `TotalCost` only. `deriveActualCostWindow` stores
a monthly projection on `Monthly`; allocating that field would bill a
projected month. Missing result, `Error != nil`, or `TotalCost <= 0` is
unpriced (`priced=false`, cost 0, a note). Every node unpriced stays fatal.

Mode check after `GetStats`:

- No window and mode is not run-rate: keep `ErrHistoricalUnsupported`.
- Window set and mode is not historical: new sentinel, naming both modes.
- Match: allocate. `AllocateRequest.Mode` is already copied from the stats
  response.

`GetStatsRequest.start` and `.end` are set only for a window
(`timestamppb.New`). A stats warning with prefix `incomplete:` sets
`ClusterResult.Incomplete`. Other warnings stay warnings. There is no proto
field for this; the prefix is the signal.

`ClusterResult` gains `Period`. Run-rate sets `monthly`. Historical sets
`engine.FormatPeriod(from, to)`. The CLI stops hardcoding `"monthly"` and
stops printing `730 h` on a historical table. JSON key `priced[].monthly`
stays, and in historical mode its value is the window `TotalCost`. See
[contracts/cost-cluster-window.md](contracts/cost-cluster-window.md).

`--from` and `--to` use `ParseTimeRange` / `ParseTime` (date-only midnight
UTC, `--to` defaults to now, end must be after start, future and max-past
rejected, range within `ValidateDateRange`). Invalid flags fail before plugin
load. Plugin selection is unchanged.

### Allocator

`allocateNode` adds `spot node priced on-demand` whenever `capacity_type=spot`.
That note is a run-rate substitute for a missing spot price. When
`AllocateRequest.mode` is `STATS_MODE_HISTORICAL`, skip the note. The cost
value is already the actual spend core put on `PricedResource.cost`. Run-rate
tests stay as they are.

Node-split weights are per core-hour. CPU and memory amounts are both
integrated over the same window and the same 60s step, so the hours cancel
in the ratio. Workload share stays `max(request, usage) / allocatable`.
Historical rows omit requests, so the charge is usage.

### Prometheus plugin

Declare only `PLUGIN_CAPABILITY_USAGE_STATS` via `WithCapabilities`.
`pluginsdk.Serve` uses that list instead of `inferCapabilities`, which would
also advertise pricing. `Supports` returns `Supported: false` for every
resource. Embed `BasePlugin` so the required cost methods exist but are not
advertised. Reject a stats request that is missing either bound or whose end
is not after start, with `InvalidArgument` and the text that this source is
historical only. Do not import `plugins/kubernetes`.

Address: `FINFOCUS_PROMETHEUS_URL`. When unset and
`KUBERNETES_SERVICE_HOST` is set, use
`http://prometheus-operated.monitoring.svc:9090` (the Prometheus Operator
service). Any other chart sets the variable. A bearer token comes from
`FINFOCUS_PROMETHEUS_BEARER_TOKEN` and is absent from errors and logs.

Queries and row assembly are specified in
[contracts/prometheus-stats.md](contracts/prometheus-stats.md). Goldens pin
the query strings. httptest fixtures pin the hours, including counter reset,
partial lifetime, a hole, namespace filter, label selector, and an empty
window.

### Kind proof

`make test-e2e-kind` keeps the current run-rate test. After it, on the same
cluster, install Prometheus with remote-write enabled, write a fixed series
fixture from the e2e test, and run `cost cluster --from/--to` with
`FINFOCUS_HOME` containing only the kubernetes allocator, the prometheus
plugin, and a fixed-cost actual plugin (`test/e2e/kind/fixedcost`,
`TotalCost` from the environment). aws-public is not on that home: a `$0`
actual answer would mark the node unpriced. Assert mode, period, the
fixture's resource-hours, and conservation within 1e-6. No cloud credentials.

### Release wiring, not the registry

Add a `plugins/prometheus` release-please package (component `prometheus`,
tag separator `-`, initial version `0.1.0`) and exclude that path from the
root package, matching kubernetes and jev. Do not add `registry.json`. That
entry is a follow-up once tag `prometheus-v0.1.0` exists.

## Complexity Tracking

No constitution violations to justify.
