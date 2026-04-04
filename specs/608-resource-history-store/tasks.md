# Tasks: Resource History Store with Layered Cost Attribution

**Input**: Design documents from `/specs/608-resource-history-store/`
**Prerequisites**: plan.md (required), spec.md (required for user stories),
research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths). TUI changes
MUST include golden file snapshot tests and visual render verification.

**Completeness**: Per Constitution Principle VI (Implementation Completeness),
all tasks MUST be fully implemented. Stub functions, placeholders, and TODO
comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity),
documentation (README, docs/) MUST be updated concurrently with implementation
and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Go Test Path Conventions

Unit tests for Go projects MUST be colocated with source code, not placed in
`test/unit/`.

- **Unit tests**: `internal/[package]/[name]_test.go` (colocated with source)
  - Black-box (public API): `package foo_test`
  - White-box (unexported access): `package foo`
  - Run with: `go test ./internal/...`
- **Integration tests**: `test/integration/` (cross-component, requires
  running plugins)
  - Run with: `go test ./test/integration/...`
- **E2E tests**: `test/e2e/` (requires built binary and external credentials)
  - Run with: `go test -tags e2e ./test/e2e/...`

> **RETIRED**: `test/unit/` is retired as of issue #732. Do NOT place new Go
> unit tests there — they will not be discovered by `make test` or CI.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the `internal/history/` package and define core types

- [X] T001 Create `internal/history/` package directory structure
- [X] T002 [P] Implement `ResourceHistoryEntry` struct with JSON serialization
  (custom Unix timestamp marshaling) in `internal/history/entry.go`. Include
  `Source` constants (`SourceStateSnapshot`, `SourcePlanLineage`,
  `SourceAnalyzerEvent`) and validation method `Validate()` that enforces rules
  from data-model.md (URN max 1024 chars, CloudID max 512 chars, Source must be
  one of the three constants, LastSeen >= FirstSeen)
- [X] T003 [P] Implement `StackHash()` and `URNHash()` helper functions in
  `internal/history/hash.go`. Use SHA-256 of
  `"{org}/{project}/{stack}"` for stack hash and SHA-256 of full URN for URN
  hash, both hex-encoded and truncated to first 16 characters. Include
  `StackContext` struct with `Organization`, `Project`, `Stack` fields and a
  `Hash()` method

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core BoltDB store implementation, config extension, and engine
integration. MUST complete before ANY user story work.

**CRITICAL**: No user story work can begin until this phase is complete

### Tests (TDD - Write FIRST)

- [X] T004 [P] Write tests for `BoltStore` constructor, `Upsert`, `UpsertBatch`,
  `GetCloudIDsForURN`, `GetAllForStack`, `CleanupExpired`, `IsEnabled`, `Close`
  in `internal/history/store_test.go`. Use `t.TempDir()` for isolated BoltDB
  files. Cover: enabled store creation, disabled store (all ops return early),
  empty directory validation, directory auto-creation, upsert creates new entry,
  upsert same (URN,CloudID) updates LastSeen only, upsert different CloudID for
  same URN creates new entry, GetCloudIDsForURN returns all incarnations
  filtered by time range, GetAllForStack prefix scan, corruption recovery
  (detect corrupt file → delete → recreate), lock timeout returns graceful
  error, concurrent access (open two BoltStores at same path — second returns
  graceful lock error), same cloud ID under two different URNs creates separate
  entries (cloud ID reuse edge case). Use testify `require` for setup, `assert`
  for value checks
- [X] T005 [P] Write tests for retention cleanup in
  `internal/history/retention_test.go`. Cover: entries older than retention
  window are removed, entries within window are kept, entries exactly at
  boundary are kept, cleanup returns correct count, cleanup on empty store
  returns 0, cleanup with disabled store is no-op
- [X] T006 [P] Write tests for `HistoryConfig` and `AllocationConfig` YAML
  deserialization in `internal/config/budget_test.go` (add to existing file).
  Cover: default values when omitted, explicit values override defaults, nested
  under `cost.history` and `cost.allocation` keys, full round-trip
  marshal/unmarshal

