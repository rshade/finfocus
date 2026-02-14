# Data Model: finfocus setup — One-Command Bootstrap

**Date**: 2026-02-14
**Branch**: `591-setup-command`

## Entities

### StepStatus

Enumeration representing the outcome of a single setup step.

| Value     | Meaning                                          |
|-----------|--------------------------------------------------|
| `success` | Step completed successfully                      |
| `warning` | Step completed with a non-fatal issue            |
| `skipped` | Step was intentionally skipped (via flag)         |
| `error`   | Step failed (may or may not be critical)          |

### StepResult

The outcome of executing a single setup step.

| Field    | Type       | Description                                      |
|----------|------------|--------------------------------------------------|
| Name     | string     | Human-readable step name (e.g., "Create directories") |
| Status   | StepStatus | Outcome of the step                              |
| Message  | string     | Status line message displayed to the user        |
| Critical | bool       | Whether failure of this step causes non-zero exit |
| Error    | error      | Underlying error, if any (nil on success/skip)   |

### SetupOptions

Configuration for the setup command, derived from CLI flags.

| Field          | Type   | Default | Description                              |
|----------------|--------|---------|------------------------------------------|
| SkipAnalyzer   | bool   | false   | Skip Pulumi analyzer installation        |
| SkipPlugins    | bool   | false   | Skip default plugin installation         |
| NonInteractive | bool   | false   | Force non-interactive mode (ASCII output) |

### SetupResult

Aggregate outcome of all setup steps.

| Field      | Type           | Description                                |
|------------|----------------|--------------------------------------------|
| Steps      | []StepResult   | Ordered list of step outcomes              |
| HasErrors  | bool           | True if any critical step failed           |
| HasWarnings| bool           | True if any step produced a warning        |

**Derived behavior**:

- `HasErrors` is computed by scanning Steps for any entry where
  `Status == error && Critical == true`
- Exit code is 1 if `HasErrors` is true, 0 otherwise

## Step Criticality Classification

| Step                | Critical | Rationale                                    |
|---------------------|----------|----------------------------------------------|
| Version display     | No       | Informational only                           |
| Pulumi detection    | No       | Advisory — Pulumi is optional for some workflows |
| Directory creation  | Yes      | Required for all subsequent operations       |
| Config init         | Yes      | Required for CLI to function                 |
| Analyzer install    | No       | Optional component, can be installed later   |
| Plugin install      | No       | Optional component, can be installed later   |

## State Transitions

The setup command is stateless — it does not maintain persistent state beyond
what the individual steps create (directories, files). Each invocation evaluates
the current filesystem state and acts accordingly (idempotent).

## Relationships

```text
SetupOptions ──[configures]──> Setup Command
Setup Command ──[produces]──> SetupResult
SetupResult ──[contains]──> []StepResult
StepResult ──[has]──> StepStatus
```

## Validation Rules

- `SetupOptions`: No validation needed — all fields have safe defaults.
- `StepResult.Name`: Must be non-empty.
- `StepResult.Message`: Must be non-empty.
- `SetupResult.Steps`: Must contain exactly 6 entries (one per step) after
  a complete run, though some may be `skipped`.
