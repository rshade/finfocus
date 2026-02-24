# Quickstart: Integration Test Suite

**Feature**: #602 Integration Test Suite Expansion

## Running the New Integration Tests

### All Integration Tests

```bash
# Run all integration tests (includes new subsystem tests)
make test-integration
```

### By Subsystem

```bash
# Plugin resilience (crash recovery, timeouts, zombies)
go test -v -tags integration -run TestPlugin ./test/integration/...

# Cache system (hit/miss, TTL, corruption, precedence)
go test -v -tags integration -run TestCache ./test/integration/...

# Analyzer concurrency (large stacks, concurrent calls, partial failures)
go test -v -tags integration -run TestAnalyzer ./test/integration/...

# Config precedence (flag > env > project > global)
go test -v -tags integration -run TestConfig ./test/integration/...

# Concurrency correctness (jobs flag, parallel processing)
go test -v -tags integration -run TestConcurrency ./test/integration/...

# TUI state machine (ViewState transitions, keyboard navigation)
go test -v -tags integration -run TestTUI ./test/integration/...
```

### With Race Detection

```bash
# All integration tests with race detector (required for FR-028)
go test -v -tags integration -race ./test/integration/...
```

### Nightly Tests

```bash
# Run nightly-only tests (binary build + subprocess tests)
go test -v -tags nightly ./test/integration/...
```

## Test Categories and Expected Behavior

### Plugin Resilience Tests

| Test | What It Does | Expected Output |
|------|-------------|-----------------|
| Crash mid-RPC | Mock plugin exits during handler | Structured error, no panic |
| Timeout | Mock plugin sleeps past deadline | `ErrCodeTimeoutError` in JSON |
| Missing binary | Plugin path does not exist | Error with file path |
| Zombie prevention | Kill plugin, check process table | No zombie processes |
| Recovery after crash | Second request after crash | Clean error or re-launch |

### Cache System Tests

| Test | What It Does | Expected Output |
|------|-------------|-----------------|
| Cache hit | Run same command twice | Second run has `(cached)` adapter |
| TTL expiry | Wait for TTL, re-query | Plugin called again |
| Corruption recovery | Write bad bytes to cache.db | Auto-delete and recreate |
| Flag precedence | Set flag, env, config | Flag value wins |
| Bucket isolation | Cache projected, query actual | No cross-contamination |

### Analyzer Concurrency Tests

| Test | What It Does | Expected Output |
|------|-------------|-----------------|
| 100-resource stack | Send large AnalyzeStack | Completes in under 10s |
| 5 concurrent calls | Parallel AnalyzeStack with -race | No data races |
| Partial failures | Mix of success/error resources | Mixed diagnostics |
| Context cancellation | Cancel mid-analysis | Graceful teardown |
| Unknown types | 50% unknown resource types | Advisory warnings |

## Prerequisites

- Go 1.25.7+ installed
- `make build` completed (for binary-dependent tests)
- No cloud credentials needed (all tests use mock plugins)
