# Research: Benchmark PR Reporting with Regression Detection

**Date**: 2026-02-16
**Feature**: `594-ci-benchmark-reporting`

## R1: benchstat Output Format and Regression Parsing

**Decision**: Parse benchstat's tabular output for percentage delta values across all metrics (sec/op, B/op, allocs/op).

**Rationale**: benchstat v2 outputs a structured comparison table with columns for old, new, and delta values. The delta column contains percentage changes (e.g., "+12.3%", "-5.2%", "~") that can be parsed with grep/awk. The "~" marker indicates no statistically significant change. This provides reliable regression detection without custom statistical analysis.

**Alternatives considered**:

- Custom percentage calculation from raw numbers: Rejected — benchstat already handles statistical significance (p-value) with `-count=5`.
- JSON output from benchstat: Not available in the standard benchstat CLI.
- Third-party benchmark comparison actions: Rejected — adds dependency on unmaintained actions; benchstat is the Go ecosystem standard.

## R2: Zerolog Noise Filtering Strategy

**Decision**: Use `FINFOCUS_LOG_LEVEL=error` environment variable to suppress warn/info zerolog output during benchmark runs, combined with a grep-based filter script as a safety net.

**Rationale**: The project already supports `FINFOCUS_LOG_LEVEL` environment variable for log level control. Setting it to `error` eliminates most zerolog JSON noise at the source. The filter script (`scripts/ci-benchmark-filter.sh`) provides a secondary safety net by extracting only benchstat-compatible lines (goos, goarch, pkg, cpu, Benchmark*, ok, PASS).

**Alternatives considered**:

- Filter-only approach (no env var): Rejected — 46MB baseline.txt observed with zerolog pollution; filtering after the fact is wasteful and fragile.
- Redirecting stderr to /dev/null: Rejected — would also suppress legitimate benchmark errors.
- Modifying benchmark test code to disable logging: Rejected — violates spec boundary (no changes to existing benchmark functions).

## R3: GitHub Actions Cache Strategy for Baseline

**Decision**: Use `actions/cache/save@v5` on main push and `actions/cache/restore@v5` on PR, with a fixed cache key `benchmark-baseline-${{ runner.os }}-go1.25.8`.

**Rationale**: A fixed key ensures PRs always compare against the latest main baseline. Each main push overwrites the cache entry. Cache eviction (GitHub's 10GB repo limit, 7-day retention) is handled gracefully — if the baseline is missing, the PR job posts an informational comment.

**Alternatives considered**:

- Git-based storage (committing baseline.txt): Rejected — adds noise to git history, baseline changes on every main push.
- GitHub Artifacts: Rejected — artifacts are scoped to workflow runs and cannot be shared across workflows easily.
- SHA-based cache keys: Rejected — would create a new cache entry per commit, never matching PR restores.

## R4: PR Comment Update Strategy

**Decision**: Use `actions/github-script@v7` with a `<!-- benchmark-results -->` HTML comment marker to find and update existing comments.

**Rationale**: The GitHub API allows listing PR comments, finding one with the marker, and updating it via `PATCH`. This prevents comment duplication on subsequent pushes. The `actions/github-script` action provides inline JavaScript with the `github` and `context` objects, avoiding the need for a separate script.

**Alternatives considered**:

- Separate GitHub Actions for comment management (e.g., `marocchino/sticky-pull-request-comment`): Rejected — adds external dependency; inline script is simpler and more transparent.
- Always creating new comments: Rejected — violates FR-005 (comment deduplication).
- Using `gh` CLI for comment management: Viable but less ergonomic than `actions/github-script` for find-and-update logic.

## R5: Benchmark Selection Pattern for 100K Exclusion

**Decision**: Use Go test's `-run` exclusion via negative lookahead regex: `-bench='Benchmark(?!.*100K)'`.

**Rationale**: Go's `-bench` flag accepts regex patterns. The negative lookahead `(?!.*100K)` excludes any benchmark with "100K" in its name while including all others. This matches the 3 active 100K benchmarks (`BenchmarkScale100K`, `BenchmarkEngine_GetProjectedCost_100K`, `BenchmarkGeneratorOverhead/Large_100K`) without needing to maintain an explicit include list.

**Alternatives considered**:

- Explicit include list: Rejected — requires updating CI whenever new benchmarks are added.
- Build tags: Rejected — requires modifying benchmark source files (out of scope).
- `-skip` flag: Not available in Go's testing package as a direct flag; would need test-level skip logic.

## R6: Regression Threshold Detection Approach

**Decision**: Parse benchstat output lines for percentage values using awk/grep, categorize into error (>50%), warning (20-50%), or acceptable (<20%) severity, and set shell variables for the PR comment step.

**Rationale**: benchstat reports delta as percentage strings (e.g., "+45.2%", "-12.3%"). A shell script can extract these values, compare against thresholds, and output severity indicators. Since regressions are informational only (no CI failure), the logic is simple: scan for percentages, flag the highest severity found.

**Alternatives considered**:

- Dedicated regression detection tool: Rejected — over-engineered for report-only use case.
- benchstat's `-geomean` flag for single summary: Rejected — loses per-benchmark granularity needed for the PR comment table.

## R7: CI Workflow Permissions

**Decision**: Add `pull-requests: write` permission to the benchmark job to allow posting PR comments.

**Rationale**: The existing `test` job already has `pull-requests: write`. The benchmark job needs the same permission to create/update PR comments via the GitHub API. This is scoped to the job level, not the workflow level, following least-privilege.

**Alternatives considered**:

- Workflow-level permission: Rejected — would grant write access to all jobs unnecessarily.
- Using `GITHUB_TOKEN` without explicit permission: Would fail on PRs from forks.

## R8: Existing Infrastructure Compatibility

**Decision**: The existing `scripts/compare-benchmarks.sh` remains unchanged (FR-012). The new `scripts/ci-benchmark-filter.sh` is a separate file.

**Rationale**: `compare-benchmarks.sh` is used for local development and runs all benchmarks without filtering. It already supports benchstat when installed. The CI filter script has a different purpose (stripping non-benchstat lines from CI output) and should not modify the local workflow.

**Verification**: `compare-benchmarks.sh` does not reference any CI-specific patterns and will continue to work as-is.
