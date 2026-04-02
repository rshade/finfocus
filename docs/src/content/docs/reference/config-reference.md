---
title: Configuration Reference
description: Configuration options for FinFocus Core
---

FinFocus is configured via a configuration file (default:
`~/.finfocus/config.yaml`) and environment variables.

## File Format

The configuration file is in YAML format.

```yaml
output:
  default_format: table # table, json, ndjson
  precision: 2

logging:
  level: info # debug, info, warn, error

plugin_dir: ~/.finfocus/plugins
```

## Configuration Resolution

FinFocus uses a two-tier configuration system where project-local settings
override user-global defaults.

### Two-Tier Configuration

| Tier | Location | Purpose |
| --- | --- | --- |
| **Project-local** | `$PULUMI_PROJECT/.finfocus/config.yaml` | Project-specific budgets, output preferences, plugin config |
| **User-global** | `~/.finfocus/config.yaml` | Shared defaults across all projects |

### Project-Local Directory Structure

When a Pulumi project is detected, FinFocus creates a `.finfocus/` directory
alongside `Pulumi.yaml`:

```text
my-pulumi-project/
├── Pulumi.yaml
├── .finfocus/
│   ├── .gitignore       # Auto-generated, protects user-specific data
│   ├── config.yaml      # Project budgets, output prefs
│   └── dismissed.json   # Per-project recommendation dismissals
```

### Resolution Precedence

**Project-specific settings** (config, dismissals) are resolved in this order:

1. `--project-dir` flag (explicit override)
2. `FINFOCUS_PROJECT_DIR` environment variable
3. Walk up from CWD to find `Pulumi.yaml`, then use `$PROJECT/.finfocus/`
4. Fall back to `~/.finfocus/` (backward compatible)

**Global resources** (plugins, cache, logs) are resolved separately:

1. `FINFOCUS_HOME` environment variable
2. `PULUMI_HOME/finfocus`
3. `~/.finfocus/`

### Config Merge Behavior

Project `config.yaml` overrides global `config.yaml` at the **top-level key**
level (shallow merge). Keys absent in the project config inherit from global
defaults.

For example, if the global config defines `output` and `logging` sections, and
the project config only defines `output`, the project `output` section replaces
the global one entirely while `logging` is inherited from the global config.

```yaml
# ~/.finfocus/config.yaml (global)
output:
  default_format: table
  precision: 2
logging:
  level: info

# my-project/.finfocus/config.yaml (project)
output:
  default_format: json
  precision: 4

# Effective config: output from project, logging from global
```

## Sections

### Output

- `default_format`: The default output format for commands.
- `precision`: Number of decimal places for cost values.

### Logging

- `level`: The verbosity of logs.

### Plugin Directory

- `plugin_dir`: Top-level key that overrides the computed plugin installation directory.
  Equivalent to setting the `FINFOCUS_PLUGIN_DIR` environment variable (env var takes precedence).
  Example: `plugin_dir: /opt/finfocus/plugins`

### Cache

Configure the BoltDB-backed cost calculation cache.

- `enabled`: Master switch for caching. Set to `true` to enable. Default: `false`.
- `ttl_seconds`: Seconds until a cached entry expires. `0` disables expiration.
  Default: `3600` (1 hour).
- `directory`: Explicit path for the cache database file. When empty, auto-resolves
  to the project `.finfocus/` directory or `~/.finfocus/cache/`.

### Resource History

Configure the BoltDB-backed resource history store that tracks cloud resource
identity across replacements and deletions. The history store enables accurate
full-month cost reporting by remembering all cloud IDs a resource has ever had.

> **Important**: Unlike the cache, the history database contains persistent data
> that cannot be regenerated. Deleting `history.db` causes accuracy degradation
> for past billing periods.

| Option | Type | Default | Env Var | Description |
| --- | --- | --- | --- | --- |
| `cost.history.enabled` | bool | `true` | `FINFOCUS_HISTORY_ENABLED` | Master switch for resource history tracking. When disabled, all history operations are no-ops and commands fall back to current-state-only behavior. |
| `cost.history.retention_days` | int | `90` | `FINFOCUS_HISTORY_RETENTION_DAYS` | Days before stale entries are removed. Entries with `LastSeen` older than this window are cleaned up on startup. |
| `cost.history.directory` | string | `~/.finfocus/history` | `FINFOCUS_HISTORY_DIR` | Override the history database directory. When empty, auto-resolves to `~/.finfocus/history/`. |

#### Example: History Configuration

