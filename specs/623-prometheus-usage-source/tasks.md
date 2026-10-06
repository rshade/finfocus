# Tasks: Prometheus Historical Cluster Usage

**Input**: Design documents from `specs/623-prometheus-usage-source/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: TDD (Constitution Principle II). Each test task comes before its
implementation task and must fail first. Testify `require` and `assert` only.
Unit tests are colocated. Call `t.Parallel()` unless the test uses `t.Setenv`,
`t.Chdir`, or execs a binary it just wrote (CLAUDE.md "Parallel Tests").

**Completeness**: No stubs or TODOs (Principle VI). The registry entry is an
explicit non-goal until tag `prometheus-v0.1.0` exists, not a placeholder in
the plugin.

**Documentation**: The cluster-cost guide, the plugin README, and the
CLAUDE.md cluster gotcha land with the code (Principle IV).

**Do not** `git commit`. The user commits.

**Story order**: Phases follow spec priority. Executable order is in
Dependencies: the Prometheus module (US2) can proceed beside the engine
pricing edges (US3) after Foundational. The kind proof (US5) is last.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an
  incomplete task)
- **[Story]**: User story label. Setup, Foundational, and Polish have none
- Every task names a file path

---

## Phase 1: Setup

**Purpose**: Nested module and release wiring, with no usage behavior yet.

- [X] T001 Confirm the branch is `623-prometheus-usage-source` and that
  `go.mod` requires `github.com/rshade/finfocus-spec` v0.7.5. Confirm
  `pluginsdk` already exports `STATS_MODE_HISTORICAL`, `UnitCoreHours`,
  `UnitGiBHours`, and `GetStatsRequest` start/end. Confirm
  `internal/cli/cost_actual.go` `ParseTimeRange` and `internal/engine`
  `FormatPeriod` exist, and `TestRunClusterAllocation_Errors` /
  `historical_mode_rejected` in `internal/engine/cluster_test.go` still
  expects `ErrHistoricalUnsupported`. Leave `internal/registry/registry.json`
  unchanged
- [X] T002 Create `plugins/prometheus/go.mod` (`module
  github.com/rshade/finfocus/plugins/prometheus`, `go 1.27.1`,
  `github.com/rshade/finfocus-spec` v0.7.5). From that module run `go get`
  `github.com/prometheus/client_golang@latest`, `k8s.io/client-go@v0.37.1`,
  and the matching `k8s.io/api` and `k8s.io/apimachinery` versions, then
  `go mod tidy`. Record the resolved client_golang version in `go.sum`. Do
  not import `github.com/rshade/finfocus/internal` or `pkg`, and do not
  import `plugins/kubernetes`
- [X] T003 [P] Add `test-prometheus`, `lint-prometheus`, `build-prometheus`,
  and `install-prometheus` to `Makefile`, mirroring the kubernetes targets
  (`go -C`, binary `bin/finfocus-plugin-prometheus`, version from
  `.release-please-manifest.json`). Add `plugins/prometheus` to the
  `check-plugin-boundaries` loop, and to `test`, `lint`, and `build-all`.
  Update the `make help` lines that list those targets
- [X] T004 [P] In `release-please-config.json`, copy the
  `plugins/kubernetes` package onto `plugins/prometheus` (component
  `prometheus`, `include-component-in-tag`, `tag-separator` `-`,
  `initial-version` `0.1.0`) and add `plugins/prometheus` to the root
  package `exclude-paths`. Set `"plugins/prometheus": "0.1.0"` in
  `.release-please-manifest.json`. Add `plugins/prometheus/CHANGELOG.md`
  containing only the heading `# Changelog`. Leave
  `internal/registry/registry.json` unchanged

**Checkpoint**: `go -C plugins/prometheus test ./...` runs (no tests yet)
and `make check-plugin-boundaries` includes the new module.

---

## Phase 2: Foundational (blocks every story)

**Purpose**: Window pricing entry point and Prometheus address resolution.
`cost actual` behavior stays as it is.

