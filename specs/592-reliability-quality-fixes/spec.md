# Feature Specification: Reliability & Quality Fixes Batch

**Feature Branch**: `592-reliability-quality-fixes`
**Created**: 2026-02-14
**Status**: Draft
**Input**: User description: "Fix group of tickets: --jobs flag (#602), test isolation (#605), DRY recommendation helpers (#610), overview worker pool (#652), cache goroutine fix (#653), context-cancelable HTTP (#654), fuzz test masking (#655), stdio proxy lifecycle (#656)"
**Related Issues**: #602, #605, #610, #652, #653, #654, #655, #656

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configurable Parallel Cost Analysis (Priority: P1)

As a user analyzing large infrastructure deployments, I want to control how many resources
are evaluated simultaneously and see throughput metrics, so I can optimize performance for
my environment and demonstrate scale capabilities.

**Why this priority**: Directly user-facing feature that enables scale demos and gives users
control over resource utilization. Highest visibility among all changes.

**Independent Test**: Can be fully tested by running `finfocus cost projected --jobs 4 --pulumi-json plan.json`
and verifying the concurrency override and timing output appear.

**Acceptance Scenarios**:

1. **Given** a Pulumi plan with 100 resources, **When** I run cost projected with `--jobs 4`,
   **Then** the system uses 4 concurrent workers and displays timing output
   showing "Analyzed 100 resources in X.Xs (Y.Y resources/sec)".
2. **Given** a cost command with default settings, **When** I run without `--jobs`,
   **Then** the system auto-calculates worker count and preserves existing behavior.
3. **Given** `--jobs 1`, **When** I run cost projected, **Then** the system uses a single
   worker (useful for debugging sequential execution).
4. **Given** JSON or NDJSON output format, **When** timing output would normally appear,
   **Then** timing output is suppressed to avoid corrupting machine-readable output.

---

### User Story 2 - Cancellable Plugin Operations (Priority: P1)

As a user running plugin install or update operations, I want to be able to cancel
long-running network operations with Ctrl+C, so I'm not stuck waiting for hung network
requests.

**Why this priority**: Affects core user experience. Users currently cannot cancel
long-running plugin registry operations, leading to frustration and potential process
hangs.

**Independent Test**: Can be tested by initiating a plugin install and pressing Ctrl+C,
verifying the operation cancels promptly rather than hanging.

**Acceptance Scenarios**:

1. **Given** a slow or unresponsive plugin registry, **When** I press Ctrl+C during
   a plugin operation, **Then** all in-flight network requests are canceled promptly.
2. **Given** a normal plugin operation, **When** it completes successfully,
   **Then** behavior is unchanged from before this fix.

---

### User Story 3 - Reliable Nightly Quality Checks (Priority: P2)

As a project maintainer, I want nightly automated testing to actually report failures
when bugs are found, so I can catch regressions before they reach users.

**Why this priority**: Silent test failures allow regressions to accumulate unnoticed,
degrading overall product quality over time.

**Independent Test**: Can be verified by checking that a nightly workflow run fails
when a test finds a real issue, rather than silently succeeding.

**Acceptance Scenarios**:

1. **Given** a fuzz test discovers a crash, **When** the nightly workflow runs,
   **Then** the workflow reports failure instead of masking it.
2. **Given** all fuzz tests pass, **When** the nightly workflow runs,
   **Then** test artifacts are still uploaded and cached normally.

---

### User Story 4 - Stable Long-Running Operations (Priority: P2)

As a user running cost analysis on large infrastructure, I want the system to manage
its internal resources efficiently, so it doesn't consume excessive memory or hang
during long operations.

**Why this priority**: Resource leaks and unbounded goroutine creation degrade stability
during extended use and with large infrastructure stacks.

**Independent Test**: Can be tested by running cost analysis on a large plan (500+
resources) and verifying stable memory usage and clean shutdown.

**Acceptance Scenarios**:

1. **Given** a large Pulumi stack with many resources, **When** overview enrichment runs,
   **Then** the system uses a bounded worker pool instead of spawning unbounded goroutines.
2. **Given** a cache with many expired entries, **When** cache lookups occur under load,
   **Then** the system cleans up expired entries without spawning unbounded goroutines.
