# Research: CodeRabbit Cleanup

**Branch**: `589-coderabbit-cleanup` | **Date**: 2026-02-13

## R1: Conditional Simplification Verification (Group 3a)

**Decision**: The conditional `params.statePath != "" || (params.planPath == "" && params.statePath == "")` simplifies to `params.planPath == ""`.

**Rationale**: The `validateActualInputFlags` function enforces mutual exclusivity between `--pulumi-json` and `--pulumi-state`. After validation passes, exactly one of three states holds:

1. `planPath != ""` and `statePath == ""` (plan provided)
2. `planPath == ""` and `statePath != ""` (state provided)
3. `planPath == ""` and `statePath == ""` (auto-detection)

The original condition matches cases 2 and 3. Since case 1 is `planPath != ""`, the simplified condition `planPath == ""` is logically equivalent.

**Alternatives considered**: Keep verbose condition with explanatory comment. Rejected because the simplified form is clearer and the mutual exclusivity invariant is enforced upstream.

## R2: Audit Pattern for Auto-Detect Path (Group 3c)

**Decision**: Mirror the existing `loadAndMapResources` audit pattern by calling `audit.logFailure(ctx, err)` before returning the error from the auto-detect path.

**Rationale**: The audit context is already created at `cost_projected.go:147` before the branching logic. The `loadAndMapResources` path (via `common_execution.go:65,72`) calls `audit.logFailure` on both plan loading and resource mapping errors. The auto-detect path should be consistent.

**Implementation**: After `resolveResourcesFromPulumi` fails (line 159), add `audit.logFailure(ctx, err)` before `return err`. The audit params already record `"pulumi_json": "auto-detect"` at line 155.

## R3: PulumiClient Struct Design (Group 5)

**Decision**: Convert `GetCurrentStack`, `Preview`, `StackExport`, and `runPulumiCommand` to methods on a `PulumiClient` struct. Keep `FindBinary` and `FindProject` as package-level functions (they don't use `Runner`).

**Rationale**: The global `Runner` variable is a shared mutable state concern. A struct-based approach enables concurrent test isolation and follows Go idioms for dependency injection.

**Alternatives considered**:

1. **Functional injection** (pass `CommandRunner` to each function): Rejected because it adds noise to every call site and doesn't provide a natural place for future state.
2. **Interface on existing functions**: Rejected because Go doesn't support interfaces on package-level functions.
3. **Context-based injection**: Rejected because it hides the dependency and makes testing less explicit.

**Call site impact**:

- `common_execution.go`: `detectPulumiProject` needs a `*PulumiClient` parameter or creates one internally
- `overview.go`: Same pattern for `loadOverviewFromAutoDetect` and `resolveOverviewPlan`
- Tests: Replace `setMockRunner` with `NewClientWithRunner(mock)`

## R4: Registry Region Enrichment Fix (Group 3f)

**Decision**: When `meta` is non-nil but lacks a `"region"` key, parse region from binary name and add it.

**Rationale**: The current code only enriches metadata when `meta == nil`. This misses the case where `plugin.metadata.json` exists with other keys but no region. The fix checks `if _, ok := meta["region"]; !ok` regardless of whether meta was nil.

**Implementation**:

```go
if meta == nil {
    if region, ok := ParseRegionFromBinaryName(binPath); ok {
        meta = map[string]string{"region": region}
    }
} else if _, ok := meta["region"]; !ok {
    if region, ok := ParseRegionFromBinaryName(binPath); ok {
        meta["region"] = region
    }
}
```

## R5: Context Cancellation Test Pattern (Group 4b)

**Decision**: Replace `time.Sleep(60ms)` with explicit `cancel()` before invoking the function under test.

**Rationale**: The current tests create a context with 50ms timeout, then sleep 60ms to ensure it expires. This is timing-dependent and can flake under load. Explicitly canceling the context before the function call is deterministic.

**Implementation**:

```go
ctx, cancel := context.WithCancel(context.Background())
cancel() // Cancel immediately before calling function
_, err := Preview(ctx, PreviewOptions{...})
```

Note: The mock runner already returns `context.DeadlineExceeded`, so the test still verifies the error wrapping behavior. The key insight is that the function under test checks `ctx.Err()` before or during execution.

## R6: NotFoundError Message Update (Group 3g)

**Decision**: Update error message from `"provide --pulumi-json"` to `"provide --pulumi-json or --pulumi-state"`.

**Rationale**: Since the `cost actual` command now supports `--pulumi-state`, the error message should mention both options. The `NotFoundError` is returned by `FindBinary()` which is called from both `cost projected` and `cost actual` paths.

**Alternatives considered**: Separate error messages per command. Rejected because `NotFoundError` is a shared function and both flags are valid alternatives in all contexts.
