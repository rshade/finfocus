# Implementation Plan: Project Rename to FinFocus

**Branch**: `113-rebrand-to-finfocus` | **Date**: 2026-01-14 | **Spec**: [specs/113-rebrand-to-finfocus/spec.md](spec.md)
**Input**: Feature specification from `specs/113-rebrand-to-finfocus/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Rename the entire `finfocus` ecosystem to `finfocus`. This includes renaming the CLI binary, Go module, configuration directory (`~/.finfocus` -> `~/.finfocus`), environment variables (`FINFOCUS_*` -> `FINFOCUS_*`), and internal branding. It also implements a migration path for existing users and legacy compatibility toggles.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: `github.com/spf13/cobra` (CLI), `github.com/spf13/viper` (Config), `github.com/rshade/finfocus-spec` (renamed from `finfocus-spec`)
**Storage**: Filesystem (`~/.finfocus/config.yaml`, `~/.finfocus/plugins/`)
**Testing**: Go standard library testing + `testify`
**Target Platform**: Linux, macOS, Windows (Cross-platform support required)
**Project Type**: Single CLI application
**Performance Goals**: N/A (Branding change)
**Constraints**: Zero data loss during config migration. Backward compatibility for plugins via toggle.
**Scale/Scope**: Entire repository (module rename).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This change supports the plugin architecture by renaming the host.
- [x] **Test-Driven Development**: Tests planned for migration logic and config loading.
- [x] **Cross-Platform Compatibility**: Path handling MUST use `filepath.Join` and user home dir resolution compatible with all OSs.
- [x] **Documentation Synchronization**: README and docs will be updated with new names.
- [x] **Protocol Stability**: Breaking change (v0.2.0) explicitly managed via semantic versioning.
- [x] **Implementation Completeness**: All renames will be complete; no "TODO" renaming left behind.
- [x] **Quality Gates**: CI will be updated to enforce quality on the new codebase.
- [x] **Multi-Repo Coordination**: `finfocus-spec` is already released (v0.5.1); this plan updates Core to use it.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/113-rebrand-to-finfocus/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cmd/
└── finfocus/            # Renamed from finfocus
    └── main.go
internal/
├── config/              # Updated config loading logic
├── cli/                 # Updated branding text
└── migration/           # NEW: Migration logic
```

**Structure Decision**: Renaming `cmd/finfocus` to `cmd/finfocus` is the primary structural change. A new `internal/migration` package will encapsulate the one-time migration logic to keep it isolated.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | | |
