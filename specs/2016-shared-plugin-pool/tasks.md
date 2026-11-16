# Tasks: Shared Plugin Processes Across Web Stacks

**Input**: Design documents from `specs/2016-shared-plugin-pool/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/internal-api.md

**Tests**: Required (Constitution II). Write each test first and watch it fail.

## Phase 1: Foundational

- [x] T001 Split `NewClient` into launch-input resolution and launch so the
  pool can key on the same inputs without resolving them twice, in
  `internal/pluginhost/host.go` and `internal/pluginhost/settings_env.go`

## Phase 2: User Story 1 - one process per plugin across stacks (P1)

- [x] T002 [US1] Write failing tests in `internal/pluginhost/pool_test.go`:
  two acquires with the same key launch once; concurrent acquires launch once;
  each holder's `Metadata` is its own copy
- [x] T003 [US1] Implement `Pool`, `NewPool`, `WithPool`, `PoolFromContext`,
  `Acquire`, holds, and holder clients in `internal/pluginhost/pool.go`
- [x] T004 [US1] Use the context's pool in `registry.openPlugin`
  (`internal/registry/registry.go`) with a test in
  `internal/registry/registry_pool_test.go`
- [x] T005 [US1] Create, attach, and close the pool in `runWebStacks`
  (`internal/cli/web_multistack.go`), with a test that every stack's pipeline
  and session context carry the pool, which is how reloads and the scorer
  (FR-008) reach it

## Phase 3: User Story 2 - isolation by environment (P1)

- [x] T006 [US2] Test in `internal/pluginhost/pool_test.go`: a different
  launch environment, and a different ESC offer for an advertised setting
  (same launch environment), each launch a separate process whose environment
  holds no other stack's value
- [x] T007 [US2] Key on binary path, launch environment, and ESC offer in
  `internal/pluginhost/pool.go`

## Phase 4: User Story 3 - lifecycle (P1)

- [x] T008 [US3] Tests in `internal/pluginhost/pool_test.go`: cancelling the
  launching context does not stop a shared process; the last release closes
  it once; a requester cancelled mid-launch leaves no process behind; a failed
  launch reaches every waiter and is retried by the next acquire; a dead
  process is replaced on the next acquire; `Close` stops held processes and
  rejects later acquires
- [x] T009 [US3] Implement pool-owned launch contexts, eviction, failure
  removal, and `Close` in `internal/pluginhost/pool.go`
- [x] T010 [US3] Integration test: multi-stack dashboard SIGTERM leaves no
  plugin process (`test/integration/web_sigterm_orphan_test.go`)

## Phase 5: User Story 4 - unchanged single-stack and CLI (P2)

- [x] T011 [US4] Test that a context without a pool keeps `NewClient`
  behavior (one launch per open, `DeliveredDigest` unchanged) in
  `internal/registry/registry_pool_test.go`

## Phase 6: Polish

- [x] T012 Document the sharing rule in
  `docs/src/content/docs/architecture/web-ui.md` and `CLAUDE.md`
- [x] T013 Run `make validate`, `make test`, `make test-race`, `make lint`,
  `make docs-lint`, `make test-e2e-web`
- [x] T014 Manual check with `--stack dev,test` on initial-30 (quickstart.md;
  `../finfocus-demo` has only a `dev` stack), counting plugin processes with
  `ps` against an `origin/main` build and comparing both stacks' snapshots
- [x] T016 [US2] Learn each binary's advertised settings and key later
  requests on the final environment; wait for a binary's first launch instead
  of launching beside it. Tests: ESC values the plugin never receives, a wrong
  prediction, and concurrent first launches with different offers
  (`internal/pluginhost/pool_test.go`)
- [x] T015 Mark spec `Status: Implemented`

## Web Parity

Web UI Fit is `Not applicable` (spec): no hook, render check, or `web-ui`
follow-up issue is required.

## Dependencies

T001 → T002/T003 → T004/T005 → T006/T007 → T008/T009 → T010/T011 → T012-T015.
