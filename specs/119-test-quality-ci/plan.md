# Implementation Plan: Test Quality and CI/CD Improvements

**Branch**: `118-test-quality-ci` | **Date**: 2026-01-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/118-test-quality-ci/spec.md`

## Summary

Improve test quality and CI/CD infrastructure by: (1) adding `newState` seed corpus to fuzz tests for realistic Pulumi plan coverage, (2) fixing benchmark JSON structure to use valid `steps` format, (3) creating cross-repository integration workflow with nightly schedule and failure notifications, (4) implementing E2E tests for the `cost actual` command, and (5) fixing test error handling patterns across the codebase.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: testify v1.11.1, GitHub Actions, finfocus-spec v0.4.11 (pluginsdk)
**Storage**: N/A (test infrastructure, no persistent storage)
**Testing**: Go native testing with fuzz support, testify assertions, E2E via CLI binary execution
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module with test subdirectories
**Performance Goals**: Benchmarks report > 0 ns/op; fuzz tests run without panic for 30+ seconds
**Constraints**: CI workflow completes in < 10 minutes; no modifications to `.golangci.yml`
**Scale/Scope**: ~15 files modified across test/, internal/ingest/, .github/workflows/

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: N/A - This feature improves test infrastructure, not plugin functionality
- [x] **Test-Driven Development**: Tests are the primary deliverable; 80% coverage maintained
- [x] **Cross-Platform Compatibility**: GitHub Actions workflow tests on Linux; CLI already cross-platform
- [x] **Documentation Synchronization**: CLAUDE.md will be updated with any new test patterns
- [x] **Protocol Stability**: N/A - No protocol changes
- [x] **Implementation Completeness**: All test files will be complete implementations, no stubs
- [x] **Quality Gates**: `make lint` and `make test` will be run before completion
- [x] **Multi-Repo Coordination**: Cross-repo workflow documents dependencies on finfocus-plugin-aws-public

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/118-test-quality-ci/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (minimal - test fixtures)
├── quickstart.md        # Phase 1 output (verification commands)
└── checklists/
    └── requirements.md  # Quality checklist
```

### Source Code (repository root)

```text
internal/ingest/
├── fuzz_test.go         # Add newState seed corpus entries

test/
├── benchmarks/
│   └── parse_bench_test.go  # Fix JSON structure (steps vs resourceChanges)
├── e2e/
│   ├── gcp_test.go          # Fix filepath.Abs error handling
│   ├── output_ndjson_test.go # Add schema validation
│   └── actual_cost_test.go  # NEW: E2E test for cost actual command
├── fixtures/                 # EXISTING: Test fixtures (shared)
│   └── state/
│       ├── valid-state.json        # Existing fixture with timestamps
│       ├── imported-resources.json # Existing fixture for external resources
│       └── no-timestamps.json      # Existing fixture for edge case

.github/workflows/
└── cross-repo-integration.yml  # NEW: Cross-repo integration workflow
```

**Structure Decision**: Single Go project with test infrastructure improvements spread across existing test directories. One new workflow file for cross-repo CI/CD.

## Complexity Tracking

No constitution violations to justify.

## Phase 0: Research Complete

All technical decisions documented in [research.md](./research.md):

1. **Fuzz Test Seed Corpus**: Add inline `newState` entries using existing `f.Add()` pattern
2. **Benchmark JSON Structure**: Replace `resourceChanges` with `steps` array format
3. **Cross-Repo Workflow**: New workflow with `workflow_dispatch` + nightly schedule
4. **E2E Test Fixtures**: Reuse existing `test/fixtures/state/` files (no new fixtures needed)
5. **Test Error Handling**: Use `require.NoError(t, err)` for fallible operations
6. **NDJSON Schema Validation**: Add `assert.Contains` for `resource_type` and `currency`

## Phase 1: Design Artifacts Complete

Generated artifacts:

- [data-model.md](./data-model.md) - Test fixture schemas and workflow configuration
- [quickstart.md](./quickstart.md) - Verification commands and success criteria
- [contracts/README.md](./contracts/README.md) - References to existing contracts (no new APIs)

## Post-Design Constitution Re-Check

- [x] **Plugin-First Architecture**: Verified - no plugin modifications
- [x] **Test-Driven Development**: Verified - tests are the deliverable
- [x] **Cross-Platform Compatibility**: Verified - workflow runs on ubuntu-latest
- [x] **Documentation Synchronization**: Verified - CLAUDE.md updated via agent script
- [x] **Protocol Stability**: Verified - no protocol changes
- [x] **Implementation Completeness**: Verified - no stubs planned
- [x] **Quality Gates**: Verified - lint/test commands in quickstart.md
- [x] **Multi-Repo Coordination**: Verified - workflow documents plugin dependency

## Ready for Task Generation

Run `/speckit.tasks` to generate the implementation task list.