- [X] T005 [P] Write failing tests in `internal/engine/window_cost_test.go`.
  `ActualCostRequest.SkipStateEstimate` defaults false, and a zero
  `TotalCost` with the flag false still takes the existing state-based
  estimate path in `getActualCostForResource`. With the flag true, that
  path is not called and a plugin `TotalCost` of 0 stays 0. `GetWindowCost`
  calls actual cost with `From`, `To`, and `SkipStateEstimate` true, and
  the returned `CostResult.TotalCost` is the plugin period total. Use a
  fake actual-cost plugin. Do not assert on `Monthly` as the allocated
  amount
- [X] T006 Add `SkipStateEstimate bool` to `ActualCostRequest` in
  `internal/engine/types.go`. In `getActualCostForResource` in
  `internal/engine/engine.go`, skip `tryStateBasedEstimation` when the flag
  is true. `GetActualCost` leaves the flag false. Add `GetWindowCost` to
  the `ResourcePricer` interface in `internal/engine/cluster.go` and
  implement `(*Engine).GetWindowCost` so it calls
  `GetActualCostWithOptions` with the flag set. Add `GetWindowCost` to
  `fakePricer` in `internal/engine/cluster_test.go` so the package
  compiles. The run-rate fake may return an error from that method until
  T012
- [X] T007 [P] Write failing tests in `plugins/prometheus/config_test.go`
  for `LoadConfig`. A set `FINFOCUS_PROMETHEUS_URL` wins. URL unset and
  `KUBERNETES_SERVICE_HOST` set uses
  `http://prometheus-operated.monitoring.svc:9090`. Both unset returns an
  error that names `FINFOCUS_PROMETHEUS_URL`. `FINFOCUS_PROMETHEUS_BEARER_TOKEN`
  is returned to the caller and is absent from the error string. These
  tests use `t.Setenv` and are not parallel
- [X] T008 Implement `LoadConfig` in `plugins/prometheus/config.go` per
  `contracts/prometheus-stats.md` Address. The token is not written by the
  logger

**Checkpoint**: `go test ./internal/engine/ -run 'TestWindowCost|TestSkipStateEstimate'`
and `go -C plugins/prometheus test ./...` pass. `cost actual` tests still
pass.

---

## Phase 3: User Story 1 - Ask what a namespace cost last week (Priority: P1) MVP

**Goal**: `finfocus cost cluster --from/--to` validates the window before
plugins load, asks stats for that window, prices with `GetWindowCost`, and
prints mode `historical` and the window period. No window stays run-rate.

**Independent Test**: A fake usage source returns historical rows for a
fixed window and a fake pricer returns a known `TotalCost`. Group totals
match that total within 1e-6. Mode is `historical`. Period is
`FormatPeriod`, not `monthly`. The table footer does not cite 730 hours.
A bad window exits 1 before any plugin call. A no-window run still expects
`ErrHistoricalUnsupported` when the fake returns historical mode.

### Tests for User Story 1

- [X] T009 [P] [US1] Add failing tests to `internal/engine/cluster_test.go`.
  Both `ClusterRequest.From` and `To` set: `GetStats` receives those
  `timestamppb` bounds, `GetWindowCost` is called with the same bounds,
  `PricedSummary.Monthly` equals `TotalCost`, `ClusterResult.Mode` is
  `historical`, and `Period` is `FormatPeriod(From, To)`. A stats warning
  prefixed `incomplete:` sets `Incomplete`. A warning without that prefix
  does not. No window still prices with `GetProjectedCostWithErrors` and
  `Monthly`, and `historical_mode_rejected` still expects
  `ErrHistoricalUnsupported`. Namespace scope still drops idle and cluster
  rows
- [X] T010 [P] [US1] Add failing tests to `internal/cli/cost_cluster_test.go`.
  `--from` / `--to` accept `2006-01-02` and RFC3339 via `ParseTimeRange`.
  `--to` defaults to now when only `--from` is set. `--from` is required
  when `--to` is set. A same-day date-only pair, a reversed range, a future
  time, and a time older than `maxPastYears` fail before plugin load
  (exit 1, no plugin client opened). No window leaves `From` and `To`
  zero. `--output` and `--group-by` are still validated before plugins
- [X] T011 [P] [US1] Add failing tests to
  `internal/cli/cost_cluster_render_test.go`. Historical table footer is
  `Mode: historical (<period>)` and does not contain `monthly` or `730`.
  Run-rate footer still cites monthly and 730 hours. JSON `mode` and
  `period` follow `ClusterResult`. JSON key `priced[].monthly` remains,
  and for historical mode the number is the window total

