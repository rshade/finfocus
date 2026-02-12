---
layout: default
title: overview Command
description: Unified cost dashboard combining state, plan, actual costs, projected costs, drift, and recommendations
---

# finfocus overview

Display a unified cost dashboard combining Pulumi state and plan data with actual
costs, projected costs, drift analysis, and recommendations.

## Usage

```bash
finfocus overview --pulumi-state <file> [options]
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--pulumi-state` | Path to Pulumi state JSON (required) | - |
| `--pulumi-json` | Path to Pulumi preview JSON | - |
| `--from` | Start date (YYYY-MM-DD or RFC3339) | 1st of current month |
| `--to` | End date (YYYY-MM-DD or RFC3339) | Now |
| `--adapter` | Restrict to a specific adapter plugin | All plugins |
| `--output` | Output format: table, json, ndjson | table |
| `--filter` | Resource filters (repeatable) | - |
| `--plain` | Force non-interactive plain text output | false |
| `--yes`, `-y` | Skip confirmation prompts | false |
| `--no-pagination` | Disable pagination (plain mode only) | false |

## Examples

### Interactive dashboard (default)

```bash
finfocus overview --pulumi-state state.json
```

Opens an interactive TUI with progressive data loading. Resources appear as
they are enriched with cost data.

### With pending changes from plan

```bash
finfocus overview --pulumi-state state.json --pulumi-json plan.json
```

Shows resources with pending changes and their cost impact.

### Plain text table output

```bash
finfocus overview --pulumi-state state.json --plain --yes
```

Renders an ASCII table suitable for piping or CI/CD environments.

### JSON output for scripting

```bash
finfocus overview --pulumi-state state.json --output json --yes
```

Produces structured JSON with metadata, resources, summary, and errors.

### NDJSON streaming output

```bash
finfocus overview --pulumi-state state.json --output ndjson --yes
```

One JSON object per line, suitable for streaming processors.

### Custom date range

```bash
finfocus overview --pulumi-state state.json --from 2025-01-01 --to 2025-01-31
```

### Filter by provider

```bash
finfocus overview --pulumi-state state.json --filter provider=aws --yes
```

### Filter by resource type

```bash
finfocus overview --pulumi-state state.json --filter type=aws:ec2/instance:Instance --yes
```

## Interactive TUI

When running in a terminal (TTY) without `--plain`, the overview launches an
interactive dashboard built with Bubble Tea.

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `Up` / `k` | Move cursor up |
| `Down` / `j` | Move cursor down |
| `Enter` | Open resource detail view |
| `Escape` | Return to list / clear filter |
| `s` | Cycle sort field (Cost, Name, Type, Delta) |
| `/` | Enter filter mode |
| `PgUp` / `PgDn` | Navigate pages (when >250 resources) |
| `q` / `Ctrl+C` | Quit |

### Progressive loading

The dashboard opens immediately and shows a progress banner while fetching cost
data from plugins. Resources update in-place as data arrives.

### Resource detail view

Press Enter on a resource to see a detailed breakdown including:

- Actual cost (MTD) with breakdown by category
- Projected cost (monthly) with breakdown
- Cost drift analysis with extrapolation
- Optimization recommendations with estimated savings

## Output formats

### Table (default)

ASCII table with columns: Resource, Type, Status, Actual(MTD), Projected,
Delta, Drift%, Recs.

Status icons: check mark (active), + (creating), ~ (updating), - (deleting),
clockwise arrow (replacing).

### JSON

Structured JSON object:

```json
{
  "metadata": {
    "stackName": "prod",
    "region": "us-east-1",
    "timeWindow": { "Start": "...", "End": "..." },
    "hasChanges": true,
    "totalResources": 50,
    "pendingChanges": 10,
    "generatedAt": "..."
  },
  "resources": [ ... ],
  "summary": {
    "totalActualMTD": 1234.56,
    "projectedMonthly": 5678.90,
    "projectedDelta": 4444.34,
    "potentialSavings": 500.00,
    "currency": "USD"
  },
  "errors": [ ... ]
}
```

### NDJSON

One JSON object per line, no metadata wrapper:

```text
{"urn":"urn:pulumi:...","type":"aws:ec2:Instance","status":"active",...}
{"urn":"urn:pulumi:...","type":"aws:s3:Bucket","status":"creating",...}
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error (invalid input, plugin failure) |
| 2 | User cancelled pre-flight prompt |
| 130 | Interrupted (Ctrl+C) |

## Filter syntax

Filters use `key=value` format and support:

- `provider=aws` - Filter by cloud provider
- `type=aws:ec2/instance:Instance` - Filter by resource type
- `status=active` - Filter by resource status

Multiple filters are ANDed together.

## Troubleshooting

### No cost data shown

Ensure plugins are installed and accessible. Run `finfocus plugin list` to verify.
The overview enrichment requires at least one plugin to fetch cost data.

### Slow loading

The overview fetches cost data concurrently (up to 10 resources at a time).
Large stacks with many resources may take longer. Use `--filter` to narrow scope.

### Drift warnings

A warning icon appears when the extrapolated monthly spend differs from projected
cost by more than 10%. This helps identify resources with unexpected cost changes.
