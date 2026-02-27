# Research: Analyzer Quality Cluster

**Branch**: `603-analyzer-quality` | **Date**: 2026-02-24

## Research Task 1: Root Cause of $0.00 Stack Summary (#746)

### RT1 Decision

The `StackSummaryDiagnostic()` function in `diagnostics.go:80-117` has a
**different counting algorithm** than `BuildCostSummary()` in `summary.go:65-111`.
Both receive the same cached costs but produce inconsistent results. The root
cause is a logic discrepancy between these two functions, combined with a
potential cache population issue when cost calculation fails.

### RT1 Analysis

**`StackSummaryDiagnostic` counting** (`diagnostics.go:88-96`):

- Sums `c.Monthly` for ALL costs (including errors)
- Counts only resources where `c.Monthly > 0` as "analyzed"
- Does NOT skip error resources

**`BuildCostSummary` counting** (`summary.go:80-104`):

- Skips resources with `c.Error != nil || isErrorNote(c.Notes)`
- Counts ALL non-error resources (even zero-cost ones) in `ResourceCount`
- Only sums `c.Monthly` from non-error resources

**Cache population gaps** (`server.go:172-265`):

- When `calcErr != nil` (line 203), `Analyze()` returns a `WarningDiagnostic`
  but does **NOT** cache the cost. The diagnostic IS shown to the user
  (appearing as individual resource cost), but the cost is lost for the
  stack summary.
- Internal Pulumi types (`pulumi:pulumi:Stack`, `pulumi:providers:aws`) are
  cached with $0.00 — correct behavior.

**Potential protocol ordering issue**:

- The Pulumi protocol calls: `ConfigureStack` → `Analyze` (per resource) →
  `AnalyzeStack`
- In current code, `ConfigureStack()` does NOT clear the cache (good).
- `clearCostCache()` is only called AFTER `AnalyzeStack()` reads the cache
  (line 342) — correct ordering.
- However, if `GetProjectedCost` returns zero costs (empty slice) for a
  resource, neither the success nor the error path caches a result. The
  `len(diagnostics) == 0` fallback (line 248) catches this, but uses a
  generic zero-cost result.

### RT1 Root Cause Summary

1. **Primary**: `StackSummaryDiagnostic` does not filter error costs but
   only counts `Monthly > 0`, creating a mismatch with what users see as
   per-resource costs.
2. **Secondary**: When `GetProjectedCost` returns an error, the cost is
   shown to the user as a WarningDiagnostic but NOT cached, so it's
   invisible to AnalyzeStack.

### RT1 Fix Strategy

1. Unify the counting logic: `StackSummaryDiagnostic` should use
   `BuildCostSummary` (or share its filtering logic) to ensure consistency.
2. When `GetProjectedCost` fails, cache a zero-cost error result so it
   appears in the summary count as a failed resource.
3. Change "analyzed" count to include all non-error resources (including
   $0.00 ones), matching `BuildCostSummary.ResourceCount`.

### RT1 Alternatives Considered

- **Re-querying plugins in AnalyzeStack**: Rejected because (a) plugins may
  return different results due to property format differences between
  `AnalyzeRequest` and `AnalyzerResource`, and (b) re-querying is
  unnecessary overhead.
- **Using AnalyzeStackRequest resources instead of cache**: Rejected because
  AnalyzerResource has different field shapes than AnalyzeRequest, making
  property mapping inconsistent.

## Research Task 2: Pulumi Policy Pack Directory Requirements

### RT2 Decision

Use the standard Pulumi policy pack directory structure with `PulumiPolicy.yaml`
declaring `runtime: finfocus` and a binary named `pulumi-analyzer-policy-finfocus`.

### RT2 Analysis

**Pulumi policy pack directory structure**:

```text
~/.finfocus/analyzer/
├── PulumiPolicy.yaml
└── pulumi-analyzer-policy-finfocus  (symlink or copy)
```

**PulumiPolicy.yaml schema** (from Pulumi docs):

```yaml
name: finfocus
runtime: finfocus
description: FinFocus cost estimation analyzer
```