### Implementation for User Story 1

- [X] T012 [US1] In `internal/engine/cluster.go`, add `From` and `To` to
  `ClusterRequest`, `Period` to `ClusterResult`, and `ModeHistorical`.
  Exactly one of `From` or `To` non-zero returns an error before
  `GetStats`. Both set and `To.After(From)`: set `GetStatsRequest` start
  and end, require `STATS_MODE_HISTORICAL`, price with `GetWindowCost`,
  and copy `TotalCost` onto the priced summary (JSON `monthly`). Set
  `Period` with `FormatPeriod`. Treat `TotalCost <= 0`, a missing result,
  or `Error != nil` as unpriced. A warning prefixed `incomplete:` or any
  unpriced priceable sets `Incomplete`. Both zero: keep today's run-rate
  path, including `ErrHistoricalUnsupported`
- [X] T013 [US1] Add `--from` and `--to` to `internal/cli/cost_cluster.go`.
  Parse them with `ParseTimeRange` before plugin load and pass `From` and
  `To` on `ClusterRequest`. Plugin selection stays `selectCapablePlugin`.
  Help text names the two date forms
- [X] T014 [P] [US1] In `internal/cli/cost_cluster_render.go`, stop
  hardcoding period `monthly`. Read `ClusterResult.Period`. Historical
  footer prints the period and does not print `730 h`. JSON and NDJSON
  keep their field names. `priced[].monthly` stays the summary field

**Checkpoint**: Fake-plugin `go test ./internal/engine/ ./internal/cli/ -run 'Cluster|CostCluster'`
passes. A no-window report is unchanged. Prometheus is not required yet.

---

## Phase 4: User Story 2 - Measure consumption, not requests (Priority: P1)

**Goal**: The Prometheus plugin answers a valid window with historical
`cpu_usage` and `mem_usage` in core-hours and GiB-hours, plus allocatable
in the same units. Request metrics are absent.

**Independent Test**: httptest fixtures for counter reset, partial
lifetime, a hole, a namespace filter, a label selector, and an empty
window produce the hours named in `contracts/prometheus-stats.md`. A
2-core request with 0.5 core usage charges the usage amount because no
request metric is sent. Query goldens match the PromQL strings in that
contract.

### Tests for User Story 2

- [X] T015 [P] [US2] Golden tests in `plugins/prometheus/promql/query_test.go`
  pin the CPU, memory, allocatable, and `last_over_time` owner/label
  queries from `contracts/prometheus-stats.md`, including `<dur>s`, step
  `60s`, and namespace and cluster matchers when those are selected.
  Evaluation time is the window end
- [X] T016 [P] [US2] httptest tests in `plugins/prometheus/collect/hours_test.go`
  for the six checked examples: counter reset sums both sides; partial
  lifetime counts only samples that exist; a hole of more than one 60s
  step inside a series' own first-to-last span adds a warning prefixed
  `incomplete:`; a series that starts after `From` is not a hole;
  namespace matcher limits rows; a non-namespace selector is applied in
  Go against pod labels; an empty store returns no workload rows and no
  error. Workload rows have `cpu_usage` and `mem_usage` only, units
  `UnitCoreHours` and `UnitGiBHours`. Node rows use allocatable in those
  same units. Subject keys match the run-rate set
- [X] T017 [P] [US2] Tests in `plugins/prometheus/plugin_test.go`: missing
  either bound, or end not after start, returns `InvalidArgument` and the
  message says historical only. Both bounds in order return
  `STATS_MODE_HISTORICAL`. `Info` capabilities are exactly
  `PLUGIN_CAPABILITY_USAGE_STATS`. `Supports` is `Supported: false` for a
  pricing descriptor. An unknown name in `metrics` adds a warning that
  does not start with `incomplete:`. A down server and a missing URL
  outside the cluster return an error, not an empty success. The bearer
  token never appears in that error. Plugin logs in these tests go to
  stderr

### Implementation for User Story 2

- [X] T018 [US2] Implement query builders in `plugins/prometheus/promql/query.go`
  for the contract strings. Use instant query at `end`. Do not use
  `increase()` over the whole range
