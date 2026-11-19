---
title: CLI Commands Reference
description: FinFocus CLI commands, behavior notes, and links to the generated reference for every command
layout: docs
---

Every command has a reference page generated from its help text: usage,
flags, inherited flags, and examples. Start at [`finfocus`](./cli/finfocus.md).
This page lists the commands and keeps the behavior notes that the help text
does not cover. Task guides are under Commands, for example the
[overview command](../commands/overview.md).

## Commands Overview

| Group | Commands |
| --- | --- |
| Root | [`finfocus`](./cli/finfocus.md) (`--web`, `--mcp`, `--import-cloud-history`) |
| Overview | [`overview`](./cli/finfocus_overview.md) |
| Cost | [`cost`](./cli/finfocus_cost.md): [`projected`](./cli/finfocus_cost_projected.md), [`actual`](./cli/finfocus_cost_actual.md), [`estimate`](./cli/finfocus_cost_estimate.md), [`forecast`](./cli/finfocus_cost_forecast.md), [`anomalies`](./cli/finfocus_cost_anomalies.md), [`cluster`](./cli/finfocus_cost_cluster.md) |
| Recommendations | [`cost recommendations`](./cli/finfocus_cost_recommendations.md): [`dismiss`](./cli/finfocus_cost_recommendations_dismiss.md), [`snooze`](./cli/finfocus_cost_recommendations_snooze.md), [`undismiss`](./cli/finfocus_cost_recommendations_undismiss.md), [`history`](./cli/finfocus_cost_recommendations_history.md) |
| Cost history | [`cost history`](./cli/finfocus_cost_history.md): [`collect`](./cli/finfocus_cost_history_collect.md), [`collect-daily`](./cli/finfocus_cost_history_collect-daily.md), [`view`](./cli/finfocus_cost_history_view.md), [`list`](./cli/finfocus_cost_history_list.md), [`diff`](./cli/finfocus_cost_history_diff.md), [`export`](./cli/finfocus_cost_history_export.md), [`prune`](./cli/finfocus_cost_history_prune.md) |
| Carbon | [`carbon`](./cli/finfocus_carbon.md): [`actual`](./cli/finfocus_carbon_actual.md) |
| Configuration | [`config`](./cli/finfocus_config.md): [`init`](./cli/finfocus_config_init.md), [`set`](./cli/finfocus_config_set.md), [`get`](./cli/finfocus_config_get.md), [`list`](./cli/finfocus_config_list.md), [`validate`](./cli/finfocus_config_validate.md), [`routes`](./cli/finfocus_config_routes.md), [`routes list`](./cli/finfocus_config_routes_list.md), [`routes test`](./cli/finfocus_config_routes_test.md) |
| Plugins | [`plugin`](./cli/finfocus_plugin.md): [`init`](./cli/finfocus_plugin_init.md), [`install`](./cli/finfocus_plugin_install.md), [`update`](./cli/finfocus_plugin_update.md), [`upgrade`](./cli/finfocus_plugin_upgrade.md), [`remove`](./cli/finfocus_plugin_remove.md), [`list`](./cli/finfocus_plugin_list.md), [`inspect`](./cli/finfocus_plugin_inspect.md), [`validate`](./cli/finfocus_plugin_validate.md), [`conformance`](./cli/finfocus_plugin_conformance.md), [`certify`](./cli/finfocus_plugin_certify.md) |
| Analyzer | [`analyzer`](./cli/finfocus_analyzer.md): [`install`](./cli/finfocus_analyzer_install.md), [`uninstall`](./cli/finfocus_analyzer_uninstall.md), [`check`](./cli/finfocus_analyzer_check.md), [`serve`](./cli/finfocus_analyzer_serve.md) |
| Other | [`logs`](./cli/finfocus_logs.md), [`setup`](./cli/finfocus_setup.md) |
| Agents | [`mcp-server`](./cli/finfocus_mcp-server.md), [`__schema`](./cli/finfocus___schema.md) |
| Shell completion | [`completion`](./cli/finfocus_completion.md): [`bash`](./cli/finfocus_completion_bash.md), [`fish`](./cli/finfocus_completion_fish.md), [`powershell`](./cli/finfocus_completion_powershell.md), [`zsh`](./cli/finfocus_completion_zsh.md) |

