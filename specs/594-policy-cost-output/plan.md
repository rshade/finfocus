# Implementation Plan: Policy-Compatible Cost Output

**Branch**: `594-policy-cost-output` | **Date**: 2026-02-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/594-policy-cost-output/spec.md`

## Summary

Enable cost-based policy enforcement by extending the analyzer to: (1) write a structured cost summary file after each AnalyzeStack call, (2) support configurable cost thresholds with advisory/mandatory enforcement modes, and (3) embed machine-parseable metadata in diagnostic messages. The implementation extends existing config, analyzer server, and diagnostic infrastructure without protocol changes.

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: Pulumi SDK v3.220.0 (EnforcementLevel protobuf), cobra, zerolog, finfocus-spec v0.5.6
**Storage**: JSON file (`last-cost-summary.json`) using atomic write pattern (temp file + rename)
**Testing**: `go test` with testify (assert/require), 80% minimum coverage
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go project with existing package structure
**Performance Goals**: Summary file write adds <10ms overhead to AnalyzeStack
**Constraints**: Stdout reserved for port handshake in analyzer; all output to stderr or diagnostics
**Scale/Scope**: Single summary file per project, typical stacks have 10-500 resources

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: This is orchestration logic in core (analyzer server + config). No direct provider integrations added. Plugins are unchanged.
- [x] **Test-Driven Development**: Tests planned for all new code. Target 80%+ coverage for config, diagnostics, and summary file. 95% for threshold enforcement logic.
- [x] **Cross-Platform Compatibility**: Uses `filepath.Join()` for paths, `os.MkdirAll()` for directories, `os.Rename()` for atomic writes. All cross-platform compatible. No platform-specific code.
- [x] **Documentation Integrity**: Quickstart guide produced. CLAUDE.md update planned for new env vars and config fields. Godoc for all exported symbols.
- [x] **Protocol Stability**: No protocol buffer changes. Uses existing Pulumi SDK enforcement levels (ADVISORY, MANDATORY). No finfocus-spec changes needed.
- [x] **Implementation Completeness**: Full implementation planned — no stubs or TODOs. All three parts (summary file, threshold, metadata) are delivered together.
- [x] **Quality Gates**: `make test` and `make lint` required before completion. CI checks maintained.
- [x] **Multi-Repo Coordination**: No cross-repo changes. Feature is entirely within finfocus-core. Existing finfocus-spec v0.5.6 is sufficient.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/594-policy-cost-output/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 research decisions
├── data-model.md        # Entity definitions and relationships
├── quickstart.md        # User-facing getting started guide
├── contracts/
│   ├── cost-summary-schema.json    # JSON Schema for summary file
│   └── diagnostic-metadata.md      # Metadata embedding contract
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── analyzer/
│   ├── server.go              # MODIFY: Add WithConfig(), extend AnalyzeStack()
│   ├── server_test.go         # MODIFY: Add threshold + summary tests
│   ├── diagnostics.go         # MODIFY: Add metadata, threshold diagnostic
│   ├── diagnostics_test.go    # MODIFY: Add metadata + threshold tests
│   ├── summary.go             # NEW: CostSummary type + WriteSummary()
│   └── summary_test.go        # NEW: Summary file write/read tests
├── config/
│   ├── config.go              # MODIFY: Extend AnalyzerConfig struct
│   └── config_test.go         # MODIFY: Add threshold config tests
└── cli/
    └── analyzer_serve.go      # MODIFY: Pass config to server, project-aware

test/
├── integration/
│   └── analyzer_test.go       # MODIFY: Add threshold integration tests
└── fixtures/
    └── configs/
        └── analyzer-threshold.yaml  # NEW: Test config with threshold
```

**Structure Decision**: Extends existing Go package structure. New file `summary.go` in `internal/analyzer/` for cost summary types and file writing logic, keeping the analyzer package cohesive. Config changes go in existing `config.go`. No new packages needed.

## Implementation Approach

### Phase 1: Config Extension (FR-002, FR-003, FR-003a, FR-004, FR-012)

**Files**: `internal/config/config.go`, `internal/config/config_test.go`

1. Add `MaxMonthlyCost float64` and `Enforcement string` fields to `AnalyzerConfig` struct
2. Add defaults in `newDefaultConfig()`: MaxMonthlyCost=0 (disabled), Enforcement="advisory"
3. Add env var overrides in `applyEnvOverrides()`:
   - `FINFOCUS_MAX_MONTHLY_COST` → `c.Analyzer.MaxMonthlyCost`
   - `FINFOCUS_ENFORCEMENT` → `c.Analyzer.Enforcement`
4. Add validation in `Validate()`:
   - MaxMonthlyCost <= 0 → warning log, treated as disabled
   - Enforcement not in {"advisory", "mandatory"} → warning log, default to "advisory"
5. Tests: config parsing, env var precedence, validation edge cases

### Phase 2: Cost Summary File (FR-001, FR-010, FR-011, FR-013)

**Files**: `internal/analyzer/summary.go`, `internal/analyzer/summary_test.go`

