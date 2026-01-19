# Quickstart: Cost Commands TUI

**Feature Branch**: `106-cost-tui-upgrade`

This guide covers the enhanced terminal UI for FinFocus cost commands.

## Overview

The cost commands (`cost projected`, `cost actual`) now feature:

- **Styled output** with color-coded costs and provider breakdowns
- **Interactive navigation** for browsing resources with keyboard controls
- **Loading indicators** showing plugin query progress
- **Cost deltas** with trend indicators (when available)

## Basic Usage

### View Projected Costs

```bash
# Basic usage - styled output in TTY, plain text otherwise
finfocus cost projected --pulumi-json plan.json

# Force JSON output (no TUI styling)
finfocus cost projected --pulumi-json plan.json --output json
```

### View Actual Costs

```bash
# Historical costs with styled output
finfocus cost actual --pulumi-json plan.json --from 2025-01-01

# Daily breakdown with cross-provider table
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --group-by daily
```

## Interactive Mode

When running in a TTY terminal, you get full interactive mode:

### Keyboard Controls

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate resources |
| `Enter` | View resource details |
| `Esc` | Return to list |
| `s` | Open sort menu |
| `/` | Filter resources |
| `q` | Quit |

### Navigation Example

```text
┌──────────────────────────────────────────────────────────────────┐
│ COST SUMMARY                                                     │
├──────────────────────────────────────────────────────────────────┤
│ Total Monthly: $1,234.56 USD    Resources: 42                    │
│ AWS: $800.00 (64.8%)  GCP: $300.00 (24.3%)  Azure: $134.56       │
├──────────────────────────────────────────────────────────────────┤
│ Resource                      Type              Monthly    Delta │
│ ──────────────────────────────────────────────────────────────── │
│ > production-db               aws:rds/instance  $450.00  +$50 ↑  │
│   api-gateway                 aws:apigateway    $200.00   $0 →   │
│   k8s-cluster                 gcp:container     $180.00  -$20 ↓  │
└──────────────────────────────────────────────────────────────────┘
                    ↑/↓ Navigate  Enter: Details  q: Quit
```

## Output Modes

The TUI automatically detects your terminal and selects the best mode:

| Environment | Output Mode |
|-------------|-------------|
| TTY terminal | Interactive (Bubble Tea) |
| CI/CD pipeline | Styled text (Lip Gloss) |
| Piped/redirected | Plain text (no ANSI) |
| NO_COLOR set | Plain text |

### Force Plain Output

```bash
# Disable styling explicitly
finfocus cost projected --pulumi-json plan.json --output table

# Or set NO_COLOR environment variable
NO_COLOR=1 finfocus cost projected --pulumi-json plan.json
```

## Cost Deltas

When plugins provide delta information, you'll see trend indicators:

| Indicator | Color | Meaning |
|-----------|-------|---------|
| `+$50.00 ↑` | Orange | Cost increase |
| `-$20.00 ↓` | Green | Cost decrease (savings) |
| `$0.00 →` | Gray | No change |

## Loading States

During plugin queries, you'll see progress:

```text
  ⣾ Querying cost data from plugins...
    AWS: ✓ 12 resources
    GCP: ⣾ loading...
    Azure: pending
```

## Troubleshooting

### No Styling Appears

Check if your terminal supports colors:

```bash
echo $TERM
# Should be xterm-256color, screen-256color, or similar
```

### Interactive Mode Not Working

Ensure stdout is a TTY:

```bash
# This works (TTY)
finfocus cost projected --pulumi-json plan.json

# This falls back to plain (piped)
finfocus cost projected --pulumi-json plan.json | less
```

### Narrow Terminal Issues

If terminal width is less than 60 characters, output falls back to plain text.
Resize your terminal window for styled output.

## Configuration

No additional configuration is required. The TUI respects:

- Terminal width and height (auto-detected)
- `NO_COLOR` environment variable
- `--output` flag for explicit format selection
