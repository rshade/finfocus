# Implementation Plan: Analyzer Quality Cluster

**Branch**: `603-analyzer-quality` | **Date**: 2026-02-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/603-analyzer-quality/spec.md`
**Issues**: #746, #754, #755, #756, #757

## Summary

Fix five analyzer quality issues: (1) stack cost summary always showing $0.00
due to inconsistent counting logic between `StackSummaryDiagnostic` and
`BuildCostSummary`, plus missing cache entries for failed cost calculations;
(2) automatic policy pack directory setup during `analyzer install`;
(3) `--force` reinstall syncing the policy pack binary;
(4) post-install PATH instructions; and (5) a new `analyzer check` diagnostic
command. The approach uses the existing analyzer/install infrastructure and
adds minimal new types.

## Technical Context

**Language/Version**: Go 1.25.8 (see `go.mod`)
**Primary Dependencies**: Cobra (CLI), gRPC (Pulumi protocol), zerolog (logging),
Pulumi SDK v3 (Analyzer RPC types), testify (testing)
**Storage**: Filesystem — `~/.finfocus/analyzer/` (policy pack), `~/.pulumi/plugins/`
(Pulumi plugin)
**Testing**: `go test` with testify (assert/require), table-driven tests
**Target Platform**: Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64)
**Project Type**: Single CLI tool
**Performance Goals**: N/A — all operations are file I/O or single gRPC calls
**Constraints**: Must not break existing `analyzer serve` protocol; all diagnostics
remain ADVISORY
**Scale/Scope**: 5 issues, ~8 files modified, ~3 new files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: This is orchestration/CLI logic, not a
  direct provider integration. The analyzer delegates to plugins via the engine.
- [x] **Test-Driven Development**: Tests planned for all changes. Target 80%+
  coverage on new code (existing analyzer coverage is 92.7%).
- [x] **Cross-Platform Compatibility**: Symlink/copy fallback already exists
  in `linkOrCopy()`. `exec.LookPath` is cross-platform. All new code uses
  `filepath` and `os` packages for portability.
- [x] **Documentation Integrity**: CLAUDE.md analyzer section will be updated.
  New `check` command documented in CLI examples.
- [x] **Protocol Stability**: No protocol buffer changes. All changes are
  internal to the analyzer server and CLI layer.
- [x] **Implementation Completeness**: All five issues will be fully
  implemented. No stubs or TODOs.
- [x] **Quality Gates**: `make lint` and `make test` must pass before
  completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. All changes
  are in `finfocus` core.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/603-analyzer-quality/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 research findings
├── data-model.md        # Phase 1 data model
├── quickstart.md        # Phase 1 quickstart guide
├── contracts/
│   ├── check-result-schema.json   # JSON schema for analyzer check output
│   └── install-result-schema.json # Extended install result schema
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/analyzer/
├── server.go            # MODIFIED: Cache error costs in Analyze()
├── diagnostics.go       # MODIFIED: Fix StackSummaryDiagnostic counting
├── install.go           # MODIFIED: Add PolicyPackDir/Method to InstallResult,
│                        #           add --force policy pack sync
├── policypack.go        # NEW: Policy pack directory setup logic
├── check.go             # NEW: Check command verification logic
├── policypack_test.go   # NEW: Policy pack tests
├── check_test.go        # NEW: Check command tests
├── diagnostics_test.go  # MODIFIED: Add tests for fixed counting
├── server_test.go       # MODIFIED: Add tests for error cost caching
└── install_test.go      # MODIFIED: Add tests for policy pack sync

internal/cli/
├── analyzer.go          # MODIFIED: Wire check subcommand
├── analyzer_install.go  # MODIFIED: Policy pack output + PATH instructions
├── analyzer_check.go    # NEW: CLI wiring for analyzer check
└── analyzer_check_test.go # NEW: CLI integration tests
```

**Structure Decision**: Single project layout following existing Go package
conventions. All analyzer logic lives in `internal/analyzer/`, CLI wiring in
`internal/cli/`. No new packages needed.

## Implementation Details

### Issue #746: Fix $0.00 Stack Summary (P1)

**Root cause**: `StackSummaryDiagnostic()` and `BuildCostSummary()` have
different counting/filtering logic. See `research.md` for full analysis.

**Changes**:

1. **`diagnostics.go`** — Refactor `StackSummaryDiagnostic()` to use
   `BuildCostSummary()` for consistent aggregation, then format the message
   from the summary struct. This eliminates the duplicated counting logic.

2. **`server.go`** — In `Analyze()`, when `GetProjectedCost` returns an
   error (`calcErr != nil`), cache a zero-cost error result so
   `AnalyzeStack` can see that the resource was attempted:

   ```go
   if calcErr != nil {
       errResult := engine.CostResult{
           ResourceType: resourceType,
           ResourceID:   resourceID,
           Currency:     "USD",
           Notes:        "ERROR: " + calcErr.Error(),
       }
       s.cacheCost(resourceID, errResult)
       // ... return WarningDiagnostic as before
   }
   ```

**Testing**: Unit tests verifying:

- Stack summary matches sum of individual costs
- Error resources excluded from summary total but counted
- Empty cache produces "$0.00 (0 resources analyzed)"
- Mixed success/error scenario

### Issue #755: Policy Pack Directory Setup (P2)

**New file**: `internal/analyzer/policypack.go`

**Functions**:

- `SetupPolicyPack(ctx, execPath) (string, string, error)` — Creates
  `~/.finfocus/analyzer/` with `PulumiPolicy.yaml` and binary reference.
  Returns `(policyPackDir, method, error)`.
- `ResolvePolicyPackDir() (string, error)` — Returns
  `~/.finfocus/analyzer/` (or `$FINFOCUS_HOME/analyzer/`).
- `WritePulumiPolicyYAML(dir string) error` — Writes `PulumiPolicy.yaml`.

**Integration**: Called from `Install()` after the Pulumi plugin install
succeeds. Results stored in `InstallResult.PolicyPackDir` and
`InstallResult.PolicyPackMethod`.

**CLI output** (`analyzer_install.go`): After install, print policy pack
directory path.

**Testing**: Unit tests for directory creation, YAML content validation,
symlink/copy, idempotent re-setup.

### Issue #754: `--force` Syncs Policy Pack (P3)

**Changes to `install.go`**:

After the existing `--force` reinstall logic, check if the policy pack
directory exists. If it does, re-sync the binary reference. If the directory
doesn't exist, skip silently. If sync fails, log a warning.

```go
// In Install(), after force reinstall succeeds:
if opts.Force {
    ppDir, _ := ResolvePolicyPackDir()
    if _, statErr := os.Stat(ppDir); statErr == nil {
        if syncErr := syncPolicyPackBinary(ctx, execPath, ppDir); syncErr != nil {
            log.Warn().Err(syncErr).Msg("failed to sync policy pack binary")
        }
    }
}
```

**Testing**: Tests for force sync updating both locations, missing policy
pack dir no-op, sync failure warning.

### Issue #756: Post-Install PATH Instructions (P4)

**Changes to `analyzer_install.go`**:

After the install success message, print PATH instructions when:

- `result.Action == ActionInstalled` (fresh install or force)
- Not JSON output mode

```text
To use the analyzer with pulumi preview:

  export PATH="$HOME/.finfocus/analyzer:$PATH"

Then run:

  pulumi preview --policy-pack ~/.finfocus/analyzer
```

**Testing**: Capture command output and verify instructions are present for
fresh install, absent for no-op and JSON mode.

### Issue #757: `analyzer check` Command (P5)

**New file**: `internal/analyzer/check.go`

**Functions**:

- `RunChecks(ctx) (*CheckReport, error)` — Executes all checks in order.
- `checkPolicyPackDir() CheckResult`
- `checkPulumiPolicyYAML(dir string) CheckResult`
- `checkBinaryInPATH() CheckResult`
- `checkGRPCSmokeTest(ctx) CheckResult`

**New file**: `internal/cli/analyzer_check.go`

- `NewAnalyzerCheckCmd() *cobra.Command` — Cobra command with `--output`
  flag (table/json).
- Human-readable output: pass/fail indicators per check.
- JSON output: `CheckReport` struct marshaled.
- Exit code 0 if all pass, 1 if any fail.

**CLI wiring**: Add `cmd.AddCommand(NewAnalyzerCheckCmd())` in
`analyzer.go`.

**Testing**: Tests for all-pass, individual failures, skip cascading,
JSON output format, exit code behavior.

## Complexity Tracking

No constitution violations. No complexity tracking needed.