3. **Given** a plugin using stdio communication, **When** the plugin exits or the client
   disconnects, **Then** all proxy goroutines exit cleanly without leaks or hangs.

---

### User Story 5 - Consistent Developer Experience (Priority: P3)

As a developer contributing to the project, I want tests to be reliable regardless of
the environment they run in, and I want the codebase to follow DRY principles, so I can
maintain code efficiently and trust test results.

**Why this priority**: Improves developer productivity and reduces maintenance burden,
but doesn't directly affect end users.

**Independent Test**: Can be verified by running `make test` from any directory and
getting consistent results, and by confirming no duplicate helper functions exist.

**Acceptance Scenarios**:

1. **Given** auto-detection tests, **When** run from any working directory
   (including one containing Pulumi project files), **Then** tests produce
   consistent results by using isolated temporary directories.
2. **Given** recommendation count formatting logic, **When** used in table output
   or TUI rendering, **Then** both use a single shared helper function with identical behavior.

---

### Edge Cases

- What happens when `--jobs` is set to a value exceeding the resource count? (Capped at resource count)
- What happens when `--jobs` is set to a negative number? (Rejected with a clear error)
- What happens when multiple expired cache entries are accessed concurrently? (No race conditions or goroutine leaks)
- What happens when a plugin's stdio proxy receives a half-close? (Gracefully shuts down both directions)
- What happens when Ctrl+C is pressed during multiple concurrent HTTP requests? (All cancel promptly)
- What happens when fuzz tests time out normally vs discovering a crash? (Timeouts succeed; crashes fail the workflow)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept a `--jobs` / `-j` flag on cost projected and cost actual commands to control worker concurrency
- **FR-002**: System MUST default to automatic worker count calculation when `--jobs` is not specified or set to 0
- **FR-003**: System MUST display timing output ("Analyzed N resources in X.Xs (Y.Y resources/sec)") after cost calculations in table output format only
- **FR-004**: System MUST suppress timing output when output format is JSON or NDJSON
- **FR-005**: System MUST cap the jobs value at the resource count to prevent over-allocation
- **FR-006**: System MUST cancel in-flight network requests to the plugin registry when the user's operation context is canceled
- **FR-007**: System MUST report fuzz test failures as workflow failures instead of masking them
- **FR-008**: System MUST preserve fuzz test artifact upload even when tests fail
- **FR-009**: System MUST limit concurrent goroutines for overview enrichment to a bounded worker pool model
- **FR-010**: System MUST clean up expired cache entries without spawning unbounded goroutines
- **FR-011**: System MUST ensure stdio proxy goroutines exit cleanly on plugin or client disconnect
- **FR-012**: System MUST isolate auto-detection tests from the host filesystem environment
- **FR-013**: System MUST provide a single shared helper for recommendation count formatting, eliminating duplication
- **FR-014**: System MUST export the shared recommendation counting function for use across packages

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can control cost analysis parallelism via a single flag, with auto mode preserving existing behavior
- **SC-002**: Timing output displays resource count, elapsed time, and throughput rate after each cost analysis (table format only)
- **SC-003**: Canceling a plugin operation via Ctrl+C terminates all in-flight network requests within 1 second
- **SC-004**: Nightly automated tests correctly report failures when bugs are discovered (zero false-pass rate)
- **SC-005**: Overview enrichment uses no more than a fixed number of concurrent workers regardless of input size
- **SC-006**: Cache operations produce no goroutine leaks under sustained load with expired entries
- **SC-007**: Stdio proxy goroutines exit within 5 seconds of plugin or client disconnect
- **SC-008**: Auto-detection tests produce identical results regardless of working directory
- **SC-009**: Zero duplicate recommendation formatting functions exist in the codebase
- **SC-010**: All existing tests continue to pass (`make test` and `make lint` succeed)

### Assumptions

- The existing auto-calculated worker count formula (based on CPU count, capped at resource count) is acceptable as the default
- Timing output on stderr is acceptable to avoid interfering with stdout machine-readable output
- Synchronous cache entry deletion performance is acceptable (file deletion is fast on modern filesystems)
- The existing concurrency limit constant is an appropriate bound for the worker pool
- Fuzz test `|| true` was added as a temporary workaround and can now be safely removed
- The recommendation count helper can be exported from the engine package without breaking package boundaries
