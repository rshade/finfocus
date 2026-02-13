# Tasks: TUI Detail View Recommendations

**Input**: Design documents from `specs/510-tui-detail-recommendations/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md
**GitHub Issue**: #575

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must maintain
minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all
tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are
strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity),
documentation MUST be updated concurrently with implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation
and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Data Model Extension)

**Purpose**: Extend the `Recommendation` type with the `Reasoning` field across the
proto adapter and engine layers. This is foundational for all user stories.

- [x] T001 [P] Add `Reasoning []string` field to `proto.Recommendation` struct in
  `internal/proto/adapter.go` (line ~381-405) with godoc comment explaining it carries
  plugin-provided warnings and caveats mapped from proto `Recommendation.Reasoning`
  (field 14)
- [x] T002 [P] Add `Reasoning []string` field with `json:"reasoning,omitempty"` tag
  to `engine.Recommendation` struct in `internal/engine/types.go` (line ~134-157) with
  godoc comment
- [x] T003 Map `Reasoning` in proto-to-internal conversion: add
  `protoRec.Reasoning = rec.GetReasoning()` in `GetRecommendations` method in
  `internal/proto/adapter.go` (line ~862-888, after line 868)
- [x] T004 Map `Reasoning` in internal-to-engine conversion: add
  `engineRec.Reasoning = rec.Reasoning` in `convertProtoRecommendation` function in
  `internal/engine/engine.go` (line ~2635-2652)

- [x] T005 Write test `TestConvertProtoRecommendationReasoning` in
  `internal/engine/engine_test.go` verifying: `convertProtoRecommendation` copies
  `Reasoning` field from `proto.Recommendation` to `engine.Recommendation`, empty
  `Reasoning` slice produces nil/empty on engine side, multi-entry `Reasoning` slice
  is preserved in order

**Checkpoint**: `Reasoning` field flows from proto through adapter through engine.
Run `make test` to verify no regressions.

---

## Phase 2: Foundational (CLI Fetch-and-Merge Helper)

**Purpose**: Create the shared helper that fetches recommendations and merges them
into cost results. This MUST be complete before any user story work.

- [x] T006 Write test `TestFetchAndMergeRecommendations` in
  `internal/cli/common_execution_test.go` covering: successful merge by ResourceID,
  empty recommendations (no-op), fetch error (logs warning, returns gracefully),
  multiple resources with partial recommendation coverage
- [x] T007 Implement `fetchAndMergeRecommendations(ctx, eng, resources, results)`
  helper function in `internal/cli/common_execution.go` that: calls
  `eng.GetRecommendationsForResources`, builds ResourceID lookup map, merges
  recommendations into matching `CostResult` entries, logs failures at WARN level
  without propagating errors (FR-006)

**Checkpoint**: Helper function exists with tests passing. Run `go test ./internal/cli/...`

---

## Phase 3: User Story 1 - View Recommendations in Projected Cost Detail (P1)

**Goal**: Users running `cost projected` in interactive mode see a RECOMMENDATIONS
section in the detail view showing action type, description, savings (sorted by
savings descending), and reasoning/caveats.

**Independent Test**: Run `cost projected` with a plan that produces resources with
recommendations, press Enter on a resource, verify the RECOMMENDATIONS section
renders with correct content and ordering.

### Tests for User Story 1

- [x] T008 [P] [US1] Write test `TestRenderRecommendationsSection` in
  `internal/tui/cost_view_test.go` covering: single recommendation with savings
  renders `[TYPE] description ($X.XX USD/mo savings)`, recommendation without savings
  renders `[TYPE] description` only, multiple recommendations sorted by savings
  descending, recommendations with reasoning display indented warning lines, empty
  recommendations slice produces no output, default currency USD when currency field
  is empty, unrecognized action type rendered as-is in brackets (e.g.,
  `[UNKNOWN_TYPE]`), 10+ recommendations all render without truncation
- [x] T009 [P] [US1] Write test `TestRenderDetailViewWithRecommendations` in
  `internal/tui/cost_view_test.go` verifying: RECOMMENDATIONS section appears after
  SUSTAINABILITY and before NOTES in full `RenderDetailView` output, section is
  absent when `CostResult.Recommendations` is nil

### Implementation for User Story 1

- [x] T010 [US1] Implement `renderRecommendationsSection(content *strings.Builder,
  recommendations []engine.Recommendation)` in `internal/tui/cost_view.go` that:
  returns immediately if recommendations is empty (FR-005), copies and sorts
  recommendations by `EstimatedSavings` descending using `sort.SliceStable` (FR-009),
  renders header with `HeaderStyle.Render("RECOMMENDATIONS")`, renders each
  recommendation as `- [TYPE] Description ($X.XX Currency/mo savings)` with savings
  only when `EstimatedSavings > 0` (FR-003/FR-004), renders each `Reasoning` entry
  as indented line with `WarningStyle` (FR-002), defaults currency to "USD" when empty
- [x] T011 [US1] Update `RenderDetailView` in `internal/tui/cost_view.go` (line ~344)
  to call `renderRecommendationsSection(&content, resource.Recommendations)` between
  `renderSustainabilitySection` call and the `Notes` section (FR-008)
- [x] T012 [US1] Wire `fetchAndMergeRecommendations` into `executeCostProjected` in
  `internal/cli/cost_projected.go`: call after
  `eng.GetProjectedCostWithErrors(ctx, resources)` returns and before
  `RenderCostOutput()` is called, passing `ctx`, `eng`, `resources`, and
  `resultWithErrors.Results`

**Checkpoint**: `cost projected` in interactive mode shows RECOMMENDATIONS in detail
view. Run `go test ./internal/tui/... ./internal/cli/...`

---

## Phase 4: User Story 2 - View Recommendations in Actual Cost Detail (P1)

**Goal**: Users running `cost actual` in interactive mode see recommendations in the
same format and position as projected cost, providing consistent UX across commands.

**Independent Test**: Run `cost actual` with resources that have recommendations,
press Enter on a resource, verify the RECOMMENDATIONS section appears identically
to projected cost detail view.

### Tests for User Story 2

- [x] T013 [US2] Write test in `internal/cli/cost_actual_test.go` verifying that
  `executeCostActual` calls `fetchAndMergeRecommendations` and recommendations
  appear in the results passed to `RenderActualCostOutput`

### Implementation for User Story 2

- [x] T014 [US2] Wire `fetchAndMergeRecommendations` into `executeCostActual` in
  `internal/cli/cost_actual.go`: call after
  `eng.GetActualCostWithOptionsAndErrors(ctx, request)` returns and before
  `RenderActualCostOutput()` is called, passing `ctx`, `eng`, `resources`, and
  `resultWithErrors.Results`

**Checkpoint**: `cost actual` in interactive mode shows RECOMMENDATIONS identically
to projected. Run `go test ./internal/cli/...`

---

## Phase 5: User Story 3 - Graceful Absence of Recommendations (P2)

**Goal**: When no recommendations exist, the detail view renders normally without any
empty RECOMMENDATIONS section or visual gaps.

**Independent Test**: View a resource with no recommendations and verify no
RECOMMENDATIONS header appears.

### Tests for User Story 3

- [x] T015 [US3] Write test `TestRenderDetailViewNoRecommendations` in
  `internal/tui/cost_view_test.go` verifying: `RenderDetailView` with nil
  `Recommendations` field produces output containing no "RECOMMENDATIONS" string,
  `RenderDetailView` with empty `[]engine.Recommendation{}` also produces no
  RECOMMENDATIONS section, all other sections (RESOURCE DETAIL, BREAKDOWN,
  SUSTAINABILITY, NOTES) render normally

### Implementation for User Story 3

No implementation needed beyond T010 (which returns early on empty recommendations).
This test validates the guard clause works end-to-end through `RenderDetailView`.

**Checkpoint**: Detail view clean when no recommendations. Run
`go test ./internal/tui/...`

---

## Phase 6: User Story 4 - Machine-Readable Output Includes Recommendations (P3)

**Goal**: JSON and NDJSON output includes the `recommendations` array for resources
that have them, and omits the field entirely when empty.

**Independent Test**: Run `cost projected --output json` and verify JSON output
includes `recommendations` array with correct fields for resources with
recommendations, and omits the field for resources without.

### Tests for User Story 4

- [x] T016 [US4] Write test `TestCostResultJSONRecommendations` in
  `internal/engine/types_test.go` verifying: `json.Marshal` of `CostResult` with
  populated `Recommendations` includes `"recommendations"` array with `type`,
  `description`, `estimatedSavings`, `currency`, and `reasoning` fields;
  `json.Marshal` of `CostResult` with nil `Recommendations` omits the
  `"recommendations"` key entirely (verified via `!strings.Contains`)

### Implementation for User Story 4

No code changes needed. The existing `json:"recommendations,omitempty"` tag on
`CostResult.Recommendations` and `json:"reasoning,omitempty"` on
`Recommendation.Reasoning` (added in T002) handle serialization automatically.
The `fetchAndMergeRecommendations` call (T012/T014) populates the field before
any output rendering occurs, so JSON/NDJSON formats inherit the data.

**Checkpoint**: JSON/NDJSON output verified. Run `go test ./internal/cli/...`

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Quality gates, documentation, and final validation.

- [x] T017 [P] Run `make test` and verify all tests pass with no regressions
- [x] T018 [P] Run `make lint` and fix any linting issues in modified files
- [x] T019 [P] Verify test coverage on new code meets 80% minimum: run
  `go test -coverprofile=coverage.out ./internal/tui/... ./internal/cli/...
  ./internal/engine/... ./internal/proto/...` and check coverage of modified functions
- [x] T020 Validate quickstart scenario: build binary with `make build`, run
  `./bin/finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json`
  in interactive mode, press Enter on a resource, confirm detail view renders
  correctly (with or without recommendations depending on plugin availability)

---

## Dependencies and Execution Order

### Phase Dependencies

```text
Phase 1 (Setup) ──→ Phase 2 (Foundational) ──→ Phase 3 (US1) ──→ Phase 7 (Polish)
                                             ├─→ Phase 4 (US2) ──→ Phase 7
                                             ├─→ Phase 5 (US3) ──→ Phase 7
                                             └─→ Phase 6 (US4) ──→ Phase 7
