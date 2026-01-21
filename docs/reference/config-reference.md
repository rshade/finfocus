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

#### `cost.budgets`

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `amount` | number | - | **Required**. The budget limit amount. |
| `currency` | string | `USD` | ISO 4217 currency code. |
| `period` | string | `monthly` | Budget period (daily, weekly, monthly, yearly). |
| `alerts` | list | `[]` | List of alert definitions. |

#### `cost.budgets.alerts`

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `threshold` | number | - | **Required**. Percentage of budget (1-100) to trigger alert. |
| `type` | string | `actual` | Trigger on `actual` (historical) or `forecasted` (projected) cost. |

## JSON Schema Validation

For IDE autocompletion (VS Code, JetBrains), add this comment to the top of your `config.yaml`:

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
```
