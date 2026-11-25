# Tasks: Per-Request Credentials for Pooled Plugins

**Input**: Design documents from `specs/2023-per-request-credentials/`

**Tests**: Required (Constitution II). Write each test first and watch it fail.

## Phase 1: Foundational

- [x] T001 Tests then `Credentials` in `internal/config/esc.go` and
  `internal/esc/source.go`: JSON load, clone, options copy, accessor, and
  validation messages that repeat no entry (FR-001, FR-011)
- [x] T002 Tests then the implied allowlist in
  `internal/pluginhost/settings_env.go` (FR-002, FR-013)
- [x] T003 Widen `Client.Conn` and `proto.NewCostSourceClient` to
  `grpc.ClientConnInterface`

## Phase 2: User Story 1 - share and attach (P1)

- [x] T004 [US1] Failing tests in `internal/pluginhost/pool_credentials_test.go`
  with a plugin served by `pluginsdk` that reports `ExtractCredentials`:
  one shared process, per-call sets, concurrent calls, no value in the process
  environment, a stack with no credentials (FR-003, FR-004, FR-005, FR-008)
- [x] T005 [US1] Plan, withheld resolution, launch and restart, key flag, and
  holder connection in `internal/pluginhost/` (`credentials.go`, `host.go`,
  `settings_env.go`, `pool.go`)
- [x] T006 [US1] Test that a service built from `Client.Conn` carries the set
  (FR-004)

## Phase 3: User Story 2 - isolation (P1)

- [x] T007 [US2] Tests: no support, no map, no pool, an undeclared difference,
  an unusable set, a changed map, and support lost after launch (FR-006,
  FR-007, FR-013, FR-014)

## Phase 4: User Story 3 - records and caches (P1)

- [x] T008 [US3] Tests then masking in `TraceInterceptor`: log, error text,
  code, details, and the OpenTelemetry span (FR-009)
- [x] T009 [US3] Tests then the holder digest: it differs between stacks and
  between maps, and equals the isolated digest when calls carry none (FR-010)
- [x] T010 [US3] Tests then the dial guard in `internal/pluginhost/process.go`
  (FR-012)
- [x] T014 [US3] Tests then the port proof in `internal/pluginhost/process.go`
  and `host.go`, and the unproven binary in `pool.go` (FR-015)

## Phase 5: Polish

- [x] T011 Document in `docs/src/content/docs/guides/esc-credentials.md`,
  `docs/src/content/docs/reference/config-reference.md`, and `CLAUDE.md`
- [x] T012 `make validate`, `make test`, `make test-race`, `make lint`,
  `make docs-lint`, `make test-e2e-web`, integration tests for the touched
  boundaries. The kubernetes and prometheus modules do not build on
  `origin/main` (9a411d3a), so their tests and lint could not run
- [x] T013 Set the spec status to Implemented
