# Implementation Plan: Analyzer Install/Uninstall

**Branch**: `590-analyzer-install` | **Date**: 2026-02-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/590-analyzer-install/spec.md`
**GitHub Issue**: #597

## Summary

Replace the 4-step manual Pulumi Analyzer installation process with `finfocus analyzer install` and `finfocus analyzer uninstall` commands. The install command resolves the current binary path, creates the Pulumi plugin directory, and creates a symlink (Unix) or copy (Windows) of the binary with the correct Pulumi naming convention. The uninstall command removes the installed plugin directory entry.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: cobra (CLI), os/filepath/runtime (platform detection), pkg/version (version info)
**Storage**: Filesystem only (symlinks on Unix, file copies on Windows)
**Testing**: go test with testify (assert/require), filesystem-based tests with t.TempDir()
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go project (existing monorepo structure)
**Performance Goals**: N/A (one-time CLI operation)
**Constraints**: No new dependencies; must work without Pulumi CLI installed
**Scale/Scope**: 2 new CLI commands, 1 new source file (install logic), 1 new test file

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: This is core orchestration logic (CLI commands managing the analyzer plugin installation), not a data-source plugin. Compliant.
- [x] **Test-Driven Development**: Tests planned for all 5 functions (Install, Uninstall, IsInstalled, InstalledVersion, NeedsUpdate) with 80%+ coverage target.
- [x] **Cross-Platform Compatibility**: Platform-specific behavior (symlink vs copy) isolated using `runtime.GOOS` checks. Build file for platform-specific code if needed.
- [x] **Documentation Integrity**: Quickstart and CLI reference docs will be updated. Analyzer command examples will reflect new subcommands.
- [x] **Protocol Stability**: No protocol buffer changes. No cross-repo spec changes needed.
- [x] **Implementation Completeness**: Full implementation of all 5 functions, 2 CLI commands. No stubs or TODOs.
- [x] **Quality Gates**: `make lint` and `make test` will be run. 80%+ coverage enforced.
- [x] **Multi-Repo Coordination**: No cross-repo dependencies. This is core-only.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/590-analyzer-install/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (Go interfaces)
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── analyzer/
│   ├── install.go           # NEW: Install, Uninstall, IsInstalled, InstalledVersion, NeedsUpdate
│   └── install_test.go      # NEW: Unit tests for all install functions
├── cli/
│   ├── analyzer.go          # MODIFY: Register install/uninstall subcommands
│   ├── analyzer_install.go  # NEW: CLI install command
│   ├── analyzer_install_test.go  # NEW: CLI install command tests
│   ├── analyzer_uninstall.go     # NEW: CLI uninstall command
│   └── analyzer_uninstall_test.go # NEW: CLI uninstall command tests

test/
└── integration/
    └── analyzer_install_test.go  # NEW: Integration test (install → verify → uninstall)
```

**Structure Decision**: Follows existing project structure. Core logic in `internal/analyzer/` (alongside `server.go`, `mapper.go`, `diagnostics.go`). CLI commands in `internal/cli/` following the `analyzer_*.go` naming pattern established by `analyzer_serve.go`.

## Key Design Decisions

### Binary Naming Convention

**Critical Finding**: There is a discrepancy between `main.go` and the existing quickstart:

- `main.go` detects: `pulumi-analyzer-finfocus` and `pulumi-analyzer-policy-finfocus`
- `quickstart.md` documents: `pulumi-analyzer-cost`

**Decision**: The install command will use `pulumi-analyzer-finfocus` to match `main.go` (the actual runtime detection). The directory will be `analyzer-finfocus-vX.Y.Z/`. The existing quickstart documentation should be updated separately to align.

### Pulumi Plugin Directory Resolution

**Precedence order** (matching existing `ResolveConfigDir()` pattern):

1. `--target-dir` flag (explicit override)
2. `$PULUMI_HOME/plugins/` (Pulumi ecosystem integration)
3. `~/.pulumi/plugins/` (default)

### Symlink vs Copy Strategy

- **Unix (Linux, macOS)**: Symlink the binary. If symlink fails (e.g., cross-device), fall back to copy.
- **Windows**: Always copy the binary (symlinks require elevated privileges).
- **Detection**: Use `runtime.GOOS == "windows"` at compile time or runtime.

### Version Comparison

- Use `pkg/version.GetVersion()` for the current binary version.
- Extract installed version from the directory name pattern `analyzer-finfocus-v<version>`.
- Simple string comparison (no semver parsing needed for equality check).

### Uninstall Scope

- Uninstall scans for ALL `analyzer-finfocus-v*` directories in the plugin directory.
- Removes all found entries (not just the version matching the current binary).
- This prevents orphaned old-version directories from accumulating.
