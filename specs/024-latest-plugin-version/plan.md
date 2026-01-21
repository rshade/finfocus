# Implementation Plan: Latest Plugin Version Selection

**Branch**: `001-latest-plugin-version` | **Date**: 2026-01-06 | **Spec**: [specs/001-latest-plugin-version/spec.md](specs/001-latest-plugin-version/spec.md)
**Input**: Feature specification from `/specs/001-latest-plugin-version/spec.md`

## Summary

The system will implement logic to automatically select the latest version of installed plugins when multiple versions are present, using Semantic Versioning (SemVer) rules. This prevents duplicate cost calculations and ensures users always use the most up-to-date plugin logic available locally. The implementation leverages `github.com/Masterminds/semver/v3` for version comparison and extends the `Registry` to filter discovered plugins.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: `github.com/Masterminds/semver/v3`
**Storage**: Filesystem (`~/.finfocus/plugins/`)
**Testing**: Go `testing` package (Unit tests in `internal/registry`)
**Target Platform**: Linux, macOS, Windows
**Project Type**: CLI / Library (internal package)
**Performance Goals**: <50ms for <100 plugins
**Constraints**: Must work with existing plugin directory structure.
**Scale/Scope**: Local plugin management.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Orchestration logic (Registry) supporting plugins.
- [x] **Test-Driven Development**: Tests planned (Retroactive TDD for existing logic).
- [x] **Cross-Platform Compatibility**: Uses `filepath.Join` and standard libs.
- [x] **Documentation as Code**: Spec and plan updated.
- [x] **Protocol Stability**: No protocol changes.
- [x] **Implementation Completeness**: Goal is to finalize tests and logic.
- [x] **Quality Gates**: CI checks planned.
- [x] **Multi-Repo Coordination**: No cross-repo dependencies for this internal logic.

## Project Structure

### Documentation (this feature)

```text
specs/001-latest-plugin-version/
├── plan.md              # This file
├── research.md          # Implementation analysis
├── data-model.md        # PluginInfo struct
├── quickstart.md        # Verification steps
├── contracts/           # API contracts
│   └── registry-api.md
└── tasks.md             # Task list
```

### Source Code (repository root)

```text
internal/registry/
├── registry.go          # Implementation of ListLatestPlugins
└── registry_test.go     # Unit tests (to be added/expanded)
```

**Structure Decision**: Enhancing existing `internal/registry` package.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |