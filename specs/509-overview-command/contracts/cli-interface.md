# CLI Interface Contract

**Component**: `finfocus overview` command  
**Version**: v1.0.0  
**Date**: 2026-02-11

---

## Command Specification

### Command Name

```bash
finfocus overview
```

**Alias**: None (future consideration: `finfocus ov`)

---

## Command Flags

### Required Flags

None. The command operates on the current Pulumi stack by default.

### Optional Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--pulumi-json` | string | (auto-detect) | Path to Pulumi preview JSON output |
| `--pulumi-state` | string | (auto-detect) | Path to Pulumi state JSON from `pulumi stack export` |
| `--from` | string | (auto-detect from state) | Start date for actual costs (YYYY-MM-DD or RFC3339) |
| `--to` | string | (current time) | End date for actual costs (YYYY-MM-DD or RFC3339) |
| `--adapter` | string | "" (all adapters) | Restrict to specific adapter plugin |
| `--output` | string | (config default) | Output format: `table`, `json`, `ndjson` |
| `--filter` | []string | [] | Resource filter expressions (can specify multiple) |
| `--plain` | bool | false | Force non-interactive mode (skip TUI) |
| `--yes` / `-y` | bool | false | Skip pre-flight confirmation prompt |
| `--no-pagination` | bool | false | Disable automatic pagination (show all resources) |

### Inherited Global Flags

(From root command)

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--debug` | bool | false | Enable debug logging |
| `--cache-ttl` | int | 0 (use config) | Cache TTL in seconds |
| `--skip-version-check` | bool | false | Skip plugin version compatibility check |

---

## Command Behavior

### 1. Default Behavior (Interactive)

**When**: TTY is detected AND `--plain` is NOT set

1. Load Pulumi state and preview
2. Display pre-flight confirmation prompt (unless `--yes` is set)
3. Launch interactive TUI with:
   - Main table view
   - Progressive loading with progress banner
   - Keyboard navigation
   - Detail view on Enter
4. Exit on 'q', Ctrl+C, or Escape

**User Interaction**:
```
Stack: my-stack-dev
Resources: 150 total
Pending changes: 10 (5 create, 3 update, 2 delete)
Plugins: aws-plugin, azure-plugin

This will query costs for 150 resources across 2 plugins.
Estimated API calls: ~300 (actual + projected + recommendations)

Continue? [Y/n]: _
```

### 2. Non-Interactive Behavior (Plain Mode)

**When**: NO TTY detected OR `--plain` is set

1. Load Pulumi state and preview
2. Query costs for all resources (no confirmation prompt)
3. Render ASCII table to stdout
4. Print summary footer
5. Exit with code 0 (success) or 1 (error)

**Output Example**:
```
RESOURCE                          TYPE              STATUS    ACTUAL(MTD)  PROJECTED   DELTA     DRIFT%   RECS
aws:ec2/instance:Instance-web-1   aws:ec2/instance  active    $42.50       $100.00    +$57.50   +15%     3
aws:s3/bucket:Bucket-logs         aws:s3/bucket     active    $5.20        $12.00     +$6.80    +8%      1

SUMMARY
-------
Total Actual (MTD):        $47.70
Projected Monthly:         $312.00
Projected Delta:           +$264.30
Potential Savings (Recs):  $150.00
```

---

## Function Signature

```go
// NewOverviewCmd creates the "overview" subcommand for unified cost dashboard.
//
// The command merges Pulumi state, Pulumi preview, actual costs, projected costs,
// and recommendations into a single interactive or plain-text view.
//
// Interactive mode (default with TTY):
//   - Launches TUI with progressive loading
//   - Supports keyboard navigation, filtering, sorting
//   - Detail view for individual resources
//
// Non-interactive mode (no TTY or --plain flag):
//   - Renders ASCII table to stdout
//   - Includes summary footer with totals
//   - Suitable for CI/CD pipelines
//
// Returns a cobra.Command ready to add to the CLI command tree.
func NewOverviewCmd() *cobra.Command
```

---

## Input Validation

### Pre-Execution Checks

1. **Pulumi Context**: Verify we're in a Pulumi project directory (contains `Pulumi.yaml`)
2. **State/Preview Files**:
   - If `--pulumi-json` or `--pulumi-state` specified, files MUST exist
   - If not specified, attempt auto-detect:
     - Preview: `pulumi preview --json > /tmp/finfocus-preview-<uuid>.json`
     - State: `pulumi stack export > /tmp/finfocus-state-<uuid>.json`
3. **Date Range**:
   - `--from` must parse as valid date (YYYY-MM-DD or RFC3339)
   - `--to` must parse as valid date
   - `--to` must be after `--from`
   - Range must not exceed 366 days (per existing actual cost validation)
4. **Filter Expressions**:
   - Must match format: `key=value` or `key~regex`
   - Valid keys: `type`, `name`, `provider`, `tag`
5. **Adapter**: If specified, plugin must exist in registry

### Validation Error Examples

```bash
# Missing Pulumi context
$ finfocus overview
Error: not in a Pulumi project directory (missing Pulumi.yaml)

