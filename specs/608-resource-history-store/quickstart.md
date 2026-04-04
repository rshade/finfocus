# Quickstart: Resource History Store Implementation

**Branch**: `608-resource-history-store`

## Prerequisites

- Go 1.25.8+ (see `go.mod`)
- Existing BoltDB dependency (`go.etcd.io/bbolt`) already in `go.mod`
- Familiarity with `internal/engine/cache/store.go` (reference implementation)

## Implementation Order

### Step 1: History Store Core (internal/history/)

Start with the store — everything else depends on it.

```bash
# Create the package
mkdir -p internal/history

# Files to create:
# entry.go       - ResourceHistoryEntry struct + JSON serialization
# store.go       - HistoryStore interface + BoltStore implementation
# writer.go      - RecordStateSnapshot, RecordPlanLineage, RecordAnalyzerEvent
# reader.go      - GetResourcesForPeriod, GetCloudIdsForURN, GetDeletedResources
# retention.go   - CleanupExpired
# store_test.go  - Tests (write FIRST per constitution Principle II)
```

**Reference**: Mirror `internal/engine/cache/store.go` patterns for:

- Constructor: `NewBoltStore(ctx, directory, enabled, retentionDays)`
- Corruption recovery: detect → delete → recreate
- Lock timeout: 500ms graceful degradation
- Disabled mode: no-op when `enabled=false`

### Step 2: Config Extension (internal/config/)

Add `HistoryConfig` and `AllocationConfig` to `CostConfig`.

```bash
# Files to modify:
# budget.go  - Add History and Allocation fields to CostConfig
# merge.go   - No changes needed (cost key already handled)
```

The `unmarshalSection` for `keyCost` deserializes the entire `CostConfig`
struct, so adding new fields to `CostConfig` is automatically handled by
the YAML unmarshaler.

### Step 3: Engine Integration (internal/engine/)

Add `WithHistory()` following the `WithCache()` pattern.

```bash
# Files to modify:
# engine.go  - Add history field + WithHistory() method
```

### Step 4: Write Path — State Snapshot (internal/cli/)

Wire history writes into `cost_actual.go` and `overview.go`.

```bash
# Files to modify:
# common_execution.go  - Add initHistoryFromConfig() + newEngineWithHistory()
# cost_actual.go       - Record state snapshot after loading resources
# overview.go          - Record state snapshot after stack export
```

### Step 5: Write Path — Plan Lineage (internal/ingest/)

Process replace/delete operations and extract old cloud IDs.

```bash
# Files to modify:
# pulumi_plan.go  - Add ID field to PulumiState, process replace/delete ops
```

### Step 6: Read Path — Enhanced Actual Cost Query

Enrich resource descriptors with historical cloud IDs before billing queries.

```bash
# Files to modify:
# cost_actual.go  - Query history for additional cloud IDs
# engine.go       - Support multiple cloud IDs per resource
```

### Step 7: Write Path — Analyzer Events (internal/analyzer/)

Record resource observations during `pulumi up`.

```bash
# Files to modify:
# server.go  - Add history writer, record in Analyze()/AnalyzeStack()
```

## Verification at Each Step

```bash
# After every step:
make test    # All existing tests must pass
make lint    # Linting must pass

# History-specific tests:
go test -v ./internal/history/...
go test -v -coverprofile=cover.out ./internal/history/...
go tool cover -func=cover.out | grep total  # Must be >= 80%
```

## Key Design Decisions

1. **History is separate from cache**: `~/.finfocus/history/history.db`
   vs `~/.finfocus/cache/cache.db`
2. **PulumiState needs an ID field**: Add `ID string` to the struct
   (one-line change, maps the existing JSON `id` field)
3. **All write paths are fire-and-forget**: History write failures log
   warnings but never block the main operation
4. **Tag-based allocation (Layer 2) is deferred**: Config fields are
   defined but query logic is not implemented in this milestone
