# Quickstart: Overview Cost Caching

**Feature**: 600-overview-cache
**Date**: 2026-02-22

## Enable Caching for Overview

### Via CLI Flag

```bash
# First run: all resources enriched via plugin calls, results cached
finfocus overview --cache-ttl 300

# Second run: cached resources show (cached) in adapter field, much faster
finfocus overview --cache-ttl 300
```

### Via Environment Variable

```bash
export FINFOCUS_CACHE_TTL=600
finfocus overview  # 10-minute cache TTL
```

### Via Config File

In `~/.finfocus/config.yaml` or `$PROJECT/.finfocus/config.yaml`:

```yaml
cost:
  cache:
    ttl_seconds: 300
```

Then run:

```bash
finfocus overview
```

### Disable Caching (Default)

```bash
# No flag, no env var, no config = caching disabled (TTL=0)
finfocus overview

# Explicitly disable even if env/config sets a TTL
finfocus overview --cache-ttl 0
```

## TTL Precedence

1. `--cache-ttl` CLI flag (highest)
2. `FINFOCUS_CACHE_TTL` environment variable
3. `config.yaml` `cost.cache.ttl_seconds`
4. Default: 0 (disabled)

## Verify Caching Works

After the second run with caching enabled, look for `(cached)` in the adapter
column of the overview table or in the detail view of the TUI. This indicates
the cost result was served from the local BoltDB cache rather than a plugin
API call.
