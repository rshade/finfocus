# Implementation Plan: Plugin Install Version Fallback

**Branch**: `116-plugin-install-fallback` | **Date**: 2026-01-18 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/116-plugin-install-fallback/spec.md`

## Summary

Add fallback behavior to `finfocus plugin install` when a requested version lacks platform assets. In interactive mode, prompt users to accept installation of the latest stable version with compatible assets (defaulting to abort). In automated/CI mode, provide `--fallback-to-latest` flag for silent fallback and `--no-fallback` flag to disable fallback entirely.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Cobra v1.10.2 (CLI), golang.org/x/term (TTY detection), existing `internal/registry` and `internal/tui` packages
**Storage**: N/A (stateless CLI feature)
**Testing**: Go testing + testify, existing test infrastructure in `test/unit/`, `test/integration/`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application
**Performance Goals**: Same as current install behavior (network-bound by GitHub API)
**Constraints**: Must preserve backwards compatibility with existing `plugin install` behavior
**Scale/Scope**: Single command enhancement, ~200-300 lines of new/modified code

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic (CLI layer), not plugin implementation
- [x] **Test-Driven Development**: Tests planned before implementation (80% minimum coverage)
- [x] **Cross-Platform Compatibility**: Uses existing cross-platform TTY detection from `internal/tui`
- [x] **Documentation Synchronization**: CLI help text and README updates planned in same PR
- [x] **Protocol Stability**: No protocol changes required (CLI-only feature)
- [x] **Implementation Completeness**: Full implementation without stubs or TODOs
- [x] **Quality Gates**: All CI checks (tests, lint, security) will pass
- [x] **Multi-Repo Coordination**: No cross-repo dependencies (core-only change)

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/116-plugin-install-fallback/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (CLI contract)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   ├── plugin_install.go       # Modified: Add flags and fallback logic
│   ├── plugin_install_test.go  # Modified: Add fallback tests
│   └── prompt.go               # New: Interactive prompt utilities
├── registry/
│   ├── installer.go            # Modified: Expose fallback search method
│   └── github.go               # Existing: FindReleaseWithAsset() already implements search
└── tui/
    └── detect.go               # Existing: IsTTY() for interactive detection

test/
├── unit/
│   └── cli/
│       └── plugin_install_fallback_test.go  # New: Unit tests for fallback
└── integration/
    └── plugin_install_test.go   # Modified: Integration tests for fallback
```

**Structure Decision**: Single project structure. Modifications primarily in `internal/cli/` with minimal changes to `internal/registry/` to expose existing fallback search capabilities.

## Complexity Tracking

No violations - complexity tracking not required.