```yaml
cost:
  history:
    enabled: true
    retention_days: 90
    directory: ""  # uses default ~/.finfocus/history
```

### Cost Allocation

Configure tag-based cost attribution for resources that lack direct billing
identity. This is a Layer 2 feature — the configuration is validated and stored,
but the tag-based query logic is deferred to a future milestone.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `cost.allocation.enabled` | bool | `false` | Will enable tag-based cost attribution fallback. The configuration is validated and stored now; tag-based query logic will be applied in a future milestone. |
| `cost.allocation.tags` | string list | `[]` | Tag keys that will be used for billing API queries (e.g., `pulumi:project`, `aws:createdBy`). Each key must be non-empty and at most 128 characters. Validated on load but not yet applied for attribution. |

#### Example: Allocation Configuration

```yaml
cost:
  allocation:
    enabled: true
    tags:
      - "pulumi:project"
      - "aws:createdBy"
      - "team"
```

### Cost & Budgets

Configure budget limits, alerts, and cost calculation preferences.

#### Hierarchical Budget Configuration

The `cost.budgets` section supports hierarchical scoping with `global`, `providers`, `tags`, and `types` sections.

#### `cost.budgets.global`

Global budget applied to all resources.

| Option              | Type    | Default   | Description                                                                        |
| ------------------- | ------- | --------- | ---------------------------------------------------------------------------------- |
| `amount`            | number  | -         | **Required**. The budget limit amount.                                             |
| `currency`          | string  | `USD`     | ISO 4217 currency code.                                                            |
| `period`            | string  | `monthly` | Budget period (daily, weekly, monthly, yearly).                                    |
| `alerts`            | list    | `[]`      | List of alert definitions.                                                         |
| `exit_on_threshold` | boolean | `false`   | Whether to exit CI/CD when the budget threshold is reached (global and per-scope). |
| `exit_code`         | number  | 2         | Exit code when budget exceeded (CI/CD integration).                                |

#### `cost.budgets.providers`

Per-provider budgets for multi-cloud cost control.

| Option                | Type   | Default         | Description                                           |
| --------------------- | ------ | --------------- | ----------------------------------------------------- |
| `<provider>`          | object | -               | Provider name (aws, gcp, azure) with budget settings. |
| `<provider>.amount`   | number | -               | **Required**. Provider budget limit.                  |
| `<provider>.currency` | string | Global currency | Must match global budget currency.                    |

#### `cost.budgets.tags`

Tag-based budgets for team/project cost allocation.

| Option     | Type   | Default         | Description                                         |
| ---------- | ------ | --------------- | --------------------------------------------------- |
| `selector` | string | -               | **Required**. Tag pattern (`key:value` or `key:*`). |
| `priority` | number | 0               | Priority for overlapping tags (higher wins).        |
| `amount`   | number | -               | **Required**. Tag budget limit.                     |
| `currency` | string | Global currency | Must match global budget currency.                  |

#### `cost.budgets.types`

Per-resource-type budgets for category control.

| Option            | Type   | Default         | Description                                                    |
| ----------------- | ------ | --------------- | -------------------------------------------------------------- |
| `<type>`          | object | -               | Resource type (e.g., `aws:ec2/instance`) with budget settings. |
| `<type>.amount`   | number | -               | **Required**. Type budget limit.                               |
| `<type>.currency` | string | Global currency | Must match global budget currency.                             |

#### `cost.budgets.alerts` (within any scope)

| Option      | Type   | Default  | Description                                                        |
| ----------- | ------ | -------- | ------------------------------------------------------------------ |
| `threshold` | number | -        | **Required**. Percentage of budget (1-100) to trigger alert.       |
| `type`      | string | `actual` | Trigger on `actual` (historical) or `forecasted` (projected) cost. |

#### Example: Scoped Budget Configuration

```yaml
cost:
  budgets:
    global:
      amount: 10000.00
      currency: USD
      period: monthly
      exit_code: 2
      alerts:
        - threshold: 80
          type: actual
    providers:
      aws:
        amount: 5000.00
      gcp:
        amount: 3000.00
    tags:
      - selector: 'team:platform'
        priority: 100
        amount: 3000.00
      - selector: 'env:prod'
        priority: 50
        amount: 5000.00
    types:
      'aws:ec2/instance':
        amount: 2000.00
      'aws:rds/instance':
        amount: 3000.00
```

See [Budget Configuration Guide](../guides/budgets.md) for detailed usage.

## JSON Schema Validation

For IDE autocompletion (VS Code, JetBrains), add this comment to the top of your `config.yaml`:

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
```
