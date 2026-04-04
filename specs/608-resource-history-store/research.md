# Research: Resource History Store

**Date**: 2026-03-30
**Branch**: `608-resource-history-store`

## Decision 1: History Store Backend

**Decision**: Use BoltDB (bbolt), following the exact patterns from
`internal/engine/cache/store.go`.

**Rationale**: BoltDB is already a proven dependency in this project. The
cache store demonstrates established patterns for corruption recovery, lock
timeout handling, startup cleanup, compaction, and graceful degradation. Using
the same backend avoids introducing new dependencies and lets the team apply
known operational knowledge.

**Alternatives considered**:

- SQLite: More powerful queries but heavier dependency, cross-compilation
  complexity on Windows/ARM
- Flat JSON files: Simpler but no transactional safety, poor performance for
  prefix scans
- Badger: Better write throughput but larger dependency, less established in
  this codebase

## Decision 2: PulumiState.ID Field Gap

**Decision**: Add `ID string` field to `PulumiState` struct in
`internal/ingest/pulumi_plan.go`.

**Rationale**: The issue design references `OldState.ID` for plan lineage
extraction, but the current `PulumiState` struct does not map the `id` JSON
field. The raw Pulumi plan JSON output includes `id` on both `oldState` and
`newState` objects for replace/delete operations. The sister struct
`StackExportResource` already maps this field. Adding it to `PulumiState`
is a one-line change with zero breaking impact.

**Alternatives considered**:

- Extract from Outputs map: Works for some resources but not all providers
  consistently populate `id` in outputs
- Keep current struct, add separate extraction: More complex, duplicates
  logic already solved by JSON deserialization

## Decision 3: History Store Location and Separation

**Decision**: Store at `~/.finfocus/history/history.db`, completely separate
from the cache at `~/.finfocus/cache/cache.db`.

**Rationale**: The constitution (v1.7.0) explicitly documents this as a
permitted store with the note that history is "delete-safe" only in the sense
that accuracy degrades — unlike cache which rebuilds on next run. Separate
files prevent accidental deletion of history when users clear cache, and allow
independent backup/corruption-recovery.

**Alternatives considered**:

- Same BoltDB file with separate buckets: Couples lifecycle of ephemeral
  cache with persistent history
- Subdirectory under cache: Confusing semantics (users might delete the
  whole cache directory)

## Decision 4: History Key Schema

**Decision**: Use `{stack_hash}/{urn_hash}/{cloud_id}` as composite key with
SHA-256 hashing for stack and URN components.

**Rationale**: Hashing keeps keys fixed-length for efficient B-tree lookups.
Using cloud_id as the final segment (unhashed) enables human-readable
debugging. Prefix scans on `{stack_hash}/{urn_hash}/` retrieve all
incarnations of a resource. Prefix scans on `{stack_hash}/` retrieve all
resources in a stack.

**Alternatives considered**:

- Unhashed URNs as keys: URNs can be very long (200+ chars), wasting B-tree
  space
- Numeric auto-increment keys with secondary index: More complex, requires
  maintaining index consistency
- Cloud ID as primary key: Loses the URN-to-cloudID grouping needed for
  lineage queries

## Decision 5: History Interface Pattern

**Decision**: Define a `HistoryStore` interface following the `cache.Cache`
interface pattern, inject via `Engine.WithHistory()` functional option.

**Rationale**: The Engine already uses this pattern for cache, router, and
dismissal store. A consistent approach means the history store is optional
(commands work without it), testable (mock in unit tests), and follows the
constitution's stateless-core principle.

**Alternatives considered**:

- Global singleton: Violates testability, complicates concurrent tests
- Direct dependency in Engine struct: Loses optionality, breaks existing
  constructor signature

## Decision 6: Write Path Ordering

**Decision**: Implement write paths in this order:
1. State snapshot (P0, every run)
2. Plan lineage (P0, overview/preview)
3. Analyzer events (P1, pulumi up)

**Rationale**: State snapshot is the simplest and most impactful write path —
it captures the current cloud IDs for all resources. Plan lineage captures
the critical replace/delete operations that state snapshots miss. Analyzer
events are the least common write path (only during `pulumi up`) and require
the most careful handling (DryRun flag checks, post-provisioning timing).

**Alternatives considered**:

- All three simultaneously: Higher implementation risk, harder to test
  incrementally
- Analyzer first: Less immediate user value since `pulumi up` with analyzer
  is a less common workflow than `cost actual`

## Decision 7: Tag-Based Allocation Scope

**Decision**: Tag-based allocation (Layer 2) is a separate milestone from the
history store (Layer 1). The config fields are defined and validated in Layer
1, but the actual tag query logic is deferred.

**Rationale**: Tag-based queries require provider-specific setup (AWS Cost
Allocation Tags activation has a 24h delay). Shipping the history store first
provides immediate value for the replacement cost gap. The proto already has a
`Tags` field on `GetActualCostRequest`, so the plugin interface is ready when
we implement Layer 2.

**Alternatives considered**:

- Ship both together: Doubles scope, delays the more impactful Layer 1
- Skip tags entirely: Leaves the cold-start problem unsolved permanently

## Decision 8: Retention Cleanup Strategy

**Decision**: Run retention cleanup on startup (same as cache), comparing
`last_seen` timestamp against `now - retention_days`.

**Rationale**: Startup cleanup is a proven pattern from the cache store.
Running it once per process avoids background goroutines and timer complexity.
The `last_seen` field is the natural choice since it represents when finfocus
last observed the resource — entries not seen for 90+ days are unlikely to be
queried.

**Alternatives considered**:

- Background timer: More complex, unnecessary for typical usage patterns
  (finfocus runs intermittently, not as a daemon)
- No automatic cleanup: History grows unbounded, eventually causing
  performance issues

## Decision 9: Concurrent Access Safety

**Decision**: Use BoltDB's built-in MVCC for concurrent reads and serialized
writes, with the same lock timeout pattern as the cache store (500ms).

**Rationale**: BoltDB supports unlimited concurrent readers with a single
writer. This matches finfocus's usage: multiple goroutines may read history
during parallel cost queries, while writes happen sequentially during state
snapshot recording. The 500ms lock timeout prevents hangs when another finfocus
process holds the lock.

**Alternatives considered**:

- Application-level file locking: Redundant with BoltDB's built-in locking
- Read-write mutex wrapper: Unnecessary since BoltDB already provides this
  at the transaction level

## Decision 10: Overview State Loading

**Decision**: History writes in overview.go inject after
`LoadStackExportWithContext()` returns and before plan generation begins.

**Rationale**: At this point, the full state with cloud IDs is available.
Writing before plan generation means even if the preview fails or times out,
the state snapshot is already recorded. The overview flow calls `pulumi stack
export` via subprocess, then `pulumi preview --json` — the state export is the
reliable data source.

**Alternatives considered**:

- After plan generation: Risks losing state data if preview fails
- In a separate goroutine: Adds complexity for minimal benefit (BoltDB write
  is fast, typically <5ms)
