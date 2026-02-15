# Tasks: Reliability & Quality Fixes Batch

**Input**: Design documents from `/specs/592-reliability-quality-fixes/`
**Prerequisites**: plan.md (required), spec.md (required), research.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. All 8 tickets (#602, #605, #610, #652, #653, #654, #655, #656) are mapped to 5 user stories.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: User Story 1 - Configurable Parallel Cost Analysis (Priority: P1)

**Goal**: Add `--jobs`/`-j` flag to cost commands for configurable worker concurrency, plus timing output showing throughput metrics.

**Independent Test**: Run `finfocus cost projected --jobs 4 --pulumi-json plan.json` and verify concurrency override and timing output.

**Ticket**: #602

### Tests for User Story 1

- [X] T001 [US1] Write tests for --jobs flag validation (negative, zero, positive, exceeds resource count) in `internal/cli/cost_projected_test.go`
- [X] T002 [P] [US1] Write tests for timing output format and suppression in JSON/NDJSON mode in `internal/cli/cost_projected_test.go`
- [X] T003 [P] [US1] Write tests for engine worker count override when jobs > 0 in `internal/engine/engine_test.go`

### Implementation for User Story 1

- [X] T004 [US1] Add jobs field to engine config and modify `getWorkerCount()` to accept optional override in `internal/engine/engine.go`
- [X] T005 [US1] Add `jobs` field to `costProjectedParams` struct and register `--jobs`/`-j` flag in `internal/cli/cost_projected.go`
- [X] T006 [US1] Add timing output logic (time.Now/Since, stderr, table-only) to `executeCostProjected` in `internal/cli/cost_projected.go`
- [X] T007 [P] [US1] Add `jobs` field to `costActualParams` struct, register `--jobs`/`-j` flag, and add timing output to `internal/cli/cost_actual.go`

**Checkpoint**: `--jobs` flag works on both cost commands, timing output displays for table format, suppressed for JSON/NDJSON

---

## Phase 2: User Story 2 - Cancellable Plugin Operations (Priority: P1)

**Goal**: Make all GitHub registry HTTP requests context-cancelable so Ctrl+C terminates in-flight network operations.

**Independent Test**: Initiate a plugin install, press Ctrl+C, verify prompt cancellation.

**Ticket**: #654

### Tests for User Story 2

- [X] T008 [US2] Write tests for context cancellation of HTTP requests in `internal/registry/github_test.go`

### Implementation for User Story 2

- [X] T009 [US2] Add `context.Context` as first parameter to all `GitHubClient` public methods (`GetLatestRelease`, `GetReleaseByTag`, `ListStableReleases`, `FindReleaseWithAsset`, `FindReleaseWithFallbackInfo`, `DownloadAsset`) and internal `fetchRelease` in `internal/registry/github.go`
- [X] T010 [US2] Replace all `http.NewRequest()` with `http.NewRequestWithContext(ctx, ...)` and remove `//nolint:noctx` directives in `internal/registry/github.go`
- [X] T011 [US2] Update all callers to pass `cmd.Context()` or propagated context in `internal/cli/plugin_install.go`, `internal/cli/plugin_update.go`, and any other registry consumers

**Checkpoint**: All registry HTTP requests are context-aware; Ctrl+C cancels in-flight requests

---

## Phase 3: User Story 3 - Reliable Nightly Quality Checks (Priority: P2)

**Goal**: Remove `|| true` masking from nightly fuzz tests so failures are properly reported.

**Independent Test**: Verify no `|| true` on fuzz test lines; confirm `if: always()` on corpus upload steps.

**Ticket**: #655

### Implementation for User Story 3

- [X] T012 [US3] Remove `|| true` from all 4 fuzz test steps (`FuzzJSON`, `FuzzPulumiPlanParse`, `FuzzYAML`, `FuzzSpecFilename`) in `.github/workflows/nightly.yml`
- [X] T013 [US3] Verify `if: always()` is set on corpus upload and cache steps to preserve artifacts on failure in `.github/workflows/nightly.yml`

**Checkpoint**: Fuzz test failures fail the nightly workflow; corpus upload still works on failure

---

## Phase 4: User Story 4 - Stable Long-Running Operations (Priority: P2)

**Goal**: Fix goroutine leaks and unbounded concurrency in overview enrichment, cache expired entry cleanup, and stdio proxy.

**Independent Test**: Run `make test-race` with zero data races; verify stable goroutine counts under load.

**Tickets**: #652, #653, #656

### Tests for User Story 4

- [X] T014 [P] [US4] Write test for worker pool goroutine bound in overview enrichment in `internal/engine/overview_enrich_test.go`
- [X] T015 [P] [US4] Write test for synchronous expired entry deletion (no goroutine leak) in `internal/engine/cache/store_test.go`
- [X] T016 [P] [US4] Write test for proxy graceful shutdown (both io.Copy directions complete) in `internal/pluginhost/stdio_test.go`

### Implementation for User Story 4

- [X] T017 [P] [US4] Refactor `EnrichOverviewRows` from goroutine-per-row + semaphore to fixed worker pool (channel + WaitGroup, `overviewConcurrencyLimit` workers) in `internal/engine/overview_enrich.go`
- [X] T018 [P] [US4] Replace async goroutine deletion in `FileStore.Get()` with synchronous deletion: release RLock, acquire write Lock, delete file, return `ErrCacheExpired` in `internal/engine/cache/store.go`
- [X] T019 [P] [US4] Add `sync.WaitGroup` to `proxy()` for both `io.Copy` directions; close opposite stream on completion to unblock paired copy in `internal/pluginhost/stdio.go`

**Checkpoint**: `make test-race` passes; goroutine count bounded; clean shutdown on disconnect

---

## Phase 5: User Story 5 - Consistent Developer Experience (Priority: P3)

**Goal**: Isolate flaky auto-detection tests and consolidate duplicate recommendation helpers.

**Independent Test**: `make test` passes from any directory; `grep formatRecsColumn internal/tui/` returns zero matches.

**Tickets**: #605, #610

### Tests for User Story 5

- [X] T020 [US5] Verify existing `TestCountRecommendations` passes with exported name `CountRecommendations` in `internal/engine/project_test.go`

### Implementation for User Story 5

- [X] T021 [P] [US5] Add temp directory isolation (`os.Getwd`, `t.TempDir`, `os.Chdir`, `t.Cleanup`) to `TestCostProjectedWithoutPulumiJson` and `TestStackFlagPassedThrough` in `internal/cli/cost_projected_test.go`
- [X] T022 [P] [US5] Export `countRecommendations` as `CountRecommendations` and `formatRecommendationCount` as `FormatRecommendationCount` in `internal/engine/project.go`
- [X] T023 [US5] Update `TestCountRecommendations` to use exported name in `internal/engine/project_test.go`
- [X] T024 [US5] Replace inline counting and `formatRecsColumn` with `engine.CountRecommendations()` and `engine.FormatRecommendationCount()` in `internal/tui/cost_view.go`; remove `formatRecsColumn` function

**Checkpoint**: Tests pass from any directory; zero duplicate recommendation helpers in codebase

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and documentation updates

- [X] T025 Run `make test` and verify all tests pass
- [X] T026 Run `make test-race` and verify zero data races
- [X] T027 Run `make lint` and verify zero lint errors
- [X] T028 [P] Update CLAUDE.md to document `--jobs`/`-j` flag and timing output behavior
- [X] T029 Run `specs/592-reliability-quality-fixes/quickstart.md` verification steps

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (US1)**: No dependencies on other phases - can start immediately
- **Phase 2 (US2)**: No dependencies on other phases - can start immediately
- **Phase 3 (US3)**: No dependencies on other phases - can start immediately
- **Phase 4 (US4)**: No dependencies on other phases - can start immediately
- **Phase 5 (US5)**: No dependencies on other phases - can start immediately
- **Phase 6 (Polish)**: Depends on ALL user story phases being complete

### User Story Dependencies

- **US1 (P1)**: Independent - modifies CLI + engine exclusively
- **US2 (P1)**: Independent - modifies registry + CLI plugin commands exclusively
- **US3 (P2)**: Independent - modifies CI workflow only (no Go code)
- **US4 (P2)**: Independent - modifies engine/cache, engine/overview, pluginhost exclusively
- **US5 (P3)**: Independent - modifies CLI tests, engine/project, TUI exclusively

**All 5 user stories can proceed in parallel** since they modify disjoint file sets.

### Within Each User Story

- Tests written FIRST and must FAIL before implementation
- Implementation tasks in dependency order (engine before CLI)
- Story complete before moving to polish phase

### Parallel Opportunities

Within each phase, tasks marked [P] touch different files and can execute concurrently:

- **US1**: T002 + T003 (different test files); T005 + T007 (projected vs actual)
- **US4**: T014 + T015 + T016 (three different packages); T017 + T018 + T019 (three different packages)
- **US5**: T021 + T022 (test file vs engine file)

---

## Parallel Example: User Story 4 (All 3 Fixes)

```text
# Launch all tests in parallel (3 different packages):
Task T014: "Test worker pool bound in internal/engine/overview_enrich_test.go"
Task T015: "Test synchronous deletion in internal/engine/cache/store_test.go"
Task T016: "Test proxy shutdown in internal/pluginhost/stdio_test.go"

# Launch all implementations in parallel (3 different packages):
Task T017: "Refactor overview enrichment in internal/engine/overview_enrich.go"
Task T018: "Fix cache deletion in internal/engine/cache/store.go"
Task T019: "Fix proxy lifecycle in internal/pluginhost/stdio.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: US1 (--jobs flag + timing output)
2. **STOP and VALIDATE**: Test with `finfocus cost projected --jobs 4`
3. Deploy/demo if ready

### Incremental Delivery

1. US1 (--jobs flag) → Test → Commit
2. US2 (context HTTP) → Test → Commit
3. US3 (fuzz masking) → Test → Commit
4. US4 (goroutine fixes) → Test → Commit
5. US5 (DRY + test isolation) → Test → Commit
6. Polish → Final validation → PR

### Fastest Path (Maximum Parallelism)

All 5 user stories modify disjoint files and can execute simultaneously:

1. US1 + US2 + US3 + US4 + US5 in parallel
2. Polish phase
3. Final `make test && make lint` validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- All 5 user stories touch disjoint file sets: full parallelism possible
- Commit after each user story for clean git history
- Total: 29 tasks across 5 user stories + polish phase
