# Tasks: Recommendation Dismissal and Lifecycle Management

**Input**: Design documents from `/specs/508-recommendation-dismissal/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No project scaffolding needed (existing Go module). This phase is a no-op.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared utilities and data types that ALL user stories depend on. MUST complete before any user story work begins.

### Tests (TDD - Write First)

- [x] T001 [P] Write unit tests for DismissalReason parsing utilities (ParseDismissalReason, DismissalReasonLabel, ValidDismissalReasons, ParseDismissalReasonFilter) with table-driven tests covering all 7 reason values plus invalid input in `internal/proto/dismissal_reasons_test.go`
- [x] T002 [P] Write unit tests for DismissalStore (Load, Save, Get, Set, Delete, GetDismissedIDs, GetAllRecords, GetExpiredSnoozes, CleanExpiredSnoozes) covering empty store, single dismissal, multiple dismissals, snooze expiry, corrupted file, missing file, and version mismatch in `internal/config/dismissed_test.go`

### Implementation

- [x] T003 [P] Implement DismissalReason parsing utilities following the action_types.go pattern: ParseDismissalReason(string) maps CLI flag values (not-applicable, already-implemented, business-constraint, technical-constraint, deferred, inaccurate, other) to pbc.DismissalReason enum values; DismissalReasonLabel(pbc.DismissalReason) returns human-readable labels; ValidDismissalReasons() returns valid CLI flag values in `internal/proto/dismissal_reasons.go`
- [x] T004 [P] Implement DismissalStore with JSON file persistence at ~/.finfocus/dismissed.json: DismissalStore struct with Version (int) and Dismissals (map[string]*DismissalRecord); DismissalRecord struct with RecommendationID, Status (dismissed/snoozed), Reason, CustomReason, DismissedAt, DismissedBy, ExpiresAt (*time.Time), LastKnown (*LastKnownRecommendation), History ([]LifecycleEvent); LastKnownRecommendation struct; LifecycleEvent struct; Load() reads JSON with graceful corruption handling; Save() atomic write; all interface methods from contracts/adapter.md in `internal/config/dismissed.go`
- [x] T005 Add engine types for dismiss operations: DismissRequest (RecommendationID, Reason, CustomReason, ExpiresAt, Recommendation), DismissResult (RecommendationID, PluginDismissed, PluginName, PluginMessage, LocalPersisted, Warning), UndismissResult (RecommendationID, WasDismissed, Message) as defined in contracts/adapter.md in `internal/engine/types.go`

**Checkpoint**: Foundational types and utilities ready. All T001-T002 tests should pass. User story implementation can begin.

---

## Phase 3: User Story 1 + User Story 6 - Dismiss Recommendation with Plugin Delegation (Priority: P1) MVP

**Goal**: Users can dismiss a recommendation by ID with a reason. When the plugin advertises DISMISS_RECOMMENDATIONS capability, the DismissRecommendation RPC is called. Dismissal is always persisted locally. Dismissed recommendations are excluded from subsequent default listings.

**Independent Test**: Dismiss rec-123 with reason "business-constraint", verify it disappears from default list output. Test with plugin that has dismiss capability, verify RPC is called. Test with plugin without capability, verify local-only succeeds.

### Tests (TDD - Write First)

- [x] T006 [P] [US1] Write unit tests for DismissRecommendation adapter method: conversion from internal DismissRecommendationRequest to pbc.DismissRecommendationRequest (including ExpiresAt timestamp conversion), response mapping, error handling in `internal/proto/adapter_test.go`
- [x] T007 [P] [US1] Write unit tests for Engine.DismissRecommendation: plugin with dismiss capability calls RPC + persists locally; plugin without capability persists locally only; plugin RPC failure still persists locally with warning; already-dismissed returns informational message; ExcludedRecommendationIds passed on GetRecommendations in `internal/engine/engine_dismiss_test.go`
- [x] T008 [P] [US1] Write unit tests for dismiss CLI subcommand: flag parsing (--reason, --note, --force, --pulumi-json), reason validation, "other" requires --note, recommendation-id as positional arg, confirmation prompt skipped with --force, Snoozed->Dismissed direct transition (FR-010a) in `internal/cli/cost_recommendations_dismiss_test.go`

### Implementation

- [x] T009 [US1] Add DismissRecommendation method to CostSourceClient interface and implement in clientAdapter: convert internal DismissRecommendationRequest to pbc.DismissRecommendationRequest (map Reason enum, convert ExpiresAt to timestamppb.Timestamp, set CustomReason and DismissedBy), call c.client.DismissRecommendation(), convert pbc.DismissRecommendationResponse back to internal type in `internal/proto/adapter.go`
- [x] T010 [US1] Add HasCapability(capability string) bool method to pluginhost.Client that checks client.Metadata.Capabilities slice for a given capability string in `internal/pluginhost/host.go`
- [x] T011 [US1] Implement Engine.DismissRecommendation(ctx, DismissRequest) (*DismissResult, error): load DismissalStore, check if already dismissed, iterate plugins with HasCapability("dismiss_recommendations") and call DismissRecommendation RPC (log warn on failure), always persist locally via DismissalStore.Set() with LastKnown snapshot and LifecycleEvent, return DismissResult in `internal/engine/engine.go`
- [x] T012 [US1] Modify Engine.GetRecommendationsForResources to load DismissalStore at start, extract non-expired dismissed IDs via GetDismissedIDs(), clean expired snoozes via CleanExpiredSnoozes(), and pass IDs via ExcludedRecommendationIds field on the GetRecommendationsRequest sent to plugins in `internal/engine/engine.go`
- [x] T013 [US1] Implement dismiss CLI subcommand: newRecommendationsDismissCmd() returning *cobra.Command with Use "dismiss", Args cobra.ExactArgs(1) for recommendation-id, flags --reason/-r (required string), --note/-n (optional string), --force/-f (bool), --pulumi-json (string), --adapter (string); RunE loads plan, opens plugins, creates engine, calls engine.DismissRecommendation, renders result; includes confirmation prompt unless --force in `internal/cli/cost_recommendations_dismiss.go`
- [x] T014 [US1] Register dismiss subcommand in NewCostRecommendationsCmd(): call cmd.AddCommand(newRecommendationsDismissCmd()) in `internal/cli/cost_recommendations.go`

**Checkpoint**: Users can dismiss recommendations. Plugin delegation works. Dismissed recs excluded from default list. MVP complete.

---

## Phase 4: User Story 2 - Snooze a Recommendation (Priority: P2)

**Goal**: Users can snooze a recommendation until a future date. Snoozed recommendations automatically reappear when the date passes. Direct transitions (Dismissed->Snoozed, re-snooze) are allowed.

**Independent Test**: Snooze rec-456 until 2026-04-01, verify it disappears. Verify it reappears when listed after that date.

### Tests (TDD - Write First)

- [x] T015 [P] [US2] Write unit tests for snooze CLI subcommand: --until flag required and validated (future date, ISO 8601 and YYYY-MM-DD formats), --reason defaults to "deferred", past date rejection, direct Dismissed->Snoozed transition (FR-010a), Snoozed->Snoozed re-snooze (FR-010a: update expiry date) in `internal/cli/cost_recommendations_dismiss_test.go`

### Implementation

- [x] T016 [US2] Implement snooze CLI subcommand: newRecommendationsSnoozeCmd() returning *cobra.Command with Use "snooze", Args cobra.ExactArgs(1), flags --until (required string), --reason/-r (default "deferred"), --note/-n, --force/-f, --pulumi-json, --adapter; RunE parses --until as time.Time (support "2006-01-02" and RFC3339), validates future date, calls engine.DismissRecommendation with ExpiresAt set, renders result in `internal/cli/cost_recommendations_dismiss.go`
- [x] T017 [US2] Register snooze subcommand: call cmd.AddCommand(newRecommendationsSnoozeCmd()) in `internal/cli/cost_recommendations.go`

**Checkpoint**: Snooze works. Auto-unsnooze handled by T012's CleanExpiredSnoozes. Direct transitions work via DismissalStore.Set() overwriting existing records.

---

## Phase 5: User Story 3 - View All Including Dismissed (Priority: P2)

**Goal**: Users can list all recommendations including dismissed/snoozed with status indicators by using --include-dismissed flag. Dismissed items show last-known details merged with active plugin results.

**Independent Test**: Dismiss and snooze some recommendations, then list with --include-dismissed. Verify all appear with correct status labels.

### Tests (TDD - Write First)

- [x] T018 [P] [US3] Write unit tests for --include-dismissed merge behavior: active recs from plugin + dismissed/snoozed from local state merged into unified list; status indicators (Active, Dismissed, Snoozed) added; last-known details populated; default list excludes dismissed; table and JSON output formats both show status in `internal/cli/cost_recommendations_test.go`

### Implementation

- [x] T019 [US3] Add --include-dismissed bool flag to NewCostRecommendationsCmd() in `internal/cli/cost_recommendations.go`
- [x] T020 [US3] Implement merge logic in executeCostRecommendations: when --include-dismissed is true, after fetching active recommendations from engine, load DismissalStore, iterate all records, convert each DismissalRecord's LastKnown to engine.Recommendation with status annotation, append to results; pass combined results to RenderRecommendationsOutput in `internal/cli/cost_recommendations.go`
- [x] T021 [US3] Update RenderRecommendationsOutput to display Status column (Active/Dismissed/Snoozed) in table format; include status field in JSON output; show reason, note, and dismissal date for dismissed/snoozed items in `internal/cli/cost_recommendations.go`

**Checkpoint**: Audit view works. Users see all recommendations with lifecycle status.

---

## Phase 6: User Story 4 - Re-enable Dismissed Recommendation (Priority: P2)

**Goal**: Users can undismiss a previously dismissed or snoozed recommendation so it reappears in default listings.

**Independent Test**: Dismiss rec-123, undismiss it, verify it reappears in default listing.

### Tests (TDD - Write First)

- [x] T022 [P] [US4] Write unit tests for Engine.UndismissRecommendation: dismissed record removed from store and lifecycle event added; snoozed record removed; undismiss of non-dismissed ID returns informational error; undismiss CLI flag parsing in `internal/engine/engine_dismiss_test.go` and `internal/cli/cost_recommendations_undismiss_test.go`

### Implementation

- [x] T023 [US4] Implement Engine.UndismissRecommendation(ctx, recommendationID) (*UndismissResult, error): load DismissalStore, check if ID exists, append LifecycleEvent with action "undismissed", delete record from store, save, return UndismissResult in `internal/engine/engine.go`
- [x] T024 [US4] Implement undismiss CLI subcommand: newRecommendationsUndismissCmd() returning *cobra.Command with Use "undismiss", Args cobra.ExactArgs(1), flags --force/-f; RunE loads DismissalStore, calls engine.UndismissRecommendation, renders result (no plugin connection needed) in `internal/cli/cost_recommendations_undismiss.go`
- [x] T025 [US4] Register undismiss subcommand: call cmd.AddCommand(newRecommendationsUndismissCmd()) in `internal/cli/cost_recommendations.go`

**Checkpoint**: Full dismiss/undismiss lifecycle works. Users can reverse any dismissal decision.

---

## Phase 7: User Story 5 - View Dismissal History (Priority: P3)

**Goal**: Users can view the lifecycle history of a specific recommendation showing all dismiss/snooze/undismiss events with timestamps.

**Independent Test**: Dismiss rec-123, undismiss it, dismiss again. View history and verify 3 events in chronological order.

### Tests (TDD - Write First)

- [x] T026 [P] [US5] Write unit tests for Engine.GetRecommendationHistory: returns lifecycle events in chronological order; returns empty for unknown IDs; returns empty for active (never dismissed) IDs; history CLI table and JSON output in `internal/engine/engine_dismiss_test.go` and `internal/cli/cost_recommendations_history_test.go`

### Implementation

- [x] T027 [US5] Implement Engine.GetRecommendationHistory(ctx, recommendationID) ([]LifecycleEvent, error): load DismissalStore, look up record by ID, return record.History slice (empty slice if not found) in `internal/engine/engine.go`
- [x] T028 [US5] Implement history CLI subcommand: newRecommendationsHistoryCmd() returning *cobra.Command with Use "history", Args cobra.ExactArgs(1), flags --output (table/json, default table); RunE loads DismissalStore, calls engine.GetRecommendationHistory, renders as table (columns: Timestamp, Action, Reason, Note, Expires) or JSON in `internal/cli/cost_recommendations_history.go`
- [x] T029 [US5] Register history subcommand: call cmd.AddCommand(newRecommendationsHistoryCmd()) in `internal/cli/cost_recommendations.go`

**Checkpoint**: Full audit trail accessible. All lifecycle events visible per recommendation.

---

## Phase 8: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, validation, and quality gates

- [x] T030 [P] Update CLI reference documentation with dismiss, snooze, undismiss, and history subcommands including flag descriptions and examples in `docs/reference/cli-commands.md`
- [x] T031 [P] Add godoc comments to all exported functions, types, and methods in new files: internal/proto/dismissal_reasons.go, internal/config/dismissed.go, internal/cli/cost_recommendations_dismiss.go, internal/cli/cost_recommendations_undismiss.go, internal/cli/cost_recommendations_history.go
- [x] T032 [P] Write integration test for full dismiss lifecycle: dismiss with plugin, list excludes dismissed, include-dismissed shows all, undismiss restores, history shows events in `test/integration/cli/recommendations_dismiss_test.go`
- [x] T033 Run `make lint` and fix all linting issues across all new and modified files
- [x] T034 Run `make test` and verify all tests pass with minimum 80% coverage on new code
- [x] T035 Create follow-up GitHub issues for finfocus-spec enhancements: (1) Add include_dismissed field to GetRecommendationsRequest (#545), (2) Add GetRecommendationHistory RPC (#546)
- [x] T036 [P] Write benchmark test for DismissalStore with 1,000+ dismissal records (SC-008): Load/Save operations, GetDismissedIDs(), CleanExpiredSnoozes() must complete in <100ms in `internal/config/dismissed_benchmark_test.go`

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No-op for existing project
- **Phase 2 (Foundational)**: No dependencies, start immediately. BLOCKS all user stories.
- **Phase 3 (US1+US6)**: Depends on Phase 2 completion. MVP delivery point.
- **Phase 4 (US2)**: Depends on Phase 3 (reuses dismiss infrastructure, adds snooze).
- **Phase 5 (US3)**: Depends on Phase 2 (needs DismissalStore). Can run parallel with Phase 4.
- **Phase 6 (US4)**: Depends on Phase 2 (needs DismissalStore). Can run parallel with Phase 4/5.
- **Phase 7 (US5)**: Depends on Phase 2 (needs DismissalStore). Can run parallel with Phase 4/5/6.
- **Phase 8 (Polish)**: Depends on all user story phases being complete.

### User Story Dependencies

- **US1+US6 (P1)**: Depends on Foundational only. No other story dependencies.
- **US2 (P2)**: Depends on US1+US6 (snooze reuses dismiss infrastructure and CLI file).
- **US3 (P2)**: Depends on Foundational only. Can run parallel with US2.
- **US4 (P2)**: Depends on Foundational only. Can run parallel with US2/US3.
- **US5 (P3)**: Depends on Foundational only. Can run parallel with US2/US3/US4.

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Types/models before engine methods
- Engine methods before CLI commands
- CLI commands before subcommand registration
- Story complete before moving to next priority

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- T003 and T004 can run in parallel (different files)
- T006, T007, T008 can run in parallel (different test files)
- After Phase 3, US3/US4/US5 can all run in parallel (different files, no cross-dependencies)
- T030, T031, T032 can run in parallel (different files)

---

## Parallel Example: Phase 2 (Foundational)

```bash
# Launch both test files in parallel:
Task: T001 "DismissalReason tests in internal/proto/dismissal_reasons_test.go"
Task: T002 "DismissalStore tests in internal/config/dismissed_test.go"

