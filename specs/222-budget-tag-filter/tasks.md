# Tasks: Tag-Based Budget Filtering

**Input**: Design documents from `/specs/222-budget-tag-filter/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go project**: `internal/` for packages, `test/` for tests
- All paths are relative to repository root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Extend core data structures for tag filtering

- [X] T001 Add `Tags map[string]string` field to `BudgetFilterOptions` struct in `internal/engine/budget.go`
- [X] T002 Add `matchesBudgetTagsWithGlob()` helper function with `path.Match()` support in `internal/engine/budget.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core filtering logic that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Implement `FilterBudgetsByTags()` function applying AND logic for tag matching in `internal/engine/budget.go`
- [X] T004 Update `GetBudgets()` to apply tag filtering via `BudgetFilterOptions.Tags` in `internal/engine/budget.go`
- [X] T005 Create `parseBudgetFilters()` function to parse `--filter` flags into `BudgetFilterOptions` in `internal/cli/filters.go`
- [X] T006 Create `validateBudgetFilter()` function to validate filter syntax (FR-010) in `internal/cli/filters.go`

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Filter Budgets by Namespace (Priority: P1) 🎯 MVP

**Goal**: Enable filtering budgets by exact metadata tag match (e.g., `--filter "tag:namespace=production"`)

**Independent Test**: Filter with `--filter "tag:namespace=production"` returns only budgets with matching metadata

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T007 [P] [US1] Unit test for exact tag matching in `internal/engine/budget_test.go` (test `matchesBudgetTagsWithGlob` with exact values)
- [X] T008 [P] [US1] Unit test for missing tag key exclusion (budget lacks required key) in `internal/engine/budget_test.go`
- [X] T009 [P] [US1] Unit test for empty result set when no budgets match in `internal/engine/budget_test.go`
- [X] T010 [P] [US1] Unit test for `parseBudgetFilters()` with valid `tag:key=value` syntax in `internal/cli/filters_test.go`
- [X] T011 [P] [US1] Unit test for empty Tags map returning all budgets in `internal/engine/budget_test.go`

### Implementation for User Story 1

- [X] T012 [US1] Wire `--filter` flag to `GetBudgets()` call site - add filter flag to budget-related CLI commands in `internal/cli/` (research which command exposes budget listing first)
- [X] T013 [US1] Verify backward compatibility: existing provider-only filtering unchanged in `internal/engine/budget_test.go`
- [X] T014 [US1] Add debug logging for tag filtering operations in `internal/engine/budget.go`

**Checkpoint**: User Story 1 complete - exact tag matching works end-to-end

---

## Phase 4: User Story 2 - Filter Budgets with Glob Patterns (Priority: P2)

**Goal**: Support wildcard patterns in tag values (e.g., `--filter "tag:namespace=prod-*"`)

**Independent Test**: Filter with `--filter "tag:namespace=prod-*"` matches `prod-us`, `prod-eu` but excludes `staging`

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [X] T015 [P] [US2] Unit test for prefix glob pattern (`prod-*` matches `prod-us`) in `internal/engine/budget_test.go`
- [X] T016 [P] [US2] Unit test for suffix glob pattern (`*-production` matches `team-a-production`) in `internal/engine/budget_test.go`
- [X] T017 [P] [US2] Unit test for both-ends glob pattern (`*prod*` matches `production`) in `internal/engine/budget_test.go`
- [X] T018 [P] [US2] Unit test for glob pattern with no matches in `internal/engine/budget_test.go`

### Implementation for User Story 2

- [X] T019 [US2] Ensure `matchesBudgetTagsWithGlob()` uses `path.Match()` correctly for all glob patterns in `internal/engine/budget.go`
- [X] T020 [US2] Handle `path.Match()` error cases (invalid pattern syntax) gracefully in `internal/engine/budget.go`

**Checkpoint**: User Story 2 complete - glob pattern matching works

---

## Phase 5: User Story 3 - Combine Multiple Tag Filters (Priority: P2)

**Goal**: Support multiple `--filter` flags with AND logic (all tags must match)

**Independent Test**: `--filter "tag:namespace=prod" --filter "tag:cluster=us-east-1"` returns only budgets matching BOTH

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [X] T021 [P] [US3] Unit test for multiple tags with AND logic (both match) in `internal/engine/budget_test.go`
- [X] T022 [P] [US3] Unit test for multiple tags with partial match (one fails) excluded in `internal/engine/budget_test.go`
- [X] T023 [P] [US3] Unit test for `parseBudgetFilters()` with multiple `--filter` flags in `internal/cli/filters_test.go`

### Implementation for User Story 3

- [X] T024 [US3] Verify `FilterBudgetsByTags()` iterates all tags and short-circuits on first mismatch in `internal/engine/budget.go`
- [X] T025 [US3] Handle duplicate tag keys (later value overwrites earlier) in `internal/cli/filters.go`

**Checkpoint**: User Story 3 complete - multiple tag filters work with AND logic

---

## Phase 6: User Story 4 - Combine Provider and Tag Filters (Priority: P3)

**Goal**: Provider filters (OR) combined with tag filters (AND) work together

**Independent Test**: `--filter "provider=kubecost" --filter "tag:namespace=staging"` returns kubecost budgets in staging only

### Tests for User Story 4 (MANDATORY - TDD Required) ⚠️

- [X] T026 [P] [US4] Unit test for combined provider and tag filtering in `internal/engine/budget_test.go`
- [X] T027 [P] [US4] Unit test for `parseBudgetFilters()` with mixed `provider=` and `tag:` filters in `internal/cli/filters_test.go`

