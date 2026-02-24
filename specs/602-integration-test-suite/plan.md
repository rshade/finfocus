# Implementation Plan: Integration Test Suite Expansion

**Branch**: `602-integration-test-suite` | **Date**: 2026-02-22 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/602-integration-test-suite/spec.md`

## Summary

Expand integration test coverage across 7 subsystems: plugin resilience/crash
recovery, cache system end-to-end, analyzer concurrency, config precedence,
engine concurrency correctness, CI build tag promotion, and TUI state machine
testing. All tests use mock plugins and local fixtures — no cloud credentials
required. The approach extends the existing `test/mocks/plugin/` infrastructure
(53+ tests, 5 scenarios, 4 error injection types) and `test/integration/helpers/`
CLI helper rather than creating new frameworks.

## Technical Context

**Language/Version**: Go 1.25.7 (see `go.mod`)
**Primary Dependencies**: testify (assert/require), BoltDB (bbolt), Bubble Tea
(bubbletea), zerolog, cobra, gRPC, Pulumi SDK v3.210.0+
**Storage**: BoltDB for cache (`cache.db`), YAML for config, JSON for fixtures
**Testing**: `go test` with `-race`, `-tags integration`, `-tags nightly`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module (with separate `test/e2e/` module)
**Performance Goals**: 100-resource analyzer stack < 10s, 500-resource engine plan
< 30s, 5 concurrent cache users with zero corruption
**Constraints**: No cloud credentials, no production code changes beyond test hooks,
all new tests pass with `-race` flag
**Scale/Scope**: ~10 new files (6 test files, 2 helper files, 1 mock test file, 1 CI
workflow), ~30 test functions, extending 2-3 existing files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Tests validate plugin communication and error
  handling via the existing gRPC plugin host. No direct provider integrations added.
- [x] **Test-Driven Development**: This feature IS test development. Tests cover
  critical paths (plugin crash recovery, cache correctness, concurrency safety) to
  help achieve and maintain 80%+ coverage. Tests written before any production
  code changes.
- [x] **Cross-Platform Compatibility**: All tests use portable Go standard library
  and testify assertions. Zombie process detection uses `os.FindProcess` (portable).
  No OS-specific system calls.
- [x] **Documentation Integrity**: No API changes. Quickstart.md documents how to
  run new tests. No README or docs/ changes needed since no production behavior changes.
- [x] **Protocol Stability**: No protocol buffer changes. Tests use existing gRPC
  protocol via mock plugins.
- [x] **Implementation Completeness**: All tests will be fully implemented — no
  stubs, no TODOs, no placeholder assertions. Each test exercises real behavior via
  mock plugins and actual code paths.
- [x] **Quality Gates**: All tests must pass with `-race`, `make lint`, `make test`.
  New tests add zero lint warnings.
- [x] **Multi-Repo Coordination**: No cross-repo changes. Tests use existing
  `finfocus-spec` SDK interfaces (mock CostCalculator, pluginsdk constants).

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/602-integration-test-suite/
├── plan.md              # This file
├── research.md          # Phase 0: research decisions
├── data-model.md        # Phase 1: test entities and state machines
├── quickstart.md        # Phase 1: how to run the new tests
├── contracts/
│   └── test-contracts.md  # Phase 1: test behavior contracts
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
test/
├── integration/
│   ├── helpers/
│   │   └── cli_helper.go           # Existing (may extend)
│   ├── plugin_resilience_test.go   # NEW: US-1 plugin crash/timeout/zombie
│   ├── cache_system_test.go        # NEW: US-2 cache hit/miss/TTL/corruption
│   ├── analyzer_concurrency_test.go # NEW: US-3 large stacks/concurrent/partial
│   ├── config_precedence_test.go   # NEW: US-4 full precedence chain
│   ├── concurrency_correctness_test.go # NEW: US-5 jobs flag/parallel correctness
│   ├── trace_propagation_test.go   # MODIFY: promote 4 tests to integration tag
│   ├── audit_test.go              # MODIFY: add nightly justification comments
│   └── tui_state_machine_test.go  # NEW: US-7 ViewState transitions/keyboard
├── mocks/
│   └── plugin/
│       └── config.go              # MODIFY: add crash/sleep/partial scenarios
└── fixtures/
    └── (existing fixtures sufficient)

.github/
└── workflows/
    ├── ci.yml                     # MODIFY: add go.mod sync check step
    └── nightly.yml                # NEW: nightly cron workflow for nightly tests
```

**Structure Decision**: All new integration tests go in `test/integration/` as
top-level files following the existing pattern (47 files already there). One file
per subsystem for clear ownership and independent execution. Mock plugin extensions
stay in the existing `test/mocks/plugin/` package.

## Implementation Approach

### Story 1: Plugin Resilience (P1)

**File**: `test/integration/plugin_resilience_test.go`
**Build tag**: `//go:build integration`

Tests:

1. **Crash mid-RPC**: Start mock plugin, configure to exit during handler, verify
   structured error (not panic) with actionable message
