# Test Contracts: Integration Test Suite

**Feature**: #602 Integration Test Suite Expansion
**Date**: 2026-02-22

## Contract 1: Mock Plugin Error Injection Interface

The mock plugin MUST support the following error injection behaviors:

```go
// Extended scenario types for test/mocks/plugin/config.go
type ErrorInjection struct {
    // ExitMidCall causes the plugin process to exit during handler execution.
    ExitMidCall bool

    // SleepDuration causes the handler to sleep for the specified duration.
    // If longer than the client context deadline, triggers timeout.
    SleepDuration time.Duration

    // FailForTypes causes errors for specific resource types while
    // succeeding for others (partial failure testing).
    FailForTypes []string
}
```

### Contract Guarantees

- `ExitMidCall=true` MUST cause the engine to return `ErrCodePluginError`
- `SleepDuration > deadline` MUST cause the engine to return `ErrCodeTimeoutError`
- `FailForTypes` MUST return errors only for matching types; others succeed
- No error injection MUST cause a panic in the engine

## Contract 2: Cache Behavior Across CLI-Engine Boundary

### Cache Hit Contract

```text
GIVEN: A plan file P and mock plugin M
WHEN:  `cost projected --pulumi-json P` runs twice
THEN:  Second run MUST have "(cached)" in adapter field for all resources
AND:   Second run MUST NOT call plugin M for cached resources
```

### TTL Expiry Contract

```text
GIVEN: Cache TTL = T seconds
WHEN:  Entry was written at time W and current time > W + T
THEN:  Get() MUST return cache miss
AND:   Next cost command MUST call plugin for fresh data
```

### Corruption Recovery Contract

```text
GIVEN: cache.db contains invalid bytes
WHEN:  Next cache operation is attempted
THEN:  System MUST delete corrupted file
AND:   System MUST create new empty database
AND:   Operation MUST succeed (cache miss, plugin called)
AND:   No panic or unrecoverable error
```

### Precedence Contract

```text
GIVEN: --cache-ttl=60 (CLI flag)
AND:   FINFOCUS_CACHE_TTL=120 (env var)
AND:   config.yaml cache_ttl: 180 (config file)
WHEN:  Cache is initialized
THEN:  Effective TTL MUST be 60 (CLI flag wins)
```

## Contract 3: Analyzer Concurrency Guarantees

### Large Stack Contract

```text
GIVEN: A synthetic stack with N=100 resources
WHEN:  AnalyzeStack is called with mock plugin (zero latency)
THEN:  Response MUST complete within 10 seconds
AND:   Response MUST contain N diagnostics (one per resource)
AND:   No goroutine leaks (verified via runtime.NumGoroutine delta)
```

### Concurrent Calls Contract

```text
GIVEN: 5 goroutines each calling Analyze() concurrently
WHEN:  Executed with -race flag
THEN:  All 5 MUST return valid diagnostics
AND:   Race detector MUST report zero races
AND:   Cost cache MUST be consistent after all calls complete
```

### Partial Failure Contract

```text
GIVEN: Mock plugin errors for type A, succeeds for type B
WHEN:  AnalyzeStack processes resources of both types
THEN:  Type A resources MUST have WARNING diagnostics
AND:   Type B resources MUST have cost estimate diagnostics
AND:   Overall AnalyzeStack call MUST succeed (not error)
```

## Contract 4: Config Precedence Chain

### Resolution Order Contract

```text
Priority (highest to lowest):
1. --project-dir CLI flag
2. FINFOCUS_PROJECT_DIR environment variable
3. Walk-up from CWD to find Pulumi.yaml, use $PROJECT/.finfocus/
4. Fall back to ~/.finfocus/ (global)

MUST: Each level overrides all lower levels completely.
MUST: Walk-up traverses parent directories until filesystem root.
MUST: Malformed YAML returns descriptive error, not panic.
```

### Shallow Merge Contract

```text
GIVEN: Global config has keys {output, plugins, logging}
AND:   Project config has keys {output, analyzer}
WHEN:  Configs are merged
THEN:  Result MUST have:
  - output: from project config (replaced entirely)
  - plugins: from global config (inherited)
  - logging: from global config (inherited)
  - analyzer: from project config (new key added)
```

## Contract 5: Concurrency Correctness

### Determinism Contract

```text
GIVEN: Plan file P with N resources
WHEN:  `cost projected -j 1 --pulumi-json P` produces result R1
AND:   `cost projected -j 8 --pulumi-json P` produces result R2
THEN:  Sum(R1.monthly_costs) MUST equal Sum(R2.monthly_costs)
AND:   len(R1.resources) MUST equal len(R2.resources)
AND:   Set(R1.resource_types) MUST equal Set(R2.resource_types)
```

### Concurrent Cache Access Contract

```text
GIVEN: 5 parallel processes sharing cache.db
WHEN:  All run `cost projected` simultaneously
THEN:  No BoltDB "database is locked" fatal errors
AND:   cache.db MUST NOT be corrupted after all processes complete
AND:   Each process MUST complete (may degrade to uncached mode)
```

## Contract 6: Build Tag Promotion

### Promoted Tests Contract

```text
GIVEN: Tests promoted from nightly to integration tag
WHEN:  CI runs with -tags integration
THEN:  All promoted tests MUST pass
AND:   No external dependencies (no cloud credentials, no binary builds)
AND:   Execution time < 5 seconds per promoted test
```

### Nightly Justification Contract

```text
GIVEN: Tests remaining with nightly build tag
THEN:  Each test file MUST contain a comment block explaining:
  1. Why the test cannot run in PR CI
  2. What external dependencies it requires
  3. Approximate execution time
```

## Contract 7: TUI State Machine

### State Transition Contract

```text
ViewState transitions MUST follow this graph:
  Initializing → Loading (OverviewDataReadyMsg)
  Loading → List (OverviewAllResourcesLoadedMsg)
  List → Detail (Enter key)
  Detail → List (Escape key)
  List → Quitting ('q' key)
  Any → Error (OverviewInitErrorMsg)

Invalid transitions MUST be no-ops (model unchanged).
No transition MUST cause a panic.
```

### Keyboard Navigation Contract

```text
In List state:
  'j' or Down arrow → Move cursor down (wrap at bottom)
  'k' or Up arrow → Move cursor up (wrap at top)
  Enter → Transition to Detail
  'q' → Transition to Quitting

In Detail state:
  Escape → Transition to List
  'q' → Transition to Quitting
```