The `runtime` field tells Pulumi which binary to execute. The binary must be
named `pulumi-analyzer-policy-{runtime}` to be discovered.

**Usage workflow**:

```bash
pulumi preview --policy-pack ~/.finfocus/analyzer
```

This requires the binary to be both:

1. Present in the policy pack directory as `pulumi-analyzer-policy-finfocus`
2. Executable and pointing to the correct finfocus version

### RT2 Alternatives Considered

- **Using `--policy-group` instead**: Rejected — requires Pulumi Cloud.
- **Using the Pulumi plugin directory directly**: Rejected — the `--policy-pack`
  flag requires the PulumiPolicy.yaml + binary naming convention, which is
  different from the Pulumi plugin directory convention.

## Research Task 3: Cross-Platform Symlink Behavior

### RT3 Decision

Use symlinks on Unix (macOS, Linux) with a copy fallback. Always use file
copy on Windows. The existing `linkOrCopy()` function in `install.go` already
implements this pattern correctly.

### RT3 Analysis

**Unix symlinks**: Work reliably. May fail on cross-device scenarios (e.g.,
network mounts), which the existing fallback handles.

**Windows symlinks**: Require either:

- Developer Mode enabled, or
- `SeCreateSymbolicLinkPrivilege` privilege (admin-only by default)

Since most Windows users don't have developer mode enabled, file copy is the
safe default. The existing code already uses `runtime.GOOS == "windows"` to
branch.

### RT3 Alternatives Considered

- **Hard links**: Rejected — break on cross-filesystem scenarios and don't
  survive file deletion of the original.
- **Always copy**: Rejected — symlinks provide better upgrade behavior on Unix
  (updating the original automatically updates all symlinks).

## Research Task 4: PATH Detection and Check Command Patterns

### RT4 Decision

Implement `finfocus analyzer check` with four sequential checks:

1. Policy pack directory existence
2. `PulumiPolicy.yaml` validity
3. Binary in PATH (using `exec.LookPath`)
4. gRPC smoke test (start server, probe, clean up)

### RT4 Analysis

**PATH detection**: Go's `exec.LookPath("pulumi-analyzer-policy-finfocus")`
is the idiomatic way to check if a binary is in PATH. Cross-platform, handles
Windows `.exe` extension automatically.

**gRPC smoke test**: Start `finfocus analyzer serve` as a subprocess, read the
port from stdout, make a `GetAnalyzerInfo` gRPC call, verify response, and
kill the subprocess. Use a 5-second timeout to avoid blocking.

**Output format**: The check command should support both human-readable table
output (with pass/fail indicators) and `--output json` for machine consumption.

### RT4 Alternatives Considered

- **Only checking file existence without gRPC**: Rejected — a binary might be
  present but broken (wrong architecture, missing dependencies). The gRPC smoke
  test catches runtime failures.
- **Running `which` instead of `exec.LookPath`**: Rejected — `which` is not
  portable to Windows.

## Research Task 5: Force Policy Pack Sync Behavior

### RT5 Decision

When `--force` is used and the policy pack directory exists, update the policy
pack binary. If the policy pack directory doesn't exist, silently skip the sync.
If the sync fails, log a warning but don't fail the overall install.

### RT5 Analysis

**Sync flow**:

1. `Install()` with `Force=true` already removes old Pulumi plugin directories
   and creates new ones.
2. After the Pulumi plugin install succeeds, check if the policy pack directory
   (`~/.finfocus/analyzer/`) exists.
3. If it exists, update the binary reference (re-symlink or re-copy).
4. If it doesn't exist, skip — the user hasn't opted into the policy pack
   workflow yet.

**Error handling**: The Pulumi plugin install is the primary operation. Policy
pack sync is a secondary convenience. A failed sync should not roll back the
successful plugin install.

### RT5 Alternatives Considered

- **Always create the policy pack directory on `--force`**: Rejected — the user
  might have intentionally removed it. Only sync if it already exists.
- **Failing the install on sync failure**: Rejected — the Pulumi plugin install
  already succeeded, and the user can manually fix the policy pack.
