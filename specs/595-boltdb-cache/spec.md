# Feature Specification: Transition Persistent Cache to Single-File KV Store (BoltDB)

**Feature Branch**: `595-boltdb-cache`
**Created**: 2026-02-16
**Status**: Draft
**Input**: User description: "Transition persistent cache from JSON to BoltDB (bbolt) - Replace the existing JSON file-based cache with a BoltDB single-file KV store for faster lookups, atomic updates, and reduced disk I/O."
**GitHub Issue**: #674

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Fast Cost Lookups for Large Infrastructure (Priority: P1)

A user managing a large cloud infrastructure project (hundreds to thousands of resources) runs a cost calculation command. The system retrieves previously cached cost data near-instantly from a single database file, rather than scanning hundreds of individual cache files on disk. The user experiences significantly faster command response times.

**Why this priority**: This is the core value proposition. Users with large stacks are the primary beneficiaries, and cache read performance directly impacts every cached cost operation (projected, actual, and recommendations).

**Independent Test**: Can be fully tested by running a cost command against a project with many cached resources and measuring lookup latency. Delivers immediate value through faster response times.

**Acceptance Scenarios**:

1. **Given** a project with 500+ previously cached resource costs, **When** the user runs a cost calculation command, **Then** cached results are retrieved and displayed within the same time frame regardless of cache size (no linear scan degradation).
2. **Given** a cold start with an empty cache, **When** the user runs a cost calculation for the first time, **Then** results are computed normally and stored in the cache for future lookups.
3. **Given** a cached entry that has expired (TTL exceeded), **When** the user runs a cost calculation, **Then** the expired entry is not returned and a fresh calculation is performed.

---

### User Story 2 - Cache Integrity During Interrupted Operations (Priority: P2)

A user's cost scan is interrupted mid-way (e.g., network failure, process termination, system crash). When the user re-runs the command, the cache is in a consistent state - partially written entries do not corrupt previously cached data, and the cache continues to function normally.

**Why this priority**: Data integrity is critical for trust in cached results. Users should never encounter corrupt cache states that require manual intervention.

**Independent Test**: Can be tested by simulating process interruption during cache write operations and verifying cache consistency on restart.

**Acceptance Scenarios**:

1. **Given** a cache write operation is in progress, **When** the process is forcefully terminated, **Then** the cache file remains consistent and previously cached entries are preserved.
2. **Given** multiple concurrent cost calculations writing to the cache, **When** all operations complete, **Then** all entries are correctly stored without data loss or corruption.
3. **Given** a corrupted cache file (e.g., disk error), **When** the user runs a cost command, **Then** the system detects the corruption, reports a warning, and recreates the cache from scratch rather than crashing.

---

### User Story 3 - Targeted Cache Invalidation (Priority: P3)

A user needs to selectively clear cached data for specific resources without wiping the entire cache. For example, after changing an EC2 instance type in their infrastructure code, they want to invalidate only EC2-related cache entries so that fresh costs are fetched on the next run, while keeping all other cached data intact.

**Why this priority**: Targeted invalidation provides precise cache control. Without it, users must choose between stale data or clearing everything. This is especially valuable during iterative infrastructure changes where only a subset of resources change between runs.

**Independent Test**: Can be tested by populating the cache with entries across multiple resource types, invalidating entries for one type, and verifying only the targeted entries were removed while others remain.

**Acceptance Scenarios**:

1. **Given** a cache containing entries for multiple resource types (EC2, Lambda, S3), **When** the user invalidates entries matching a specific resource type prefix, **Then** only entries for that resource type are removed and all other entries remain accessible.
2. **Given** a cache containing entries for multiple providers, **When** the user invalidates all entries for a specific provider, **Then** only that provider's entries are removed.
3. **Given** an invalidation request with a prefix that matches no entries, **When** the invalidation runs, **Then** no entries are removed and the operation completes without error.

---

### User Story 4 - Reduced Disk Footprint and Efficient Storage (Priority: P4)

A user with a large number of cached entries observes that the cache uses a single compact file instead of hundreds of individual files. This reduces filesystem overhead, simplifies backup/restore, and plays nicely with file-watching tools and backup software.

**Why this priority**: While not a primary driver, reduced filesystem clutter is a quality-of-life improvement that benefits all users, especially those on filesystems with inode limits or slow metadata operations (e.g., networked filesystems).

**Independent Test**: Can be tested by caching results for many resources and comparing the number of filesystem entries (single file vs. many files).

**Acceptance Scenarios**:

1. **Given** cost data for 1000 resources has been cached, **When** the user inspects the cache directory, **Then** the cache is stored in a single database file (plus any lock file the storage engine requires).
2. **Given** the cache file has exceeded its configured maximum size, **When** the cache store is opened on the next command invocation, **Then** the system cleans up expired entries and compacts the database to reclaim space, logging a warning about the size threshold.

---

### Edge Cases

