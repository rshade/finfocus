# Feature Specification: Split Project-Local and User-Global Configuration

**Feature Branch**: `591-config-split`
**Created**: 2026-02-14
**Status**: Draft
**Input**: GitHub Issue #548 - feat(config): split project-local and user-global .finfocus directories

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Per-Project Budget Configuration (Priority: P1)

A cloud engineer manages three Pulumi projects (web-app, data-pipeline, ML-infra) from the same
workstation. Each project has different cost profiles: the web-app budget is $5,000/month, the data
pipeline is $15,000/month, and the ML infrastructure is $50,000/month. Today, all three share a
single global budget configuration, making per-project cost tracking impossible.

With this feature, the engineer navigates into each project directory and runs FinFocus cost
commands. The tool automatically detects the project by walking up from the current working directory
to find `Pulumi.yaml`, then loads project-specific configuration from `$PROJECT/.finfocus/config.yaml`
merged with global defaults from `~/.finfocus/config.yaml`.

**Why this priority**: Per-project configuration is the core value proposition. Without it, users
working on multiple projects cannot meaningfully track budgets or customize settings per project.

**Independent Test**: Can be fully tested by creating two Pulumi project directories with different
`.finfocus/config.yaml` files and verifying that running FinFocus from each directory loads the
correct project-specific settings while inheriting global defaults.

**Acceptance Scenarios**:

1. **Given** a Pulumi project directory containing `.finfocus/config.yaml` with a $5,000 budget,
   **When** the user runs a cost command from within that project, **Then** the tool uses the
   project's $5,000 budget (not the global default).
2. **Given** a Pulumi project directory with `.finfocus/config.yaml` that only sets budget values,
   **When** the user runs a cost command, **Then** the tool inherits output format, logging, and
   other settings from the global `~/.finfocus/config.yaml`.
3. **Given** a directory that is NOT inside any Pulumi project, **When** the user runs a cost
   command, **Then** the tool falls back to `~/.finfocus/config.yaml` exactly as it does today
   (backward compatible).

---

### User Story 2 - Per-Project Recommendation Dismissals (Priority: P1)

A platform team member reviews cost optimization recommendations across multiple projects. They
dismiss a "rightsize this instance" recommendation in the web-app project because it has been
evaluated and rejected for performance reasons. Today, this dismissal is stored globally, so the
same recommendation is hidden when reviewing the data-pipeline project where it may still be
relevant.

With this feature, dismissals are stored in the project-local `.finfocus/dismissed.json` file. Each
project maintains its own dismissal state independently.

**Why this priority**: Dismissals leaking across projects is an active bug that causes users to miss
relevant recommendations. This is tied directly to the `DismissalStore` bug (hardcoded
`os.UserHomeDir()`) which also needs fixing.

**Independent Test**: Can be fully tested by dismissing a recommendation in Project A and verifying
it remains visible in Project B.

**Acceptance Scenarios**:

1. **Given** a user dismisses recommendation "rec-123" while in Project A, **When** they check
   recommendations in Project B, **Then** "rec-123" is still visible in Project B.
2. **Given** a user dismisses recommendation "rec-456" while in Project A, **When** they check
   recommendations in Project A again, **Then** "rec-456" remains dismissed.
3. **Given** a directory outside any Pulumi project, **When** the user dismisses a recommendation,
   **Then** the dismissal is stored in `~/.finfocus/dismissed.json` (backward compatible fallback).
4. **Given** the `FINFOCUS_HOME` environment variable is set, **When** the user dismisses a
   recommendation outside a Pulumi project, **Then** the dismissal store respects
   `FINFOCUS_HOME` instead of hardcoding `$HOME`.

---

### User Story 3 - Project Directory Discovery (Priority: P1)

A developer is working in a deeply nested subdirectory of their Pulumi project
(`project/src/services/api/`). They run a FinFocus cost command and expect the tool to automatically
find the project root by walking up the directory tree until it finds `Pulumi.yaml`, then load
configuration from `$PROJECT_ROOT/.finfocus/`.

**Why this priority**: Directory discovery is the foundation that enables Stories 1 and 2. Without
automatic project detection, users would need to manually specify the project directory every time.

**Independent Test**: Can be fully tested by creating a nested directory structure with
`Pulumi.yaml` at the root and running FinFocus from a deeply nested subdirectory.

**Acceptance Scenarios**:

