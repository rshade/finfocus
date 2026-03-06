# Implementation Plan: Benchmark PR Reporting with Regression Detection

**Branch**: `594-ci-benchmark-reporting` | **Date**: 2026-02-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/594-ci-benchmark-reporting/spec.md`

## Summary

Replace the current smoke-only benchmark job in `ci.yml` (lines 213-234) with a two-phase workflow: (1) baseline generation on `main` push, (2) PR comparison with benchstat. The PR comparison posts/updates a benchmark results comment with visual regression severity indicators. Regressions never fail CI; only benchmark execution failures (panics, compilation errors) fail the job. A new filter script strips zerolog noise from benchmark output for clean benchstat input.

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: GitHub Actions (actions/checkout@v6, actions/setup-go@v6, actions/cache@v5, actions/github-script@v7), golang.org/x/perf/cmd/benchstat
**Storage**: GitHub Actions cache (benchmark baseline file)
**Testing**: `go test -bench` with benchstat for statistical comparison
**Target Platform**: GitHub Actions ubuntu-latest runners
**Project Type**: CI/CD workflow (YAML + shell script)
**Performance Goals**: Benchmark comparison job completes in under 5 minutes
**Constraints**: Must not fail CI on regression detection; must handle cache miss gracefully
**Scale/Scope**: 28 benchmark functions across 5 files; excludes 100K-scale benchmarks

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: N/A — this feature modifies CI workflow configuration, not core/plugin code.
- [x] **Test-Driven Development**: The filter script will be validated by CI itself (self-testing). Workflow correctness is verified by opening a PR.
- [x] **Cross-Platform Compatibility**: N/A — CI workflows run on ubuntu-latest only. No Go source code changes.
- [x] **Documentation Integrity**: No README or docs changes needed; this is CI infrastructure.
- [x] **Protocol Stability**: N/A — no protocol buffer changes.
- [x] **Implementation Completeness**: All workflow steps fully implemented; no stubs or TODOs.
- [x] **Quality Gates**: `make lint` validates the filter script (shellcheck via markdownlint for .sh files). `actionlint` validates workflow YAML.
- [x] **Multi-Repo Coordination**: N/A — changes are confined to this repository's CI configuration.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/594-ci-benchmark-reporting/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 research output
├── data-model.md        # Phase 1 data model
├── quickstart.md        # Phase 1 quickstart guide
└── checklists/
    └── requirements.md  # Specification quality checklist
```

### Source Code (repository root)

```text
.github/workflows/
└── ci.yml                          # Modified: replace benchmark job (lines 213-234)

scripts/
├── ci-benchmark-filter.sh          # New: filter zerolog noise from benchmark output
└── compare-benchmarks.sh           # Unchanged: existing local comparison script

.gitignore                          # Modified: add benchmark artifact patterns
```

**Structure Decision**: This feature modifies existing CI workflow infrastructure and adds one new shell script. No new Go source code, packages, or test files are created.

## Complexity Tracking

No constitution violations. No complexity justification needed.
