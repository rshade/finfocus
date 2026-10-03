# Feature Specification: Plugin upgrade command and agent skill

**Feature Branch**: `617-plugin-upgrade`
**Created**: 2026-10-02
**Status**: Implemented (issue #270)
**Input**: GitHub issue #270 (research: plugin developer upgrade for
finfocus-spec SDK migrations). Resolved as both a CLI command and an agent
skill. Findings are in [research.md](research.md).

## Overview

`finfocus plugin init` creates a plugin project pinned to one finfocus-spec
version. Nothing helps an existing plugin move to a newer one. Authors read
finfocus-spec release notes, bump `go.mod`, and fix compile errors by hand.

Core does not catch drift either. Plugin hosts compare spec versions by major
number only, so every 0.x plugin counts as compatible with every 0.x core.

The work splits by who is good at it:

- **`finfocus plugin upgrade`** does the deterministic part. It detects the
  plugin's spec version, plans the hop path, and applies the edits that
  never need judgment: the `go.mod` require line and `go` directive, and the
  `SpecVersion` constant.
- **The `finfocus-plugin-upgrade` agent skill** does the part that needs
  judgment. It runs the command, then works through the manual steps of each
  hop with that hop's migration guide, and verifies the result.

A hop ends at a release that requires plugin-author action, not at every
minor line: v0.5.7 (a patch release) changed a handler signature, and v0.6.1
raised the minimum Go version. Each hop has one migration guide, the way a
database has one migration per schema change. The command and the skill read the same hop list, and a test
fails when a hop has no guide or the newest hop is behind core's own spec
version.

### Non-goals

