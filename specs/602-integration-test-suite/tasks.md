# Tasks: Integration Test Suite Expansion

**Input**: Design documents from `/specs/602-integration-test-suite/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: This feature IS test development — all tasks produce integration tests. Per Constitution
Principle II (Test-Driven Development), all tests exercise real behavior via mock plugins and actual
code paths.

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be
fully implemented. No stub assertions, no placeholder test bodies, no skipped tests without
justification.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), no production code
documentation changes are needed since this feature adds only tests. Spec documentation is already
complete in `specs/602-integration-test-suite/`.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing
of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Go Test Path Conventions

- **Integration tests**: `test/integration/` (cross-component, requires mock plugins)
  - Run with: `go test -tags integration ./test/integration/...`
- **Mock extensions**: `test/mocks/plugin/` (shared test infrastructure)
- **CI workflows**: `.github/workflows/`

> **RETIRED**: `test/unit/` is retired as of issue #732. Do NOT place new Go unit tests
> there — they will not be discovered by `make test` or CI.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Extend mock plugin infrastructure needed by multiple user stories

- [x] T001 Add `SleepDuration` (`time.Duration`) field to `MockConfig` struct and implement
  sleep behavior in gRPC handlers in `test/mocks/plugin/api.go` — when `SleepDuration > 0`,
  each RPC handler sleeps for the configured duration before responding (used by US1 timeout,
  US3 latency). **Note**: The existing `LatencyMS` field adds latency to simulate slow
  responses; `SleepDuration` is distinct because it is intended to **exceed context deadlines**
  for timeout testing. If `SleepDuration > 0`, it takes precedence over `LatencyMS`
- [x] T002 Add `FailForTypes` field (`[]string`) to `MockConfig` struct in
  `test/mocks/plugin/api.go` — when set, `GetProjectedCost` returns
  `codes.Internal` error for matching resource types and succeeds for non-matching types
  (used by US1 partial failure, US3 partial failure)
- [x] T003 [P] Add `CallCount` tracking to `MockPlugin` in `test/mocks/plugin/api.go` — thread-safe
  counter incremented on each `GetProjectedCost` call, with `GetCallCount()` and `ResetCallCount()`
  methods (used by US2 cache hit verification, US5 concurrency verification)
- [x] T004 [P] Add unit tests for the new mock plugin features (`SleepDuration`, `FailForTypes`,
  `CallCount`) in `test/mocks/plugin/config_test.go` — verify sleep behavior, selective failure,
  and thread-safe call counting with at least 3 test cases each

**Checkpoint**: Mock plugin infrastructure extended — user story implementation can begin

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Create shared test helpers and synthetic data generators used across multiple stories

- [x] T005 Create synthetic plan generator function `generateSyntheticPlan(t *testing.T,
  count int, resourceTypes []string) string` in `test/integration/helpers/synthetic.go` —
  generates valid Pulumi preview JSON with `count` resources cycling through `resourceTypes`,
  writes to a temp file in `t.TempDir()`, and returns the file path as `string` (used by
  US1, US2, US5 for large plans)
- [x] T006 [P] Create synthetic analyzer stack generator function
  `generateSyntheticStack(count int) []*pulumirpc.AnalyzerResource` in
  `test/integration/helpers/synthetic.go` — generates `count` AnalyzerResource objects
  with unique URNs and valid properties (used by US3)
- [x] T007 [P] Add unit tests for synthetic generators in
  `test/integration/helpers/synthetic_test.go` — verify generated plans parse correctly,
  resource count matches, and analyzer resources have valid URNs

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel

---

## Phase 3: User Story 1 — Plugin Crash Recovery Confidence (Priority: P1)

**Goal**: Verify the system handles plugin crashes, timeouts, and missing binaries gracefully
with structured errors (not panics) and no zombie processes.

**Independent Test**: Run with `go test -v -tags integration -race -run TestPlugin ./test/integration/...`

**FRs Covered**: FR-001, FR-002, FR-003, FR-004, FR-030

- [x] T008 [US1] Implement `TestPluginResilience_CrashMidRPC` in
  `test/integration/plugin_resilience_test.go` — start mock plugin with TCP launcher,
  configure `SleepDuration` long enough to start processing then kill the process mid-call,
  verify the engine returns a structured error containing `ErrCodePluginError` (not a panic),
  and the error message is actionable (contains resource type or plugin name)
- [x] T009 [US1] Implement `TestPluginResilience_TimeoutExceedsDeadline` in
  `test/integration/plugin_resilience_test.go` — configure mock plugin with `SleepDuration`
  exceeding the context deadline (15s sleep vs 10s timeout), run `cost projected` with
  `--output json`, parse JSON output, verify at least one result contains
  `error.code == "TIMEOUT_ERROR"`, and check goroutine count before/after for leaks
  (delta should be 0 after cleanup)
- [x] T010 [US1] Implement `TestPluginResilience_MissingBinary` in
  `test/integration/plugin_resilience_test.go` — create a registry entry pointing to
  `/nonexistent/path/finfocus-plugin-fake`, attempt to launch via ProcessLauncher,
  verify the error message contains the exact missing path string
  `/nonexistent/path/finfocus-plugin-fake`
- [x] T011 [US1] Implement `TestPluginResilience_ZombieProcessPrevention` in
  `test/integration/plugin_resilience_test.go` — start a real subprocess (e.g., `sleep 60`),
  capture its PID, kill it via `cmd.Process.Kill()`, call `cmd.Wait()`, then verify the process
  no longer exists using `os.FindProcess(pid)` followed by `process.Signal(syscall.Signal(0))`
  returning an error on Unix (skip on Windows with `runtime.GOOS` check)
- [x] T012 [US1] Implement `TestPluginResilience_RecoveryAfterCrash` in
  `test/integration/plugin_resilience_test.go` — configure mock plugin to fail on first
  `GetProjectedCost` call (using call count + `FailForTypes`), send a second request,
  verify the second request either succeeds or returns a clean error (not a stale
  connection error like "transport is closing"). Include a rapid-succession sub-test
  that crashes the plugin 3+ times within 1 second and verifies no goroutine leaks or
  panics after recovery (edge case: rapid successive crashes)

**Checkpoint**: Plugin resilience tests complete — run with `-race` to verify no data races

---

## Phase 4: User Story 2 — Cache System End-to-End Verification (Priority: P1)

**Goal**: Verify cache hit/miss, TTL expiration, corruption recovery, flag precedence, and
bucket isolation across the CLI-Engine-Cache boundary.

**Independent Test**: Run with `go test -v -tags integration -race -run TestCache ./test/integration/...`

**FRs Covered**: FR-005, FR-006, FR-007, FR-008, FR-009, FR-030

- [x] T013 [US2] Implement `TestCache_HitReturnsAdapterSuffix` in
  `test/integration/cache_system_test.go` — set up mock plugin with `ScenarioSuccess`,
  create temp cache dir, run `cost projected --output json --cache-ttl 300` twice via
  CLI helper, parse JSON output from second run, verify all resources have `(cached)` in
  adapter field, and verify mock plugin `GetCallCount()` equals the resource count (not
  doubled)
- [x] T014 [US2] Implement `TestCache_TTLExpiryRequeriesPlugin` in
  `test/integration/cache_system_test.go` — set cache TTL to 1 second, run
  `cost projected --cache-ttl 1`, wait 2 seconds with `time.Sleep`, run again, verify
  second run does NOT have `(cached)` in adapter field (cache miss), and verify mock
  plugin `GetCallCount()` was called twice per resource
- [x] T015 [US2] Implement `TestCache_CorruptionAutoRecovery` in
  `test/integration/cache_system_test.go` — create a valid cache by running
  `cost projected` once, then overwrite `cache.db` with 128 random bytes using
  `os.WriteFile`, run `cost projected` again, verify command succeeds (no panic),
  results are valid, and the corrupted file was replaced with a valid new database
- [x] T016 [US2] Implement `TestCache_FlagPrecedenceOverEnvAndConfig` in
  `test/integration/cache_system_test.go` — create temp config with `cache_ttl: 180`,
  set `FINFOCUS_CACHE_TTL=120` env var via CLI helper `WithEnv`, run with `--cache-ttl 60`,
  verify effective TTL is 60 by checking that a 61-second-old entry is expired (use
  BoltDB directly to write an entry with a controlled timestamp)
- [x] T017 [US2] Implement `TestCache_BucketIsolation` in
  `test/integration/cache_system_test.go` — run `cost projected` to populate the
  `projected` bucket, then run `cost actual` with the same plan, verify the `projected`
  bucket entry count is unchanged and the `actual` bucket has its own entries (open
  `cache.db` with BoltDB read-only to inspect bucket contents)

**Checkpoint**: Cache system tests complete — verify with `-race` flag

---

## Phase 5: User Story 3 — Analyzer Concurrency and Partial Failures (Priority: P2)

**Goal**: Verify the analyzer handles large stacks, concurrent analysis requests, and partial
plugin failures correctly.

**Independent Test**: Run with `go test -v -tags integration -race -run TestAnalyzer ./test/integration/...`

**FRs Covered**: FR-010, FR-011, FR-012, FR-013, FR-030

- [x] T018 [US3] Implement `TestAnalyzer_LargeStack100Resources` in
  `test/integration/analyzer_concurrency_test.go` — create analyzer `Server` with mock
  `CostCalculator` (success scenario), generate 100-resource synthetic stack via
  `generateSyntheticStack(100)`, call `Analyze()` for each resource then `AnalyzeStack()`,
  verify completion within 10 seconds using `time.After`, verify diagnostic count >= 100,
  and check goroutine delta (capture `runtime.NumGoroutine()` before and after, assert
  delta <= 2)
- [x] T019 [US3] Implement `TestAnalyzer_ConcurrentAnalyzeCalls` in
  `test/integration/analyzer_concurrency_test.go` — create a single analyzer `Server`,
  launch 5 goroutines each calling `Analyze()` with different resources using
  `sync.WaitGroup`, run with `-race` flag, verify all 5 return non-nil responses with
  valid diagnostics, and verify `costCache` is consistent (no missing or duplicate entries)
- [x] T020 [US3] Implement `TestAnalyzer_PartialPluginFailures` in
  `test/integration/analyzer_concurrency_test.go` — configure mock `CostCalculator` to
  return error for `aws:ec2/instance:Instance` but succeed for `aws:s3/bucket:Bucket`,
  call `Analyze()` for both types, verify instance resource gets WARNING diagnostic and
  bucket resource gets cost estimate diagnostic, and the overall analysis did not panic
- [x] T021 [US3] Implement `TestAnalyzer_ContextCancellationMidAnalysis` in
  `test/integration/analyzer_concurrency_test.go` — create a context with `cancel()`,
  start processing 50 resources in a goroutine, cancel the context after 10 resources
  are processed, verify the server returns without panic and no goroutines are leaked
- [x] T022 [US3] Implement `TestAnalyzer_UnknownResourceTypes` in
  `test/integration/analyzer_concurrency_test.go` — send 10 resources where 5 have
  resource type `custom:unknown/widget:Widget` (not configured in mock), verify those
  5 produce advisory warning diagnostics (not hard errors), and the other 5 configured
  types produce valid cost diagnostics. Include a sub-test `TestAnalyzer_ZeroPriceableResources`
  that sends an empty resource array to `AnalyzeStack` and verifies a valid (possibly
  empty) diagnostics response with no panic (edge case: zero priceable resources)

**Checkpoint**: Analyzer concurrency tests complete — must pass with `-race` flag

---

## Phase 6: User Story 4 — Config Precedence End-to-End Validation (Priority: P2)

**Goal**: Verify the full config precedence chain (flag > env > project config > global
config > defaults) and error handling.

**Independent Test**: Run with `go test -v -tags integration -run TestConfig ./test/integration/...`

**FRs Covered**: FR-014, FR-015, FR-016, FR-017, FR-018, FR-030, FR-033

- [x] T023 [US4] Implement `TestConfig_WalkUpDiscovery` in
  `test/integration/config_precedence_test.go` — create temp dir tree:
  `project/Pulumi.yaml`, `project/.finfocus/config.yaml` (with `budget: {limit: 100}`),
  `project/subdir/nested/`, call `config.ResolveProjectDir(ctx, "", nestedPath)`, verify
  it returns `project/.finfocus/` by walking up from the nested subdirectory
- [x] T024 [US4] Implement `TestConfig_ProjectOverridesGlobal` in
  `test/integration/config_precedence_test.go` — create global config with
  `output: {format: table}` and project config with `output: {format: json}`, call
  `config.NewWithProjectDir()`, verify the resolved output format is `json` (project wins)
  and other global keys are inherited
- [x] T025 [US4] Implement `TestConfig_FlagOverridesEnv` in
  `test/integration/config_precedence_test.go` — create temp dirs for flag path and env
  path with different config values, set `FINFOCUS_PROJECT_DIR` env var to env path, call
  `config.ResolveProjectDir(ctx, flagPath, "")`, verify the flag path is returned (flag wins)
- [x] T026 [US4] Implement `TestConfig_MalformedYAMLReturnsDescriptiveError` in
  `test/integration/config_precedence_test.go` — write `"{{invalid yaml: ["` to a temp
  config file, call `config.NewWithProjectDir()` pointing to that dir, verify an error is
  returned (not a panic), and the error message contains `yaml` or `unmarshal` or `parse`.
  Include a sub-test `TestConfig_SemanticallyInvalidValues` that writes syntactically valid
  YAML with semantically invalid values (e.g., `budget: {limit: "not-a-number"}`,
  `output: {format: "nonexistent"}`) and verifies appropriate error or default fallback
  behavior (edge case: valid YAML syntax with invalid semantics)
- [x] T027 [US4] Implement `TestConfig_EnsureGitignoreIdempotent` in
  `test/integration/config_precedence_test.go` — create a temp `.finfocus/` dir, call
  `config.EnsureGitignore(dir)` twice, verify `.gitignore` exists with expected content
  after first call, verify second call does not modify the file (compare contents or
  modification time)
- [x] T028 [US4] Implement `TestConfig_ShallowMergeReplacement` in
  `test/integration/config_precedence_test.go` — create global config with keys
  `{output, plugins, logging}` and project config with keys `{output, analyzer}`, call
  `config.ShallowMergeYAML()`, verify `output` is from project (replaced entirely),
  `plugins` and `logging` are from global (inherited), and `analyzer` is from project
  (new key added)

- [x] T047 [US4] Implement `TestConfig_DismissedJsonProjectLocalPrecedence` in
  `test/integration/config_precedence_test.go` — create both a project-local
  `dismissed.json` (with recommendation ID "rec-001" status "dismissed") and a global
  `dismissed.json` (with recommendation ID "rec-001" status "active"), load dismissal
  store via `internal/config/dismissed.go` with project-local path, verify "rec-001"
  resolves as "dismissed" (project-local wins over global). Verify that recommendations
  present only in the global file are still accessible when no project-local entry exists
  for that ID (FR-033, US4 acceptance scenario 6)

**Checkpoint**: Config precedence tests complete — verify all 4+ override scenarios work

---

## Phase 7: User Story 5 — Concurrency and Performance Regression Detection (Priority: P2)

**Goal**: Verify `--jobs` flag correctness and concurrent resource processing produces
identical results regardless of parallelism level.

**Independent Test**: Run with `go test -v -tags integration -race -run TestConcurrency ./test/integration/...`

**FRs Covered**: FR-019, FR-020, FR-021, FR-030

- [x] T029 [US5] Implement `TestConcurrency_JobsEquivalence` in
  `test/integration/concurrency_correctness_test.go` — set up mock plugin with
  `ScenarioSuccess`, use a plan with at least 10 resources, run engine with `WithJobs(1)`
  and `WithJobs(8)`, compare the sum of `MonthlyCost` across all results (must be
  identical to 2 decimal places), and compare resource counts (must match exactly).
  Include a sub-test with only 3 resources and `WithJobs(100)` to verify correct behavior
  when the job count exceeds the resource count (edge case: jobs > resources)
- [x] T030 [US5] Implement `TestConcurrency_Jobs0AutoDetect` in
  `test/integration/concurrency_correctness_test.go` — run engine with `WithJobs(0)`,
  verify it completes without error and produces valid results (non-zero resource count)
- [x] T031 [US5] Implement `TestConcurrency_LargePlan500Resources` in
  `test/integration/concurrency_correctness_test.go` — generate 500-resource synthetic
  plan via `generateSyntheticPlan(500, ...)`, set up mock plugin with `ScenarioSuccess`,
  run engine with default concurrency, verify completion within 30 seconds using
  `context.WithTimeout`, verify result count equals 500
- [x] T032 [US5] Implement `TestConcurrency_ParallelCacheAccess` in
  `test/integration/concurrency_correctness_test.go` — create a shared temp cache dir,
  spawn 5 separate OS processes via `exec.Command` each running
  `finfocus cost projected --cache-ttl 300` pointing to the same cache dir, use
  `sync.WaitGroup` to wait for all processes, verify no BoltDB errors occurred (collect
  exit codes and stderr from each process), and verify `cache.db` is readable and not
  corrupted after all processes complete. **Rationale**: Using OS processes (not
  goroutines) tests real BoltDB file-level locking between separate process address spaces,
  which matches the spec's "5 parallel processes" requirement
- [x] T033 [US5] Implement `TestConcurrency_ThroughputMetricOutput` in
  `test/integration/concurrency_correctness_test.go` — run `cost projected` with
  `--jobs 4` and table format via CLI helper, capture stderr, verify it contains
  `resources/sec`, then run with `--output json`, verify stderr does NOT contain
  `resources/sec`

**Checkpoint**: Concurrency correctness tests complete — must pass with `-race` flag

---

## Phase 8: User Story 6 — CI Build Tag Fragmentation Resolution (Priority: P3)

**Goal**: Promote eligible trace propagation tests to `integration` tag, add nightly
justification comments, create nightly CI workflow, and add go.mod sync check.

**Independent Test**: Verify promoted tests pass with `go test -v -tags integration -run TestTracePropagation ./test/integration/...`

**FRs Covered**: FR-022, FR-023, FR-024, FR-025

- [x] T034 [US6] Split `test/integration/trace_propagation_test.go` into two files:
  (a) `test/integration/trace_propagation_test.go` with `//go:build integration` tag
  containing the 4 context-only tests (`TestTracePropagation_ContextHelpers`,
  `TestTracePropagation_GetOrGenerateFromContext`,
  `TestTracePropagation_GeneratesNewTraceID`,
  `TestTracePropagation_ExternalTraceIDPrecedence`), and
  (b) `test/integration/trace_propagation_nightly_test.go` with `//go:build nightly` tag
  containing the 3 binary-building tests (`TestTracePropagation_TraceIDInDebugOutput`,
  `TestTracePropagation_ConsistentTraceID`,
  `TestTracePropagation_ExternalTraceIDFlow`).
  **Verification**: After splitting, confirm that `go test -tags "integration,nightly"`
  does not double-run promoted tests (each test should appear in exactly one file) and
  that neither build tag constraint causes compilation errors (edge case: combined tags)
