# Feature Specification: Integration Test Suite Expansion

**Feature Branch**: `602-integration-test-suite`
**Created**: 2026-02-22
**Status**: Draft
**Input**: User description: "Expand integration test coverage across 7 subsystems: plugin resilience/crash recovery (#742), CI build tag fragmentation (#741), analyzer concurrency/partial failures (#740), project-local config precedence (#739), concurrency/performance regression (#738), cache system (#736), and TUI interactive mode (#735)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Plugin Crash Recovery Confidence (Priority: P1)

As a developer making changes to the plugin host or engine, I need integration tests
that verify the system handles plugin crashes, timeouts, and missing binaries gracefully,
so that regressions in error-recovery paths are caught before merge.

**Why this priority**: Plugin crashes are the highest-severity production failure mode.
A panic or zombie process from a crashed plugin directly impacts end users. The plugin
host retry logic, zombie prevention (`cmd.Wait()` after `Kill()`), and timeout handling
are critical paths with zero integration test coverage today.

**Independent Test**: Can be fully tested by running mock plugins that crash mid-request,
sleep past deadlines, or point to missing binaries. Delivers confidence that the plugin
lifecycle handles all failure modes without panics.

**Acceptance Scenarios**:

1. **Given** a mock plugin that exits mid-gRPC-call, **When** the engine requests a
   projected cost, **Then** the engine returns a structured error (not a panic) with
   an actionable error message.
2. **Given** a mock plugin that sleeps beyond the context deadline, **When** the engine
   requests a cost, **Then** JSON output contains `ErrCodeTimeoutError` and no
   goroutines are leaked.
3. **Given** a plugin binary path that no longer exists on disk, **When** the registry
   attempts to launch it, **Then** the error message contains the missing path and is
   actionable.
4. **Given** a plugin that was killed, **When** the process table is inspected, **Then**
   no zombie processes (state `Z`) remain.
5. **Given** a plugin that crashed on the first request, **When** a second request is
   sent, **Then** the engine either re-launches the plugin or returns a clean error
   (not a stale connection error).

---

### User Story 2 - Cache System End-to-End Verification (Priority: P1)

As a developer modifying the cache layer or CLI flag handling, I need integration tests
that verify end-to-end cache behavior across the CLI-Engine-Cache boundary, so that
cache hits, TTL expiration, corruption recovery, and flag precedence are validated.

**Why this priority**: The BoltDB cache was added as a significant feature but has no
integration tests. Cache bugs (stale data, corruption, silent failures) directly affect
cost accuracy and user trust. Concurrent access correctness is critical since multiple
CLI invocations may share the same cache database.

**Independent Test**: Can be tested by running `cost projected` twice with the same
plan and verifying the second run returns cached results (adapter suffix `(cached)`),
then testing TTL expiry, corruption recovery, and flag precedence.

**Acceptance Scenarios**:

1. **Given** a plan file and a mock plugin, **When** `cost projected` is run twice,
   **Then** the second run returns results with `(cached)` in the adapter field and
   completes faster.
2. **Given** a short cache TTL, **When** the TTL expires and another request is made,
   **Then** the cache misses and the plugin is called again.
3. **Given** a corrupted `cache.db` file, **When** the next cost command runs, **Then**
   the system auto-recovers by deleting and recreating the database.
4. **Given** `--cache-ttl 60` flag, `FINFOCUS_CACHE_TTL=120` env var, and a config
   default, **When** the cache is initialized, **Then** the CLI flag value takes
   precedence.
5. **Given** a projected cost cached entry, **When** an actual cost query is made,
   **Then** the projected bucket does not interfere with the actual bucket.

---

### User Story 3 - Analyzer Concurrency and Partial Failure Handling (Priority: P2)

As a developer maintaining the Pulumi Analyzer, I need integration tests for large
stacks, concurrent analysis requests, and partial failures, so that real production
conditions are simulated before merge.

**Why this priority**: The analyzer iterates resources synchronously. Production stacks
can have 50-100+ resources. Without concurrency and partial-failure tests, timeouts,
OOM conditions, and mixed diagnostic output are invisible until production.

**Independent Test**: Can be tested by sending synthetic `AnalyzeStack` requests with
100 resources, launching concurrent analysis goroutines, and injecting per-resource
errors via the mock plugin.

**Acceptance Scenarios**:

