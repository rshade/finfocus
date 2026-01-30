# Implementation Plan: Engine Test Coverage Completion

**Branch**: `007-engine-test-coverage` | **Date**: 2025-12-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/007-engine-test-coverage/spec.md`

## Summary

Complete the engine package test coverage to meet 80% threshold with emphasis on
quality over quantity. Focus on meaningful tests for functions with substantive
logic (tryStoragePricing, getDefaultMonthlyByType, parseFloatValue,
distributeDailyCosts) while avoiding slop tests for simple delegations. Extend
benchmarks to 1K, 10K, 100K resource scales.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: testing (stdlib), github.com/stretchr/testify
**Storage**: N/A (testing framework, no persistence)
**Testing**: `go test` with `-coverprofile`, `-race`, `-bench`
**Target Platform**: Linux, macOS, Windows (cross-platform via Go)
**Project Type**: Single project (CLI tool with internal packages)
**Performance Goals**: Test suite < 60s total, benchmarks at 1K/10K/100K scale
**Constraints**: Quality over coverage; no AI slop tests
**Scale/Scope**: Engine package at 78.5% → 80%+ coverage

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution:

- [x] **Plugin-First Architecture**: N/A - tests are internal to core,
      not a plugin implementation
- [x] **Test-Driven Development**: Tests are the primary deliverable; 80%
      coverage target aligns with Constitution requirement
- [x] **Cross-Platform Compatibility**: Go tests run on all platforms;
      no platform-specific test code planned
- [x] **Documentation as Code**: Test files serve as living documentation;
      each test comment documents purpose
- [x] **Protocol Stability**: N/A - no protocol changes, testing existing
      functionality
- [x] **Quality Gates**: Tests must pass with `-race`, coverage threshold
      enforced, linting required
- [x] **Multi-Repo Coordination**: N/A - contained within finfocus-core

**Violations Requiring Justification**:

| Principle                   | Deviation                        | Justification                                                                                                                                                                                                                                                                                                                                           |
| --------------------------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| II. TDD (95% critical path) | Target is 80% not 95% for engine | Anti-slop constraints explicitly prohibit coverage-chasing tests. Quality over quantity is a feature requirement. Achieving 95% would require testing simple delegations (GetActualCost, getStorageSize, tryFallbackNumericValue) which violates spec's anti-slop rules. The 80% target meets overall project threshold while maintaining test quality. |

## Project Structure

### Documentation (this feature)

```text
specs/007-engine-test-coverage/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── tasks.md             # Phase 2 output (created by /speckit.tasks)
└── checklists/
    └── requirements.md  # Quality checklist
```

### Source Code (repository root)

```text
internal/engine/
├── engine.go            # Core calculation logic (target for coverage)
├── engine_test.go       # Existing unit tests
├── types.go             # Type definitions
├── types_test.go        # Existing type tests
└── project.go           # Output rendering

test/
├── unit/engine/
│   ├── aggregation_test.go   # Cross-provider aggregation tests
│   ├── engine_test.go        # Engine unit tests
│   ├── errors_test.go        # Error type tests
│   └── render_test.go        # Rendering tests
├── integration/
│   └── plugin/               # Plugin integration tests
└── benchmarks/
    └── engine_bench_test.go  # Performance benchmarks
```

**Structure Decision**: Tests are distributed between `internal/engine/` (colocated
unit tests) and `test/` (external test suites for integration, benchmarks). This
follows existing project conventions.

## Complexity Tracking

> No Constitution violations - table not required.
