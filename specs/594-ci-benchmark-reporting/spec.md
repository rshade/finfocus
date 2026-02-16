# Feature Specification: Benchmark PR Reporting with Regression Detection

**Feature Branch**: `594-ci-benchmark-reporting`
**Created**: 2026-02-15
**Status**: Draft
**Input**: User description: "Replace the current smoke-only benchmark job in CI with a full comparison workflow that runs benchmarks on every PR, compares against a cached main baseline using benchstat, and posts results as a PR comment with regression detection thresholds."

## Clarifications

### Session 2026-02-15

- Q: Which benchmark metrics should trigger regression thresholds, and should regressions fail CI? → A: Report all metrics (ns/op, B/op, allocs/op) in the PR comment with visual severity indicators, but never fail CI on regressions. Thresholds are informational only.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Contributor Sees Performance Impact on PR (Priority: P1)

As a contributor opening a pull request, I want to see a benchmark comparison comment on my PR so that I understand the performance impact of my changes before merging.

**Why this priority**: This is the core value of the feature. Without a visible benchmark report on the PR, the entire workflow delivers no value to contributors.

**Independent Test**: Can be fully tested by opening a PR against a repository with a cached baseline and verifying a benchstat comparison comment appears with formatted results.

**Acceptance Scenarios**:

1. **Given** a PR is opened against `main` and a baseline cache exists, **When** the CI benchmark job completes, **Then** a comment containing a benchstat comparison table is posted to the PR.
2. **Given** a PR already has a benchmark comment and new commits are pushed, **When** the benchmark job runs again, **Then** the existing comment is updated (not duplicated) with fresh results.
3. **Given** a PR is opened but no baseline cache exists yet, **When** the benchmark job completes, **Then** an informational comment is posted explaining that no baseline is available and results will be compared once the main branch baseline is generated.

---

### User Story 2 - CI Reports Severe Performance Regression (Priority: P1)

As a project maintainer, I want the CI pipeline to visibly flag performance regressions in the PR comment so that contributors and reviewers are aware of performance impact before merging.

**Why this priority**: Regression visibility is the primary protective mechanism. Without it, regressions go undetected. Reporting (rather than blocking) allows informed merge decisions while avoiding false-positive CI failures from benchmark noise.

**Independent Test**: Can be fully tested by introducing an artificial slowdown exceeding the threshold in a benchmark function and verifying the PR comment includes the appropriate severity indicator. The CI job itself always passes (regression does not fail CI).

**Acceptance Scenarios**:

1. **Given** a PR introduces a benchmark regression exceeding 50% on any metric, **When** the benchmark comparison step runs, **Then** the CI job passes and the PR comment includes an error-level indicator for the affected benchmarks.
2. **Given** a PR introduces a benchmark regression between 20% and 50% on any metric, **When** the benchmark comparison step runs, **Then** the CI job passes and the PR comment includes a warning-level indicator for the affected benchmarks.
3. **Given** a PR introduces a benchmark regression below 20%, **When** the benchmark comparison step runs, **Then** the CI job passes and the regression is noted as within acceptable noise.
4. **Given** a PR improves benchmark performance, **When** the benchmark comparison step runs, **Then** the improvement is noted positively in the PR comment.

---

### User Story 3 - Baseline Refreshes on Main Push (Priority: P2)

As a project maintainer, I want the benchmark baseline to be automatically regenerated whenever code is pushed to `main` so that PR comparisons always reflect the latest state of the default branch.

**Why this priority**: Without a fresh baseline, comparisons become stale and misleading. This is essential infrastructure but secondary to the PR-visible reporting.

**Independent Test**: Can be fully tested by pushing a commit to `main` and verifying the benchmark baseline cache is updated.

**Acceptance Scenarios**:

1. **Given** a commit is pushed to the `main` branch, **When** the baseline generation job runs, **Then** a new baseline file is produced and cached.
2. **Given** the baseline generation job runs, **When** the benchmarks execute, **Then** all benchmarks except the 100K-scale benchmarks are included.
3. **Given** the baseline generation job runs, **When** benchmark output contains non-benchmark noise (zerolog JSON, debug messages), **Then** the output is filtered to contain only benchstat-compatible lines.

---

### User Story 4 - Benchmark Crash Surfaces as CI Failure (Priority: P3)

As a contributor, I want benchmark crashes and errors to be surfaced as CI failures so that broken benchmark code is not silently ignored.

**Why this priority**: The current `|| true` suppression masks real failures. Surfacing crashes ensures benchmark code health, though it is less critical than the reporting and regression detection flows.

**Independent Test**: Can be fully tested by introducing a syntax error or panic in a benchmark function and verifying the CI job fails instead of silently passing.

**Acceptance Scenarios**:

1. **Given** a PR introduces code that causes a benchmark to panic, **When** the benchmark job runs, **Then** the job fails with a visible error (no silent suppression).
2. **Given** a PR introduces code that causes a benchmark compilation error, **When** the benchmark job runs, **Then** the job fails with the compiler error visible in logs.

