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

| Option | Type | Default | Required | Description |
|--------|------|---------|----------|-------------|
| `amount` | number | - | Yes | Budget amount in specified currency |
| `currency` | string | `"USD"` | No | ISO 4217 currency code (USD, EUR, GBP) |
| `period` | string | `"monthly"` | No | Budget period (daily, weekly, monthly, yearly) |
| `alerts` | list | `[]` | No | Alert thresholds (see Alerts Options below) |

### Alerts Options

| Option | Type | Default | Required | Description |
|--------|------|---------|----------|-------------|
| `threshold` | number | - | Yes | Percentage of budget (1-100) |
| `type` | string | `"actual"` | No | Alert type (actual, forecasted) |

### Environment Variables

Override configuration with environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `FINFOCUS_BUDGET_AMOUNT` | Override budget amount | `500.00` |
| `FINFOCUS_BUDGET_CURRENCY` | Override currency | `EUR` |

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

This setup provides early visibility. You'll get notified when you hit 50% of budget, if you're *projected* to hit
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

By using `type: forecasted` at 100% threshold, FinFocus checks if the *new* infrastructure plan will push total costs
over budget. If yes, it returns a non-zero exit code, stopping the deployment.

See [complete example](../examples/config-budgets/cicd-integration.yaml).

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
currency: USD  # Ensure this matches plugin output
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

**Last Updated**: 2026-01-20
**FinFocus Version**: v0.1.0
**Feedback**: [Open an issue](https://github.com/rshade/finfocus/issues/new) to improve this guide
