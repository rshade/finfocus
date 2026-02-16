# Tasks: Policy-Compatible Cost Output

**Input**: Design documents from `/specs/594-policy-cost-output/`
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

**Purpose**: Create test fixtures and minimal scaffolding

- [x] T001 [P] Create test config fixture with threshold settings in test/fixtures/configs/analyzer-threshold.yaml
- [x] T002 [P] Create empty summary.go file with package declaration in internal/analyzer/summary.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Config extension, Server builder, and project-aware analyzer serve — required by ALL user stories

**CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational Phase (TDD)

- [x] T003 [P] Write tests for AnalyzerConfig MaxMonthlyCost and Enforcement field parsing from YAML in internal/config/config_test.go
- [x] T004 [P] Write tests for FINFOCUS_MAX_MONTHLY_COST and FINFOCUS_ENFORCEMENT env var overrides in internal/config/config_test.go
- [x] T005 [P] Write tests for analyzer config validation (zero threshold, negative threshold, invalid enforcement mode) in internal/config/config_test.go
- [x] T006 [P] Write test for Server WithConfig() builder method preserving backward compatibility in internal/analyzer/server_test.go

### Implementation for Foundational Phase

- [x] T007 Add MaxMonthlyCost float64 and Enforcement string fields to AnalyzerConfig struct with yaml/json tags and defaults (MaxMonthlyCost=0, Enforcement="advisory") in internal/config/config.go
- [x] T008 Add FINFOCUS_MAX_MONTHLY_COST (ParseFloat) and FINFOCUS_ENFORCEMENT (string) overrides in applyEnvOverrides() in internal/config/config.go
- [x] T009 Add validation for MaxMonthlyCost (<=0 warns, treated as disabled) and Enforcement (not in advisory/mandatory warns, defaults to advisory) in Validate() in internal/config/config.go
- [x] T010 Add WithConfig(cfg *config.Config) builder method to Server struct, storing config as a field, in internal/analyzer/server.go
- [x] T011 Modify analyzer serve command to use config.ResolveProjectDir() from CWD and config.NewWithProjectDir() instead of config.New(), then pass config via analyzer.NewServer(eng, version).WithConfig(cfg) in internal/cli/analyzer_serve.go

**Checkpoint**: Config system extended, Server accepts config, analyzer serve is project-aware. User story implementation can begin.

---

## Phase 3: User Story 1 - Cost Threshold Enforcement (Priority: P1) MVP

**Goal**: Enable operators to set a maximum monthly cost threshold that blocks (mandatory) or warns (advisory) when total stack cost exceeds the limit during `pulumi preview`.

**Independent Test**: Configure a threshold of $5,000/mo, run AnalyzeStack with costs totaling $7,500/mo, verify MANDATORY diagnostic is returned that would block deployment.

### Tests for User Story 1 (TDD)

- [x] T012 [P] [US1] Write tests for ThresholdDiagnostic() covering: within-budget ADVISORY, exceeded-advisory WARNING, exceeded-mandatory MANDATORY, message format with cost and threshold values in internal/analyzer/diagnostics_test.go
- [x] T013 [P] [US1] Write tests for AnalyzeStack threshold integration covering: no threshold configured (unchanged behavior), threshold exceeded advisory mode, threshold exceeded mandatory mode, within-budget confirmation, mixed currencies skip enforcement, zero/negative threshold treated as disabled, all resources failed with threshold configured (no threshold diagnostic emitted) in internal/analyzer/server_test.go

### Implementation for User Story 1

- [x] T014 [US1] Implement ThresholdDiagnostic(totalCost, threshold float64, currency, enforcement, version string) returning *pulumirpc.AnalyzeDiagnostic with PolicyName "cost-threshold", configurable EnforcementLevel (ADVISORY/MANDATORY), Severity (MEDIUM/HIGH), and moderate-detail message format in internal/analyzer/diagnostics.go
- [x] T015 [US1] Integrate threshold evaluation into AnalyzeStack(): after computing total cost from cached costs, check if config has MaxMonthlyCost > 0, detect mixed currencies (skip enforcement if mixed), emit ThresholdDiagnostic alongside existing StackSummaryDiagnostic in internal/analyzer/server.go
- [x] T016 [US1] Register "cost-threshold" policy in GetAnalyzerInfo() with enforcement level derived from config (default ADVISORY) in internal/analyzer/server.go

