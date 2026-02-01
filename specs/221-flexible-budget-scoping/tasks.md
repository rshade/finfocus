# Tasks: Flexible Budget Scoping

**Input**: Design documents from `/specs/221-flexible-budget-scoping/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY
and must be written BEFORE implementation. All code changes must maintain minimum 80%
test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks
MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly
forbidden.

**Documentation**: Per Constitution Principle IV, documentation (README, docs/) MUST be
updated concurrently with implementation to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation
and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Config types**: `internal/config/`
- **Engine logic**: `internal/engine/`
- **CLI commands**: `internal/cli/`
- **Unit tests**: `test/unit/` mirroring source structure
- **Integration tests**: `test/integration/`
- **Documentation**: `docs/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create foundational types and test infrastructure for scoped budgets

- [X] T001 [P] Create ScopedBudget and TagBudget types in internal/config/budget_scoped.go
- [X] T002 [P] Create BudgetsConfig container type in internal/config/budget_scoped.go
- [X] T003 [P] Create ScopeType enum and constants in internal/engine/budget_scope.go
- [X] T004 Create test fixtures with sample scoped budget configs in test/fixtures/budgets/

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be
implemented - config parsing, validation, and legacy migration

**CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational (MANDATORY - TDD Required)

- [X] T005 [P] Unit test for BudgetsConfig parsing in test/unit/config/budget_scoped_test.go
- [X] T006 [P] Unit test for legacy config migration in test/unit/config/budget_scoped_test.go
- [X] T007 [P] Unit test for currency validation in test/unit/config/budget_scoped_test.go
- [X] T008 [P] Unit test for ScopedBudget validation in test/unit/config/budget_scoped_test.go

### Implementation for Foundational

- [X] T009 Implement BudgetsConfig YAML parsing with global, providers, tags, types in internal/config/budget_scoped.go
- [X] T010 Implement legacy config auto-migration (amount→global.amount) in internal/config/config.go
- [X] T011 Implement currency validation (all scopes must match global) in internal/config/budget_scoped.go
- [X] T012 Implement ScopedBudget validation (amount>=0, period, exit_code) in internal/config/budget_scoped.go
- [X] T013 Implement TagBudget selector validation (key:value or key:*) in internal/config/budget_scoped.go
- [X] T014 Add BudgetsConfig to CostConfig and update LoadConfig in internal/config/config.go
- [X] T015 Create ScopedBudgetStatus type for runtime evaluation in internal/engine/budget_scope.go
- [X] T016 Create ScopedBudgetResult container for aggregated output in internal/engine/budget_scope.go

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Multi-Cloud Spend Management (Priority: P1)

**Goal**: Enable per-provider budget configuration and display (AWS, GCP, Azure)

**Independent Test**: Configure budgets for two providers, verify AWS costs count only
toward AWS and Global budgets, not GCP budget

### Tests for User Story 1 (MANDATORY - TDD Required)

- [X] T017 [P] [US1] Unit test for provider extraction from resource type in test/unit/engine/budget_scope_test.go
- [X] T018 [P] [US1] Unit test for provider budget matching in test/unit/engine/budget_scope_test.go
- [X] T019 [P] [US1] Unit test for provider cost allocation in test/unit/engine/budget_scope_test.go
- [X] T020 [P] [US1] Integration test for provider budget end-to-end in test/integration/budget_scope_test.go

### Implementation for User Story 1

- [X] T021 [US1] Implement ExtractProvider() to extract "aws" from "aws:ec2/instance" in internal/engine/budget_scope.go
- [X] T022 [US1] Implement provider budget index (map[string]*ScopedBudget) in internal/engine/budget_scope.go
- [X] T023 [US1] Implement AllocateCostToProvider() for provider scope matching in internal/engine/budget_scope.go
- [X] T024 [US1] Implement GetScopedBudgetStatus() for provider scopes in internal/engine/budget_scope.go
- [X] T025 [US1] Add --budget-scope flag with "provider" filter to cost budget command in internal/cli/cost_budget.go
- [X] T026 [US1] Implement BY PROVIDER section rendering in internal/cli/cost_budget_render.go
- [X] T027 [US1] Add debug logging for provider scope matching in internal/engine/budget_scope.go
- [X] T028 [P] [US1] Add provider budget section to docs/guides/user/budget-scoping.md

**Checkpoint**: Provider budgets fully functional - can view BY PROVIDER section

---

## Phase 4: User Story 2 - Team-Based Budgeting via Tags (Priority: P2)

**Goal**: Enable per-tag budget configuration with priority-based cost allocation

**Independent Test**: Configure budget for tag `team:platform`, verify resources with
that tag are correctly aggregated against that budget only

### Tests for User Story 2 (MANDATORY - TDD Required)