### Implementation

- [X] T007 Define `HistoryStore` interface in `internal/history/store.go` with
  methods: `Upsert(entry ResourceHistoryEntry) error`,
  `UpsertBatch(entries []ResourceHistoryEntry) error`,
  `GetCloudIDsForURN(stackHash, urnHash string, from, to int64) ([]ResourceHistoryEntry, error)`,
  `GetAllForStack(stackHash string, from, to int64) ([]ResourceHistoryEntry, error)`,
  `GetDeletedResources(stackHash string, currentURNHashes map[string]bool, from, to int64) ([]ResourceHistoryEntry, error)`,
  `CleanupExpired(retentionDays int) (int, error)`, `IsEnabled() bool`,
  `Close() error`. Follow `cache.Cache` interface pattern
- [X] T008 Implement `BoltStore` struct and `NewBoltStore(ctx context.Context, directory string, enabled bool, retentionDays int) (*BoltStore, error)`
  constructor in `internal/history/store.go`. Follow `cache.NewBoltStore`
  patterns exactly: return disabled store when `enabled=false`, create directory
  with `0o750`, open BoltDB at `{directory}/history.db` with `0o600` and 500ms
  lock timeout, handle corruption (detect → delete → recreate), initialize
  `resource_history` and `resource_tags` buckets, run `CleanupExpired` on
  startup, use zerolog with `component=history` and `backend=boltdb` tags
- [X] T009 Implement `Upsert` and `UpsertBatch` methods on `BoltStore` in
  `internal/history/store.go`. Key format:
  `{stack_hash}/{urn_hash}/{cloud_id}`. Upsert logic: read existing entry, if
  found update `LastSeen` only, if not found create new entry with `FirstSeen`
  and `LastSeen` both set to current time. `UpsertBatch` uses `db.Batch()` for
  coalesced writes. Also upsert into `resource_tags` bucket for each tag on the
  entry using key format `{stack_hash}/{tag_key}:{tag_value}/{urn_hash}`
- [X] T010 Implement `GetCloudIDsForURN` and `GetAllForStack` read methods on
  `BoltStore` in `internal/history/store.go`. `GetCloudIDsForURN`: prefix scan
  on `{stackHash}/{urnHash}/`, deserialize entries, filter to entries where
  `[FirstSeen, LastSeen]` overlaps `[from, to]`, return slice.
  `GetAllForStack`: prefix scan on `{stackHash}/`, same overlap filter
- [X] T011 Implement `CleanupExpired` in `internal/history/retention.go`.
  Iterate all entries in `resource_history` bucket, delete entries where
  `LastSeen < now - retentionDays`. Also clean corresponding `resource_tags`
  entries. Use single `db.Update()` transaction (same TOCTOU prevention as
  cache). Return count of deleted entries. Log at debug level
- [X] T012 [P] Add `HistoryConfig` struct (`Enabled bool`, `RetentionDays int`,
  `Directory string`) and `AllocationConfig` struct (`Enabled bool`,
  `Tags []string`) to `internal/config/budget.go`. Add both as fields on
  `CostConfig` struct with YAML tags `history` and `allocation`. Add default
  constants: `HistoryDefaultRetentionDays = 90`,
  `HistoryDefaultEnabled = true`, `AllocationDefaultEnabled = false`
- [X] T013 [P] Add `history` field of type `history.HistoryStore` to `Engine`
  struct and `WithHistory(store history.HistoryStore) *Engine` method in
  `internal/engine/engine.go`, following the exact pattern of `WithCache()`

**Checkpoint**: Core store operational — all CRUD operations work, config
parses, engine accepts history store. Run `make test && make lint`.

---

## Phase 3: User Story 1 - Accurate Full-Month Costs (Priority: P1) MVP

**Goal**: When querying actual costs for a date range, include costs from ALL
resource incarnations (old + new cloud IDs from replacements), not just current
state.

