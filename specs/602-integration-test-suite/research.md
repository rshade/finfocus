# Research: Integration Test Suite Expansion

**Feature**: #602 Integration Test Suite Expansion
**Date**: 2026-02-22

## Decision Log

### D-001: Mock Plugin Extension Strategy

**Decision**: Extend existing `test/mocks/plugin/` infrastructure rather than
creating new mock frameworks.

**Rationale**: The mock plugin already has 53+ tests, 100% coverage, 5 pre-configured
scenarios, and 4 error injection types (Timeout, Protocol, InvalidData, Unavailable).
It supports TCP and bufconn modes. Extending it with crash injection (exit mid-RPC)
and configurable sleep behavior requires adding new scenario types, not a new
framework.

**Alternatives Considered**:

- New mock framework: Rejected — duplicates existing capability and increases
  maintenance burden
- Third-party mock plugins: Rejected — no control over crash behavior

### D-002: TUI Testing Without TTY

**Decision**: Use `model.Update(msg)` pattern for programmatic TUI testing without
requiring a real terminal.

**Rationale**: Bubble Tea models are testable via direct `Update()` calls. The
existing `overview_model_test.go` already demonstrates this pattern with
`TestOverviewModel_StateTransitions()`. No TTY is needed because we test the model
logic, not terminal rendering.

**Alternatives Considered**:

- Virtual TTY (pty): Rejected — adds OS-specific complexity, fragile in CI
- Screenshot comparison: Rejected — brittle and hard to maintain
- Headless terminal emulator: Rejected — overkill for state machine testing

### D-003: Cache Corruption Simulation

**Decision**: Simulate corruption by writing random bytes directly to the `cache.db`
file before the next read operation.

**Rationale**: BoltDB detects corruption via magic number and checksum validation.
Writing invalid bytes triggers the `ErrInvalid` / `ErrChecksum` code path that
the auto-recovery handles. This approach is deterministic and portable.

**Alternatives Considered**:

- Truncating the file: Only tests one corruption type
- Using flock to simulate lock contention: OS-specific behavior differs
- Replacing with a zero-byte file: Doesn't test partial corruption

### D-004: Build Tag Promotion Criteria

**Decision**: Promote trace propagation tests that use only Go standard library and
project code (no subprocess/binary builds) to `//go:build integration`. Keep tests
requiring binary builds as `//go:build nightly`.

**Rationale**: Four tests in `trace_propagation_test.go` are pure unit tests that
only call context helper functions. They have zero external dependencies and complete
in milliseconds. The remaining 3-4 tests build the finfocus binary and spawn
subprocesses, which is appropriate for nightly runs.

**Alternatives Considered**:

- Promote all: Rejected — binary-building tests are slow and may fail on CI workers
  without build tools
- Keep all nightly: Rejected — leaves core context helpers untested in PR CI

### D-005: Concurrency Correctness Testing Strategy

**Decision**: Compare `-j 1` vs `-j 8` results on the same plan file, asserting
identical cost totals and resource counts.

**Rationale**: If the worker pool has ordering bugs or data races, single-threaded
execution produces correct results while multi-threaded execution diverges. This
differential testing catches both correctness and race condition issues.

**Alternatives Considered**:

- Only race detector: Catches races but not ordering bugs
- Fuzzing worker count: Adds test time without targeted verification
- Property-based testing: Over-engineering for this specific check

### D-006: Zombie Process Detection Approach

**Decision**: Use `os.FindProcess` + signal 0 (Unix) or process existence check to
verify killed plugin processes are properly reaped.

**Rationale**: After `cmd.Process.Kill()`, the process enters zombie state until
`cmd.Wait()` is called. We can verify the process no longer exists (or is no longer
a zombie) after the cleanup sequence. Cross-platform: on Windows, killed processes
are immediately removed.

**Alternatives Considered**:

- Parse `/proc/<pid>/status`: Linux-only, not portable
- Use `ps aux | grep Z`: Fragile parsing, not available on all CI runners
- Skip zombie check: Misses the core safety guarantee

### D-007: Config Precedence Test Architecture

**Decision**: Use `t.TempDir()` with nested directory structures containing
`Pulumi.yaml` markers and layered config files.

**Rationale**: The config system walks up from CWD to find `Pulumi.yaml`. Creating
a temporary directory tree with known structure allows deterministic testing of the
full precedence chain without touching the real filesystem.

**Alternatives Considered**:

- Mock filesystem: Go's `os` package doesn't support mocking easily
- Environment variable only: Doesn't test walk-up behavior
- Shared fixture directory: Risks test pollution across parallel runs

### D-008: Analyzer Concurrency Test Design

**Decision**: Use `sync.WaitGroup` with 5 goroutines calling `Analyze()` concurrently,
plus `-race` flag for detection.

**Rationale**: The analyzer's `costCacheMu` mutex and `cancelMu` mutex protect shared
state. Concurrent test with race detector validates both correctness (no panics, valid
results) and safety (no data races). The mock `CostCalculator` can inject per-resource
errors to test partial failures.

**Alternatives Considered**:

- Sequential stress test: Doesn't test concurrency
- Third-party concurrency test library: Unnecessary complexity
- Chaos monkey approach: Non-deterministic, hard to reproduce

### D-009: CI Nightly Workflow Design

**Decision**: Add a GitHub Actions workflow file
(`.github/workflows/nightly.yml`) triggered by `schedule` (cron) and manual
`workflow_dispatch`, running tests with `//go:build nightly` tag against `main`.

**Rationale**: FR-031 requires a post-merge or nightly cron CI workflow for the
nightly test suite. GitHub Actions `schedule` triggers are the standard approach.
Manual dispatch allows on-demand runs during development.

**Alternatives Considered**:

- Extend existing CI workflow: Rejected — nightly tests are slow and should not block
  PR merges
- Separate nightly branch: Rejected — adds merge overhead
- External CI service: Rejected — already using GitHub Actions

### D-010: E2E Module Sync Verification

**Decision**: Add a CI step that compares shared dependency versions between
root `go.mod` and `test/e2e/go.mod`.

**Rationale**: FR-032 requires CI verification that the E2E module stays in sync.
A simple script comparing Go version and key dependency versions (finfocus-spec,
Pulumi SDK, testify) catches drift early.

**Alternatives Considered**:

- Go workspace mode: Changes project structure significantly
- Single go.mod: E2E module intentionally separate for cloud credential isolation
- Manual review: Error-prone and easily forgotten

## Technology Assessment

### Existing Infrastructure Strengths

| Component | Readiness | Notes |
|-----------|-----------|-------|
| Mock plugin (test/mocks/plugin/) | High | 53+ tests, 5 scenarios, 4 error types |
| CLI helper (test/integration/helpers/) | High | 10+ methods, cleanup, env management |
| Test fixtures (test/fixtures/) | High | 53 files, 11 categories |
| Large datasets | High | 1K and 10K resource plans exist |
| BoltDB cache | Medium | Unit tests exist, no integration tests |
| Analyzer server | High | 92.7% unit coverage, mock calculator |
| Config system | Medium | Unit tests per function, no E2E precedence |
| TUI model | Medium | State transitions tested, gaps in keyboard |

### Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Zombie detection not portable | Medium | Low | Use process existence check, skip on Windows |
| BoltDB lock contention in CI | Low | Medium | Use separate temp dirs per test |
| TUI tests flaky without TTY | Medium | Medium | Use model.Update() only, no rendering |
| Nightly CI schedule unreliable | Low | Low | Add manual workflow_dispatch trigger |
| Large-stack tests slow | Medium | Low | Use mock plugin with zero latency |