- [X] T019 [US2] Implement row assembly in `plugins/prometheus/collect/collect.go`.
  Join owners in Go (ReplicaSet to Deployment, Job to CronJob) the way
  `plugins/kubernetes/usage` walks owners. A missing owner leaves
  controller fields empty and warns without the `incomplete:` prefix.
  HTTP calls use the request context. Honor namespace on the query and
  other selectors against `kube_pod_labels`
- [X] T020 [US2] Implement `GetStats` in `plugins/prometheus/plugin.go`.
  Embed `pluginsdk.BasePlugin`. Construct with `WithCapabilities` of only
  `PLUGIN_CAPABILITY_USAGE_STATS`. `Supports` returns false for every
  resource. Reject a bad window before any query. On success set
  `STATS_MODE_HISTORICAL`
- [X] T021 [US2] Serve the plugin from `plugins/prometheus/cmd/main.go`.
  Logs go to stderr only. Load config with `os.Getenv`. Add
  `plugins/prometheus/plugin.manifest.json` in the same shape as
  `plugins/kubernetes/plugin.manifest.json` with name `prometheus` and
  binary `finfocus-plugin-prometheus`

**Checkpoint**: `go -C plugins/prometheus test ./...` passes. The six hour
examples fail if the reported hours change.

---

## Phase 5: User Story 3 - Price the window from actual spend (Priority: P1)

**Goal**: Mode mismatches fail before allocation. Missing, error, and `$0`
actual prices stay unpriced. A historical spot node does not get the
on-demand note. Run-rate pricing is unchanged.

**Independent Test**: A windowed run whose stats mode is run-rate fails
and names both modes. `TotalCost <= 0` is unpriced and `GetProjectedCost`
is not called. Every node unpriced is fatal. Historical allocate output
for a spot node has no `spot node priced on-demand` note. The existing
run-rate allocate test still expects that note.

### Tests for User Story 3

- [X] T022 [P] [US3] Add failing cases to `internal/engine/cluster_test.go`.
  Exactly one of `From` or `To` errors before `GetStats`. A window plus a
  run-rate stats response returns an error that names both modes
  (`ErrStatsModeMismatch`). A window plus historical stats with a plugin
  error, a missing result, or `TotalCost <= 0` marks that node unpriced,
  sets `Incomplete`, and does not call `GetProjectedCostWithErrors`. All
  nodes unpriced is fatal. Conservation failure is still
  `ErrConservation`. Mixed currencies still fail
- [X] T023 [P] [US3] Add a failing test in
  `plugins/kubernetes/allocate/allocate_test.go`: when `AllocateRequest`
  mode is `STATS_MODE_HISTORICAL` and `capacity_type` is `spot`, the
  workload note is not `spot node priced on-demand`. The existing run-rate
  spot test still expects that note

### Implementation for User Story 3

- [X] T024 [US3] In `internal/engine/cluster.go`, return
  `ErrStatsModeMismatch` (text names the requested mode and the returned
  mode) when a window is set and the stats mode is not historical. Keep
  `ErrHistoricalUnsupported` for no window plus historical. Finish the
  unpriced rules from T022 on the `GetWindowCost` result. Do not call
  `deriveActualCostWindow` or read `CostResult.Monthly` on this path
- [X] T025 [US3] In `plugins/kubernetes/allocate/allocate.go`
  `allocateNode`, set `noteSpotOnDemand` only when the mode is not
  `STATS_MODE_HISTORICAL`. Leave the cost value on `PricedResource` as
  core supplied it

**Checkpoint**: `go test ./internal/engine/ -run TestRunClusterAllocation`
and `go -C plugins/kubernetes test ./allocate/ -run TestAllocate` pass.
Run-rate spot and historical-rejection tests still pass.

---

## Phase 6: User Story 4 - Identify nodes that no longer exist (Priority: P2)

**Goal**: Priceable identity comes from node-label metrics recorded in the
window. The live API is only the fallback for a node that still exists.

**Independent Test**: A node with no live object and recorded instance
type, region, and provider still yields a priceable whose id equals the
node subject. A node with neither record nor live object is omitted, with
an `incomplete:` warning. Two cluster label values and a scope that names
neither fails and lists them.

