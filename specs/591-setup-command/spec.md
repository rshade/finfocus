# Feature Specification: finfocus setup — One-Command Bootstrap

**Feature Branch**: `591-setup-command`
**Created**: 2026-02-14
**Status**: Draft
**Input**: GitHub Issue #598 — Provide a single `finfocus setup` command that bootstraps the entire FinFocus environment for a new user

## Clarifications

### Session 2026-02-14

- Q: What prompts appear in interactive mode that `--non-interactive` suppresses? → A: No prompts — setup runs to completion automatically without user interaction. The `--non-interactive` flag controls TTY-dependent behavior only (e.g., status symbols, color output) and explicitly signals CI/CD intent.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First-Time Setup (Priority: P1)

A new user installs FinFocus and runs `finfocus setup` to bootstrap their environment. The command creates all required directories, writes a default configuration file, installs the Pulumi analyzer, and installs the default plugin set. The user sees a clear status line for each step and a summary message at the end.

**Why this priority**: This is the core value proposition — eliminating the multi-step manual setup process and reducing time-to-first-cost-estimate from minutes of manual configuration to a single command.

**Independent Test**: Can be fully tested by running `finfocus setup` on a clean system (no `~/.finfocus/` directory) and verifying all directories, config, analyzer, and plugins are present afterward.

**Acceptance Scenarios**:

1. **Given** a system with no `~/.finfocus/` directory, **When** the user runs `finfocus setup`, **Then** the command creates `~/.finfocus/`, `~/.finfocus/plugins/`, `~/.finfocus/cache/`, and `~/.finfocus/logs/` directories, writes a default `config.yaml`, installs the Pulumi analyzer, installs default plugins, and prints a success summary with a next-steps hint.
2. **Given** a system with no `~/.finfocus/` directory and Pulumi CLI not installed, **When** the user runs `finfocus setup`, **Then** the command prints a warning about missing Pulumi but continues with all other steps, and exits successfully.
3. **Given** a system with no `~/.finfocus/` directory, **When** the user runs `finfocus setup`, **Then** each completed step displays a status line indicating success or warning.

---

### User Story 2 - Idempotent Re-Run (Priority: P1)

A user who has already run `finfocus setup` runs it again (e.g., after an upgrade or to verify their environment). The command detects existing directories and config, skips creation of already-present resources, and reports their status without errors or data loss.

**Why this priority**: Idempotency is essential for user confidence — users must never fear that running setup will overwrite their customized config or corrupt their environment.

**Independent Test**: Run `finfocus setup` twice in succession on the same environment and verify no errors, no data loss, and existing config is preserved.

**Acceptance Scenarios**:

1. **Given** a system where `finfocus setup` has already been run, **When** the user runs `finfocus setup` again, **Then** existing directories are detected (not recreated), the existing `config.yaml` is preserved, and the command reports that each component is already present.
2. **Given** a system with a customized `config.yaml`, **When** the user runs `finfocus setup`, **Then** the existing config file is not overwritten or modified.
3. **Given** a system where the analyzer is already installed at the current version, **When** the user runs `finfocus setup`, **Then** the analyzer step reports it is already current without reinstalling.

---

### User Story 3 - CI/CD Non-Interactive Setup (Priority: P2)

A DevOps engineer runs `finfocus setup --non-interactive` in a CI/CD pipeline or Docker build to bootstrap FinFocus with TTY-independent output (no color, no status symbols). The command runs to completion automatically and exits with a clear status code.

**Why this priority**: Automated environments are a key deployment target, and non-interactive mode enables container builds, CI pipelines, and scripted provisioning.

**Independent Test**: Run `finfocus setup --non-interactive` with stdin redirected from `/dev/null` and verify it completes without hanging or prompting.

**Acceptance Scenarios**:

1. **Given** a CI/CD pipeline with no TTY, **When** `finfocus setup --non-interactive` is executed, **Then** the command completes without prompts, uses all defaults, and returns exit code 0 on success.
2. **Given** a Docker build step, **When** `finfocus setup --non-interactive --skip-plugins` is executed, **Then** the command skips plugin installation and completes successfully.

---

### User Story 4 - Selective Setup with Skip Flags (Priority: P2)

A user wants to set up only parts of the environment — for example, they want directories and config but not the analyzer or plugins. They use `--skip-analyzer` and/or `--skip-plugins` flags to control which steps are executed.

**Why this priority**: Flexibility allows users to tailor setup to their specific needs — e.g., offline environments without plugin downloads, or environments where the analyzer is managed separately.

**Independent Test**: Run `finfocus setup --skip-analyzer --skip-plugins` and verify only directories and config are created.

**Acceptance Scenarios**:

1. **Given** a clean system, **When** the user runs `finfocus setup --skip-analyzer`, **Then** directories and config are created, plugins are installed, but the analyzer installation step is skipped entirely.
2. **Given** a clean system, **When** the user runs `finfocus setup --skip-plugins`, **Then** directories, config, and analyzer are set up, but no plugins are downloaded or installed.
3. **Given** a clean system, **When** the user runs `finfocus setup --skip-analyzer --skip-plugins`, **Then** only directories and config are created.

---

### User Story 5 - Custom Home Directory (Priority: P3)

A user or organization uses a custom FinFocus home directory via the `FINFOCUS_HOME` environment variable. The setup command respects this override and creates all resources under the custom path instead of `~/.finfocus/`.

**Why this priority**: Supports enterprise deployments and multi-tenant environments where the default home directory is not appropriate.

