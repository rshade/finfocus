# Data Model: Neo-Friendly CLI Fixes

**Feature Branch**: `590-neo-cli-fixes`
**Date**: 2026-02-13

## Entities

### StructuredError (New)

A machine-readable error representation for JSON/NDJSON output.

| Field | Type | JSON Key | Description |
|-------|------|----------|-------------|
| Code | `string` | `code` | Stable error category identifier |
| Message | `string` | `message` | Human-readable error description |
| ResourceType | `string` | `resourceType` | Affected resource type (e.g., `aws:ec2:Instance`) |

**Location**: `internal/engine/types.go`

**Error Code Enumeration** (stable, additive-only per FR-005):

| Constant | Value | When Used |
|----------|-------|-----------|
| `ErrCodePluginError` | `"PLUGIN_ERROR"` | gRPC call to plugin fails |
| `ErrCodeValidationError` | `"VALIDATION_ERROR"` | Pre-flight request validation fails |
| `ErrCodeTimeoutError` | `"TIMEOUT_ERROR"` | Plugin call exceeds deadline |
| `ErrCodeNoCostData` | `"NO_COST_DATA"` | No plugin or spec returns cost data |

### CostResult (Modified)

Add optional `Error` field to existing `CostResult` struct.

| Field | Type | JSON Key | Change |
|-------|------|----------|--------|
| Error | `*StructuredError` | `error,omitempty` | **Added** |

**Behavior**:

- When `Error` is non-nil and output is JSON/NDJSON: serialized as nested object
- When `Error` is non-nil and output is table: ignored (Notes field used as before)
- When `Error` is non-nil: `Notes` field MUST NOT contain `ERROR:` or `VALIDATION:`
  prefixes (FR-011). The original message is in `Error.Message`.

### BudgetExitError (Existing, No Changes)

Already exists at `internal/cli/cost_budget.go:767-776`. No structural changes needed.

| Field | Type | Description |
|-------|------|-------------|
| ExitCode | `int` | Process exit code (0-255) |
| Reason | `string` | Human-readable exit reason |

**Fix needed**: `main()` must use `errors.As()` to extract `ExitCode` instead of
always exiting with code 1.

### enrichedPluginInfo (Existing, No Structural Changes)

Already exists at `internal/cli/plugin_list.go:100-108` with JSON struct tags.
No field changes needed; only output rendering changes.

| Field | Type | JSON Key | Source |
|-------|------|----------|--------|
| Name | `string` | `name` | Registry |
| Version | `string` | `version` | Registry |
| Path | `string` | `path` | Registry |
| SpecVersion | `string` | `specVersion` | gRPC runtime |
| RuntimeVersion | `string` | `runtimeVersion` | gRPC runtime |
| SupportedProviders | `[]string` | `supportedProviders` | gRPC runtime |
| Capabilities | `[]string` | `capabilities` | gRPC runtime |
| Notes | `string` | `notes` | Error/status |

## Relationships

```text
CostResult ---> StructuredError (0..1, optional error detail)
BudgetExitError ---> main() (via errors.As extraction)
enrichedPluginInfo ---> JSON output (via json.Marshal)
```

## State Transitions

### Exit Code Flow

```text
Budget Config (exit_code: N)
    → Engine BudgetStatus.GetExitCode() → N
    → CLI checkBudgetExit() → BudgetExitError{ExitCode: N}
    → Cobra RunE → returns error
    → main() run() → returns error
    → main() → errors.As(*BudgetExitError) → os.Exit(N)
```

### Error Code Assignment Flow

```text
Plugin Call
    ├── Success → No error
    ├── Validation failure → StructuredError{Code: "VALIDATION_ERROR"}
    ├── Timeout → StructuredError{Code: "TIMEOUT_ERROR"}
    ├── Other gRPC failure → StructuredError{Code: "PLUGIN_ERROR"}
    └── No data returned → StructuredError{Code: "NO_COST_DATA"}
```

## Validation Rules

- Error codes are string constants, not user input; no runtime validation needed
- Exit codes are already validated in `config.BudgetConfig.Validate()` (0-255 range)
- Plugin list JSON output: empty array `[]` when no plugins (never null)
- `StructuredError.Code` must be one of the four defined constants