### Tests for User Story 4

- [X] T026 [P] [US4] Table tests in `plugins/prometheus/identity/labels_test.go`
  round-trip the label keys listed in `contracts/prometheus-stats.md` Node
  labels (kube-state-metrics sanitizer: non-alphanumeric to `_`). Build
  the same descriptor `plugins/kubernetes/usage/nodes.go` `NodeDescriptor`
  would build: provider, type token, region, `capacity_type` spot or
  on-demand, `id` equal to the node name, tags `kind=node`, and `cluster`
  and `node` when the cluster subject is set. `provider_id` comes from
  `kube_node_info`. A fargate compute-type node does not get an
  allocatable priceable. No Fargate price is added
- [X] T027 [P] [US4] Tests in `plugins/prometheus/identity/scope_test.go`
  for the cluster-subject table in `data-model.md`: no cluster label uses
  the request scope; one value matches an empty or equal scope; a
  disagreeing scope is `InvalidArgument`; several values without a
  matching scope is `InvalidArgument` and the error lists the values.
  Recorded identity wins. Missing recorded identity with a live node uses
  the fake client. Neither source omits the priceable and warns
  `incomplete: node <name>: cannot determine provider, instance type, or region`.
  Kubeconfig failure warns `incomplete:`. An EKS control-plane priceable
  is emitted only on the live path when the API host matches the regex in
  `plugins/kubernetes/usage/nodes.go`. Otherwise one warning without the
  `incomplete:` prefix

### Implementation for User Story 4

- [X] T028 [US4] Implement label identity in
  `plugins/prometheus/identity/labels.go`. Copy the descriptor rules from
  `plugins/kubernetes/usage/nodes.go` and name that file in a comment. Do
  not import `plugins/kubernetes`
- [X] T029 [US4] Implement cluster selection and the live-API fallback in
  `plugins/prometheus/identity/scope.go`. Client-go uses the request scope
  as the kubeconfig context. Skip fargate allocatable rows. Emit the
  control-plane descriptor only on the live path when the host matches
- [X] T030 [US4] Call the identity package from
  `plugins/prometheus/collect/collect.go` after the node queries. Priceable
  id equals the node subject on that node's usage rows. Never sum rows
  across cluster label values

**Checkpoint**: `go -C plugins/prometheus test ./identity/ ./collect/`
passes. A replaced node with recorded labels is priceable without a live
object.

---

## Phase 7: User Story 5 - Prove it in a local cluster (Priority: P2)

**Goal**: The existing kind run-rate test stays. A later step on the same
cluster asserts historical conservation and fixture hours. No cloud bill.

**Independent Test**: `make test-e2e-kind` still runs
`TestCostCluster_Kind`. It then starts Prometheus with the remote-write
receiver, writes a fixed series, and `TestCostCluster_KindHistorical`
asserts mode, period, the fixture's resource-hours, and conservation
within 1e-6. `FINFOCUS_HOME` for that test contains only the kubernetes
allocator, the prometheus plugin, and the fixed-cost plugin.

### Tests and harness for User Story 5

- [X] T031 [US5] Add module `test/e2e/kind/fixedcost` (own `go.mod`,
  finfocus-spec v0.7.5, pluginsdk only). `GetActualCost` sets `TotalCost`
  from `FINFOCUS_FIXEDCOST_TOTAL`. It does not answer projected cost as a
  substitute. Build tag stays off this module. The e2e test builds the
  binary and installs it under the test's `FINFOCUS_HOME`
- [X] T032 [P] [US5] Add `test/e2e/kind/prometheus.yaml`: one Prometheus
  container with remote-write receiver enabled
  (`--web.enable-remote-write-receiver` or
  `--enable-feature=remote-write-receiver`, matching the image). Do not
  install kube-prometheus-stack. Do not change `test/e2e/kind/setup.sh`
  or the workloads the run-rate test applies