- [x] T035 [US6] Add nightly justification comment block to
  `test/integration/trace_propagation_nightly_test.go` and
  `test/integration/audit_test.go` — each file gets a comment at the top explaining:
  (1) Why the test cannot run in PR CI (builds binary, spawns subprocess),
  (2) External dependencies (Go toolchain for binary build, filesystem for audit log),
  (3) Approximate execution time (10-30s per test due to binary compilation)
- [x] T036 [P] [US6] Create `.github/workflows/nightly.yml` — GitHub Actions workflow
  triggered by `schedule` (cron: `0 3 * * *` UTC) and `workflow_dispatch` (manual),
  running on `ubuntu-latest`, checking out `main` branch, setting up Go 1.25.7,
  running `go test -v -tags nightly -race -timeout 15m ./test/integration/...`, with
  proper caching of Go modules and build artifacts
- [x] T037 [P] [US6] Add go.mod sync verification step to `.github/workflows/ci.yml` —
  add a new job or step that compares Go version in root `go.mod` vs `test/e2e/go.mod`
  (must match), and compares versions of shared dependencies
  (`github.com/rshade/finfocus-spec`, `github.com/stretchr/testify`) between the two
  modules using `grep` and version comparison

**Checkpoint**: CI build tag fragmentation resolved — promoted tests pass in PR CI

