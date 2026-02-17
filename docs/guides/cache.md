---
layout: default
title: Cache Configuration Guide
description: Configure the BoltDB cost cache to speed up repeated cost queries in FinFocus.
---

## Overview

FinFocus includes an optional BoltDB-backed cache that stores cost query results locally.
When enabled, repeated runs against the same resources return results from disk instead of
making live plugin calls, cutting latency significantly for large stacks.

The cache is **disabled by default**. You opt in by setting a positive TTL value.

**Target Audience**: End Users, DevOps Engineers, CI/CD Operators

**Prerequisites**:

- [FinFocus CLI installed](../getting-started/installation.md)
- [Pulumi project configured](../getting-started/quickstart.md) with at least one plugin

**Learning Objectives**:

- Enable and configure the cost cache
- Understand cache key structure and TTL behaviour
- Disable or invalidate the cache when needed
- Troubleshoot stale or corrupted cache entries

**Estimated Time**: 5 minutes

---

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration Reference](#configuration-reference)
- [How the Cache Works](#how-the-cache-works)
- [Examples](#examples)
  - [Enable Cache for a Project](#example-1-enable-cache-for-a-project)
  - [Override TTL per Run](#example-2-override-ttl-per-run)
  - [CI/CD with Cache Disabled](#example-3-cicd-with-cache-disabled)
- [Troubleshooting](#troubleshooting)
- [See Also](#see-also)

---

## Quick Start

Enable the cache in under 2 minutes.

### Step 1: Add Cache Config

Edit `~/.finfocus/config.yaml` (global) or your project's `.finfocus/config.yaml`:

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
cost:
  cache:
    enabled: true
    ttl_seconds: 3600   # 1 hour; 0 disables the cache
    directory: ""       # empty = auto-resolve
```

### Step 2: Run a Cost Command

```bash
finfocus cost projected --pulumi-json plan.json
```

### Step 3: Run Again and See Cache Hits

```bash
finfocus cost projected --pulumi-json plan.json
```

**Expected Output (second run):**

```text
RESOURCE                          ADAPTER              MONTHLY   CURRENCY  NOTES
aws:ec2/instance:Instance         aws-public (cached)  $73.00    USD       t3.medium
aws:rds/instance:DbInstance       aws-public (cached)  $146.00   USD       db.t3.medium
```

The `(cached)` suffix on the Adapter column confirms results came from disk.

---

## Configuration Reference

### File Location

Cache settings belong under the `cost.cache` key in either:

- `~/.finfocus/config.yaml` - global default, applies to all projects
- `$PROJECT/.finfocus/config.yaml` - project-local override (takes precedence)

### Configuration Options

| Option        | Type    | Default | Required | Description                                        |
| ------------- | ------- | ------- | -------- | -------------------------------------------------- |
| `enabled`     | boolean | `false` | No       | Master switch. `false` disables caching entirely.  |
| `ttl_seconds` | integer | `3600`  | No       | Seconds until a cached entry expires. `0` disables.|
| `directory`   | string  | `""`    | No       | Explicit cache directory path. Empty = auto.       |

### CLI Flag

Override the TTL for a single run without changing config:

```bash
finfocus cost projected --cache-ttl 1800 --pulumi-json plan.json
# 0 disables the cache for this run
finfocus cost projected --cache-ttl 0 --pulumi-json plan.json
```

### Environment Variables

| Variable                  | Description                                   | Example               |
| ------------------------- | --------------------------------------------- | --------------------- |
| `FINFOCUS_CACHE_TTL`      | Override TTL in seconds (integer)             | `7200`                |
| `FINFOCUS_CACHE_TTL_SECONDS` | Fallback alias for `FINFOCUS_CACHE_TTL`    | `7200`                |
| `FINFOCUS_CACHE_DIR`      | Override cache directory                      | `/tmp/finfocus-cache` |

**Precedence** (highest to lowest): `--cache-ttl` flag > `FINFOCUS_CACHE_TTL` env >
`cost.cache.ttl_seconds` config > built-in default (3600 when enabled).

### Cache Directory Resolution

When `directory` is empty, FinFocus resolves the cache location in this order:

1. `FINFOCUS_CACHE_DIR` environment variable
2. `cost.cache.directory` config value
3. Project-local `.finfocus/cache/` (when a `Pulumi.yaml` is found)
4. `~/.finfocus/cache/`

The database file is always named `cache.db` inside the resolved directory.

---

## How the Cache Works

### Storage Backend

The cache uses [BoltDB](https://github.com/etcd-io/bbolt), a single-file embedded
key-value store. No external database process is required.

```text
~/.finfocus/cache/
└── cache.db          # Single BoltDB file containing all cached data
```

### Buckets and Key Structure

The database contains three buckets, one per cost operation:

| Bucket            | Key Format                                    | Scope          |
| ----------------- | --------------------------------------------- | -------------- |
| `projected`       | `projected/{provider}/{type}/{region}/{sku}`  | Per resource   |
| `actual`          | `actual/{adapter}/{start}/{end}/{groupBy}/...`| Per query      |
| `recommendations` | `recommendations/{provider}/{type}/{region}`  | Per resource   |

Projected costs are cached per individual resource, so changing one resource only
invalidates that resource's entry. Actual cost queries are cached as a whole (the full
query including time range and filters forms the key).

### TTL and Expiration

- Expiration is **lazy**: entries are checked on read and discarded if past their TTL.
- A startup cleanup pass removes all expired entries when the cache is opened.
- Setting `ttl_seconds: 0` or `enabled: false` disables the cache; no file is written.

### Graceful Degradation

Cache failures never block cost operations. If the cache file cannot be opened or a
read fails, FinFocus logs a warning and continues with live plugin calls. If the file
is corrupted, FinFocus automatically deletes and recreates it.

### Commands That Use the Cache

`cost projected`, `cost actual`, `cost recommendations`, `cost estimate`, `overview`,
and `analyzer serve` all participate in caching when enabled.

---

## Examples

### Example 1: Enable Cache for a Project

**Use Case**: Speed up repeated `cost projected` runs during active development.

**Configuration** (in `$PROJECT/.finfocus/config.yaml`):

```yaml
cost:
  cache:
    enabled: true
    ttl_seconds: 3600
```

**Usage:**

```bash
# First run - fetches from plugins, populates cache
finfocus cost projected --pulumi-json plan.json

# Second run - returns cached results
finfocus cost projected --pulumi-json plan.json
```

---

### Example 2: Override TTL per Run

**Use Case**: Force fresh data for a specific run without changing global config.

```bash
# Bypass cache entirely for this run
finfocus cost projected --cache-ttl 0 --pulumi-json plan.json

# Use a shorter TTL of 5 minutes for this run
finfocus cost projected --cache-ttl 300 --pulumi-json plan.json
```

---

### Example 3: CI/CD with Cache Disabled

**Use Case**: Always fetch live pricing data in automated pipelines.

```bash
# In your CI pipeline script
finfocus cost projected --cache-ttl 0 --pulumi-json plan.json
```

Alternatively, set the environment variable for the entire job:

```yaml
env:
  FINFOCUS_CACHE_TTL: "0"
steps:
  - run: finfocus cost projected --pulumi-json plan.json
```

---

## Troubleshooting

### Issue: Cache entries never expire

**Symptoms:** Running with fresh plugin data still shows `(cached)` results.

**Cause:** TTL is larger than expected, or the system clock changed.

**Solution:** Force a fresh run by passing `--cache-ttl 0`, then restore the desired TTL:

```bash
finfocus cost projected --cache-ttl 0 --pulumi-json plan.json
```

### Issue: Stale data after resource changes

**Symptoms:** Cost output does not reflect a resource type or SKU change.

**Cause:** Projected cost cache is keyed by `provider/type/region/sku`. If you changed
an input that is part of the key (e.g., instance type), the old key is no longer matched
and the new key is a cache miss - fresh data is fetched automatically. If you changed a
field that is NOT part of the key (e.g., tags), the cached entry is reused.

**Solution:** Use `--cache-ttl 0` for a single run or reduce `ttl_seconds` in config.

### Issue: Corrupted cache file

**Symptoms:** Errors mentioning `bbolt`, `unexpected EOF`, or `invalid database`.

**Cause:** The `cache.db` file was corrupted (e.g., process killed mid-write).

**Solution:** FinFocus auto-recovers by deleting and recreating the file. If auto-recovery
does not trigger, delete the file manually:

```bash
rm ~/.finfocus/cache/cache.db
```

### Issue: Cache directory permission denied

**Symptoms:** Warning like `failed to open cache: permission denied`.

**Solution:** Check ownership of the cache directory:

```bash
ls -la ~/.finfocus/cache/
# Fix ownership if needed
chown -R $USER ~/.finfocus/cache/
```

---

## See Also

**Related Guides:**

- [Budget Configuration Guide](./budgets.md) - Set spending limits and alerts
- [Recommendations Guide](./recommendations.md) - Cost optimization suggestions
- [Troubleshooting Guide](./troubleshooting.md) - General issue resolution

**CLI Reference:**

- [cost projected](../reference/cli-commands.md#cost-projected) - Estimate projected costs
- [cost actual](../reference/cli-commands.md#cost-actual) - Fetch historical costs
- [cost recommendations](../reference/cli-commands.md#cost-recommendations) - Display recommendations

**Configuration Reference:**

- [Cache Configuration](../reference/config-reference.md#cache) - Complete option reference

---

**Last Updated**: 2026-02-17
**FinFocus Version**: v0.3.0
**Feedback**: [Open an issue](https://github.com/rshade/finfocus/issues/new) to improve this guide
