# Tasks: GreenOps Impact Equivalencies

**Input**: Design documents from `/specs/125-greenops-equivalencies/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV, documentation (README, docs/) MUST be updated concurrently with implementation to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Source code**: `internal/` at repository root (Go project)
- **Tests**: Colocated with source (`*_test.go`) per Go conventions
- **Integration tests**: `test/integration/`
- **Documentation**: `docs/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the greenops package structure and shared types

- [x] T001 Create internal/greenops/ package directory structure
- [x] T002 [P] Define EquivalencyType enum and constants in internal/greenops/types.go
- [x] T003 [P] Define CarbonInput, EquivalencyResult, EquivalencyOutput structs in internal/greenops/types.go
- [x] T004 [P] Define EPA formula constants with source comments in internal/greenops/constants.go
- [x] T005 [P] Define unit conversion constants in internal/greenops/constants.go
- [x] T006 [P] Define display threshold constants in internal/greenops/constants.go
- [x] T007 [P] Define error types (ErrInvalidUnit, ErrNegativeValue, ErrNoCarbon) in internal/greenops/errors.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core calculation and formatting utilities that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational (MANDATORY - TDD Required) ⚠️

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T008 [P] Unit tests for NormalizeToKg() function in internal/greenops/normalizer_test.go
- [x] T009 [P] Unit tests for FormatNumber/FormatFloat/FormatLarge() in internal/greenops/formatter_test.go
- [x] T010 [P] Unit tests for Calculate() function with EPA formula verification (1% margin) in internal/greenops/equivalency_test.go. MUST include reference values: 150 kg → 781.25 miles (150/0.192), 150 kg → 18248.18 smartphones (150/0.00822)
- [x] T011 [P] Unit tests for CalculateFromMap() with canonical/legacy key handling in internal/greenops/equivalency_test.go
- [x] T012 [P] Unit tests for edge cases (zero, negative, below threshold, very large) in internal/greenops/equivalency_test.go

### Implementation for Foundational

- [x] T013 Implement NormalizeToKg() with unit conversion table in internal/greenops/normalizer.go. Conversions: g→0.001, kg→1.0, t→1000.0, lb→0.453592; also gCO2e, kgCO2e, tCO2e, lbCO2e variants
- [x] T014 Implement IsRecognizedUnit() validation in internal/greenops/normalizer.go
- [x] T015 Implement FormatNumber() with golang.org/x/text/message in internal/greenops/formatter.go
- [x] T016 Implement FormatFloat() with precision control in internal/greenops/formatter.go
- [x] T017 Implement FormatLarge() with million/billion scaling in internal/greenops/formatter.go
- [x] T018 Implement Calculate() EPA formula calculations in internal/greenops/equivalency.go
- [x] T019 Implement CalculateFromMap() with carbon_footprint/gCO2e key detection in internal/greenops/equivalency.go. Log deprecation warning when legacy "gCO2e" key is used: `deprecated key 'gCO2e' used, prefer 'carbon_footprint'`
- [x] T020 Implement DisplayText and CompactText formatting in internal/greenops/equivalency.go

**Checkpoint**: Foundation ready - greenops package is fully functional. User story integration can now begin.

---

## Phase 3: User Story 1 - View Carbon Equivalencies in CLI Table Output (Priority: P1) 🎯 MVP

**Goal**: Display real-world equivalencies (miles driven, smartphones charged) in CLI summary output when carbon data is present

**Independent Test**: Run `finfocus cost projected --pulumi-json plan.json` with a plan containing carbon-emitting resources and verify the summary includes equivalency text like "Equivalent to driving ~781 miles or charging ~18,248 smartphones"

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

- [x] T021 [P] [US1] Unit test for renderSustainabilitySummary() equivalency display in internal/engine/project_test.go
- [x] T022 [P] [US1] Integration test for CLI equivalency output with sample plan in test/integration/greenops_cli_test.go
- [x] T023 [P] [US1] Test graceful omission when carbon emissions are zero in test/integration/greenops_cli_test.go

### Implementation for User Story 1

- [x] T024 [US1] Import greenops package in internal/engine/project.go
- [x] T025 [US1] Modify renderSustainabilitySummary() to call greenops.Calculate() in internal/engine/project.go
- [x] T026 [US1] Add equivalency DisplayText output after carbon_footprint line in internal/engine/project.go
- [x] T027 [US1] Add logging for equivalency calculation (debug level) in internal/engine/project.go

**Checkpoint**: User Story 1 complete. CLI users see carbon equivalencies in cost projected output.

---

## Phase 4: User Story 2 - View Carbon Equivalencies in TUI Summary (Priority: P2)

**Goal**: Display consistent carbon equivalencies in the TUI summary view with proper styling

**Independent Test**: Launch TUI with cost data containing carbon metrics and verify the summary panel includes styled equivalency text

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [x] T028 [P] [US2] Unit test for RenderCostSummary() equivalency display in internal/tui/cost_view_test.go
- [x] T029 [P] [US2] Test TUI styling consistency for equivalency text in internal/tui/cost_view_test.go

### Implementation for User Story 2

