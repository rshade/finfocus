# Tasks: Multi-Plugin Routing

**Input**: Design documents from `/specs/126-multi-plugin-routing/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV, documentation (README, docs/) MUST be updated concurrently with implementation to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go project**: `internal/` for packages, `test/` for tests
- Paths follow plan.md structure

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create router package structure and shared utilities

- [X] T001 Create router package directory structure at `internal/router/`
- [X] T002 [P] Create Feature enum and validation helpers in `internal/router/features.go`
- [X] T003 [P] Move `extractProviderFromType()` to `internal/router/provider.go` with tests
- [X] T004 [P] Create test fixtures directory at `test/fixtures/routing/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core config and router types that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T005 Create RoutingConfig struct in `internal/config/routing.go`
- [X] T006 [P] Create PluginRouting struct in `internal/config/routing.go`
- [X] T007 [P] Create ResourcePattern struct in `internal/config/routing.go`
- [X] T008 Add `Routing *RoutingConfig` field to Config struct in `internal/config/config.go`
- [X] T009 Create Router interface in `internal/router/router.go`
- [X] T010 [P] Create PluginMatch struct and MatchReason enum in `internal/router/router.go`
- [X] T011 [P] Create ValidationResult, ValidationError, ValidationWarning structs in `internal/router/validation.go`
- [X] T012 Add `router router.Router` field to Engine struct in `internal/engine/engine.go`
- [X] T013 Create test routing config fixtures in `test/fixtures/routing/valid_config.yaml`
- [X] T014 [P] Create test routing config fixtures in `test/fixtures/routing/invalid_config.yaml`
- [X] T014a [P] Create backward compatibility test: config without `routing:` key uses automatic routing per FR-023 (F-005) in `test/integration/routing_backward_compat_test.go`

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Automatic Provider-Based Routing (Priority: P1) 🎯 MVP

**Goal**: AWS resources automatically route to AWS plugins, GCP resources to GCP plugins, with zero configuration

**Independent Test**: Install aws-public and gcp-public plugins, run cost calculation on mixed-cloud plan, verify each resource routes to correct provider plugin

### Tests for User Story 1 (TDD Required)

- [X] T015 [P] [US1] Unit test for provider extraction in `internal/router/provider_test.go`
- [X] T016 [P] [US1] Unit test for automatic routing in `internal/router/automatic_test.go`
- [X] T017 [P] [US1] Unit test for global plugin matching (empty/wildcard providers) in `internal/router/automatic_test.go`
- [X] T018 [US1] Integration test for automatic routing in `test/integration/routing_automatic_test.go` (include wildcard `["*"]` provider case per F-009)
- [X] T018a [P] [US1] Unit test for source field attribution in automatic routing results in `internal/router/automatic_test.go` (F-002)

### Implementation for User Story 1

- [X] T019 [US1] Implement automatic provider matching in `internal/router/router.go` (Note: Implemented in router.go:272-313, not separate automatic.go file)
- [X] T020 [US1] Implement `matchesProvider()` helper checking SupportedProviders in `internal/router/router.go:371-382`
- [X] T021 [US1] Implement global plugin detection (empty/wildcard) in `internal/router/router.go:372-380`
- [X] T022 [US1] Implement `SelectPlugins()` for automatic routing in `internal/router/router.go:216-326`
- [X] T023 [US1] Add debug logging for routing decisions per FR-020 in `internal/router/router.go`
- [X] T024 [US1] Integrate router into Engine.GetProjectedCost() in `internal/engine/engine.go:179`
- [X] T025 [US1] Modify engine plugin loop to use router-selected plugins in `internal/engine/engine.go`

**Checkpoint**: Automatic routing works - resources route to provider-matching plugins without config

---

## Phase 4: User Story 2 - Feature-Specific Plugin Routing (Priority: P1)

**Goal**: Configure different plugins for different features (aws-ce for Recommendations, aws-public for ProjectedCosts)

**Independent Test**: Configure two plugins with different feature assignments, verify cost commands route to correct plugin based on feature

### Tests for User Story 2 (TDD Required)