---

## Phase 9: User Story 7 — TUI Interactive Mode Regression Coverage (Priority: P3)

**Goal**: Verify TUI state machine transitions, keyboard navigation, and error recovery
using `model.Update(msg)` pattern without requiring a real TTY.

**Independent Test**: Run with `go test -v -tags integration -run TestTUI ./test/integration/...`

**FRs Covered**: FR-026, FR-027, FR-028, FR-029, FR-030

- [x] T038 [US7] Implement `TestTUI_PhaseProgression` in
  `test/integration/tui_state_machine_test.go` — create `OverviewModel` via constructor,
  verify initial state is `ViewStateInitializing`, send `OverviewPhaseMsg` for phases 0-5
  via `model.Update()`, then send `OverviewDataReadyMsg` with sample rows, verify state
  transitions to `ViewStateLoading`, then send `OverviewAllResourcesLoadedMsg`, verify
  state transitions to `ViewStateList`. Include a sub-test
  `TestTUI_UnexpectedMessageOrder` that sends `OverviewDataReadyMsg` while still in
  `ViewStateInitializing` (before any phase messages) and verifies no panic occurs and
  the model handles the out-of-order message gracefully (edge case: unexpected message
  order)
- [x] T039 [US7] Implement `TestTUI_KeyboardNavigation` in
  `test/integration/tui_state_machine_test.go` — bring model to `ViewStateList` state
  (by sending required messages), send `tea.KeyMsg{Type: tea.KeyDown}` and verify cursor
  moves, send `tea.KeyMsg{Type: tea.KeyEnter}` and verify state transitions to
  `ViewStateDetail`, send `tea.KeyMsg{Type: tea.KeyEscape}` and verify state returns to
  `ViewStateList`, send `tea.KeyMsg{Runes: []rune{'q'}}` and verify the returned `tea.Cmd`
  is `tea.Quit`
