# Quickstart: Cache Expires-At Hints

**Branch**: `606-cache-expires-at` | **Date**: 2026-03-12

## What This Feature Does

Allows plugins to control how long their cost responses are cached by setting
an `expires_at` timestamp in the gRPC response. Previously, all cache entries
used a single fixed TTL (default: 1 hour). Now, each plugin can specify per-response
expiration based on its domain knowledge of data freshness.

## How It Works

1. A plugin returns a cost response with `expires_at` set to a future timestamp
2. The engine calculates the remaining time until that timestamp
3. The cache entry uses that duration as its TTL instead of the default
4. If `expires_at` is in the past, the result is not cached at all
5. If `expires_at` is not set, the default TTL applies (no change in behavior)

## Verifying the Feature

### Debug Logging

Enable debug logging to see TTL decisions:

```bash
finfocus cost projected --debug --pulumi-json plan.json
```

Look for log messages like:

```text
DBG using plugin TTL hint component=engine operation=storeProjectedCostCache resource_type=aws:ec2:Instance plugin_ttl_seconds=86400
DBG caching skipped: plugin expires_at is in the past component=engine operation=storeProjectedCostCache resource_type=aws:ec2:Instance
WRN plugin TTL capped at maximum component=engine operation=storeProjectedCostCache resource_type=aws:ec2:Instance capped_ttl_seconds=604800
```

### Cache Behavior

With a plugin that sets `expires_at` 24 hours in the future:

```bash
# First query: calls plugin, caches result with 24h TTL
finfocus cost projected --pulumi-json plan.json

# Second query within 24h: cache hit (no plugin call)
finfocus cost projected --pulumi-json plan.json

# After 24h: cache miss, fresh plugin call
```

### No Plugin Support

If a plugin does not set `expires_at`, behavior is identical to before:

```bash
# Uses default 1h TTL (or user-configured value)
finfocus cost projected --pulumi-json plan.json
```

## Configuration

No new configuration options. The feature is automatic when plugins provide
`expires_at` hints.

Existing cache configuration continues to work:

```bash
# Override default TTL (applies when plugin provides no hint)
export FINFOCUS_CACHE_TTL_SECONDS=7200

# Disable cache entirely (expires_at hints are ignored)
export FINFOCUS_CACHE_ENABLED=false
```

## For Plugin Developers

Set `expires_at` on your gRPC responses to control caching:

```protobuf
// In GetProjectedCostResponse:
message GetProjectedCostResponse {
  // ... other fields ...
  google.protobuf.Timestamp expires_at = 13;  // Caching hint
}

// In ActualCostResult:
message ActualCostResult {
  // ... other fields ...
  google.protobuf.Timestamp expires_at = 8;  // Caching hint
}
```

Guidelines:

- **Stable pricing** (e.g., on-demand instances): Set 24h+ expiration
- **Volatile pricing** (e.g., spot instances): Set short TTL or leave unset
- **Rate-limited APIs**: Set longer TTL to reduce API calls
- **Past timestamp**: Signals "do not cache this response"
- **Maximum**: Core caps at 7 days regardless of plugin hint