---

### Edge Cases

- What happens when the cache is evicted between baseline generation and PR comparison? The system handles this identically to "first run" — informational comment, no failure.
- What happens when a benchmark is added or removed between baseline and current? Benchstat handles new/removed benchmarks by listing them separately; the PR comment reflects this.
- What happens when benchstat itself is unavailable or fails to install? The CI job fails with a clear error rather than silently continuing without comparison.
- What happens when the PR comment body exceeds GitHub's maximum comment size? The system truncates the detailed output using a collapsible section while keeping the summary visible.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: CI MUST run all benchmarks except 100K-scale benchmarks on every pull request.
- **FR-002**: CI MUST generate a benchmark baseline whenever code is pushed to the `main` branch.
- **FR-003**: CI MUST compare PR benchmark results against the cached `main` baseline using benchstat.
- **FR-004**: CI MUST post benchmark comparison results as a PR comment.
- **FR-005**: CI MUST update (not duplicate) the benchmark comment on subsequent pushes to the same PR, using a stable HTML comment marker for identification.
- **FR-006**: CI MUST display an error-level indicator in the PR comment when any benchmark shows a regression exceeding 50% on any metric (ns/op, B/op, or allocs/op). The CI job MUST NOT fail due to regression detection.
- **FR-007**: CI MUST display a warning-level indicator in the PR comment when any benchmark shows a regression between 20% and 50% on any metric.
- **FR-008**: CI MUST handle the "no baseline available" case gracefully by posting an informational comment instead of failing.
- **FR-009**: CI MUST run benchmarks with `-count=5` to provide sufficient statistical samples for benchstat analysis.
- **FR-010**: CI MUST filter benchmark output to remove non-benchmark noise (zerolog JSON, debug messages) before passing to benchstat.
- **FR-011**: CI MUST NOT suppress benchmark failures with `|| true` or equivalent patterns. Benchmark execution failures (panics, compilation errors) MUST still fail the CI job.
- **FR-012**: The existing local comparison script (`scripts/compare-benchmarks.sh`) MUST continue to work unchanged.
- **FR-013**: The filter script MUST be executable and pass linting validation.
- **FR-014**: CI MUST report all benchstat metrics (time, memory, allocations) in the PR comment for full visibility.

### Key Entities

- **Baseline**: A cached benchmark output file from the `main` branch, containing benchstat-compatible benchmark results with 5 samples per benchmark.
- **Current Results**: A benchmark output file from the PR branch, produced with identical benchmark parameters as the baseline.
- **Comparison Report**: A benchstat-generated statistical comparison between baseline and current results, posted as a PR comment. Includes all metrics (ns/op, B/op, allocs/op).
- **Regression Threshold**: A visual severity boundary (20% for warning indicator, 50% for error indicator) applied to all reported metrics. Thresholds are informational only and do not fail CI.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every pull request receives a benchmark comparison comment within the CI job duration (target: under 5 minutes).
- **SC-002**: Performance regressions exceeding 50% are visibly flagged with error-level indicators in the PR comment, ensuring reviewers are aware before approving.
- **SC-003**: Contributors can see the performance impact of their changes across all metrics (time, memory, allocations) without leaving the PR page.
- **SC-004**: Benchmark crashes and compilation errors are surfaced as CI failures (zero silent suppressions). Regression detection is informational only and never fails CI.
- **SC-005**: The baseline stays current with `main` branch (updated on every push to `main`).
- **SC-006**: PR comments are not duplicated on subsequent pushes (exactly one benchmark comment per PR at any time).

## Assumptions

- GitHub Actions cache is reliable enough for baseline storage; cache eviction is handled gracefully as a "first run" scenario.
- `-benchtime=1x` with `-count=5` provides sufficient statistical signal for benchstat to detect meaningful regressions on CI runners.
- The 20%/50% regression thresholds are appropriate starting points; they can be adjusted based on observed CI noise levels.
- The `actions/github-script@v7` action provides sufficient GitHub API access for creating and updating PR comments.
- benchstat is installable via `go install golang.org/x/perf/cmd/benchstat@latest` in CI.
- The existing benchmark suite in `test/benchmarks/` produces benchstat-compatible output when filtered properly.

## Scope Boundaries

### In Scope

- Replacing the existing smoke benchmark CI job with a two-phase (baseline + compare) workflow
- Creating a filter script for cleaning benchmark output
- Posting and updating PR comments with benchmark results
- Regression detection with threshold-based visual indicators (informational, non-blocking)
- Updating `.gitignore` for generated benchmark files

### Out of Scope

- Changing existing benchmark functions or adding new benchmarks
- Modifying the local `scripts/compare-benchmarks.sh` script
- Historical benchmark trend tracking across multiple baselines
- Benchmark result storage beyond the single latest `main` baseline
- Performance budget configuration files or per-benchmark threshold overrides
- CI failure gating on regression thresholds (intentionally report-only)
