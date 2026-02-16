# Implementation Plan: BoltDB Cache Backend

**Branch**: `595-boltdb-cache` | **Date**: 2026-02-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/595-boltdb-cache/spec.md`
**GitHub Issue**: #674

## Summary

Replace the unreleased JSON file-based cache (`FileStore`) with a BoltDB (`go.etcd.io/bbolt`) backend. The new `BoltStore` uses 3 buckets (projected, actual, recommendations) for scoped lookups, structured human-readable keys for debuggability and prefix scanning, and `DB.Batch()` for efficient concurrent writes. Targeted invalidation by key prefix is added as a new capability. Since the JSON cache is unreleased, `FileStore` is removed entirely with no migration needed.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: `go.etcd.io/bbolt` (new), existing deps unchanged
**Storage**: BoltDB single-file B+tree KV store at `{projectDir}/.finfocus/cache.db`
**Testing**: `go test` with `testify/assert` and `testify/require`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application
**Performance Goals**: <5ms cache lookup for up to 10,000 entries (SC-001)
**Constraints**: maxSizeMB default 100MB, TTL range [60s, 7d], file lock timeout 500ms
**Scale/Scope**: Up to 10,000 cache entries across 3 buckets

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is core orchestration infrastructure (cache layer), not a cost data source. No plugin boundary is crossed.
- [x] **Test-Driven Development**: Tests are planned before implementation. Target 80% minimum coverage, 95% for cache operations (critical path). Benchmarks for SC-001.
- [x] **Cross-Platform Compatibility**: bbolt is pure Go with no CGo dependencies. Uses `os.Rename` for atomicity (works on all platforms). File locking uses OS-native primitives via bbolt internals.
- [x] **Documentation Integrity**: CLAUDE.md cache section will be updated. Package godoc for new `BoltStore` type and all exported functions.
- [x] **Protocol Stability**: No protocol buffer changes. This is entirely internal to finfocus-core.
- [x] **Implementation Completeness**: Full implementation with no stubs. All 15 functional requirements addressed.
- [x] **Quality Gates**: `make lint` and `make test` will pass. Benchmarks added for performance validation.
- [x] **Multi-Repo Coordination**: None needed. Change is entirely within finfocus-core. No spec or plugin changes.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/595-boltdb-cache/
├── plan.md              # This file
├── research.md          # Phase 0: bbolt best practices and decisions
├── data-model.md        # Phase 1: entity definitions and key schema
├── quickstart.md        # Phase 1: developer getting started guide
├── contracts/           # Phase 1: interface contracts
│   └── cache-interface.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/engine/cache/
├── store.go             # REPLACE: FileStore → BoltStore implementation
├── entry.go             # MODIFY: update JSON serialization for Unix timestamps
├── key.go               # REPLACE: SHA256 keys → structured human-readable keys
├── ttl.go               # KEEP: TTL validation unchanged
├── doc.go               # MODIFY: update package documentation
└── cache_test.go        # REPLACE: all tests rewritten for BoltStore

internal/engine/
├── engine.go            # MODIFY: update key generation functions (3 functions)
└── engine_cache_test.go # MODIFY: update key generation test assertions

internal/cli/
├── common_execution.go  # MODIFY: NewFileStore → NewBoltStore, add Close()
├── init_cache_test.go   # MODIFY: update initialization tests
└── root.go              # KEEP: flag registration unchanged

internal/config/
└── cache_defaults.go    # KEEP: constants unchanged

go.mod                   # MODIFY: add go.etcd.io/bbolt dependency
```

**Structure Decision**: This is a backend replacement within the existing `internal/engine/cache/` package. No new packages or directories are created in the source tree. The change is contained within 8 files (5 replace/rewrite, 3 modify).

### Files Changed Summary

