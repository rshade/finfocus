# Implementation Plan: Split Project-Local and User-Global Configuration

**Branch**: `591-config-split` | **Date**: 2026-02-14 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/591-config-split/spec.md`

## Summary

Split FinFocus configuration into two tiers: a user-global directory (`~/.finfocus/`)
for shared resources (plugins, cache, logs) and a project-local directory
(`$PULUMI_PROJECT/.finfocus/`) for project-specific settings (config, dismissals).
The implementation adds project directory discovery via `Pulumi.yaml` walk-up,
shallow config merging, project-scoped dismissal storage, and a `config init`
enhancement for project-local setup with `.gitignore` generation.

## Technical Context

**Language/Version**: Go 1.25.7 (see `go.mod`)
**Primary Dependencies**: Cobra v1.10.2 (CLI), zerolog v1.34.0 (logging),
finfocus-spec v0.5.6 (protocol), gopkg.in/yaml.v3 (config parsing)
**Storage**: YAML (`config.yaml`) + JSON (`dismissed.json`) on local filesystem
**Testing**: `go test` with testify v1.11.1 (require/assert), race detector
**Target Platform**: Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64)
**Project Type**: Single Go module CLI application
**Performance Goals**: Project directory discovery < 100ms for 50-level-deep trees (SC-004)
**Constraints**: Full backward compatibility (SC-003), no new dependencies
**Scale/Scope**: Internal refactor of `internal/config` and `internal/cli` packages

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration/config logic, not a cost
  data source. No plugin changes required.
- [x] **Test-Driven Development**: Tests planned before implementation. Target 80%+
  coverage for new code, 95% for config resolution (critical path).
- [x] **Cross-Platform Compatibility**: Uses `filepath.Join()`, `os.UserHomeDir()`,
  and `filepath.Abs()` for cross-platform paths. `Pulumi.yaml` walk-up uses
  `filepath.Dir()` which handles platform differences. No platform-specific code.
- [x] **Documentation Integrity**: CLAUDE.md, CLI package CLAUDE.md, and docs/ will
  be updated with new config resolution logic, new flags, and new env vars.
- [x] **Protocol Stability**: No protocol buffer changes. Pure core-side refactor.
- [x] **Implementation Completeness**: All features fully implemented. No stubs or TODOs.
- [x] **Quality Gates**: `make lint` and `make test` required before completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. Pure core refactor.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/591-config-split/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── config-resolution.md  # Config resolution contract
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── config/
│   ├── config.go           # EXISTING: Config struct (unchanged)
│   ├── dismissed.go        # MODIFY: Fix NewDismissalStore() to use ResolveConfigDir()
│   ├── integration.go      # MODIFY: Add project-aware global config init
│   ├── project.go          # NEW: Project directory detection + config resolution
│   ├── project_test.go     # NEW: Tests for project detection
│   ├── merge.go            # NEW: Shallow config merge logic
│   ├── merge_test.go       # NEW: Tests for config merging
│   └── gitignore.go        # NEW: .gitignore generation for .finfocus/
├── cli/
│   ├── root.go             # MODIFY: Add --project-dir persistent flag
│   ├── config_init.go      # MODIFY: Project-local init with .gitignore
│   └── cost_recommendations_dismiss.go  # MODIFY: loadDismissalStore() uses project dir
└── pulumi/
    └── pulumi.go           # EXISTING: FindProject() already implemented

test/
├── unit/
│   └── config/             # Unit tests for new config logic
└── integration/
    └── cli/                # Integration tests for project-aware CLI
```

**Structure Decision**: Single Go module. New files in `internal/config/` for project
detection and merge logic. Leverages existing `internal/pulumi.FindProject()` for
Pulumi.yaml walk-up.

## Complexity Tracking

No constitution violations. No complexity justification needed.
