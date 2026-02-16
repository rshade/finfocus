# CI Workflow Contract: Benchmark Jobs

**Date**: 2026-02-16
**Feature**: `594-ci-benchmark-reporting`

## Overview

This contract defines the two benchmark jobs that replace the existing `benchmark` job in `.github/workflows/ci.yml`.

## Job 1: benchmark-baseline

**Trigger**: Push to `main` branch only
**Condition**: `github.ref == 'refs/heads/main'`

### Baseline Inputs

- Go source code at HEAD of `main`

### Baseline Steps

1. Checkout code (actions/checkout@v6)
2. Setup Go 1.25.7 (actions/setup-go@v6, cache: true)
3. Install benchstat (`go install golang.org/x/perf/cmd/benchstat@latest`)
4. Run benchmarks with filter:
   - Env: `FINFOCUS_LOG_LEVEL=error`
   - Command: `go test -bench='Benchmark(?!.*100K)' -benchtime=1x -count=5 -benchmem ./test/benchmarks/...`
   - Filter: pipe through `scripts/ci-benchmark-filter.sh`
   - Output: `test/benchmarks/baseline.txt`
5. Save cache (actions/cache/save@v5):
   - Path: `test/benchmarks/baseline.txt`
   - Key: `benchmark-baseline-${{ runner.os }}-go1.25.7`

### Baseline Outputs

- Cached baseline file

### Baseline Failure Behavior

- If benchmarks fail to compile or panic, the job fails (no suppression)
- Cache save runs only on success

## Job 2: benchmark-compare

**Trigger**: Pull request events only
**Condition**: `github.event_name == 'pull_request'`
**Permissions**: `pull-requests: write`

### Compare Inputs

- Go source code at PR HEAD
- Cached baseline file (optional — may not exist)

### Compare Steps

1. Checkout code (actions/checkout@v6)
2. Setup Go 1.25.7 (actions/setup-go@v6, cache: true)
3. Install benchstat
4. Restore baseline cache (actions/cache/restore@v5):
   - Key: `benchmark-baseline-${{ runner.os }}-go1.25.7`
   - Step output: `steps.baseline.outputs.cache-hit`
5. Run benchmarks with filter (same parameters as baseline):
   - Output: `test/benchmarks/current.txt`
6. Compare with benchstat (if baseline exists):
   - Command: `benchstat test/benchmarks/baseline.txt test/benchmarks/current.txt`
   - Output: `test/benchmarks/comparison.txt`
   - Parse regression severities (error: >50%, warning: 20-50%)
   - Set env vars: `REGRESSION_LEVEL`, `COMPARISON_BODY`
7. Post/update PR comment (actions/github-script@v7):
   - Find existing comment by `<!-- benchmark-results -->` marker
   - If baseline missing: post informational "no baseline" message
   - If baseline exists: post formatted comparison with severity indicators
   - Update existing comment if found; create new if not

### Compare Outputs

- PR comment with benchmark comparison (or informational message)

### Compare Failure Behavior

- If benchmarks fail to compile or panic, the job fails
- If baseline is missing, job passes with informational comment
- If benchstat comparison fails, job passes with raw results posted
- Regression detection never fails the job

## Filter Script Contract: scripts/ci-benchmark-filter.sh

**Type**: stdin/stdout filter (pipe-compatible)
**Input**: Raw `go test -bench` output (mixed benchmark lines + zerolog JSON + debug messages)
**Output**: benchstat-compatible lines only

### Accepted line patterns

```text
^goos:
^goarch:
^pkg:
^cpu:
^Benchmark
^ok\s
^PASS
```

### Rejected line patterns

- JSON objects (zerolog output): `^{`
- Debug/info messages
- Empty lines
- Any other non-benchmark output

### Exit code

- Always exits 0 (filter should not fail the pipeline)
- Empty input produces empty output