**Checkpoint**: Threshold enforcement fully functional. Configuring max_monthly_cost and enforcement in config or env vars produces correct ADVISORY/MANDATORY diagnostics.

---

## Phase 4: User Story 2 - Structured Cost Summary File (Priority: P2)

**Goal**: Write a structured JSON cost summary file to the project-local `.finfocus/` directory after each AnalyzeStack call, enabling external tools and CI/CD pipelines to consume cost data programmatically.

**Independent Test**: Run AnalyzeStack with mock costs, verify `last-cost-summary.json` is written with correct schema_version, total, currency, resource list, and validate against JSON Schema in contracts/cost-summary-schema.json.

### Tests for User Story 2 (TDD)

- [x] T017 [P] [US2] Write tests for CostSummary and ResourceCost type construction via BuildCostSummary() covering: normal costs, mixed currencies detection, error resources excluded from total, empty cost list, resource count accuracy in internal/analyzer/summary_test.go
- [x] T018 [P] [US2] Write tests for WriteCostSummary() covering: successful write and read-back, atomic overwrite of existing file, directory creation if missing, write failure returns error (read-only dir), file permissions 0o600, JSON schema validity in internal/analyzer/summary_test.go
- [x] T019 [P] [US2] Write tests for AnalyzeStack summary file integration covering: summary file written after successful analysis, write failure does not fail the RPC (graceful degradation), summary file uses project dir from config with global fallback in internal/analyzer/server_test.go

### Implementation for User Story 2

- [x] T020 [US2] Define CostSummary struct (SchemaVersion, Timestamp, Stack, Project, TotalMonthlyCost, Currency, ResourceCount, MixedCurrencies, Resources) and ResourceCost struct (Type, Name, MonthlyCost, Currency, Adapter) with json tags matching contracts/cost-summary-schema.json in internal/analyzer/summary.go
- [x] T021 [US2] Implement BuildCostSummary(costs []engine.CostResult, stack, project string) *CostSummary that aggregates costs, detects mixed currencies, excludes error resources, sets schema_version "1" and RFC 3339 timestamp in internal/analyzer/summary.go
- [x] T022 [US2] Implement WriteCostSummary(summary *CostSummary, dir string) error using atomic write pattern (MkdirAll, temp file, json.MarshalIndent, os.Rename, 0o600 permissions) in internal/analyzer/summary.go
- [x] T023 [US2] Integrate summary file write into AnalyzeStack(): after computing cached costs, call BuildCostSummary() then WriteCostSummary() to project dir (from config) or global fallback, log warning on write failure without failing the RPC in internal/analyzer/server.go

**Checkpoint**: Cost summary file written after every AnalyzeStack. External tools can read `$PROJECT/.finfocus/last-cost-summary.json` and parse cost data.

---

## Phase 5: User Story 3 - Machine-Parseable Diagnostic Metadata (Priority: P3)

**Goal**: Embed structured cost metadata as an HTML comment in per-resource diagnostic messages so that tooling can extract cost data without parsing human-readable text.

**Independent Test**: Run Analyze() for a resource with known cost, extract the `<!-- finfocus:cost:{...} -->` comment from the diagnostic message, parse the JSON, verify monthly cost, currency, and adapter fields match.

### Tests for User Story 3 (TDD)

- [x] T024 [P] [US3] Write tests for FormatCostMetadata() covering: normal cost metadata JSON formatting, zero-cost skip (no metadata appended), metadata parsing roundtrip in internal/analyzer/diagnostics_test.go
- [x] T025 [P] [US3] Write tests for formatCostMessage() backward compatibility covering: message still starts with existing format "Estimated Monthly Cost:", metadata appended as last line, human-readable portion unchanged from current behavior in internal/analyzer/diagnostics_test.go

### Implementation for User Story 3

- [x] T026 [US3] Define CostMetadata struct (Monthly float64, Currency string, Adapter string) and implement FormatCostMetadata(m CostMetadata) string returning HTML comment format `<!-- finfocus:cost:{"monthly":X,"currency":"Y","adapter":"Z"} -->` in internal/analyzer/diagnostics.go
- [x] T027 [US3] Modify formatCostMessage() to append FormatCostMetadata() output as a newline after the human-readable message, skipping metadata for zero-cost internal resources in internal/analyzer/diagnostics.go

**Checkpoint**: All per-resource diagnostics contain embedded metadata. Tooling can extract cost data via the `<!-- finfocus:cost: -->` pattern.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Validation, documentation, and backward compatibility verification

