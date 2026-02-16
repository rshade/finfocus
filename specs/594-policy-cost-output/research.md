# Research: Policy-Compatible Cost Output

**Feature Branch**: `594-policy-cost-output`
**Date**: 2026-02-16

## R1: Pulumi Enforcement Level System

**Decision**: Use `EnforcementLevel_MANDATORY` (value 1) from Pulumi SDK to block deployments when threshold is exceeded in mandatory mode.

**Rationale**: The Pulumi SDK v3.220.0 (`analyzer.pb.go`) provides four enforcement levels:

- `ADVISORY` (0) — displayed to users, does not block deployment
- `MANDATORY` (1) — stops deployment, cannot be overridden
- `DISABLED` (2) — policy does not run
- `REMEDIATE` (3) — fixes problems instead of reporting

MANDATORY is the correct choice for cost threshold blocking because it cannot be overridden by the user, ensuring cost governance is enforceable.

**Alternatives Considered**:

- Using REMEDIATE to auto-adjust resources — rejected: out of scope, FinFocus doesn't modify infrastructure
- Using ADVISORY with severity escalation — rejected: does not actually block deployment regardless of severity

## R2: Diagnostic Metadata Embedding

**Decision**: Embed machine-parseable JSON metadata as an HTML comment appended to the diagnostic `Message` field.

**Rationale**: The `AnalyzeDiagnostic` protobuf struct (v3.220.0) has no Tags or Metadata field. Available fields are: PolicyName, PolicyPackName, PolicyPackVersion, Description, Message, EnforcementLevel, Urn, Severity. The only extensible text fields are Description and Message. HTML comment syntax (`<!-- finfocus:cost:{...} -->`) is invisible in most rendering contexts while being extractable by tooling.

**Alternatives Considered**:

- Adding metadata to Description field — rejected: Description is used for policy rule description, not per-resource data
- Using a custom protobuf extension — rejected: requires Pulumi platform changes
- Relying on the summary file only — rejected: doesn't help tools that process diagnostic output directly

## R3: Summary File Location and Project Awareness

**Decision**: Write to project-local `$PROJECT/.finfocus/last-cost-summary.json` with fallback to `~/.finfocus/last-cost-summary.json`. Make the analyzer serve command project-aware.

**Rationale**: The analyzer serve command currently uses `config.New()` (global only, at `analyzer_serve.go:108`). To support project-local summary files, the analyzer needs access to the project directory. Two options:

1. Use CWD-based detection via `config.ResolveProjectDir()` — the analyzer subprocess inherits the CWD from the Pulumi CLI invocation, which is typically the project directory
2. Accept a `--project-dir` flag — more explicit but requires passing through Pulumi

Option 1 is preferred because the analyzer inherits the working directory from `pulumi preview`, which runs in the project directory.

**Alternatives Considered**:

- Global-only summary file — rejected: multiple projects would overwrite each other
- Stack-specific filenames (e.g., `last-cost-summary-dev.json`) — rejected: adds complexity, single file per project is sufficient for MVP

## R4: Configuration Extension Pattern

**Decision**: Add `MaxMonthlyCost` and `Enforcement` fields to existing `AnalyzerConfig` struct, with environment variable overrides following the established `applyEnvOverrides()` pattern.

**Rationale**: The `AnalyzerConfig` struct (`config.go:127-145`) already supports `Timeout` and `Plugins`. Adding two new fields is consistent with the existing pattern. The `applyEnvOverrides()` method (`config.go:696-772`) already handles typed parsing with `strconv.ParseFloat()` and string matching, making env var support straightforward.

**Key Pattern** (from existing code):

```text
Config file:       analyzer.max_monthly_cost: 5000.00
Config file:       analyzer.enforcement: mandatory
Env var override:  FINFOCUS_MAX_MONTHLY_COST=5000
Env var override:  FINFOCUS_ENFORCEMENT=mandatory
Precedence:        env var > project config > global config > default
```

**Alternatives Considered**:

- Separate `PolicyConfig` struct — rejected: adds unnecessary indirection when only two fields are needed
- Using `Cost.Budgets` instead — rejected: budgets are for CLI output, not analyzer enforcement

## R5: Atomic File Writing Pattern

**Decision**: Use the atomic write pattern from `DismissalStore` (`dismissed.go:241-281`): temp file + `os.Rename()` with `0o600` permissions.

**Rationale**: The existing pattern provides:

1. Cross-process safety via file locking
2. Atomic write via temp file + rename
3. Restricted permissions (`0o600` — owner read/write only)
4. Directory creation with `os.MkdirAll(dir, 0o750)`
5. Cleanup of temp file on error

This is battle-tested in the codebase and handles the edge cases (permissions, disk full, concurrent access).

**Alternatives Considered**:

- Direct `os.WriteFile()` — rejected: not atomic, partial writes on failure
- Separate writer abstraction — rejected: over-engineering for a single file write

## R6: Server Constructor Extension

**Decision**: Add a `WithConfig(*config.Config)` builder method to `Server` rather than changing the `NewServer()` signature.

**Rationale**: The builder pattern is already used throughout the codebase (e.g., `engine.New().WithRouter()`, `engine.New().WithJobs()`). Adding a builder method preserves backward compatibility — existing callers don't need to change. The Server stores config internally and uses it during AnalyzeStack for threshold checking and summary file writing.

**Alternatives Considered**:

- Changing `NewServer()` signature to include config — rejected: breaks all existing callers and tests
- Passing only threshold/enforcement values — rejected: limits future extensibility (may need project dir, output preferences)

## R7: Threshold Diagnostic Placement

**Decision**: Emit the threshold diagnostic in `AnalyzeStack()` as an additional diagnostic alongside the existing stack summary.

**Rationale**: `AnalyzeStack()` (`server.go:246-262`) is called once after all per-resource `Analyze()` calls complete. It already aggregates cached costs and produces a summary. Adding threshold checking here means:

1. Total cost is already computed
2. All per-resource costs have been cached
3. The diagnostic appears at stack level (no URN), consistent with the budget-level enforcement concept
4. It's returned as an additional item in the `Diagnostics` slice alongside the existing stack summary

**Alternatives Considered**:

- Checking threshold in each `Analyze()` call — rejected: can't know total cost until all resources are analyzed
- Separate gRPC endpoint — rejected: not part of the Pulumi Analyzer protocol
