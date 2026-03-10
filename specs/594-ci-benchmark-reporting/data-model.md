# Data Model: Benchmark PR Reporting

**Date**: 2026-02-16
**Feature**: `594-ci-benchmark-reporting`

## Entities

### Baseline File

A text file containing benchstat-compatible benchmark output from the `main` branch.

**Location**: GitHub Actions cache (key: `benchmark-baseline-Linux-go1.25.8`)
**Local path**: `test/benchmarks/baseline.txt` (cached artifact, not committed)

**Format** (benchstat-compatible Go test output):

```text
goos: linux
goarch: amd64
pkg: github.com/rshade/finfocus/test/benchmarks
cpu: [CPU model]
BenchmarkScale1K-2           5     234567890 ns/op    12345678 B/op     123456 allocs/op
BenchmarkScale10K-2          5    2345678901 ns/op   123456789 B/op    1234567 allocs/op
...
PASS
ok      github.com/rshade/finfocus/test/benchmarks    45.123s
```

**Lifecycle**:

1. Created: On every push to `main` branch
2. Cached: Stored in GitHub Actions cache with fixed key (overwritten each time)
3. Consumed: Restored during PR benchmark comparison
4. Evicted: GitHub cache eviction policy (10GB repo limit, 7-day inactivity)

### Current Results File

A text file containing benchstat-compatible benchmark output from the PR branch.

**Location**: `test/benchmarks/current.txt` (ephemeral, exists only during CI job)

**Format**: Identical to Baseline File format.

**Lifecycle**:

1. Created: During PR benchmark comparison job
2. Consumed: Passed to benchstat for comparison
3. Discarded: Not cached or uploaded (ephemeral)

### Comparison Report

The benchstat diff output comparing baseline vs current results.

**Location**: `test/benchmarks/comparison.txt` (ephemeral, exists only during CI job)

**Format** (benchstat v2 output):

```text
                          │ baseline.txt │          current.txt           │
                          │    sec/op    │   sec/op     vs base           │
Scale1K-2                   234.6m ± 5%   245.8m ± 3%  +4.77% (p=0.008)
Scale10K-2                  2.346  ± 4%   2.567  ± 2%  +9.42% (p=0.008)
...
```

**Lifecycle**:

1. Created: By benchstat during PR comparison
2. Parsed: Shell script extracts severity indicators
3. Posted: Included in PR comment body
4. Discarded: Not cached or uploaded

### PR Comment

A GitHub PR comment containing formatted benchmark results with severity indicators.

**Identification**: `<!-- benchmark-results -->` HTML comment marker (first line of comment body)

**Structure**:

```markdown
<!-- benchmark-results -->
## Benchmark Results

[severity summary line]

| Benchmark | Old | New | Delta | Status |
|-----------|-----|-----|-------|--------|
| ...       | ... | ... | ...   | ...    |

<details>
<summary>Full benchstat output</summary>

[raw benchstat output in code block]

</details>
```

**Lifecycle**:

1. Created: First benchmark run on a PR
2. Updated: Subsequent pushes to the same PR (find by marker, update body)
3. Closed: When PR is merged or closed (comment persists)

## Relationships

```text
main push → [Baseline File] → GitHub Actions Cache
                                      ↓
PR push → [Current Results] → benchstat ← [Baseline File (restored)]
                                 ↓
                         [Comparison Report]
                                 ↓
                      Parse thresholds (20%/50%)
                                 ↓
                          [PR Comment]
                     (create or update by marker)
```

## Severity Classification

Severity is determined by scanning benchstat delta percentages:

| Delta Range       | Severity | Visual Indicator        |
| ----------------- | -------- | ----------------------- |
| > +50%            | Error    | Red indicator in comment |
| +20% to +50%     | Warning  | Yellow indicator in comment |
| -20% to +20%     | Normal   | No indicator            |
| < -20%           | Improved | Green indicator in comment |

Applied across all metrics: sec/op, B/op, allocs/op. The highest severity across all benchmarks determines the summary line.
