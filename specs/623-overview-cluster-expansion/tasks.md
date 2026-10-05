# Tasks: Overview Cluster Expansion

**Input**: Design documents from `/specs/623-overview-cluster-expansion/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/json-ndjson-expansion.md

## Format: `[ID] [P?] [Story] Description`

---

## Phase 1: Setup

- [x] T001 Create fixture `testdata/overview/state-cluster-expansion.json`: a Pulumi stack export with one `aws:eks/cluster:Cluster` (properties `name: "prod-cluster"`), two `kubernetes:apps/v1:Deployment` resources (`api`, `worker`), and one `kubernetes:core/v1:ConfigMap`, following the shape of `testdata/overview/state-no-changes.json`.

## Phase 2: Foundational (blocking prerequisites)

- [x] T002 Add `ParentURN`, `ExpansionSource`, and `ChildURNs` fields (JSON keys `parentUrn`, `expansionSource`, `childUrns`, all omitempty) to `OverviewRow`, and `ExpansionNotes` (`expansionNotes,omitempty`) to `StackContext` in `internal/engine/overview_types.go`; extend `OverviewRow.Validate()` so `ExpansionSource`, when non-empty, must be `"live"` or `"projected"`.
- [x] T003 Create `internal/engine/overview_cluster.go` with `ExpansionSourceLive = "live"` / `ExpansionSourceProjected = "projected"` constants, `IsClusterResource(typeToken string) bool` covering exactly the tokens in data-model.md (`aws:eks/cluster:Cluster`, `eks:index:Cluster`, `gcp:container/cluster:Cluster`, `google-native:container/v1:Cluster`, `azure:containerservice/kubernetesCluster:KubernetesCluster`, `azure-native:containerservice:ManagedCluster`), and `IsWorkloadResource(typeToken string) bool` covering the five spec-621 kinds.
- [x] T004 [P] Table-driven unit tests for `IsClusterResource` and `IsWorkloadResource` in `internal/engine/overview_cluster_test.go` (testify require/assert per AGENTS.md).
- [x] T005 [P] Add `OverviewConfig{ClusterContexts map[string]string \`yaml:"cluster_contexts,omitempty" json:"cluster_contexts,omitempty"\`}` and `Overview *OverviewConfig \`yaml:"overview,omitempty" json:"overview,omitempty"\` to `Config` in `internal/config/config.go`, with a parsing test in the config package's existing test file style.

## Phase 3: User Story 1 — Declared workloads nest under their cluster (P1)

**Goal**: Projected expansion groups declared workload rows under the single cluster row.
**Independent Test**: `TestExpandClustersProjected` + golden test on the T001 fixture with no plugins.

- [x] T006 [US1] Implement `ExpandClustersProjected(rows []OverviewRow) []OverviewRow` in `internal/engine/overview_cluster.go`: no-op when zero or more than one cluster row exists; with exactly one cluster, set `ParentURN`/`ExpansionSource=projected` on every workload row, set `ChildURNs` on the cluster row, and order the slice parent-then-children.
- [x] T007 [P] [US1] Unit tests for `ExpandClustersProjected` in `internal/engine/overview_cluster_test.go`: zero clusters, one cluster with workloads, multiple clusters (flat), cluster with no workloads (no `ChildURNs`), workload-only stack (unchanged), ordering parent-then-children.
- [x] T008 [US1] Render children in the plain table renderer in `internal/engine/overview_render.go`: rows with non-empty `ParentURN` get a ↳ prefix on the resource cell; after the state-only footnote block, print each `stackCtx.ExpansionNotes` entry prefixed with †.
- [x] T009 [P] [US1] Golden test `TestIntegration_ClusterExpansion_Projected` in `internal/cli/overview_integration_test.go`: load the T001 fixture, `MergeResourcesForOverview`, `ExpandClustersProjected`, render table + JSON + NDJSON, assert goldens `testdata/overview/golden/table-cluster-expansion.txt`, `json-cluster-expansion.json`, `ndjson-cluster-expansion.ndjson` (generate with `UPDATE_GOLDEN=1`, then read and verify the full output by eye).
- [x] T010 [US1] TUI expansion in `internal/tui/overview_model.go` and `internal/tui/overview_view.go`: model gains `expanded map[string]bool` and `displayOrder []int`; cluster rows with `ChildURNs` render a `▸`/`▾` marker prefix in the Resource cell; keys `e` (toggle), `right` (expand), `left` (collapse) in `handleListKeypress`; `buildOverviewTable` emits children (indented with ↳) immediately after their parent when expanded, hides them when collapsed; selection/Enter maps through `displayOrder`; active filter renders flat.
- [x] T011 [P] [US1] TUI golden tests in `internal/tui/overview_golden_test.go` (`overview_expanded_cluster.golden`, `overview_collapsed_cluster.golden`): build a model with a cluster + two projected children, render collapsed and expanded views, and read the full rendered output (not string-contains) per the constitution's TUI visual verification requirement.

## Phase 4: User Story 2 — Live allocation expands a cluster (P2)

**Goal**: When usage-source + allocator plugins are installed, a cluster row expands into live namespace allocation rows.
**Independent Test**: `TestApplyLiveExpansion` with a hand-built `ClusterResult`; orchestration test with fake `UsageSource`/`Allocator`.

- [x] T012 [US2] Implement `LiveChildrenFromResult(clusterURN string, res *ClusterResult) []OverviewRow` in `internal/engine/overview_cluster.go`: group `res.Rows` with `GroupClusterRows(rows, "namespace")`, synthesize one `OverviewRow` per group with URN `<clusterURN>#ns/<namespace>`, type `finfocus:k8s/namespace:Allocation`, `StatusActive`, `ParentURN`, `ExpansionSource=live`, and `ProjectedCost{MonthlyCost: total, Currency: res.Currency, Breakdown: {"cpu":…,"memory":…}}`; idle group uses namespace `__idle__`.
- [x] T013 [US2] Implement `ApplyLiveExpansion(rows []OverviewRow, clusterURN string, children []OverviewRow) ([]OverviewRow, int)` in `internal/engine/overview_cluster.go`: remove existing projected children of that cluster (return suppressed count), attach live children, set `ChildURNs`, order parent-then-children.
- [x] T014 [US2] Update `aggregateOverviewRows` in `internal/engine/overview_render.go` to skip rows with `ExpansionSource == ExpansionSourceLive` (document: live rows re-allocate node cost already represented elsewhere).
- [x] T015 [P] [US2] Unit tests for `LiveChildrenFromResult`, `ApplyLiveExpansion`, and live-row exclusion from totals in `internal/engine/overview_cluster_test.go` and `internal/engine/overview_render_test.go`.
- [x] T016 [US2] Create `internal/cli/overview_cluster.go`: `resolveClusterScope(row engine.OverviewRow, clusterCount int, cfg *config.Config) (scope string, assumed bool)` implementing config mapping → `name`/ARN-segment → single-cluster current-context precedence; `expandClustersLive(ctx, rows, clients, eng, cfg, policyJSON)` selecting plugins via `selectCapablePlugin` semantics (skip silently when zero capable plugins, warn+skip when ambiguous), calling `engine.RunClusterAllocation` per cluster, and applying `ApplyLiveExpansion`; every error path downgrades to projected/none with a logged warning, never fatal.
- [x] T017 [P] [US2] Unit tests for `resolveClusterScope` (each precedence branch) and `expandClustersLive` (fake `UsageSource`/`Allocator` via the small interfaces in `internal/engine/cluster.go`) in `internal/cli/overview_cluster_test.go`.
- [x] T018 [US2] Wire expansion into the plain path in `internal/cli/overview.go` `executeOverview`: after `PopulateComputedDeltas`, call projected grouping then live expansion, collecting notes into `stackCtx.ExpansionNotes` (suppress-count note per FR-004, assumed-context note per FR-012).

## Phase 5: User Story 3 — Live data wins when both sources exist (P2)

**Goal**: Live children replace projected children without double counting; footnote reports suppression.
**Independent Test**: merge test with both sources for one cluster.

- [x] T019 [US3] Unit test in `internal/engine/overview_cluster_test.go`: rows containing a cluster with projected children + a live result → children are the live rows, projected rows removed, suppressed count returned; totals exclude live rows (FR-006) and a note string is produced (FR-004).

## Phase 6: User Story 4 — Machine-readable parent/child references (P1)

**Goal**: JSON/NDJSON contract per contracts/json-ndjson-expansion.md, golden-pinned.
**Independent Test**: golden tests including a live-expansion case.

- [x] T020 [US4] Extend `TestIntegration_ClusterExpansion_Projected` (T009) assertions to verify contract invariants: `childUrns` order, `parentUrn`, `expansionSource` values, summary unchanged vs. unexpanded run, NDJSON parent-then-children line order.
- [x] T021 [P] [US4] Golden test `TestIntegration_ClusterExpansion_Live` in `internal/cli/overview_integration_test.go`: build rows from the T001 fixture, apply `ApplyLiveExpansion` with a synthetic `ClusterResult` (two namespaces + idle), render JSON + NDJSON + table, assert goldens including `expansionNotes` and `†` footnote.

## Phase 7: TUI wiring for live path + polish

- [x] T022 Wire expansion into the TUI path in `internal/cli/overview.go` `overviewInitAndEnrich`: after `bridgeEnrichmentToTUI` completes, run projected grouping + live expansion, then send a new `OverviewExpansionReadyMsg{Rows, Notes}`; handle it in `internal/tui/overview_model.go` by replacing `allRows`/`rows`, rebuilding the table, and storing notes for the footer in `internal/tui/overview_view.go`.
- [x] T023 [P] Unit test the TUI message handling: expansion message replaces rows, expansion state survives, footer note renders.
- [x] T024 Run `golines -w` on changed Go files, `make lint`, `make test`; fix all findings.
- [x] T025 Update `specs/623-overview-cluster-expansion/spec.md` status to `Implemented` and verify spec status tracking conventions used by sibling specs (e.g. specs/621).

## Dependencies

- Phase 2 blocks all stories (shared types + detection).
- US1 (Phase 3) is the MVP and must land before US2/US3 (live merge reuses its grouping).
- US4 (Phase 6) builds on US1–US3 output shapes.
- T022 depends on T010 and T018.

## Parallel Execution Examples

- T004, T005 can run in parallel (different packages).
- T007, T009, T011 can run in parallel once T006/T008/T010 exist.
- T015, T017, T021 can run in parallel once T012–T014/T016 exist.
