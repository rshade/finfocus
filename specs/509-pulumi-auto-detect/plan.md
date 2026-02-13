# Implementation Plan: Automatic Pulumi Integration for Cost Commands

**Branch**: `509-pulumi-auto-detect` | **Date**: 2026-02-11 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/509-pulumi-auto-detect/spec.md`

## Summary

When `--pulumi-json` and `--pulumi-state` flags are omitted from `finfocus cost projected` and `finfocus cost actual`, automatically detect the Pulumi project in the current directory, resolve the active stack, and execute the appropriate Pulumi CLI commands (`pulumi preview --json` or `pulumi stack export`) to generate input data. This uses `exec.CommandContext()` to shell out to the Pulumi CLI binary (not the Go Automation API), matching existing subprocess patterns in the codebase.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Cobra v1.10.2 (CLI), zerolog v1.34.0 (logging), testify v1.11.1 (testing). No new dependencies.
**Storage**: N/A (stateless CLI invocation)
**Testing**: `go test` with testify, `make test`, `make lint`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go project (existing monorepo structure)
**Performance Goals**: Auto-detection overhead < 100ms; Pulumi CLI execution bounded by user-configured timeouts (5min preview, 60s export)
**Constraints**: No new Go module dependencies; Pulumi CLI is a runtime dependency (not build); must not break existing flag-based workflows
**Scale/Scope**: ~500 new lines of Go code across 3 new files + modifications to 6 existing files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This feature is orchestration logic in the CLI/input layer. It does not add provider-specific integrations to core. Plugins remain the cost data source; this only changes how resource descriptors are obtained (from Pulumi CLI output instead of files).
- [x] **Test-Driven Development**: Tests planned before implementation. Unit tests for new `internal/pulumi/` package (80%+ coverage), updated CLI tests for changed flag behavior, integration test for end-to-end flow. Critical paths (detection, execution, parsing) targeted at 95%.
- [x] **Cross-Platform Compatibility**: Uses `exec.LookPath()` for binary detection (handles PATH differences across OS), `filepath.Join()` for path construction. No Unix-specific assumptions. Windows `.exe` handled automatically.
- [x] **Documentation Integrity**: CLI help text updates planned. `docs/guides/` and `docs/reference/` updates included in scope. Godoc comments required on all new exported functions.
- [x] **Protocol Stability**: No protocol buffer changes. No cross-repo changes needed. Existing ingestion layer formats unchanged.
- [x] **Implementation Completeness**: Full implementation planned — no stubs, no TODOs. All error paths implemented with actionable messages.
- [x] **Quality Gates**: `make lint` and `make test` required before completion. Coverage targets enforced.
- [x] **Multi-Repo Coordination**: No cross-repo dependencies. Only `finfocus-core` is affected.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/509-pulumi-auto-detect/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── cli-interface.md # CLI contract (flags, behavior)
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── pulumi/                    # NEW: Pulumi CLI integration package
│   ├── pulumi.go              # Detection + execution functions
│   ├── errors.go              # Sentinel error types
│   └── pulumi_test.go         # Unit tests (80%+ coverage)
├── ingest/
│   ├── pulumi_plan.go         # MODIFY: Add ParsePulumiPlan() bytes variant
│   ├── pulumi_plan_test.go    # MODIFY: Add bytes-based parsing tests
│   ├── state.go               # MODIFY: Add ParseStackExport() bytes variant
│   └── state_test.go          # MODIFY: Add bytes-based parsing tests
├── cli/
│   ├── cost_projected.go      # MODIFY: Remove MarkFlagRequired, add fallback
│   ├── cost_projected_test.go # MODIFY: Update flag requirement tests
│   ├── cost_actual.go         # MODIFY: Relax validation, add fallback
│   ├── cost_actual_test.go    # MODIFY: Update validation tests
│   ├── common_execution.go    # MODIFY: Add resolveResourcesFromPulumi()
│   └── root.go                # MODIFY: Add --stack persistent flag to cost cmd
test/
└── integration/
    └── pulumi_auto_test.go    # NEW: Integration test with Pulumi fixture
docs/
├── guides/                    # MODIFY: Update quickstart, add Pulumi integration
└── reference/                 # MODIFY: Update CLI reference
```

**Structure Decision**: Follows existing Go package conventions. New `internal/pulumi/` package isolates all Pulumi CLI concerns from the rest of the codebase. CLI layer orchestrates between the new package and existing ingestion/engine layers.

## Complexity Tracking

No constitution violations. No complexity justification needed.