- [x] T028 Run make test to verify all tests pass with 80%+ coverage across analyzer and config packages
- [x] T029 Run make lint to verify all linting passes including markdownlint on new spec files
- [x] T030 [P] Verify backward compatibility: run existing analyzer tests with no threshold configured and confirm behavior is identical to pre-feature baseline (SC-004, SC-006)
- [x] T031 [P] Update CLAUDE.md Active Technologies and Branch-Specific Notes with new env vars (FINFOCUS_MAX_MONTHLY_COST, FINFOCUS_ENFORCEMENT) and config fields (analyzer.max_monthly_cost, analyzer.enforcement)
- [x] T032 [P] Add integration test for full analyzer lifecycle with threshold enforcement in test/integration/analyzer_test.go
- [x] T033 [P] Add Godoc comments for all new exported symbols (CostSummary, ResourceCost, CostMetadata, BuildCostSummary, WriteCostSummary, FormatCostMetadata, ThresholdDiagnostic, WithConfig) in internal/analyzer/summary.go, internal/analyzer/diagnostics.go, internal/analyzer/server.go

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3, 4, 5)**: All depend on Foundational phase completion
  - US1 (P1) should be completed first as MVP
  - US2 (P2) can proceed after US1 or in parallel (different files for most tasks)
  - US3 (P3) is fully independent of US1 and US2 (different functions in diagnostics.go)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Depends on Foundational (Phase 2). No dependencies on other stories. Modifies: diagnostics.go, server.go
- **User Story 2 (P2)**: Depends on Foundational (Phase 2). No dependencies on US1. Modifies: summary.go (new), server.go (additive to AnalyzeStack)
- **User Story 3 (P3)**: Depends on Foundational (Phase 2). No dependencies on US1 or US2. Modifies: diagnostics.go (additive to formatCostMessage)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Type definitions before functions that use them
- Core functions before integration (server wiring)
- Story complete before moving to next priority

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- T003, T004, T005, T006 can run in parallel (different test files)
- T012 and T013 can run in parallel (different test files)
- T017, T018, T019 can run in parallel (different test files)
- T024 and T025 can run in parallel (same file but independent test functions)
- T030, T031, T032 can run in parallel (independent validation tasks)
- US3 (Phase 5) can run in parallel with US2 (Phase 4) if desired

---

## Parallel Example: User Story 1

```bash
# Launch tests in parallel (TDD - write first, expect failures):
Task T012: "Write ThresholdDiagnostic tests in internal/analyzer/diagnostics_test.go"
Task T013: "Write AnalyzeStack threshold integration tests in internal/analyzer/server_test.go"

# Then implement sequentially (each makes its tests pass):
Task T014: "Implement ThresholdDiagnostic() in internal/analyzer/diagnostics.go"
Task T015: "Integrate threshold into AnalyzeStack() in internal/analyzer/server.go"
Task T016: "Register cost-threshold policy in GetAnalyzerInfo() in internal/analyzer/server.go"
```

## Parallel Example: User Story 2

```bash
# Launch tests in parallel (TDD):
Task T017: "Write BuildCostSummary tests in internal/analyzer/summary_test.go"
Task T018: "Write WriteCostSummary tests in internal/analyzer/summary_test.go"
Task T019: "Write AnalyzeStack summary integration tests in internal/analyzer/server_test.go"

# Then implement sequentially:
Task T020: "Define CostSummary/ResourceCost types in internal/analyzer/summary.go"
Task T021: "Implement BuildCostSummary() in internal/analyzer/summary.go"
Task T022: "Implement WriteCostSummary() in internal/analyzer/summary.go"
Task T023: "Integrate summary write into AnalyzeStack() in internal/analyzer/server.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T011) — CRITICAL, blocks all stories
3. Complete Phase 3: User Story 1 (T012-T016)
4. **STOP and VALIDATE**: Run `make test` and `make lint`, verify threshold enforcement works
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Polish phase → Final validation

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (threshold enforcement)
   - Developer B: User Story 2 (summary file)
   - Developer C: User Story 3 (diagnostic metadata)
3. Stories complete and integrate independently
4. Merge order: US1 first (modifies server.go), then US2 (additive), then US3 (additive)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing
- Stop at any checkpoint to validate story independently
- server.go is touched by US1 (T015-T016) and US2 (T023) — implement US1 first to avoid merge conflicts
- diagnostics.go is touched by US1 (T014) and US3 (T026-T027) — implement US1 first, US3 is additive