# Then launch both implementations in parallel:
Task: T003 "DismissalReason utilities in internal/proto/dismissal_reasons.go"
Task: T004 "DismissalStore in internal/config/dismissed.go"
```

## Parallel Example: Phase 3 (US1+US6)

```bash
# Launch all test files in parallel:
Task: T006 "Adapter dismiss tests in internal/proto/adapter_test.go"
Task: T007 "Engine dismiss tests in internal/engine/engine_dismiss_test.go"
Task: T008 "CLI dismiss tests in internal/cli/cost_recommendations_dismiss_test.go"
```

---

## Implementation Strategy

### MVP First (Phase 2 + Phase 3 = US1+US6)

1. Complete Phase 2: Foundational utilities and types
2. Complete Phase 3: Dismiss + plugin delegation
3. **STOP and VALIDATE**: `make test && make lint`
4. Test: `finfocus cost recommendations dismiss <id> --reason business-constraint --pulumi-json plan.json`
5. Verify dismissed rec excluded from subsequent list

### Incremental Delivery

1. Phase 2 + Phase 3 -> Dismiss works (MVP)
2. Add Phase 4 -> Snooze works
3. Add Phase 5 -> Audit view works
4. Add Phase 6 -> Undismiss works (full lifecycle)
5. Add Phase 7 -> History works (audit trail)
6. Phase 8 -> Docs, lint, test coverage validated

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 and US6 are combined into Phase 3 because US6 (plugin delegation) is the infrastructure layer of US1 (dismiss CLI)
- Snooze (US2) shares the dismiss CLI file and is implemented as a variant of dismiss with ExpiresAt set
- The DismissalStore is the single shared dependency -- all story phases need it
- `make lint` and `make test` MUST pass before claiming any phase complete