### Implementation for User Story 4

- [X] T028 [US4] Ensure `GetBudgets()` applies provider filter (OR) then tag filter (AND) in correct order in `internal/engine/budget.go`
- [X] T029 [US4] Verify filter logic: provider OR first, then tag AND on provider-matched set in `internal/engine/budget.go`

**Checkpoint**: User Story 4 complete - combined filtering works

---

## Phase 7: Edge Cases & Validation

**Purpose**: Handle edge cases and malformed input per FR-010

### Tests for Edge Cases (MANDATORY) ⚠️

- [X] T030 [P] Unit test for tag key with special characters (`kubernetes.io/name`) in `internal/engine/budget_test.go`
- [X] T031 [P] Unit test for empty tag value (`tag:namespace=`) matching empty metadata in `internal/engine/budget_test.go`
- [X] T032 [P] Unit test for case-sensitive matching (tag keys and values) in `internal/engine/budget_test.go`
- [X] T033 [P] Unit test for malformed filter syntax (`tag:namespace` missing `=`) returning error in `internal/cli/filters_test.go`
- [X] T034 [P] Unit test for empty key after `tag:` (`tag:=value`) returning error in `internal/cli/filters_test.go`

### Implementation for Edge Cases

- [X] T035 Implement error handling for malformed `tag:key=value` syntax in `internal/cli/filters.go`
- [X] T036 Ensure case-sensitive comparison in `matchesBudgetTagsWithGlob()` in `internal/engine/budget.go`

**Checkpoint**: Edge cases handled - robust error handling in place

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, coverage, and final validation

- [X] T037 [P] Update `internal/engine/CLAUDE.md` with tag filtering documentation
- [X] T038 [P] Add tag filtering examples to `specs/222-budget-tag-filter/quickstart.md` validation
- [X] T039 Run `make lint` and fix any linting issues
- [X] T040 Run `make test` and verify all tests pass with 90%+ coverage for new code
- [X] T041 Integration test: CLI end-to-end tag filtering in `test/integration/budget_tag_filter_test.go`
- [X] T042 Verify backward compatibility: existing tests still pass
- [X] T043 [P] Add Godoc comments to all new exported functions (`matchesBudgetTagsWithGlob`, `FilterBudgetsByTags`, `parseBudgetFilters`, `validateBudgetFilter`) per Constitution Principle IV
- [X] T044 Verify SC-006: tag filtering reduces visible budgets by 80%+ in multi-tenant test scenario in `test/integration/budget_tag_filter_test.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 - BLOCKS all user stories
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion
  - US1 (P1): Can start after Phase 2
  - US2 (P2): Can start after Phase 2 (independent of US1)
  - US3 (P2): Can start after Phase 2 (independent of US1/US2)
  - US4 (P3): Can start after Phase 2 (may integrate US1-US3 concepts)
- **Edge Cases (Phase 7)**: Depends on Phase 2, can run in parallel with user stories
- **Polish (Phase 8)**: Depends on all previous phases

### User Story Dependencies

- **User Story 1 (P1)**: Foundation only - No dependencies on other stories
- **User Story 2 (P2)**: Foundation only - Extends US1 pattern matching but independently testable
- **User Story 3 (P2)**: Foundation only - Extends US1 multi-filter but independently testable
- **User Story 4 (P3)**: Foundation only - Combines US1 concepts but independently testable

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Core logic before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks (T001-T002) run sequentially (same file)
- Foundational tests (T005-T006) can run in parallel (different files)
- All US tests marked [P] can run in parallel
- Edge case tests (T030-T034) can run in parallel
- Polish documentation tasks (T037-T038) can run in parallel

---

## Parallel Example: Phase 2 (Foundational)

```bash
# After T003-T004 complete (same file, sequential):
# Launch parallel tasks:
Task: "T005 [P] Create parseBudgetFilters() in internal/cli/filters.go"
Task: "T006 [P] Create validateBudgetFilter() in internal/cli/filters.go"
```

## Parallel Example: User Story 1 Tests

```bash
# Launch all US1 tests in parallel:
Task: "T007 [P] [US1] Unit test for exact tag matching"
Task: "T008 [P] [US1] Unit test for missing tag key exclusion"
Task: "T009 [P] [US1] Unit test for empty result set"
Task: "T010 [P] [US1] Unit test for parseBudgetFilters()"
Task: "T011 [P] [US1] Unit test for empty Tags map"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T006)
3. Complete Phase 3: User Story 1 (T007-T014)
4. **STOP and VALIDATE**: Test exact tag filtering works end-to-end
5. Run `make lint && make test` to validate

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → **MVP Complete!**
3. Add User Story 2 → Test glob patterns → Enhanced capability
4. Add User Story 3 → Test multi-filter → Power user feature
5. Add User Story 4 → Test combined filters → Full feature set
6. Add Edge Cases → Robust error handling
7. Polish → Documentation and coverage

### Suggested Execution

For a single developer:

1. T001 → T002 → T003 → T004 (sequential, same file)
2. T005 + T006 (parallel, filters.go)
3. T007-T011 (parallel tests for US1)
4. T012-T014 (US1 implementation)
5. T015-T020 (US2 tests + implementation)
6. T021-T025 (US3 tests + implementation)
7. T026-T029 (US4 tests + implementation)
8. T030-T036 (edge cases)
9. T037-T042 (polish)

---

## Notes

- All paths relative to repository root
- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story independently completable and testable
- Verify tests fail before implementing
- Run `make lint && make test` after each phase
- Stop at any checkpoint to validate incrementally
