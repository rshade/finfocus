---
title: Resource History Guide
description: How FinFocus tracks resource identity over time for accurate month-long cost queries.
---

## Overview

FinFocus maintains a **Resource History Store** that tracks which cloud resource
IDs existed over time. This enables accurate actual cost queries across full
billing periods, even when resources are replaced or destroyed mid-month.

**Why this exists**: Cloud provider billing APIs report costs by resource ID
(e.g., EC2 instance `i-abc123`). When Pulumi replaces a resource, the old cloud
ID disappears from state and a new one takes its place. Without history, FinFocus
can only query costs for the *current* resource — missing the old one entirely.

**Target Audience**: End Users, DevOps Engineers, Platform Engineers

**Prerequisites**:

- [FinFocus CLI installed](../getting-started/installation.md)
- [Pulumi project configured](../getting-started/quickstart.md) with at least
  one plugin

**Estimated Time**: 10 minutes

---

## Table of Contents

- [How It Works](#how-it-works)
- [Configuration](#configuration)
- [Data Sources](#data-sources)
- [Cost Allocation Tags](#cost-allocation-tags)
  - [AWS Setup](#aws-setup)
  - [GCP Setup](#gcp-setup)
  - [Azure Setup](#azure-setup)
  - [FinFocus Tag Configuration](#finfocus-tag-configuration)
- [Storage and Retention](#storage-and-retention)
- [Backup and Recovery](#backup-and-recovery)
- [Troubleshooting](#troubleshooting)

---

## How It Works

Every time FinFocus runs (`finfocus overview`, `finfocus cost actual`, or during
`pulumi up` with the analyzer), it records a snapshot of your current resources:

```text
March 1 run:   web-server → i-old123 (recorded)
March 15:      Resource replaced (Pulumi up)
March 16 run:  web-server → i-new456 (recorded)
March 30:      finfocus cost actual --from 03-01 --to 03-31
               → Queries costs for BOTH i-old123 AND i-new456
               → Returns full month: $100 (not just $50)
```

The history store tracks three pieces of information per observation:

| Field | Description |
|-------|-------------|
| **URN** | Pulumi logical resource name (stable across replacements) |
| **Cloud ID** | Physical cloud resource ID (changes on replacement) |
| **Timestamps** | First seen / last seen dates for this (URN, Cloud ID) pair |

When the same URN gets a new cloud ID (replacement), both entries are preserved.
When you query actual costs, FinFocus looks up **all** cloud IDs that existed
during the requested period.

### Query Strategy: ID-First, Tag-Fallback

FinFocus uses a layered query strategy:

1. **Primary**: Query billing APIs by all known cloud IDs from the history store
2. **Fallback**: For resources with no history (cold start, created and destroyed
   between runs), query billing APIs by configured cost allocation tags
3. **Deduplication**: Costs from tag-based queries are deduplicated against
   ID-based results to prevent double-counting

---

## Configuration

```yaml
# ~/.finfocus/config.yaml
cost:
  history:
    enabled: true          # default: true
    retention_days: 90     # how long to keep entries (default: 90)
  allocation:
    enabled: false         # default: false (opt-in)
    tags:                  # tags for billing API fallback queries
      - "pulumi:project"
      - "pulumi:stack"
```

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cost.history.enabled` | bool | `true` | Enable resource history tracking |
| `cost.history.retention_days` | int | `90` | Days to keep history entries |
| `cost.allocation.enabled` | bool | `false` | Enable tag-based billing queries |
| `cost.allocation.tags` | list | `[]` | Tag keys to query billing APIs by |

---

## Data Sources

The history store is fed by multiple sources automatically:

### State Snapshots (Automatic)

Every `finfocus overview` or `finfocus cost actual` run reads the current Pulumi
state and records all resources. This is the primary data source and requires no
configuration.

**What it captures**: URN, cloud ID, resource type, provider, resource tags.

### Plan Lineage (From Preview)

When `finfocus overview` runs `pulumi preview`, it extracts old and new cloud IDs
from replacement and deletion operations. This captures the **outgoing** resource
ID that would otherwise be lost.

**What it captures**: Old cloud ID from `OldState` on replace/delete steps.

### Analyzer Events (From Pulumi Up)

When FinFocus is configured as a Pulumi analyzer, it records resource observations
during every `pulumi up` operation in real-time.

**What it captures**: URN, type, provider, properties per analyzed resource.

---

## Cost Allocation Tags

For the most complete cost coverage — especially for resources that are created
and destroyed between FinFocus runs — configure cost allocation tags on your
cloud resources.

> **Key requirement**: Tags must be present on the resource **while it incurs
> costs**. Cloud billing systems snapshot the tag state per cost record. You
> cannot retroactively tag resources and have historical costs updated.

### AWS Setup

AWS supports provider-level default tags that are applied to all resources:

```yaml
# Pulumi.dev.yaml
config:
  aws:defaultTags:
    tags:
      pulumi:project: my-app
      pulumi:stack: dev
```

Or in code for dynamic values:

```go
provider, _ := aws.NewProvider(ctx, "tagged", &aws.ProviderArgs{
    DefaultTags: &aws.ProviderDefaultTagsArgs{
        Tags: pulumi.StringMap{
            "pulumi:project": pulumi.String(ctx.Project()),
            "pulumi:stack":   pulumi.String(ctx.Stack()),
        },
    },
})
```

**After tagging**: Activate `pulumi:project` and `pulumi:stack` as
[Cost Allocation Tags](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/cost-alloc-tags.html)
in the AWS Billing Console. Tags take approximately 24 hours to appear in
Cost Explorer after activation. Historical costs from before activation are
**not** retroactively tagged.

### GCP Setup

GCP labels must be lowercase with no colons. Use underscores instead:

```yaml
# Pulumi.dev.yaml
config:
  gcp:defaultLabels:
    pulumi_project: my-app
    pulumi_stack: dev
```

FinFocus automatically normalizes `pulumi:project` to `pulumi_project` when
querying GCP billing exports.

> GCP v8.0+ automatically adds the `goog-pulumi-provisioned` label. To opt out,
> set `gcp:addPulumiAttributionLabel = false`.

### Azure Setup

Azure Native does not have a provider-level default tags option. Use a stack
transformation:

```go
ctx.RegisterStackTransformation(func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
    if tags, ok := args.Props["tags"]; ok {
        if tagMap, ok := tags.(pulumi.StringMap); ok {
            tagMap["pulumi:project"] = pulumi.String(ctx.Project())
            tagMap["pulumi:stack"] = pulumi.String(ctx.Stack())
            args.Props["tags"] = tagMap
        }
    }
    return &pulumi.ResourceTransformationResult{Props: args.Props, Opts: args.Opts}
})
```

### FinFocus Tag Configuration

After setting up tags on your cloud resources, configure FinFocus to use them:

```yaml
# ~/.finfocus/config.yaml
cost:
  allocation:
    enabled: true
    tags:
      - "pulumi:project"
      - "pulumi:stack"
      - "Environment"     # well-known billing tag
      - "CostCenter"      # well-known billing tag
```

### Well-Known Cost Allocation Tags

These tags are commonly used for cost attribution across cloud providers:

| Tag Key | Description | AWS | GCP | Azure |
|---------|-------------|-----|-----|-------|
| `pulumi:project` | Pulumi project name | `pulumi:project` | `pulumi_project` | `pulumi:project` |
| `pulumi:stack` | Pulumi stack name | `pulumi:stack` | `pulumi_stack` | `pulumi:stack` |
| `Environment` | dev/staging/prod | `Environment` | `environment` | `Environment` |
| `CostCenter` | Finance cost center | `CostCenter` | `cost_center` | `CostCenter` |
| `Team` | Owning team | `Team` | `team` | `Team` |
| `Application` | Application name | `Application` | `application` | `Application` |

---

## Storage and Retention

### File Location

```text
~/.finfocus/history/history.db    # Resource history (BoltDB)
~/.finfocus/cache/cache.db        # Cost result cache (BoltDB, separate)
```

### History vs Cache

| Property | History (`history.db`) | Cache (`cache.db`) |
|----------|----------------------|-------------------|
| **Purpose** | Resource identity timeline | Cost query results |
| **Lifecycle** | Persistent, accumulates | Ephemeral, rebuildable |
| **Safe to delete?** | **No** — loses identity data | Yes — rebuilds on next query |
| **Retention** | Configurable (default: 90 days) | TTL-based (default: 1 hour) |
| **Grows over time?** | Yes, bounded by retention | No, bounded by TTL + compaction |

### Retention Cleanup

On startup, FinFocus removes history entries with `last_seen` older than the
retention window. Default retention is 90 days.

For long-running environments, consider increasing retention to cover full
billing cycles:

```yaml
cost:
  history:
    retention_days: 365    # keep 1 year of history
```

---

## Backup and Recovery

### Why Back Up History

The history database contains the resource identity timeline that enables
accurate month-long cost queries. If lost:

- Cost queries for past periods may undercount (missing replaced/destroyed
  resources)
- The timeline rebuilds gradually as FinFocus runs, but past observations
  are lost

### Backup

```bash
# Simple file copy (stop finfocus first for consistency)
cp ~/.finfocus/history/history.db ~/.finfocus/history/history.db.bak

# Or include in your infrastructure backup
tar -czf finfocus-history.tar.gz ~/.finfocus/history/
```

### Recovery

```bash
# Restore from backup
cp ~/.finfocus/history/history.db.bak ~/.finfocus/history/history.db
```

If the history database is corrupted, FinFocus will automatically detect the
corruption and recreate the database (following the same pattern as the cache
store). Corrupted data is lost, but FinFocus continues to function with
reduced historical accuracy.

---

## Troubleshooting

### History not recording

1. Check that history is enabled: `finfocus config get cost.history.enabled`
2. Verify the history directory is writable: `ls -la ~/.finfocus/history/`
3. Enable debug logging: `finfocus --debug cost actual ...` and look for
   history-related log entries

### Costs seem incomplete for past months

This usually means FinFocus didn't run while a resource existed:

1. Check history coverage: resources must have been observed by at least one
   FinFocus run during their lifetime
2. Consider enabling tag-based fallback (`cost.allocation.enabled: true`)
   for gap coverage
3. For CI/CD environments, schedule regular FinFocus runs (daily or on every
   `pulumi up`) to minimize observation gaps

### History database growing large

The database is bounded by `retention_days`. If it's larger than expected:

1. Reduce retention: `cost.history.retention_days: 30`
2. Restart FinFocus to trigger cleanup
3. For stacks with many resources, a larger history file is normal

### Tag-based queries returning unexpected results

1. Verify tags are on your cloud resources: check the AWS/GCP/Azure console
2. For AWS: confirm tags are activated as Cost Allocation Tags in Billing Console
3. Check tag key format: GCP uses underscores (`pulumi_project`), not colons
4. Tags must have been present **while costs were incurred** — retroactive
   tagging does not apply to historical billing data
