---
layout: default
title: Recommendations Guide
description: Explore and apply cost optimization recommendations from cloud providers using FinFocus.
---

## Overview

FinFocus can aggregate cost optimization recommendations from various cloud providers (via plugins) and present them in
a unified, interactive terminal interface. This helps you identify right-sizing opportunities, idle resources, and
other savings potential.

This guide covers how to view recommendations, filter them by type or priority, and use the interactive Terminal UI (TUI)
to explore details.

**Target Audience**: DevOps Engineers, FinOps Practitioners

**Prerequisites**:

- [FinFocus CLI installed](../getting-started/installation.md)
- [Pulumi project configured](../getting-started/quickstart.md)
- Cost plugins installed (e.g., `vantage`, `kubecost`) that support recommendations

**Learning Objectives**:

- Run cost recommendation scans
- Navigate the interactive recommendation table
- Filter recommendations by category and priority
- Export recommendations for reporting

**Estimated Time**: 10 minutes

---

## Quick Start

Start finding savings in under 5 minutes.

### Step 1: Run Recommendations Scan

Execute the recommendations command against your Pulumi plan:

```bash
finfocus cost recommendations --pulumi-json plan.json
```

### Step 2: Navigate Interactive UI

The command opens an interactive table. Use arrow keys to navigate and `Enter` to see details.

![Interactive recommendations table showing list of savings](../assets/screenshots/recommendations-table.png)

**Figure 1**: Interactive recommendation table with savings summary.

### Step 3: View Details

Select a recommendation to see actionable steps:

```text
Recommendation Details
----------------------
Resource:   aws:ec2/instance:Web-Server
Action:     Right-size
Savings:    $45.00/month
Reason:     CPU utilization < 5% for 30 days
Suggested:  t3.medium -> t3.small
```

---

## Interactive Controls

When running in interactive mode (default for TTY), use these keyboard shortcuts:

| Key       | Action                      |
| --------- | --------------------------- |
| `↑` / `↓` | Navigate list               |
| `Enter`   | View recommendation details |
| `Esc`     | Back to list (from details) |
| `/`       | Filter list (future)        |
| `q`       | Quit                        |

---

## Filtering Recommendations

You can filter recommendations to focus on specific types of savings.

### Filter Syntax

The `--filter` flag accepts key-value pairs:

```bash
finfocus cost recommendations --pulumi-json plan.json --filter "key=value"
```

### Common Filters

| Filter Key | Description      | Example                |
| ---------- | ---------------- | ---------------------- |
| `priority` | Importance level | `priority=high`        |
| `category` | Savings category | `category=cost`        |
| `savings`  | Minimum savings  | `savings>100` (future) |

### Example: High Priority Only

```bash
finfocus cost recommendations --pulumi-json plan.json --filter "priority=high"
```

---

## Examples

### Example 1: Non-Interactive Output (CI/CD)

**Use Case**: Export recommendations to JSON for reporting or automation.

**Command:**

```bash
finfocus cost recommendations --pulumi-json plan.json --output json > savings.json
```

**Output Snippet:**

```json
{
  "summary": {
    "totalPotentialSavings": 150.0,
    "currency": "USD"
  },
  "recommendations": [
    {
      "resourceId": "i-1234567890abcdef0",
      "action": "Terminate",
      "savings": 50.0,
      "priority": "high"
    }
  ]
}
```

### Example 2: Filtering by Category

**Use Case**: Focus only on "right-sizing" opportunities.

**Command:**

```bash
finfocus cost recommendations --pulumi-json plan.json --filter "category=rightsize"
```

---

## Troubleshooting

### Issue: No recommendations found

**Symptoms:**

- Command runs successfully but shows "No recommendations found"

**Cause:**

- Plugins may not be installed or configured
- Cloud provider has no recommendations
- Plan JSON might not match live resources

**Solution:**

Ensure you have a plugin installed (e.g., `vantage`) that provides recommendations and it is properly authenticated.

### Issue: TUI not displaying correctly

**Symptoms:**

- Garbled text or colors
- Layout broken

**Solution:**

Try running with plain mode if your terminal has issues:

```bash
finfocus cost recommendations --pulumi-json plan.json --output table
```

(Note: `--output table` forces non-interactive standard output)

---

## See Also

**Related Guides:**

- [Budget Configuration](./budgets.md) - Set up spending limits

**CLI Reference:**

- [cost recommendations](../reference/cli-commands.md#cost-recommendations) - Command reference

**Configuration Reference:**

- [Plugins](../plugins/README.md) - Plugin configuration

---

**Last Updated**: 2026-01-20
**FinFocus Version**: v0.1.0
**Feedback**: [Open an issue](https://github.com/rshade/finfocus/issues/new) to improve this guide