- [x] T030 [US2] Import greenops package in internal/tui/cost_view.go
- [x] T031 [US2] Add aggregateCarbonFromResults() helper function in internal/tui/cost_view.go
- [x] T032 [US2] Modify RenderCostSummary() to call greenops.Calculate() in internal/tui/cost_view.go
- [x] T033 [US2] Apply Lip Gloss styling to equivalency text matching TUI design patterns in internal/tui/cost_view.go

**Checkpoint**: User Story 2 complete. TUI users see styled carbon equivalencies in summary view.

---

## Phase 5: User Story 3 - View Carbon Equivalencies in Analyzer Diagnostics (Priority: P3)

**Goal**: Display compact carbon equivalencies in Pulumi Analyzer diagnostic output during `pulumi preview`

**Independent Test**: Run `pulumi preview` with FinFocus analyzer configured and verify diagnostic messages include carbon equivalencies in compact format

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [x] T034 [P] [US3] Unit test for formatCostMessage() equivalency display in internal/analyzer/diagnostics_test.go
- [x] T035 [P] [US3] Test compact format output in analyzer diagnostics in internal/analyzer/diagnostics_test.go

### Implementation for User Story 3

- [x] T036 [US3] Import greenops package in internal/analyzer/diagnostics.go
- [x] T037 [US3] Modify formatCostMessage() to call greenops.Calculate() in internal/analyzer/diagnostics.go
- [x] T038 [US3] Append CompactText to diagnostic message after sustainability metrics in internal/analyzer/diagnostics.go

**Checkpoint**: User Story 3 complete. Analyzer users see carbon equivalencies in Pulumi diagnostic output.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, validation, and quality assurance

- [x] T039 [P] Create user guide for equivalency feature in docs/guides/greenops-equivalencies.md
- [x] T040 [P] Create architecture documentation for greenops package in docs/architecture/greenops-package.md
- [x] T041 Run quickstart.md validation scenarios manually
- [x] T042 Verify 80%+ test coverage for internal/greenops/ package (achieved 89.6%)
- [x] T043 Run make lint and fix any issues
- [x] T044 Run make test and verify all tests pass
- [x] T045 Update docs/llms.txt with greenops package entry

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-5)**: All depend on Foundational phase completion
  - User stories can proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Independent of US1
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Independent of US1/US2

### Within Each Phase

- Tests MUST be written and FAIL before implementation
- Types/structs before functions
- Core functions before integration
- Verify tests pass after implementation

### Parallel Opportunities

- All Setup tasks T002-T007 can run in parallel (different files)
- All Foundational tests T008-T012 can run in parallel
- Once Foundational completes, US1/US2/US3 can start in parallel
- Within each user story, tests can run in parallel
- Polish documentation tasks T039-T040 can run in parallel

---

## Parallel Example: Phase 1 Setup

```bash
# Launch all type/constant tasks together:
Task: "Define EquivalencyType enum and constants in internal/greenops/types.go"
Task: "Define CarbonInput, EquivalencyResult, EquivalencyOutput structs in internal/greenops/types.go"
Task: "Define EPA formula constants with source comments in internal/greenops/constants.go"
Task: "Define unit conversion constants in internal/greenops/constants.go"
Task: "Define display threshold constants in internal/greenops/constants.go"
Task: "Define error types in internal/greenops/errors.go"
```

## Parallel Example: Phase 2 Foundational Tests

```bash
# Launch all test files together (TDD - write first, ensure they fail):
Task: "Unit tests for NormalizeToKg() in internal/greenops/normalizer_test.go"
Task: "Unit tests for FormatNumber/FormatFloat/FormatLarge() in internal/greenops/formatter_test.go"
Task: "Unit tests for Calculate() with EPA formula verification in internal/greenops/equivalency_test.go"
Task: "Unit tests for CalculateFromMap() in internal/greenops/equivalency_test.go"
Task: "Unit tests for edge cases in internal/greenops/equivalency_test.go"
```

## Parallel Example: All User Stories

```bash
# After Foundational phase completes, launch all user stories in parallel:
# Developer A: User Story 1 (CLI integration)
# Developer B: User Story 2 (TUI integration)
# Developer C: User Story 3 (Analyzer integration)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (CLI equivalencies)
4. **STOP and VALIDATE**: Run `finfocus cost projected` and verify equivalencies display
5. Deploy/demo if ready - CLI users immediately benefit

### Incremental Delivery

1. Complete Setup + Foundational → greenops package ready
2. Add User Story 1 (CLI) → Test independently → Deploy (MVP!)
3. Add User Story 2 (TUI) → Test independently → Deploy
4. Add User Story 3 (Analyzer) → Test independently → Deploy
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (CLI)
   - Developer B: User Story 2 (TUI)
   - Developer C: User Story 3 (Analyzer)
3. Stories complete and integrate independently
4. Merge order: US1 → US2 → US3 (priority order)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing (TDD)
- greenops package has no external dependencies beyond golang.org/x/text
- EPA formulas are hardcoded constants - no configuration needed
- All calculations normalize to kg internally before applying formulas
