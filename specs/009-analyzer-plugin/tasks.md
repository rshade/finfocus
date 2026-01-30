# Tasks: Pulumi Analyzer Plugin Integration

**Input**: Design documents from `/specs/009-analyzer-plugin/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY
and must be written BEFORE implementation. All code changes must maintain minimum 80%
test coverage (95% for critical paths).

**Organization**: Tasks are grouped by user story to enable independent implementation
and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go monorepo**: `internal/` packages at repository root
- **Tests**: Co-located with source files as `*_test.go`
- **Integration tests**: `test/integration/`
- **Fixtures**: `test/fixtures/analyzer/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and dependency setup

- [x] T001 Add Pulumi SDK dependency `github.com/pulumi/pulumi/sdk/v3` to go.mod
- [x] T002 [P] Create `internal/analyzer/` package directory structure
- [x] T003 [P] Create `internal/analyzer/doc.go` with package documentation
- [x] T004 [P] Create `test/fixtures/analyzer/` directory for test data
- [x] T005 Run `go mod tidy` and verify dependencies resolve

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 Extend `internal/config/config.go` with `AnalyzerConfig` struct
- [x] T007 Extend `internal/config/config.go` with `AnalyzerTimeout` struct
- [x] T008 Extend `internal/config/config.go` with `AnalyzerPlugin` struct
- [x] T009 Add analyzer configuration parsing to config loader
- [x] T010 Create `test/fixtures/analyzer/sample-stack.json` with mock resources
- [x] T011 Create `test/fixtures/analyzer/expected-diagnostics.json` for test validation

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Automatic Cost Estimation in Preview (Priority: P1)

**Goal**: Display estimated cost impact directly within `pulumi preview` output

**Independent Test**: Run `pulumi preview` on a project with the analyzer configured
and verify that cost estimates appear in the standard output.

### Tests for User Story 1 (MANDATORY - TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before
> implementation**

- [x] T012 [P] [US1] Unit test for `MapResource` in `internal/analyzer/mapper_test.go`
- [x] T013 [P] [US1] Unit test for `MapResources` in `internal/analyzer/mapper_test.go`
- [x] T014 [P] [US1] Unit test for `extractResourceID` in `internal/analyzer/mapper_test.go`
- [x] T015 [P] [US1] Unit test for `extractProvider` in `internal/analyzer/mapper_test.go`
- [x] T016 [P] [US1] Unit test for `structToMap` in `internal/analyzer/mapper_test.go`
- [x] T017 [P] [US1] Unit test for `CostToDiagnostic` in `internal/analyzer/diagnostics_test.go`
- [x] T018 [P] [US1] Unit test for `StackSummaryDiagnostic` in `internal/analyzer/diagnostics_test.go`
- [x] T019 [P] [US1] Unit test for `formatCostMessage` in `internal/analyzer/diagnostics_test.go`
- [x] T020 [US1] Unit test for `Server.AnalyzeStack` in `internal/analyzer/server_test.go`

### Implementation for User Story 1

- [x] T021 [P] [US1] Implement `MapResource` function in `internal/analyzer/mapper.go`
- [x] T022 [P] [US1] Implement `MapResources` function in `internal/analyzer/mapper.go`
- [x] T023 [P] [US1] Implement `extractResourceID` function in `internal/analyzer/mapper.go`
- [x] T024 [P] [US1] Implement `extractProvider` function in `internal/analyzer/mapper.go`
- [x] T025 [P] [US1] Implement `structToMap` function in `internal/analyzer/mapper.go`
- [x] T026 [P] [US1] Implement `CostToDiagnostic` function in `internal/analyzer/diagnostics.go`
- [x] T027 [P] [US1] Implement `StackSummaryDiagnostic` function in `internal/analyzer/diagnostics.go`
- [x] T028 [P] [US1] Implement `formatCostMessage` function in `internal/analyzer/diagnostics.go`
- [x] T029 [US1] Create `Server` struct in `internal/analyzer/server.go`
- [x] T030 [US1] Implement `NewServer` constructor in `internal/analyzer/server.go`
- [x] T031 [US1] Implement `Server.AnalyzeStack` RPC in `internal/analyzer/server.go`
- [x] T032 [US1] Implement `Server.GetAnalyzerInfo` RPC in `internal/analyzer/server.go`
- [x] T033 [US1] Implement `Server.GetPluginInfo` RPC in `internal/analyzer/server.go`
- [x] T034 [US1] Verify all US1 tests pass with `go test ./internal/analyzer/...`