- [x] T040 [US7] Implement `TestTUI_ErrorStateOnInitFailure` in
  `test/integration/tui_state_machine_test.go` — create model in `ViewStateInitializing`,
  send `OverviewInitErrorMsg{Err: errors.New("test init error")}`, verify state
  transitions to `ViewStateError`, and verify the `View()` output contains the error
  message text
- [x] T041 [US7] Implement `TestTUI_WindowResizeNoPanic` in
  `test/integration/tui_state_machine_test.go` — create model in various states
  (`ViewStateInitializing`, `ViewStateLoading`, `ViewStateList`), send
  `tea.WindowSizeMsg{Width: 120, Height: 40}` to each, verify no panic occurs
  (test passes without recover) and returned model is non-nil

**Checkpoint**: TUI state machine tests complete — verify no panics across all states

---

## Phase 10: Polish and Cross-Cutting Concerns

**Purpose**: Final validation, quality gates, and documentation

- [x] T042 Run `make lint` and fix any lint warnings introduced by new test files —
  verify zero new warnings across all new files in `test/integration/` and
  `test/mocks/plugin/`
- [x] T043 Run `make test` to verify all existing tests still pass with no regressions
- [x] T044 Run `go test -v -tags integration -race ./test/integration/...` to verify
  all new integration tests pass with race detection enabled (FR-028)