1. **Given** a synthetic stack with 100 resources, **When** `AnalyzeStack` is called,
   **Then** the response completes within 10 seconds with no goroutine leaks.
2. **Given** 5 concurrent `AnalyzeStack` calls, **When** executed with `-race`,
   **Then** all return valid diagnostics with no data races.
3. **Given** a mock plugin that errors for resource type A but succeeds for type B,
   **When** `AnalyzeStack` processes both, **Then** diagnostics contain a warning for
   A and a cost estimate for B, and the overall call succeeds.
4. **Given** a context cancelled mid-analysis, **When** some resources are already
   processed, **Then** the server tears down gracefully with no panic.
5. **Given** a stack with 50% unknown resource types, **When** analyzed, **Then**
   unknown types produce advisory warnings (not hard errors).

---

### User Story 4 - Config Precedence End-to-End Validation (Priority: P2)

As a developer changing config loading or the two-tier config system, I need integration
tests that verify the full precedence chain (flag > env > project config > global
config > defaults), so that configuration regressions are caught.

**Why this priority**: Configuration bugs silently change behavior for all users. The
two-tier system (project-local `.finfocus/` over global `~/.finfocus/`) has minimal
integration coverage. Incorrect merge behavior or precedence violations are hard to
diagnose in production.

**Independent Test**: Can be tested by creating temporary directory trees with
`Pulumi.yaml` markers and layered config files, then verifying resolved values match
expected precedence.

**Acceptance Scenarios**:

1. **Given** a subdirectory inside a Pulumi project, **When** `finfocus` is run from
   that subdirectory, **Then** the `.finfocus/` directory in the project root is
   discovered.
2. **Given** a project config with `budget.limit: 100` and a global config with
   `budget.limit: 50`, **When** config is loaded, **Then** the project value (100)
   is used.
3. **Given** `--project-dir /explicit/path` flag and `FINFOCUS_PROJECT_DIR=/env/path`,
   **When** config is loaded, **Then** the flag value takes precedence.
4. **Given** malformed YAML in the project config, **When** config loading is attempted,
   **Then** a descriptive error message is returned (not a panic).
5. **Given** a fresh `.finfocus/` directory, **When** `EnsureGitignore` is called twice,
   **Then** the `.gitignore` file is created on the first call and the second call is
   idempotent.
6. **Given** a project-local `dismissed.json` and a global `dismissed.json`, **When**
   dismissals are checked, **Then** the project-local file takes precedence.

---

### User Story 5 - Concurrency and Performance Regression Detection (Priority: P2)

As a developer modifying the engine worker pool or the `--jobs` flag, I need integration
tests that verify concurrent resource processing correctness and detect performance
regressions, so that the parallel enrichment pipeline remains correct and fast.

**Why this priority**: The `--jobs`/`-j` flag and parallel enrichment pipeline were
added but have no concurrency correctness tests. Race conditions or ordering bugs would
produce silently wrong cost calculations.

**Independent Test**: Can be tested by comparing results with `-j 1` vs `-j 8`, running
large synthetic plans, and launching parallel CLI invocations against a shared cache.

**Acceptance Scenarios**:

1. **Given** the same plan file, **When** run with `-j 1` and `-j 8`, **Then** cost
   totals are identical (no ordering or data race issues).
2. **Given** `-j 0` (auto-detect), **When** the command runs, **Then** it completes
   without error on any machine.
3. **Given** a synthetic plan with 500 resources, **When** processed with a mock plugin,
   **Then** the command completes within 30 seconds.
4. **Given** 5 parallel `finfocus cost projected` processes sharing the same cache DB,
   **When** all run simultaneously, **Then** no BoltDB corruption errors occur.
5. **Given** table format output with `--jobs`, **When** the command completes, **Then**
   stderr contains `resources/sec` throughput metric; for JSON/NDJSON it is absent.

---

### User Story 6 - CI Build Tag Fragmentation Resolution (Priority: P3)

As a CI maintainer, I need nightly-only test files reviewed and promotable tests moved
to the standard integration suite, so that regressions in trace propagation and audit
logging are caught during PR CI rather than after merge.

**Why this priority**: Three test files are gated behind `//go:build nightly` and never
run during PR CI. Regressions in core observability infrastructure (trace ID
propagation) and audit logging are invisible until post-merge nightly runs.

**Independent Test**: Can be tested by promoting eligible tests to `//go:build
integration`, verifying they pass in CI, and confirming remaining nightly tests have
documented justification.