**Checkpoint**: User Story 1 core logic complete - resources mapped, costs calculated,
diagnostics generated

---

## Phase 4: User Story 2 - Plugin Installation & Handshake (Priority: P1)

**Goal**: Plugin reliably starts and connects to the Pulumi engine

**Independent Test**: Manually run `pulumi-analyzer-cost --serve` and verify it prints
a port and listens on it.

### Tests for User Story 2 (MANDATORY - TDD Required)

- [x] T035 [P] [US2] Unit test for TCP listener binding in `internal/analyzer/server_test.go`
- [x] T036 [P] [US2] Unit test for `Server.Handshake` RPC in `internal/analyzer/server_test.go`
- [x] T037 [P] [US2] Unit test for `Server.ConfigureStack` RPC in `internal/analyzer/server_test.go`
- [x] T038 [P] [US2] Unit test for `Server.Cancel` RPC in `internal/analyzer/server_test.go`
- [x] T039 [US2] Unit test for `analyzer serve` command in `internal/cli/analyzer_test.go`

### Implementation for User Story 2

- [x] T040 [US2] Implement `Server.Handshake` RPC in `internal/analyzer/server.go`
- [x] T041 [US2] Implement `Server.ConfigureStack` RPC in `internal/analyzer/server.go`
- [x] T042 [US2] Implement `Server.Cancel` RPC in `internal/analyzer/server.go`
- [x] T043 [US2] Create `analyzer` command group in `internal/cli/analyzer.go`
- [x] T044 [US2] Implement `analyzer serve` subcommand in `internal/cli/analyzer_serve.go`
- [x] T045 [US2] Implement gRPC server startup with random port in `internal/cli/analyzer_serve.go`
- [x] T046 [US2] Implement stdout port handshake (CRITICAL: only port to stdout)
- [x] T047 [US2] Configure zerolog to use stderr exclusively in analyzer serve command
- [x] T048 [US2] Register `analyzer` command with root command in `internal/cli/root.go`
- [x] T049 [US2] Verify all US2 tests pass with `go test ./internal/cli/... ./internal/analyzer/...`

**Checkpoint**: User Story 2 complete - plugin starts, handshakes, and serves gRPC

---

## Phase 5: User Story 3 - Robust Error Handling (Priority: P2)

**Goal**: Preview completes successfully even if cost estimation fails

**Independent Test**: Simulate a pricing API failure and ensure `pulumi preview`
still finishes with warnings.

### Tests for User Story 3 (MANDATORY - TDD Required)

- [x] T050 [P] [US3] Unit test for plugin timeout handling in `internal/analyzer/server_test.go`
- [x] T051 [P] [US3] Unit test for network failure handling in `internal/analyzer/server_test.go`
- [x] T052 [P] [US3] Unit test for unsupported resource type handling in `internal/analyzer/mapper_test.go`
- [x] T053 [P] [US3] Unit test for invalid resource data handling in `internal/analyzer/mapper_test.go`
- [x] T054 [US3] Unit test for warning diagnostic generation in `internal/analyzer/diagnostics_test.go`

### Implementation for User Story 3

- [x] T055 [US3] Add `MappingError` type to `internal/analyzer/mapper.go`
- [x] T056 [US3] Implement graceful degradation in `MapResources` function
- [x] T057 [US3] Add timeout context handling in `Server.AnalyzeStack`
- [x] T058 [US3] Implement warning diagnostic generation for failures
- [x] T059 [US3] Add debug logging for skipped unsupported resources
- [x] T060 [US3] Verify all US3 tests pass with `go test ./internal/analyzer/...`

**Checkpoint**: User Story 3 complete - failures are graceful, never blocking

---

## Phase 6: Integration Testing

**Purpose**: End-to-end validation across all user stories