- What happens when the cache file is locked by another process (e.g., two CLI invocations running simultaneously)? The system should wait briefly for the lock or proceed without caching.
- What happens when the disk is full and a cache write is attempted? The system should log a warning and continue without caching rather than failing the cost command.
- What happens when the user manually deletes the cache file while the tool is running? The system should detect the missing file and recreate it on the next write.
- What happens when the cache database file exceeds the configured maximum size? The system cleans up expired entries and compacts the database on startup to reclaim space, logging a warning.
- What happens when a targeted invalidation prefix is overly broad (e.g., matches everything)? The system should behave identically to a full cache clear.
- What happens when targeted invalidation is called on an empty cache or empty bucket? The operation should complete without error.
- What happens when the cache directory permissions prevent file creation? The system should disable caching and log a warning rather than crashing.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST store and retrieve cost data using unique cache keys derived from resource properties (provider, type, region, SKU, and additional attributes).
- **FR-002**: System MUST support time-based expiration (TTL) for all cache entries, with expired entries not returned during lookups.
- **FR-003**: System MUST store the entire cache in a single database file within the project's local configuration directory (the `.finfocus/` directory adjacent to `Pulumi.yaml`), falling back to the global `~/.finfocus/` directory when no project context is available.
- **FR-004**: System MUST provide atomic write operations so that interrupted writes do not corrupt existing cache data.
- **FR-005**: System MUST support concurrent read access from multiple goroutines without data corruption.
- **FR-006**: System MUST maintain full backward compatibility with all existing cache configuration options (TTL settings, cache directory, max size, enable/disable, environment variables, and CLI flags).
- **FR-007**: System MUST support all three caching strategies: per-resource caching (projected costs), whole-query caching (actual costs), and recommendation caching.
- **FR-008**: System MUST append the "(cached)" indicator to results retrieved from cache, consistent with current behavior.
- **FR-009**: System MUST clean up expired entries to prevent unbounded database growth.
- **FR-010**: System MUST organize cache entries into separate namespaces (buckets) by cost calculation type: projected costs, actual costs, and recommendations. Each bucket is scoped independently so that lookups only traverse entries of the same type.
- **FR-011**: System MUST handle cache file corruption gracefully by recreating the cache rather than crashing.
- **FR-012**: System MUST support the existing cache maintenance operations: delete single entry, clear all entries, and clean up expired entries.
- **FR-013**: System MUST use structured, human-readable cache keys that encode resource identity (provider, resource type, and distinguishing attributes such as region and SKU). Keys MUST support prefix-based lookups and invalidation.
- **FR-014**: System MUST support targeted invalidation of cache entries by key prefix, allowing selective removal of entries matching a provider, resource type, or other key component without clearing the entire cache.
- **FR-015**: System MUST support closing the cache store to release file handles and flush pending writes on shutdown.

### Key Entities

- **Cache Entry**: A stored cost result with metadata including the cache key, serialized cost data, creation timestamp, expiration timestamp, and TTL value.
- **Cache Store**: The persistent storage backend that manages cache entries, enforces TTL policies, and provides atomic read/write operations through a single database file.
- **Cache Bucket**: A logical namespace within the cache store that groups entries by cost calculation type (projected, actual, recommendations). Buckets are scoped independently so lookups only traverse same-type entries.
- **Cache Key**: A structured, human-readable identifier derived from resource properties and query parameters. Keys encode provider, resource type, and distinguishing attributes, enabling prefix-based lookups and targeted invalidation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Cache lookup for a single resource completes in under 5 milliseconds, regardless of total cache size (up to 10,000 entries).
- **SC-002**: Cache remains fully consistent and operational after simulated process interruption during write operations (zero data corruption incidents).
- **SC-003**: All existing cost commands (projected, actual, recommendations) continue to function identically with the new cache backend - no user-visible behavior changes except improved performance.
- **SC-004**: Cache storage uses a single file instead of N files for N entries, reducing filesystem entry count to constant overhead.
- **SC-005**: All existing cache configuration options (CLI flags, environment variables, config file settings) work without modification.
- **SC-006**: Targeted invalidation by resource type prefix removes only matching entries; all non-matching entries remain accessible and valid.
- **SC-007**: Cache lookups scoped to a single bucket (e.g., projected costs) do not traverse entries in other buckets (e.g., actual costs), maintaining constant-time performance independent of total cache size across all buckets.

## Assumptions

- The JSON-based file cache has not been released to users. There are zero existing cache files in the wild, so no migration is needed and the key schema can be designed from scratch.
- The existing `Cache` interface (`Get`, `Set`, `IsEnabled`) may be extended to support new capabilities (targeted invalidation, close/shutdown) while remaining backward compatible for existing callers.
- The maximum practical cache size is bounded by the existing `maxSizeMB` configuration (default 100 MB).
- Concurrent access patterns include multiple goroutines within a single process; cross-process concurrent write access is handled by the storage engine's native file locking.
- The existing TTL validation rules (minimum 60 seconds, maximum 7 days, default 1 hour) remain unchanged.

## Scope Boundaries

### In Scope

- New cache storage backend implementation using a single-file KV store
- Bucket-based organization separating projected, actual, and recommendation cache entries
- Structured, human-readable cache key schema replacing SHA256 hashes
- Targeted cache invalidation by key prefix (provider, resource type)
- All existing cache operations (get, set, delete, clear, cleanup)
- Thread-safe concurrent access
- Atomic write operations for crash safety
- Cache file corruption detection and recovery

### Out of Scope

- Migration of existing JSON cache entries (unreleased feature, no users to migrate)
- Changes to TTL configuration or validation rules
- Changes to how the engine uses the cache (caching strategies remain the same)
- Remote or distributed caching
- Cache compression or encryption
- New CLI commands for cache management beyond existing capabilities