| File | Action | Reason |
|------|--------|--------|
| `internal/engine/cache/store.go` | Replace | FileStore → BoltStore with bucket support |
| `internal/engine/cache/key.go` | Replace | SHA256 → structured human-readable keys |
| `internal/engine/cache/entry.go` | Modify | Unix timestamp storage for bbolt efficiency |
| `internal/engine/cache/cache_test.go` | Replace | All tests rewritten for BoltStore |
| `internal/engine/cache/doc.go` | Modify | Package documentation update |
| `internal/engine/engine.go` | Modify | 3 key generation functions updated |
| `internal/engine/engine_cache_test.go` | Modify | Key generation test assertions updated |
| `internal/cli/common_execution.go` | Modify | NewFileStore → NewBoltStore, defer Close() |
| `internal/cli/init_cache_test.go` | Modify | Initialization tests updated |
| `go.mod` | Modify | Add bbolt dependency |

### Files Unchanged

| File | Reason |
|------|--------|
| `internal/engine/cache/ttl.go` | TTL validation rules unchanged |
| `internal/config/cache_defaults.go` | Configuration constants unchanged |
| `internal/cli/root.go` | Flag registration unchanged |
| `internal/cli/cost_projected.go` | Uses engine interface, no direct cache access |
| `internal/cli/cost_actual.go` | Uses engine interface, no direct cache access |
| `internal/cli/cost_recommendations.go` | Uses engine interface, no direct cache access |

## Key Design Decisions

### 1. Bucket Strategy

Three top-level bbolt buckets: `projected`, `actual`, `recommendations`. Created on database `Open()`. No nested buckets.

**Why**: Natural namespace isolation per FR-010. Lookups traverse only same-type entries (SC-007). Clearing one type doesn't touch others.

### 2. Key Schema

`/`-separated hierarchical keys. First segment is bucket name, remaining segments encode resource identity.

| Bucket | Key Format | Example |
|--------|------------|---------|
| projected | `projected/{provider}/{type}/{region}/{sku}` | `projected/aws/ec2:Instance/us-east-1/t3.micro` |
| actual | `actual/{provider}/{type}/{from}/{to}/{filter-hash}` | `actual/aws/ec2:Instance/2025-01-01/2025-02-01/a3f2b1` |
| recommendations | `recommendations/multi/{sorted-types-hash}` | `recommendations/multi/ec2+rds` |

**Why**: Human-readable, supports prefix scanning via `Cursor.Seek()`, and field ordering matches query patterns (provider > type > region).

### 3. Concurrency

- Reads: `DB.View()` (unlimited concurrent readers)
- Writes: `DB.Batch()` (automatic coalescing of concurrent writes into fewer fsyncs)

**Why**: `Batch()` turns N concurrent cache writes from N goroutines into ~1-3 disk transactions. Critical for CLI processing hundreds of resources concurrently.

### 4. Interface Evolution

Extended `Cache` interface with 2 new methods:

```go
type Cache interface {
    Get(key string) (*CacheEntry, error)     // existing
    Set(key string, data json.RawMessage) error  // existing
    IsEnabled() bool                         // existing
    Close() error                            // NEW (FR-015)
    InvalidateByPrefix(prefix string) (int, error)  // NEW (FR-014)
}
```

**Why**: `Close()` is mandatory for bbolt file handle release. `InvalidateByPrefix()` enables targeted invalidation. FileStore is removed (unreleased), so no backward compatibility concern.

### 5. Error Handling

- Corruption on Open(): delete file, recreate database, log warning
- Lock timeout (500ms): return nil store, engine runs without cache
- Disk full on write: log warning, skip cache write, command continues
- Expired entry on read: return `ErrCacheExpired`, async `db.Batch()` delete for lazy cleanup

### 6. Database Lifecycle

```text
CLI startup
  → initCacheFromConfig(ctx, cmd, cfg)
    → Resolve project dir (Pulumi.yaml walk-up, fallback to ~/.finfocus/)
    → NewBoltStore(projectDir/.finfocus/, enabled, ttl, maxSize)
      → bbolt.Open(cache.db, 0600, {Timeout: 500ms})
      → initBuckets (CreateBucketIfNotExists × 3)
      → CleanupExpired() on startup
  → defer store.Close()
  → engine.WithCache(store)
  → ... cost calculations with cache ...
CLI shutdown
  → store.Close()
    → db.Close() releases file lock
```
