# Tasks: Harden Nightly Analysis Workflow

**Feature Branch**: `121-harden-nightly-analysis`
**Status**: In Progress

## Phase 1: Setup

1. **Project Initialization**
   - [x] T001 Determine stable @opencode/cli version manually and record it in research.md

## Phase 2: Foundational

1. **Workflow Configuration**
   - [x] T002 Update .github/workflows/nightly-analysis.yml to use actions/checkout@v6 and set permissions: contents: read

## Phase 3: User Story 1 (Reliable Failure Reporting)

**Goal**: Ensure workflow fails explicitly on any error and avoids silent failures.
**Independent Test**: Introduce failure in script/CLI and verify job fails.

1. **Error Handling Implementation**
   - [x] T003 [US1] Update .github/workflows/nightly-analysis.yml to fail on Go script failure (exit code check)
   - [x] T004 [US1] Update .github/workflows/nightly-analysis.yml to fail on opencode CLI failure (exit code check)
   - [x] T005 [US1] Update .github/workflows/nightly-analysis.yml to check analysis.md existence and non-zero size before posting

## Phase 4: User Story 2 (Reproducible & Secure Execution)

**Goal**: Pin dependencies for reproducible builds.
**Independent Test**: Verify configuration uses specific versions.

1. **Dependency Pinning**
   - [x] T006 [US2] Update .github/workflows/nightly-analysis.yml to install specific version of @opencode/cli (from T001)

## Phase 5: User Story 3 (Cross-Repository Integration)

**Goal**: Ensure workflow can access required repositories.
**Independent Test**: Verify logs show successful access/cloning.

1. **Cross-Repo Access**
   - [x] T007 [US3] Verify and ensure checkout step in .github/workflows/nightly-analysis.yml uses correct token/permissions for cross-repo access

## Phase 6: User Story 4 (Resource Protection)

**Goal**: Prevent zombie jobs with timeouts.
**Independent Test**: Verify timeout setting in workflow logs/metadata.

1. **Timeout Configuration**
   - [x] T008 [US4] Add timeout-minutes: 59 to .github/workflows/nightly-analysis.yml

## Phase 7: Polish & Documentation

1. **Documentation**
   - [x] T009 Update docs/analyzer-integration.md to reflect new hardening standards (timeouts, pinning) and run documentation linting (e.g., make docs-lint)

2. **Final Verification**
   - [x] T010 Perform manual dry-run/validation of the hardened workflow, verifying all Acceptance Scenarios in spec.md are met

## Dependencies

- Phase 2 (Foundational) blocks all US phases.
- US1, US2, US3, US4 can be implemented in parallel after Phase 2 (modifying same file, but distinct sections/properties).

## Parallel Execution Examples

- **Developer A**: Implements T003, T004, T005 (Error Handling)
- **Developer B**: Implements T006, T008 (Pinning & Timeouts)

## Implementation Strategy

- **MVP**: Complete US1 (Error Handling) and US4 (Timeouts) first for immediate stability.
- **Incremental**: Add Pinning (US2) and Cross-Repo (US3) improvements subsequently.
- **Note**: Principle VI (Implementation Completeness) requires all tasks to be fully implemented, no TODOs.