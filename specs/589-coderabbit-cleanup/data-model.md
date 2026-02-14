# Data Model: CodeRabbit Cleanup

**Branch**: `589-coderabbit-cleanup` | **Date**: 2026-02-13

## Overview

This cleanup task introduces no new entities, data structures, or storage changes. The only structural change is the conversion of package-level functions to methods on a new `PulumiClient` struct (Group 5).

## New Type: PulumiClient (Group 5)

### Definition

```text
PulumiClient
├── runner: CommandRunner (interface)
└── Methods:
    ├── GetCurrentStack(ctx, projectDir) → (string, error)
    ├── Preview(ctx, opts) → ([]byte, error)
    ├── StackExport(ctx, opts) → ([]byte, error)
    └── runPulumiCommand(ctx, cfg) → ([]byte, error) [private]
```

### Constructors

- `NewClient()` - Creates client with default `execRunner` (production use)
- `NewClientWithRunner(r CommandRunner)` - Creates client with injected runner (test use)

### Relationships

```text
PulumiClient ──uses──▶ CommandRunner (interface)
     │
     ├──called by──▶ detectPulumiProject() in common_execution.go
     ├──called by──▶ resolveResourcesFromPulumi() in common_execution.go
     └──called by──▶ loadOverviewFromAutoDetect() / resolveOverviewPlan() in overview.go
```

### Unchanged Package-Level Functions

These remain as standalone functions (no `Runner` dependency):

- `FindBinary()` - Uses `exec.LookPath` directly
- `FindProject(dir)` - Uses `os.Stat` directly

## Modified Behavior: parseMetadataFlags (Group 3e)

### Metadata Before

```text
Input: "=value" → Output: map["": "value"] (empty key accepted)
```

### Metadata After

```text
Input: "=value" → Output: nil, warning "ignored metadata entry '=value': empty key"
Input: "  =value" → Output: nil, warning (trimmed key is empty)
```

## Modified Behavior: Registry Region Enrichment (Group 3f)

### Enrichment Before

```text
meta == nil → enrich with region from binary name
meta != nil → no enrichment (even if region key missing)
```

### Enrichment After

```text
meta == nil → enrich with region from binary name (create new map)
meta != nil, no "region" key → enrich with region from binary name (add to existing map)
meta != nil, has "region" key → preserve existing value (no overwrite)
```
