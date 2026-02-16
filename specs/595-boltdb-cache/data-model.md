# Data Model: BoltDB Cache Backend

**Feature**: 595-boltdb-cache
**Date**: 2026-02-16

## Entity: CacheEntry

The fundamental unit of cached data. Stored as a JSON-encoded value in BoltDB.

| Field | Type | Description |
|-------|------|-------------|
| Key | string | Structured cache key (human-readable, `/`-separated) |
| Data | json.RawMessage | Cached cost result data (opaque JSON blob) |
| CreatedAt | int64 (Unix) | When the entry was created |
| ExpiresAt | int64 (Unix) | When the entry expires (CreatedAt + TTL) |
| TTLSeconds | int | Original TTL for reference and Touch() operations |

**Validation Rules**:

- Key must be non-empty
- Data must be valid JSON
- ExpiresAt must be > CreatedAt
- TTLSeconds must be in range [60, 604800]

**State Transitions**:

```text
[Created] → (TTL elapsed) → [Expired] → (lazy delete or cleanup) → [Deleted]
```

## Entity: BoltStore

The cache storage backend wrapping a bbolt database.

| Field | Type | Description |
|-------|------|-------------|
| db | *bbolt.DB | Open database handle |
| ttlSeconds | int | Default TTL for new entries |
| maxSizeMB | int | Maximum database file size (0 = unlimited) |
| enabled | bool | Whether caching is active |

**Lifecycle**:

```text
[Closed] → Open(path) → [Open] → Close() → [Closed]
                            ↓
                   (corruption) → Remove + Reopen → [Open]
                            ↓
                   (lock timeout) → [Disabled]
```

## Entity: Cache Bucket

Logical namespace within the BoltDB file. Created on database initialization.

| Bucket Name | Purpose | Key Pattern |
|-------------|---------|-------------|
| `projected` | Per-resource projected cost results | `{provider}/{resourceType}/{region}/{sku}` |
| `actual` | Whole-query actual cost results | `{provider}/{resourceType}/{from}/{to}/{filter-hash}` |
| `recommendations` | Recommendation results per resource set | `multi/{sorted-types-hash}` |

**Relationships**:

- A BoltStore contains exactly 3 buckets
- Each bucket contains 0..N CacheEntry values
- Buckets are independent; operations on one do not affect others

## Entity: Cache Key

Structured identifier that encodes both the bucket and resource identity.

**Full Key Format**: `{bucket}/{provider}/{resourceType}/{...additional}`

The first path segment identifies the bucket. The remaining segments form the key within that bucket and support prefix-based operations.

**Key Construction Rules**:

1. All segments are joined with `/`
2. Provider and resource type are used as-is (case-sensitive, matching Pulumi conventions)
3. Additional segments vary by bucket type
4. For actual costs, time ranges use `YYYY-MM-DD` format
5. For complex filters, a deterministic hash of sorted filter key-value pairs is appended

**Prefix Scan Examples**:

| Prefix | Matches |
|--------|---------|
| `projected/aws/` | All projected AWS costs |
| `projected/aws/ec2:Instance/` | All projected AWS EC2 instance costs |
| `projected/aws/ec2:Instance/us-east-1/` | All projected AWS EC2 in us-east-1 |
| `actual/aws/` | All actual AWS costs |

## Database File Layout

```text
$PROJECT/.finfocus/
└── cache.db             # BoltDB database file (project-local)
    ├── [meta page 0]    # Internal: transaction metadata
    ├── [meta page 1]    # Internal: backup metadata
    ├── projected/        # Bucket: per-resource projected costs
    │   ├── aws/ec2:Instance/us-east-1/t3.micro → {CacheEntry JSON}
    │   └── aws/rds:DBInstance/us-west-2/db.t3.medium → {CacheEntry JSON}
    ├── actual/           # Bucket: whole-query actual costs
    │   └── aws/ec2:Instance/2025-01-01/2025-02-01/a3f2... → {CacheEntry JSON}
    └── recommendations/  # Bucket: recommendation results
        └── multi/ec2:Instance+rds:DBInstance → {CacheEntry JSON}
```