- [X] T026 [P] [US2] Unit test for feature matching in `internal/router/features_test.go`
- [X] T027 [P] [US2] Unit test for feature-based routing in `internal/router/router_test.go` (Note: Implemented in router_test.go, not separate declarative_test.go)
- [X] T028 [US2] Integration test for feature routing in `test/integration/routing_features_test.go`
- [X] T028a [P] [US2] Integration test for DryRun feature routing in `test/integration/routing_features_test.go` (F-001)
- [X] T028b [P] [US2] Integration test for Budgets feature routing in `test/integration/routing_features_test.go` (F-001)

### Implementation for User Story 2

- [X] T029 [US2] Implement `matchesFeature()` helper in `internal/router/router.go:328-341`
- [X] T030 [US2] Implement feature filtering in SelectPlugins() in `internal/router/router.go:241-243,283-286`
- [X] T031 [US2] Add feature validation warnings per FR-017 in `internal/router/validation.go`
- [X] T032a [US2] Integrate feature routing for `cost projected` command in `internal/engine/engine.go`
- [X] T032b [US2] Integrate feature routing for `cost actual` command in `internal/engine/engine.go`
- [X] T032c [US2] Integrate feature routing for `cost recommendations` command in `internal/engine/engine.go`

**Checkpoint**: Feature-specific routing works - different features route to different plugins

---

## Phase 5: User Story 3 - Declarative Resource Pattern Overrides (Priority: P2)

**Goal**: Configure custom resource patterns (glob/regex) that override automatic provider matching

**Independent Test**: Configure pattern `aws:eks:.*` for eks-plugin, verify EKS resources route to eks-plugin instead of aws-public

### Tests for User Story 3 (TDD Required)

- [X] T033 [P] [US3] Unit test for glob pattern matching in `internal/router/pattern_test.go`
- [X] T034 [P] [US3] Unit test for regex pattern matching in `internal/router/pattern_test.go`
- [X] T035 [P] [US3] Unit test for pattern compilation caching in `internal/router/pattern_test.go`
- [X] T036 [US3] Integration test for pattern-based routing in `test/integration/routing_patterns_test.go`

### Implementation for User Story 3

- [X] T037 [US3] Implement CompiledPattern struct in `internal/router/pattern.go:12-31`
- [X] T038 [US3] Implement glob matching with `filepath.Match` in `internal/router/pattern.go:23-25,67-69`
- [X] T039 [US3] Implement regex matching with compiled patterns in `internal/router/pattern.go:26-30,72-95`
- [X] T040 [US3] Implement pattern cache for compiled regexes per FR-022 in `internal/router/pattern.go:33-95` and `router.go:195-206`
- [X] T041 [US3] Implement `matchesPattern()` helper in `internal/router/router.go:343-358` (Note: Named matchesAnyPattern in implementation)
- [X] T042 [US3] Implement pattern precedence over automatic routing per FR-009 in `internal/router/router.go:238-269`

**Checkpoint**: Pattern-based routing works - resource patterns override automatic provider matching

---

## Phase 6: User Story 4 - Priority-Based Plugin Selection (Priority: P2)

**Goal**: Assign priorities to plugins so higher-quality sources are preferred

**Independent Test**: Configure two plugins with different priorities for same resource, verify higher priority is queried first

### Tests for User Story 4 (TDD Required)

- [X] T043 [P] [US4] Unit test for priority sorting in `internal/router/priority_test.go`
- [X] T044 [P] [US4] Unit test for equal priority (query all) in `internal/router/priority_test.go`
- [X] T045 [US4] Integration test for priority-based selection in `test/integration/routing_priority_test.go`

### Implementation for User Story 4

- [X] T046 [US4] Implement `sortByPriority()` helper in `internal/router/router.go:427-435`
- [X] T047 [US4] Implement equal-priority detection for querying all plugins per FR-014 in `internal/router/router.go:438-444` (AllEqualPriority helper)
- [X] T048 [US4] Modify SelectPlugins() to return priority-ordered list in `internal/router/router.go:315-316`
- [X] T049 [US4] Add source field population for result attribution per FR-014 in `internal/router/router.go:254-255,301`

**Checkpoint**: Priority-based selection works - highest priority plugins queried first

---

## Phase 7: User Story 5 - Fallback on Plugin Failure (Priority: P2)

**Goal**: Automatically try alternative plugins when preferred plugin fails

**Independent Test**: Simulate plugin timeout, verify fallback plugin is automatically invoked