- [x] T061 Create integration test for full AnalyzeStack flow in `test/integration/analyzer_test.go`
- [x] T062 Create integration test for handshake protocol in `test/integration/analyzer_test.go`
- [x] T063 Create integration test for error recovery in `test/integration/analyzer_test.go`
- [x] T063.5 Create integration test for SC-003 latency requirement in `test/integration/analyzer_test.go` to verify <2s latency on small stacks.
- [x] T064 Run full test suite: `make test`
- [x] T065 Run linting: `make lint`
- [x] T065.5 Run `govulncheck ./...` to verify no high/critical vulnerabilities (stdlib vulns noted, need Go 1.25.6)
- [x] T066 Verify coverage meets 80% threshold for analyzer package (achieved 92.7%)
- [x] T066.5 Verify 80% docstring coverage for `internal/analyzer` package (all exported symbols documented)

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [x] T067 [P] Add package documentation to `internal/analyzer/doc.go`
- [x] T068 [P] Update `CLAUDE.md` with analyzer package documentation
- [x] T069 [P] Update CLI help text for `analyzer serve` command
- [x] T070 Run quickstart.md validation steps
- [x] T071 Final `make lint && make test` verification
- [x] T072 Verify binary builds on all target platforms (linux, darwin, windows)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational (Phase 2) completion
- **User Story 2 (Phase 4)**: Depends on Foundational (Phase 2) completion
  - Can run in parallel with US1 if desired
- **User Story 3 (Phase 5)**: Depends on US1 and US2 (builds on their implementations)
- **Integration (Phase 6)**: Depends on all user stories complete
- **Polish (Phase 7)**: Depends on Integration phase

### User Story Dependencies

- **User Story 1 (P1)**: Core cost estimation logic - no dependencies on other stories
- **User Story 2 (P1)**: Plugin handshake - can be developed in parallel with US1
- **User Story 3 (P2)**: Error handling - depends on US1/US2 base implementations

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Mapper functions before server methods
- Server methods before CLI commands
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

**Phase 1 (Setup)**:

- T002, T003, T004 can run in parallel

**Phase 3 (US1 Tests)**:

- T012-T019 can all run in parallel (different test functions)

**Phase 3 (US1 Implementation)**:

- T021-T028 can all run in parallel (different functions)

**Phase 4 (US2 Tests)**:

- T035-T038 can run in parallel

**Phase 5 (US3)**:

- T050-T054 tests can run in parallel

---

## Parallel Example: User Story 1

```bash
# Launch all mapper tests for User Story 1 together:
Task: "Unit test for MapResource in internal/analyzer/mapper_test.go"
Task: "Unit test for MapResources in internal/analyzer/mapper_test.go"
Task: "Unit test for extractResourceID in internal/analyzer/mapper_test.go"
Task: "Unit test for extractProvider in internal/analyzer/mapper_test.go"
Task: "Unit test for structToMap in internal/analyzer/mapper_test.go"

# Launch all diagnostics tests together:
Task: "Unit test for CostToDiagnostic in internal/analyzer/diagnostics_test.go"
Task: "Unit test for StackSummaryDiagnostic in internal/analyzer/diagnostics_test.go"
Task: "Unit test for formatCostMessage in internal/analyzer/diagnostics_test.go"

# Launch all mapper implementations together:
Task: "Implement MapResource function in internal/analyzer/mapper.go"
Task: "Implement MapResources function in internal/analyzer/mapper.go"
Task: "Implement extractResourceID function in internal/analyzer/mapper.go"
Task: "Implement extractProvider function in internal/analyzer/mapper.go"
Task: "Implement structToMap function in internal/analyzer/mapper.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (core cost estimation)
4. Complete Phase 4: User Story 2 (plugin handshake)
5. **STOP and VALIDATE**: Test with actual `pulumi preview`
6. Deploy/demo if ready - MVP complete!

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Costs display in diagnostics (partial MVP)
3. Add User Story 2 → Test independently → Full `pulumi preview` integration (MVP!)
4. Add User Story 3 → Test independently → Robust error handling
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (mapper, diagnostics, AnalyzeStack)
   - Developer B: User Story 2 (handshake, CLI, server startup)
3. After US1+US2: Developer A+B: User Story 3 (error handling)
4. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- **CRITICAL**: stdout is reserved for port handshake only - all logs to stderr
- **TDD Required**: Tests written first per Constitution Principle II
