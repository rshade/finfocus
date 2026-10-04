# Tasks: Pass include_dismissed From cost recommendations

**Input**: Design documents from `specs/622-include-dismissed/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md, and finfocus-spec
field `include_dismissed` (specs/599-include-dismissed)

**Tests**: Engine and adapter tests before the production wiring they cover.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [x] T001 Pin finfocus-spec to the commit that adds `include_dismissed` (replace while iterating,
  then the module version for the PR)

## Phase 2: User Story 1 - The flag reaches the plugin (P1)

- [x] T002 [US1] Write the adapter test that `IncludeDismissed` true is copied and excluded IDs
  remain, and that false stays false, in `internal/proto/adapter_recommendations_test.go`
- [x] T003 [US1] Add `IncludeDismissed` to the internal request and copy it in
  `internal/proto/adapter.go`
- [x] T004 [US1] Write the engine test that the flagged method sets the field, keeps excluded IDs,
  and the default method does not set the field
- [x] T005 [US1] Add `GetRecommendationsForResourcesWithDismissed`, and set the field on both the
  sequential and batch request builders in `internal/engine/engine.go`
- [x] T006 [US1] Call the flagged method from `fetchRecommendationsWithProgress` when
  `--include-dismissed` is set

## Phase 3: User Story 2 - Cache isolation (P1)

- [x] T007 [US2] Assert the flagged and default fetches miss each other's cache, and a repeated
  flagged fetch hits
- [x] T008 [US2] Suffix the cache key with `/include-dismissed` only when the flag is set

## Phase 4: User Story 3 - Docs (P2)

- [x] T009 [P] [US3] Update the flag help, the command reference, both roadmap checkboxes for #545,
  and the CLAUDE.md recommendations note

## Phase 5: Polish

- [x] T010 Run `make test` and `make lint`

## Dependencies

T001 blocks every test that names the generated getter. T002 precedes T003. T004 precedes T005.
T007 precedes T008. T006 needs T005. T009 can proceed once the flag behavior is settled.
