---
layout: default
title: overview Command
description: Unified cost dashboard combining state, plan, actual costs, projected costs, drift, and recommendations
---

Display a unified cost dashboard combining Pulumi state and plan data with actual
costs, projected costs, drift analysis, and recommendations.

## Usage

```bash
finfocus overview [options]
```

All flags are optional. When `--pulumi-state` and `--pulumi-json` are omitted,
the command auto-detects your Pulumi project and stack from the current directory.

> **Tip:** Running `finfocus` with no arguments inside a Pulumi project directory
> automatically launches the overview — no subcommand needed.

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--pulumi-state` | Path to Pulumi state JSON (skips auto-detection) | Auto-detected |
| `--pulumi-json` | Path to Pulumi preview JSON (skips auto-detection) | Auto-detected |
| `--stack` | Pulumi stack name for auto-detection (ignored with `--pulumi-state`/`--pulumi-json`) | Current stack |
| `--from` | Start date (YYYY-MM-DD or RFC3339) | 1st of current month |
| `--to` | End date (YYYY-MM-DD or RFC3339) | Now |
| `--adapter` | Restrict to a specific adapter plugin | All plugins |
| `--output` | Output format: table, json, ndjson | table |
| `--filter` | Resource filters (repeatable) | - |
| `--plain` | Force non-interactive plain text output | false |
| `--yes`, `-y` | Skip confirmation prompts | false |
| `--cache-ttl` | Cache TTL in seconds; 0 disables caching | 0 (disabled) |
| `--no-pagination` | Disable pagination (plain mode only) | false |
| `--exit-on-threshold` | Exit non-zero when budget threshold exceeded | false |
| `--exit-code` | Exit code for threshold breach (0-255) | 1 |
| `--state-only` | Skip pulumi preview (faster, no pending change detection) | false |
| `--budget-scope` | Filter budget scopes: global, provider, tag, type | All |

## Auto-Detection

When no `--pulumi-state` or `--pulumi-json` flags are provided, `finfocus overview`
locates your Pulumi project automatically:

1. Walks up from the current directory to find `Pulumi.yaml`.
2. Runs `pulumi stack export` to fetch the current state JSON.
3. Runs `pulumi preview --json` to fetch the pending-change plan.
4. Uses `--stack` to select a non-default stack (e.g., `--stack production`).

Use `--pulumi-state` / `--pulumi-json` to skip these Pulumi CLI calls and provide
pre-exported files directly — useful in CI/CD when you manage the export step yourself.

## Examples

### Auto-detect from current directory (recommended)

```bash
finfocus overview
```

Walks up from the current directory to find `Pulumi.yaml`, exports the current
stack state, and runs `pulumi preview` automatically. Opens an interactive TUI
with progressive data loading.

### Cost-only overview (skip preview)

```bash
finfocus overview --state-only
```

Skips `pulumi preview` entirely, reducing overview time from ~18s to ~3s. Shows
cost data from the current stack state without detecting pending infrastructure
changes. In the TUI, the `p` key is still available to run preview on demand.

### Specific stack with auto-detection

```bash
finfocus overview --stack production
```

Same as above but selects the `production` stack instead of the current default.

### Pre-exported files (CI/CD or offline)

```bash
finfocus overview --pulumi-state state.json --pulumi-json plan.json
```

Opens an interactive TUI with progressive data loading. Resources appear as
they are enriched with cost data. Use this when you manage the `pulumi stack export`
and `pulumi preview --json` steps yourself.

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

### With caching enabled

```bash
finfocus overview --cache-ttl 300 --pulumi-state state.json
```

Enables a 5-minute cache. The first run enriches all resources via plugin calls and
stores results locally. Subsequent runs within the TTL window show `(cached)` in the
adapter field and complete faster.

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

## Budget Status

The overview command displays budget health information differently depending on
the output mode.

### Visibility by Output Mode

| Mode | Budget Shown | Details |
|------|-------------|---------|
| Interactive TUI | Footer bar + detail view | Color-coded health badge, spend/limit, utilization %. Press Enter for per-budget breakdown with forecasts and triggered alerts |
| Plain (`--plain`) | Not rendered | Budget enforcement available via `--exit-on-threshold` |
| JSON (`--output json`) | `budgets` array in output | Full budget health objects with utilization, forecasted spend, triggered thresholds |
| NDJSON (`--output ndjson`) | Not included | NDJSON is resource-scoped; budgets are stack-scoped |

### Budget Flags

| Flag | Description | Applies To |
|------|-------------|-----------|
| `--exit-on-threshold` | Exit non-zero when budget threshold exceeded | Plain, JSON |
| `--exit-code` | Exit code when threshold exceeded (0-255) | Plain, JSON |
| `--budget-scope` | Filter budget scopes (global, provider, tag, type) | All modes |

### Why is budget status missing in plain or NDJSON mode?

Plain mode focuses on the resource table for piping and CI/CD use. Use
`--exit-on-threshold` for budget enforcement in non-interactive environments.
NDJSON emits one object per resource and budgets are stack-scoped, so they
do not fit the per-line streaming model. Use `--output json` or the interactive
TUI to see full budget details.

## Output formats

### Table (default)

ASCII table with the following columns:

| Column | Description |
|--------|-------------|
| `Resource` | Resource name truncated to 40 characters; full URN shown in detail view |
| `Type` | Pulumi resource type (e.g., `aws:ec2/instance:Instance`) |
| `Status` | Lifecycle state with an icon prefix (see below) |
| `Actual(MTD)` | Month-to-date actual spend fetched from your cost adapter plugin |
| `Projected` | Estimated monthly cost from your pricing plugin |
| `Delta` | Projected minus Actual(MTD) — positive means projected exceeds actual |
| `Drift%` | Annualized deviation: how much the extrapolated monthly spend differs from projected |
| `Recs` | Number of open optimization recommendations (`N(-M)` format where M = dismissed) |

**Status icons:**

| Icon | Status | Meaning |
|------|--------|---------|
| ✓ | `active` | Resource exists and is running |
| `+` | `creating` | Resource will be created by the pending plan |
| `~` | `updating` | Resource will be updated by the pending plan |
| `-` | `deleting` | Resource will be deleted by the pending plan |
| `↻` | `replacing` | Resource will be deleted and re-created |

### JSON

Structured JSON object:

```json
{
  "metadata": {
    "stackName": "prod",
    "region": "us-east-1",
    "timeWindow": { "start": "...", "end": "..." },
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
  "budgets": [
    {
      "budgetID": "global",
      "health": "WARNING",
      "utilization": 85.2,
      "limit": 5000.00,
      "currentSpend": 4260.00,
      "forecastedSpend": 5800.00
    }
  ],
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