- Changes to finfocus-spec, or new protocol definitions.
- Changes to `plugin init` templates (#248).
- Rewriting plugin pricing logic. Edits stop at what an API change requires.
- Network access. The target version comes from the core binary, not from
  GitHub.
- Running `go mod tidy`, builds, or tests from the command. Those need the
  network and the author's toolchain; the command prints them as next steps.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See what an upgrade involves (Priority: P1)

A plugin author runs `finfocus plugin upgrade --dry-run` in their plugin
repository. They see the current spec version, the target, each hop, and for
each hop the automatic edits and the manual steps, with a link to its guide.
Nothing on disk changes.

**Why this priority**: The plan is useful on its own, and both the apply step
and the skill depend on it.

**Independent Test**: Run the dry run against a fixture plugin on an old
version and compare the plan to the expected hops. Confirm no file changed.

**Acceptance Scenarios**:

1. **Given** a plugin requiring finfocus-spec v0.5.x, **When** the author runs
   the dry run, **Then** the plan lists every hop after the plugin's version,
   up to core's spec version.
2. **Given** a plugin already at the target, **When** the author runs the dry
   run, **Then** the command reports it is up to date and exits 0.
3. **Given** `--output json`, **When** the dry run runs, **Then** stdout is one
   JSON document carrying the detected version, target, and hops.

---

### User Story 2 - Apply the mechanical edits (Priority: P1)

The author runs `finfocus plugin upgrade` without `--dry-run`. The command
sets the `go.mod` require line to the target, raises the `go` directive when a hop requires it, and updates
the `SpecVersion` constant. It lists every file it changed and the
manual steps still open.

**Why this priority**: These edits are the tedious, error-prone part, and a
program does them reliably.

**Independent Test**: Apply against a copy of a fixture plugin and diff the
result against an expected tree.

**Acceptance Scenarios**:

1. **Given** a clean git working tree, **When** the author applies, **Then**
   only the `go.mod` require and `go` lines and the `SpecVersion` constant
   change.
2. **Given** uncommitted changes, **When** the author applies, **Then** the
   command refuses and names the dirty state, so the upgrade is always one
   reviewable diff. `--allow-dirty` overrides this.
3. **Given** a directory that is not a git repository, **When** the author
   applies, **Then** the command refuses unless `--allow-dirty` is set.

---

### User Story 3 - Let an AI assistant finish the upgrade (Priority: P2)

An author asks their coding assistant to upgrade the plugin. The skill runs
the dry run, shows the plan, and waits for approval. It then works on a
branch: runs the apply step, fixes the remaining compile errors guided by
each hop's guide, and verifies with build, tests, and a smoke run against
finfocus.

**Why this priority**: Breaking API changes need reading code, which a
program cannot do safely. The skill depends on Stories 1 and 2.

**Independent Test**: Follow the skill against a fixture plugin on an old
version; the result builds, its tests pass, and `finfocus plugin validate`
accepts it.

**Acceptance Scenarios**:

1. **Given** the skill is invoked, **When** it starts, **Then** it shows the
   plan and gets approval before editing anything.
2. **Given** a hop's guide lists a renamed SDK call, **When** the build fails
   on it, **Then** the skill applies the guide's replacement and leaves
   pricing logic alone.
3. **Given** verification fails after all hops, **When** the skill stops,
   **Then** it reports the failing step and leaves the branch for review.

### Edge Cases

- No `go.mod`, or `go.mod` does not require either spec module: the command
  says this is not a FinFocus plugin and exits non-zero.
- The plugin is newer than core's spec version: report it and change nothing.
  Downgrades are not supported.
- `--to` names a version above core's spec version, below the plugin's, or
  not known to be released (only hop versions and core's own are known
  without the network): reject it.
- A `replace` pinned to the old version stops applying after the bump: warn
  that it must be updated or removed.
- The plugin is below v0.5.0, the first finfocus-spec release: report that
  it is too old to upgrade in place and point to `plugin init`. A `go.mod`
  without a finfocus-spec requirement is not a plugin.
- A `replace` directive points the spec module at a local path: report it;
  the require line is still updated, the replace is left alone.
- No `SpecVersion` constant is found: skip that edit and say so. Plugins are
  not required to declare one.
- The `SpecVersion` constant disagrees with `go.mod`: `go.mod` wins for
  planning; the mismatch is reported.
- A pseudo-version in `go.mod`: plan from its base version.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The command MUST detect the spec module path and version from
  the plugin's `go.mod` require line.
- **FR-002**: The command MUST detect a top-level `SpecVersion` version
  literal in non-test Go files that import finfocus-spec. Other
  `SpecVersion` declarations are not the plugin's spec version.
- **FR-003**: The default target MUST be the spec version the running core
  binary was built against. `--to` MAY select an earlier known version.
- **FR-004**: The plan MUST list, in order, every hop whose end version is
  above the plugin's version and at or below the target, each with its
  automatic edits, manual steps, and guide name.
- **FR-005**: The global `--dry-run` MUST print the plan and change nothing.
- **FR-006**: Apply MUST refuse a dirty or non-git tree unless
  `--allow-dirty` is set.
- **FR-007**: Apply MUST set the `go.mod` require version and the
  `SpecVersion` constant to the target, always with the `v` prefix that
  `GetPluginInfo` validation requires, and MUST raise the `go` directive to the highest minimum
  Go version required by a planned hop when it is lower.
- **FR-008**: Apply MUST NOT edit any other Go source content. `go.mod` is
  re-rendered by Go's own formatter, as `go mod edit` does.
- **FR-009**: Output MUST support table and JSON through the standard
  output resolver, and report changed files and open manual steps.
- **FR-010**: The command MUST NOT use the network.
- **FR-011**: The command MUST be excluded from the MCP tool list: it
  rewrites a local source tree, the same reason `plugin init` is excluded.
- **FR-012**: Every hop MUST have a migration guide in the skill's
  `references/` directory. A test MUST fail if one is missing, or if the
  newest hop ends below core's spec version.
- **FR-013**: The skill MUST show the plan and obtain approval before any
  edit, work on a branch, and finish with build, test, and validate steps.

### Key Entities

- **Hop**: the step up to one release that requires author action (`to`),
  with an optional minimum Go version, automatic edits, manual steps, and a
  guide name.
- **Plan**: detected module path and version, target, the ordered hops, and
  any warnings.
- **Migration guide**: a Markdown reference for one hop, read by people and
  by the skill.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A fixture plugin on an old line reaches the target with the
  mechanical edits applied by one command and no manual `go.mod` editing.
- **SC-002**: The dry run changes zero files.
- **SC-003**: Bumping finfocus-spec in core without adding a hop and guide
  fails the test suite.
- **SC-004**: Following the skill, a fixture plugin builds, passes its tests,
  and passes `finfocus plugin validate` at the target version.

## Assumptions

- Plugins are single Go modules with `go.mod` at the directory given.
- finfocus-spec is pre-1.0, so any release may break plugins. Releases
  between hops need no author action; research.md records which releases
  are hops and why.
- Tightening core's major-only compatibility check is recorded in
  research.md and left to a follow-up issue.
