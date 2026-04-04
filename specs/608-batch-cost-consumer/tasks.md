# Tasks: BatchCost RPC Consumer for Multi-Resource Queries

**Input**: Design documents from `/specs/608-batch-cost-consumer/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths). TUI changes
MUST include golden file snapshot tests and visual render verification.

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Go Test Path Conventions

Unit tests for Go projects MUST be colocated with source code, not placed in `test/unit/`.

- **Unit tests**: `internal/[package]/[name]_test.go` (colocated with source)
  - Black-box (public API): `package foo_test`
  - White-box (unexported access): `package foo`
  - Run with: `go test ./internal/...`
- **Integration tests**: `test/integration/` (cross-component, requires running plugins)
  - Run with: `go test ./test/integration/...`
- **E2E tests**: `test/e2e/` (requires built binary and external credentials)
  - Run with: `go test -tags e2e ./test/e2e/...`

> **RETIRED**: `test/unit/` is retired as of issue #732. Do NOT place new Go unit tests
> there — they will not be discovered by `make test` or CI.

---

## Phase 1: Foundational — Adapter Layer

**Purpose**: Extend the internal plugin client interface to expose BatchCost RPC and implement response mapping functions. This phase MUST be complete before any engine work can begin.

**Why foundational**: Every user story (batch execution, fallback, partial failure) depends on the adapter being able to send batch requests and map responses. Without this, the engine has no way to communicate batch operations to plugins.

### Tests for Adapter Layer (TDD Required)

- [x] T001 [P] Write unit tests for `clientAdapter.BatchCost` pass-through delegation in `internal/proto/adapter_test.go` — test that the adapter calls the underlying gRPC client's `BatchCost` method with the correct request and returns the response
- [x] T002 [P] Write unit tests for `mapBatchProjectedResults` in `internal/proto/adapter_test.go` — test cases: all success (3 resources with projected cost data), all errors, mixed success/error, empty response, nil CostData (triggers fallback), ExpiresAt extraction
- [x] T003 [P] Write unit tests for `mapBatchActualResults` in `internal/proto/adapter_test.go` — test cases: all success with ActualCostData, ResourceError with and without ResourceTypeUnsupported, empty results array

### Implementation for Adapter Layer

- [x] T004 Add `BatchCost` method to `CostSourceClient` interface in `internal/proto/adapter.go` (line ~708) with signature: `BatchCost(ctx context.Context, in *pbc.BatchCostRequest, opts ...grpc.CallOption) (*pbc.BatchCostResponse, error)`
- [x] T005 Implement `clientAdapter.BatchCost` as pass-through to `c.client.BatchCost(ctx, in, opts...)` in `internal/proto/adapter.go`
- [x] T006 Define `batchMappedResult` struct in `internal/proto/adapter.go` with fields: `Result *CostResult`, `ActualResult *ActualCostResult`, `Err error`, `Skip bool` (for resource_type_unsupported)
- [x] T007 Implement `mapBatchProjectedResults(resp *pbc.BatchCostResponse) []batchMappedResult` in `internal/proto/adapter.go` — map each `ResourceCostResult`: `CostData.GetProjectedCost()` → `CostResult` (reuse existing projected cost mapping logic), `ResourceError` → error or skip, nil CostData → nil result
- [x] T008 Implement `mapBatchActualResults(resp *pbc.BatchCostResponse) []batchMappedResult` in `internal/proto/adapter.go` — map each `ResourceCostResult`: `CostData.GetActualCost()` → `ActualCostResult` (reuse existing actual cost mapping logic), `ResourceError` → error or skip
- [x] T009 Update `mockCostSourceClient` in `internal/proto/adapter_test.go` to implement the new `BatchCost` interface method with configurable response/error fields
- [x] T010 Update any other mock implementations of `CostSourceClient` across the codebase (search for types implementing `CostSourceClient` interface) to add the `BatchCost` method

**Checkpoint**: Adapter can send batch requests and map responses. All T001-T003 tests pass. `go test ./internal/proto/...` green.

---

## Phase 2: User Story 1 — Batch Cost Estimation for Large Stacks (Priority: P1) MVP

**Goal**: When a plugin supports batch cost, the engine sends all resources in a single BatchCost RPC (chunked by max_batch_size) instead of N individual calls. Results are mapped back to per-resource CostResult entries and cached independently.

**Independent Test**: Run `GetProjectedCost` with a mock batch-capable plugin and 150 resources. Verify: 2 batch calls (chunked at 100), all 150 resources receive cost results, cached resources excluded from batch, results cached after receipt.

### Tests for User Story 1 (TDD Required)

- [x] T011 [P] [US1] Write unit tests for `chunkResources` in `internal/engine/engine_batch_test.go` — test cases: 50 resources with chunk size 100 (1 chunk), 200 resources with chunk size 100 (2 chunks), 101 resources (2 chunks: 100+1), chunk size 0 (uses default 100), empty input
- [x] T012 [P] [US1] Write unit tests for `groupResourcesByPlugin` in `internal/engine/engine_batch_test.go` — test cases: all resources match one plugin, resources split across 2 plugins, resources with no plugin match, mix of batch-capable and non-batch plugins, internal Pulumi types filtered
- [x] T013 [P] [US1] Write unit tests for `executeBatchForPlugin` in `internal/engine/engine_batch_test.go` — test cases: single chunk success, multi-chunk with max_batch_size adjustment, projected query type, actual query type with date range, dry_run always false
- [x] T014 [US1] Write integration test for batch path in `GetProjectedCost` in `internal/engine/engine_batch_test.go` — test: 50 resources, batch-capable mock plugin, verify single BatchCost call made, all results returned correctly, results cached
- [x] T015 [US1] Write integration test for batch path in `GetActualCostWithOptions` in `internal/engine/engine_batch_test.go` — test: resources with date range, verify COST_QUERY_TYPE_ACTUAL and start/end timestamps passed correctly
- [x] T016 [US1] Write unit test for cache pre-check in batch path in `internal/engine/engine_batch_test.go` — test: 10 resources where 3 are already cached, verify batch request contains only 7 resources, cached results merged into final output

### Implementation for User Story 1

- [x] T017 [P] [US1] Create `internal/engine/engine_batch.go` with internal types: `pluginBatch` (plugin client, indexed resources, hasBatch flag), `indexedResource` (original index + ResourceDescriptor), `batchResult` (index + CostResult/ActualCostResult + error), `batchOptions` (queryType, start, end timestamps)
- [x] T018 [US1] Implement `chunkResources(resources []indexedResource, chunkSize int) [][]indexedResource` in `internal/engine/engine_batch.go` — split resources into chunks of at most chunkSize; if chunkSize <= 0, use `batchProcessingThreshold` (100)
- [x] T019 [US1] Implement `groupResourcesByPlugin(ctx, resources, feature) map[string]*pluginBatch` in `internal/engine/engine_batch.go` — call `selectPluginMatchesForResource` per resource with feature `"BatchCost"`, group by primary match plugin name, set `hasBatch` from `client.HasCapability("batch_cost")`
- [x] T020 [US1] Implement `executeBatchForPlugin(ctx, plugin, resources, queryType, opts) ([]batchResult, error)` in `internal/engine/engine_batch.go` — chunk resources, build `pbc.BatchCostRequest` per chunk (set query_type, start/end for actual, dry_run=false), call `plugin.API.BatchCost`, map response via adapter mapping functions, adjust chunk size if `MaxBatchSize > 0`
- [x] T021 [US1] Implement `buildBatchCostRequest(resources []indexedResource, opts batchOptions) *pbc.BatchCostRequest` in `internal/engine/engine_batch.go` — construct proto request with ResourceDescriptors (resolving SKU/region via existing `resolveSKUAndRegion`), set QueryType, Start/End, DryRun=false
- [x] T022 [US1] Integrate batch path into `GetProjectedCost` in `internal/engine/engine.go` — before worker pool setup: call `groupResourcesByPlugin`, for each batch-capable plugin group: pre-check cache per resource (using `tryProjectedCostCache`), call `executeBatchForPlugin` for uncached resources, store results via `storeProjectedCostCache`, collect handled indices; pass remaining unhandled resources to existing worker pool
- [x] T023 [US1] Integrate batch path into `GetActualCostWithOptions` in `internal/engine/engine.go` — same pattern as T022 but with `COST_QUERY_TYPE_ACTUAL`, date range from options, and `tryActualCostCache`/`storeActualCostCache` for cache integration

**Checkpoint**: Batch cost estimation works end-to-end for both projected and actual paths. Mock batch plugins receive chunked requests, results are mapped and cached. `go test ./internal/engine/...` green. `make test` green.

---

## Phase 3: User Story 2 — Graceful Fallback for Non-Batch Plugins (Priority: P1)

**Goal**: When a plugin does not support batch cost capability, the system falls back to the existing per-resource query path with zero behavior change. When a batch-capable plugin returns a batch-level gRPC error, the system falls back to per-resource queries and logs a warning.

**Independent Test**: Run `GetProjectedCost` with a non-batch plugin — verify identical behavior to pre-feature code. Run with a batch plugin that returns `codes.Unimplemented` — verify fallback to per-resource and warning logged.

### Tests for User Story 2 (TDD Required)

- [x] T024 [P] [US2] Write unit test for non-batch plugin fallback in `internal/engine/engine_batch_test.go` — test: plugin without `batch_cost` capability, verify all resources go through existing per-resource worker pool, no BatchCost RPC called
- [x] T025 [P] [US2] Write unit test for batch-level gRPC error fallback in `internal/engine/engine_batch_test.go` — test cases: `codes.Unimplemented` → fallback, `codes.Unavailable` → fallback, `codes.Internal` → fallback, `codes.DeadlineExceeded` → fallback if time remains / propagate if not
- [x] T026 [US2] Write unit test for response count mismatch fallback in `internal/engine/engine_batch_test.go` — test: batch response returns fewer results than request resources, verify fallback to per-resource for all resources with warning logged

### Implementation for User Story 2

- [x] T027 [US2] Implement capability-based routing in batch integration in `internal/engine/engine.go` — in the batch path (T022/T023): for plugin groups where `hasBatch == false`, skip batch and pass resources directly to the per-resource worker pool
- [x] T028 [US2] Implement batch-level error fallback in `internal/engine/engine_batch.go` — in `executeBatchForPlugin`: when batch RPC returns a gRPC status error (`codes.Unimplemented`, `codes.Unavailable`, `codes.Internal`), return the error; in the engine integration (T022/T023): catch the error, log warning with plugin name and error, redirect all resources from that batch to the per-resource worker pool
- [x] T029 [US2] Implement response count mismatch detection in `internal/engine/engine_batch.go` — in result mapping: if `len(resp.Results) != len(requestResources)`, log error with counts, return error to trigger fallback
- [x] T030 [US2] Implement deadline-aware fallback in `internal/engine/engine_batch.go` — on `codes.DeadlineExceeded`: check `ctx.Err()`, if context is still valid (deadline not globally exceeded), fall back to per-resource; if context is done, propagate the timeout error

**Checkpoint**: Non-batch plugins work identically to pre-feature behavior. Batch errors trigger graceful fallback with logging. `go test ./internal/engine/...` green. `make test` green.

---

## Phase 4: User Story 3 — Partial Failure Handling (Priority: P2)

**Goal**: When some resources in a batch succeed and others fail, successful results are preserved and errors are reported per-resource. `resource_type_unsupported` errors are treated as skips (same as current per-resource validation skip).

**Independent Test**: Submit a batch of 10 resources where 3 return `ResourceError` (1 with `resource_type_unsupported`, 2 with other errors). Verify: 7 successful results returned, 1 resource skipped silently, 2 errors logged with resource context.

### Tests for User Story 3 (TDD Required)

- [x] T031 [P] [US3] Write unit test for mixed success/error batch results in `internal/engine/engine_batch_test.go` — test: 10 resources, 7 succeed, 3 fail (mix of error types), verify 7 CostResults returned, 3 errors logged
- [x] T032 [P] [US3] Write unit test for `resource_type_unsupported` skip behavior in `internal/engine/engine_batch_test.go` — test: ResourceError with `ResourceTypeUnsupported=true`, verify resource is skipped (WARN log emitted for observability, no error result returned, same as per-resource validation skip behavior)
- [x] T033 [US3] Write unit test for all-fail batch in `internal/engine/engine_batch_test.go` — test: all resources return ResourceError, verify all individual errors reported (no batch-level fallback, each error treated independently)
- [x] T034 [US3] Write unit test for nil/empty CostData fallback in `internal/engine/engine_batch_test.go` — test: ResourceCostResult has nil CostData (not an error, just empty), verify that resource is queued for fallback to next plugin in chain if `ShouldFallback` is true

### Implementation for User Story 3

- [x] T035 [US3] Implement per-resource error handling in batch result processing in `internal/engine/engine_batch.go` — when mapping batch results back to engine types: for each `batchMappedResult` with `Skip=true` (resource_type_unsupported), log at WARN with resource type and skip; for each with `Err != nil`, log at WARN with resource type, error message, and include in error collection
- [x] T036 [US3] Implement nil/empty result fallback in batch integration in `internal/engine/engine.go` — after processing batch results: collect resources where `batchMappedResult.Result == nil && !Skip && Err == nil` (nil data, not an error), check if `router.ShouldFallback(pluginName)`, if yes queue for next plugin in fallback chain via per-resource worker pool
- [x] T037 [US3] Implement successful result preservation in `internal/engine/engine_batch.go` — ensure that per-resource errors do NOT discard successful results from the same batch; successful results are added to the final results slice and cached independently regardless of other resources' failures

**Checkpoint**: Partial failures handled gracefully. Successful results preserved. Unsupported types skipped. `go test ./internal/engine/...` green. `make test` green.

---

## Phase 5: User Story 4 — Batch Capability Visibility (Priority: P3)

**Goal**: Users can see which plugins support batch cost by inspecting `finfocus plugin list` output.

**Independent Test**: Run `finfocus plugin list --verbose` with a batch-capable plugin. Verify `batch_cost` appears in the Capabilities column.

### Tests for User Story 4 (TDD Required)

- [x] T038 [US4] Write unit test verifying `batch_cost` capability appears in plugin list output in `internal/cli/plugin_list_test.go` — test: mock plugin with `Metadata.Capabilities = ["projected_costs", "actual_costs", "batch_cost"]`, verify `batch_cost` appears in verbose table output and JSON output

### Implementation for User Story 4

- [x] T039 [US4] Verify `ConvertCapabilities` correctly maps `PLUGIN_CAPABILITY_BATCH_COST` → `"batch_cost"` in `internal/pluginhost/host.go` — this mapping already exists; add a unit test in `internal/pluginhost/host_test.go` if not already covered to confirm the string `"batch_cost"` is produced

**Checkpoint**: Plugin list shows batch cost capability. No code changes needed — verification and test coverage only. `go test ./internal/cli/... ./internal/pluginhost/...` green.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Final validation, coverage, and quality gates

- [x] T040 Update documentation if any public-facing docs reference the plugin client interface or plugin development — add batch cost capability details to relevant README or docs/ sections per Constitution Principle IV
- [x] T041 Run `make test` and verify all tests pass across the entire codebase
- [x] T042 Run `make lint` and fix any linting issues in new/modified files
- [x] T043 Run coverage check: `go test -coverprofile=coverage.out ./internal/engine/... ./internal/proto/...` and verify 80%+ overall, 95%+ on `engine_batch.go` and adapter batch mapping functions
- [x] T044 Verify no regressions: run `go test -race ./internal/engine/... ./internal/proto/...` to confirm no race conditions in batch path (batch + worker pool concurrent access)

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Foundational)**: No dependencies — start immediately
- **Phase 2 (US1 - Batch Estimation)**: Depends on Phase 1 completion (adapter interface required)
- **Phase 3 (US2 - Fallback)**: Depends on Phase 2 (batch path must exist to add fallback)
- **Phase 4 (US3 - Partial Failure)**: Depends on Phase 2 (batch result processing must exist). Can run in parallel with Phase 3.
- **Phase 5 (US4 - Visibility)**: No dependencies on other user stories — can run in parallel with Phase 2+
- **Phase 6 (Polish)**: Depends on all previous phases

### User Story Dependencies

- **US1 (Batch Estimation)**: Depends only on foundational adapter work
- **US2 (Fallback)**: Depends on US1 batch path existing (adds error handling to it)
- **US3 (Partial Failure)**: Depends on US1 batch result mapping (adds per-resource error handling)
- **US4 (Visibility)**: Independent — existing infrastructure already works

### Within Each Phase

- Tests MUST be written and FAIL before implementation (TDD)
- Types/helpers before integration
- Unit tests before integration tests
- `make test && make lint` after each phase checkpoint

### Parallel Opportunities

- T001, T002, T003 can run in parallel (different test functions)
- T011, T012, T013 can run in parallel (different test functions)
- T017 can run in parallel with T011-T016 (types file vs test file)
- T024, T025 can run in parallel (different test scenarios)
- T031, T032 can run in parallel (different test scenarios)
- Phase 5 (US4) can run in parallel with Phases 2-4

---

## Parallel Example: Phase 2 (User Story 1)

```bash
# Launch all US1 tests in parallel (different test functions, same file):
Task T011: "chunkResources tests in internal/engine/engine_batch_test.go"
Task T012: "groupResourcesByPlugin tests in internal/engine/engine_batch_test.go"
Task T013: "executeBatchForPlugin tests in internal/engine/engine_batch_test.go"

