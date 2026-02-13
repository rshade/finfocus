# Data Model: Automatic Pulumi Integration for Cost Commands

**Feature**: 509-pulumi-auto-detect
**Date**: 2026-02-11

## Entities

### PreviewOptions

Configuration for running a Pulumi preview command.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ProjectDir | string | Yes | Directory containing Pulumi.yaml |
| Stack | string | No | Specific stack name (empty = current) |
| Timeout | duration | No | Max execution time (default: 5 minutes) |

### ExportOptions

Configuration for running a Pulumi stack export command.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ProjectDir | string | Yes | Directory containing Pulumi.yaml |
| Stack | string | No | Specific stack name (empty = current) |
| Timeout | duration | No | Max execution time (default: 60 seconds) |

### StackInfo

Parsed representation of a single stack from `pulumi stack ls --json`.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | string | Yes | Stack name (e.g., "dev", "production") |
| Current | boolean | Yes | Whether this is the currently active stack |
| URL | string | No | Backend URL for this stack |

### Error Types

Sentinel errors for structured error handling across the feature.

| Error | Condition | User Message Pattern |
|-------|-----------|---------------------|
| ErrPulumiNotFound | `exec.LookPath("pulumi")` fails | "pulumi CLI not found; install from [URL] or use --pulumi-json" |
| ErrNoProject | No Pulumi.yaml/yml found walking up from cwd | "no Pulumi project found; use --pulumi-json or cd to project dir" |
| ErrNoCurrentStack | `pulumi stack ls` has no current stack | "no active stack; use --stack [available: dev, prod]" |
| ErrPreviewFailed | `pulumi preview --json` exits non-zero | "pulumi preview failed: [stderr output]" |
| ErrExportFailed | `pulumi stack export` exits non-zero | "stack export failed: [stderr output]" |

## Relationships

```text
CLI Command (cost projected / cost actual)
    │
    ├── [--pulumi-json provided] → existing file-based path (unchanged)
    ├── [--pulumi-state provided] → existing file-based path (unchanged)
    └── [no file flag] → Auto-Detection Flow
            │
            ├── FindBinary() → path or ErrPulumiNotFound
            ├── FindProject(cwd) → projectDir or ErrNoProject
            ├── GetCurrentStack(projectDir) or --stack → stackName or ErrNoCurrentStack
            │
            ├── [cost projected] → Preview(opts) → []byte → ParsePulumiPlan() → ResourceDescriptors
            └── [cost actual] → StackExport(opts) → []byte → ParseStackExport() → ResourceDescriptors
                    │
                    └── Continue to existing engine pipeline (unchanged)
```

## State Transitions

This feature is stateless — no persistent state or lifecycle management. Each CLI invocation is independent:

1. **Detection**: Find binary → Find project → Resolve stack
2. **Execution**: Run Pulumi CLI command → Capture stdout
3. **Parsing**: Parse JSON bytes → Map to ResourceDescriptors
4. **Handoff**: Pass descriptors to existing engine pipeline

No data is cached, stored, or persisted between invocations.

## Validation Rules

| Rule | Applies To | Description |
|------|-----------|-------------|
| Mutual exclusivity | CLI flags | `--pulumi-json` and `--pulumi-state` cannot both be provided |
| File precedence | CLI flags | Explicit file flags always override auto-detection |
| Stack flag scope | `--stack` | Ignored when file flags are provided |
| Timeout bounds | Preview/Export | Must be positive duration; enforced by context deadline |
| Project file | FindProject | Must be `Pulumi.yaml` or `Pulumi.yml` (case-sensitive on Linux, insensitive on macOS/Windows) |
