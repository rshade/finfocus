# Feature Specification: Harden Nightly Analysis Workflow

**Feature Branch**: `121-harden-nightly-analysis`  
**Created**: 2026-01-20  
**Status**: Draft  
**Input**: User description: "title: ci: Harden Nightly Analysis Workflow security and reliability state: OPEN author: rshade labels: ci, roadmap/current comments: 1 assignees: projects: milestone: number: 325 -- ## Context The new .github/workflows/nightly-analysis.yml workflow introduced in PR #315 lacks security pinning, resource limits, and robust error handling. ## Research - **Unpinned Dependency**: The workflow installs @opencode/cli using latest, creating non-reproducible builds. - **No Timeouts**: The analyze-failure job has no timeout-minutes set, risking long-running hangs (default 6h). - **Missing Error Handling**: - go run scripts/analysis/analyze_failure.go is not checked for failure. - opencode run is not checked for failure. - analysis.md is checked with -f (exists) instead of -s (non-empty), potentially posting empty comments. - **Stale Action**: Uses actions/checkout@v4 instead of the project standard v6. ## Plan 1. **Pin Dependencies**: Update install command to npm install -g @opencode/cli@<VERSION>. 2. **Set Limits**: Add timeout-minutes: 10 to the job configuration. 3. **Error Handling**: Wrap commands in if ! cmd; then exit 1; fi blocks. Change file check to if [ -s analysis.md ]. 4. **Update Action**: Upgrade to actions/checkout@v6."
**Updates**:
- Increased timeout to 59 minutes.
- Added requirement to fix Cross-Repository Integration Failure.

## Clarifications

### Session 2026-01-20

- Q: Which authentication method should be used for cross-repository operations? → A: Option A (GITHUB_TOKEN with explicit permissions).
- Q: What version strategy should be used for pinning @opencode/cli? → A: Option A (Current Stable found during implementation).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reliable Failure Reporting (Priority: P1)

As a maintainer, I want the nightly analysis workflow to fail explicitly if any step encounters an error, so that I am not misled by false positive success states.

**Why this priority**: Silent failures in CI workflows mask real issues, defeating the purpose of nightly analysis.

**Independent Test**: Can be tested by intentionally introducing a failure in the analysis script and verifying the workflow reports a failure status.

**Acceptance Scenarios**:

1. **Given** the analysis script encounters a runtime error, **When** the workflow executes, **Then** the job MUST fail and stop execution.
2. **Given** the CLI tool returns a non-zero exit code, **When** the workflow executes, **Then** the job MUST fail.
3. **Given** the analysis report file is empty, **When** the comment step runs, **Then** no comment is posted to the issue.

---

### User Story 2 - Reproducible & Secure Execution (Priority: P2)

As a DevOps engineer, I want the workflow to use pinned versions of dependencies and actions, so that builds are reproducible and immune to breaking changes in upstream tools.

**Why this priority**: "latest" tags can introduce breaking changes unexpectedly, causing CI instability unrelated to code changes.

**Independent Test**: Verify the workflow configuration uses specific version numbers for all external tools and actions.

**Acceptance Scenarios**:

1. **Given** a new version of `@opencode/cli` is released, **When** the workflow runs, **Then** it continues to use the specifically defined version, not the new one.
2. **Given** the workflow configuration, **When** inspected, **Then** it uses the project-standard version of `actions/checkout`.

---

### User Story 3 - Cross-Repository Integration (Priority: P1)

As a developer, I want the workflow to successfully access and integrate with required external repositories, so that analysis can be performed across the full project scope without permission errors.

**Why this priority**: Critical functionality; the analysis fails if it cannot access necessary repositories.

**Independent Test**: Trigger the workflow and verify logs show successful access/cloning of secondary repositories.

**Acceptance Scenarios**:

1. **Given** the workflow needs to access a separate repository, **When** the step executes, **Then** it authenticates successfully and performs the required action (e.g., checkout/read) without a 403 or "Repository not found" error.

---

### User Story 4 - Resource Protection (Priority: P3)

As a system administrator, I want the analysis job to have a reasonable timeout (59 minutes) if it hangs, so that we don't waste compute resources on stuck processes while allowing enough time for legitimate long-running tasks.

**Why this priority**: Prevents "zombie" jobs from consuming GitHub Actions minutes (up to 6 hours by default) but provides ample buffer for heavy analysis.

**Independent Test**: Verify the workflow configuration includes a timeout setting.

**Acceptance Scenarios**:

1. **Given** the analysis process hangs indefinitely, **When** 59 minutes pass, **Then** the job is automatically terminated by the system.

### Edge Cases

- What happens when the `analysis.md` file is created but contains only whitespace?
- How does the system handle network timeouts during tool installation?
- What happens if the cross-repository token expires or permissions are revoked?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST use a fixed, specific version (the current stable release at time of implementation) for the `@opencode/cli` tool installation.
- **FR-002**: The system MUST enforce a maximum execution time limit of 59 minutes for the analysis job.
- **FR-003**: The system MUST detect non-zero exit codes from the analysis script and fail the workflow step immediately.
- **FR-004**: The system MUST detect non-zero exit codes from the `opencode` CLI execution and fail the workflow step immediately.
- **FR-005**: The system MUST verify that the analysis report file has non-zero size before attempting to post it.
- **FR-006**: The system MUST use the project-standard version (v6) for the checkout action.
- **FR-007**: The system MUST utilize the default `GITHUB_TOKEN` with explicitly configured permissions (e.g., `contents: read` for target repositories) to enable cross-repository operations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Workflow jobs execute with a defined timeout of 59 minutes.
- **SC-002**: 100% of command failures in the workflow result in a failed job status (no silent failures).
- **SC-003**: Zero occurrences of empty comments posted to issues from this workflow.
- **SC-004**: External dependencies (CLI tools, Actions) are pinned to specific versions in the configuration.
- **SC-005**: Workflow successfully completes cross-repository actions (0% authentication failure rate).