```

- **Phase 1 (Setup)**: No dependencies - start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 (needs Reasoning field in types)
- **Phases 3-6 (User Stories)**: All depend on Phase 2 completion
  - US1 (Phase 3): Implements core rendering - recommended first
  - US2 (Phase 4): Can run in parallel with US1 (different file: cost_actual.go)
  - US3 (Phase 5): Depends on US1 (validates T010 guard clause)
  - US4 (Phase 6): Can run in parallel with US1 (test-only, no code changes)
- **Phase 7 (Polish)**: Depends on all user stories complete

### Within Each Phase

- Tests MUST be written and FAIL before implementation (TDD)
- Tasks marked [P] within a phase can run in parallel
- Tasks without [P] must run sequentially in listed order

### Parallel Opportunities

```text
Phase 1: T001 ║ T002 (parallel - different files)
              then T003 + T004 (sequential - depend on T001/T002)
              then T005 (TDD: conversion pipeline test)

Phase 2: T006 then T007 (TDD: test first)

Phase 3: T008 ║ T009 (parallel tests)
              then T010 → T011 → T012 (sequential implementation)

Phase 4-6: Can run in parallel with each other after Phase 2
           (US2 is a single file change, US3/US4 are test-only)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Data model extension + conversion test (T001-T005)
2. Complete Phase 2: CLI helper (T006-T007)
3. Complete Phase 3: US1 projected cost detail view (T008-T012)
4. **STOP and VALIDATE**: Test US1 independently with `make test && make lint`
5. This delivers the core value: recommendations in projected cost detail view

### Incremental Delivery

1. Phase 1 + 2 → Foundation ready
2. Add US1 (Phase 3) → Test → MVP delivers projected cost recommendations
3. Add US2 (Phase 4) → Test → Actual cost parity achieved
4. Add US3 (Phase 5) → Test → Empty state validated
5. Add US4 (Phase 6) → Test → JSON/NDJSON output verified
6. Phase 7 → Quality gates pass → Ready for PR

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US3 and US4 require no new implementation code - they validate existing behavior
  through tests
- The `Reasoning` field addition (Phase 1) is backward compatible: plugins that
  don't populate it return empty slices, which are handled gracefully
- Total new/modified production code files: 7 (engine/types.go, proto/adapter.go,
  engine/engine.go, cli/common_execution.go, tui/cost_view.go,
  cli/cost_projected.go, cli/cost_actual.go)
