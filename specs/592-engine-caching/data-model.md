# Data Model: Unified Engine Caching System

**Branch**: `592-engine-caching` | **Date**: 2026-02-14

## Entities

### Cache (Interface) - NEW

Abstraction over cache storage, used by the engine.

| Method | Input | Output | Description |
| ------ | ----- | ------ | ----------- |
| `Get` | key (string) | entry, error | Retrieve cached data by key |
| `Set` | key (string), data (JSON) | error | Store data with TTL |
| `IsEnabled` | none | bool | Check if caching is active |

**Implementations**: `FileStore` (existing), mock (for tests).

### CacheEntry (Existing - No Changes)

| Field | Type | Description |
| ----- | ---- | ----------- |
| Key | string | SHA256 hash of query parameters |
| Data | JSON | Cached cost result payload |
| CreatedAt | timestamp | When entry was created |
| ExpiresAt | timestamp | When entry expires (CreatedAt + TTL) |
| TTLSeconds | int | TTL for reference |

### KeyParams (Existing - No Changes)

| Field | Type | Description |
| ----- | ---- | ----------- |
| Operation | string | "projected_cost", "actual_cost", "recommendations" |
| Provider | string | "aws", "gcp", "azure", "multi" |
| ResourceTypes | string list | Resource types in the query |
| Filters | string map | Query-specific parameters (SKU, region, dates, tags) |

## Cache Key Compositions

### Projected Cost (Per-Resource)

```text
KeyParams {
  Operation:     "projected_cost"
  Provider:      resource.Provider
  ResourceTypes: [resource.Type]
  Filters:       resource.Properties (converted to string map)
}
```

**Uniqueness guarantee**: Different resource properties (e.g., t3.micro vs t3.large)
produce different filter maps, therefore different SHA256 hashes.

### Actual Cost (Whole-Query)

```text
KeyParams {
  Operation:     "actual_cost"
  Provider:      "multi"
  ResourceTypes: [all resource types in request]
  Filters: {
    "from":      RFC3339 timestamp
    "to":        RFC3339 timestamp
    "adapter":   adapter name (if specified)
    "group_by":  grouping strategy (if specified)
    "tag:<key>": tag value (for each tag)
  }
}
```

**Uniqueness guarantee**: Time ranges, tags, adapter, and groupBy all contribute
to the SHA256 hash. Changing any parameter produces a new cache entry.

### Recommendations (Existing - No Changes)

```text
KeyParams {
  Operation:     "recommendations"
  Provider:      "multi"
  ResourceTypes: [all resource types]
}
```

## State Transitions

### Cache Entry Lifecycle

```text
[Does Not Exist] --Set()--> [Valid]
[Valid] --Get() (within TTL)--> [Valid] (returns data)
[Valid] --Get() (past TTL)--> [Expired] (async delete, returns miss)
[Expired] --Set()--> [Valid] (overwritten with fresh data)
[Valid] --Clear()--> [Does Not Exist]
```

## Relationships

```text
Engine --uses--> Cache (interface)
Cache  --implemented by--> FileStore
FileStore --stores--> CacheEntry
CacheEntry --keyed by--> KeyParams (via GenerateKey)
```