- [X] T033 [US5] Add `test/e2e/cluster_kind_hist_test.go` with build tag
  `e2e_kind`. The test calls the built `finfocus` binary (no simulated
  costs). It remote-writes the series the plugin queries, runs
  `cost cluster --from/--to --usage-source prometheus --allocator kubernetes
  --output json` with `FINFOCUS_PROMETHEUS_URL` pointed at that Prometheus,
  and asserts `mode` `historical`, `period` equal to the window,
  resource-hours equal to the fixture, and row totals within 1e-6 of the
  fixed-cost `TotalCost`. Do not install aws-public into this
  `FINFOCUS_HOME`. The test is not parallel. An unreachable Prometheus
  must fail the command rather than print an empty success
- [X] T034 [US5] Extend the `test-e2e-kind` recipe in `Makefile` so the
  existing `TestCostCluster_Kind` run finishes before Prometheus is
  applied and `TestCostCluster_KindHistorical` runs. Keep
  `E2E_AWS_PUBLIC_VERSION` on the run-rate step only.
  `.github/workflows/ci.yml` job `e2e-kind` already calls
  `make test-e2e-kind`; leave that job in place

**Checkpoint**: Local `make test-e2e-kind` is the proof. Do not claim it
passed unless the command was run. The run-rate test does not wait on
Prometheus.

---

## Phase 8: User Story 6 - Ship the source on its own (Priority: P3)

**Goal**: The plugin is a usage source only. Selection stays explicit when
two usage sources are installed. No registry entry.

**Independent Test**: `make check-plugin-boundaries` passes.
`plugins/kubernetes` still rejects a window with `run-rate only`. A
window does not auto-select Prometheus. `internal/registry/registry.json`
has no prometheus key.

### Tests and docs for User Story 6

- [X] T035 [P] [US6] In `plugins/kubernetes/plugin_test.go`, keep
  `TestGetStats_RejectsHistorical`. Add a case whose start and end are
  both set with end after start, still `InvalidArgument` and still
  containing `run-rate only`
- [X] T036 [US6] In `internal/cli/cost_cluster_test.go`, assert that two
  installed `usage_stats` plugins with `--from` and `--to` still fail
  listing both candidates, and that `--usage-source` remains required.
  The test must not reach `GetStats`
- [X] T037 [P] [US6] Write `plugins/prometheus/README.md`: usage source
  only, `FINFOCUS_PROMETHEUS_URL`, the in-cluster default, the bearer
  token variable, the requirement for kube-state-metrics node labels,
  and that `finfocus plugin install prometheus` waits on tag
  `prometheus-v0.1.0` plus a later registry entry. State that logs go to
  stderr