**Independent Test**: Set `FINFOCUS_HOME=/custom/path` and run `finfocus setup`, then verify all resources are created under `/custom/path/`.

**Acceptance Scenarios**:

1. **Given** `FINFOCUS_HOME=/opt/finfocus` is set, **When** the user runs `finfocus setup`, **Then** all directories and config are created under `/opt/finfocus/` instead of `~/.finfocus/`.
2. **Given** `PULUMI_HOME=/opt/pulumi` is set (and `FINFOCUS_HOME` is not), **When** the user runs `finfocus setup`, **Then** all resources are created under `/opt/pulumi/finfocus/`.

---

### Edge Cases

- What happens when the user lacks write permissions to the target directory? The command reports a clear error with the path and suggests using `FINFOCUS_HOME` to specify an alternative location.
- What happens when a partial setup exists (e.g., directories exist but config is missing)? The command fills in the missing pieces without disturbing existing components.
- What happens when plugin download fails due to network issues? The command warns about the failure, continues with remaining steps, and suggests running `finfocus plugin install` later.
- What happens when the analyzer binary is not found (pre-#597 installation)? The command reports that the analyzer install feature is not available and suggests upgrading.
- What happens when `FINFOCUS_HOME` points to a read-only filesystem? The command fails with exit code 1 and an actionable error message explaining the permission issue.
- What happens when the user's home directory cannot be determined? The command falls back to the current working directory per existing config resolution behavior and warns the user.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `finfocus setup` top-level command that bootstraps the FinFocus environment in a single invocation.
- **FR-002**: System MUST display the FinFocus version and Go runtime version as the first output line.
- **FR-003**: System MUST check whether the `pulumi` CLI is available on PATH and report its version if found, or print a non-fatal warning if absent.
- **FR-004**: System MUST create the base directory, plugins directory, cache directory, and logs directory if they do not already exist.
- **FR-005**: System MUST write a default configuration file if one does not exist, preserving any existing configuration without modification.
- **FR-006**: System MUST invoke the analyzer installation logic (from issue #597) unless `--skip-analyzer` is specified.
- **FR-007**: System MUST install a default set of plugins unless `--skip-plugins` is specified.
- **FR-008**: System MUST support a `--non-interactive` flag that disables TTY-dependent output (status symbols, color) and explicitly signals CI/CD intent. Setup never prompts for user input regardless of this flag.
- **FR-009**: System MUST auto-detect non-interactive mode when stdin is not a TTY, applying the same TTY-independent output behavior as `--non-interactive`.
- **FR-010**: System MUST be idempotent — running setup multiple times produces the same result without errors or data loss.
- **FR-011**: System MUST respect the `FINFOCUS_HOME` environment variable for the base directory path, following the existing resolution precedence.
- **FR-012**: System MUST use command-level output functions (not global print) for all output to ensure testability.
- **FR-013**: System MUST print a status line for each step indicating success, skip, or warning.
- **FR-014**: System MUST print a completion summary with a next-steps hint upon successful completion.
- **FR-015**: System MUST exit with code 0 when all critical steps succeed (directory creation, config initialization), even if optional steps produce warnings.
- **FR-016**: System MUST exit with a non-zero code if any critical step fails (directory creation failure, config write failure).
- **FR-017**: Each setup step MUST be independent — failure in one step does not prevent subsequent steps from executing.
- **FR-018**: System MUST report failures with actionable remediation steps.

### Key Entities

- **Setup Step**: A discrete unit of work in the bootstrap sequence (version display, Pulumi detection, directory creation, config init, analyzer install, plugin install). Each step has a name, status (success/skipped/warning/error), and optional detail message.
- **Setup Result**: The aggregate outcome of all setup steps, including a list of completed steps, warnings, errors, and whether the overall setup succeeded.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new user can go from a fresh install to running their first cost estimate in under 60 seconds using `finfocus setup` followed by a cost command.
- **SC-002**: Running `finfocus setup` twice in succession produces zero errors, zero data loss, and preserves any user-customized configuration.
- **SC-003**: `finfocus setup --non-interactive` completes without any user interaction, suitable for automated pipelines.
- **SC-004**: Each setup step reports its status clearly, allowing users to identify and resolve issues without consulting documentation.
- **SC-005**: Unit test coverage for the setup command reaches 80% or higher.
- **SC-006**: Setup completes all local steps (excluding network-dependent plugin downloads) within 10 seconds.

## Assumptions

- The default plugin set is defined by the project maintainers and may change between releases. The initial set is expected to include the `aws-public` plugin at the latest available version.
- The analyzer installation function from issue #597 is available and returns a result struct with action, version, and path information.
- The existing config directory resolution function provides the correct base directory path respecting all environment variable overrides.
- Plugin installation requires network access; offline environments should use `--skip-plugins`.
- The command does not require root/administrator privileges — it operates within user-writable directories.
- Directory permissions follow existing project conventions: `0700` for config directory, `0750` for plugin directory.

## Dependencies

- **Issue #597**: `finfocus analyzer install/uninstall` — provides the analyzer installation function used in step 5 of the bootstrap sequence. This dependency is already merged.

## Scope Boundaries

### In Scope

- Creating directories, config, installing analyzer and plugins
- Version and Pulumi detection output
- Non-interactive mode and skip flags
- Idempotent behavior
- `FINFOCUS_HOME` support

### Out of Scope

- Plugin auto-update or version selection (users can use `finfocus plugin update` separately)
- Interactive wizard or guided questionnaire for config customization
- Shell completion installation (separate feature)
- Telemetry or usage reporting setup
- Authentication or credential management
