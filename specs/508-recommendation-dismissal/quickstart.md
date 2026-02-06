# Quickstart: Recommendation Dismissal

**Feature**: 508-recommendation-dismissal

## Basic Usage

### Dismiss a recommendation

```bash
# Get recommendations first to see IDs
finfocus cost recommendations --pulumi-json plan.json --output json

# Dismiss by ID with a reason
finfocus cost recommendations dismiss rec-123abc \
  --reason business-constraint \
  --note "Intentional oversizing for burst capacity" \
  --pulumi-json plan.json

# Dismiss without confirmation prompt
finfocus cost recommendations dismiss rec-123abc \
  --reason not-applicable \
  --force \
  --pulumi-json plan.json
```

### Snooze a recommendation

```bash
# Snooze until a specific date
finfocus cost recommendations snooze rec-456def \
  --until 2026-04-01 \
  --reason deferred \
  --note "Scheduled for Q2 infrastructure review" \
  --pulumi-json plan.json

# Update snooze date (direct re-snooze)
finfocus cost recommendations snooze rec-456def \
  --until 2026-07-01 \
  --pulumi-json plan.json
```

### List recommendations

```bash
# Default: excludes dismissed/snoozed
finfocus cost recommendations --pulumi-json plan.json

# Include dismissed and snoozed in output
finfocus cost recommendations --pulumi-json plan.json --include-dismissed
```

### Undismiss a recommendation

```bash
# Re-enable a dismissed or snoozed recommendation
finfocus cost recommendations undismiss rec-123abc
```

### View history

```bash
# See lifecycle events for a recommendation
finfocus cost recommendations history rec-123abc

# JSON output
finfocus cost recommendations history rec-123abc --output json
```

## Valid Dismissal Reasons

| Flag Value             | When to Use                                    |
| ---------------------- | ---------------------------------------------- |
| `not-applicable`       | Recommendation doesn't apply to your situation |
| `already-implemented`  | You've already acted on this recommendation    |
| `business-constraint`  | Business requirements prevent action           |
| `technical-constraint` | Technical limitations prevent action           |
| `deferred`             | Will address later (use with `snooze`)         |
| `inaccurate`           | Recommendation data or savings estimate is wrong |
| `other`                | Custom reason (requires `--note`)              |

## How It Works

1. **Plugin-primary**: If your plugin supports recommendation dismissal, the CLI calls the plugin's `DismissRecommendation` RPC. The plugin persists the dismissal in its own storage.
2. **Local fallback**: The CLI always saves dismissals locally to `~/.finfocus/dismissed.json` for client-side filtering and audit trail.
3. **Automatic filtering**: Dismissed IDs are passed to plugins via `ExcludedRecommendationIds` in subsequent requests.
4. **Auto-unsnooze**: Snoozed recommendations automatically reappear when the snooze date passes.