### Tests for User Story 5 (TDD Required)

- [X] T050 [P] [US5] Unit test for fallback trigger on error in `internal/router/priority_test.go`
- [X] T051 [P] [US5] Unit test for fallback disabled behavior in `internal/router/priority_test.go`
- [X] T052 [P] [US5] Unit test for empty result fallback trigger in `internal/router/priority_test.go` (Note: Empty result = no cost data triggers fallback; $0 cost = valid result, NO fallback per F-007)
- [X] T052a [P] [US5] Unit test for $0 cost result NOT triggering fallback in `internal/router/priority_test.go` (F-007)
- [X] T053 [US5] Integration test for fallback chain in `test/integration/routing_fallback_test.go`

### Implementation for User Story 5

- [X] T054 [US5] Implement `ShouldFallback()` method in `internal/router/router.go:414-420`
- [X] T055 [US5] Implement fallback logic for connection failures in `internal/engine/engine.go`
- [X] T056 [US5] Implement fallback logic for empty results in `internal/engine/engine.go`
- [X] T057 [US5] Add fallback event logging (INFO level) per FR-020 in `internal/engine/engine.go`
- [X] T058 [US5] Implement per-resource fallback for partial failures in `internal/engine/engine.go`

**Checkpoint**: Fallback works - plugin failures trigger automatic fallback to next priority

---

## Phase 8: User Story 6 - Validate Routing Configuration (Priority: P3)

**Goal**: Validate routing configuration before deploying changes

**Independent Test**: Run `finfocus config validate` against valid and invalid configs, verify appropriate feedback

### Tests for User Story 6 (TDD Required)

- [X] T059 [P] [US6] Unit test for plugin existence validation in `internal/router/validation_test.go`
- [X] T060 [P] [US6] Unit test for regex syntax validation in `internal/router/validation_test.go`
- [X] T061 [P] [US6] Unit test for feature name validation in `internal/router/validation_test.go`
- [X] T061a [P] [US6] Unit test for duplicate plugin configuration warning in `internal/router/validation_test.go` (F-003)
- [X] T062 [US6] Integration test for config validate command in `test/integration/config_validate_test.go`

### Implementation for User Story 6

- [X] T063 [US6] Implement `Validate()` method returning ValidationResult in `internal/router/validation.go`
- [X] T064 [US6] Implement plugin existence check in `internal/router/validation.go`
- [X] T065 [US6] Implement regex pattern syntax validation in `internal/router/validation.go`
- [X] T066 [US6] Implement feature name validation with warnings in `internal/router/validation.go`
- [X] T066a [US6] Implement duplicate plugin configuration detection with warning in `internal/router/validation.go` (F-003)
- [X] T067 [US6] Create `config validate` CLI command in `internal/cli/config_validate.go`
- [X] T068 [US6] Wire config validate command to root in `internal/cli/root.go:212`

**Checkpoint**: Config validation works - users can validate routing config before use

---

## Phase 9: User Story 7 - View Plugin Capabilities and Providers (Priority: P3)

**Goal**: See what capabilities and providers each installed plugin reports

**Independent Test**: Run `finfocus plugin list`, verify capabilities and providers displayed for each plugin

### Tests for User Story 7 (TDD Required)

- [X] T069 [P] [US7] Unit test for capability display formatting in `internal/cli/plugin_list_test.go`
- [X] T070 [US7] Integration test for plugin list with capabilities in `test/integration/plugin_list_test.go`

### Implementation for User Story 7

- [X] T074 [US7] Implement capability inference from RPC methods per FR-021 in `internal/router/features.go` (Note: Foundation for T071-T073 display logic)
- [X] T071 [US7] Modify plugin list output to show SupportedProviders in `internal/cli/plugin_list.go`
- [X] T072 [US7] Modify plugin list output to show capabilities in `internal/cli/plugin_list.go`
- [X] T073 [US7] Add --verbose flag for detailed capability display in `internal/cli/plugin_list.go`

**Checkpoint**: Plugin list shows capabilities and providers - users can configure routing correctly

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and final quality gates

