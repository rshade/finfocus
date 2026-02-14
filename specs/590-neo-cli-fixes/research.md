# Research: Neo-Friendly CLI Fixes

**Feature Branch**: `590-neo-cli-fixes`
**Date**: 2026-02-13

## R1: Exit Code Propagation Mechanism

### Decision

Use `errors.As()` in `main()` to detect `*BudgetExitError` and extract the custom
exit code before falling back to `os.Exit(1)`.

### Rationale

- `BudgetExitError` already exists in `internal/cli/cost_budget.go:767-776` with
  an `ExitCode int` field
- The error flows correctly through Cobra's `Execute()` to the `run()` function
- Only `main()` needs to change (single point of fix)
- `errors.As()` is the idiomatic Go way to extract typed errors from wrapped chains

### Alternatives Considered

1. **Custom Cobra error handler via `SetFlagErrorFunc`**: Rejected because budget
   errors are not flag errors; they come from `RunE`.
2. **Exit code interface (`ExitCoder`)**: Over-engineering for a single error type.
   Could be added later if other exit-code-carrying errors appear.
3. **Signal-based exit (channel)**: Too complex for the simple requirement.

### Additional Finding: cost actual Bug

`internal/cli/cost_actual.go:247-249` logs the `BudgetExitError` but does not
return it (unlike `cost_projected.go:247-249` which correctly returns the error).
This must be fixed as part of this story.

## R2: Structured Error Object Design

### R2 Decision

Add a `StructuredError` struct to `internal/engine/types.go` and an optional
`Error *StructuredError` field (JSON: `"error,omitempty"`) to `CostResult`. Populate
it at the error origin points in `internal/proto/adapter.go` alongside the existing
`Notes` field. In JSON/NDJSON output the structured error serializes naturally. In
table output, behavior is unchanged.

### R2 Rationale

- **Additive change**: Adding an `Error` field to `CostResult` does not break
  existing consumers (table output ignores it, JSON gains it).
- **Error origin is centralized**: All error-producing code paths go through
  `adapter.go`'s `GetProjectedCostWithErrors()` and `GetActualCostWithErrors()`,
  plus `engine.go`'s "No pricing information available" path.
- **Existing `ErrorDetail`** tracks errors internally but is not exposed in JSON;
  the new `StructuredError` fills this gap.
- **FR-011 compliance**: When a `StructuredError` is present, the `Notes` field
  must not contain `ERROR:` or `VALIDATION:` prefixes. The human-readable message
  moves into `StructuredError.Message`.

### Error Code Mapping

| Scenario | Current Notes Prefix | New Error Code | Origin File |
|----------|---------------------|----------------|-------------|
| Pre-flight validation fails | `VALIDATION: %v` | `VALIDATION_ERROR` | `adapter.go:139,246` |
| Plugin gRPC call fails | `ERROR: %v` | `PLUGIN_ERROR` | `adapter.go:164,266` |
| Context deadline exceeded | (none, treated as plugin error) | `TIMEOUT_ERROR` | `engine.go/estimate.go` |
| No plugin/spec returns data | (no prefix) | `NO_COST_DATA` | `engine.go:398` |

### R2 Alternatives Considered

1. **Separate error array at top level**: Rejected because errors need per-resource
   association, not a flat list. Per-resource `Error` field is more ergonomic for
   consumers iterating over results.
2. **Replace Notes with Error entirely**: Rejected because Notes carries non-error
   information too (e.g., "Calculated from local spec"). Error is additive.
3. **Error codes as integers**: Rejected per FR-005; stable string codes are easier
   to match and more readable than numeric codes.

## R3: Plugin List Structured Output

### R3 Decision

Add `--output` flag (values: `table`, `json`) to the `plugin list` command. When
`json` is selected, marshal the `[]enrichedPluginInfo` slice as a JSON array. When
no plugins are installed, output `[]` (empty JSON array).

### R3 Rationale

- `enrichedPluginInfo` in `plugin_list.go:100-108` already has JSON struct tags
- The data structure already contains all required fields (name, version, path,
  supportedProviders, capabilities, notes)
- Follows the same `--output` flag pattern used by cost commands
- Empty array for no-plugins case (FR-008) matches JSON conventions

### R3 Alternatives Considered

1. **Separate `plugin list --json` boolean flag**: Rejected for consistency with
   cost commands which use `--output json`.
2. **NDJSON support for plugin list**: Deferred; plugin lists are small and don't
   benefit from streaming. Can be added later.
3. **YAML output**: Rejected; not requested by spec, and JSON is the standard
   machine-readable format for this project.

## R4: Timeout Error Detection

### R4 Decision

Detect `context.DeadlineExceeded` errors at the point where plugin call errors are
recorded and set the error code to `TIMEOUT_ERROR` instead of `PLUGIN_ERROR`.

### R4 Rationale

- Timeouts are already detected in `internal/engine/estimate.go:100-108` for
  logging purposes but are treated as generic plugin errors in the Notes field
- The detection should happen in `adapter.go` where errors are recorded, using
  `errors.Is(err, context.DeadlineExceeded)` to distinguish timeouts from other
  plugin failures
- This gives AI agents the ability to decide whether to retry (timeout) vs. report
  (plugin error)

### R4 Alternatives Considered

1. **Detect only in engine.go**: Rejected because the error recording happens in
   `adapter.go`, which is the right place to set the error code.
2. **Separate timeout error type**: Over-engineering; the context deadline error
   is sufficient for detection.