**Independent Test**: Replace a resource mid-month in the history store, then
query `cost actual` for the full month. Verify total includes costs from both
old and new cloud IDs.

### Tests (TDD - Write FIRST)

- [X] T014 [P] [US1] Write tests for `HistoryWriter.RecordStateSnapshot` in
  `internal/history/writer_test.go`. Cover: records all resources from state
  with correct URN/CloudID/Type/Provider/Tags, sets source to
  `state_snapshot`, subsequent calls with same resources update LastSeen only,
  resources without cloud IDs are skipped, fire-and-forget behavior (errors
  logged, not returned fatally)
- [X] T015 [P] [US1] Write tests for `HistoryReader.GetResourcesForPeriod` in
  `internal/history/reader_test.go`. Cover: returns current + historical cloud
  IDs, filters by time range overlap, groups multiple cloud IDs under same URN
  into single `HistoricalResource` with `CloudIDs []string`, empty store
  returns empty slice, resources outside time range excluded
- [X] T016 [P] [US1] Write tests for history-enhanced actual cost flow in
  `internal/cli/cost_actual_test.go` (add to existing). Cover: with history
  store, merge logic creates separate ResourceDescriptors for each historical
  cloud ID; without history store (nil), behavior unchanged (no regression);
  a resource with 2 historical cloud IDs produces 2 ResourceDescriptors in
  the resources list; existing resources are not duplicated if their cloud ID
  is already in the current state list

### Implementation

- [X] T017 [US1] Implement `HistoryWriter` struct and
  `RecordStateSnapshot(stack StackContext, resources []StateResource) error`
  method in `internal/history/writer.go`. Convert each `StateResource` to
  `ResourceHistoryEntry` with `Source=SourceStateSnapshot`, extract stack hash
  and URN hash, call `store.UpsertBatch()`. Skip resources with empty CloudID.
  Log warning on error but do not propagate (fire-and-forget). Also implement
  `NewHistoryWriter(store HistoryStore, logger zerolog.Logger) *HistoryWriter`
  constructor
- [X] T018 [US1] Implement `HistoryReader` struct and
  `GetResourcesForPeriod(stack StackContext, from, to int64) ([]HistoricalResource, error)`
  method in `internal/history/reader.go`. Call `store.GetAllForStack()` with
  time range, group entries by URN hash, collect all unique cloud IDs per URN
  into `HistoricalResource.CloudIDs` slice. Also implement
  `NewHistoryReader(store HistoryStore, logger zerolog.Logger) *HistoryReader`
  constructor
- [X] T019 [US1] Implement `initHistoryFromConfig` function in
  `internal/cli/common_execution.go` following the exact pattern of
  `initCacheFromConfig`. Resolve history directory (config override → env var
  `FINFOCUS_HISTORY_DIR` → default `~/.finfocus/history`). Resolve
  enabled/retention from config with env var overrides. Create `BoltStore` via
  `history.NewBoltStore()`. Handle errors gracefully (log warning, return nil).
  Return `history.HistoryStore` and cleanup function
- [X] T020 [US1] Wire history into `cost_actual.go`: after
  `loadActualResources()` returns and before `ParseTimeRange()`, if history
  store is available: (1) call `writer.RecordStateSnapshot()` to record current
  state, (2) call `reader.GetResourcesForPeriod()` to get all historical cloud
  IDs for the date range, (3) merge historical cloud IDs into the
  `ResourceDescriptor` list (enrich existing resources with additional cloud IDs
  and add entries for deleted resources). Construct `StackContext` from Pulumi
  project/stack detection. If history store is nil, skip all history operations
  (no regression)
