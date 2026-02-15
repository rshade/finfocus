# Research: finfocus setup — One-Command Bootstrap

**Date**: 2026-02-14
**Branch**: `591-setup-command`

## R1: CLI Command Registration Pattern

**Decision**: Add `NewSetupCmd()` as a top-level command via `cmd.AddCommand()` in `root.go:94`.

**Rationale**: The setup command is a first-class user action (not a subcommand of
`cost`, `plugin`, or `config`). It follows the same pattern as `NewAnalyzerCmd()`
and `NewOverviewCmd()` — top-level commands exported from `internal/cli/`.

**Alternatives considered**:

- Subcommand of `config` (e.g., `finfocus config setup`): Rejected because setup
  does more than config — it also installs analyzer and plugins.
- Separate binary: Rejected — unnecessary complexity for a bootstrapping operation.

## R2: Step Orchestration Pattern

**Decision**: Sequential execution of independent step functions, each returning a
`StepResult` struct. Results collected into a `SetupResult` aggregate.

**Rationale**: The existing multi-step pattern in `cost_recommendations.go:173-250`
uses sequential steps with early returns. However, setup needs to continue past
failures (FR-017), so a collect-and-continue pattern is more appropriate. Each step
is a function `func(ctx, cmd, opts) StepResult` that encapsulates its own error
handling.

**Alternatives considered**:

- Pipeline/middleware chain: Rejected — over-engineered for 6 sequential steps.
- Goroutine parallel execution: Rejected — steps have ordering dependencies
  (directories must exist before config write) and parallel execution complicates
  output ordering.

## R3: Analyzer Installation Integration

**Decision**: Call `analyzer.Install(ctx, InstallOptions{})` directly. Map the
result's `Action` field to appropriate status output.

**Rationale**: The `Install()` function at `internal/analyzer/install.go:150` already
handles idempotency (returns `ActionAlreadyCurrent`), version comparison, and
platform-specific behavior (symlink vs copy). No wrapper needed.

**Key mappings**:

- `ActionInstalled` → success status with version
- `ActionAlreadyCurrent` → success status noting already installed
- `ActionUpdateAvailable` → warning status suggesting update
- Error → warning status with remediation hint

## R4: Plugin Installation Integration

**Decision**: Create a `registry.NewInstaller()` and call `Install(specifier, opts, progress)`
for each plugin in the default set.

**Rationale**: The installer at `registry/installer.go:207` handles GitHub release
discovery, platform asset matching, download, extraction, and binary validation.
The setup command just needs to iterate over a default plugin list and call the
existing installer.

**Key details**:

- Specifier format: `"aws-public"` (name only, latest version resolved automatically)
- `InstallOptions.FallbackToLatest = true` for setup (auto-resolve version)
- Progress callback writes to `cmd.Printf()` for visibility
- Failure is non-fatal: warn and continue to next plugin

## R5: TTY Detection and Output Modes

**Decision**: Reuse `isTerminal()` from `root.go:16-19` (uses `golang.org/x/term`).
Two output modes: TTY (Unicode symbols) and non-TTY (ASCII markers).

**Rationale**: The project already depends on `golang.org/x/term` and has the
`isTerminal()` helper. No new dependency needed.

**Output mapping**:

| Status  | TTY Output | Non-TTY Output |
|---------|------------|----------------|
| Success | `✓`        | `[OK]`         |
| Warning | `!`        | `[WARN]`       |
| Skip    | `-`        | `[SKIP]`       |
| Error   | `✗`        | `[ERR]`        |

## R6: Pulumi Version Detection

**Decision**: Use `internal/pulumi.FindBinary()` for PATH detection. For version
extraction, run `pulumi version` via `exec.CommandContext()`.

**Rationale**: `FindBinary()` at `pulumi/pulumi.go:69-79` already uses
`exec.LookPath("pulumi")` which searches PATH correctly on all platforms.
Version extraction requires running the binary since there's no existing
helper for just `pulumi version`.

**Alternatives considered**:

- Parse `pulumi` binary metadata: Rejected — not portable.
- Skip version display: Rejected — version info helps debugging.

## R7: Config Initialization Strategy

**Decision**: Use `config.New()` to create defaults, then check if config file
exists via `os.Stat()`. Only call `config.Save()` if the file does not exist.

**Rationale**: `config.New()` at `config/config.go:181-261` creates a fully
populated default config. `config.Save()` at line 348 creates the directory
with `0700` permissions and writes the file with `0600` permissions. Checking
existence first ensures idempotency (FR-010) and config preservation (FR-005).

**Alternatives considered**:

- Always call `config.Save()`: Rejected — would overwrite user customizations.
- Call `NewConfigInitCmd().Execute()`: Rejected — embedding command execution
  is fragile; better to use the underlying functions directly.

## R8: Directory Creation

**Decision**: Use `os.MkdirAll()` for each required directory. Directories are
derived from `config.ResolveConfigDir()`.

**Rationale**: `os.MkdirAll()` is inherently idempotent — it succeeds if the
directory already exists. Using `ResolveConfigDir()` ensures `FINFOCUS_HOME`
and `PULUMI_HOME` overrides are respected (FR-011).

**Required directories**:

- Base: `ResolveConfigDir()` (e.g., `~/.finfocus/`)
- Plugins: `base/plugins/`
- Cache: `base/cache/`
- Logs: `base/logs/`

**Permissions**: `0700` for base dir (matches `config.Save()`), `0750` for
plugin dir (matches `plugin_init.go:34`).
