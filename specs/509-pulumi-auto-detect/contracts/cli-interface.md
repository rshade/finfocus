# CLI Interface Contract: Automatic Pulumi Integration

**Feature**: 509-pulumi-auto-detect
**Date**: 2026-02-11

## New Flag: `--stack`

**Parent Command**: `finfocus cost` (persistent, inherited by all subcommands)

| Attribute | Value |
|-----------|-------|
| Name | `--stack` |
| Type | string |
| Required | No |
| Default | "" (empty = auto-detect current stack) |
| Description | "Pulumi stack name for auto-detection (ignored with --pulumi-json/--pulumi-state)" |

## Modified Command: `finfocus cost projected`

### Flag Changes

| Flag | Before | After |
|------|--------|-------|
| `--pulumi-json` | Required | Optional (auto-detected from Pulumi project if omitted) |

### Behavior Matrix

| `--pulumi-json` | `--stack` | Behavior |
|-----------------|-----------|----------|
| Provided | Any | Use file (existing behavior). `--stack` ignored. |
| Omitted | Omitted | Auto-detect project + current stack + run preview |
| Omitted | Provided | Auto-detect project + use specified stack + run preview |

### Help Text (Long Description)

```text
Calculate projected infrastructure costs from Pulumi resource definitions.

When run inside a Pulumi project directory, automatically detects the project
and runs a preview to calculate costs. Alternatively, provide a pre-generated
preview JSON file with --pulumi-json.

Examples:
  # Auto-detect from Pulumi project (recommended)
  finfocus cost projected

  # Use a specific stack
  finfocus cost projected --stack production

  # Use a pre-generated file
  finfocus cost projected --pulumi-json plan.json

  # Combine with output format
  finfocus cost projected --output json
```

### Error Responses

| Condition | Exit Code | Error Message |
|-----------|-----------|---------------|
| No `pulumi` in PATH | 1 | "pulumi CLI not found in PATH; install from <https://www.pulumi.com/docs/install/> or provide --pulumi-json" |
| No Pulumi.yaml found | 1 | "no Pulumi project found in current or parent directories; use --pulumi-json to provide input directly" |
| No active stack | 1 | "no active Pulumi stack; use --stack to specify one (available: dev, staging, production)" |
| Preview fails | 1 | "pulumi preview failed: [stderr from pulumi]" |
| Preview timeout | 1 | "pulumi preview timed out after 5m0s" |

## Modified Command: `finfocus cost actual`

### Actual Flag Changes

| Flag | Before | After |
|------|--------|-------|
| `--pulumi-json` | One of json/state required | Optional (auto-detected if both omitted) |
| `--pulumi-state` | One of json/state required | Optional (auto-detected if both omitted) |

### Actual Behavior Matrix

| `--pulumi-json` | `--pulumi-state` | `--stack` | `--from` | Behavior |
|-----------------|-------------------|-----------|----------|----------|
| Provided | - | Any | Required | Use plan file (existing). `--stack` ignored. |
| - | Provided | Any | Optional | Use state file (existing). `--stack` ignored. |
| Omitted | Omitted | Any | Optional | Auto-detect: export stack state, auto-detect `--from` |
| Provided | Provided | Any | Any | Error: mutually exclusive |

### Actual Help Text (Long Description)

```text
Retrieve actual historical costs for deployed infrastructure.

When run inside a Pulumi project directory, automatically exports the stack
state and calculates actual costs. The start date is auto-detected from
resource creation timestamps. Alternatively, provide input files directly.

Examples:
  # Auto-detect from Pulumi project (recommended)
  finfocus cost actual

  # Use a specific stack with date override
  finfocus cost actual --stack production --from 2026-01-01

  # Use a pre-generated state file
  finfocus cost actual --pulumi-state state.json

  # Use a pre-generated plan file with explicit date
  finfocus cost actual --pulumi-json plan.json --from 2026-01-01
```

### Actual Error Responses

Same as `cost projected` errors, plus:

| Condition | Exit Code | Error Message |
|-----------|-----------|---------------|
| Export fails | 1 | "pulumi stack export failed: [stderr from pulumi]" |
| Export timeout | 1 | "pulumi stack export timed out after 1m0s" |
| No timestamps in state | 1 | "no resource timestamps found in state; use --from to specify start date" |

## New Package API: `internal/pulumi`

### Exported Functions

```text
FindBinary() (string, error)
FindProject(dir string) (string, error)
GetCurrentStack(ctx context.Context, projectDir string) (string, error)
Preview(ctx context.Context, opts PreviewOptions) ([]byte, error)
StackExport(ctx context.Context, opts ExportOptions) ([]byte, error)
```

### Exported Types

```text
PreviewOptions { ProjectDir string; Stack string; Timeout time.Duration }
ExportOptions  { ProjectDir string; Stack string; Timeout time.Duration }
StackInfo      { Name string; Current bool; URL string }
```

### Exported Errors

```text
ErrPulumiNotFound  - sentinel: pulumi binary not in PATH
ErrNoProject       - sentinel: no Pulumi.yaml found
ErrNoCurrentStack  - sentinel: no stack marked as current
ErrPreviewFailed   - wraps: pulumi preview non-zero exit
ErrExportFailed    - wraps: pulumi stack export non-zero exit
```

## Extended Ingestion API: `internal/ingest`

### New Functions (additive, no breaking changes)

```text
ParsePulumiPlan(data []byte) (*PulumiPlan, error)
ParsePulumiPlanWithContext(ctx context.Context, data []byte) (*PulumiPlan, error)
ParseStackExport(data []byte) (*StackExport, error)
ParseStackExportWithContext(ctx context.Context, data []byte) (*StackExport, error)
```

### Existing Functions (unchanged external behavior)

```text
LoadPulumiPlan(path string) (*PulumiPlan, error)              # Internally refactored to call ParsePulumiPlan
LoadPulumiPlanWithContext(ctx, path) (*PulumiPlan, error)      # Internally refactored to call ParsePulumiPlanWithContext
LoadStackExport(path string) (*StackExport, error)             # Internally refactored to call ParseStackExport
LoadStackExportWithContext(ctx, path) (*StackExport, error)    # Internally refactored to call ParseStackExportWithContext
```