- [X] T021 [US1] Implement the merge logic that combines current
  `ResourceDescriptor` list with `HistoricalResource` data in
  `internal/cli/cost_actual.go`. Strategy: create one `ResourceDescriptor` per
  historical cloud ID (preserving the engine's single-ID-per-resource invariant).
  For each `HistoricalResource` from the reader: iterate its `CloudIDs` slice
  and for each cloud ID, check if a `ResourceDescriptor` already exists with
  that `pulumi:cloudId` property value. If not, create a new
  `ResourceDescriptor` with `Properties["pulumi:cloudId"]` set to that cloud ID,
  copying Type/Provider from the historical entry. This means a resource replaced
  mid-month produces TWO ResourceDescriptors (one per cloud ID), each flowing
  through the existing engine/adapter pipeline unchanged. Deduplicate by
  skipping cloud IDs already present in the current state's resource list.
  No changes to `internal/engine/engine.go` or `internal/proto/adapter.go`
  are required — the engine already handles one cloud ID per ResourceDescriptor

**Checkpoint**: `cost actual` now queries billing for ALL historical cloud IDs.
A resource replaced mid-month reports full-month costs. Run
`make test && make lint`.

---

## Phase 4: User Story 2 - Automatic Resource Identity Tracking (Priority: P2)

**Goal**: Every finfocus command that loads state automatically records resource
identity observations to the history store without user action.

**Independent Test**: Run finfocus multiple times across state changes. Verify
history store accumulates entries for all observed cloud IDs with correct
timestamps.

### Tests (TDD - Write FIRST)

- [X] T023 [P] [US2] Write tests for overview state snapshot recording in
  `internal/cli/overview_test.go` (add to existing). Cover: after
  `LoadStackExportWithContext()`, state resources are recorded to history;
  history write failure does not block overview command; disabled history store
  results in no write attempt

### Implementation

- [X] T024 [US2] Wire history writer into `overview.go`: after
  `LoadStackExportWithContext()` returns (before plan generation), call
  `writer.RecordStateSnapshot()` with converted state resources. Construct
  `StackContext` from the loaded stack metadata. History init follows same
  pattern as cost_actual.go (use `initHistoryFromConfig`). Fire-and-forget on
  error
- [X] T025 [US2] Add `initHistoryFromConfig` call and history cleanup defer in
  `newEngineWithCache` or create a new `newEngineWithCacheAndHistory` helper in
  `internal/cli/common_execution.go` that returns both cache and history
  cleanup functions. Wire this into all commands that load state: `cost actual`
  (already done in T019-T020), `overview`, and any other commands that call
  `loadActualResources` or `LoadStackExportWithContext`

**Checkpoint**: History store accumulates entries automatically across multiple
runs. Run `make test && make lint`.

---

## Phase 5: User Story 3 - Deleted Resource Cost Visibility (Priority: P3)

**Goal**: Resources that were deleted during the queried date range still appear
in `cost actual` output with their incurred costs.

**Independent Test**: Record a resource in history, remove it from current
state, then query actual costs for its active period. Verify costs appear.

### Tests (TDD - Write FIRST)

- [X] T026 [P] [US3] Write tests for `GetDeletedResources` in
  `internal/history/store_test.go` (add to existing). Cover: entries in store
  but NOT in current URN set are returned, entries in BOTH store and current set
  are excluded, time range filter applies (deleted resources outside range
  excluded), resources past retention window not returned

### Implementation

- [X] T027 [US3] Implement `GetDeletedResources` method on `BoltStore` in
  `internal/history/store.go`. Prefix scan on `{stackHash}/`, for each entry
  extract URN hash from key, if URN hash is NOT in `currentURNHashes` set AND
  entry's `[FirstSeen, LastSeen]` overlaps `[from, to]`, include in results.
  Group by URN hash to deduplicate
- [X] T028 [US3] Extend the merge logic in `internal/cli/cost_actual.go`
  (from T021) to also call `store.GetDeletedResources()` with the set of
  current URN hashes and the date range. For each deleted resource entry,
  create a `ResourceDescriptor` with historical data and add to the resources
  list for billing queries

- [X] T028a [US3] Create integration test in
  `test/integration/history_cost_test.go` that exercises the full history flow:
  (1) create a BoltStore in a temp directory, (2) write history entries for two
  cloud IDs under the same URN (simulating a mid-month replacement), (3) write
  a history entry for a resource that is NOT in the current state (simulating
  deletion), (4) call the merge logic with a current state containing only the
  new cloud ID, (5) verify the output ResourceDescriptor list contains entries
  for BOTH the old and new cloud IDs plus the deleted resource. This tests the
  end-to-end write→read→merge pipeline without requiring running plugins

**Checkpoint**: Deleted resources appear in `cost actual` output with their
historical costs. Integration test validates full write→read→merge pipeline.
Run `make test && make lint`.

---

## Phase 6: User Story 4 - Plan and Deployment Lineage Capture (Priority: P4)

**Goal**: Replace/delete operations from `pulumi preview` and analyzer events
from `pulumi up` feed old/new cloud IDs into the history store.

**Independent Test**: Run a preview with a replace operation and verify both old
and new cloud IDs are recorded in the history store.

### Tests (TDD - Write FIRST)

- [X] T029 [P] [US4] Write tests for `PulumiState.ID` field deserialization in
  `internal/ingest/pulumi_plan_test.go` (add to existing). Cover: plan JSON
  with `oldState.id` and `newState.id` fields deserialize correctly, missing
  `id` field results in empty string, replace operation produces both old and
  new IDs
- [X] T030 [P] [US4] Write tests for `RecordPlanLineage` in
  `internal/history/writer_test.go` (add to existing). Cover: replace op
  records both old and new cloud IDs as separate entries, delete op records old
  cloud ID with source `plan_lineage`, create op records new cloud ID, steps
  with empty cloud IDs are skipped
- [X] T031 [P] [US4] Write tests for `RecordAnalyzerEvent` in
  `internal/history/writer_test.go` (add to existing). Cover: analyzer event
  records URN/type/provider, cloud ID recorded when DryRun=false, event without
  cloud ID still records type/provider observation

### Implementation

- [X] T032 [US4] Add `ID string \`json:"id,omitempty"\`` field to `PulumiState`
  struct in `internal/ingest/pulumi_plan.go`. This maps the existing JSON `id`
  field from Pulumi plan output that was previously unmapped
- [X] T033 [US4] Modify `extractResources` function in
  `internal/ingest/pulumi_plan.go` to process `"replace"` and `"delete"`
  operations in addition to `"create"`, `"update"`, `"same"`. For replace/delete
  ops: extract `OldState.ID` as the previous cloud ID. For replace ops: also
  extract `NewState.ID`. Store both IDs on the returned `PulumiResource` (add
  `OldID string` and `NewID string` fields to `PulumiResource` struct). Continue
  to extract type, URN, provider, and inputs as before
- [X] T034 [US4] Implement `RecordPlanLineage(stack StackContext, steps []PlanStep) error`
  method on `HistoryWriter` in `internal/history/writer.go`. For each step:
  if `OldCloudID` is non-empty, upsert with `Source=SourcePlanLineage`; if
  `NewCloudID` is non-empty, upsert with `Source=SourcePlanLineage`. Skip
  steps where both cloud IDs are empty. Fire-and-forget on error
- [X] T035 [US4] Wire plan lineage recording into `overview.go`: after
  `pulumi preview --json` results are parsed and resources extracted, convert
  the replace/delete steps to `[]PlanStep` and call
  `writer.RecordPlanLineage()`. Only record steps that have non-empty cloud IDs
- [X] T036 [US4] Implement `RecordAnalyzerEvent(stack StackContext, event AnalyzerResource) error`
  method on `HistoryWriter` in `internal/history/writer.go`. Convert
  `AnalyzerResource` to `ResourceHistoryEntry` with
  `Source=SourceAnalyzerEvent`. If `CloudID` is empty (DryRun=true), still
  record the type/provider/URN observation but with empty CloudID (for future
  correlation). Fire-and-forget on error
- [X] T037 [US4] Wire analyzer event recording into `internal/analyzer/server.go`:
  add `historyWriter *history.HistoryWriter` field to `Server` struct. In
  `Analyze()` method, after cost calculation, call
  `historyWriter.RecordAnalyzerEvent()` with the resource data. Construct
  `StackContext` from `s.organization`, `s.projectName`, `s.stackName` (set
  by `ConfigureStack`). Set `CloudID` from resource properties when
  `s.dryRun == false`. If `historyWriter` is nil, skip (no-op)

**Checkpoint**: Replace/delete operations from previews and analyzer events
feed into history store. Run `make test && make lint`.

---

## Phase 7: User Story 5 - Tag-Based Cost Attribution Config (Priority: P5)

**Goal**: Define and validate configuration for tag-based cost attribution
fallback. Query logic is deferred to a separate milestone (Layer 2).

**Independent Test**: Set `cost.allocation.enabled: true` and
`cost.allocation.tags: [pulumi:project]` in config, verify config parses and
validates correctly.

### Tests (TDD - Write FIRST)

- [X] T038 [P] [US5] Write tests for `AllocationConfig` validation in
  `internal/config/budget_test.go` (add to existing). Cover: valid config with
  tags parses correctly, empty tags list is valid when disabled, enabled with
  empty tags logs warning, tags are preserved through config merge

### Implementation

- [X] T039 [US5] Add validation logic for `AllocationConfig` in
  `internal/config/budget.go`: when `Enabled=true` and `Tags` is empty, log
  a warning that tag-based allocation is enabled but no tags are configured.
  Validate each tag key is non-empty and <= 128 characters. The
  `AllocationConfig` struct and fields were already added in T012; this task
  adds the validation method `Validate() error`

**Checkpoint**: Tag-based allocation config is defined, parsed, and validated.
Query logic intentionally not implemented (separate milestone). Run
`make test && make lint`.

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, performance validation, and final quality checks

- [X] T040 [P] Create resource history guide at `docs/guides/cost-history.md`
  covering: what the history store is, why `history.db` is important persistent
  data (not safe to delete like cache), how resource identity tracking works,
  configuration options (`cost.history.enabled`, `cost.history.retention_days`),
  backup recommendations
- [X] T041 [P] Update `docs/reference/config-reference.md` with new config keys:
  `cost.history.enabled` (bool, default true), `cost.history.retention_days`
  (int, default 90), `cost.history.directory` (string, default
  `~/.finfocus/history`), `cost.allocation.enabled` (bool, default false),
  `cost.allocation.tags` (string list, default empty)
- [X] T042 [P] Add benchmark test `BenchmarkBoltStore_UpsertBatch` in
  `internal/history/store_test.go` for 500 resources (typical workload per
  SC-007). Verify total operation completes within 200ms. Also add
  `BenchmarkBoltStore_GetAllForStack` for prefix scan performance
- [X] T043 Verify all existing tests pass with history enabled by running
  `make test` and `make test-race` with no environment overrides. The race
  detector run satisfies the constitution quality gate requiring `-race` flag.
  Ensure no regressions (SC-009)
- [X] T044 Verify test coverage for `internal/history/` package achieves 80%+
  by running `go test -coverprofile=cover.out ./internal/history/... && go tool cover -func=cover.out | grep total`
  (SC-010)
- [X] T045 Run `make lint` and fix any linting issues across all new and
  modified files
- [X] T046 Run `make docs-lint` to verify markdown documentation passes
  markdownlint

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user
  stories
- **US1 (Phase 3)**: Depends on Foundational — this is the MVP
- **US2 (Phase 4)**: Depends on Foundational + US1 (reuses writer/reader from
  US1)
- **US3 (Phase 5)**: Depends on Foundational + US1 (extends reader/merge logic)
- **US4 (Phase 6)**: Depends on Foundational + US1 (uses writer, adds new
  write paths)
- **US5 (Phase 7)**: Depends on Foundational only (config-only, no store
  interaction)
- **Polish (Phase 8)**: Depends on all desired user stories being complete

### User Story Dependencies

```text
Phase 1 (Setup)
    │
    ▼
Phase 2 (Foundational) ──────────────────────────┐
    │                                             │
    ▼                                             ▼
Phase 3 (US1 - MVP) ──┬──────────────┐   Phase 7 (US5 - Config)
    │                  │              │
    ▼                  ▼              ▼
Phase 4 (US2)    Phase 5 (US3)  Phase 6 (US4)
    │                  │              │
    └──────────────────┴──────────────┘
                       │
                       ▼
                Phase 8 (Polish)
```

### Within Each User Story

1. Tests MUST be written and FAIL before implementation
2. Types/interfaces before implementations
3. Store operations before CLI wiring
4. Writer before reader (must write data to read it)
5. Core implementation before integration/wiring

### Parallel Opportunities

**Phase 1** (all parallel):

- T002 (entry.go) and T003 (hash.go) are independent files

**Phase 2** (partial parallel):

- T004, T005, T006 (all test files) can run in parallel
- T012 (config) and T013 (engine) are independent files
- T007-T011 are sequential (interface → constructor → methods → retention)

**Phase 3 US1** (partial parallel):

- T014, T015, T016 (test files) can run in parallel
- T017 (writer) and T018 (reader) are independent files after store exists
- T019 (CLI init) depends on store being complete
- T020-T021 are sequential (wiring → merge logic)

**Phase 6 US4** (partial parallel):

- T029, T030, T031 (test files) can run in parallel
- T034 (plan lineage writer) and T036 (analyzer writer) are independent
- T035 (overview wiring) and T037 (analyzer wiring) are independent

**Phase 7 US5** can run in parallel with Phase 4-6 (no dependencies)

**Phase 8** (partial parallel):

- T040, T041, T042 (docs and benchmarks) are independent files

---

## Parallel Example: Phase 2 (Foundational)

```text
# Launch all test files in parallel:
Task T004: "Store CRUD tests in internal/history/store_test.go"
Task T005: "Retention tests in internal/history/retention_test.go"
Task T006: "Config tests in internal/config/budget_test.go"

# After tests exist, launch independent implementation files:
Task T012: "Config structs in internal/config/budget.go"
Task T013: "Engine WithHistory in internal/engine/engine.go"

# Sequential store implementation:
Task T007 → T008 → T009 → T010 → T011
```

## Parallel Example: Phase 3 (US1 - MVP)

```text
# Launch all US1 test files in parallel:
Task T014: "Writer tests in internal/history/writer_test.go"
Task T015: "Reader tests in internal/history/reader_test.go"
Task T016: "CLI integration tests in internal/cli/cost_actual_test.go"

# Launch independent implementations:
Task T017: "HistoryWriter in internal/history/writer.go"
Task T018: "HistoryReader in internal/history/reader.go"

# Sequential CLI wiring:
Task T019 → T020 → T021
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T013) — CRITICAL, blocks all stories
3. Complete Phase 3: User Story 1 (T014-T021)
4. **STOP and VALIDATE**: Run `make test && make lint`. Test US1 independently
   by recording resources with different cloud IDs and querying actual costs
5. Deploy/demo if ready — this alone solves the core cost under-reporting bug

### Incremental Delivery

1. Setup + Foundational → Store operational
2. Add US1 → Full-month cost accuracy for `cost actual` (MVP!)
3. Add US2 → Automatic tracking across all commands
4. Add US3 → Deleted resource cost visibility
5. Add US4 → Plan lineage and analyzer event capture
6. Add US5 → Tag-based allocation config (query logic deferred)
7. Polish → Documentation, benchmarks, coverage verification

### Parallel Team Strategy

With multiple developers after Foundational phase:

- Developer A: US1 (P1 - MVP, most impactful)
- Developer B: US5 (P5 - config only, independent)
- After US1 complete:
  - Developer A: US3 (extends US1 reader)
  - Developer B: US4 (new write paths, uses US1 writer)
  - Developer C: US2 (overview wiring, uses US1 patterns)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Run `make test && make lint` after each phase checkpoint
- Stop at any checkpoint to validate story independently
- Fire-and-forget pattern: all history write operations log warnings on
  failure but never block the main command
- Layer 2 (tag-based query logic) is intentionally NOT included — config
  fields are defined but query implementation is a separate milestone