**Acceptance Scenarios**:

1. **Given** `TestTracePropagation_ContextHelpers` and related context-only tests,
   **When** their build tag is changed to `integration`, **Then** they pass in the
   standard PR CI pipeline.
2. **Given** nightly-only tests that cannot be promoted, **When** reviewed, **Then**
   each has an explicit comment explaining why nightly-only is required.
3. **Given** the `test/e2e/` separate Go module, **When** CI runs on `main`, **Then**
   a post-merge job runs the nightly test suite.
4. **Given** `test/e2e/go.mod`, **When** CI runs, **Then** module sync verification
   confirms it is in sync with the main module.

---

### User Story 7 - TUI Interactive Mode Regression Coverage (Priority: P3)

As a developer modifying the TUI or the overview model, I need integration tests for
state machine transitions, keyboard navigation, phase checklist rendering, and error
states, so that interactive regressions are caught automatically.

**Why this priority**: The TUI was significantly refactored for state-first loading.
Current integration tests only cover output format selection (JSON/NDJSON bypass TUI).
Zero tests exist for the actual interactive model, meaning interactive regressions are
only caught manually.

**Independent Test**: Can be tested using model-level test helpers (`model.Update(msg)`)
without requiring a real TTY. Verifies state transitions, keyboard handling, and error
recovery programmatically.

**Acceptance Scenarios**:

1. **Given** a new `OverviewModel`, **When** phase messages 0-5 are sent, **Then** the
   state machine progresses through `Initializing` -> `Loading` -> `List`.
2. **Given** the model in `List` state, **When** up/down/enter/q keys are sent, **Then**
   the correct state transitions occur (navigation, detail view, quit).
3. **Given** an `OverviewPassphraseRequiredMsg`, **When** received, **Then** the inline
   passphrase prompt is triggered and the channel receives input.
4. **Given** an `OverviewInitErrorMsg`, **When** received, **Then** the model transitions
   to the `Error` ViewState.
5. **Given** a `WindowSizeMsg`, **When** received, **Then** the layout recalculates
   without panic.

---

### Out of Scope

- **Bug fixes**: If integration tests reveal that the system does not correctly handle
  a failure mode (e.g., zombie processes remain, cache does not auto-recover), the test
  should document the expected behavior via assertions. Any discovered bugs MUST be
  filed as new GitHub issues, not fixed within this feature.
- **New production features**: No changes to production code paths beyond what is needed
  to make existing behavior testable (e.g., exposing a test hook).
- **E2E tests requiring cloud credentials**: All new tests use mock plugins and local
  fixtures only.

### Edge Cases

- What happens when the cache DB is locked by another process during concurrent access?
- How does the system behave when a plugin crashes repeatedly in rapid succession?
- What happens when config files have correct YAML syntax but semantically invalid values?
- How does the analyzer handle a stack with zero priceable resources?
- What happens when the TUI receives messages in an unexpected order?
- How does the system behave when `--jobs` is set higher than the number of resources?
- What happens when the nightly build tag is combined with the integration tag?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST return a structured error (not a panic) containing the plugin
  name or resource type when a plugin crashes mid-request.
- **FR-002**: System MUST surface `ErrCodeTimeoutError` in JSON output when a plugin
  exceeds the context deadline.
- **FR-003**: System MUST prevent zombie processes after plugin kill cycles.
- **FR-004**: System MUST return an error message containing the exact binary file path
  when a plugin binary is missing.
- **FR-005**: System MUST return cached results with `(cached)` adapter suffix on cache
  hits across the CLI-Engine-Cache boundary.
- **FR-006**: System MUST respect cache TTL and re-query plugins after expiration.
- **FR-007**: System MUST auto-recover from a corrupted cache database by deleting and
  recreating it.
- **FR-008**: System MUST respect cache TTL precedence: CLI flag > env var > config >
  default.
- **FR-009**: System MUST isolate cache buckets (projected, actual, recommendations)
  so cross-contamination cannot occur.
- **FR-010**: Analyzer MUST handle 100+ resource stacks within 10 seconds with mock
  plugins.
- **FR-011**: Analyzer MUST handle concurrent `AnalyzeStack` calls without data races.
- **FR-012**: Analyzer MUST produce mixed diagnostics (warnings + cost estimates) on
  partial plugin failures.
- **FR-013**: Analyzer MUST handle mid-flight cancellation without panics or goroutine
  leaks.
