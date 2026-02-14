# Tasks: Wire Router into Cost Commands

**Input**: Design documents from `/specs/511-wire-router/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Create the type bridge and CLI helper that ALL user stories depend on. No wiring can occur until these components exist.

- [x] T001 Implement EngineAdapter struct with NewEngineAdapter constructor, SelectPlugins method (converts `[]router.PluginMatch` to `[]engine.PluginMatch` using `MatchReason.String()`), and ShouldFallback method (delegates to underlying router) in `internal/router/engine_adapter.go`
- [x] T002 Write unit tests for EngineAdapter covering: all MatchReason enum values convert correctly (automatic, pattern, global), empty match slice returns empty slice, ShouldFallback delegates correctly, multiple matches preserve order/priority/fallback/source fields, and SelectPlugins forwards the `feature` parameter unchanged to the underlying router (verifies FR-007 feature-specific routing passthrough) in `internal/router/engine_adapter_test.go`
- [x] T003 Add `createRouterForEngine(ctx context.Context, clients []*pluginhost.Client) engine.Router` helper function to `internal/cli/common_execution.go` that loads config via `config.New()`, returns nil if `cfg.Routing` is nil, creates `router.NewRouter(WithClients, WithConfig)` and wraps in `router.NewEngineAdapter()`, and logs WARN and returns nil on router creation failure. Add imports for `config` and `router` packages.

**Checkpoint**: EngineAdapter and CLI helper are ready. All user story wiring can now proceed.

---

## Phase 2: User Story 1 - Region-Aware Cost Queries (Priority: P1)

**Goal**: Wire the router into the primary cost commands (projected, actual) so that region-specific plugins are selected per resource instead of querying all plugins.

**Independent Test**: Run `cost projected` or `cost actual` with routing configuration specifying region preferences and verify only the matching plugin is queried for each resource.

### Tests for User Story 1

- [x] T004 [US1] Write unit tests for `createRouterForEngine()` helper covering: nil routing config returns nil (no warning logged), valid routing config returns non-nil `engine.Router`, invalid pattern in routing config returns nil with WARN log, empty clients slice returns non-nil router, and a benchmark `BenchmarkCreateRouterForEngine` that verifies router initialization completes in <10ms (SC-004) in `internal/cli/common_execution_test.go`

### Implementation for User Story 1

- [x] T005 [P] [US1] Chain `.WithRouter(createRouterForEngine(ctx, clients))` on `engine.New(clients, spec.NewLoader(specDir))` at line 181 of `internal/cli/cost_projected.go`
- [x] T006 [P] [US1] Chain `.WithRouter(createRouterForEngine(ctx, clients))` on `engine.New(clients, nil)` at line 214 of `internal/cli/cost_actual.go`

**Checkpoint**: `cost projected` and `cost actual` now use region-aware routing when configured. User Story 1 is testable.

---

## Phase 3: User Story 2 - Transparent Backward Compatibility (Priority: P1)

**Goal**: Verify that all commands continue to work identically when no routing configuration is present. The `createRouterForEngine()` helper returns nil when `cfg.Routing` is nil, and `engine.WithRouter(nil)` preserves the existing "query all plugins" behavior.

**Independent Test**: Run any cost command without routing configuration and verify all plugins are queried for every resource (identical to current behavior).

### Implementation for User Story 2

- [x] T007 [US2] Write tests verifying backward compatibility: engine with nil router queries all clients (verify via `selectPluginMatchesForResource` behavior), empty routing section in config is equivalent to nil routing, wired commands produce same output as unwired commands when no routing config exists, routing config referencing a non-installed plugin falls back to querying available plugins (edge case 1), and routing config where no plugins match a resource falls back to querying all plugins (edge case 4), in `internal/cli/common_execution_test.go`

**Checkpoint**: Backward compatibility is verified. No behavioral regression for zero-config users.

---

## Phase 4: User Story 4 - Consistent Routing Across All Commands (Priority: P2)

**Goal**: Wire the router into ALL remaining cost commands so routing applies uniformly. Skip nil-client call sites (history, undismiss, dismiss-local).

**Independent Test**: Configure routing and run each command to verify routing is applied consistently.

### Implementation for User Story 4

- [x] T008 [P] [US4] Chain `.WithRouter(createRouterForEngine(ctx, clients))` on `engine.New(clients, spec.NewLoader(specDir))` at lines 361, 419, and 746 of `internal/cli/cost_estimate.go`
- [x] T009 [P] [US4] Chain `.WithRouter(createRouterForEngine(ctx, clients))` on `engine.New(clients, nil)` at line 243 of `internal/cli/cost_recommendations.go`
- [x] T010 [P] [US4] Chain `.WithRouter(createRouterForEngine(ctx, clients))` on `engine.New(clients, nil)` at line 314 of `internal/cli/cost_recommendations_dismiss.go` (skip line 301 which uses `engine.New(nil, nil)` for local-only mode)
- [x] T011 [P] [US4] Chain `.WithRouter(createRouterForEngine(ctx, clients))` on `engine.New(clients, nil)` at line 143 of `internal/cli/overview.go`
- [x] T012 [P] [US4] Chain `.WithRouter(createRouterForEngine(ctx, clients))` on `engine.New(clients, specLoader)` at line 122 of `internal/cli/analyzer_serve.go`
- [x] T013 [US4] Verify nil-client call sites remain unchanged: `cost_recommendations_dismiss.go:301` (`engine.New(nil, nil)`), `cost_recommendations_history.go:55` (`engine.New(nil, nil)`), `cost_recommendations_undismiss.go:62` (`engine.New(nil, nil)`) must NOT have `.WithRouter()` added

**Checkpoint**: All 9 plugin-using call sites are wired. 3 nil-client sites are verified unchanged.

---

## Phase 5: User Story 3 - Priority-Based Plugin Selection (Priority: P2)

**Goal**: Verify that priority-based plugin selection and fallback behavior work correctly through the EngineAdapter. The router already implements priority logic; the adapter must faithfully pass Priority and Fallback fields.

**Independent Test**: Configure two plugins with different priorities for the same resource type and verify the higher-priority plugin is preferred.

### Implementation for User Story 3

- [x] T014 [US3] Add test cases to `internal/router/engine_adapter_test.go` verifying: Priority field is preserved across conversion (priority 10 stays 10), Fallback field is preserved (true stays true, false stays false), multiple matches maintain priority ordering from router, and MatchReason string values map correctly for all enum values including edge case `MatchReasonNoMatch`

**Checkpoint**: Priority and fallback behavior verified through the adapter.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Final validation, documentation, and quality gates.

- [x] T015 Run `make test` to verify all existing tests still pass with router wiring changes
- [x] T016 Run `make lint` to verify code quality and formatting compliance
- [x] T017 Update router wiring documentation in `CLAUDE.md` under the CLI Package section to document the `createRouterForEngine()` helper pattern and the list of wired call sites

---

## Dependencies and Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies - start immediately. T001 before T002 (tests need types). T001 before T003 (helper uses adapter).
- **US1 (Phase 2)**: Depends on Phase 1 completion. T004 can run in parallel with T005/T006.
- **US2 (Phase 3)**: Depends on Phase 1 and at least T005 or T006 from Phase 2 (needs wired commands to test backward compat).
- **US4 (Phase 4)**: Depends on Phase 1 completion only. Can run in parallel with Phase 2/3.
- **US3 (Phase 5)**: Depends on Phase 1 (T002 must exist to extend). Can run in parallel with Phases 2-4.
- **Polish (Phase 6)**: Depends on all previous phases completing.

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational. Core routing proof.
- **US2 (P1)**: Can start after Foundational + US1. Backward compat verification.
- **US3 (P2)**: Can start after Foundational. Priority verification (independent of wiring).
- **US4 (P2)**: Can start after Foundational. Extends wiring to all commands.

### Parallel Opportunities

- T005 and T006 can run in parallel (different files)
- T008 through T012 can ALL run in parallel (different files, no dependencies)
- T002 and T014 target the same test file - run T002 first, then T014 extends it
- Phases 2, 4, and 5 can proceed in parallel after Phase 1 completes

---

## Parallel Example: Phase 4 (User Story 4)

```text
# All 5 wiring tasks can launch simultaneously:
T008: Wire cost_estimate.go (3 sites)
T009: Wire cost_recommendations.go
T010: Wire cost_recommendations_dismiss.go
T011: Wire overview.go
T012: Wire analyzer_serve.go
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational (T001-T003)
2. Complete Phase 2: US1 - Wire projected + actual (T004-T006)
3. **STOP and VALIDATE**: Run `cost projected` and `cost actual` with and without routing config
4. Region-aware routing works for the two primary cost commands

### Incremental Delivery

1. Foundational (T001-T003) - Adapter + helper ready
2. US1 (T004-T006) - Core routing active for projected/actual (MVP)
3. US2 (T007) - Backward compatibility verified
4. US4 (T008-T013) - All commands wired consistently
5. US3 (T014) - Priority behavior verified
6. Polish (T015-T017) - Quality gates and documentation

### Full Implementation (Recommended)

Since all tasks are small (single-line changes for wiring), the full implementation can be completed in a single session:

1. T001-T003: Create adapter + helper (~50 lines production code)
2. T004-T006: Wire primary commands + tests
3. T007: Backward compat tests
4. T008-T013: Wire remaining commands (~1 line change each)
5. T014: Priority tests
6. T015-T017: Validate and document

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- All wiring changes are single-line additions (`.WithRouter(createRouterForEngine(ctx, clients))`)
- The router package already implements all selection logic; this feature only connects it
- 9 call sites to wire, 3 to explicitly skip (nil clients)
- Total new production code: ~50 lines (adapter) + ~20 lines (helper) + 9 one-line changes