- [x] T045 Verify test naming conventions — all new test functions follow project
  patterns (`TestSubsystem_Scenario`), all files have `//go:build integration` tag
  (except nightly files), and all use testify `require`/`assert` (not manual if/t.Errorf)
- [x] T046 File GitHub issues for any bugs discovered during test development — per
  FR-030, any discovered production bugs must be tracked as new issues, not fixed
  within this feature scope. **Result**: No new production bugs discovered. Pre-existing
  failures tracked in #809.

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 (T001-T004) — synthetic generators
  use extended mock plugin features
- **Phases 3-9 (User Stories)**: All depend on Phase 2 completion
  - US1-US5 depend on `SleepDuration`, `FailForTypes`, `CallCount` from Phase 1
  - US3 depends on `generateSyntheticStack` from Phase 2
  - US5 depends on `generateSyntheticPlan` from Phase 2
  - US6-US7 have no dependency on Phase 2 (can start after Phase 1)
- **Phase 10 (Polish)**: Depends on all story phases being complete

### User Story Dependencies

- **US1 (P1)**: Independent — no dependency on other stories
- **US2 (P1)**: Independent — no dependency on other stories
- **US3 (P2)**: Independent — uses mock CostCalculator, not mock plugin gRPC server
- **US4 (P2)**: Independent — tests config system only, no mock plugin needed
- **US5 (P2)**: Independent — uses mock plugin for concurrency testing
- **US6 (P3)**: Independent — modifies existing test files and CI workflows
- **US7 (P3)**: Independent — tests TUI model directly, no external dependencies

