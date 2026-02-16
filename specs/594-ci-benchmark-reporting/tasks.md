# Tasks: Benchmark PR Reporting with Regression Detection

**Input**: Design documents from `/specs/594-ci-benchmark-reporting/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: This feature modifies CI workflow configuration (YAML + shell script), not Go source code. Testing is performed by CI validation tools (actionlint, markdownlint) and by exercising the workflow on a real PR. No Go unit tests are generated.

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV, no README or docs changes are needed — this is CI infrastructure.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Shared infrastructure for all benchmark jobs

- [x] T001 [P] Update `.gitignore` to add benchmark artifact patterns: `test/benchmarks/baseline.txt`, `test/benchmarks/current.txt`, `test/benchmarks/comparison.txt`
- [x] T002 [P] Create `scripts/ci-benchmark-filter.sh` as a stdin/stdout pipe filter that extracts only benchstat-compatible lines (goos, goarch, pkg, cpu, Benchmark\*, ok, PASS) using grep, set executable permission, and ensure it always exits 0

**Checkpoint**: Filter script and gitignore ready — workflow jobs can now reference the filter

---

## Phase 2: US3 - Baseline Refreshes on Main Push (Priority: P2)

**Goal**: Generate and cache a benchmark baseline on every push to `main` so PR comparisons have a reference point.

**Independent Test**: Push a commit to `main` and verify the `benchmark-baseline` job runs, produces `baseline.txt`, and saves it to GitHub Actions cache.

**Why US3 before US1**: The baseline generation job is a logical prerequisite — the PR comparison job restores from this cache. Implementing baseline first ensures a cached file exists for comparison testing.

- [x] T003 [US3] Remove the existing `benchmark` job (lines 213-234) from `.github/workflows/ci.yml` — delete the entire `benchmark:` block including `name: Benchmark (Smoke)`, both `Run benchmark smoke tests` and `Run generator benchmarks` steps
- [x] T004 [US3] Add `benchmark-baseline` job to `.github/workflows/ci.yml` with `timeout-minutes: 10` and condition `if: github.ref == 'refs/heads/main'` — include checkout (actions/checkout@v6), Go 1.25.7 setup (actions/setup-go@v6 with cache), benchstat install (`go install golang.org/x/perf/cmd/benchstat@latest`), benchmark execution with `FINFOCUS_LOG_LEVEL=error` env var using `-bench='Benchmark(?!.*100K)' -benchtime=1x -count=5 -benchmem ./test/benchmarks/...` piped through `scripts/ci-benchmark-filter.sh` to `test/benchmarks/baseline.txt`, and cache save (actions/cache/save@v5) with path `test/benchmarks/baseline.txt` and key `benchmark-baseline-${{ runner.os }}-go1.25.7`

**Checkpoint**: Baseline generation job is complete. Pushing to `main` will produce and cache a baseline file.

---

## Phase 3: US1 - Contributor Sees Performance Impact on PR (Priority: P1)

**Goal**: Post a benchstat comparison comment on every PR showing performance differences against the `main` baseline.

**Independent Test**: Open a PR and verify a benchmark comparison comment appears. Push another commit and verify the comment is updated (not duplicated). Open a PR when no baseline cache exists and verify an informational message is posted.

- [x] T005 [US1] Add `benchmark-compare` job to `.github/workflows/ci.yml` with `timeout-minutes: 10`, condition `if: github.event_name == 'pull_request'`, and `permissions: pull-requests: write` — include checkout, Go 1.25.7 setup with cache, and benchstat install steps identical to baseline job
- [x] T006 [US1] Add baseline cache restore step to `benchmark-compare` job using `actions/cache/restore@v5` with id `baseline` and key `benchmark-baseline-${{ runner.os }}-go1.25.7`
- [x] T007 [US1] Add benchmark execution step to `benchmark-compare` job with identical parameters to baseline job (`FINFOCUS_LOG_LEVEL=error`, same `-bench`/`-benchtime`/`-count`/`-benchmem` flags, same filter script), outputting to `test/benchmarks/current.txt`
- [x] T008 [US1] Add benchstat comparison step to `benchmark-compare` job that checks if `test/benchmarks/baseline.txt` exists, runs `benchstat test/benchmarks/baseline.txt test/benchmarks/current.txt > test/benchmarks/comparison.txt 2>&1`, and reads comparison output into a `COMPARISON_BODY` environment variable for the next step
- [x] T009 [US1] Add PR comment posting step using `actions/github-script@v7` that: (1) lists PR comments via `github.rest.issues.listComments`, (2) finds an existing comment containing `<!-- benchmark-results -->` marker, (3) builds a comment body with the marker, `## Benchmark Results` heading, benchstat comparison table, and full output in a collapsible `<details>` section, (4) updates the existing comment via `github.rest.issues.updateComment` or creates a new one via `github.rest.issues.createComment`
- [x] T010 [US1] Handle the no-baseline edge case in the PR comment step — when `steps.baseline.outputs.cache-hit != 'true'`, post an informational comment: "No baseline available yet. Benchmark results will be compared once the main branch baseline is generated." with the current benchmark raw results

