# Research: Automatic Pulumi Integration for Cost Commands

**Feature**: 509-pulumi-auto-detect
**Date**: 2026-02-11

## Decision 1: CLI Shelling vs Go Automation API

**Decision**: Shell out to `pulumi` CLI binary via `exec.CommandContext()`

**Rationale**:

- The Go Automation API (`github.com/pulumi/pulumi/sdk/v3/go/auto`) has a known config caching bug (pulumi/pulumi#12152) where repeated Preview calls ignore config file changes
- The Go Automation API does NOT support `--json` output from Preview — returns Go structs that don't match the `pulumi preview --json` schema FinFocus already parses
- The Pulumi CLI must be installed regardless (Automation API wraps it via gRPC internally)
- CLI invocation gives fresh state every time with no caching issues
- Matches existing subprocess patterns: `internal/pluginhost/process.go` (ProcessLauncher) and `test/e2e/context.go`
- Simpler error handling via exit codes and stderr capture

**Alternatives Considered**:

- **Go Automation API**: Rich Go types, no process overhead. Rejected due to caching bug, missing JSON output, and requirement for CLI binary anyway.
- **Direct gRPC to Pulumi engine**: Too low-level, undocumented internal protocol, fragile across versions.

## Decision 2: Ingestion Layer Extension

**Decision**: Add `[]byte`-based parsing functions; refactor file-based functions to delegate

**Rationale**:

- Current `LoadPulumiPlan(path)` and `LoadStackExport(path)` do `os.ReadFile()` → `json.Unmarshal()` internally
- Extracting the parsing step into `ParsePulumiPlan(data []byte)` allows both file-based and CLI-based callers
- File-based functions become thin wrappers: read file → call Parse variant
- Zero API changes for existing callers — purely additive refactor

**Alternatives Considered**:

- **Write CLI output to temp file, then load**: Extra I/O, temp file cleanup, no benefit.
- **`io.Reader` interface**: Over-engineering for this use case; `[]byte` is simpler and matches `exec.Command.Output()` return type.

## Decision 3: Stack Detection Strategy

**Decision**: Run `pulumi stack ls --json`, parse for `current: true` entry

**Rationale**:

- `pulumi stack ls --json` returns structured JSON with a `current` boolean field per stack
- This is the canonical way to determine the active stack programmatically
- Falls back to `--stack` flag for explicit override
- Error case (no current stack) provides the stack list for user guidance

**Alternatives Considered**:

- **Parse `.pulumi/` directory**: Fragile, backend-specific (local vs S3 vs Pulumi Cloud), undocumented format.
- **Environment variable `PULUMI_STACK`**: Not standard; Pulumi uses workspace state, not env vars.

## Decision 4: Project Detection Strategy

**Decision**: Walk up directory tree looking for `Pulumi.yaml` or `Pulumi.yml`

**Rationale**:

- Standard pattern used by git (`.git`), npm (`package.json`), Cargo (`Cargo.toml`)
- Pulumi projects always have `Pulumi.yaml` or `Pulumi.yml` at root
- Walk from current directory upward to filesystem root
- Uses `filepath.Join()` and `os.Stat()` for cross-platform compatibility

**Alternatives Considered**:

- **Only check current directory**: Too restrictive — users often run from subdirectories.
- **`--pulumi-dir` flag**: Deferred to future enhancement. `cd` to project is sufficient.

## Decision 5: Actual Cost Auto-Detection Preference

**Decision**: For `cost actual`, auto-detection uses `pulumi stack export` (state-based) not `pulumi preview --json` (plan-based)

**Rationale**:

- Stack state provides richer data for actual cost calculation:
  - Real cloud resource IDs (not just types)
  - `Created` and `Modified` timestamps for automatic date range detection
  - `pulumi:cloudId` and `pulumi:arn` for precise cost lookups
- Matches the existing `--pulumi-state` path which has auto-detection of `--from` date
- Preview JSON lacks these timestamps and real IDs

**Alternatives Considered**:

- **Use preview for both**: Would lose timestamp auto-detection and real resource IDs. Rejected.

## Decision 6: Timeout Strategy

**Decision**: 5-minute timeout for preview, 60-second timeout for export. Not user-configurable initially.

**Rationale**:

- `pulumi preview` can be slow for large stacks (compiling programs, resolving dependencies, API calls)
- `pulumi stack export` is a local operation reading stored state — fast
- Context cancellation from `cmd.Context()` allows user to Ctrl+C at any time
- User-configurable timeouts deferred to avoid flag proliferation

**Alternatives Considered**:

- **No timeout**: Risk of indefinite hang if Pulumi prompts for input (passphrase).
- **Configurable via flags**: Over-engineering for initial release. Can add `--preview-timeout` later.
- **Configurable via env var**: Reasonable future enhancement (`FINFOCUS_PREVIEW_TIMEOUT`).

## Decision 7: Error Message Design

**Decision**: Layered error hierarchy with actionable suggestions at each level

**Rationale**:

- Error hierarchy: binary not found > no project > no stack > command failure > parse failure
- Each error includes both what went wrong and how to fix it
- Binary not found includes install URL: `https://www.pulumi.com/docs/install/`
- No project found suggests: `use --pulumi-json to provide input directly`
- No stack found lists available stacks and suggests: `use --stack <name>`
- Command failure includes Pulumi stderr for diagnosis

**Alternatives Considered**:

- **Generic "auto-detection failed"**: Not actionable. Rejected.
- **Separate `--verbose-errors` flag**: Unnecessary — always show helpful errors.

## Existing Codebase Patterns Referenced

### Subprocess Execution (from `internal/pluginhost/process.go`)

- Use `exec.CommandContext()` for timeout support
- Set `cmd.WaitDelay` before `cmd.Start()` for graceful I/O shutdown
- Always call `cmd.Wait()` after `cmd.Process.Kill()` to prevent zombies
- Inherit `os.Environ()` for environment passthrough
- Capture stderr separately for error reporting

### CLI Flag Patterns (from `internal/cli/cost_actual.go`)

- `cost actual` already has runtime validation via `validateActualInputFlags()`
- Flags are NOT marked required via `MarkFlagRequired()` — validation is custom
- `cost projected` uses `MarkFlagRequired("pulumi-json")` — needs removal

### Ingestion Layer (from `internal/ingest/`)

- `LoadPulumiPlan(path)` → `os.ReadFile()` → `json.Unmarshal()` → `*PulumiPlan`
- `LoadStackExport(path)` → `os.ReadFile()` → `json.Unmarshal()` → `*StackExport`
- Both return pointer + error pattern
- Context variants add zerolog logging

### Test Patterns (from constitution and CLAUDE.md)

- Use `testify/require` for setup, `testify/assert` for assertions
- Table-driven tests for variations
- TestHelperProcess pattern for mocking `exec.Command`
- `t.Setenv("FINFOCUS_LOG_LEVEL", "error")` to suppress log noise