# Invalid date format
$ finfocus overview --from 2026-13-01
Error: invalid --from date: parsing time "2026-13-01": month out of range

# Invalid filter expression
$ finfocus overview --filter "invalid"
Error: invalid filter expression "invalid": expected format key=value or key~regex
```

---

## Output Formats

### Table Format (Default for TTY)

Interactive TUI with Bubble Tea table component.

**Columns**:
- Resource ID (truncated to 30 chars)
- Type (truncated to 20 chars)
- Status (icon + text)
- Actual (MTD, formatted currency)
- Projected (Monthly, formatted currency)
- Delta (formatted currency with +/- sign)
- Drift% (formatted percent with warning icon if >10%)
- Recs (count)

**Progressive Loading**:
- Show partial results as they arrive
- Display progress banner: "Loading: 45/100 resources (45%)"
- Banner disappears when all data loaded

### JSON Format (`--output json`)

Single JSON object with metadata and array of overview rows.

**Schema**: See [output-format.md](output-format.md)

### NDJSON Format (`--output ndjson`)

Newline-delimited JSON, one OverviewRow per line.

**Schema**: See [output-format.md](output-format.md)

---

## Exit Codes

| Code | Condition | Description |
|------|-----------|-------------|
| 0 | Success | All operations completed successfully |
| 1 | Error | Generic error (invalid args, file not found, etc.) |
| 2 | User Cancelled | User declined pre-flight confirmation |
| 130 | Interrupted | User pressed Ctrl+C during execution |

---

## Performance Expectations

- **Initial Render**: <500ms (requirement from spec)
- **Pre-Flight Prompt**: <100ms (fast state/preview parse)
- **Per-Resource Update**: <50ms (TUI refresh rate)
- **Total Execution Time**: Variable (depends on API latency)
  - Small stack (10 resources): 5-10 seconds
  - Medium stack (100 resources): 30-60 seconds
  - Large stack (250 resources): 60-120 seconds

---

## Error Handling

### Error Display (Interactive Mode)

Errors shown in:
1. **Top banner**: Transient errors (network blips, retryable)
2. **Table row**: Per-resource errors (auth failure, rate limit)
3. **Footer**: Summary of all errors at end

**Interactive Retry Prompt** (per spec edge case):
```
API call failed for aws:ec2/instance:Instance-web-1
Error: rate limit exceeded (429)
Retry? [y/n/skip]: _
```

### Error Display (Non-Interactive Mode)

```
ERRORS
======
aws:ec2/instance:Instance-web-1: rate limit exceeded (429)
aws:rds/instance:Database-main: authentication failed (check credentials)

2 errors occurred. See above for details.
```

---

## Examples

### Example 1: Basic Usage (Interactive)

```bash
finfocus overview
```

Auto-detects state and preview, launches interactive TUI.

### Example 2: Specific Files (Non-Interactive)

```bash
finfocus overview \
  --pulumi-state state.json \
  --pulumi-json plan.json \
  --plain
```

Uses provided files, outputs ASCII table.

### Example 3: Filter by Resource Type

```bash
finfocus overview --filter "type=aws:ec2/instance"
```

Shows only EC2 instances in TUI.

### Example 4: JSON Output for CI/CD

```bash
finfocus overview --output json --yes > overview.json
```

Skips confirmation, outputs JSON, suitable for automation.

### Example 5: Specific Date Range

```bash
finfocus overview --from 2026-02-01 --to 2026-02-11
```

Actual costs for specific period.

### Example 6: Specific Adapter

```bash
finfocus overview --adapter aws-plugin
```

Only query AWS resources (skip other providers).

---

## Testing Requirements

### Unit Tests

- Flag parsing and validation
- Input file detection logic
- Error message formatting

### Integration Tests

- Full command execution with fixture files
- TTY detection logic
- Pre-flight confirmation prompt behavior
- Exit code verification

### Snapshot Tests

- ASCII table output (golden file comparison)
- JSON output schema validation
- Error message formatting

**Test Coverage Target**: 95% (critical path per constitution)

---

## References

- **Data Model**: `../data-model.md`
- **Engine Interface**: `engine-interface.md`
- **TUI Interface**: `tui-interface.md`
- **Existing CLI**: `internal/cli/cost_actual.go`, `internal/cli/cost_projected.go`

---

**Contract Version**: v1.0.0  
**Last Updated**: 2026-02-11