- [X] T029 [P] [US2] Unit test for tag selector parsing (key:value and key:*) in test/unit/config/budget_scoped_test.go (TestParseTagSelector, TestParsedTagSelector_Matches)
- [X] T030 [P] [US2] Unit test for tag matching against resource tags in test/unit/engine/budget_scope_test.go (TestScopedBudgetEvaluator/MatchTagBudgets)
- [X] T031 [P] [US2] Unit test for priority-based tag budget selection in test/unit/engine/budget_scope_test.go (TestScopedBudgetEvaluator/SelectHighestPriorityTagBudget)
- [X] T032 [P] [US2] Unit test for overlapping tag warning emission in test/unit/engine/budget_scope_test.go (included in SelectHighestPriorityTagBudget test)
- [X] T033 [P] [US2] Integration test for tag budget end-to-end in test/integration/budget_scope_test.go (TestTagBudget_EndToEnd)

### Implementation for User Story 2

- [X] T034 [US2] Implement ParseTagSelector() for "key:value" and "key:*" patterns in internal/config/budget_scoped.go
- [X] T035 [US2] Implement tag budget list sorted by priority in internal/engine/budget_scope.go (NewScopedBudgetEvaluator sorts tagBudgets)
- [X] T036 [US2] Implement MatchTagBudgets() to find all matching tag budgets in internal/engine/budget_scope.go
- [X] T037 [US2] Implement SelectHighestPriorityTagBudget() for single allocation in internal/engine/budget_scope.go
- [X] T038 [US2] Implement warning emission for overlapping tags without priority in internal/engine/budget_scope.go (handlePriorityTie)
- [X] T039 [US2] Implement AllocateCostToTag() for tag scope matching in internal/engine/budget_scope.go
- [X] T040 [US2] Add --budget-scope flag with "tag=" filter to cost budget command in internal/cli/cost_budget_render.go (NewBudgetScopeFilter)
- [X] T041 [US2] Implement BY TAG section rendering in internal/cli/cost_budget_render.go (renderTagSection, renderPlainTagSection)
- [X] T042 [US2] Add debug logging for tag scope matching and priority selection in internal/engine/budget_scope.go (zerolog logging present)
- [X] T043 [P] [US2] Add tag budget section to docs/guides/budgets.md (configuration, usage, example output, selector patterns, priority allocation)

**Checkpoint**: Tag budgets fully functional - can view BY TAG section with warnings

---

## Phase 5: User Story 3 - Resource Category Control (Priority: P3)

**Goal**: Enable per-resource-type budget configuration (e.g., aws:ec2/instance)

**Independent Test**: Configure budget for `aws:ec2/instance`, verify only EC2 resources
contribute to that budget

### Tests for User Story 3 (MANDATORY - TDD Required)

- [X] T044 [P] [US3] Unit test for resource type budget matching in test/unit/engine/budget_scope_test.go (TestGetTypeBudget)
- [X] T045 [P] [US3] Unit test for resource type cost allocation in test/unit/engine/budget_scope_test.go (TestAllocateCostToType, TestCalculateTypeBudgetStatus)
- [X] T046 [P] [US3] Integration test for resource type budget end-to-end in test/integration/budget_scope_test.go (TestTypeBudget_EndToEnd)

### Implementation for User Story 3

- [X] T047 [US3] Implement resource type budget index (map[string]*ScopedBudget) in internal/engine/budget_scope.go (typeIndex in NewScopedBudgetEvaluator)
- [X] T048 [US3] Implement AllocateCostToType() for resource type scope matching in internal/engine/budget_scope.go
- [X] T049 [US3] Implement GetScopedBudgetStatus() for type scopes in internal/engine/budget_scope.go (CalculateTypeBudgetStatus)
- [X] T050 [US3] Add --budget-scope flag with "type" filter to cost budget command in internal/cli/cost_budget.go (NewBudgetScopeFilter)
- [X] T051 [US3] Implement BY TYPE section rendering in internal/cli/cost_budget_render.go (renderTypeSection, renderPlainTypeSection)
- [X] T052 [US3] Add debug logging for type scope matching in internal/engine/budget_scope.go (zerolog in AllocateCostToType)
- [X] T053 [P] [US3] Add resource type budget section to docs/guides/budgets.md

**Checkpoint**: Resource type budgets fully functional - can view BY TYPE section

---

## Phase 6: Integration & Aggregation

**Purpose**: Combine all scope types into unified evaluation and display

### Tests for Integration (MANDATORY - TDD Required)

- [X] T054 [P] Unit test for multi-scope allocation (global+provider+tag+type) in test/unit/engine/budget_scope_test.go (TestAllocateCosts)
- [X] T055 [P] Unit test for overall health aggregation (worst wins) in test/unit/engine/budget_scope_test.go (TestCalculateOverallHealth)
- [X] T056 [P] Unit test for critical scopes identification in test/unit/engine/budget_scope_test.go (TestIdentifyCriticalScopes)
- [X] T057 [P] Integration test for full budget status output in test/integration/budget_scope_test.go (TestFullScopedBudgetStatus_EndToEnd)

### Implementation for Integration