# Then launch types + helpers (T017 can overlap with test writing):
Task T017: "Create engine_batch.go with internal types"
Task T018: "Implement chunkResources"
Task T019: "Implement groupResourcesByPlugin"

# Sequential: integration depends on helpers
Task T022: "Integrate batch path into GetProjectedCost" (depends on T018-T021)
Task T023: "Integrate batch path into GetActualCostWithOptions" (depends on T022 pattern)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational — adapter interface + mapping
2. Complete Phase 2: US1 — batch estimation (projected + actual)
3. **STOP and VALIDATE**: Test batch path with mock plugins, verify cache integration
4. At this point, batch works for the happy path. Deploy/demo if ready.

### Incremental Delivery

1. Phase 1 (Foundational) → Adapter ready
2. Phase 2 (US1) → Batch works for happy path → MVP!
3. Phase 3 (US2) → Fallback for non-batch plugins and errors → Production-ready
4. Phase 4 (US3) → Partial failure handling → Robust
5. Phase 5 (US4) → Visibility → Complete
6. Phase 6 (Polish) → Quality gates → Ship

### Critical Path

```text
T004-T010 (Adapter) → T017-T023 (US1 Core) → T027-T030 (US2 Fallback) → T040-T044 (Polish)
                                              → T035-T037 (US3 Partial)  ↗
```

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Tests MUST fail before implementing (TDD per Constitution Principle II)
- `make test && make lint` after each phase checkpoint
- No TUI changes → no golden file tests needed
- Plugin list display works automatically — US4 is verification only
- `batchProcessingThreshold = 100` already exists at `engine.go:47` — reuse it
