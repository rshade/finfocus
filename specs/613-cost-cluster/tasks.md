# Tasks: `finfocus cost cluster`

**Input**: Design documents from `/specs/613-cost-cluster/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness),
all tasks MUST be fully implemented. Stub functions, placeholders, and TODO
comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity),
documentation (guide, CLAUDE.md, README) MUST be updated concurrently with
implementation.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story. Converted from the superpowers SP3
plan (original task numbers cited as `(SP3 Task N)`); re-verified against
`main` on 2026-09-29 (see plan.md).

**Tracked as**: issue #1528. Repo rule (CLAUDE.md): never `git commit`; each
task's final step is stage-and-hand-off with a proposed conventional-commit
message.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm prerequisites on current `main` before writing code.

- [x] T001 Verify prerequisites: `go.mod` pins a finfocus-spec release with
  `usage.proto`/`allocation.proto` and the SDK helpers
  (`pluginsdk.DecodePolicy`, `ValidateAllocateRequest`,
  `ValidateAllocateResponse`, `ResolveCurrency`, `CheckConservation`,
  `DefaultConservationEpsilon`); `resolveOutputFormat`,
  `machineOutputRequested`, `writeJSON`, and
  `internal/cli/testdata/mcp/tools.golden` exist (PR #1510 merged); branch
  from `main`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Capability plumbing that every story depends on.

- [x] T002 Add `pluginhost.CapabilityUsageStats = "usage_stats"` and
  `pluginhost.CapabilityAllocation = "allocation"` and map enums 14/15 in
  `ConvertCapabilities` (`internal/pluginhost/host.go`); failing test first
  (`TestConvertCapabilities_UsageAndAllocation` covering `HasCapability` for
  both) (SP3 Task 1)

**Checkpoint**: `(*Client).HasCapability` works for both new capabilities.

---

## Phase 3: User Story 1 - View Cluster Cost Breakdown (Priority: P1) 🎯 MVP

**Goal**: `finfocus cost cluster` renders a grouped table of allocated costs
with a footer, from a live cluster, with conservation enforced.

**Independent Test**: Pipeline unit tests with in-process fakes (plugin
resolution 0/1/many, partial pricing, conservation violation from a
deliberately broken fake, mixed currencies); kind E2E (T900) proves the real
path.

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

- [x] T101 [P] [US1] Failing pipeline tests in `internal/engine/cluster_test.go`:
  fake `UsageSource`/`Allocator`/`ResourcePricer`; zero-cost node is unpriced;
  results joined by `(type, id)` not slice index; conservation violation;
  historical request → `ErrHistoricalUnsupported` (SP3 Task 2)
- [x] T102 [P] [US1] Failing grouping tests in
  `internal/engine/cluster_group_test.go`: every dimension, `label:<key>` with
  missing values under `<none>`, controller alias, `__idle__`/`__cluster__`
  placement, sort order, note de-duplication (SP3 Task 3)
- [x] T103 [P] [US1] Failing command/render tests in
  `internal/cli/cost_cluster_test.go` and `cost_cluster_render_test.go`:
  `selectCapablePlugin` 0/1/many, `--output`/`--group-by` validated before
  plugin load, table golden footer (mode, total, idle %, policy digest,
  incomplete) (SP3 Task 5)

### Implementation for User Story 1

- [x] T104 [US1] Implement the pipeline in `internal/engine/cluster.go`:
  `UsageSource`/`Allocator`/`ResourcePricer` interfaces, `ClusterRequest`,
  `ClusterRow`, `PricedSummary`, `ClusterResult`, `RunClusterAllocation`,
  `PriceableToResource`, `ModeRunRate`, `ErrConservation`,
  `ErrHistoricalUnsupported`; price via `GetProjectedCostWithErrors`;
  `Monthly <= 0` or structured error ⇒ unpriced (SP3 Task 2)
- [x] T105 [US1] Implement `VerifyConservation` in
  `internal/engine/cluster.go` delegating to
  `pluginsdk.ValidateAllocateResponse` + `pluginsdk.CheckConservation` at
  `pluginsdk.DefaultConservationEpsilon` (SP3 Task 2)
- [x] T106 [US1] Implement grouping in `internal/engine/cluster_group.go`:
  `ValidateClusterGroupBy`, `GroupClusterRows`, `ClusterGroup`,
  `GroupKeyNone` (SP3 Task 3)
- [x] T107 [US1] Implement `NewCostClusterCmd` and `selectCapablePlugin` in
  `internal/cli/cost_cluster.go`, register in `internal/cli/root.go`
  (`newCostCmd`); wire `openPlugins` + `newEngineWithCache`; flags per
  contracts/cost-cluster.md; warnings to stderr; errors exit 1 (SP3 Task 5)
- [x] T108 [US1] Implement table rendering in
  `internal/cli/cost_cluster_render.go` (tabwriter `GROUP CPU MEMORY TOTAL
  NOTES`, footer per contracts/) (SP3 Task 5)

**Checkpoint**: `finfocus cost cluster` renders a correct grouped table
against fake plugins; US2/US3 can proceed independently.

---

## Phase 4: User Story 2 - Inspect and Override the Allocation Policy (Priority: P2)

**Goal**: Policy file resolution with strict precedence and fatal parse
errors; `--show-policy` prints the effective policy without a cluster.

**Independent Test**: `internal/config` policy tests with isolated
`FINFOCUS_HOME`; `--show-policy` with only an allocator installed never
touches the cluster.

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [x] T201 [P] [US2] Failing policy-resolution tests in
  `internal/config/allocation_policy_test.go`: precedence
  flag > project > global > none, first-found-wins (no merge), broken
  discovered file is fatal, missing explicit `--policy` is fatal, HuJSON
  standardized via `ax.ParseConfig`; `isolatePolicyDirs(t)` sets
  `FINFOCUS_HOME` to a temp dir (SP3 Task 4)
- [x] T202 [P] [US2] Failing `--show-policy` CLI test: works with only an
  allocator installed, does not contact the cluster, prints effective policy +
  64-hex digest (SP3 Task 5)

### Implementation for User Story 2

- [x] T203 [US2] Implement `ResolveAllocationPolicy`, `AllocationPolicy`,
  `AllocationPolicyFile` in `internal/config/allocation_policy.go` (SP3 Task 4)
- [x] T204 [US2] Implement `ShowAllocationPolicy` in
  `internal/engine/cluster.go` (empty-usage `Allocate` returns effective
  policy + digest) and wire `--policy`/`--show-policy` into
  `internal/cli/cost_cluster.go` (SP3 Tasks 2, 5)

**Checkpoint**: Footer/JSON report policy source + digest; bad policies exit 1
naming the JSON path.

---

## Phase 5: User Story 3 - Automate via JSON/NDJSON and MCP (Priority: P2)

**Goal**: Stable machine-readable output and MCP exposure.

**Independent Test**: JSON/NDJSON shape tests; MCP golden + live `tools/list`.

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [x] T301 [P] [US3] Failing JSON/NDJSON shape tests in
  `internal/cli/cost_cluster_render_test.go` matching
  contracts/cost-cluster.md (`idle` omitted when namespace-scoped) (SP3 Task 5)
- [x] T302 [P] [US3] Confirm `go test ./internal/cli/ -run
  TestMCPSchemaToolAllowList` fails showing the new `cost_cluster` tool (SP3
  Task 6)

### Implementation for User Story 3

- [x] T303 [US3] Implement `clusterOutput`/`clusterPolicyOutput` and
  JSON/NDJSON rendering via `writeJSON` in
  `internal/cli/cost_cluster_render.go` (SP3 Task 5)
- [x] T304 [US3] Regenerate `internal/cli/testdata/mcp/tools.golden`
  (`UPDATE_GOLDEN=1 go test ./internal/cli/ -run TestMCPSchemaToolAllowList`)
  and pass `TestMCPServer_ToolListMatchesGoldenForBothEntryPoints` in
  `test/integration/` (expose, not exclude; diff shows only the added tool)
  (SP3 Task 6)

**Checkpoint**: Machine consumers and MCP clients get the documented shapes.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Integration/E2E proof and documentation.

- [x] T401 [P] Add `test/integration/cost_cluster_test.go`: no plugins →
  error naming `finfocus plugin install kubernetes`; installed kubernetes
  plugin (built in-test from `plugins/kubernetes`) does not pollute
  `cost projected` (no "not supported" errors) — use
  `helpers.NewCLIHelper(t, helpers.WithEnv(...))` with isolated
  `FINFOCUS_HOME`/`HOME` (SP3 Task 7)
- [x] T402 [P] Write `docs/src/content/docs/guides/cluster-costs.md`
  (prerequisites, first run, `--group-by`, footer, `--namespace` semantics,
  policy locations/precedence/`--show-policy`, RBAC link, limitations,
  JSON/NDJSON + MCP), add the CLAUDE.md "Cluster allocation" gotchas
  subsection, and the README feature line; `make docs-lint` and markdownlint
  pass (SP3 Task 8)
- [x] T900 Add the kind E2E: `test/e2e/kind/setup.sh` (kind cluster, nodes
  labeled m5.large/us-east-1/`finfocus.dev/provider=aws`),
  `test/e2e/kind/workloads.yaml` (Deployment/StatefulSet/DaemonSet with known
  requests), `test/e2e/cluster_kind_test.go` (tag `e2e_kind`; conservation,
  Deployment cost = request/allocatable × reported node price within epsilon,
  namespace scope omits idle, bad policy exits 1, `--show-policy` needs no
  cluster), `make test-e2e-kind` target, CI job `e2e-kind` via
  `helm/kind-action` (pin current major); add `k8s.io/apimachinery` to
  `test/e2e/go.mod` (SP3 Task 9 — requires SP2 merged; released aws-public
  installed with `--metadata region=us-east-1`)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — verification only.
- **Foundational (Phase 2)**: Blocks all stories.
- **US1 (Phase 3)**: MVP; depends on Phase 2 only.
- **US2 (Phase 4)**: Depends on Phase 2; integrates with US1's command but is
  independently testable via `internal/config` and `--show-policy`.
- **US3 (Phase 5)**: Depends on US1's rendering surface (T107/T108).
- **Polish (Phase 6)**: T401/T402 parallel; T900 (kind E2E) needs US1–US3 and
  the merged SP2 plugin.

### Within Each User Story

- Tests MUST be written and FAIL before implementation.
- Engine pipeline before CLI command; grouping before rendering.
- Story complete before moving to the next priority.

---

## Notes

- `VerifyConservation` MUST call `pluginsdk.ValidateAllocateResponse` +
  `pluginsdk.CheckConservation` — do not re-implement the rules in core.
- Every test touching config sets `FINFOCUS_HOME` to a temp dir
  (`isolateConfig(t)` in `internal/cli`).
- Use `RunE`, `cmd.Printf`/`cmd.OutOrStdout()`; testify `require`/`assert`
  only.
- Deferred (non-goals, tracked on the roadmap): `STATS_MODE_HISTORICAL`
  (SP4), config routing for the new capabilities, idle/share redistribution
  policies.

## Implementation Notes (2026-09-30, issue #1528)

All tasks complete and verified (`make validate`, `make test`, `make test-race`,
`make lint`, `make docs-lint`, and `make test-e2e-kind` against a real kind
cluster). Deviations from the original SP3 plan text, all test-side:

- `TestGroupClusterRows` fixture sums to 105, not 115 (the plan's constant was
  a typo; the per-group expectations were correct).
- The SP3 Task 7 pollution test asserts `NotContains "not supported"` /
  `"ERROR:"` plus the engine's deliberate `declined by kubernetes` note
  (`internal/engine/engine.go` `declineNotes`) instead of
  `NotContains "kubernetes:"` — decline notes are an intentional engine
  feature, not pollution.
- The kind E2E bad-policy assertion matches `unknown field` + the offending
  key separately because stderr carries an ax JSON error envelope (quotes are
  escaped inside `message`).
- `TestResolveAllocationPolicy` treats a comment-only policy file as a fatal
  parse error (ax.ParseConfig rejects it), consistent with the spec's fatal
  parse-error rule; a literal `null` document is normalized to "no policy".
