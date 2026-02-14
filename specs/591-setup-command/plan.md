# Implementation Plan: finfocus setup — One-Command Bootstrap

**Branch**: `591-setup-command` | **Date**: 2026-02-14 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/591-setup-command/spec.md`

## Summary

Add a `finfocus setup` top-level CLI command that bootstraps the entire FinFocus
environment in a single idempotent invocation. The command sequentially executes
independent setup steps (version display, Pulumi detection, directory creation,
config initialization, analyzer installation, plugin installation) and reports
per-step status. Each step is fault-tolerant: critical failures (directories,
config) cause a non-zero exit, while optional failures (Pulumi missing, plugin
download) produce warnings without blocking subsequent steps.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: cobra v1.10.2 (CLI), golang.org/x/term (TTY detection), zerolog v1.34.0 (logging)
**Storage**: Filesystem only — directories, YAML config file, symlinks/copies for analyzer
**Testing**: go test with testify (assert/require), t.TempDir() for isolation
**Target Platform**: Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64)
**Project Type**: Single Go project — CLI command addition
**Performance Goals**: All local steps complete within 10 seconds (network-dependent plugin download excluded)
**Constraints**: No new dependencies beyond what's already in go.mod; idempotent; no root privileges required
**Scale/Scope**: Single new CLI command with ~300-400 lines of implementation + ~500-600 lines of tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: This is orchestration logic (CLI command), not a cost data source. No plugin boundary violated.
- [x] **Test-Driven Development**: Tests planned before implementation with 80% minimum coverage target (SC-005).
- [x] **Cross-Platform Compatibility**: Uses `os.MkdirAll`, `filepath.Join`, `exec.LookPath` — all cross-platform. Analyzer install already handles symlink vs copy for Windows.
- [x] **Documentation Integrity**: CLI help text and docs/ updates planned. No new exported API requiring godoc.
- [x] **Protocol Stability**: No protocol buffer changes. Uses existing `analyzer.Install()` and `registry.Installer.Install()`.
- [x] **Implementation Completeness**: Full implementation planned — no stubs or TODOs.
- [x] **Quality Gates**: `make lint` and `make test` required before completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes. All dependencies already merged (#597).

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/591-setup-command/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/cli/
├── setup.go             # NewSetupCmd() — Cobra command, step orchestration
└── setup_test.go        # Unit tests for all setup steps

internal/cli/root.go     # Modified: add NewSetupCmd() to cmd.AddCommand()
```

**Structure Decision**: This feature adds a single top-level CLI command.
Following existing patterns (e.g., `analyzer_install.go`, `config_init.go`),
the command lives in `internal/cli/setup.go` with the Cobra command constructor
and step execution logic in the same file. No new packages are needed — the
setup command orchestrates existing packages (`config`, `analyzer`, `registry`,
`pulumi`, `version`).

## Design Decisions

### Step Execution Model

Each setup step is an independent function with a uniform signature. Steps
execute sequentially in a fixed order. Each step returns a result (status +
message) that is collected into a `SetupResult` aggregate. A step failure does
not short-circuit subsequent steps.

**Step Order**:

1. Version display — `pkg/version.GetVersion()` + `runtime.Version()`
2. Pulumi detection — `internal/pulumi.FindBinary()` + version extraction
3. Directory creation — `os.MkdirAll` for base, plugins, cache, logs
4. Config initialization — `config.New()` + conditional `config.Save()`
5. Analyzer installation — `analyzer.Install(ctx, opts)` (skip if `--skip-analyzer`)
6. Plugin installation — `registry.Installer.Install()` (skip if `--skip-plugins`)

### Status Reporting

Each step prints a status line using `cmd.Printf()`:

- Success: `checkmark + " " + message` (e.g., `"✓ Created ~/.finfocus/"`)
- Warning: `"! " + message` (e.g., `"! Pulumi CLI not found on PATH"`)
- Skip: `"- " + message` (e.g., `"- Skipped analyzer installation"`)
- Error: `"✗ " + message` (e.g., `"✗ Failed to create ~/.finfocus/: permission denied"`)

In non-interactive mode (no TTY or `--non-interactive`), plain ASCII markers
are used instead of Unicode symbols (`[OK]`, `[WARN]`, `[SKIP]`, `[ERR]`).

### Exit Code Logic

- Exit 0: All critical steps (directories, config) succeed
- Exit 1: Any critical step fails
- Warnings from optional steps (Pulumi detection, analyzer, plugins) do not
  affect exit code

### Idempotency Strategy

- Directories: `os.MkdirAll` is inherently idempotent
- Config: Check `os.Stat()` before writing; skip if file exists
- Analyzer: Delegates to `analyzer.Install()` which returns `ActionAlreadyCurrent`
- Plugins: Delegates to `registry.Installer.Install()` which checks version existence

### TTY Detection

Reuse existing `isTerminal()` from `root.go` (uses `golang.org/x/term`).
When `--non-interactive` is set OR stdin is not a TTY, use plain ASCII output
markers instead of Unicode checkmarks/symbols.

### Default Plugin Set

Hardcoded slice in the setup command:

```go
var defaultPlugins = []string{"aws-public"}
```

This is intentionally simple. The list can be expanded in future releases.
Plugin version is resolved to latest by the installer's existing fallback logic.