1. **Given** a Pulumi project at `/home/user/projects/web-app/` with `Pulumi.yaml` at the root,
   **When** the user runs FinFocus from `/home/user/projects/web-app/src/handlers/`, **Then** the
   tool finds the project root and loads `.finfocus/` config from `/home/user/projects/web-app/`.
2. **Given** the `FINFOCUS_PROJECT_DIR` environment variable is set to `/custom/project/`,
   **When** the user runs FinFocus from any directory, **Then** the tool uses
   `/custom/project/.finfocus/` as the project config directory (overriding auto-detection).
3. **Given** no `Pulumi.yaml` exists anywhere in the directory hierarchy, **When** the user runs
   FinFocus, **Then** the tool falls back to the global `~/.finfocus/` directory with no error.
4. **Given** a deeply nested directory 20 levels deep, **When** the tool walks up looking for
   `Pulumi.yaml`, **Then** it reaches the filesystem root and stops without hanging or erroring.

---

### User Story 4 - Configuration Initialization with .gitignore (Priority: P2)

A developer sets up a new Pulumi project and runs `config init` to create project-local FinFocus
configuration. The tool creates the `.finfocus/` directory in the project root alongside
`Pulumi.yaml` and automatically generates a `.gitignore` inside it to prevent accidentally
committing user-specific data like dismissal state.

**Why this priority**: Important for clean project setup and preventing accidental commits of
user-specific data, but not required for the core read path to work.

**Independent Test**: Can be fully tested by running `config init` inside a Pulumi project directory
and verifying the created directory structure and `.gitignore` contents.

**Acceptance Scenarios**:

1. **Given** a Pulumi project directory without `.finfocus/`, **When** the user runs `config init`,
   **Then** a `.finfocus/` directory is created in the project root with a `.gitignore` and default
   `config.yaml`.
2. **Given** a `.finfocus/` directory already exists with a custom `.gitignore`, **When** the user
   runs `config init`, **Then** the existing `.gitignore` is preserved (not overwritten).
3. **Given** the project is inside a Git repository, **When** `.finfocus/` is created,
   **Then** a `.gitignore` file is auto-generated inside it.
4. **Given** the project is NOT inside a Git repository, **When** `.finfocus/` is created,
   **Then** the `.gitignore` is still created (defensive, for when Git is initialized later).

---

### User Story 5 - Global Resources Remain Shared (Priority: P2)

An operations team member installs plugins once in `~/.finfocus/plugins/` and uses them across all
projects. Cache, history databases, and logs are also stored globally. The config split must not
break this existing behavior -- only project-specific settings (budgets, dismissals, plugin
overrides) move to the project-local directory.

**Why this priority**: Preserving existing global resource behavior prevents breaking changes and
ensures plugins don't need to be installed per-project.

**Independent Test**: Can be fully tested by installing a plugin globally and verifying it is
discovered when running FinFocus from any project directory.

**Acceptance Scenarios**:

1. **Given** a plugin installed at `~/.finfocus/plugins/aws-public/0.5.0/`, **When** the user runs
   a cost command from any Pulumi project, **Then** the plugin is discovered and used regardless of
   project context.
2. **Given** cache data stored in `~/.finfocus/cache/`, **When** switching between projects,
   **Then** the cache directory remains the global one.
3. **Given** log output configured globally, **When** running from any project directory,
   **Then** logs write to the global `~/.finfocus/logs/finfocus.log`.

---

### Edge Cases

- What happens when `Pulumi.yaml` exists at multiple levels in the hierarchy (nested Pulumi
  projects)? The nearest (deepest) `Pulumi.yaml` wins, matching Pulumi CLI behavior.
- What happens when the project `.finfocus/` directory does not exist yet but `Pulumi.yaml` is
  found? The tool uses global config as fallback for reads; `config init` creates the directory for
  writes.
- What happens when the user has `FINFOCUS_HOME` set AND is inside a Pulumi project? Both apply:
  `FINFOCUS_HOME` resolves global resources (plugins, cache, logs), while the Pulumi project's
  `.finfocus/` resolves project-specific settings (config, dismissals).
- What happens when `config.yaml` exists in both global and project locations with conflicting keys?
  Project-local values override global values at the top-level key level (shallow merge).
- What happens on systems where `os.UserHomeDir()` fails (e.g., containers without HOME)? The tool
  falls back gracefully using the existing `ResolveConfigDir()` fallback chain, and the dismissal
  store uses the same fallback instead of returning an error.
