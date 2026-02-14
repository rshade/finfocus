# Implementation Plan: Neo-Friendly CLI Fixes

**Branch**: `590-neo-cli-fixes` | **Date**: 2026-02-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/590-neo-cli-fixes/spec.md`

## Summary

Fix three CLI gaps that prevent reliable use by AI agents (Pulumi Neo):
(1) Propagate custom budget exit codes through `main()` instead of always exiting 1,
(2) Add structured error objects to JSON/NDJSON output with stable error codes, and
(3) Add `--output json` support to the `plugin list` command. All changes are additive
and preserve existing table output behavior.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Cobra v1.10.2 (CLI), gRPC v1.78.0 (plugins), finfocus-spec v0.5.6 (protocol)
**Storage**: N/A (stateless CLI)
**Testing**: `go test` with testify v1.11.1
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module
**Performance Goals**: No performance regression; all changes are in output rendering paths
**Constraints**: Zero breaking changes to table output; error codes are stable across versions
**Scale/Scope**: 3 user stories, ~6 files modified, ~200 lines of new code

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Feature is in CLI/engine orchestration layer, not
  provider-specific. No plugin changes needed.
- [x] **Test-Driven Development**: Tests planned for all three stories. Coverage targets:
  80% overall, 95% for exit code path in `main.go`.
- [x] **Cross-Platform Compatibility**: All changes are pure Go with no platform-specific
  code. Exit codes work identically on Linux/macOS/Windows.
- [x] **Documentation Integrity**: CLAUDE.md will be updated with new error code
  constants and plugin list JSON support. No external docs changes needed.
- [x] **Protocol Stability**: No protocol buffer changes. Error codes are defined in
  core only. The four error code strings are committed as stable API.
- [x] **Implementation Completeness**: All three stories will be fully implemented with
  no TODOs or stubs.
- [x] **Quality Gates**: `make test` and `make lint` will pass before completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. Error codes are
  core-internal, not in finfocus-spec.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/590-neo-cli-fixes/
├── plan.md              # This file
├── research.md          # Phase 0 research findings
├── data-model.md        # Entity definitions and relationships
├── quickstart.md        # Verification guide
├── contracts/           # API contracts
│   ├── exit-codes.md    # Exit code behavior contract
│   ├── structured-error.md  # JSON error object schema
│   └── plugin-list-json.md  # Plugin list JSON schema
└── tasks.md             # (Created by /speckit.tasks)
```

### Source Code (files to modify)

```text
cmd/finfocus/
└── main.go                    # Story 1: Exit code extraction with errors.As()

internal/cli/
├── cost_actual.go             # Story 1: Fix budget error return (bug)
├── cost_budget.go             # Story 1: Export BudgetExitError (if needed)
└── plugin_list.go             # Story 3: Add --output json flag and JSON rendering

internal/engine/
├── types.go                   # Story 2: Add StructuredError type and CostResult.Error field
└── overview_enrich.go         # Story 2: Update error detection to use StructuredError

internal/tui/
└── cost_view.go               # Story 2: Update error detection to use StructuredError

internal/proto/
└── adapter.go                 # Story 2: Populate StructuredError at error origins
```

### Test Code (files to add/modify)

```text
cmd/finfocus/
└── main_test.go               # Story 1: Test exit code extraction

internal/cli/
├── cost_budget_test.go        # Story 1: Test budget error propagation
└── plugin_list_test.go        # Story 3: Test JSON output rendering

internal/engine/
└── types_test.go              # Story 2: Test StructuredError serialization

internal/proto/
└── adapter_test.go            # Story 2: Test error code assignment
```

**Structure Decision**: Single Go module with changes concentrated in 8 source files
across 5 packages. No new packages or modules needed.

## Implementation Design

### Story 1: Semantic Exit Codes (P1)

**Changes**:

1. `cmd/finfocus/main.go`: In `main()`, use `errors.As(err, &budgetErr)` to check
   for `*cli.BudgetExitError`. If matched, call `os.Exit(budgetErr.ExitCode)`.
   Otherwise, fall through to `os.Exit(1)`.

2. `internal/cli/cost_actual.go:247-249`: Change from logging-only to returning
   the `BudgetExitError`, matching the pattern in `cost_projected.go:247-249`.

3. `internal/cli/cost_actual.go:242`: Remove the table-only output format guard
   (`params.output == outputFormatTable`) so budget exit codes propagate in all
   output modes (JSON, NDJSON, table), matching `cost_projected.go` which has
   no output format guard.

4. `internal/cli/cost_budget.go`: Verify `BudgetExitError` is exported (it already
   is with uppercase name). No changes needed if already exported.

**Testing**:

- Unit test in `main_test.go`: Verify `run()` returns `*BudgetExitError` with
  custom exit codes, and test the `errors.As` extraction logic.
- Unit test in `cost_budget_test.go`: Verify both `cost projected` and `cost actual`
  paths return the error correctly.

### Story 2: Structured Error Objects (P2)

**Changes**:

1. `internal/engine/types.go`: Add `StructuredError` struct with `Code`, `Message`,
   `ResourceType` fields. Add error code constants. Add `Error *StructuredError`
   field to `CostResult`.

2. `internal/proto/adapter.go`: At each error origin point, create a `StructuredError`
   and set it on the `CostResult`. Detect `context.DeadlineExceeded` for timeout
   classification. Set `Notes` to the human-readable message without the prefix
   when a `StructuredError` is present.

3. `internal/engine/engine.go`: At the "No pricing information available" path,
   create a `StructuredError` with `NO_COST_DATA` code.

4. `internal/tui/cost_view.go` and `internal/engine/overview_enrich.go`: Update
   error detection from `strings.HasPrefix(result.Notes, "ERROR:")` to
   `result.Error != nil` so table output and overview enrichment continue to
   detect errors after Notes prefix stripping (FR-009 compliance).

**Testing**:

- Unit test in `types_test.go`: Verify JSON serialization of `StructuredError`.
- Unit test in `adapter_test.go`: Verify error code assignment for each category.
- Integration: Verify JSON output contains structured errors and Notes field is clean.

### Story 3: Plugin List Structured Output (P3)

**Changes**:

1. `internal/cli/plugin_list.go`: Add `--output` string flag (default `"table"`).
   In `runPluginListCmd`, check the output format. For `"json"`, marshal the
   `[]enrichedPluginInfo` slice to JSON. Handle empty list as `[]`.

**Testing**:

- Unit test in `plugin_list_test.go`: Verify JSON output structure, empty array
  case, and failed plugin metadata case.

## Complexity Tracking

> No Constitution violations. No complexity justification needed.
