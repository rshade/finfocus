# Tasks: Recommendation History

**Input**: Design documents from `/specs/1729-recommendation-history/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/
**Tests**: Write the tests in each story before the production change they describe.

## Phase 1: Foundational

- [x] T001 Add optional `Actor` to `config.LifecycleEvent` and a load test for a dismissal file that has no actor, in `internal/config/dismissed.go` and `internal/config/dismissed_test.go`.
- [x] T002 Add `GetRecommendationHistory` to `CostSourceClient` and `clientAdapter` in `internal/proto/adapter.go`, including preservation of status codes, context, and event fields, and skipping of malformed events.
- [x] T003 Give existing `CostSourceClient` test doubles an `Unimplemented` history method so the tree still builds: `internal/proto/adapter_test.go`, `internal/engine/budget_engine_test.go`, `internal/engine/engine_batch_test.go`, `internal/engine/estimate_test.go`, `test/integration/budget_health_test.go`, and `test/integration/budget_tag_filter_test.go`.

## Phase 2: User Story 1 - Read plugin lifecycle history (P1)

- [x] T004 [US1] Adapter tests for a rich event, an empty list, a nil response, an empty id, context, and status codes in `internal/proto/adapter_test.go`.
- [x] T005 [US1] Engine tests for plugin-only history and an empty plugin list in `internal/engine/recommendation_history_test.go`.
- [x] T006 [US1] Show actor in table, JSON, and NDJSON, and describe combined history in `internal/cli/cost_recommendations_history.go` and the command tests.

## Phase 3: User Story 2 - One chronological history (P2)

- [x] T007 [US2] Engine merge tests for ordering, duplicate collapse that keeps an actor, and distinct actors in `internal/engine/recommendation_history_test.go`.
- [x] T008 [US2] Implement the merge in `internal/engine/recommendation_history.go` and stop returning the store slice from `GetRecommendationHistory`.

## Phase 4: User Story 3 - Local history survives plugin trouble (P3)

- [x] T009 [US3] Tests for unsupported plugins, backend errors, cancellation, a nil client, and an unchanged dismissal file in `internal/engine/recommendation_history_test.go`.
- [x] T010 [US3] Adapter tests for a nil event, an invalid timestamp, and an invalid expiry beside a valid sibling in `internal/proto/adapter_test.go`.
- [x] T011 [US3] Open installed plugins from the history command, close them, and keep local history when open fails, using `internal/cli/cost_recommendations_history.go` and `internal/cli/export_test.go`.

## Phase 5: Documentation and gates

- [x] T012 [P] Update `docs/src/content/docs/reference/cli-commands.md` and the history command help so they describe combined history.
- [x] T013 Record the merge rules in `CLAUDE.md`.
- [x] T014 Run `make validate`, `make test`, `make lint`, and `make docs-lint`. Mark this spec Implemented when they pass.

## Dependencies

T001 and T002 block the engine and CLI work. T003 is required for the build.
US1 can land on top of T002 before the dedup rules, but US2 is the merge those
tests lock. US3 extends the same functions. Docs follow the command text.