2. **Timeout**: Configure mock plugin to sleep beyond context deadline, verify
   `ErrCodeTimeoutError` in JSON output, check goroutine count delta for leaks
3. **Missing binary**: Point registry at nonexistent path, verify error contains
   the missing file path
4. **Zombie prevention**: Kill a plugin process, verify PID no longer exists via
   `os.FindProcess` + signal 0
5. **Recovery after crash**: Crash on first request, verify second request gets
   clean error or successful re-launch

**Mock plugin changes**: Add `ExitMidCall` and `SleepDuration` to error injection
config in `test/mocks/plugin/config.go`.

### Story 2: Cache System (P1)

**File**: `test/integration/cache_system_test.go`
**Build tag**: `//go:build integration`

Tests:

1. **Cache hit**: Run `cost projected` twice with same plan and mock plugin, verify
   second run has `(cached)` in adapter field
2. **TTL expiry**: Set short TTL (1s), write entry, wait, verify cache miss and
   plugin called again
3. **Corruption recovery**: Write random bytes to `cache.db`, run next command,
   verify auto-recovery (delete + recreate)
4. **Flag precedence**: Set `--cache-ttl`, env var, and config value, verify CLI
   flag value is used
5. **Bucket isolation**: Cache a projected entry, run an actual cost query, verify
   projected bucket untouched

### Story 3: Analyzer Concurrency (P2)

**File**: `test/integration/analyzer_concurrency_test.go`
**Build tag**: `//go:build integration`

Tests:

1. **100-resource stack**: Generate synthetic stack, call AnalyzeStack via gRPC,
   verify completion under 10s with diagnostic count matching resource count
2. **5 concurrent calls**: Launch 5 goroutines calling Analyze() with `-race`,
   verify all return valid diagnostics
3. **Partial failures**: Configure mock to error for type A, succeed for type B,
   verify mixed diagnostics (warning + cost estimate)
4. **Context cancellation**: Cancel context mid-analysis, verify graceful teardown
   with no panic
5. **Unknown types**: Send 50% unknown resource types, verify advisory warnings

### Story 4: Config Precedence (P2)

**File**: `test/integration/config_precedence_test.go`
**Build tag**: `//go:build integration`

Tests:

1. **Walk-up discovery**: Create nested temp dirs with `Pulumi.yaml`, run from
   subdirectory, verify `.finfocus/` found in project root
2. **Project overrides global**: Create both configs with different `budget.limit`,
   verify project value used
3. **Flag overrides env**: Set `--project-dir` and `FINFOCUS_PROJECT_DIR`, verify
   flag wins
4. **Malformed YAML**: Write invalid YAML to project config, verify descriptive
   error (not panic)
5. **EnsureGitignore idempotency**: Call twice on fresh dir, verify `.gitignore`
   created once, second call is no-op
6. **Dismissed.json precedence**: Create both project-local and global dismissed
   files, verify project-local wins (uses `internal/config/dismissed.go`)

### Story 5: Concurrency Correctness (P2)

**File**: `test/integration/concurrency_correctness_test.go`
**Build tag**: `//go:build integration`

Tests:

1. **j1 vs j8 equivalence**: Same plan, compare cost totals with single-threaded
   and multi-threaded execution
2. **j0 auto-detect**: Run with `-j 0`, verify completion without error
3. **500-resource plan**: Generate large synthetic plan with mock plugin, verify
   completion under 30s
4. **Concurrent cache access**: Spawn 5 parallel OS processes (via `exec.Command`)
   sharing cache.db, verify no BoltDB file-level locking corruption
5. **Throughput metric**: Run with table format and `--jobs`, verify stderr
   contains `resources/sec`; verify absent for JSON/NDJSON

### Story 6: CI Build Tag Promotion (P3)

**Files modified**:

- `test/integration/trace_propagation_test.go`: Change build tag from `nightly`
  to `integration` for 4 context-only tests. Keep `nightly` for 3 binary-building
  tests with justification comments.
- `test/integration/audit_test.go`: Add explicit comment explaining why nightly-only
  is required.
- `.github/workflows/nightly.yml`: New workflow with `schedule` (cron) and
  `workflow_dispatch` triggers, running `go test -tags nightly ./test/integration/...`
- `.github/workflows/ci.yml`: Add step to verify `test/e2e/go.mod` Go version and
  key dependency versions match root module.

### Story 7: TUI State Machine (P3)

**File**: `test/integration/tui_state_machine_test.go`
**Build tag**: `//go:build integration`

Tests:

1. **Phase progression**: Create model, send phase messages 0-5, verify state
   progresses `Initializing -> Loading -> List`
2. **Keyboard navigation**: In List state, send up/down/enter/q keys, verify
   correct state transitions
3. **Passphrase prompt**: Send `OverviewPassphraseRequiredMsg`, verify inline
   prompt is triggered
4. **Error state**: Send `OverviewInitErrorMsg`, verify transition to
   `ViewStateError`
5. **Window resize**: Send `WindowSizeMsg`, verify layout recalculates without
   panic

## Complexity Tracking

> No Constitution Check violations. No complexity justification needed.
