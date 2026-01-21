# Implementation Plan - Harden Nightly Analysis Workflow

**Feature**: Harden Nightly Analysis Workflow
**Spec**: [specs/121-harden-nightly-analysis/spec.md](spec.md)
**Status**: Approved

## Technical Context

This feature hardens the existing `.github/workflows/nightly-analysis.yml` workflow by implementing strict version pinning, timeouts, and comprehensive error handling. The workflow relies on the `@opencode/cli` tool and Go scripts for analysis.

**Key Technical Decisions**:
- **Authentication**: Use `GITHUB_TOKEN` with explicit permissions for cross-repo access (simpler maintenance than PATs).
- **Versioning**: Pin `@opencode/cli` to a specific version (developer to verify exact version at runtime, placeholder `1.2.3` used in plan).
- **Timeouts**: Enforce a 59-minute timeout to balance resource protection with analysis needs.
- **Error Handling**: Shell-level exit code checks (`set -e` or explicit checks) for all commands.

**Unknowns & Risks**:
- **Resolved**: `actions/checkout@v6` is the project standard.
- **Resolved**: `@opencode/cli` version cannot be queried automatically; will be determined manually during implementation.

## Constitution Check (Pre-Plan)

- [x] **I. Plugin-First Architecture**: N/A (CI workflow change, not core logic).
- [x] **II. Test-Driven Development**: Workflow changes will be verified by "Independent Test" scenarios defined in the spec (manual/dry-run verification).
- [x] **III. Cross-Platform Compatibility**: N/A (GitHub Actions Linux runner).
- [x] **IV. Documentation Synchronization**: Will update `docs/analyzer-integration.md` or equivalent if it describes the nightly workflow.
- [x] **V. No Breaking Changes**: Hardening existing workflow, no public API changes.
- [x] **VI. Implementation Completeness**: Plan includes full error handling and configuration, no TODOs.

## Phase 0: Research & Design

### Research Tasks

- [x] **Research Dependency Versions**: Confirmed `actions/checkout@v6` is standard.
- [x] **Research Permissions**: Confirmed `permissions: contents: read` strategy.

### Design Artifacts

- **research.md**: Findings recorded.
- **data-model.md**: N/A (No new data entities).
- **contracts/**: N/A (No new APIs).

## Phase 1: Implementation

### Tasks

1. **Update Workflow Configuration**:
   - Modify `.github/workflows/nightly-analysis.yml`.
   - Update `actions/checkout` to `v6`.
   - Add `timeout-minutes: 59` to the job.
   - Pin `@opencode/cli` installation to a specific version (e.g., `latest` -> `1.2.3` - need to find working version).
   - Add `permissions: contents: read`.

2. **Harden Scripts**:
   - Wrap `go run` and `opencode run` commands in strict error checking blocks.
   - Verify `analysis.md` file existence and size (`-s`) before posting.

3. **Verify Cross-Repo Access**:
   - Ensure the checkout step for secondary repos uses the correct token/auth method if applicable (spec says GITHUB_TOKEN with permissions).

## Phase 2: Verification

### Tasks

1. **Dry Run**:
   - Manually trigger the workflow (if possible) or push to a branch to test syntax.
   - Verify timeout setting appears in logs (metadata).
   - Verify correct package versions are installed.

2. **Failure Simulation**:
   - Temporarily introduce a failure in the script to ensure the job fails red (not green/silent).

## Phase 3: Documentation

### Tasks

1. **Update Docs**:
   - Update `docs/analyzer-integration.md` (or relevant CI docs) to reflect the new hardening standards (timeouts, pinning).