**Checkpoint**: PRs now receive benchmark comparison comments. Comments are deduplicated on subsequent pushes. No-baseline case is handled gracefully.

---

## Phase 4: US2 - CI Reports Severe Performance Regression (Priority: P1)

**Goal**: Parse benchstat delta percentages and add visual severity indicators (error >50%, warning 20-50%, improvement <-20%) to the PR comment.

**Independent Test**: Introduce an artificial slowdown in a benchmark and verify the PR comment shows the appropriate severity indicator. Verify the CI job still passes (regression never fails CI).

- [x] T011 [US2] Add regression threshold parsing to the benchstat comparison step in `.github/workflows/ci.yml` — use awk/grep to extract percentage delta values from `test/benchmarks/comparison.txt`, categorize each as error (positive delta > 50% = regression), warning (positive delta 20-50% = regression), normal (positive delta < 20%), or improved (negative delta beyond -20%), and track the highest severity found across all metrics (sec/op, B/op, allocs/op) in a `REGRESSION_LEVEL` environment variable (values: `error`, `warning`, `none`, `improved`). Only positive deltas indicate regressions; negative deltas indicate improvements.
- [x] T012 [US2] Enhance the PR comment body construction in the `actions/github-script` step to include severity indicators — add a summary line showing the highest severity level (e.g., "2 benchmarks showed >50% regression", "1 benchmark showed >20% regression", "All benchmarks within noise", "3 benchmarks improved"), and annotate individual benchmark rows in the comparison with their severity status

**Checkpoint**: PR comments now include severity indicators. Regressions are visibly flagged but never fail CI.

---

## Phase 5: US4 - Benchmark Crash Surfaces as CI Failure (Priority: P3)

**Goal**: Ensure benchmark execution failures (panics, compilation errors) are not silently suppressed and properly fail the CI job.

**Independent Test**: Verify that neither `benchmark-baseline` nor `benchmark-compare` jobs contain `|| true`, `set +e`, or any other failure suppression pattern for the benchmark execution step. The filter script exits 0 but the `go test` command itself must propagate failures.

- [x] T013 [US4] Verify and ensure both `benchmark-baseline` and `benchmark-compare` jobs in `.github/workflows/ci.yml` do not suppress benchmark execution failures — confirm no `|| true` after `go test`, no `continue-on-error: true` on benchmark steps, and that `set -e` (or default shell behavior) ensures panics and compilation errors fail the job. Use `set -eo pipefail` in the benchmark run steps to ensure pipe failures from `go test` propagate through the filter script.

**Checkpoint**: Benchmark crashes and compilation errors now fail CI instead of being silently ignored.

