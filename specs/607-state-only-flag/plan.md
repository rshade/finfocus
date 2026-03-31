# Implementation Plan: State-Only Flag for Overview Command

**Branch**: `607-state-only-flag` | **Date**: 2026-03-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/607-state-only-flag/spec.md`

## Summary

Add a `--state-only` boolean flag to the `overview` command that unconditionally
skips `pulumi preview --json`, eliminating ~15 seconds of latency (83% of total
overview time) for users who only need cost data. The implementation reuses the
existing `isStateOnly` code path — the flag short-circuits the preview decision
logic rather than introducing new downstream behavior.

## Technical Context

**Language/Version**: Go 1.25.8 (see `go.mod`)
**Primary Dependencies**: Cobra (CLI framework), Bubble Tea (TUI), zerolog (logging)
**Storage**: N/A (no storage changes)
**Testing**: `go test` with testify (assert/require), table-driven tests
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application
**Performance Goals**: `--state-only` overview in <5 seconds on 8-resource stack
**Constraints**: Zero regression to existing behavior when flag is not set
**Scale/Scope**: 1 file modified (`overview.go`), 2 test files updated, 2 doc files updated

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic (CLI flag + data loading), not a plugin. No plugin changes needed.
- [x] **Test-Driven Development**: Tests planned for flag parsing, `resolveIsStateOnly` with `stateOnly=true`, flag conflict validation, and help text verification. No TUI view changes → no golden file updates needed.
- [x] **Cross-Platform Compatibility**: Boolean flag parsing is platform-agnostic. No OS-specific code.
- [x] **Documentation Integrity**: `docs/commands/overview.md` and command help text will be updated in the same PR.
- [x] **Protocol Stability**: No protocol buffer changes.
- [x] **Implementation Completeness**: Full implementation — no stubs, TODOs, or deferred work.
- [x] **Quality Gates**: `make test` and `make lint` will pass before completion.
- [x] **Multi-Repo Coordination**: Single-repo change (finfocus core only). No spec or plugin changes.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/607-state-only-flag/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (minimal — no new entities)
├── quickstart.md        # Phase 1 output
└── contracts/           # Phase 1 output (CLI contract only)
```

### Source Code (repository root)

```text
internal/cli/
├── overview.go                       # Add stateOnly field, flag registration, validation, short-circuit logic
├── overview_test.go                  # Add flag parsing, conflict validation, help text tests
└── overview_phase_internal_test.go   # Add resolveIsStateOnly tests with stateOnly=true

docs/commands/
└── overview.md                       # Add --state-only to options table and examples
```

**Structure Decision**: All changes are within the existing `internal/cli/` package
and `docs/commands/` directory. No new files or packages needed — this feature adds
a field to an existing struct, a flag to an existing command, and early-return logic
to existing functions.

## Complexity Tracking

No violations — no complexity justification needed.
