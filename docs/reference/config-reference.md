---
title: Configuration Reference
description: Configuration options for FinFocus Core
layout: default
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

plugins:
  dir: ~/.finfocus/plugins
```

## Sections

### Output

- `default_format`: The default output format for commands.
- `precision`: Number of decimal places for cost values.

### Logging

- `level`: The verbosity of logs.

### Plugins

- `dir`: The directory where plugins are installed.

### Cost & Budgets

Configure budget limits, alerts, and cost calculation preferences.

#### Legacy Budget Configuration (Deprecated)

The flat `cost.budgets` structure with `amount`/`currency`/`period` at the top level is deprecated.
Use `cost.scoped_budgets` with `global`, `providers`, `tags`, and `types` sections instead.

#### `cost.scoped_budgets.global`

Global budget applied to all resources.

| Option              | Type    | Default   | Description                                                                        |
| ------------------- | ------- | --------- | ---------------------------------------------------------------------------------- |
| `amount`            | number  | -         | **Required**. The budget limit amount.                                             |
| `currency`          | string  | `USD`     | ISO 4217 currency code.                                                            |
| `period`            | string  | `monthly` | Budget period (daily, weekly, monthly, yearly).                                    |
| `alerts`            | list    | `[]`      | List of alert definitions.                                                         |
| `exit_on_threshold` | boolean | `false`   | Whether to exit CI/CD when the budget threshold is reached (global and per-scope). |
| `exit_code`         | number  | 2         | Exit code when budget exceeded (CI/CD integration).                                |

#### `cost.scoped_budgets.providers`

Per-provider budgets for multi-cloud cost control.

| Option                | Type   | Default         | Description                                           |
| --------------------- | ------ | --------------- | ----------------------------------------------------- |
| `<provider>`          | object | -               | Provider name (aws, gcp, azure) with budget settings. |
| `<provider>.amount`   | number | -               | **Required**. Provider budget limit.                  |
| `<provider>.currency` | string | Global currency | Must match global budget currency.                    |

#### `cost.scoped_budgets.tags`

Tag-based budgets for team/project cost allocation.

| Option     | Type   | Default         | Description                                         |
| ---------- | ------ | --------------- | --------------------------------------------------- |
| `selector` | string | -               | **Required**. Tag pattern (`key:value` or `key:*`). |
| `priority` | number | 0               | Priority for overlapping tags (higher wins).        |
| `amount`   | number | -               | **Required**. Tag budget limit.                     |
| `currency` | string | Global currency | Must match global budget currency.                  |

#### `cost.scoped_budgets.types`

Per-resource-type budgets for category control.

| Option            | Type   | Default         | Description                                                    |
| ----------------- | ------ | --------------- | -------------------------------------------------------------- |
| `<type>`          | object | -               | Resource type (e.g., `aws:ec2/instance`) with budget settings. |
| `<type>.amount`   | number | -               | **Required**. Type budget limit.                               |
| `<type>.currency` | string | Global currency | Must match global budget currency.                             |

#### `cost.scoped_budgets.alerts` (within any scope)

| Option      | Type   | Default  | Description                                                        |
| ----------- | ------ | -------- | ------------------------------------------------------------------ |
| `threshold` | number | -        | **Required**. Percentage of budget (1-100) to trigger alert.       |
| `type`      | string | `actual` | Trigger on `actual` (historical) or `forecasted` (projected) cost. |

#### Example: Scoped Budget Configuration

```yaml
cost:
  scoped_budgets:
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
