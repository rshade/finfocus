# Data Model: Overview Cost Caching

**Feature**: 600-overview-cache
**Date**: 2026-02-22

## No New Data Model

This feature introduces no new data types, entities, or storage schemas. It
wires the existing cache infrastructure into the overview command's engine
construction.

### Existing Types Reused

| Type | Package | Purpose |
|---|---|---|
| `cache.Cache` | `internal/engine/cache` | Interface: Get, Set, IsEnabled, Close |
| `cache.BoltStore` | `internal/engine/cache` | BoltDB implementation of Cache |
| `cache.CacheEntry` | `internal/engine/cache` | Cached cost result with TTL |

### Existing Cache Key Format

Overview uses projected cost enrichment, so cache keys follow the existing
`projected` bucket format:

```text
projected/{provider}/{type}/{region}/{sku}
```

### Existing Cache Behavior

- TTL-based expiration (lazy check-on-read)
- Cache hits append `(cached)` to the Adapter field
- Corruption detection with auto-recovery (delete + recreate)
- Concurrent reads via `DB.View()`, write coalescing via `DB.Batch()`
