---
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

![Interactive recommendations table showing list of savings](/finfocus/screenshots/recommendations-table.png)

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
    "total_count": 1,
    "total_savings": 50.0,
    "currency": "USD",
    "count_by_action_type": { "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE": 1 },
    "savings_by_action_type": { "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE": 50.0 }
  },
  "recommendations": [
    {
      "resource_id": "i-1234567890abcdef0",
      "action_type": "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE",
      "description": "Switch to t3.small",
      "estimated_savings": 50.0,
      "currency": "USD",
      "id": "rec-123",
      "category": "RECOMMENDATION_CATEGORY_COST",
      "priority": "RECOMMENDATION_PRIORITY_HIGH",
      "confidence_score": 0.82,
      "source": "kubecost",
      "impact": { "projection_period": "monthly", "current_cost": 100.0, "projected_cost": 50.0 },
      "resource": { "name": "web", "region": "us-east-1", "tags": { "env": "prod" } }
    }
  ],
  "total_savings": 50.0,
  "currency": "USD"
}
```

The first five keys of each recommendation (`resource_id`, `action_type`, `description`,
`estimated_savings`, `currency`) and `status` are stable. The remaining keys carry the
full record the plugin returned and are omitted when the plugin did not report them:
`id`, `category`, `priority`, `confidence_score`, `source`, `created_at`, `metadata`,
`reasoning`, `impact` (`projection_period`, `current_cost`, `projected_cost`,
`savings_percentage`, `implementation_cost`, `migration_effort_hours`) and `resource`
(`name`, `provider`, `resource_type`, `region`, `sku`, `tags`, `utilization`).
NDJSON output carries the same keys on each recommendation line. The table output is
unchanged.

The `id` value is the identifier to pass to `dismiss`, `snooze`, `undismiss` and
`history`.

### Scores

When the optional [scoring step](./recommendation-scoring.md) is enabled, each recommendation also carries a `scores`
object (`risk`, `false_positive`, `worth_acting`, `priority`, `insufficient_evidence`, `duplicate_group_id` and
`needs_review`) and the output carries a `scoring` summary. Scores rank work for review; they never dismiss or apply
anything.

### Plugin Routing and Caching

Each resource is sent only to the plugins that the router selects for the
`Recommendations` feature (see the [routing guide](./routing.md)). Internal Pulumi types
are never sent to plugins.

Results are cached per request. The cache key includes the identity, provider, type and
properties of every requested resource plus the IDs of dismissed recommendations, so a
different set of resources, a changed property, or a new dismissal produces a fresh
query. Older cache entries written under the previous key format are not matched and
expire by TTL.

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

- [Plugins](../plugin-system.md) - Plugin configuration

---

**Last Updated**: 2026-01-20
**FinFocus Version**: v0.3.0
**Feedback**: [Open an issue](https://github.com/rshade/finfocus/issues/new) to improve this guide