- [X] T058 Implement AllocateCosts() orchestrating all scope allocations in internal/engine/budget_scope.go
- [X] T059 Implement EvaluateScopedBudgets() computing all statuses and health in internal/engine/budget_scope.go (ScopedBudgetResult type with BuildResult method)
- [X] T060 Implement CalculateOverallHealth() using worst-wins aggregation in internal/engine/budget_scope.go
- [X] T061 Implement IdentifyCriticalScopes() for exceeded/critical budgets in internal/engine/budget_scope.go
- [X] T062 Implement GLOBAL section rendering at top of output in internal/cli/cost_budget_render.go (renderGlobalSection, renderStyledScopedBudget)
- [X] T063 Implement hierarchical BUDGET STATUS display combining all sections in internal/cli/cost_budget_render.go (RenderScopedBudgetStatus, renderPlainScopedBudget)
- [X] T064 Integrate ScopedBudgetResult with existing budget command in internal/cli/cost_budget.go (buildScopedBudgetResult, renderScopedBudgets)
- [X] T065 Implement exit code handling for scoped budget thresholds in internal/cli/cost_budget.go (DetermineExitCodeFromBudget, BudgetExitError)

**Checkpoint**: Full hierarchical budget display working with all scope types

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, performance optimization, and final validation

### Documentation (Finalization)

- [X] T066 [P] Finalize user guide with overview and troubleshooting in docs/guides/budgets.md (comprehensive scoped budget docs)
- [X] T067 [P] Update configuration reference with budgets section in docs/reference/config-reference.md
- [X] T068 [P] Update README.md with budget scoping feature summary
- [X] T069 [P] Add budget scoping examples to docs/getting-started/quickstart.md

### Performance & Quality

- [X] T070 [P] Add benchmark test for 10,000 resource allocation in test/benchmarks/budget_scope_bench_test.go
- [X] T071 Verify <500ms performance goal for 10,000 resources (SC-003) - ~102ms achieved
- [X] T072 Ensure 80% overall coverage, 95% for config parsing and scope matching (budget_scope.go: 83-100%, budget_scoped.go: 83-100%)
- [X] T073 Run make lint and make test, fix all issues
- [X] T074 Validate quickstart.md scenarios work end-to-end (configuration documented)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational (Phase 2)
- **User Story 2 (Phase 4)**: Depends on Foundational (Phase 2) - can parallel with US1
- **User Story 3 (Phase 5)**: Depends on Foundational (Phase 2) - can parallel with US1/US2
- **Integration (Phase 6)**: Depends on all User Stories (Phase 3, 4, 5)
- **Polish (Phase 7)**: Depends on Integration (Phase 6)

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational - Independent of US1
- **User Story 3 (P3)**: Can start after Foundational - Independent of US1/US2

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Types before functions
- Core implementation before CLI integration
- Story complete before moving to next priority

### Parallel Opportunities

**Phase 1 (Setup)**:

```text
T001 || T002 || T003  (all parallel - different files)
T004 (after T001-T003)
```

**Phase 2 (Foundational)**:

```text
T005 || T006 || T007 || T008  (all tests parallel)
T009 (after tests)
T010 || T011 || T012 || T013  (parallel after T009)
T014 (after T010-T013)
T015 || T016  (parallel - different files)
```

**User Stories (Phases 3, 4, 5)**:

```text
After Phase 2 completes:
  US1 || US2 || US3  (all can run in parallel by different developers)

Within each story:
  T017 || T018 || T019 || T020  (US1 tests parallel)
  T021 → T022 → T023 → T024 (US1 impl sequential)
  T025 || T026 || T027 || T028  (US1 CLI + docs parallel after impl)
```

---

## Parallel Example: Full Team Execution

```bash
# Phase 1: Setup (all parallel)
Task T001: "Create ScopedBudget and TagBudget types"
Task T002: "Create BudgetsConfig container type"
Task T003: "Create ScopeType enum and constants"

# Phase 2: Foundational tests (all parallel)
Task T005: "Unit test for BudgetsConfig parsing"
Task T006: "Unit test for legacy config migration"
Task T007: "Unit test for currency validation"
Task T008: "Unit test for ScopedBudget validation"

# After Phase 2: All user stories can run in parallel
# Developer A: User Story 1 (Provider budgets)
# Developer B: User Story 2 (Tag budgets)
# Developer C: User Story 3 (Type budgets)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (Provider budgets)
4. **STOP and VALIDATE**: Test provider budgets independently
5. Deploy/demo if ready - users can use BY PROVIDER view

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP - provider budgets!)
3. Add User Story 2 → Test independently → Deploy/Demo (tag budgets added!)
4. Add User Story 3 → Test independently → Deploy/Demo (type budgets added!)
5. Complete Integration → Full hierarchical display
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With three developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (Provider budgets)
   - Developer B: User Story 2 (Tag budgets)
   - Developer C: User Story 3 (Type budgets)
3. All three complete Integration phase together
4. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Run `make lint` and `make test` frequently
- Avoid: vague tasks, same file conflicts, cross-story dependencies