- [X] T038 [US6] Run `make check-plugin-boundaries` and
  `jq -e 'has("prometheus") | not' internal/registry/registry.json` (or
  the file's actual array shape: no object whose name is `prometheus`).
  Fix any core import. Do not add a registry object

**Checkpoint**: The plugin module builds alone. Ambiguous selection and
the kubernetes window rejection still fail closed.

---

## Phase 9: Polish

**Purpose**: Docs and the repository gates.

- [X] T039 [P] Update `docs/src/content/docs/guides/cluster-costs.md` so a
  windowed `cost cluster` is documented (`--from` / `--to`, Prometheus
  source, `FINFOCUS_PROMETHEUS_URL`, node-label metrics, historical mode
  and period). Remove the sentence that every historical request is
  rejected. Keep the run-rate notes for spot projection and Fargate on
  the no-window path. `priced[].monthly` in historical JSON is the window
  `TotalCost`
- [X] T040 [P] Add a historical bullet under the cluster allocation gotcha
  in `CLAUDE.md`: window uses `GetWindowCost` and `TotalCost` with
  `SkipStateEstimate`; no window stays `GetProjectedCostWithErrors` and
  `Monthly`; `incomplete:` warnings set `ClusterResult.Incomplete`; the
  prometheus module must not import core or `plugins/kubernetes`
- [X] T041 Confirm `internal/cli/testdata/mcp/tools.golden` still lists
  `finfocus-cost-cluster` and no new tool name. New flags do not change
  the tool name. Regenerate the golden only if a command was added or
  removed
- [X] T042 Check coverage. `go -C plugins/prometheus test -coverprofile`
  stays at or above 80% of statements. The new historical pricing
  functions in `internal/engine/cluster.go` and `Engine.GetWindowCost`
  stay at or above 95%
- [X] T043 Run `make test`, `make lint`, and `make docs-lint`. Fix
  failures from this feature. `make test-e2e-kind` is the US5 checkpoint
  and is not part of `make test`
- [X] T044 Walk `specs/623-prometheus-usage-source/quickstart.md` against
  the flags and env vars that landed. Update the quickstart if a flag
  name or the default URL differs. Leave the note that registry install
  waits on `prometheus-v0.1.0`
- [X] T045 Warn when retention is shorter than the window (spec edge case
  "Retention shorter than the window"): `promql.StoreStart` reads the
  earliest node allocatable sample, and `collect.addRetentionGap` adds an
  `incomplete:` warning when it is more than one step after `start`. Tests
  in `plugins/prometheus/collect/hours_test.go` and the promql golden

**Checkpoint**: `make test` and `make lint` pass. Docs match the command.

---

## Dependencies and execution order

### Phase dependencies

- Setup (Phase 1): start immediately. T002, T003, and T004 touch different
  files and can run together after T001
- Foundational (Phase 2): after Setup. Blocks every story. T005 and T007
  can run together. T006 follows T005. T008 follows T007
- US1 (Phase 3): after Foundational. T009, T010, and T011 can run
  together. T012 follows T009 and T006. T013 follows T010 and T012. T014
  follows T011 and can run beside T012
- US2 (Phase 4): after Foundational (needs `config.go`). Parallel with
  US1 and US3. T015, T016, and T017 together, then T018, T019, T020, T021
- US3 (Phase 5): after US1, because both edit `internal/engine/cluster.go`.
  T022 and T023 can run together. T025 is only `plugins/kubernetes` and
  can run beside T024
- US4 (Phase 6): after US2. T026 and T027 together, then T028, T029, T030
- US5 (Phase 7): after US1, US2, US3, and US4
- US6 (Phase 8): T035 any time after Setup. T036 after T013. T037 after
  T021. T038 after T002 and T003
- Polish (Phase 9): after the stories being shipped. T039 and T040 can
  run together

### User story dependencies

- US1 needs Foundational `GetWindowCost`. It does not need the Prometheus
  module. Its checkpoint uses fakes
- US2 needs the module and `LoadConfig`. It does not need the CLI
- US3 needs the US1 window path in `cluster.go`
- US4 needs US2 row assembly
- US5 needs the command, the plugin, historical pricing, and node identity
- US6's selection test needs the CLI flags. Its boundary check needs the
  module

### Parallel example: after Foundational

```bash
# Engine report (US1) and Prometheus hours (US2) are different trees.
# US3 waits until US1 has finished editing internal/engine/cluster.go.

# US1 tests together:
# internal/engine/cluster_test.go
# internal/cli/cost_cluster_test.go
# internal/cli/cost_cluster_render_test.go

# US2 tests together:
# plugins/prometheus/promql/query_test.go
# plugins/prometheus/collect/hours_test.go
# plugins/prometheus/plugin_test.go
```

---

## Implementation strategy

### MVP first

1. Phase 1 and Phase 2
2. Phase 3 (User Story 1) with fake stats and a fake window pricer
3. Stop and run the US1 checkpoint tests
4. Phase 4 so the same report can be asked of Prometheus
5. Phase 5 before any real bill is trusted (`$0` must stay unpriced, and
   the spot note must stay off the historical path)

### Incremental delivery

1. US1 proves the command and the engine window
2. US2 proves the hours
3. US3 locks the pricing edges
4. US4 keeps replaced nodes priceable
5. US5 proves conservation on kind
6. US6 and Polish make the module releasable and the docs true

### Parallel team

After Foundational: one person takes US1 then US3 (`internal/engine` and
`internal/cli`), another takes US2 then US4 (`plugins/prometheus`). US5
starts when both are done. T035 (kubernetes rejection test) can land any
time.

---

## Notes

- Write the failing test, watch it fail, then implement
- `PricedSummary.Monthly` keeps the JSON name `monthly`. Historical mode
  stores `TotalCost` there
- `TestRunClusterAllocation_Errors` / `historical_mode_rejected` stays
- Do not edit `.golangci.yml` or `internal/registry/registry.json`
- Do not publish tag `prometheus-v0.1.0` in this change