- **FR-014**: Config system MUST discover `.finfocus/` by walking up from CWD to find
  `Pulumi.yaml`.
- **FR-015**: Config system MUST apply shallow merge: project keys override global keys,
  absent keys inherit.
- **FR-016**: Config system MUST respect precedence: `--project-dir` flag >
  `FINFOCUS_PROJECT_DIR` env > CWD walk > global fallback.
- **FR-017**: Config system MUST return descriptive errors for malformed YAML (not
  panics).
- **FR-018**: `EnsureGitignore` MUST be idempotent (safe to call multiple times).
- **FR-019**: Engine MUST produce identical cost totals regardless of `--jobs` value
  (concurrency correctness).
- **FR-020**: Engine MUST complete 500-resource plans within 30 seconds using mock
  plugins.
- **FR-021**: Concurrent CLI invocations sharing a cache DB MUST NOT corrupt the
  database.
- **FR-022**: Context-only trace propagation tests MUST run in standard PR CI (promoted
  from nightly).
- **FR-023**: Remaining nightly-only tests MUST have explicit comments explaining why
  promotion is not possible.
- **FR-024**: A post-merge or nightly cron CI workflow MUST run the nightly test suite
  against the `main` branch.
- **FR-025**: CI MUST verify `test/e2e/go.mod` is in sync with the main module.
- **FR-026**: TUI model MUST progress through ViewState transitions in correct order.
- **FR-027**: TUI model MUST handle keyboard navigation without panics.
- **FR-028**: TUI model MUST transition to Error state on initialization failure.
- **FR-029**: TUI model MUST handle terminal resize without panics.
- **FR-030**: All new tests MUST pass with race detection enabled.
- **FR-031**: All new tests MUST produce zero new lint warnings.
- **FR-032**: Discovered bugs MUST be filed as new GitHub issues, not fixed within this
  feature scope.
- **FR-033**: Config system MUST respect `dismissed.json` project-local precedence over
  global `dismissed.json`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 7 subsystems (plugin resilience, cache, analyzer, config, concurrency,
  CI tags, TUI) have dedicated integration tests that pass in PR CI.
- **SC-002**: Plugin crash, timeout, and missing-binary scenarios produce structured
  errors (verified by test assertions) with zero panics across all scenarios.
- **SC-003**: Cache hit/miss, TTL expiry, and corruption recovery are verified
  end-to-end across the CLI-Engine-Cache boundary.
- **SC-004**: Analyzer handles 100-resource stacks in under 10 seconds and 5 concurrent
  calls with no data races detected.
- **SC-005**: Config precedence is verified for at least 3 override scenarios (flag >
  env > file).
- **SC-006**: `--jobs` correctness is verified by comparing single-threaded vs
  multi-threaded results on the same plan with identical totals.
- **SC-007**: Context-only trace tests are promoted to standard integration suite and
  run in every PR CI pipeline.
- **SC-008**: TUI ViewState transition happy path and at least one error path are
  covered by automated tests.
- **SC-009**: All new tests pass with race detection enabled and linting produces zero
  new warnings.
- **SC-010**: 500-resource plans complete within 30 seconds and 5 concurrent cache
  users produce no corruption.

## Clarifications

### Session 2026-02-22

- Q: If tests discover that the system doesn't handle a failure mode correctly, what should happen? → A: Tests only — write tests for expected behavior, file new GitHub issues for any discovered bugs. No bug fixes in scope.
- Q: Should CI workflow changes (nightly cron job, go.mod sync check) be included in Story 6 or deferred? → A: Include CI changes — add/modify GitHub Actions workflow files as part of Story 6 delivery.

## Assumptions

- Mock plugin infrastructure (`test/mocks/plugin/`) can be extended with crash injection
  and configurable sleep behavior without major refactoring.
- Model-level TUI testing can use `model.Update(msg)` pattern without requiring a real
  TTY, following existing patterns in the TUI test files.
- Cache corruption can be simulated by writing invalid bytes to the `cache.db` file
  before the next read.
- The existing CLIHelper in `test/integration/helpers/cli_helper.go` supports the
  patterns needed for new CLI-level integration tests.
- Zombie process detection via process table inspection is available in CI runners.
- Context-only trace propagation tests have no external dependencies beyond the Go
  standard library and project code.
- The `test/e2e/go.mod` sync check can be added to the existing CI pipeline without
  significant CI configuration changes.
