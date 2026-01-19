# Tasks: Latest Plugin Version Selection

**Feature**: `001-latest-plugin-version`
**Status**: Planned
**Input**: [Feature Spec](specs/001-latest-plugin-version/spec.md)

## Phase 1: Setup

**Goal**: Initialize environment and verify project state.

- [x] T001 Verify project structure and dependencies in `go.mod`.

## Phase 2: Foundational

**Goal**: Ensure `internal/registry` package is ready for enhancements.

- [x] T002 Ensure `internal/registry` package is buildable and lint-free.

## Phase 3: User Story 1 - Automated Cost Analysis Uses Latest Plugin Version

**Goal**: Ensure system automatically selects the latest version of installed plugins using SemVer.
**Priority**: P1
**Independent Test**: `TestListLatestPlugins` in `internal/registry/registry_test.go` passes for all scenarios.

### Tests

- [x] T003 [P] [US1] Create test helpers for generating plugin directories with various version scenarios (pre-release, invalid, etc.) in `internal/registry/registry_test.go`.
- [x] T004 [P] [US1] Implement `TestListLatestPlugins` with table-driven cases for standard scenarios (single/multiple versions) in `internal/registry/registry_test.go`.
- [x] T005 [P] [US1] Implement `TestListLatestPlugins` cases for edge cases (invalid versions, pre-releases, corrupted dirs) in `internal/registry/registry_test.go`.
- [x] T006 [P] [US1] Add unit tests for file system failure modes (permission denied, non-existent paths) and binary validation in `internal/registry/registry_test.go`.
- [x] T013 [P] [US1] Add unit test for concurrent access safety (FR-009) in `internal/registry/registry_test.go`.
- [x] T007 [P] [US1] Add unit test for `Open` method logging warnings from `ListLatestPlugins` in `internal/registry/registry_test.go`.

### Implementation

- [x] T008 [US1] Run tests and refine `ListLatestPlugins` implementation in `internal/registry/registry.go` to pass all checks (fix any SemVer precedence or error handling bugs).
- [x] T014 [US1] Ensure `ListLatestPlugins` gracefully handles file system errors (e.g. permission denied) as per FR-005.
- [x] T015 [US1] Verify `Open` method in `internal/registry/registry.go` correctly integrates `ListLatestPlugins` and handles returned warnings.

## Phase 4: User Story 2 - View All Installed Plugin Versions

**Goal**: Ensure users can list all installed versions of each plugin.
**Priority**: P2
**Independent Test**: `finfocus plugin list` displays all versions of installed plugins.

### Tests

- [x] T009 [P] [US2] Review and enhance `TestListPlugins` in `internal/registry/registry_test.go` to explicitly verify that _all_ versions are returned (not just latest).

### Implementation

- [x] T010 [US2] Verify `plugin list` command in `internal/cli/plugin_list.go` correctly displays multiple versions (ensure no regression from US1 changes).

## Phase 5: Polish & Cross-Cutting

**Goal**: Finalize documentation and code quality.

- [x] T011 Update documentation for plugin versioning behavior in `docs/plugin-system.md` (or equivalent).
- [x] T012 Run full test suite and linting to ensure no regressions.

## Dependencies

1. **US1 Completion**: T003 -> T004, T005, T006, T013 -> T008, T014 -> T015
2. **US2 Completion**: T009 -> T010 (can run parallel to US1)

## Parallel Execution Opportunities

- T003, T004, T005, T006, T007, T013 (Test creation) can be drafted in parallel.
- US1 and US2 streams are largely independent as US2 relies on existing `ListPlugins` while US1 focuses on `ListLatestPlugins`.

## Implementation Strategy

1.  **Retroactive TDD**: We will first implement the missing tests for `ListLatestPlugins` (T003-T005) to assert the current behavior and identify any gaps in the existing implementation (T007).
2.  **Integration**: We will verify the `Open` method's integration (T008) to ensure the core engine uses the logic verified in step 1.
3.  **Verification**: We will confirm US2 behavior (T009-T010) remains correct (listing all versions).
