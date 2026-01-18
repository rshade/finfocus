# Tasks: v0.2.1 Developer Experience Improvements

**Branch**: `115-v021-dx-improvements` | **Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Overview

This task list guides the implementation of developer experience improvements including parallel plugin listing, compatibility checks, cleaner upgrades, and consistent filtering.

**Key Goals**:
- 5x speedup for `plugin list` via concurrency
- Automated plugin compatibility validation
- Disk space cleanup for plugin upgrades
- Zero duplication in filter logic

## Dependencies

- **Flow**: Setup -> Foundation -> US1 -> US2 -> US3 -> US4 -> Polish
- **Critical Path**: `GetPluginInfo` implementation -> Plugin List refactor
- **Parallelism**: US3 and US4 are independent of US1/US2 and can be built in parallel after Foundation.

---

## Phase 1: Setup

**Goal**: Ensure dependencies and environment are ready for new feature development.

- [x] T001 Verify `finfocus-spec` dependency is v0.5.1 in `go.mod`

---

## Phase 2: Foundational

**Goal**: Implement core shared logic required by multiple user stories.

- [x] T002 Implement `GetPluginInfo` RPC method in `internal/pluginhost/client.go` (EXISTING)
- [x] T003 Create `internal/cli/filters.go` with `ApplyFilters` helper function

---

## Phase 3: User Story 1 - Fast Plugin Listing

**Goal**: Parallelize plugin metadata fetching to improve CLI responsiveness.
**Independent Test**: `plugin list` completes in O(1) time relative to plugin count (vs O(N)).

### Implementation
- [x] T004 [US1] Refactor `internal/cli/plugin_list.go` to use `errgroup` for fetching
- [x] T005 [US1] Implement concurrency limit (runtime.NumCPU) in `internal/cli/plugin_list.go`
- [x] T006 [US1] Ensure deterministic output sorting in `internal/cli/plugin_list.go`

### Tests
- [x] T007 [US1] Create benchmark test for plugin listing in `internal/cli/plugin_list_test.go`

---

## Phase 4: User Story 2 - Plugin Compatibility

**Goal**: Expose plugin spec versions and handle legacy plugins gracefully.
**Independent Test**: `plugin list` shows "Spec Version" column; legacy plugins show "Legacy".

### Implementation
- [x] T008 [US2] Update `internal/cli/plugin_list.go` to display "Spec Version" column (EXISTING)
- [x] T009 [US2] Implement "Legacy" fallback for `Unimplemented` RPC errors in `internal/pluginhost/client.go` (EXISTING)
- [x] T010 [US2] Add warning log for spec version mismatch in `internal/pluginhost/client.go` (Permissive mode) (EXISTING)
- [x] T011 [US2] Implement Strict Mode check (configurable) in `internal/pluginhost/client.go`

### Tests
- [x] T012 [US2] Add unit tests for legacy/error handling in `internal/pluginhost/client_test.go`

---

## Phase 5: User Story 3 - Automatic Cleanup

**Goal**: Allow users to remove old plugin versions automatically during upgrade.
**Independent Test**: `plugin install --clean` leaves only the new version on disk.

### Implementation
- [x] T013 [US3] Add `RemoveOtherVersions` method to `internal/registry/installer.go`
- [x] T014 [US3] Add `--clean` flag to `internal/cli/plugin_install.go`
- [x] T015 [US3] Wire up cleanup logic to execute ONLY after successful install in `internal/cli/plugin_install.go`

### Tests
- [x] T016 [US3] Create integration test for cleanup behavior in `internal/registry/installer_test.go`

---

## Phase 6: User Story 4 - Consistent Cost Filtering

**Goal**: Unify filter logic across all cost commands.
**Independent Test**: `cost actual` and `cost projected` accept identical filter syntax with identical results.

### Implementation
- [x] T017 [US4] Refactor `internal/cli/cost_actual.go` to use `ApplyFilters`
- [x] T018 [US4] Refactor `internal/cli/cost_projected.go` to use `ApplyFilters`

### Tests
- [x] T019 [US4] Verify filter consistency in `internal/cli/filters_test.go` (unit tests cover shared behavior)

---

## Phase 7: Polish

**Goal**: Finalize documentation and verify release readiness.

- [x] T020 Update `docs/reference/cli-commands.md` with `--clean` flag documentation
- [x] T021 Run `make lint` and `make test` to ensure Constitution compliance
- [x] T022 Manual verify: All tests pass with race detection enabled

## Implementation Strategy

1. **Foundational**: We start by upgrading the spec dependency and implementing the new RPC client method, as this unblocks US1 and US2.
2. **Parallelism**: We then implement the `errgroup` logic (US1) which changes the structure of the list command.
3. **Compatibility**: Once the structure is parallel, we enrich the data with `GetPluginInfo` results (US2).
4. **Independent Tracks**: US3 (Install cleanup) and US4 (Filter refactoring) touch different files and can be implemented in any order relative to the list command changes.