The generated pages cover every command that `finfocus --help` lists. They
include the commands and flags that ax-go adds when the CLI starts:
`mcp-server`, `__schema`, Cobra's `completion`, and the persistent `--format`,
`--dry-run`, `--yes`, `--strict`, and `--idempotency-key` flags. The [`finfocus`](./cli/finfocus.md) page lists
those flags under "Options", and every other page under "Options inherited
from parent commands". Hidden commands are left out.

## Root Command Entry Points

`finfocus` accepts root-local modes `--mcp`, `--web`, and
`--import-cloud-history`. These modes are mutually exclusive.
They are defined on the root command only: no subcommand inherits them, so
`finfocus cost actual --web` or `finfocus overview --web` fail with
`unknown flag`.

`--web` serves the browser-based cost dashboard on localhost (127.0.0.1) and
prints a session URL of the form `http://127.0.0.1:<port>/?token=<hex>`. Run
it from a Pulumi or Terraform-compatible project directory. With no source
flags, Pulumi projects load their stack and state; Terraform projects generate
a plan using OpenTofu (`tofu`). Both use the same overview and pricing engine.
Saved `--terraform-state` and `--terraform-plan` inputs are available on the
`overview` CLI command; see [overview](#overview).
`--project-dir` keeps its existing meaning (config resolution) and does not
change where project detection looks.

### Options (--web)

The `--web` flags are listed on the [`finfocus`](./cli/finfocus.md) page,
where their help says `only valid with --web`. They are root-local. Used
without `--web` (or `--import-cloud-history`, for `--from`, `--to`, and
`--stack`), they fail with an error and exit code 2. Combining `--web` with
`--mcp` also exits 2. A `--stack` value with commas, or a repeated `--stack`,
opens one tab per stack (Pulumi projects only).

For tasks, see [Use the browser dashboard](../guides/web-ui.md). Views,
controls, exit codes, and HTTP behavior are in the
[Web UI reference](web-ui.md); the security model is explained in
[Web UI design](../architecture/web-ui.md).

For `--mcp`, see the [MCP guide](../guides/mcp.md).

## overview

Display a unified cost dashboard combining Pulumi state and plan data with actual
costs, projected costs, drift analysis, and recommendations. Flags and
examples: [`finfocus overview`](./cli/finfocus_overview.md). Task guide:
[overview command](../commands/overview.md).

When run inside a Pulumi project directory without explicit file flags, `finfocus overview`
auto-detects the project and current stack, then runs `pulumi stack export` and
`pulumi preview --json` automatically. Running `finfocus` with no arguments has the
same effect when a Pulumi project is detected.

**Alias:** `ov`

Plain table output prints a pre-flight line with the resource count, any pending
changes, and the plugin count. On a terminal, without `--yes`, overview then
asks `Continue? [Y/n]` before pricing. Enter continues. `n` or `no` prints
`Cancelled.` and exits successfully. Piped input and `--yes` skip the question.

## cost projected

Calculate estimated costs from a Pulumi plan. Flags and examples:
[`finfocus cost projected`](./cli/finfocus_cost_projected.md). When
`--pulumi-json` is omitted, FinFocus auto-detects the Pulumi project and loads
its state and preview together. A saved preview can also use the state from a
detected project or an explicit `--pulumi-state` file.

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

With `--output json` or `--output ndjson` (or `--format json`), stdout carries
only the command's usual document with no resources. The sentence moves to
stderr, and the exit code stays 0. Nothing is priced for a plan with no
changes, whether it was planned automatically or read with `--terraform-plan`.

| Command | JSON on stdout | NDJSON on stdout |
| --- | --- | --- |
| `cost projected` | `finfocus` document with `emptyPlan: true`, `resources: []`, and zero `summary` and `diff` totals; no `stack` object | No lines |
| `cost recommendations` | Zero `summary`, `recommendations: []` | One `summary` line |
| `cost forecast` | `currency`, `months`, and `series: []` | One `meta` line |
| `cost estimate` | `results: []`, `totalChange: 0` | No lines |
| `overview` | `metadata` with `hasChanges: false`, `resources: []`, zero `summary` | No lines |

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
Flags and examples: [`finfocus cost history`](./cli/finfocus_cost_history.md)
and its subcommands.
`collect` needs the Pulumi CLI and a cost plugin. In Terraform directories it exits
2 without planning because Terraform plans have no checkpoint list. `view`, `list`,
`diff`, `prune`, and `export` open the detected project's database without planning.

For Terraform, the default identity hashes the normalized Git origin and relative
project path, or the absolute path when Git or origin is unavailable. The stack
name is `unscoped`. Pass `--include-workspace` to include the workspace, overriding
`cost.terraform.include_workspace` for one run. Use the same setting for projections
and history queries. This flag is ignored for Pulumi. See the
[Resource History Guide](../guides/resource-history.md#terraform-projected-cost-history).

### cost history collect

`--versions 1` is the post-deploy pattern. It stores the newest successful
checkpoint and skips a version that is already in the database. The
default `0` backfills every successful checkpoint that is missing.

The database is `<project>@<stack>.history.db`. Its directory is
`FINFOCUS_HISTORY_DIR`, then `cost.history.directory`, then `history/` under
the FinFocus home (`$FINFOCUS_HOME`, then `$PULUMI_HOME/finfocus`, then
`~/.finfocus`). CI recipes are in
[CI/CD cost tracking](../guides/ci-cd-cost-tracking.md).

### cost history collect-daily

The file is `<project>@<stack>.daily.db` in the same history directory. A
re-run replaces days the plugin sent and leaves every other day in place. A
missing daily series stores nothing for that resource. The command stores only
what plugins report: it never estimates a day from state, so a reported $0 day
is stored as $0 and a day the plugin did not send stays empty. A resource that
every plugin declines counts as skipped. A bucket with no currency is stored as
USD; a named currency is its own series. With `--dry-run` the command prices the
stack and reports what it would store, but writes and prunes nothing. A bucket
whose stated charge period spans more than one UTC day, or starts on another day
than its timestamp, is not a daily amount and is left out; resources sharing a
URN are summed per day. An input whose URNs name another stack or project than
the target exits 2 before anything is priced or written. The command is not an
MCP tool. With `cost.history.retention.auto_prune` and `max_age_days` set,
points older than that many days are deleted.
`max_snapshots` does not apply to this file.

### cost history export

`--format` is the root `--format` flag; `export` also accepts `csv` and
`ndjson` there. `json` is one document with snapshots and annotations. `csv`
has a header and one row per snapshot. `ndjson` is one snapshot per line.
`cost projected` and `cost actual` add a Trend column when that stack's history
file exists. The interactive detail view in `finfocus overview` and
`cost actual` reads that same file when you open a resource.

## cost recommendations

Display cost optimization recommendations from cloud providers. Flags and
examples: [`finfocus cost recommendations`](./cli/finfocus_cost_recommendations.md).

In a detected Terraform directory, omit input flags to run a plan. In a Pulumi
project, omit input flags to load the live inventory from state and preview,
including unchanged deployed resources and preview-only creates. Use `--stack`
to select a Pulumi stack. The four infrastructure input flags are mutually
exclusive. Saved plan files must use format version 1 and be at most 32 MiB;
invalid or missing files exit 2. Planning failures exit 1 with an actionable hint.
FinFocus does not run init, log in, or store credentials.

`dismiss` and `snooze` never plan, and both reject `--terraform-plan`. Their
optional `--pulumi-json` flag connects the plugins and looks the recommendation
up in that saved preview, so the dismissal keeps its last-known details:
resource, action, description, savings, and the spot label.
`--include-dismissed` lists a dismissed recommendation with those details after
the plugin stops returning it, and the overview counts it in the Recs column
(`3(-1)`). The lookup is best effort. Without `--pulumi-json`, or when no plugin
returns the id, the recommendation is still dismissed and records no new
details; details saved by an earlier dismissal of the same id are kept.

`--filter` also accepts score expressions such as `risk<=0.3`, and `--sort` accepts `risk`, `false_positive`,
`worth_acting`, `priority` and `insufficient_evidence`. These require the optional scoring step, which is off by default.
See the [Recommendation Scoring Guide](../guides/recommendation-scoring.md).

### Plugin failures (cost recommendations)

A plugin whose recommendations query fails does not stop the command. The
recommendations from the plugins that answered are still listed, and every
output names the failed plugin:

- Table output ends with an `ERRORS` section. With no recommendations at all it
  reads `No recommendations available: 1 plugin failed (see ERRORS below).`
  instead of a bare `No recommendations available.`
- The interactive view lists failed plugins under `PLUGIN ERRORS`, below the
  summary.
- JSON carries `errors`; NDJSON carries `errors` on its first (`summary`) line.
  Both are absent when every plugin answered.

The exit code stays 0, as it does for `cost projected` with plugin errors. A
script that must notice a failed plugin should read `errors` from JSON or NDJSON
output.

## cost recommendations dismiss

Permanently dismiss a recommendation with a reason. Flags and examples:
[`finfocus cost recommendations dismiss`](./cli/finfocus_cost_recommendations_dismiss.md).

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

## cost recommendations snooze

Temporarily dismiss a recommendation until a future date. `--until` takes
`YYYY-MM-DD` or RFC3339 and must be in the future. The reasons are the same as
for `dismiss`. Flags and examples:
[`finfocus cost recommendations snooze`](./cli/finfocus_cost_recommendations_snooze.md).

## cost recommendations undismiss

Re-enable a previously dismissed or snoozed recommendation. Flags and examples:
[`finfocus cost recommendations undismiss`](./cli/finfocus_cost_recommendations_undismiss.md).

## cost recommendations history

View the lifecycle history of one recommendation. The list combines the local
dismissal record with lifecycle events from installed plugins. When no plugins
are installed, or a plugin does not provide history, the local record is shown
on its own. The command does not read a Pulumi plan and does not change the
local dismissal file. Flags and examples:
[`finfocus cost recommendations history`](./cli/finfocus_cost_recommendations_history.md).

The table columns are timestamp, action, reason, note, expires, and actor.
Actor is blank when the event has none. JSON and NDJSON include `actor` only
when it is set.

## cost actual

Get actual historical costs from plugins. Flags and examples:
[`finfocus cost actual`](./cli/finfocus_cost_actual.md). When `--pulumi-json`
and `--pulumi-state` are both omitted, FinFocus auto-detects the Pulumi project
and runs `pulumi stack export`.

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

## cost estimate

Perform what-if cost analysis on resources without modifying infrastructure
code. Flags and examples:
[`finfocus cost estimate`](./cli/finfocus_cost_estimate.md).

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

The four infrastructure input flags are mutually exclusive. Saved plan input
must use format version 1 and be at most 32 MiB; invalid or missing files exit 2.
Planning failures exit 1. Initialize the directory and configure Terraform access
before planning; FinFocus does not perform those steps.

```bash
finfocus cost estimate --terraform-plan tfplan.json \
  --modify 'aws_instance.web:instanceType=m5.large'
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
project, creates project-local configuration at `$PROJECT/.finfocus/config.hujson`
along with a `.gitignore` to protect user-specific data. Use `--global` to force
global initialization even inside a project. Flags and examples:
[`finfocus config init`](./cli/finfocus_config_init.md).

## config set

Set a configuration value using dot notation. Flags and examples:
[`finfocus config set`](./cli/finfocus_config_set.md). Writes the global
`config.hujson` (`~/.finfocus/` unless `FINFOCUS_HOME` or `PULUMI_HOME` is
set), even inside a Pulumi project; edit a project's `.finfocus/config.hujson`
to set a value for that project only. A key that `config set` does not
support, such as `cost.history.enabled`, is rejected with the settable keys in
its section; edit `config.hujson` directly for it.

A project `config.hujson` replaces a whole global section, so a value written
for a key in a section the project defines does not apply in that project. For
example, a project file with a `cost` section for Pulumi Cloud history hides a
global `cost.budgets.global.amount`. `config set` still writes the global file,
prints a warning on stderr that names the project file, and `config get` names
it when it cannot find the key. See
[Config Merge Behavior](config-reference.md#config-merge-behavior).

For sensitive values such as API keys, use environment variables instead of
storing them in config files. Plugin processes inherit the environment FinFocus
runs in, and each plugin's docs page lists the variables it reads.

A `plugins.<name>.<key>` value reaches the plugin through the variable the
plugin advertises for that key. A key the plugin does not advertise is passed
as `FINFOCUS_PLUGIN_<NAME>_<KEY>`, in uppercase with hyphens as underscores:
for example, `plugins.aws-ce.<key>` becomes `FINFOCUS_PLUGIN_AWS_CE_<KEY>`.
The dashboard Plugins page edits the same `plugins.<name>.<key>` settings; see
the [Web UI guide](../guides/web-ui.md).

## config get

Get a configuration value using dot notation from `~/.finfocus/config.hujson`.
Flags and examples: [`finfocus config get`](./cli/finfocus_config_get.md).

## config list

List all configuration values from `~/.finfocus/config.hujson`. Flags and
examples: [`finfocus config list`](./cli/finfocus_config_list.md).

## config validate

Validate the active configuration file, or the file given by `--file`. Flags
and examples: [`finfocus config validate`](./cli/finfocus_config_validate.md).

The report covers syntax, budget amount, currency, period, alerts, and exit
codes, plus unknown fields. A close name includes a suggestion. Routing plugin
checks run only after the file itself is valid. Warnings do not fail the
command. `--output json` prints the same report for CI. Cost commands read the
active file in their pre-run and stop before plugin work when it is invalid.

A text error looks like this:

```text
Configuration Error: config.hujson
Errors:
  Error at line 2: budget period must be 'monthly': got "weekly"
  Path: cost.budgets.global.period
  Hint: The only supported period is monthly (period: monthly).
```

## config routes list

Display effective plugin routing rules. Flags and examples:
[`finfocus config routes list`](./cli/finfocus_config_routes_list.md).

## config routes test

Simulate plugin selection for a resource type and view match reasons. The
arguments are a Pulumi type token, such as `aws:ec2/instance:Instance`, and an optional
region hint, such as `us-east-1`. Flags and examples:
[`finfocus config routes test`](./cli/finfocus_config_routes_test.md).

## plugin init

Initialize a new FinFocus plugin project. Flags and examples:
[`finfocus plugin init`](./cli/finfocus_plugin_init.md).

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

## plugin install

Install a FinFocus plugin from a registry or URL. Flags and examples:
[`finfocus plugin install`](./cli/finfocus_plugin_install.md). Pin a version
with `<plugin-name>@<version>`; there is no `--version` flag.

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

Update an installed FinFocus plugin. Flags and examples:
[`finfocus plugin update`](./cli/finfocus_plugin_update.md).
`plugin update` takes one plugin name; there is no option to update every
installed plugin at once.

## plugin upgrade

Upgrade the source of a plugin project to a newer finfocus-spec version. To
update an installed plugin binary, use `plugin update`. Flags and examples:
[`finfocus plugin upgrade`](./cli/finfocus_plugin_upgrade.md).

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

## plugin remove

Remove an installed FinFocus plugin. Flags and examples:
[`finfocus plugin remove`](./cli/finfocus_plugin_remove.md).
`plugin remove` takes one plugin name. It is also available as
`plugin uninstall` and `plugin rm`.

## plugin list

List installed plugins. Flags and examples:
[`finfocus plugin list`](./cli/finfocus_plugin_list.md). The default is one
row per plugin: the highest semver,
which is the version core opens. `--all` lists every installed version and
adds an Active column (`yes` for the version core opens). Older versions are
not started unless `--verbose` is also set, so their Providers cell is `-`.
`--output json` uses the same selection. `version` is the install directory
name. `runtimeVersion` is what the plugin reports, or `N/A` when that binary
was not started. Each object has `"active": true` or `"active": false`.
Providers and capabilities are `null` for a version that was not started.
A version directory that is not semver prints a warning and stays out of the
default listing.

```text
$ finfocus plugin list --all
Name        Version  Active  Providers
----        -------  ------  ---------
aws-public  0.2.0    yes     aws
aws-public  0.1.9    no      -
aws-public  0.1.5    no      -
recorder    0.1.0    yes     test
```

## plugin inspect

Inspect a plugin's capabilities and field mappings. Flags and examples:
[`finfocus plugin inspect`](./cli/finfocus_plugin_inspect.md).

```text
$ finfocus plugin inspect aws-public aws:ec2/instance:Instance
Field Mappings:
FIELD                STATUS     CONDITION
-------------------- ---------- ------------------------------
instanceType         MAPPED
region               MAPPED
tags                 IGNORED    Not used for pricing
```

## plugin validate

Validate plugin installations. Flags and examples:
[`finfocus plugin validate`](./cli/finfocus_plugin_validate.md).

## plugin conformance

Run conformance tests against a plugin binary to verify protocol compliance.
Flags and examples:
[`finfocus plugin conformance`](./cli/finfocus_plugin_conformance.md).

## plugin certify

Run full certification tests and generate a certification report. Flags and
examples: [`finfocus plugin certify`](./cli/finfocus_plugin_certify.md).

### Certification Requirements

A plugin is certified if all conformance tests pass:

- All protocol tests (Name, GetProjectedCost, GetActualCost)
- All error handling tests
- All context/timeout tests
- All performance tests

### Certification Report

The command generates a markdown report containing:

- Plugin name and version
- Certification status (CERTIFIED or FAILED)
- Test summary (total, passed, failed, skipped)
- List of issues (if any failed)

## analyzer serve

Starts the FinFocus analyzer gRPC server. This command is intended to be run by
the Pulumi CLI as part of the `pulumi preview` workflow, typically configured in
`Pulumi.yaml`. Flags: [`finfocus analyzer serve`](./cli/finfocus_analyzer_serve.md).

When Pulumi starts the binary as `pulumi-analyzer-finfocus`, the arguments it
passes are ignored.

## analyzer install

Install the finfocus binary as a Pulumi Analyzer plugin. This replaces the
manual process of creating the plugin directory, copying the binary, and
setting permissions. Flags and examples:
[`finfocus analyzer install`](./cli/finfocus_analyzer_install.md).

After installation, the binary on PATH is required for Pulumi to find it:

```bash
export PATH="${HOME}/.pulumi/plugins/analyzer-finfocus-v<version>:${PATH}"
```

## analyzer uninstall

Remove all installed versions of the finfocus Pulumi Analyzer plugin.
All `analyzer-finfocus-v*` directories are deleted from the plugin directory.
Flags and examples:
[`finfocus analyzer uninstall`](./cli/finfocus_analyzer_uninstall.md).

## Global Options

The persistent flags on the root command apply to every command. They are
listed under "Options" on the [`finfocus`](./cli/finfocus.md) page and under
"Options inherited from parent commands" on every other page. The `cost`
group adds its own persistent flags, such as `--stack`, `--exit-on-threshold`,
and `--profile`, to its subcommands. Output flags such as `--no-color`,
`--plain`, and `--high-contrast` belong to the commands that render styled
output.

## Date Formats

### Accepted Formats

```bash
# ISO 8601 (YYYY-MM-DD)
finfocus cost actual --from 2024-01-01

# RFC3339 (full timestamp)
finfocus cost actual --from 2024-01-01T00:00:00Z
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

## Historical carbon reports

`finfocus carbon actual` queries available monthly aggregate emissions from
`aws-ce`. Enable `plugins.aws-ce.carbon_enabled` in configuration or set
`FINFOCUS_AWS_CARBON_ENABLED=true`, then restart the plugin. Flags:
[`finfocus carbon actual`](./cli/finfocus_carbon_actual.md).

```sh
finfocus carbon actual --start 2026-01-01 --end 2026-03-01 \
  --method market-based --scope total --output json
```

`--start` is an inclusive and `--end` an exclusive UTC month boundary. The
total covers returned records only. Missing reports have no numeric total;
explicit zero values remain zero. See [historical AWS carbon reporting](../plugins/historical-carbon.md)
for provenance, permissions, pagination behavior, and authenticated web parity.

---

See [User Guide](../guides/user-guide.md) for workflow examples.
