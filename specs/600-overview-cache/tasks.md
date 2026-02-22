# Tasks: Overview Cost Caching

**Input**: Design documents from `/specs/600-overview-cache/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Go Test Path Conventions

Unit tests for Go projects MUST be colocated with source code, not placed in `test/unit/`.

- **Unit tests**: `internal/[package]/[name]_test.go` (colocated with source)
  - Black-box (public API): `package foo_test`
  - White-box (unexported access): `package foo`
  - Run with: `go test ./internal/...`
- **Integration tests**: `test/integration/` (cross-component, requires running plugins)
  - Run with: `go test ./test/integration/...`
- **E2E tests**: `test/e2e/` (requires built binary and external credentials)
  - Run with: `go test -tags e2e ./test/e2e/...`

> **RETIRED**: `test/unit/` is retired as of issue #732. Do NOT place new Go unit tests
> there -- they will not be discovered by `make test` or CI.

## User Story Mapping

All three user stories are co-implemented by the same code changes. The
`newEngineWithCache()` helper from `internal/cli/common_execution.go` simultaneously
provides: caching (US1), TTL precedence (US2), and cleanup via deferred closure (US3).

| Story | Title | Satisfied By |
|---|---|---|
| US1 (P1) | Cache enrichment results | T001, T004, T005 (engine uses cache; test validates SC-002) |
| US2 (P1) | Opt-in caching with TTL control | T001, T004, T005 (same TTL precedence as other commands) |
| US3 (P2) | Cache cleanup on exit | T001, T004 (`defer cacheCleanup()`) |

---

## Phase 1: Implementation - Wire Cache Into Overview (All Stories)

**Goal**: Replace direct `engine.New()` calls with `newEngineWithCache()` in both
overview execution paths (plain-text and TUI), gaining caching, TTL control, and
cleanup for free via the existing helper.

**Independent Test**: Run `finfocus overview --cache-ttl 300` twice against the same
stack. The second run should show `(cached)` in the adapter field and complete faster.

### Plain-Text Path

- [x] T001 [US1] Replace engine construction with `newEngineWithCache(ctx, cmd, clients, nil)` and add `defer cacheCleanup()` in `executeOverview()` at `internal/cli/overview.go:197-202`, removing the local `cfg := config.New()` variable since `newEngineWithCache` creates it internally

### TUI Path

- [x] T002 [US1] Rename `_ *cobra.Command` to `cmd *cobra.Command` in `runInteractiveOverviewWithInit` signature at `internal/cli/overview.go:718` and add `cmd` as second argument to the `overviewInitAndEnrich` goroutine call at `internal/cli/overview.go:747`

- [x] T003 [US1] Add `cmd *cobra.Command` as second parameter (after `enrichCtx context.Context`) in `overviewInitAndEnrich` function signature at `internal/cli/overview.go:806-814`

- [x] T004 [US1] Replace engine construction with `newEngineWithCache(enrichCtx, cmd, clients, nil)` and add `defer cacheCleanup()` in `overviewInitAndEnrich` at `internal/cli/overview.go:915-919`, removing the local `cfg := config.New()` variable

**Checkpoint**: Both execution paths now use cached engines. US1, US2, and US3 are all satisfied.

---

## Phase 2: Testing

**Purpose**: Validate cache wiring with a dedicated test and confirm no regressions.

- [x] T005 [US1] Write a unit test in `internal/cli/overview_test.go` that exercises the overview plain-text path with `--cache-ttl` set to a positive value. The test MUST verify that a second enrichment pass against the same resources returns `(cached)` in the adapter field (validates SC-002). Use existing mock plugin patterns from other cost command tests in the `cli` package.
- [x] T006 Run `make test` to confirm all existing tests pass and the new test passes with no regressions
- [x] T007 Run `make lint` to confirm code passes linting and formatting checks

---

## Phase 3: Documentation

**Purpose**: Update documentation to reflect the change per Constitution Principle IV (Documentation Integrity).

- [x] T008 Update `docs/commands/overview.md`: add `--cache-ttl` row to the Options table (description: "Cache TTL in seconds; 0 disables caching", default: "0 (disabled)") and add a "With caching enabled" example: `finfocus overview --cache-ttl 300 --pulumi-state state.json`
- [x] T009 Update the engine construction table in `CLAUDE.md` (section "Engine" or "Caching System") to document that the overview command now uses `newEngineWithCache` with cache support in both plain-text and TUI paths

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1**: No external dependencies - all changes are in `internal/cli/overview.go`
- **Phase 2**: Depends on Phase 1 completion (T005 tests the wired cache; T006-T007 validate)
- **Phase 3**: Depends on Phase 1 completion (documents the implemented behavior)

### Task Dependencies Within Phase 1

```text
T001 (plain-text path) ─── independent, can run first or in parallel
T002 (rename cmd param) ──┐
T003 (add cmd to func)  ──┼── T002 and T003 must both complete before T004
T004 (TUI engine swap)  ──┘
```

- T001 is independent of T002-T004 (different function)
- T002 and T003 update the function signature and call site (required before T004)
- T004 uses `cmd` inside `overviewInitAndEnrich` (requires T002+T003)

### Cross-Phase Dependencies

```text
Phase 1 (T001-T004) ──→ Phase 2 (T005-T007)
                    └──→ Phase 3 (T008-T009) can start after Phase 1
```

- T005 (test) requires T001+T004 (needs the cache wiring to exist)
- T006 (make test) requires T005 (new test must be written first)
- T008-T009 (docs) can run in parallel with T005-T007 (different files)

### Parallel Opportunities

T001 can be done in parallel with T002+T003 since they modify different functions.
However, all tasks are in the same file (`overview.go`), so sequential execution
within a single agent is recommended to avoid merge conflicts.

---

## Implementation Strategy

### MVP (All Stories in One Pass)

This is a small wiring-only feature (~20 lines changed). All three user stories
are co-implemented by the same code changes, so there is no meaningful MVP subset.
The recommended approach is:

1. Complete T001 through T004 sequentially (implementation)
2. Write T005 (cache wiring test), then run T006 and T007 to validate
3. Complete T008-T009 for documentation

### Estimated Effort

- **Production code**: ~20 lines changed in 1 file
- **Test code**: ~50-80 lines in `internal/cli/overview_test.go`
- **Documentation**: ~10 lines in `docs/commands/overview.md` + ~5 lines in CLAUDE.md
- **Risk**: Very low - reusing a well-tested helper with no new logic

---

## Notes

- One new test added to `internal/cli/overview_test.go`; all other changes are edits to existing files
- No new dependencies - `newEngineWithCache` is already imported/available in the `cli` package
- Default behavior unchanged - caching is disabled when TTL=0 (the default)
- The `--cache-ttl` persistent flag is already registered on the root command (`root.go:123`)
- Cache key format and BoltDB storage are unchanged from other cost commands