- What happens when file permissions prevent reading the project `.finfocus/` directory? The tool
  logs a warning and falls back to global config.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST walk up from the current working directory to find `Pulumi.yaml` and use
  the containing directory as the project root for configuration discovery.
- **FR-002**: System MUST support a `FINFOCUS_PROJECT_DIR` environment variable that overrides
  automatic project directory detection.
- **FR-003**: System MUST merge project-local `config.yaml` over global `config.yaml` at the
  top-level key level (shallow merge: project keys replace entire global keys, missing project keys
  inherit from global).
- **FR-004**: System MUST store recommendation dismissals in the project-local `.finfocus/`
  directory when a Pulumi project is detected, falling back to the global directory otherwise.
- **FR-005**: System MUST resolve global resources (plugins, cache, history, logs) from the global
  `~/.finfocus/` directory regardless of project context.
- **FR-006**: System MUST auto-generate a `.gitignore` file inside the `.finfocus/` directory when
  it is created, containing exclusion rules for user-specific data.
- **FR-007**: System MUST NOT overwrite an existing `.gitignore` file if one already exists in the
  `.finfocus/` directory.
- **FR-008**: System MUST support a `--project-dir` CLI flag for explicit project directory override.
- **FR-009**: System MUST maintain full backward compatibility -- users with only `~/.finfocus/`
  and no Pulumi project context MUST see no behavior change.
- **FR-010**: The `config init` command MUST create a project-local `.finfocus/` directory with
  `.gitignore` and default `config.yaml` when run inside a Pulumi project.
- **FR-011**: The dismissal store MUST use the existing config directory fallback chain when
  no project directory is found, instead of hardcoding `os.UserHomeDir()`.
- **FR-012**: System MUST stop walking up the directory tree at the filesystem root to prevent
  infinite loops.

### Config Resolution Precedence

**Project-specific settings** (config.yaml, dismissed.json):

1. `--project-dir` CLI flag (explicit override)
2. `$FINFOCUS_PROJECT_DIR/.finfocus/` (environment variable override)
3. Walk up from CWD to find `Pulumi.yaml`, use `$PULUMI_PROJECT/.finfocus/`
4. Fall back to `~/.finfocus/` (backward compatible)

**Global resources** (plugins, cache, history, logs):

1. `$FINFOCUS_HOME` (explicit override)
2. `$PULUMI_HOME/finfocus`
3. `~/.finfocus/`

### Key Entities

- **Global Config Directory**: User-global directory (`~/.finfocus/`) storing shared resources --
  plugins, cache, history, logs, and default configuration.
- **Project Config Directory**: Project-local directory (`$PROJECT/.finfocus/`) storing
  project-specific configuration -- budgets, output preferences, plugin overrides, and
  recommendation dismissals.
- **Project Root**: The directory containing `Pulumi.yaml`, identified by walking up from CWD.
- **Merged Configuration**: The runtime configuration produced by overlaying project-local settings
  on top of global defaults using shallow (top-level key) merging.

## Assumptions

- The nearest (deepest) `Pulumi.yaml` in the directory hierarchy is the active project root,
  consistent with Pulumi CLI behavior.
- Shallow merge at the top-level key level is sufficient; deep recursive merging adds complexity
  with minimal benefit for the current config structure.
- The `.gitignore` should always be created (even outside Git repos) as a defensive measure for
  when Git is initialized later.
- The `config init` command is the primary write-path for creating project-local `.finfocus/`
  directories; read-path commands do not auto-create the directory.
- File locking for `dismissed.json` continues to use the existing advisory lockfile pattern.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users working on multiple Pulumi projects can maintain independent budget
  configurations, with each project loading its own settings within the time budget of the current
  config loading (no perceptible delay added).
- **SC-002**: Recommendation dismissals in one project do not affect recommendation visibility in
  other projects, verified by independent dismissal state per project directory.
- **SC-003**: Existing users with only `~/.finfocus/` and no Pulumi project context experience zero
  behavior change after the update (full backward compatibility).
- **SC-004**: Project directory discovery completes in under 100 milliseconds even for directory
  trees 50 levels deep.
- **SC-005**: New config resolution code achieves 80% or higher test coverage.
- **SC-006**: All existing tests continue to pass without modification (no regressions).
