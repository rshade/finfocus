---
title: CLI Commands Reference
description: Complete reference for all FinFocus CLI commands
layout: docs
---

Complete command reference for FinFocus.

## Commands Overview

```bash
finfocus                    # Auto-detects Pulumi project; opens overview if found, otherwise shows help
finfocus --web              # Browser-based cost dashboard (localhost web UI)
finfocus --mcp              # Serve as an MCP server over stdio (alias: mcp-server)
finfocus overview           # Unified cost dashboard (alias: ov)
finfocus cost               # Cost commands
finfocus cost projected     # Estimate costs from plan
finfocus cost actual        # Get actual historical costs
finfocus cost estimate      # What-if cost analysis
finfocus cost recommendations          # Get cost optimization recommendations
finfocus cost recommendations dismiss  # Dismiss a recommendation
finfocus cost recommendations snooze   # Snooze a recommendation
finfocus cost recommendations undismiss # Re-enable a dismissed recommendation
finfocus cost recommendations history  # View recommendation lifecycle history
finfocus config             # Configuration commands
finfocus config init        # Initialize configuration file with defaults
finfocus config set         # Set a configuration value
finfocus config get         # Get a configuration value
finfocus config list        # List all configuration values
finfocus config validate    # Validate routing configuration
finfocus config routes      # Routing inspection commands
finfocus config routes list # Show effective routing rules
finfocus config routes test # Simulate plugin selection for a resource type
finfocus plugin             # Plugin commands
finfocus plugin init        # Initialize a new plugin
finfocus plugin install     # Install a plugin
finfocus plugin update      # Update a plugin
finfocus plugin upgrade     # Upgrade a plugin project to a newer finfocus-spec
finfocus plugin remove      # Remove a plugin
finfocus plugin list        # List the latest version of each installed plugin
finfocus plugin inspect     # Inspect plugin capabilities
finfocus plugin validate    # Validate plugin setup
finfocus plugin conformance # Run conformance tests
finfocus plugin certify     # Run certification tests
finfocus analyzer           # Analyzer commands
finfocus analyzer install   # Install the Pulumi analyzer plugin
finfocus analyzer uninstall # Uninstall the Pulumi analyzer plugin
finfocus analyzer serve     # Start the analyzer gRPC server
```

## Root Command Entry Points

`finfocus` accepts root-local modes `--mcp`, `--web`, and
`--import-cloud-history`. These modes are mutually exclusive.
They are defined on the root command only: no subcommand inherits them, so
`finfocus cost actual --web` or `finfocus overview --web` fail with
`unknown flag`. `--web` and `--mcp` are mutually exclusive.

