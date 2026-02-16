# Tasks: BoltDB Cache Backend

**Input**: Design documents from `/specs/595-boltdb-cache/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add bbolt dependency and prepare the Cache interface for new capabilities.

- [X] T001 Add `go.etcd.io/bbolt` dependency to go.mod and run `go mod tidy`
- [X] T002 Update Cache interface to add `Close() error` and `InvalidateByPrefix(prefix string) (int, error)` methods, update interface compliance check, and remove FileStore implementation from internal/engine/cache/store.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented. Implements the BoltStore constructor, structured key generation, and CLI wiring.

- [X] T003 Replace SHA256 key generation with structured key builders (`BuildProjectedKey`, `BuildActualKey`, `BuildRecommendationsKey`, `BucketFromKey`) using `/`-separated hierarchical format in internal/engine/cache/key.go. Remove `GenerateKey`, `GenerateSimpleKey`, `GenerateKeyFromQuery`, `KeyParamsBuilder`, and `normalizeKeyParams`. Key format: `{bucket}/{provider}/{type}/{region}/{sku}` for projected, `{bucket}/{provider}/{type}/{from}/{to}/{filter-hash}` for actual, `{bucket}/multi/{sorted-types-hash}` for recommendations.
- [X] T004 Implement `BoltStore` struct (db, ttlSeconds, maxSizeMB, enabled fields) and `NewBoltStore(directory string, enabled bool, ttlSeconds, maxSizeMB int)` constructor in internal/engine/cache/store.go. Constructor must: open database at `{directory}/cache.db` with 500ms lock timeout, create 3 buckets (`projected`, `actual`, `recommendations`) via `CreateBucketIfNotExists`, detect corruption errors (`berrors.ErrInvalid`, `berrors.ErrChecksum`, `berrors.ErrVersionMismatch`) and recover by deleting and recreating the file, return `nil, nil` on lock timeout for graceful degradation, and implement `Close()` to release the database handle.
- [X] T005 Update CacheEntry JSON serialization to use Unix timestamps (int64) for CreatedAt and ExpiresAt fields for bbolt storage efficiency in internal/engine/cache/entry.go. Update `MarshalJSON`/`UnmarshalJSON` accordingly while keeping the public API (`time.Time` fields) unchanged.
- [X] T006 Update CLI cache initialization in internal/cli/common_execution.go: replace `cache.NewFileStore` with `cache.NewBoltStore`, resolve cache directory using project-dir resolution (`ResolveProjectDir` walk-up from CWD to find `Pulumi.yaml`, fallback to `~/.finfocus/`), handle `nil, nil` return for lock timeout graceful degradation, and wire `defer store.Close()` in `newEngineWithCache`.

**Checkpoint**: Foundation ready - BoltStore can be constructed, keys can be built, CLI wires the new store. User story implementation can now begin.

---

## Phase 3: User Story 1 - Fast Cost Lookups for Large Infrastructure (Priority: P1) MVP

**Goal**: Cache reads and writes work via BoltDB with structured keys, TTL expiration, and startup cleanup. All three caching strategies (projected, actual, recommendations) produce correct results through the new backend.

**Independent Test**: Run a cost command with `--cache-ttl 3600`, verify results are cached and subsequent lookups return `(cached)` results. Benchmark confirms <5ms lookup with 10K entries.

### Tests for User Story 1 (MANDATORY - TDD Required)

- [X] T007 [P] [US1] Write tests for key builders in internal/engine/cache/cache_test.go: test `BuildProjectedKey` produces `projected/{provider}/{type}/{region}/{sku}` format, test `BuildActualKey` produces deterministic keys with time ranges and filter hashes, test `BuildRecommendationsKey` produces deterministic keys for sorted resource types, test `BucketFromKey` extracts correct bucket name from structured keys, test key determinism (same inputs always produce same key).
- [X] T008 [P] [US1] Write tests for BoltStore Get/Set/IsEnabled in internal/engine/cache/cache_test.go: test Set then Get returns correct data, test Get with non-existent key returns `ErrCacheNotFound`, test Get with expired entry returns `ErrCacheExpired` and lazily deletes the entry, test IsEnabled returns correct state, test disabled store returns `ErrCacheDisabled` on Get and is no-op on Set, test Set with empty key returns `ErrInvalidCacheKey`, test multiple entries across different buckets are isolated.
- [X] T009 [P] [US1] Write benchmark `BenchmarkBoltStoreGet` in internal/engine/cache/cache_test.go: populate store with 10,000 entries across all 3 buckets, benchmark single Get operation, assert p50 < 5ms per SC-001.

### Implementation for User Story 1

- [X] T010 [US1] Implement `BoltStore.Get(key string)` in internal/engine/cache/store.go: parse bucket from key via `BucketFromKey`, use `db.View()` for read transaction, copy value bytes within transaction (bbolt values are only valid during tx), unmarshal `CacheEntry` from JSON, check TTL expiration and return `ErrCacheExpired` if expired (launch async `db.Batch()` delete for lazy cleanup), return `ErrCacheNotFound` if key or bucket doesn't exist.
- [X] T011 [US1] Implement `BoltStore.Set(key string, data json.RawMessage)` in internal/engine/cache/store.go: parse bucket from key, create `CacheEntry` with current time + TTL, marshal entry to JSON, use `db.Batch()` for concurrent write coalescing (function must be idempotent), validate key is non-empty, return `ErrCacheDisabled` if store is disabled.
- [X] T012 [US1] Implement `BoltStore.IsEnabled()` and `CleanupExpired() (int, error)` in internal/engine/cache/store.go. CleanupExpired iterates all 3 buckets using `Cursor`, collects keys with `ExpiresAt < now`, deletes them in a single `db.Update()` transaction per bucket, returns total count of removed entries. Called once during startup in `NewBoltStore`.
- [X] T013 [US1] Update all three engine key generation functions in internal/engine/engine.go: replace `generateProjectedCostResourceKey` to call `cache.BuildProjectedKey(resource.Provider, resource.Type, region, sku)` extracting region and sku from resource properties, replace `generateActualCostCacheKey` to call `cache.BuildActualKey(provider, resourceTypes, from, to, filters)` extracting time range and tags from request, replace `generateRecommendationsCacheKey` to call `cache.BuildRecommendationsKey(resourceTypes)`.
- [X] T014 [US1] Update engine cache test assertions in internal/engine/engine_cache_test.go: update `TestGenerateProjectedCostResourceKey` to expect structured `/`-separated keys instead of SHA256 hashes, update `TestGenerateActualCostCacheKey` to expect structured format with dates and filter hashes, update any mock cache tests to work with new key format, and verify that cache hits still append `(cached)` to the Adapter field per FR-008.

**Checkpoint**: User Story 1 complete. All cost commands (projected, actual, recommendations) correctly cache and retrieve results via BoltDB. Benchmark confirms <5ms lookup.

---

## Phase 4: User Story 2 - Cache Integrity During Interrupted Operations (Priority: P2)

**Goal**: Cache survives process crashes, concurrent access, corrupt files, and edge conditions without data loss or application failure.

**Independent Test**: Create a corrupt database file, run a cost command, verify it detects corruption, recreates cache, and completes normally. Run concurrent cache operations under race detector with zero failures.

### Tests for User Story 2 (MANDATORY - TDD Required)

- [X] T015 [P] [US2] Write test for corruption recovery and filesystem edge cases in internal/engine/cache/cache_test.go: create a file with garbage bytes at the cache.db path, call `NewBoltStore`, verify it logs a warning and successfully opens a fresh database with working buckets. Also test with truncated file and zero-byte file. Test directory with read-only permissions returns graceful degradation (`nil, nil` or disabled store, not a panic). Test that deleting the cache.db file while the store is open results in graceful error handling on subsequent Get/Set (log warning, return appropriate error, no panic).
- [X] T016 [P] [US2] Write test for concurrent read/write safety in internal/engine/cache/cache_test.go: launch 50 goroutines doing simultaneous Get and Set operations across all 3 buckets, verify zero errors and all written entries are retrievable. Run with `-race` flag.
- [X] T017 [P] [US2] Write test for lock timeout graceful degradation in internal/engine/cache/cache_test.go: open a BoltStore, then attempt to open a second BoltStore at the same path, verify the second returns `nil, nil` (not an error), and that the first store continues to function normally.

### Implementation for User Story 2

- [X] T018 [US2] Implement error handling for disk-full and I/O errors in `BoltStore.Set()` in internal/engine/cache/store.go: catch write errors from `db.Batch()`, log warning with zerolog including the key and error, return nil (skip cache write gracefully) so the cost command continues without caching. Same pattern for `CleanupExpired` errors.
- [X] T019 [US2] Implement error recovery in `BoltStore.Get()` for edge cases in internal/engine/cache/store.go: handle JSON unmarshal errors on corrupt entries (delete corrupt entry, return `ErrCacheNotFound`), handle missing bucket gracefully (return `ErrCacheNotFound`, not panic).

**Checkpoint**: User Story 2 complete. Cache integrity verified under corruption, concurrency, lock contention, and I/O failure scenarios.

---

## Phase 5: User Story 3 - Targeted Cache Invalidation (Priority: P3)

**Goal**: Users can selectively remove cached entries by key prefix (provider, resource type) without clearing the entire cache.

**Independent Test**: Populate cache with entries for multiple providers and resource types, invalidate by provider prefix, verify only that provider's entries are removed.

### Tests for User Story 3 (MANDATORY - TDD Required)

- [X] T020 [US3] Write tests for `InvalidateByPrefix` in internal/engine/cache/cache_test.go: test invalidate by provider prefix (`projected/aws/`) removes only AWS projected entries, test invalidate by resource type prefix (`projected/aws/ec2:Instance/`) removes only EC2 entries, test invalidate with no matches returns count 0 and no error, test empty prefix clears entire cache (all buckets), test invalidate on empty bucket returns 0 and no error, test invalidate on disabled store returns `ErrCacheDisabled`. Also write tests for `Delete` (single key) and `Clear` (all entries).

### Implementation for User Story 3

- [X] T021 [US3] Implement `InvalidateByPrefix(prefix string) (int, error)` in internal/engine/cache/store.go: parse bucket from prefix via `BucketFromKey`, if prefix has a bucket component use `Cursor.Seek()` + `bytes.HasPrefix()` to scan matching keys within that bucket and delete them in a single `db.Update()` transaction, if prefix is empty iterate all buckets and delete all entries (equivalent to Clear), return count of deleted entries.
- [X] T022 [P] [US3] Implement `BoltStore.Delete(key string) error` for single-key removal in internal/engine/cache/store.go: parse bucket from key, delete the key within that bucket in a `db.Update()` transaction, idempotent (no error if key doesn't exist).
- [X] T023 [P] [US3] Implement `BoltStore.Clear() error` to remove all entries from all buckets in internal/engine/cache/store.go: delete and recreate each of the 3 buckets within a single `db.Update()` transaction.

**Checkpoint**: User Story 3 complete. Targeted invalidation works by provider, resource type, or any key prefix combination.

---

## Phase 6: User Story 4 - Reduced Disk Footprint and Efficient Storage (Priority: P4)

**Goal**: Cache uses a single database file with size management through compaction.

**Independent Test**: Populate cache, verify single file exists in `.finfocus/` directory, verify Size() and Count() return correct values, verify Compact() reduces file size after bulk deletions.

### Tests for User Story 4 (MANDATORY - TDD Required)

- [X] T024 [US4] Write tests for Size/Count/Compact in internal/engine/cache/cache_test.go: test `Size()` returns positive value after entries stored, test `Count()` returns correct total across all buckets, test `Compact()` produces a valid database with all entries preserved, test that cache directory contains only `cache.db` (no individual JSON files), test that startup size check triggers CleanupExpired then Compact when file exceeds maxSizeMB and logs a warning.

### Implementation for User Story 4

- [X] T025 [P] [US4] Implement `BoltStore.Size() (int64, error)` and `BoltStore.Count() (int, error)` in internal/engine/cache/store.go. Size returns `os.Stat(db.Path()).Size()`. Count iterates all 3 buckets using `Bucket.Stats().KeyN` for O(1) key count per bucket.
- [X] T026 [US4] Implement `BoltStore.Compact() error` in internal/engine/cache/store.go: create temporary database file, use `bbolt.Compact(dst, src, 65536)` to copy live data, close source, rename temp to source path, reopen database and reinitialize buckets. Add startup size check in `NewBoltStore`: if file size exceeds `maxSizeMB`, run `CleanupExpired` then `Compact`.

**Checkpoint**: User Story 4 complete. Single-file storage with compaction-based size management verified.

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, test coverage verification, and final quality gates.

- [X] T027 [P] Update package documentation in internal/engine/cache/doc.go with BoltStore description, bucket layout, key format examples, and concurrency model
- [X] T028 [P] Update CLI initialization tests for BoltStore constructor and project-dir resolution in internal/cli/init_cache_test.go
- [X] T029 [P] Update CLAUDE.md cache architecture section: replace FileStore references with BoltStore, document bucket structure, key format, project-local storage at `{projectDir}/.finfocus/cache.db`, and add `cache.db` to `.gitignore` template in `EnsureGitignore`
- [X] T030 Run `make lint` and `make test` to verify all quality gates pass
- [X] T031 Run `make test-race` to verify no race conditions with BoltStore concurrent access

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion
  - User stories can proceed sequentially in priority order (P1 → P2 → P3 → P4)
  - US2 benefits from US1 being complete (tests validate US1's implementation)
  - US3 and US4 are independent of each other
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2). No dependencies on other stories. This is the MVP.
- **User Story 2 (P2)**: Can start after US1 (tests verify US1 implementation integrity). Independently testable.
- **User Story 3 (P3)**: Can start after Foundational (Phase 2). No dependencies on US1/US2. Independently testable.
- **User Story 4 (P4)**: Can start after Foundational (Phase 2). No dependencies on US1/US2/US3. Independently testable.

### Within Each User Story

- Tests MUST be written first and FAIL before implementation begins
- Core operations (Get/Set) before engine integration
- Engine integration before test assertion updates
- Story complete before moving to next priority

### Parallel Opportunities

Within US1 tests: T007, T008, T009 can all run in parallel (separate test functions, same file)
Within US2 tests: T015, T016, T017 can all run in parallel
Within US3 implementation: T022, T023 can run in parallel (different methods, same file)
Within US4 implementation: T025 parallel with other stories
Polish phase: T027, T028, T029 can all run in parallel (different files)

---

## Parallel Example: User Story 1

```text
# Tests (parallel - write all tests first, verify they fail):
T007: Key builder tests in cache_test.go
T008: BoltStore Get/Set/TTL tests in cache_test.go
T009: Benchmark <5ms in cache_test.go

# Implementation (sequential - core ops first, then engine wiring):
T010: BoltStore.Get() in store.go
T011: BoltStore.Set() in store.go
T012: IsEnabled + CleanupExpired in store.go
T013: Engine key generation functions in engine.go
T014: Engine cache test assertions in engine_cache_test.go
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T006)
3. Complete Phase 3: User Story 1 (T007-T014)
4. **STOP and VALIDATE**: Run `make test`, `make lint`, verify cache works end-to-end
5. This delivers the core value: fast BoltDB-backed caching for all cost commands

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → **MVP complete** (fast lookups work)
3. Add User Story 2 → Test independently → Integrity guarantees verified
4. Add User Story 3 → Test independently → Targeted invalidation available
5. Add User Story 4 → Test independently → Size management active
6. Polish → Documentation, final quality gates
7. Each story adds capability without breaking previous stories
