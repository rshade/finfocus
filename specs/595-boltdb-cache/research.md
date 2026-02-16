# Research: BoltDB Cache Backend

**Feature**: 595-boltdb-cache
**Date**: 2026-02-16

## Decision 1: Bucket Design

**Decision**: Use 3 top-level buckets (`projected`, `actual`, `recommendations`) with metadata inline in the value.

**Rationale**:

- Buckets provide natural namespace isolation; clearing projected costs doesn't touch actual costs
- Lookups only traverse entries of the same type (FR-010, SC-007)
- Inline metadata avoids double reads/writes per operation and keeps atomicity simple
- Nested buckets add B-tree traversal overhead; flat top-level buckets are optimal for 3 categories

**Alternatives Considered**:

- Single bucket with prefixed keys: Rejected because prefix scanning on a single flat bucket doesn't provide the isolation benefit of separate B-trees
- Separate metadata bucket: Rejected because it doubles transaction cost and creates orphan risk on crash
- Nested buckets: Rejected per bbolt best practices (issue #120, #293)

## Decision 2: Key Schema

**Decision**: Use `/`-separated hierarchical keys with bucket name as the first segment. The full key format is `{bucket}/{provider}/{resourceType}/{distinguishing-attrs}`.

**Rationale**:

- `/` is the conventional hierarchical separator (URIs, file paths, etcd keys)
- Pulumi resource types use `:` internally (`aws:ec2:Instance`), so `/` avoids ambiguity
- BoltDB stores keys in byte-sorted order; `Cursor.Seek()` performs efficient binary search on B+tree for prefix scans
- Field ordering (provider > resourceType > region > sku) matches most common query patterns
- Human-readable keys enable debugging via `bbolt keys` CLI tool

**Key Formats by Bucket**:

- **Projected**: `projected/aws/ec2:Instance/us-east-1/t3.micro`
- **Actual**: `actual/aws/ec2:Instance/2025-01-01/2025-02-01/{filter-hash}`
- **Recommendations**: `recommendations/multi/{sorted-resource-types-hash}`

**Alternatives Considered**:

- SHA256 hashes (current): Rejected because the JSON cache is unreleased and we can start fresh with readable keys
- `:` separator: Rejected because Pulumi types already contain colons
- URN format (`costs:urn:aws:...`): Rejected as overly verbose; `/`-separated is more conventional and concise

## Decision 3: Concurrency Model

**Decision**: Use `DB.Batch()` for writes and `DB.View()` for reads.

**Rationale**:

- `DB.Batch()` automatically coalesces concurrent writes from multiple goroutines into fewer disk transactions
- For a CLI processing 10-100 resources, this means 10-100 cache writes likely collapse into 1-3 fsyncs
- `DB.View()` allows unlimited concurrent readers
- No need for custom write queues or channel-based batching; `Batch()` handles this internally
- The function passed to `Batch()` must be idempotent; cache Set operations are naturally idempotent

**Alternatives Considered**:

- `DB.Update()` per write: Rejected because each Update does a full fsync, making 100 writes = 100 fsyncs
- Custom write channel/queue: Rejected because `Batch()` already implements this pattern
- Read-write mutex wrapper: Rejected because bbolt handles its own concurrency

## Decision 4: TTL/Expiration Strategy

**Decision**: Check-on-read (lazy deletion) with startup cleanup.

**Rationale**:

- CLI processes are short-lived; background goroutines add complexity for no benefit
- Lazy deletion on read has zero overhead for non-expired entries
- Startup cleanup reclaims space from expired entries from previous runs
- Matches the existing FileStore's synchronous expiration pattern

**Alternatives Considered**:

- Background cleanup goroutine: Rejected for CLI; appropriate for long-running services only
- Eager deletion on every write: Rejected because it adds latency to the write path

## Decision 5: Corruption Recovery

**Decision**: Delete and recreate the database file on corruption detection.

**Rationale**:

- This is a cache; all data can be regenerated
- bbolt validates meta pages via checksum on `Open()`; if both are corrupt, `Open()` returns `ErrInvalid`
- Recovery is simple: remove file, retry `Open()`, reinitialize buckets
- No user data is at risk

**Error Types to Detect**:

- `berrors.ErrInvalid`: Both meta pages corrupt
- `berrors.ErrChecksum`: Checksum mismatch
- `berrors.ErrVersionMismatch`: Incompatible database version

## Decision 6: File Locking

**Decision**: Use bbolt's built-in file lock with a 500ms timeout. On timeout, degrade gracefully (disable caching).

**Rationale**:

- bbolt acquires exclusive file lock on `Open()`
- Without a timeout, a second CLI invocation would hang indefinitely
- 500ms is long enough for normal lock acquisition, short enough to not block the CLI
- Caching is best-effort; running without cache is always acceptable

**Alternatives Considered**:

- No timeout (block forever): Rejected; CLI tools must not hang
- ReadOnly mode for second process: Rejected because read-only prevents cache writes
- Separate database per process: Rejected; defeats purpose of shared cache

## Decision 7: Size Management

**Decision**: Check file size on startup; compact if file exceeds 2x estimated data or `maxSizeMB` limit.

**Rationale**:

- bbolt never shrinks its file; deleted pages are reused but not released
- `bbolt.Compact()` copies live data to a new file, reclaiming space
- Checking on startup is safe since no other transactions are running
- For a cache under 100MB, compaction completes in milliseconds

**Alternatives Considered**:

- No compaction: Rejected because file could grow unboundedly if entries churn
- Compact on every write: Rejected; too expensive
- LRU eviction: Deferred to future enhancement; startup compaction after cleanup is sufficient

## Decision 8: Interface Evolution

**Decision**: Extend the `Cache` interface with `Close() error` and `InvalidateByPrefix(prefix string) (int, error)`. Remove `FileStore` entirely (unreleased).

**Rationale**:

- `Close()` is required for bbolt to release file handles and flush data (FR-015)
- `InvalidateByPrefix()` enables targeted invalidation (FR-014)
- FileStore is unreleased; no backward compatibility concern
- Interface remains small (5 methods total)

**New Interface**:

```go
type Cache interface {
    Get(key string) (*CacheEntry, error)
    Set(key string, data json.RawMessage) error
    IsEnabled() bool
    Close() error
    InvalidateByPrefix(prefix string) (int, error)
}
```

## Decision 9: Database File Location

**Decision**: Store the database at `{projectDir}/.finfocus/cache.db` where `projectDir` is the resolved project directory (the directory containing `Pulumi.yaml`). Falls back to `~/.finfocus/cache.db` when no project context is available.

**Rationale**:

- Cache data is project-specific (different stacks have different resources and costs)
- Follows the existing two-tier config pattern: project-local for project-specific data, global for shared resources
- Consistent with `config.yaml` and `dismissed.json` which are already project-local
- The project `.finfocus/` directory already has auto-generated `.gitignore` protection
- Single file is easy to find, backup, and delete

**Resolution Precedence** (matches existing config resolution):

1. `FINFOCUS_CACHE_DIR` env var (explicit override)
2. `{projectDir}/.finfocus/` (project-local, when Pulumi.yaml found)
3. `~/.finfocus/` (global fallback, no project context)

**Alternatives Considered**:

- Global-only at `~/.finfocus/cache/`: Rejected because different projects would share cache entries, causing potential collisions and stale data across projects
- Subdirectory `{projectDir}/.finfocus/cache/finfocus.db`: Rejected as unnecessary nesting; `cache.db` at the `.finfocus/` level is clean and self-describing

## Decision 10: Removing FileStore

**Decision**: Remove `FileStore` entirely rather than maintaining two implementations.

**Rationale**:

- JSON file cache has not been released to users (per spec assumptions)
- Maintaining two implementations doubles test surface
- No migration needed since there are zero existing cache files
- BoltStore is a strict superset of FileStore capabilities