---

## Phase 6: Polish & Validation

**Purpose**: Final validation across all modified files

- [x] T014 [P] Run `actionlint` to validate `.github/workflows/ci.yml` syntax and fix any issues
- [x] T015 [P] Run `make lint` to validate all modified files (markdownlint for specs, actionlint for workflows)
- [x] T016 [P] Verify `scripts/compare-benchmarks.sh` still works unchanged by reviewing it has no references to CI-specific patterns (FR-012)

**Checkpoint**: All validation passes. Feature is ready for PR.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **US3 Baseline (Phase 2)**: Depends on T002 (filter script) — must be complete before PR comparison can be tested with real data
- **US1 PR Comparison (Phase 3)**: Depends on Phase 1 completion (filter script exists) — can be tested independently (handles no-baseline gracefully)
- **US2 Severity (Phase 4)**: Depends on T008 (comparison step exists) — augments the comparison output
- **US4 Crash Surfacing (Phase 5)**: Depends on T004 and T005 (both jobs exist) — verification task
- **Polish (Phase 6)**: Depends on all implementation phases being complete

### User Story Dependencies

- **US3 (P2)**: Can start after T002 — no dependencies on other stories
- **US1 (P1)**: Can start after T002 — independent of US3 (handles no-baseline case)
- **US2 (P1)**: Depends on US1 T008 (comparison step must exist to add severity parsing)
- **US4 (P3)**: Depends on US3 + US1 (both jobs must exist to verify no suppression)

### Parallel Opportunities

- **Phase 1**: T001 and T002 can run in parallel (different files)
- **Phase 2-3**: US3 (T003-T004) and US1 (T005-T010) can start in parallel after Phase 1, since both modify different sections of `ci.yml` and US1 handles no-baseline independently
- **Phase 6**: T014, T015, T016 can all run in parallel (independent validation checks)

---

## Parallel Example: Phase 1

```text
# Launch both setup tasks together (different files):
Task: "Update .gitignore with benchmark artifact patterns"
Task: "Create scripts/ci-benchmark-filter.sh"
```

## Parallel Example: US3 + US1

```text
# After Phase 1, both stories can start (different ci.yml sections):
Task: "Remove old benchmark job from ci.yml"  (US3)
Task: "Add benchmark-compare job skeleton"     (US1)
# Note: US1 handles no-baseline gracefully, so it doesn't block on US3 completing
```

---

## Implementation Strategy

### MVP First (US3 + US1 Only)

1. Complete Phase 1: Setup (filter script + gitignore)
2. Complete Phase 2: US3 (baseline generation on main push)
3. Complete Phase 3: US1 (PR comparison comment)
4. **STOP and VALIDATE**: Open a test PR and verify the comparison comment appears
5. This is a fully functional MVP — PRs get benchmark comparison comments

### Incremental Delivery

1. Complete Setup → Filter script ready
2. Add US3 (baseline) + US1 (comparison) → PRs get benchmark comparison comments (MVP)
3. Add US2 (severity) → PR comments now flag regressions with visual indicators
4. Add US4 (crash surfacing) → Verify no failure suppression
5. Polish → Run validation, confirm FR-012 compatibility

---

## Notes

- All tasks modify only 3 files: `.github/workflows/ci.yml`, `scripts/ci-benchmark-filter.sh`, `.gitignore`
- No Go source code changes — no unit tests needed
- US3 is P2 but is implemented before P1 stories because the baseline cache is logically needed for comparison
- US1 can be independently tested even without a baseline (informational comment)
- US2 augments US1's comparison step — cannot be implemented before US1's T008
- US4 is inherently satisfied by the new job design (no `|| true`) but should be explicitly verified
- The `benchmark-compare` job needs `permissions: pull-requests: write` to post comments
- `FINFOCUS_LOG_LEVEL=error` suppresses zerolog noise at the source; the filter script is a safety net