- [X] T075 [P] Update README.md with multi-plugin routing section
- [X] T076 [P] Create routing configuration guide in `docs/guides/routing.md`
- [X] T077 [P] Update CLI reference docs in `docs/reference/cli-commands.md`
- [X] T078 Run `make lint` and fix any issues
- [X] T079 Run `make test` and ensure all tests pass with 80%+ coverage
- [X] T080 Run quickstart.md validation (test examples work)
- [X] T081 Final code review for Constitution compliance

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational - MVP, can start first
- **US2 (Phase 4)**: Depends on Foundational - parallel with US1 if staffed
- **US3 (Phase 5)**: Depends on Foundational - parallel with US1/US2
- **US4 (Phase 6)**: Depends on US1 (uses SelectPlugins)
- **US5 (Phase 7)**: Depends on US4 (uses priority ordering)
- **US6 (Phase 8)**: Depends on Foundational - parallel with US1-US5
- **US7 (Phase 9)**: Depends on Foundational - parallel with US1-US6
- **Polish (Phase 10)**: Depends on all user stories being complete

### User Story Dependencies

```text
                    ┌─────────────┐
                    │   Setup     │
                    │  (Phase 1)  │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ Foundational │
                    │  (Phase 2)   │
                    └──────┬──────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
┌───────▼───────┐  ┌───────▼───────┐  ┌───────▼───────┐
│   US1 (P1)    │  │   US2 (P1)    │  │   US3 (P2)    │
│  Automatic    │  │   Features    │  │   Patterns    │
└───────┬───────┘  └───────────────┘  └───────────────┘
        │
┌───────▼───────┐         ┌───────────────┐  ┌───────────────┐
│   US4 (P2)    │         │   US6 (P3)    │  │   US7 (P3)    │
│   Priority    │         │  Validation   │  │  Plugin List  │
└───────┬───────┘         └───────────────┘  └───────────────┘
        │
┌───────▼───────┐
│   US5 (P2)    │
│   Fallback    │
└───────────────┘
```

### Parallel Opportunities

**Within Phase 1 (Setup)**:

- T002, T003, T004 can run in parallel

**Within Phase 2 (Foundational)**:

- T006, T007 can run in parallel (different structs)
- T010, T011 can run in parallel (different files)
- T013, T014 can run in parallel (different fixtures)

**Across User Stories**:

- US1, US2, US3 can run in parallel after Foundational
- US6, US7 can run in parallel with US1-US5

---

## Parallel Example: Foundational Phase

```bash
# After T005 completes, launch these in parallel:
Task: "Create PluginRouting struct in internal/config/routing.go"
Task: "Create ResourcePattern struct in internal/config/routing.go"

# After T009 completes, launch these in parallel:
Task: "Create PluginMatch struct in internal/router/router.go"
Task: "Create ValidationResult structs in internal/router/validation.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (4 tasks)
2. Complete Phase 2: Foundational (11 tasks, CRITICAL)
3. Complete Phase 3: User Story 1 - Automatic Routing (12 tasks)
4. **STOP and VALIDATE**: Test with mixed-cloud plan
5. Deploy/demo - automatic routing works!

**MVP Total**: 27 tasks

### Recommended Delivery Order

1. **Increment 1**: Setup + Foundational + US1 (Automatic Routing) → MVP
2. **Increment 2**: US2 (Features) + US4 (Priority) → Feature routing with priority
3. **Increment 3**: US3 (Patterns) + US5 (Fallback) → Advanced routing
4. **Increment 4**: US6 (Validation) + US7 (Plugin List) → Observability
5. **Increment 5**: Polish → Production ready

### Task Count Summary

| Phase | Story | Task Count |
| ----- | ----- | ---------- |
| Phase 1 | Setup | 4 |
| Phase 2 | Foundational | 11 |
| Phase 3 | US1 - Automatic Routing | 12 |
| Phase 4 | US2 - Feature Routing | 11 |
| Phase 5 | US3 - Pattern Overrides | 10 |
| Phase 6 | US4 - Priority Selection | 7 |
| Phase 7 | US5 - Fallback | 10 |
| Phase 8 | US6 - Validation | 12 |
| Phase 9 | US7 - Plugin List | 6 |
| Phase 10 | Polish | 7 |
| **Total** | | **90** |

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- TDD required: Write tests first, ensure they fail before implementing
- Commit after each task or logical group
- Run `make lint` and `make test` before claiming any task complete