### Within Each User Story

1. First test in each story sets up the test file structure (build tag, imports, helpers)
2. Subsequent tests within a story can run in parallel (different test functions, same file)
3. All stories should be verified with `-race` flag after completion

### Parallel Opportunities

- T001 and T002 are sequential (both modify `api.go`), T003 and T004 are parallel
- T005, T006, T007 can run in parallel (different files)
- US1 through US7 can all run in parallel after Phase 2 (different test files)
- T036 and T037 are parallel (different workflow files)
- Phase 10 tasks are sequential (validation pipeline)

---

## Parallel Example: User Stories 1 and 2

```text
# After Phase 2 completes, launch both P1 stories in parallel:
Agent A: T008-T012 (plugin_resilience_test.go)
Agent B: T013-T017 (cache_system_test.go)

# After P1 stories complete, launch P2 stories in parallel:
Agent A: T018-T022 (analyzer_concurrency_test.go)
Agent B: T023-T028 (config_precedence_test.go)
Agent C: T029-T033 (concurrency_correctness_test.go)

# After P2 stories complete, launch P3 stories in parallel:
Agent A: T034-T037 (trace_propagation split + CI workflows)
Agent B: T038-T041 (tui_state_machine_test.go)
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Only)

1. Complete Phase 1: Mock plugin extensions (T001-T004)
2. Complete Phase 2: Synthetic generators (T005-T007)
3. Complete Phase 3: Plugin resilience tests (T008-T012)
4. Complete Phase 4: Cache system tests (T013-T017)
5. **STOP and VALIDATE**: Run `go test -v -tags integration -race -run "TestPlugin|TestCache" ./test/integration/...`
6. Run `make lint` and `make test` for regression check

### Incremental Delivery

1. Setup + Foundation → Mock infrastructure ready
2. Add US1 + US2 → P1 coverage complete (MVP!)
3. Add US3 + US4 + US5 → P2 coverage complete
4. Add US6 + US7 → P3 coverage complete (full delivery)
5. Polish → Quality gates pass, bugs filed

### Task Summary

| Phase | Story | Tasks | Files |
|-------|-------|-------|-------|
| 1 Setup | — | T001-T004 (4) | `test/mocks/plugin/api.go`, `config_test.go` |
| 2 Foundation | — | T005-T007 (3) | `test/integration/helpers/synthetic.go`, `synthetic_test.go` |
| 3 US1 | Plugin Resilience | T008-T012 (5) | `test/integration/plugin_resilience_test.go` |
| 4 US2 | Cache System | T013-T017 (5) | `test/integration/cache_system_test.go` |
| 5 US3 | Analyzer | T018-T022 (5) | `test/integration/analyzer_concurrency_test.go` |
| 6 US4 | Config | T023-T028, T047 (7) | `test/integration/config_precedence_test.go` |
| 7 US5 | Concurrency | T029-T033 (5) | `test/integration/concurrency_correctness_test.go` |
| 8 US6 | CI Tags | T034-T037 (4) | `trace_propagation*.go`, `.github/workflows/*` |
| 9 US7 | TUI | T038-T041 (4) | `test/integration/tui_state_machine_test.go` |
| 10 Polish | — | T042-T046 (5) | Cross-cutting validation |
| **Total** | | **47 tasks** | |

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- All tests must pass with `-race` flag (FR-028)
- All tests must produce zero lint warnings (FR-029)
- Discovered bugs filed as GitHub issues, not fixed in scope (FR-030)