1. Define `CostSummary` and `ResourceCost` structs matching `contracts/cost-summary-schema.json`
2. Implement `BuildCostSummary(costs []engine.CostResult, stack, project string) *CostSummary`
   - Aggregates costs, detects mixed currencies, counts resources
   - Excludes resources with errors from total
3. Implement `WriteCostSummary(summary *CostSummary, dir string) error`
   - Atomic write: temp file + `os.Rename()`
   - File permissions: `0o600`
   - Directory creation: `os.MkdirAll(dir, 0o750)`
4. Tests: build summary from mock costs, write/read roundtrip, write failure handling, mixed currencies

### Phase 3: Diagnostic Metadata (FR-009)

**Files**: `internal/analyzer/diagnostics.go`, `internal/analyzer/diagnostics_test.go`

1. Define `CostMetadata` struct: `Monthly float64`, `Currency string`, `Adapter string`
2. Implement `FormatCostMetadata(m CostMetadata) string` → HTML comment JSON
3. Modify `formatCostMessage()` to append metadata after human-readable text
4. Skip metadata for zero-cost internal resources (no useful data to embed)
5. Tests: metadata formatting, parsing roundtrip, backward compatibility of message prefix

### Phase 4: Threshold Enforcement (FR-005, FR-006, FR-007, FR-008)

**Files**: `internal/analyzer/diagnostics.go`, `internal/analyzer/server.go`, `internal/analyzer/server_test.go`

1. Implement `ThresholdDiagnostic(totalCost, threshold float64, currency, enforcement, version string) *pulumirpc.AnalyzeDiagnostic`
   - PolicyName: "cost-threshold"
   - EnforcementLevel: ADVISORY (within budget or advisory mode) / MANDATORY (exceeded + mandatory mode)
   - Severity: MEDIUM (within) / HIGH (exceeded)
   - Message: moderate detail format per clarification
2. Add `WithConfig(cfg *config.Config)` builder method to Server
3. Modify `AnalyzeStack()` to:
   - Check if threshold is configured (MaxMonthlyCost > 0)
   - Skip if mixed currencies detected (FR-013)
   - Emit ThresholdDiagnostic alongside existing StackSummaryDiagnostic
4. Tests: threshold within/exceeded, advisory/mandatory modes, no threshold, mixed currencies, zero threshold

### Phase 5: Server Integration and Summary Write (wiring)

**Files**: `internal/analyzer/server.go`, `internal/cli/analyzer_serve.go`

1. Modify `AnalyzeStack()` to call `WriteCostSummary()` after computing costs
   - Use project dir from config or CWD fallback
   - Log warning on write failure, don't fail the RPC
2. Modify `analyzer_serve.go` to:
   - Use `config.ResolveProjectDir()` from CWD to find project context
   - Use `config.NewWithProjectDir()` instead of `config.New()`
   - Pass config to server via `analyzer.NewServer(eng, version).WithConfig(cfg)`
3. Register "cost-threshold" policy in `GetAnalyzerInfo()` with configurable enforcement level
4. Tests: end-to-end AnalyzeStack with threshold + summary file write

### Phase 6: Validation and Documentation

1. Run `make test` — all tests pass
2. Run `make lint` — all linting passes
3. Update CLAUDE.md with new env vars and config fields
4. Verify backward compatibility: no threshold configured → identical behavior

## Key Design Decisions

| Decision | Choice | Rationale | Reference |
| -------- | ------ | --------- | --------- |
| Enforcement mechanism | Pulumi MANDATORY level | Cannot be overridden, protocol-native | [R1](research.md#r1-pulumi-enforcement-level-system) |
| Metadata format | HTML comment in Message | No Tags field on AnalyzeDiagnostic | [R2](research.md#r2-diagnostic-metadata-embedding) |
| Summary file location | Project-local with global fallback | Data isolation between projects | [R3](research.md#r3-summary-file-location-and-project-awareness) |
| Config extension | Fields on AnalyzerConfig | Consistent with existing pattern | [R4](research.md#r4-configuration-extension-pattern) |
| File write pattern | Atomic (temp + rename) | Battle-tested in DismissalStore | [R5](research.md#r5-atomic-file-writing-pattern) |
| Server config access | WithConfig() builder | Backward compatible, extensible | [R6](research.md#r6-server-constructor-extension) |
| Threshold check placement | In AnalyzeStack() | Total cost known only after all resources analyzed | [R7](research.md#r7-threshold-diagnostic-placement) |

## Risk Mitigation

| Risk | Impact | Mitigation |
| ---- | ------ | ---------- |
| Analyzer subprocess CWD differs from project dir | Summary file written to wrong location | Fallback to `~/.finfocus/`, log warning with actual CWD |
| MANDATORY enforcement blocks CI/CD unexpectedly | Deployment pipeline failure | Default to advisory mode; mandatory requires explicit opt-in |
| Summary file write fails in read-only filesystem | Missing cost data for external tools | Graceful failure: log warning, continue with diagnostics |
| Mixed currencies in multi-provider stack | Incorrect threshold comparison | Skip enforcement entirely, flag in summary file |
