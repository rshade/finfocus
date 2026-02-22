# Feature Specification: Overview Cost Caching

**Feature Branch**: `600-overview-cache`
**Created**: 2026-02-22
**Status**: Draft
**Input**: User description: "feat(overview): add cost caching to speed up enrichment"
**Issue**: [#745](https://github.com/rshade/finfocus/issues/745)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Cache enrichment results between overview runs (Priority: P1)

As a developer running `finfocus overview` multiple times during a work session,
I want cost enrichment results to be cached between runs so that repeat invocations
complete significantly faster and do not repeatedly call cloud cost APIs for data
that has not changed.

**Why this priority**: This is the core value proposition of the feature. Without
caching, every overview run triggers 100+ plugin round-trips on a moderately-sized
stack. Caching eliminates redundant API calls and reduces wait time from minutes to
seconds on subsequent runs.

**Independent Test**: Can be fully tested by running `finfocus overview --cache-ttl 300`
twice against the same stack and verifying that the second run is faster with cached
results marked in the adapter field.

**Acceptance Scenarios**:

1. **Given** a Pulumi stack with 10+ resources and `--cache-ttl 300`,
   **When** the user runs `finfocus overview` for the first time,
   **Then** all resources are enriched via plugin calls and results are stored in the cache.

2. **Given** cached cost data from a previous overview run within the TTL window,
   **When** the user runs `finfocus overview --cache-ttl 300` again,
   **Then** cached resources show `(cached)` in the adapter field and the command
   completes noticeably faster than the first run.

3. **Given** cached cost data from a previous overview run,
   **When** the user runs `finfocus overview` in interactive TUI mode,
   **Then** the TUI enrichment phase uses cached data and reflects `(cached)` in
   the detail view for previously cached resources.

---

### User Story 2 - Opt-in caching with TTL control (Priority: P1)

As a developer, I want caching to be opt-in and controllable via the same TTL
mechanisms used by other cost commands (`--cache-ttl` flag, `FINFOCUS_CACHE_TTL`
env var, `config.yaml`), so that the overview command behaves consistently with
`cost projected`, `cost actual`, and `cost recommendations`.

**Why this priority**: Consistency across commands is essential for user
expectations. The same precedence chain must work identically for overview.

**Independent Test**: Can be tested by setting cache TTL via each mechanism
(flag, env var, config) and verifying cache activation or deactivation.

**Acceptance Scenarios**:

1. **Given** no cache TTL configured anywhere (default),
   **When** the user runs `finfocus overview`,
   **Then** caching is disabled (TTL=0) and behavior matches the current uncached behavior.

2. **Given** `--cache-ttl 300` passed on the command line,
   **When** the user runs `finfocus overview --cache-ttl 300`,
   **Then** the cache is active with a 5-minute TTL.

3. **Given** `FINFOCUS_CACHE_TTL=600` set in the environment and no `--cache-ttl` flag,
   **When** the user runs `finfocus overview`,
   **Then** the cache is active with a 10-minute TTL.

4. **Given** `cost.cache.ttl_seconds: 120` in `config.yaml` and no env var or flag,
   **When** the user runs `finfocus overview`,
   **Then** the cache is active with a 2-minute TTL.

5. **Given** `--cache-ttl 0` explicitly passed,
   **When** the user runs `finfocus overview --cache-ttl 0`,
   **Then** caching is disabled regardless of env var or config file settings.

---

### User Story 3 - Cache cleanup on overview exit (Priority: P2)

As a developer, I want the cache database to be properly closed when the overview
command finishes (whether normally or due to an error), so that no data corruption
or file lock issues occur.

**Why this priority**: Proper resource cleanup is important for reliability but is
an implicit quality attribute rather than a user-visible feature.

**Independent Test**: Can be tested by running overview with caching enabled and
verifying the cache database file is not locked after the command exits.

**Acceptance Scenarios**:

1. **Given** caching is enabled and the overview command completes normally,
   **When** the command exits,
   **Then** the cache database is properly closed (cleanup function invoked).

2. **Given** caching is enabled and the overview command encounters an error mid-pipeline,
   **When** the command exits with an error,
   **Then** the cache database is still properly closed via deferred cleanup.

3. **Given** caching is enabled and the interactive TUI is exited by the user,
   **When** the TUI program terminates,
   **Then** the cache database is properly closed.

---

### Edge Cases

- What happens when the cache database is locked by another `finfocus` process?
  The system proceeds without caching (existing `ErrCacheLocked` handling).
- What happens when the cache database file is corrupted?
  The system auto-recovers by deleting and recreating the database (existing behavior).
- What happens when the cache TTL expires between the first and second resource
  during enrichment? Expired entries are lazily evicted on read; some resources
  may be re-fetched while others are served from cache within the same run.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The overview command MUST use the same caching infrastructure
  as the other cost commands for engine construction.
- **FR-002**: The overview command MUST respect the existing cache TTL precedence:
  `--cache-ttl` CLI flag > `FINFOCUS_CACHE_TTL` env var > `config.yaml` >
  default (0 = disabled).
- **FR-003**: When caching is enabled, cached cost results MUST display `(cached)`
  in the adapter field, consistent with other cost commands.
- **FR-004**: When caching is disabled (TTL=0 or omitted with no config), the overview
  command MUST behave identically to the current uncached implementation.
- **FR-005**: The cache database MUST be properly closed when the overview command
  exits, in both the plain-text and interactive TUI code paths.
- **FR-006**: Both the plain-text (non-interactive) and interactive TUI paths of the
  overview command MUST use the cached engine.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A second `finfocus overview` run against the same stack (within the
  cache TTL window) completes at least 50% faster than the first uncached run.
- **SC-002**: Cached resources are visually distinguishable via `(cached)` in the
  adapter field in both plain-text and TUI output.
- **SC-003**: Caching is disabled by default (TTL=0), preserving backward compatibility
  with existing workflows.
- **SC-004**: All existing tests continue to pass with no regressions.

## Scope

### In Scope

- Wiring the existing cache helper into the overview command's engine construction
- Ensuring cache cleanup in both plain-text and TUI code paths
- Consistent `--cache-ttl` behavior across all cost-related commands

### Out of Scope

- New cache storage backends
- New configuration fields or config commands
- Changes to TUI state machine or enrichment parallelism
- Changes to the cache implementation itself

## Assumptions

- The `--cache-ttl` persistent flag on the root command is already available to the
  overview command (confirmed: defined in `root.go` line 123).
- The cache helper in `common_execution.go` is reusable without modification for
  the overview command's engine construction.
- The existing cache infrastructure handles concurrent access, corruption recovery,
  and lazy TTL expiration without changes.
- The overview command has two engine construction sites (plain-text path and TUI
  path) that both need to be updated.

## Dependencies

- Existing cache infrastructure (no changes needed)
- Shared engine factory helper in `common_execution.go` (reuse as-is)
- `--cache-ttl` persistent flag on root command (already defined)
