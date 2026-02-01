---
layout: default
title: Budget Configuration Guide
description: Configure cost budgets, alerts, and thresholds to prevent cloud cost overruns in FinFocus.
---

## Overview

Budgets allow you to set spending limits for your infrastructure and receive alerts when costs exceed defined thresholds.
By configuring budgets in FinFocus, you can proactively manage cloud spending and prevent unexpected overruns before
they happen.

This guide covers how to define monthly budgets, set up alerts for actual and projected costs, and integrate budget
checks into your CI/CD pipelines.

**Target Audience**: End Users, DevOps Engineers, FinOps Practitioners

**Prerequisites**:

- [FinFocus CLI installed](../getting-started/installation.md)
- [Pulumi project configured](../getting-started/quickstart.md) with cost data

**Learning Objectives**:

- Configure monthly budget amounts and currencies
- Set up alerts for actual vs. forecasted costs
- Integrate budget enforcement into CI/CD workflows
- Troubleshoot common budget configuration issues

**Estimated Time**: 10 minutes

---

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration Reference](#configuration-reference)
- [Examples](#examples)
  - [Single Budget Threshold](#example-1-single-budget-threshold)
  - [Multiple Alert Thresholds](#example-2-multiple-alert-thresholds)
  - [CI/CD Integration](#example-3-cicd-integration)
- [Scoped Budgets](#scoped-budgets)
  - [Provider Budgets](#provider-budgets)
  - [Tag Budgets](#tag-budgets)
  - [Resource Type Budgets](#resource-type-budgets)
- [Troubleshooting](#troubleshooting)
- [See Also](#see-also)

---

## Quick Start

Get started with budgets in under 5 minutes.

### Step 1: Configure Budget

Create or edit your `~/.finfocus/config.yaml` to define a budget:

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
cost:
  budgets:
    amount: 500.00
    currency: USD
    period: monthly
    alerts:
      - threshold: 80
        type: actual
      - threshold: 100
        type: forecasted
```

### Step 2: Run Cost Analysis

Run a cost projection to see how your infrastructure compares to the budget:

```bash
finfocus cost projected --pulumi-json plan.json
```

### Step 3: Review Output

**Expected Output:**

```text
Budget: $500.00 (75% used)
[=====================>......] $375.00 / $500.00

RESOURCE                          ADAPTER     MONTHLY   CURRENCY  NOTES
aws:ec2/instance:Instance         aws-spec    $375.00   USD       t3.xlarge
```

![Budget status display with color-coded threshold bars and emoji indicators](../assets/screenshots/budget-tty-mode.png)

**Figure 1**: Budget display showing usage against defined threshold.

**What's Next?**

- [Configure advanced alerts](#examples)
- [Set up CI/CD gates](#example-3-cicd-integration)
- [Review configuration options](#configuration-reference)

---

## Configuration Reference

Complete reference for all budget configuration options.

### File Location

Configuration is stored in `~/.finfocus/config.yaml`.

### Schema Reference

For IDE autocomplete and validation, add this comment to your config file:

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
```

### Configuration Options

| Option     | Type   | Default     | Required | Description                                    |
| ---------- | ------ | ----------- | -------- | ---------------------------------------------- |
| `amount`   | number | -           | Yes      | Budget amount in specified currency            |
| `currency` | string | `"USD"`     | No       | ISO 4217 currency code (USD, EUR, GBP)         |
| `period`   | string | `"monthly"` | No       | Budget period (daily, weekly, monthly, yearly) |
| `alerts`   | list   | `[]`        | No       | Alert thresholds (see Alerts Options below)    |

### Alerts Options

| Option      | Type   | Default    | Required | Description                     |
| ----------- | ------ | ---------- | -------- | ------------------------------- |
| `threshold` | number | -          | Yes      | Percentage of budget (1-100)    |
| `type`      | string | `"actual"` | No       | Alert type (actual, forecasted) |

### Environment Variables

Override configuration with environment variables:

| Variable                   | Description            | Example  |
| -------------------------- | ---------------------- | -------- |
| `FINFOCUS_BUDGET_AMOUNT`   | Override budget amount | `500.00` |
| `FINFOCUS_BUDGET_CURRENCY` | Override currency      | `EUR`    |

See [Configuration Reference](../reference/config-reference.md#budgets) for complete details.

---

## Examples

Practical examples for common budget scenarios.

### Example 1: Single Budget Threshold

**Use Case**: Simple enforcement to ensure costs don't exceed a hard limit.

**Configuration:**

```yaml
cost:
  budgets:
    amount: 1000.00
    currency: USD
    period: monthly
    alerts:
      - threshold: 100
        type: actual
```

**Usage:**

```bash
finfocus cost projected --pulumi-json plan.json
```

**Explanation:**

This configuration sets a hard limit of $1000/month. If actual costs exceed this amount, the CLI will display a
warning and exit with a non-zero status code (if configured).

See [complete example](../examples/config-budgets/single-threshold.yaml).

---

### Example 2: Multiple Alert Thresholds

**Use Case**: Progressive warnings to catch cost creep early.

**Configuration:**

```yaml
cost:
  budgets:
    amount: 2000.00
    currency: USD
    alerts:
      - threshold: 50
        type: actual
        # Early warning at 50%
      - threshold: 80
        type: forecasted
        # Warning if projected to reach 80%
      - threshold: 100
        type: actual
        # Critical alert at 100%
```

**Explanation:**

This setup provides early visibility. You'll get notified when you hit 50% of budget, if you're _projected_ to hit
80%, and finally when you breach 100%.

See [complete example](../examples/config-budgets/multiple-thresholds.yaml).

---

### Example 3: CI/CD Integration

**Use Case**: Fail build pipelines when infrastructure changes exceed budget.

**Configuration:**

```yaml
cost:
  budgets:
    amount: 500.00
    alerts:
      - threshold: 100
        type: forecasted
```

**Usage:**

```bash
# In your CI pipeline script
finfocus cost projected --pulumi-json plan.json || {
  echo "Budget exceeded!"
  exit 1
}
```

**Explanation:**

By using `type: forecasted` at 100% threshold, FinFocus checks if the _new_ infrastructure plan will push total costs
over budget. If yes, it returns a non-zero exit code, stopping the deployment.

See [complete example](../examples/config-budgets/cicd-integration.yaml).

---

## Scoped Budgets

Define budgets at multiple levels for granular cost control: global, per-provider, per-tag, and per-resource-type.

### Provider Budgets

Track and limit spending per cloud provider (AWS, GCP, Azure).

**Configuration:**

```yaml
cost:
  budgets:
    # Global budget applies to all resources (required when scopes defined)
    global:
      amount: 5000.00
      currency: USD
      period: monthly
      alerts:
        - threshold: 80
          type: actual

    # Per-provider budgets
    providers:
      aws:
        amount: 3000.00
      gcp:
        amount: 2000.00
      azure:
        amount: 1000.00
```

**Usage:**

```bash
# View all budgets including provider breakdown
finfocus cost projected --pulumi-json plan.json

# Filter to show only provider budgets
finfocus cost projected --pulumi-json plan.json --budget-scope=provider

# Filter to a specific provider
finfocus cost projected --pulumi-json plan.json --budget-scope=provider=aws
```

**Example Output:**

```text
BUDGET STATUS
═════════════════════════════════════════════════════════════

GLOBAL
  Budget: $5,000.00  |  Spend: $3,250.00 (65.0%)
  ████████████████████░░░░░░░░░░  OK

BY PROVIDER
───────────────────────────────────────────────────────────────
  aws      Budget: $3,000.00 | Spend: $2,100.00 (70.0%)  OK
  gcp      Budget: $2,000.00 | Spend: $1,150.00 (57.5%)  OK
  azure    Budget: $1,000.00 | Spend: $0.00 (0.0%)       OK

Overall Health: OK
```

**Key Points:**

- Provider names are case-insensitive (`aws`, `AWS`, `Aws` all match)
- Provider is extracted from resource type (e.g., `aws:ec2/instance` → `aws`)
- All provider budgets must use the same currency as the global budget
- Each resource's cost counts toward both its provider budget AND the global budget

### Tag Budgets

Track costs by resource tags (e.g., `team:platform`, `env:prod`) with priority-based allocation.

**Configuration:**

```yaml
cost:
  budgets:
    # Global budget applies to all resources (required when scopes defined)
    global:
      amount: 10000.00
      currency: USD
      period: monthly

    # Per-tag budgets with priority ordering
    tags:
      - selector: 'team:platform'
        priority: 100
        amount: 3000.00
      - selector: 'team:backend'
        priority: 100
        amount: 2500.00
      - selector: 'env:prod'
        priority: 50
        amount: 5000.00
      - selector: 'cost-center:*'
        priority: 10
        amount: 1000.00
```

**Usage:**

```bash
# View all budgets including tag breakdown
finfocus cost projected --pulumi-json plan.json

# Filter to show only tag budgets
finfocus cost projected --pulumi-json plan.json --budget-scope=tag
```

**Example Output:**

```text
BUDGET STATUS
═════════════════════════════════════════════════════════════

GLOBAL
  Budget: $10,000.00  |  Spend: $6,500.00 (65.0%)
  ████████████████████░░░░░░░░░░  OK

BY TAG
───────────────────────────────────────────────────────────────
  team:platform  Budget: $3,000.00 | Spend: $2,100.00 (70.0%)  OK
  team:backend   Budget: $2,500.00 | Spend: $1,500.00 (60.0%)  OK
  env:prod       Budget: $5,000.00 | Spend: $4,200.00 (84.0%)  WARNING

Overall Health: WARNING
```

**Tag Selector Patterns:**

| Pattern     | Description                             | Example Match                                          |
| ----------- | --------------------------------------- | ------------------------------------------------------ |
| `key:value` | Exact match on tag key and value        | `team:platform` matches resources with `team=platform` |
| `key:*`     | Wildcard match on any value for the key | `env:*` matches `env=prod`, `env=dev`, `env=staging`   |

**Priority-Based Allocation:**

When a resource matches multiple tag budgets, cost is allocated to the highest priority budget only:

- Higher priority values take precedence (100 > 50 > 10)
- If multiple budgets share the same priority, the first alphabetically wins
- A warning is emitted when priority ties occur

**Configuration Tips:**

- Use specific selectors (`team:platform`) for known teams
- Use wildcards (`cost-center:*`) as catch-all budgets with lower priority
- Ensure higher-priority budgets are more specific to avoid allocation conflicts

### Resource Type Budgets

Track and limit spending per resource type (e.g., `aws:ec2/instance`, `gcp:compute/instance`).

**Configuration:**

```yaml
cost:
  budgets:
    # Global budget applies to all resources (required when scopes defined)
    global:
      amount: 10000.00
      currency: USD
      period: monthly

    # Per-resource-type budgets
    types:
      'aws:ec2/instance':
        amount: 2000.00
      'aws:rds/instance':
        amount: 3000.00
      'gcp:compute/instance':
        amount: 1500.00
```

**Usage:**

```bash
# View all budgets including type breakdown
finfocus cost projected --pulumi-json plan.json

# Filter to show only resource type budgets
finfocus cost projected --pulumi-json plan.json --budget-scope=type
```

**Example Output:**

```text
BUDGET STATUS
═════════════════════════════════════════════════════════════

GLOBAL
  Budget: $10,000.00  |  Spend: $5,500.00 (55.0%)
  █████████████████░░░░░░░░░░░░░░  OK

BY TYPE
───────────────────────────────────────────────────────────────
  aws:ec2/instance  Budget: $2,000.00 | Spend: $1,200.00 (60.0%)  OK
  aws:rds/instance  Budget: $3,000.00 | Spend: $2,700.00 (90.0%)  CRITICAL

Overall Health: CRITICAL
```

**Key Points:**

- Resource types use exact matching (case-sensitive)
- Type is extracted from Pulumi resource type (e.g., `aws:ec2/instance:Instance` → `aws:ec2/instance`)
- All type budgets must use the same currency as the global budget
- Each resource's cost counts toward its type budget AND the global budget
- Unconfigured resource types do not appear in the BY TYPE section

---

## Troubleshooting

Common issues and solutions for budget configuration.

### Issue: Alerts not triggering

**Symptoms:**

- Costs clearly exceed budget but no alert is shown
- Exit code is 0 despite overage

**Cause:**

- Mismatch between `currency` in config and cost data
- Using `actual` alert type for projected costs (or vice versa)

**Solution:**

Check your currency matches your cloud provider data:

```yaml
currency: USD # Ensure this matches plugin output
```

And ensure you're using the right alert type. Use `forecasted` for `cost projected` commands.

### Issue: Schema validation errors

**Symptoms:**

- IDE highlights config properties in red
- `unknown property` errors

**Solution:**

Ensure you have the correct schema directive and your indentation is correct:

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
```

---

## See Also

**Related Guides:**

- [Recommendations Guide](./recommendations.md) - Cost optimization suggestions
- [Accessibility Guide](./accessibility.md) - Terminal display options

**CLI Reference:**

- [cost projected](../reference/cli-commands.md#cost-projected) - Estimate projected costs
- [cost recommendations](../reference/cli-commands.md#cost-recommendations) - Display recommendations

**Configuration Reference:**

- [Budget Configuration](../reference/config-reference.md#budgets) - Complete option reference

**Examples:**

- [Budget Examples](../examples/config-budgets/) - Runnable configuration files

---

**Last Updated**: 2026-01-27
**FinFocus Version**: v0.1.0
**Feedback**: [Open an issue](https://github.com/rshade/finfocus/issues/new) to improve this guide