`--web` serves the browser-based cost dashboard on localhost (127.0.0.1) and
prints a session URL of the form `http://127.0.0.1:<port>/?token=<hex>`. Run
it from a Pulumi or Terraform-compatible project directory. With no source
flags, Pulumi projects load their stack and state; Terraform projects generate
a plan using OpenTofu (`tofu`). Both use the same overview and pricing engine.
Saved `--terraform-state` and `--terraform-plan` inputs are available on the
`overview` CLI command; see [overview](#overview).
`--project-dir` keeps its existing meaning (config resolution) and does not
change where project detection looks.

### Usage (--web)

```bash
finfocus --web [options]
```

### Options (--web)

All of these are root-local and only valid together with `--web`; used alone
they fail with an error naming `--web` and exit code 2. Combining `--web` with
`--mcp` also exits 2.

| Flag             | Description                                                        | Default           |
| ---------------- | ------------------------------------------------------------------ | ----------------- |
| `--web`          | Serve the web UI on localhost and open it in a browser             | Off               |
| `--port`         | Pin the web UI port (0 = kernel-assigned, actual port printed)     | `0`               |
| `--no-browser`   | Do not open a browser; print the URL only                          | Off               |
| `--pulumi-state` | Path to Pulumi state JSON (skips auto-detection)                   | Auto-detected     |
| `--pulumi-json`  | Preview JSON; state is auto-detected unless `--pulumi-state` is set | Auto-detected     |
| `--stack`        | Pulumi stack name for auto-detection                               | Current stack     |
| `--from`         | Start date (YYYY-MM-DD or RFC3339)                                 | 1st of current month |
| `--to`           | End date (YYYY-MM-DD or RFC3339)                                   | Now               |
| `--adapter`      | Restrict to a specific adapter plugin                              | All plugins       |
| `--filter`       | Resource filters (`type=`, `status=`, `provider=`), repeatable     | None              |

### Examples (--web)

```bash
# Auto-detect the current Pulumi project and open the dashboard
finfocus --web

# Serve on a pinned port without opening a browser (e.g. in SSH sessions)
finfocus --web --port 8484 --no-browser

# Use pre-exported files instead of running Pulumi CLI commands
finfocus --web --pulumi-state state.json --pulumi-json plan.json
```

For tasks, see [Use the browser dashboard](../guides/web-ui.md). Views,
controls, exit codes, and HTTP behavior are in the
[Web UI reference](web-ui.md); the security model is explained in
[Web UI design](../architecture/web-ui.md).

For `--mcp`, see the [MCP guide](../guides/mcp.md).

## overview

Display a unified cost dashboard combining Pulumi state and plan data with actual
costs, projected costs, drift analysis, and recommendations.

When run inside a Pulumi project directory without explicit file flags, `finfocus overview`
auto-detects the project and current stack, then runs `pulumi stack export` and
`pulumi preview --json` automatically. Running `finfocus` with no arguments has the
same effect when a Pulumi project is detected.

**Alias:** `ov`

Plain table output prints a pre-flight line with the resource count, any pending
changes, and the plugin count. On a terminal, without `--yes`, overview then
asks `Continue? [Y/n]` before pricing. Enter continues. `n` or `no` prints
`Cancelled.` and exits successfully. Piped input and `--yes` skip the question.

### Usage (overview)

```bash
finfocus overview [options]
finfocus ov [options]
finfocus                    # same as overview when inside a Pulumi project
```

### Options (overview)

| Flag               | Description                                                             | Default               |
| ------------------ | ----------------------------------------------------------------------- | --------------------- |
| `--pulumi-state`   | Path to Pulumi state JSON (skips auto-detection)                        | Auto-detected         |
| `--pulumi-json`    | Path to Pulumi preview JSON (skips auto-detection)                      | Auto-detected         |
| `--stack`          | Pulumi stack name for auto-detection (ignored with `--pulumi-state`/`--pulumi-json`) | Current stack |
| `--from`           | Start date (YYYY-MM-DD or RFC3339)                                      | 1st of current month  |
| `--to`             | End date (YYYY-MM-DD or RFC3339)                                        | Now                   |
| `--adapter`        | Restrict to a specific adapter plugin                                   | All plugins           |
| `--output`         | Output format: `table`, `json`, `ndjson`                                | `table`               |
| `--filter`         | Resource filters, repeatable                                            | -                     |
| `--plain`          | Force non-interactive plain text output                                 | false                 |
| `--force-color`    | Force styled output when stdout is not a terminal                       | false                 |
| `--no-color`       | Plain output. Wins over `--force-color` and `--high-contrast`           | false                 |
| `--color`          | Force colored output. Same switch as `--force-color`                    | false                 |
| `--high-contrast`  | Brighter budget colors (ANSI 46, 226, 196, and 231)                     | false                 |
| `--yes`, `-y`      | Skip confirmation prompts                                               | false                 |
| `--no-pagination`  | Disable pagination (plain mode only)                                    | false                 |

### Examples (overview)

```bash
# Auto-detect from current Pulumi project (recommended)
finfocus overview

# Same — bare invocation inside a Pulumi project
finfocus

# Select a non-default stack
finfocus overview --stack production

# Use pre-exported files
finfocus overview --pulumi-state state.json --pulumi-json plan.json

# CI/CD: non-interactive JSON output
finfocus overview --output json --yes

# Filter to a single provider
finfocus overview --filter provider=aws --plain --yes

# Custom date range
finfocus overview --from 2026-01-01 --to 2026-01-31 --plain --yes
```

## cost projected

Calculate estimated costs from a Pulumi plan. When `--pulumi-json` is omitted,
FinFocus auto-detects the Pulumi project and loads its state and preview together.
A saved preview can also use the state from a detected project or an explicit
`--pulumi-state` file.

The **plan total** prices resources listed in the preview. It remains the value
used for budgets, exit codes, `finfocus.summary.totalMonthly`, and
`finfocus.diff.totalAfter`. The **stack total** adds unchanged resources from
state that the preview omits, counting each resource once. Table output lists
these in an `UNCHANGED RESOURCES (not in the plan total)` section and prints a
separate `Stack total:` line.

A default `pulumi preview --json` omits unchanged resources. When it contains
no priceable resources, JSON sets `finfocus.emptyPlan` to `true`, even when
readable state supplies a stack total. With complete state, table output prints
`No changes. Projected monthly cost is zero.` followed by the stack total.
Without complete state, stderr warns that the plan's `$0` is not an estimate.
A result emptied by `--filter` is not flagged.

### Pulumi JSON fields

The `finfocus.stack` object appears for Pulumi sources. Terraform and OpenTofu
output does not include it. Existing `finfocus.resources[]` entries and summary
breakdowns continue to describe the plan total.

| JSON path | Meaning |
| --- | --- |
| `finfocus.summary.totalMonthly`, `finfocus.diff.totalAfter` | Plan total; budget evaluation uses this value. |
| `finfocus.emptyPlan` | True when the plan contains no priceable resources. |
| `finfocus.stack.totalMonthly`, `finfocus.stack.currency` | Stack total and currency. |
| `finfocus.stack.resources` | Number of resources covered by the stack total. |
| `finfocus.stack.complete` | Whether stack state was read successfully. |
| `finfocus.stack.incompleteReason` | When incomplete: `not_requested`, `backend_unreachable`, `passphrase_required`, or `read_failed`. Omitted when complete. |
| `finfocus.stack.omittedUnchanged` | Number of unchanged resources omitted by the preview's change summary. Omitted when zero. |
| `finfocus.stack.unchanged[]` | State-added resources, with `resourceType`, `resourceId`, `monthly`, and `currency`; an unpriced resource can carry `error`. Omitted when empty. |

When state is unavailable, the stack total equals the plan total,
`stack.complete` is false, and `stack.unchanged` is omitted. Notices on stderr
report safe reason categories without backend details. For a saved preview
without state, a notice reports any known omitted count; use
`pulumi preview --json --show-sames` or `finfocus overview` to see those resources.
A preview that disagrees with state produces a warning and takes precedence.
A saved preview of a different stack or project than the surrounding project is
priced on its own, as it would be outside a project, with `not_requested`.

NDJSON keeps one line per plan resource and adds no stack lines. Stack notices
remain on stderr. When the plan has nothing to price and the stack total is
complete, stderr also carries the same "No changes" summary the table prints.

### Usage (cost projected)

```bash
finfocus cost projected [options]
```

### Options (cost projected)

| Flag            | Description                                                       | Default  |
| --------------- | ----------------------------------------------------------------- | -------- |
| `--pulumi-json` | Path to Pulumi preview JSON (optional; auto-detected if omitted)  |          |
| `--pulumi-state` | Path to Pulumi state JSON; combines with `--pulumi-json` or loads deployed resources alone | |
| `--terraform-plan` | Path to `tofu show -json` output. Does not run a binary. Mutually exclusive with `--pulumi-json` and `--terraform-state` | |
| `--terraform-state` | Path to a Terraform state file (mutually exclusive with `--pulumi-json` and `--terraform-plan`) |          |
| `--stack`       | Pulumi stack name for detected project state and preview |          |
| `--filter`      | Filter resources (tag:key=value, type=\*)                         | None     |
| `--output`      | Output format: table, json, ndjson                                | table    |
| `--utilization` | Assumed resource utilization (0.0-1.0)                            | 1.0      |
| `--show-breakdown` | Per-component cost sub-rows in table output. Ignored for json and ndjson | false |
| `--help`        | Show help                                                         |          |

### Examples (cost projected)

```bash
# Auto-detect from Pulumi project
finfocus cost projected

# Specific stack
finfocus cost projected --stack production

# Explicit file (existing behavior)
finfocus cost projected --pulumi-json plan.json

# JSON output
finfocus cost projected --pulumi-json plan.json --output json

# Filter by type
finfocus cost projected --pulumi-json plan.json --filter "type=aws:ec2*"

# NDJSON for pipelines
finfocus cost projected --pulumi-json plan.json --output ndjson
```

### Terraform plan

FinFocus invokes only the MPL-2.0 OpenTofu `tofu` executable. The
`--terraform-plan` and `--terraform-state` names describe compatible input
formats; FinFocus does not bundle or invoke Terraform core.

In a directory whose nearer marker is a `.tf` or `.tf.json` file, `cost projected`
runs `tofu plan` and prices
`tofu show -json`. A Pulumi project in the same directory still wins. A
Pulumi project only in a parent loses to Terraform files in the working
directory. A plan with no creates, updates, deletes, or replaces prints
`No changes. Projected monthly cost is zero.` and exits 0.

`--terraform-plan` reads a saved `show -json` file and does not start a binary.
A missing file, a `format_version` whose major is not 1, or a file over 32 MiB
is a user-input error (exit 2).

Operational failures exit 1 and print one hint:

| Situation | Hint |
| --- | --- |
| `tofu` is not on `PATH` | Install OpenTofu and run the command again. |
| The directory is not initialized | Run `tofu init` here. FinFocus does not run init. |
| Credentials or the backend cannot be read | Fix the access Terraform already uses. FinFocus does not log in. |
| Any other plan failure | `OpenTofu plan failed.` plus the last 20 stderr lines. |

`cost actual` does not plan. In a Terraform directory with no input file it
tells you to pass `--terraform-state` and exits 2. `cost history collect` does
not plan either: checkpoint collection is for a Pulumi project, and it exits 2.
`--terraform-state` is unchanged.

### Terraform state

`--terraform-state` parses a Terraform state v4 file (`terraform.tfstate`) and
prices every managed resource as-is — the state represents current
infrastructure, so there are no create/update/delete deltas. The cost diff
reports every resource as unchanged, with the same price before and after.

Caveats:

- Resource IDs use the Terraform address format (e.g.
  `module.networking.aws_instance.web[0]`). These are unique within a single
  state file, not globally across state files.
- Type resolution to Pulumi tokens requires a plugin advertising the
  `resolve_resource_types` capability. Without it, FinFocus falls back to raw
  Terraform types and mechanical property conversion; resources that cannot be
  priced show `$0.00 / no cost data`. When no loaded plugin advertises the
  capability (it requires a plugin built on finfocus-spec v0.6.1 or later),
  FinFocus prints one warning to stderr and still exits 0.
- Encrypted state (e.g. OpenTofu state encryption) is not decrypted. Run
  `tofu state pull` (or the Terraform equivalent) to obtain plaintext state
  first.
- Deposed (create-before-destroy leftover) and tainted instances are excluded.

## Pulumi Cloud History Import

```bash
finfocus --import-cloud-history [--from <date>] [--to <date>] [--stack <name>]
finfocus --import-cloud-history --format json
```

This root flag imports historical resource identities without calculating costs
or starting plugins. It requires `cost.history.pulumi_cloud.enabled` set to
`auto` or `true` and an enabled resource history store. It cannot be combined
with `--web`, `--mcp`, or a subcommand. Root `--from` and `--to` require the
import flag; dates accept `YYYY-MM-DD` or RFC3339. `--to` defaults to now.
`--from` defaults to the resolved retention cutoff and earlier dates are
clamped to it. Retention uses `FINFOCUS_HISTORY_RETENTION_DAYS`, then
`cost.history.retention_days`, then 90 days.

The human summary reports updates listed, skipped operations, cache hits,
fetched snapshots, recorded identities, and unreadable updates. JSON output
contains `listed`, `skipped`, `fromCache`, `fetched`, `recorded`, `unreadable`,
and `clamped` fields. JSON is also used when `AGENT_MODE` selects machine
output. `--dry-run` validates the flags and range, prints what would be
imported, and makes no request or history write.

| Exit code | Meaning |
| --- | --- |
| 0 | Import completed, including a no-op. |
| 1 | Import unavailable, history locked, or an update unreadable. |
| 2 | Invalid flags, dates, disabled Cloud import, disabled resource history, or a backend that is not Pulumi Cloud. |

### Cloud history in cost actual

`cost actual --verbose` prints the same import counters to stderr. It does not
change stdout. The default `false` mode makes no requests. `auto` falls back
when Cloud history is unavailable; a partial import emits a warning and
incomplete status. `true` exits 1 before totals if import fails, or exits 2
for a backend that is not Pulumi Cloud or disabled resource history.

A saved input (`--pulumi-state`, `--pulumi-json`) whose resource URNs name a
different stack than the selected one gets no Cloud or local history. Its
billing query keeps only its own resources, and stderr says so.

Incomplete cost rows carry
`"historyImport":{"incomplete":true,"unreadableUpdates":N}`. Incomplete JSON
uses `{"results":[],"historyImport":{"incomplete":true,"unreadableUpdates":N}}`,
with the rows in `results`. Incomplete NDJSON ends with a status record of
`{"type":"historyImport","historyImport":{"incomplete":true,"unreadableUpdates":N}}`.
The status appears even with zero rows. Table output includes a footer naming
the unreadable count. Complete runs keep the existing JSON array and NDJSON
row stream, and omit `historyImport`.

See the [Resource History Guide](../guides/resource-history.md#pulumi-cloud-update-history-opt-in)
for credentials, caching, and the observation limit. Overview and web Cost
views read stored history without fetching Cloud snapshots.

## cost history

Store and read projected-cost snapshots for a Pulumi stack or Terraform project.
`collect` needs the Pulumi CLI and a cost plugin. In Terraform directories it exits
2 without planning because Terraform plans have no checkpoint list. `view`, `list`,
`diff`, `prune`, and `export` open the detected project's database without planning.

For Terraform, the default identity hashes the normalized Git origin and relative
project path, or the absolute path when Git or origin is unavailable. The stack
name is `unscoped`. Pass `--include-workspace` to include the workspace, overriding
`cost.terraform.include_workspace` for one run. Use the same setting for projections
and history queries. This flag is ignored for Pulumi. See the
[Resource History Guide](../guides/resource-history.md#terraform-projected-cost-history).

### Usage (cost history collect)

```bash
finfocus cost history collect --stack <name> [options]
```

### Options (cost history collect)

| Flag | Description | Default |
| --- | --- | --- |
| `--stack` | Pulumi stack name. This is the parent `cost` flag | required |
| `--from` | Only checkpoints on or after this date (`YYYY-MM-DD`) | none |
| `--versions` | Keep the newest N successful checkpoints. `0` is no limit | `0` |
| `--parallel` | Concurrent `pulumi stack export` calls | `4` |
| `--skip-destroy` | Skip destroy checkpoints instead of storing `$0` | `false` |

`--versions 1` is the post-deploy pattern. It stores the newest successful
checkpoint and skips a version that is already in the database. The
default `0` backfills every successful checkpoint that is missing.

The database is `~/.finfocus/history/<stack>.history.db`. Slashes in the
stack name become dashes. `FINFOCUS_HOME` changes the root. CI recipes are
in [CI/CD cost tracking](../guides/ci-cd-cost-tracking.md).

### Usage (cost history export)

```bash
finfocus cost history export --stack <name> --format <json|csv|ndjson>
```

### Options (cost history export)

| Flag | Description | Default |
| --- | --- | --- |
| `--stack` | Pulumi stack name. This is the parent `cost` flag | required |
| `--format` | `json`, `csv`, or `ndjson`. This is the root `--format` flag | required |
| `--from` | Include snapshots on or after this date (`YYYY-MM-DD`) | none |
| `--to` | Include snapshots on or before this date | none |
| `--provider` | Keep one provider's monthly cost | none |

`json` is one document with snapshots and annotations. `csv` has a header
and one row per snapshot. `ndjson` is one snapshot per line. `cost projected`
and `cost actual` add a Trend column when that stack's history file exists.
The interactive detail view in `finfocus overview` and `cost actual` reads
that same file when you open a resource.

## cost recommendations

Display cost optimization recommendations from cloud providers.

### Usage (cost recommendations)

```bash
finfocus cost recommendations [--pulumi-json <file> | --terraform-plan <file>] [options]
```

In a detected Terraform directory, omit input flags to run a plan. In a Pulumi
project, omit input flags to load the live inventory from state and preview,
including unchanged deployed resources and preview-only creates. Use `--stack`
to select a Pulumi stack. The four infrastructure input flags are mutually
exclusive. Saved plan files must use format version 1 and be at most 32 MiB;
invalid or missing files exit 2. Planning failures exit 1 with an actionable hint.
FinFocus does not run init, log in, or store credentials.

`dismiss` and `snooze` never plan. Their `--pulumi-json` flag stays optional
for plugin communication, and both reject `--terraform-plan`.

### Options (cost recommendations)

| Flag                  | Description                                                      | Default  |
| --------------------- | ---------------------------------------------------------------- | -------- |
| `--pulumi-json`       | Path to Pulumi preview JSON; optional in a detected Pulumi project | |
| `--terraform-plan`    | Path to saved `tofu show -json` output; runs no binary | |
| `--pulumi-state`      | Path to a Pulumi stack export JSON file | |
| `--terraform-state`   | Path to a Terraform state JSON file; runs no plan | |
| `--include-workspace` | Include the Terraform workspace in history identity for this run; ignored for Pulumi | false |
| `--filter`            | Filter expression (e.g., `action=RIGHTSIZE,TERMINATE`)           | None     |
| `--output`            | Output format: table, json, ndjson                               | table    |
| `--limit`             | Limit number of recommendations                                  | 0 (all)  |
| `--verbose`           | Show all recommendations with full details                       | false    |
| `--include-dismissed` | Show local dismissed and snoozed rows, and ask plugins to include ones they dismissed | false    |
| `--sort`              | Sort expression (e.g., `savings:desc`, or `risk` with scoring)   | None     |
| `--scoring-dry-run`   | Print what would be sent to the scorer plugin and send nothing   | false    |
| `--no-scoring`        | Skip the scoring step for this run                               | false    |
| `--help`              | Show help                                                        |          |

`--filter` also accepts score expressions such as `risk<=0.3`, and `--sort` accepts `risk`, `false_positive`,
`worth_acting`, `priority` and `insufficient_evidence`. These require the optional scoring step, which is off by default.
See the [Recommendation Scoring Guide](../guides/recommendation-scoring.md).

### Subcommands (cost recommendations)

| Subcommand  | Description                                 |
| ----------- | ------------------------------------------- |
| `dismiss`   | Permanently dismiss a recommendation        |
| `snooze`    | Snooze a recommendation until a date        |
| `undismiss` | Re-enable a dismissed recommendation        |
| `history`   | View lifecycle history for a recommendation |

### Examples (cost recommendations)

```bash
# Interactive mode (default)
finfocus cost recommendations --pulumi-json plan.json

# Filter by action type
finfocus cost recommendations --pulumi-json plan.json --filter "action=RIGHTSIZE,TERMINATE"

# JSON output
finfocus cost recommendations --pulumi-json plan.json --output json

# Include dismissed and snoozed recommendations
finfocus cost recommendations --pulumi-json plan.json --include-dismissed
```

## cost recommendations dismiss

Permanently dismiss a recommendation with a reason.

### Usage (cost recommendations dismiss)

```bash
finfocus cost recommendations dismiss <recommendation-id> [options]
```

### Options (cost recommendations dismiss)

| Flag            | Description                                            | Default  |
| --------------- | ------------------------------------------------------ | -------- |
| `-r, --reason`  | Dismissal reason (required)                            | Required |
| `-n, --note`    | Free-text explanation (required for `other` reason)    | None     |
| `-f, --force`   | Skip confirmation prompt                               | false    |
| `--pulumi-json` | Path to Pulumi preview JSON (for plugin communication) | None     |
| `--adapter`     | Use specific adapter plugin                            | None     |

### Valid Reasons

| Reason                 | Description                          |
| ---------------------- | ------------------------------------ |
| `not-applicable`       | Recommendation doesn't apply         |
| `already-implemented`  | Already acted on this recommendation |
| `business-constraint`  | Business requirements prevent action |
| `technical-constraint` | Technical limitations prevent action |
| `deferred`             | Will address later                   |
| `inaccurate`           | Recommendation data is wrong         |
| `other`                | Custom reason (requires `--note`)    |

### Examples (cost recommendations dismiss)

```bash
# Dismiss with a reason
finfocus cost recommendations dismiss rec-123abc \
  --reason business-constraint --pulumi-json plan.json

# Dismiss with a custom note
finfocus cost recommendations dismiss rec-123abc \
  --reason other --note "Intentional oversizing" --pulumi-json plan.json

# Skip confirmation prompt
finfocus cost recommendations dismiss rec-123abc \
  --reason not-applicable --force --pulumi-json plan.json
```

## cost recommendations snooze

Temporarily dismiss a recommendation until a future date.

### Usage (cost recommendations snooze)

```bash
finfocus cost recommendations snooze <recommendation-id> [options]
```

### Options (cost recommendations snooze)

| Flag            | Description                                                                | Default    |
| --------------- | -------------------------------------------------------------------------- | ---------- |
| `--until`       | Snooze until date (required, YYYY-MM-DD or RFC3339; must be in the future) | Required   |
| `-r, --reason`  | Dismissal reason                                                           | `deferred` |
| `-n, --note`    | Free-text explanation                                                      | None       |
| `-f, --force`   | Skip confirmation prompt                                                   | false      |
| `--pulumi-json` | Path to Pulumi preview JSON (for plugin communication)                     | None       |
| `--adapter`     | Use specific adapter plugin                                                | None       |

### Examples (cost recommendations snooze)

```bash
# Snooze until a specific date (replace with a future date)
finfocus cost recommendations snooze rec-456def \
  --until YYYY-MM-DD --pulumi-json plan.json

# Snooze with reason and note (replace with a future date)
finfocus cost recommendations snooze rec-456def \
  --until YYYY-MM-DD --reason deferred \
  --note "Scheduled for Q2 review" --pulumi-json plan.json
```

## cost recommendations undismiss

Re-enable a previously dismissed or snoozed recommendation.

### Usage (cost recommendations undismiss)

```bash
finfocus cost recommendations undismiss <recommendation-id> [options]
```

### Options (cost recommendations undismiss)

| Flag          | Description              | Default |
| ------------- | ------------------------ | ------- |
| `-f, --force` | Skip confirmation prompt | false   |

### Examples (cost recommendations undismiss)

```bash
# Re-enable a dismissed recommendation
finfocus cost recommendations undismiss rec-123abc
```

## cost recommendations history

View the lifecycle history of one recommendation. The list combines the local
dismissal record with lifecycle events from installed plugins. When no plugins
are installed, or a plugin does not provide history, the local record is shown
on its own. The command does not read a Pulumi plan and does not change the
local dismissal file.

### Usage (cost recommendations history)

```bash
finfocus cost recommendations history <recommendation-id> [options]
```

### Options (cost recommendations history)

| Flag       | Description                        | Default |
| ---------- | ---------------------------------- | ------- |
| `--output` | Output format: table, json, ndjson | table   |

The table columns are timestamp, action, reason, note, expires, and actor.
Actor is blank when the event has none. JSON and NDJSON include `actor` only
when it is set.

### Examples (cost recommendations history)

```bash
# View history in table format
finfocus cost recommendations history rec-123abc

# View history as JSON
finfocus cost recommendations history rec-123abc --output json

# View history as NDJSON (one JSON object per line)
finfocus cost recommendations history rec-123abc --output ndjson
```

## cost actual

Get actual historical costs from plugins. When `--pulumi-json` and `--pulumi-state`
are both omitted, FinFocus auto-detects the Pulumi project and runs
`pulumi stack export`.

### Usage (cost actual)

```bash
finfocus cost actual [options]
```

### Options (cost actual)

| Flag                    | Description                                                                 | Default |
| ----------------------- | --------------------------------------------------------------------------- | ------- |
| `--pulumi-json`         | Path to Pulumi preview JSON (mutually exclusive with --pulumi-state)        |         |
| `--pulumi-state`        | Path to Pulumi state JSON from `pulumi stack export`                        |         |
| `--terraform-state`     | Path to a Terraform state file (requires `--from`; mutually exclusive with `--pulumi-json`/`--pulumi-state`) |         |
| `--stack`               | Pulumi stack name for auto-detection (ignored with --pulumi-json/--pulumi-state) |         |
| `--from`                | Start date (YYYY-MM-DD or RFC3339; auto-detected from state if omitted)     |         |
| `--to`                  | End date (YYYY-MM-DD or RFC3339)                                            | Now     |
| `--filter`              | Filter resources (tag:key=value, type=\*)                                   | None    |
| `--group-by`            | Group results (resource, type, provider, daily, monthly)                    |         |
| `--output`              | Output format: table, json, ndjson                                          | table   |
| `--estimate-confidence` | Request confidence and include it in JSON and NDJSON. Also adds the table column | false   |
| `--show-confidence`     | Add a confidence column to table output. Does not change JSON or NDJSON     | false   |
| `--show-breakdown`      | Per-component cost sub-rows in table output. Ignored for json and ndjson    | false   |
| `--help`                | Show help                                                                   |         |

### Confidence Levels

`--estimate-confidence` asks the plugin for a confidence level and keeps that
field in JSON and NDJSON. `--show-confidence` only adds the table column. Either
flag shows the column. Levels:

| Level  | Description                                                     |
| ------ | --------------------------------------------------------------- |
| HIGH   | Real billing data from plugin (AWS Cost Explorer, Kubecost)     |
| MEDIUM | Runtime estimate for Pulumi-created resources                   |
| LOW    | Runtime estimate for imported resources (creation time unknown) |

### Interactive detail (cost actual)

On a terminal, `cost actual` opens an interactive table. Enter opens one
resource. That detail view loads the resource's stored cost history for
`--stack` when you open it. The list does not load it. A failure stays on
the detail view and `r` tries again.

### Examples (cost actual)

```bash
# Auto-detect from Pulumi project (dates auto-detected from state)
finfocus cost actual

# Auto-detect with specific stack
finfocus cost actual --stack production

# Estimate costs from Pulumi state (--from auto-detected from timestamps)
finfocus cost actual --pulumi-state state.json

# Estimate costs from state with explicit date range
finfocus cost actual --pulumi-state state.json --from 2025-01-01 --to 2025-01-31

# Get costs from Pulumi plan
finfocus cost actual --pulumi-json plan.json --from 2025-01-01

# Group by day
finfocus cost actual --pulumi-json plan.json --group-by daily --from 2025-01-01 --to 2025-01-31

# Group by provider
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --group-by provider

# Filter by tag
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --filter "tag:env=prod"

# JSON output
finfocus cost actual --pulumi-json plan.json --from 2025-01-01 --output json

# Show estimate confidence levels (useful for imported resources)
finfocus cost actual --pulumi-state state.json --estimate-confidence
```

## cost estimate

Perform what-if cost analysis on resources without modifying infrastructure code.

### Usage (cost estimate)

```bash
finfocus cost estimate [options]
```

### Modes

The command supports two mutually exclusive modes:

**Single-Resource Mode:**

- Specify `--provider` and `--resource-type` to estimate cost for a single resource
- Use `--property` to specify property overrides (repeatable)

**Plan-Based Mode:**

- Specify `--pulumi-json` for a Pulumi plan, or `--terraform-plan` for saved
  `tofu show -json` output
- In a Terraform directory, omit input flags to run a plan
- In a Pulumi project, omit input flags to load the live inventory from state
  and preview; use `--stack` to select a stack. Unchanged deployed resources
  and preview-only creates are available to `--modify` selectors
- Use `--modify selector:key=value` to select a resource by ID, logical name,
  or type. A Terraform address, such as `aws_instance.web`, is its resource ID

### Options (cost estimate)

| Flag              | Description                              | Default |
| ----------------- | ---------------------------------------- | ------- |
| `--provider`      | Cloud provider (aws, gcp, azure)         |         |
| `--resource-type` | Full type token (e.g., aws:ec2/instance:Instance) |         |
| `--property`      | Property override key=value (repeatable) |         |
| `--pulumi-json`   | Path to Pulumi preview JSON              |         |
| `--terraform-plan` | Path to saved `tofu show -json` output; runs no binary | |
| `--pulumi-state` | Path to a Pulumi stack export JSON file | |
| `--terraform-state` | Path to a Terraform state JSON file; runs no plan | |
| `--include-workspace` | Include the Terraform workspace in history identity for this run; ignored for Pulumi | false |
| `--modify`        | Resource modification resource:key=value |         |
| `--region`        | Region for cost calculation              |         |
| `--interactive`   | Launch interactive TUI mode              | false   |
| `--output`        | Output format: table, json, ndjson       | table   |
| `--adapter`       | Specific plugin adapter to use           |         |
| `--help`          | Show help                                |         |

The four infrastructure input flags are mutually exclusive. Saved plan input
must use format version 1 and be at most 32 MiB; invalid or missing files exit 2.
Planning failures exit 1. Initialize the directory and configure Terraform access
before planning; FinFocus does not perform those steps.

```bash
finfocus cost estimate --terraform-plan tfplan.json \
  --modify 'aws_instance.web:instanceType=m5.large'
```

### Examples (cost estimate)

```bash
# Auto-detect a Pulumi project and select a live resource
finfocus cost estimate --stack dev --modify 'web-server:instanceType=m5.large'

# Single resource estimation - estimate cost of changing instance type
finfocus cost estimate --provider aws \
  --resource-type aws:ec2/instance:Instance \
  --property instanceType=m5.large

# Single resource with region
finfocus cost estimate --provider aws \
  --resource-type aws:ec2/instance:Instance \
  --property instanceType=m5.large --region us-west-2

# Plan-based estimation - modify a specific resource in existing plan
finfocus cost estimate --pulumi-json plan.json \
  --modify "web-server:instanceType=m5.large"

# Plan-based with multiple modifications
finfocus cost estimate --pulumi-json plan.json \
  --modify "web-server:instanceType=m5.large" \
  --modify "api-server:instanceType=c5.xlarge"

# Interactive TUI mode
finfocus cost estimate --interactive

# Interactive mode with plan
finfocus cost estimate --pulumi-json plan.json --interactive

# JSON output for scripting
finfocus cost estimate --provider aws \
  --resource-type aws:ec2/instance:Instance \
  --property instanceType=m5.large --output json
```

### Interactive pricing spec

`cost estimate --interactive` asks each plugin for `GetPricingSpec` while the
TUI loads. A returned billing mode is listed with its rate, pricing tiers,
assumptions, and usage metric hints. Left and right move between modes when
more than one plugin returns one. A missing spec, or a `not_implemented`
billing mode, leaves the estimate editable. The lookup is cached for that
resource type until the command exits.

### Output Example

```text
What-If Cost Analysis
=====================

Resource: ec2:Instance (aws)
ID: estimate-resource

Baseline:  $8.32/mo (USD)
Modified:  $83.22/mo (USD)

Change:    +$74.90/mo

Property Changes:
-----------------
  instanceType: t3.micro -> m5.large (+$74.90/mo)
```

### Interactive Mode

The interactive TUI mode allows you to:

- Navigate through resource properties with arrow keys
- Edit property values inline (press Enter to edit)
- See live cost updates as you modify properties
- Press 'q' or Ctrl+C to exit

## config init

Initialize a configuration file with default values. When run inside a Pulumi
project, creates project-local configuration at `$PROJECT/.finfocus/config.yaml`
along with a `.gitignore` to protect user-specific data. Use `--global` to force
global initialization even inside a project.

### Usage (config init)

```bash
finfocus config init [options]
```

### Options (config init)

| Flag       | Description                                              | Default |
| ---------- | -------------------------------------------------------- | ------- |
| `--global` | Force global config init even inside a Pulumi project   | false   |
| `--force`  | Overwrite existing configuration file                    | false   |

### Examples (config init)

```bash
# Create project-local config (inside a Pulumi project)
finfocus config init

# Create global config
finfocus config init --global

# Overwrite existing config
finfocus config init --force
```

## config set

Set a configuration value using dot notation. Writes the global
`config.hujson` (`~/.finfocus/` unless `FINFOCUS_HOME` or `PULUMI_HOME` is
set), even inside a Pulumi project; edit a project's `.finfocus/config.hujson`
to set a value for that project only. A key that `config set` does not
support, such as `cost.history.enabled`, is rejected with the settable keys in
its section; edit `config.hujson` directly for it.

For sensitive values such as API keys, use environment variables instead of
storing them in config files.

### Usage (config set)

```bash
finfocus config set <key> <value>
```

### Examples (config set)

```bash
# Set output format
finfocus config set output.default_format json

# Set plugin configuration
finfocus config set plugins.aws.region us-west-2

# The dashboard Plugins page edits the same plugins.<name>.<key> settings
# and installs catalog plugins. See the Web UI guide.

# Set logging level
finfocus config set logging.level debug

# For sensitive values, prefer environment variables
export FINFOCUS_PLUGIN_AWS_SECRET_KEY="mysecret"
```

## config get

Get a configuration value using dot notation from `~/.finfocus/config.yaml`.

### Usage (config get)

```bash
finfocus config get <key>
```

### Examples (config get)

```bash
# Get output format
finfocus config get output.default_format

# Get a plugin section
finfocus config get plugins.aws

# Get all plugins
finfocus config get plugins

# Get logging level
finfocus config get logging.level
```

## config list

List all configuration values from `~/.finfocus/config.yaml`.

### Usage (config list)

```bash
finfocus config list [options]
```

### Options (config list)

| Flag         | Description                      | Default |
| ------------ | -------------------------------- | ------- |
| `--format`   | Output format: `yaml` or `json`  | `yaml`  |

### Examples (config list)

```bash
# List all configuration in YAML format (default)
finfocus config list

# List all configuration in JSON format
finfocus config list --as json
```

## config validate

Validate the active configuration file, or the file given by `--file`.

The report covers syntax, budget amount, currency, period, alerts, and exit
codes, plus unknown fields. A close name includes a suggestion. Routing plugin
checks run only after the file itself is valid. Warnings do not fail the
command. `--output json` prints the same report for CI. Cost commands read the
active file in their pre-run and stop before plugin work when it is invalid.

### Usage (config validate)

```bash
finfocus config validate [--file path] [--output text|json] [--verbose]
```

### Options (config validate)

| Flag         | Description                                      | Default |
| ------------ | ------------------------------------------------ | ------- |
| `--file`     | Configuration file to validate                   | active config |
| `--output`   | Report format: `text` or `json`                  | `text`  |
| `--verbose`  | Print loaded settings after a valid text report  | `false` |
| `--help`     | Show help                                        |         |

### Examples (config validate)

```bash
# Validate the active configuration
finfocus config validate

# Validate one file and emit JSON
finfocus config validate --file ./config.hujson --output json

# Text error (plain):
# Configuration Error: config.hujson
# Errors:
#   Error at line 2: budget period must be 'monthly': got "weekly"
#   Path: cost.budgets.global.period
#   Hint: The only supported period is monthly (period: monthly).
```

## config routes list

Display effective plugin routing rules.

### Usage (config routes list)

```bash
finfocus config routes list [--output table|json]
```

### Options (config routes list)

| Flag | Description | Default |
| ---- | ----------- | ------- |
| `--output` | Output format: `table` or `json` | `table` |

### Examples (config routes list)

```bash
# Show routing as a table
finfocus config routes list

# Show routing in JSON
finfocus config routes list --output json
```

## config routes test

Simulate plugin selection for a resource type and view match reasons.

### Usage (config routes test)

```bash
finfocus config routes test <resource-type> [region] [--output table|json]
```

### Arguments (config routes test)

| Argument | Required | Description |
| -------- | -------- | ----------- |
| `resource-type` | Yes | Pulumi type token (for example `aws:ec2:Instance`) |
| `region` | No | Region hint (for example `us-east-1`) |

### Options (config routes test)

| Flag | Description | Default |
| ---- | ----------- | ------- |
| `--output` | Output format: `table` or `json` | `table` |

### Examples (config routes test)

```bash
# Test routing for a type
finfocus config routes test aws:ec2:Instance

# Include region in match context
finfocus config routes test aws:ec2:Instance us-east-1

# JSON output for scripts
finfocus config routes test aws:ec2:Instance --output json
```

## plugin init

Initialize a new FinFocus plugin project.

The scaffold includes a golangci-lint v2 config at `.golangci-lint.yml`.
`make lint` and the generated CI workflow pass `--config .golangci-lint.yml`,
because golangci-lint does not discover that filename on its own.
`local-prefixes` is the plugin module path `github.com/example/<name>`.

After generating the project, `plugin init` installs the
`finfocus-plugin-dev` and `finfocus-plugin-upgrade` agent skills into
`.agents/skills/` and `.claude/skills/` with `npx skills add`, and records
them in `skills-lock.json`. A release build installs the skills from its own
tag, so they match the SDK version it scaffolds; a development build uses
`main`. The step is best-effort: without Node or network access, init still
succeeds and prints the command to run later. `--offline` and `--no-skill`
skip it.

### Usage (plugin init)

```bash
finfocus plugin init <plugin-name> --author <name> --providers <list> [options]
```

### Options (plugin init)

| Flag            | Description                                                  | Default    |
| --------------- | ------------------------------------------------------------ | ---------- |
| `--author`      | Author name for the plugin                                   | (required) |
| `--providers`   | Comma-separated list of cloud providers                      | (required) |
| `--output-dir`  | Output directory for the plugin project                      | `.`        |
| `--force`       | Overwrite existing files if the directory exists             | `false`    |
| `--with-docker` | Generate Docker support files                                | `true`     |
| `--with-docs`   | Generate documentation templates                             | `true`     |
| `--with-health` | Include health endpoint code                                 | `true`     |
| `--minimal`     | Generate minimal scaffold only (overrides `--with-*` flags)  | `false`    |
| `--no-docker`   | Skip Docker files (alias for `--with-docker=false`)          | `false`    |
| `--no-docs`     | Skip documentation (alias for `--with-docs=false`)           | `false`    |
| `--no-health`   | Skip health endpoint (alias for `--with-health=false`)       | `false`    |
| `--docker-only` | Generate only Docker files (for existing projects)           | `false`    |
| `--with-claude-review` | Generate a Claude Code review workflow                | `false`    |
| `--no-skill`    | Do not install the FinFocus plugin agent skills              | `false`    |
| `--help`        | Show help                                                    |            |

### Examples (plugin init)

```bash
# Initialize a new AWS plugin
finfocus plugin init my-aws-plugin --author "Your Name" --providers aws

# Minimal scaffold without Docker, docs, or health endpoint
finfocus plugin init my-plugin --author "Your Name" --providers aws --minimal

# Without Docker support
finfocus plugin init my-plugin --author "Your Name" --providers aws --no-docker
```

## plugin install

Install a FinFocus plugin from a registry or URL.

### Usage (plugin install)

```bash
finfocus plugin install <plugin-name> [--version <version>] [--url <url>] [options]
```

### Options (plugin install)

| Flag        | Description                                        | Default           |
| ----------- | -------------------------------------------------- | ----------------- |
| `--version` | Specify plugin version to install                  | latest            |
| `--url`     | URL to plugin binary (for custom installs)         | (registry lookup) |
| `--force`   | Force overwrite existing plugin installation       | false             |
| `--clean`   | Remove all other versions after successful install | false             |
| `--metadata` | Key=value metadata pairs (e.g., `region=us-west-2`) | (none)            |
| `--no-save` | Don't add plugin to config file                    | false             |
| `--help`    | Show help                                          |                   |

### Examples (plugin install)

```bash
# Install the latest Vantage plugin
finfocus plugin install vantage

# Install a specific version of a plugin
finfocus plugin install opencost --version 0.1.2

# Install and remove all other versions (cleanup disk space)
finfocus plugin install opencost --clean

# Install from a custom URL
finfocus plugin install my-plugin --url https://example.com/my-plugin-0.1.0.tar.gz

# Install with region metadata (selects region-specific binary)
finfocus plugin install aws-public --metadata="region=us-west-2"

# Pin a version with name@version
finfocus plugin install aws-public@v0.2.0
```

Plugins install into `plugins/` under the FinFocus home: `$FINFOCUS_HOME`,
then `$PULUMI_HOME/finfocus`, then `~/.finfocus`. `plugin update` and
`plugin remove` use the same directory, and `--plugin-dir` overrides it.
`finfocus --web` installs catalog plugins into that same directory. The page
asks you to confirm the plugin, version, repository, and security level
first. URL installs stay on this command.

A release can be published a few minutes before its archives are uploaded.
When you do not request a version and the latest release has no assets yet,
the installer prints a warning naming both versions and installs the newest
earlier stable release that has assets. When you request that version
explicitly, the installer does not swap versions on its own. The error says
`release <version> of <plugin> has no assets yet (the upload may still be in progress)`,
and the usual version fallback applies: an interactive prompt, or
`--fallback-to-latest`. Otherwise retry shortly or pin an earlier version.

## plugin update

Update an installed FinFocus plugin.

### Usage (plugin update)

```bash
finfocus plugin update <plugin-name> [options]
```

### Options (plugin update)

| Flag        | Description                                 | Default |
| ----------- | ------------------------------------------- | ------- |
| `--version` | Specify target version (defaults to latest) | latest  |
| `--all`     | Update all installed plugins                | false   |
| `--help`    | Show help                                   |         |

### Examples (plugin update)

```bash
# Update the Vantage plugin to the latest version
finfocus plugin update vantage

# Update all installed plugins
finfocus plugin update --all
```

## plugin upgrade

Upgrade the source of a plugin project to a newer finfocus-spec version. To
update an installed plugin binary, use `plugin update`.

The command reads the finfocus-spec requirement from `go.mod`. It then plans
a hop for each release that needs action from plugin authors, up to the
version this finfocus build uses. Applying the plan rewrites the `go.mod`
requirement and `go` directive, and any `SpecVersion` declaration.

Every other change is listed as a manual step, with a link to that hop's
migration guide. The
[`finfocus-plugin-upgrade` agent skill](https://github.com/rshade/finfocus/tree/main/agent-skills/finfocus-plugin-upgrade)
works through those steps.

Applying needs a clean git working tree. The code edits never use the
network, so run `go mod tidy` afterwards.

After applying, and also when the plugin is already up to date, the command
reinstalls the `finfocus-plugin-dev` and `finfocus-plugin-upgrade` agent
skills at this finfocus release with `npx skills add`. The skill files are
listed under changed files and in the JSON `skill` object. The step is
best-effort: if it fails, the command prints a warning and the command to run
later. `--dry-run` shows that command, and `--no-skill` skips the step.

### Usage (plugin upgrade)

```bash
finfocus plugin upgrade [options]
```

### Options (plugin upgrade)

| Flag            | Description                                       | Default           |
| --------------- | ------------------------------------------------- | ----------------- |
| `--dir`         | Plugin project directory (containing `go.mod`)    | `.`               |
| `--to`          | Target release: a hop version or finfocus's own   | finfocus's own    |
| `--allow-dirty` | Apply even without a clean git working tree       | false             |
| `--no-skill`    | Do not reinstall the plugin agent skills          | false             |
| `--output`      | Output format: `table` or `json`                  | table             |
| `--dry-run`     | Print the plan without changing files (global)    | false             |

### Examples (plugin upgrade)

```bash
# Show the plan for the plugin in the current directory
finfocus plugin upgrade --dry-run

# Apply the automatic edits, then refresh go.sum
finfocus plugin upgrade
go mod tidy

# Stop at a specific version
finfocus plugin upgrade --dir ../my-plugin --to v0.6.0
```

## plugin remove

Remove an installed FinFocus plugin.

### Usage (plugin remove)

```bash
finfocus plugin remove <plugin-name> [options]
```

### Options (plugin remove)

| Flag     | Description                  | Default |
| -------- | ---------------------------- | ------- |
| `--all`  | Remove all installed plugins | false   |
| `--help` | Show help                    |         |

### Examples (plugin remove)

```bash
# Remove the Vantage plugin
finfocus plugin remove vantage

# Remove all installed plugins
finfocus plugin remove --all
```

## plugin list

List installed plugins. The default is one row per plugin: the highest semver,
which is the version core opens. `--all` lists every installed version and
adds an Active column (`yes` for the version core opens). Older versions are
not started unless `--verbose` is also set, so their Providers cell is `-`.
`--output json` uses the same selection. `version` is the install directory
name. `runtimeVersion` is what the plugin reports, or `N/A` when that binary
was not started. Each object has `"active": true` or `"active": false`.
Providers and capabilities are `null` for a version that was not started.
A version directory that is not semver prints a warning and stays out of the
default listing.

### Usage (plugin list)

```bash
finfocus plugin list [options]
```

### Options (plugin list)

| Flag        | Description                                                | Default |
| ----------- | ---------------------------------------------------------- | ------- |
| `--all`     | List every installed version and mark the one core uses   | false   |
| `--verbose` | Show detailed plugin capabilities and providers            | false   |
| `--output`  | Output format: `table` or `json`                           | `table` |
| `--help`    | Show help                                                  |         |

### Examples (plugin list)

```bash
# List the latest version of each plugin
finfocus plugin list

# Output:
# Name        Version  Providers
# ----        -------  ---------
# aws-public  0.2.0    aws
# recorder    0.1.0    test

# List every installed version. Older rows are not started.
finfocus plugin list --all

# Output:
# Name        Version  Active  Providers
# ----        -------  ------  ---------
# aws-public  0.2.0    yes     aws
# aws-public  0.1.9    no      -
# aws-public  0.1.5    no      -
# recorder    0.1.0    yes     test

# Same selection as JSON
finfocus plugin list --all --output json

# List with detailed capabilities (starts every version when combined with --all)
finfocus plugin list --verbose
```

## plugin inspect

Inspect a plugin's capabilities and field mappings.

### Usage (plugin inspect)

```bash
finfocus plugin inspect <plugin-name> <resource-type> [options]
```

### Options (plugin inspect)

| Flag        | Description                       | Default |
| ----------- | --------------------------------- | ------- |
| `--version` | Specify plugin version to inspect | latest  |
| `--json`    | Output in JSON format             | false   |
| `--help`    | Show help                         |         |

### Examples (plugin inspect)

```bash
# Inspect field mappings for AWS EC2 Instance
finfocus plugin inspect aws-public aws:ec2/instance:Instance

# Output:
# Field Mappings:
# FIELD                STATUS     CONDITION
# -------------------- ---------- ------------------------------
# instanceType         MAPPED
# region               MAPPED
# tags                 IGNORED    Not used for pricing

# Inspect specific version
finfocus plugin inspect aws-public aws:ec2/instance:Instance --version v0.1.0

# Output as JSON
finfocus plugin inspect aws-public aws:ec2/instance:Instance --json
```

## plugin validate

Validate plugin installations.

### Usage (plugin validate)

```bash
finfocus plugin validate [options]
```

### Options (plugin validate)

| Flag     | Description |
| -------- | ----------- |
| `--help` | Show help   |

### Examples (plugin validate)

```bash
# Validate all plugins
finfocus plugin validate

# Output:
# vantage (0.1.0): OK
# opencost (0.1.2): OK
```

## plugin conformance

Run conformance tests against a plugin binary to verify protocol compliance.

### Usage (plugin conformance)

```bash
finfocus plugin conformance <plugin-path> [options]
```

### Options (plugin conformance)

| Flag            | Description                                                            | Default |
| --------------- | ---------------------------------------------------------------------- | ------- |
| `--mode`        | Communication mode: tcp, stdio                                         | tcp     |
| `--verbosity`   | Output detail: quiet, normal, verbose, debug                           | normal  |
| `--output`      | Output format: table, json, junit                                      | table   |
| `--output-file` | Write output to file                                                   | stdout  |
| `--timeout`     | Global suite timeout                                                   | 5m      |
| `--category`    | Filter by category (repeatable): protocol, error, performance, context | all     |
| `--filter`      | Regex filter for test names                                            |         |
| `--help`        | Show help                                                              |         |

### Examples (plugin conformance)

```bash
# Basic conformance check
finfocus plugin conformance ./plugins/aws-cost

# Verbose output with JSON
finfocus plugin conformance --verbosity verbose --output json ./plugins/aws-cost

# Filter to protocol tests only
finfocus plugin conformance --category protocol ./plugins/aws-cost

# JUnit XML for CI
finfocus plugin conformance --output junit --output-file report.xml ./plugins/aws-cost

# Use stdio mode
finfocus plugin conformance --mode stdio ./plugins/aws-cost
```

## plugin certify

Run full certification tests and generate a certification report.

### Usage (plugin certify)

```bash
finfocus plugin certify <plugin-path> [options]
```

### Options (plugin certify)

| Flag           | Description                          | Default |
| -------------- | ------------------------------------ | ------- |
| `-o, --output` | Output file for certification report | stdout  |
| `--mode`       | Communication mode: tcp, stdio       | tcp     |
| `--timeout`    | Global certification timeout         | 10m     |
| `--help`       | Show help                            |         |

### Certification Requirements

A plugin is certified if all conformance tests pass:

- All protocol tests (Name, GetProjectedCost, GetActualCost)
- All error handling tests
- All context/timeout tests
- All performance tests

### Examples (plugin certify)

```bash
# Basic certification
finfocus plugin certify ./plugins/aws-cost

# Save report to file
finfocus plugin certify --output certification.md ./plugins/aws-cost

# Use stdio mode
finfocus plugin certify --mode stdio ./plugins/aws-cost

# Output:
# 🔍 Certifying plugin at ./plugins/aws-cost...
# Running conformance tests...
# ✅ CERTIFIED - Plugin passed all conformance tests
```

### Certification Report

The command generates a markdown report containing:

- Plugin name and version
- Certification status (CERTIFIED or FAILED)
- Test summary (total, passed, failed, skipped)
- List of issues (if any failed)

## analyzer serve

Starts the FinFocus analyzer gRPC server. This command is intended to be run by
the Pulumi CLI as part of the `pulumi preview` workflow, typically configured in
`Pulumi.yaml`.

### Usage (analyzer serve)

```bash
finfocus analyzer serve [options]
```

### Options (analyzer serve)

| Flag              | Description                                  | Default     |
| ----------------- | -------------------------------------------- | ----------- |
| `--logtostderr`   | Log messages to stderr rather than log files | false       |
| `--v`             | Log level for V-logging (verbose logging)    | 0           |
| `--pulumilogfile` | Pulumi log file name (internal use)          | (generated) |
| `--help`          | Show help                                    |             |

### Examples (analyzer serve)

```bash
# This command is typically not run directly by users.
# It's configured in Pulumi.yaml for zero-click cost estimation:
#
# plugins:
#   - path: finfocus
#     args: ["analyzer", "serve"]
```

## analyzer install

Install the finfocus binary as a Pulumi Analyzer plugin. This replaces the
manual process of creating the plugin directory, copying the binary, and
setting permissions.

After installation, the binary on PATH is required for Pulumi to find it:

```bash
export PATH="${HOME}/.pulumi/plugins/analyzer-finfocus-v<version>:${PATH}"
```

### Usage (analyzer install)

```bash
finfocus analyzer install [options]
```

### Options (analyzer install)

| Flag            | Description                          | Default                   |
| --------------- | ------------------------------------ | ------------------------- |
| `--force`       | Overwrite existing installation      | false                     |
| `--target-dir`  | Override Pulumi plugin directory     | `~/.pulumi/plugins/`      |

### Examples (analyzer install)

```bash
# Install the analyzer
finfocus analyzer install

# Force reinstall after upgrading finfocus
finfocus analyzer install --force

# Install to a custom directory
finfocus analyzer install --target-dir /opt/pulumi/plugins
```

## analyzer uninstall

Remove all installed versions of the finfocus Pulumi Analyzer plugin.
All `analyzer-finfocus-v*` directories are deleted from the plugin directory.

### Usage (analyzer uninstall)

```bash
finfocus analyzer uninstall [options]
```

### Options (analyzer uninstall)

| Flag            | Description                       | Default              |
| --------------- | --------------------------------- | -------------------- |
| `--target-dir`  | Override Pulumi plugin directory  | `~/.pulumi/plugins/` |

### Examples (analyzer uninstall)

```bash
# Uninstall the analyzer
finfocus analyzer uninstall

# Uninstall from a custom directory
finfocus analyzer uninstall --target-dir /opt/pulumi/plugins
```

## Global Options

```bash
finfocus [global options] command [command options]
```

| Option                 | Description                                  |
| ---------------------- | -------------------------------------------- |
| `--help`               | Show help                                    |
| `--version`            | Show version                                 |
| `--debug`              | Enable debug logging                         |
| `--verbose`            | Enable verbose output                        |
| `--no-color`           | Plain text on overview and cost output commands |
| `--plain`              | Plain text on overview and cost output commands |
| `--color`              | Force color on overview and cost output commands |
| `--high-contrast`      | Brighter budget colors on styled output      |
| `--skip-version-check` | Skip plugin spec version compatibility check |

## Date Formats

### Accepted Formats

```bash
# ISO 8601 (YYYY-MM-DD)
finfocus cost actual --from 2024-01-01

# RFC3339 (full timestamp)
finfocus cost actual --from 2024-01-01T00:00:00Z

# Relative (future)
finfocus cost actual --from "7 days ago"
```

## Output Formats

### Table (Default)

Human-readable table format:

```text
RESOURCE    TYPE       MONTHLY   CURRENCY
Instance1   ec2        $7.50     USD
Bucket1     s3         $0.50     USD
──────────────────────────────
Total                  $8.00     USD
```

### JSON

Machine-readable JSON format:

```json
{
  "summary": { "totalMonthly": 8.0, "currency": "USD" },
  "resources": [{ "name": "Instance1", "type": "ec2", "cost": 7.5 }]
}
```

### NDJSON

Newline-delimited JSON (one per line):

```text
{"name":"Instance1","type":"ec2","cost":7.50}
{"name":"Bucket1","type":"s3","cost":0.50}
```

## Exit Codes

| Code | Meaning                                                       |
| ---- | ------------------------------------------------------------- |
| 0    | Success                                                       |
| 1    | General error                                                 |
| 2    | Invalid input (`error_code: validation_error` in JSON output) |

Exit code 2 covers invalid flag combinations, unreadable or malformed
`--terraform-state`, `--pulumi-json` and `--pulumi-state` input (missing file,
directory, empty file, bad JSON, unsupported or encrypted state), invalid
filters, and unparseable date ranges for `cost projected` and `cost actual`.
Earlier releases reported these as exit code 1 with
`error_code: internal_error`.

---

See [User Guide](../guides/user-guide.md) for workflow examples.

## Historical carbon reports

`finfocus carbon actual` queries available monthly aggregate emissions from
`aws-ce`. Enable `plugins.aws-ce.carbon_enabled` in configuration or set
`FINFOCUS_AWS_CARBON_ENABLED=true`, then restart the plugin.

```sh
finfocus carbon actual --start 2026-01-01 --end 2026-03-01 \
  --method market-based --scope total --output json
```

| Flag | Values |
| --- | --- |
| `--start` | Required inclusive UTC month boundary, `YYYY-MM-DD` |
| `--end` | Required exclusive UTC month boundary, `YYYY-MM-DD` |
| `--method` | `market-based` (default), `location-based` |
| `--scope` | `total` (default), `1`, `2`, `3` |
| `--account` | Account IDs, comma-separated or repeated |
| `--region` | AWS regions, comma-separated or repeated |
| `--service` | AWS service names, comma-separated or repeated |
| `--output` | `table` (default), `json` |

The total covers returned records only. Missing reports have no numeric total;
explicit zero values remain zero. See [historical AWS carbon reporting](../../plugins/historical-carbon/)
for provenance, permissions, pagination behavior, and authenticated web parity.
