# Data Model: Resource History Store

**Date**: 2026-03-30
**Branch**: `608-resource-history-store`

## Entities

### ResourceHistoryEntry

Represents a single observation of a resource's cloud identity at a point in
time. Multiple entries can exist for the same URN (one per cloud ID
incarnation).

| Field      | Type              | Description                                          |
| ---------- | ----------------- | ---------------------------------------------------- |
| URN        | string            | Full Pulumi URN (e.g., `urn:pulumi:dev::app::aws:ec2/instance:Instance::web`) |
| CloudID    | string            | Cloud provider resource ID (e.g., `i-0abc123def`)    |
| Type       | string            | Pulumi type token (e.g., `aws:ec2/instance:Instance`) |
| Provider   | string            | Cloud provider name (e.g., `aws`, `gcp`, `azure`)    |
| FirstSeen  | Unix timestamp    | When this URN+CloudID pair was first observed         |
| LastSeen   | Unix timestamp    | When this URN+CloudID pair was last observed          |
| Source     | string (enum)     | One of: `state_snapshot`, `plan_lineage`, `analyzer_event` |
| Tags       | map[string]string | Resource tags at time of observation                  |

**Identity rule**: `(URN, CloudID)` is the unique key. Same pair observed
again updates `LastSeen` only. New CloudID for the same URN creates a new
entry.

**Lifecycle**:

```text
[Not Exists] ──upsert──▶ [Active] ──last_seen ages──▶ [Stale] ──cleanup──▶ [Removed]
                              │                            │
                              ▼                            │
                   upsert: update LastSeen                 │
                              │                            │
                              ◀────────────────────────────┘
                         (if re-observed before cleanup)
```

### ResourceTagEntry

An index mapping tag key-value pairs to resource URNs for efficient tag-based
lookups. Used by the Layer 2 tag-based cost attribution fallback.

| Field     | Type           | Description                                   |
| --------- | -------------- | --------------------------------------------- |
| TagKey    | string         | Tag key (e.g., `pulumi:project`)              |
| TagValue  | string         | Tag value (e.g., `myapp`)                     |
| URNHash   | string         | SHA-256 hash of the resource URN              |
| CloudID   | string         | Most recent cloud ID for this URN             |
| FirstSeen | Unix timestamp | When this tag association was first observed   |
| LastSeen  | Unix timestamp | When this tag association was last observed    |

**Identity rule**: `(TagKey, TagValue, URNHash)` is the unique key.

### HistoryConfig

User-configurable settings for the history store.

| Field          | Type     | Default | Description                         |
| -------------- | -------- | ------- | ----------------------------------- |
| Enabled        | bool     | true    | Whether history tracking is active  |
| RetentionDays  | int      | 90      | Days before stale entries are removed |
| Directory      | string   | `""`    | Override history directory (default: `~/.finfocus/history`) |

### AllocationConfig

User-configurable settings for tag-based cost attribution (Layer 2).

| Field   | Type     | Default | Description                               |
| ------- | -------- | ------- | ----------------------------------------- |
| Enabled | bool     | false   | Whether tag-based fallback is active      |
| Tags    | []string | `[]`    | Tag keys to use for billing API queries   |

## Storage Schema

### BoltDB Buckets

**Bucket: `resource_history`**

- Key format: `{stack_hash}/{urn_hash}/{cloud_id}`
- Value: JSON-serialized `ResourceHistoryEntry`
- Prefix scan `{stack_hash}/{urn_hash}/` returns all incarnations of a URN
- Prefix scan `{stack_hash}/` returns all resources in a stack

**Bucket: `resource_tags`**

- Key format: `{stack_hash}/{tag_key}:{tag_value}/{urn_hash}`
- Value: JSON-serialized `ResourceTagEntry`
- Prefix scan `{stack_hash}/{tag_key}:{tag_value}/` returns all URNs with
  that tag

### Hash Functions

- `stack_hash`: SHA-256 of `"{org}/{project}/{stack}"`, hex-encoded, first
  16 characters
- `urn_hash`: SHA-256 of the full URN string, hex-encoded, first 16
  characters

### Serialization

JSON with Unix timestamps (int64) for time fields, matching the cache
entry serialization pattern from `internal/engine/cache/entry.go`.

## Relationships

```text
┌─────────────────────────┐       ┌──────────────────────────┐
│  ResourceHistoryEntry   │       │   ResourceTagEntry       │
│                         │       │                          │
│  URN ──────────────────────────── URNHash                  │
│  CloudID               │       │  CloudID                 │
│  Tags ─────────────────────────── TagKey:TagValue          │
│  FirstSeen / LastSeen   │       │  FirstSeen / LastSeen    │
└─────────────────────────┘       └──────────────────────────┘
         │                                    │
         │  1:N (one URN, many cloud IDs)     │  M:N (many tags, many URNs)
         │                                    │
         ▼                                    ▼
┌─────────────────────────────────────────────────────────────┐
│                     BoltDB history.db                        │
│  ┌─────────────────────┐  ┌──────────────────────────────┐  │
│  │  resource_history   │  │      resource_tags           │  │
│  │  bucket             │  │      bucket                  │  │
│  └─────────────────────┘  └──────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Query Patterns

| Query                            | Bucket           | Scan Type     |
| -------------------------------- | ---------------- | ------------- |
| All cloud IDs for a URN          | resource_history | Prefix: `{stack}/{urn}/` |
| All resources in a stack         | resource_history | Prefix: `{stack}/` |
| URNs active in date range        | resource_history | Prefix + filter on FirstSeen/LastSeen |
| Deleted resources (in history, not in state) | resource_history | Prefix + set difference with current state |
| Resources by tag                 | resource_tags    | Prefix: `{stack}/{key}:{value}/` |

## Validation Rules

- URN: Required, non-empty, max 1024 characters
- CloudID: Required, non-empty, max 512 characters
- Type: Required, non-empty, max 256 characters
- Provider: Required, non-empty, max 64 characters
- Source: Must be one of `state_snapshot`, `plan_lineage`, `analyzer_event`
- FirstSeen: Must be positive Unix timestamp
- LastSeen: Must be >= FirstSeen
- Tags: Max 50 entries, key max 128 chars, value max 256 chars
