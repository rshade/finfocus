# Feature Specification: Plugin development skill installed by init and upgrade

**Feature Branch**: `620-plugin-dev-skill`
**Created**: 2026-10-03
**Status**: Implemented
**Input**: "Add a skill that helps create a plugin that is installed with
`plugin init` and `plugin upgrade`", possibly through `npx skills add`.
Design agreed in a brainstorming session on 2026-10-03.

## Overview

`finfocus plugin init` scaffolds a plugin repository, and `finfocus plugin
upgrade` moves it to a newer finfocus-spec version. Neither leaves anything
behind that tells an AI coding assistant how a FinFocus plugin works: the
`Supports` contract, when `$0` is a price and when it means "unpriced", how
core sends resource types, tags and regions, or how to test against a real
plan.

The only plugin-authoring skill, `.claude/skills/finfocus-plugin/`, is written
for core developers (it walks through `plugins/recorder/`), is nested one
directory too deep to load, and never reaches plugin repositories.

This feature adds a `finfocus-plugin-dev` agent skill and has both commands
install it, together with the existing `finfocus-plugin-upgrade` skill, into
the plugin repository through the `skills` CLI (`npx skills add`). The skill
version is pinned to the finfocus release that runs the command, so the
guidance matches the SDK version that release scaffolds or upgrades to.

### Non-goals

- Embedding skill files in the finfocus binary. The `skills` CLI is the
  installer, and its `skills-lock.json` records the source.
- Making the install a requirement. A plugin project without the skill still
  builds and upgrades.
- Changing the code edits `plugin upgrade` makes, or its clean-tree rule for
  those edits.
- Installing skills for every agent the `skills` CLI knows. Only Claude Code
  (`.claude/skills/`) and the shared `.agents/skills/` directory are written.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A new plugin ships with the skill (Priority: P1)

A plugin author runs `finfocus plugin init`. After the project files are
generated, finfocus runs `npx skills add` in the project directory and the
repository contains `.agents/skills/finfocus-plugin-dev/`,
`.claude/skills/finfocus-plugin-dev/`, the same two copies of
`finfocus-plugin-upgrade`, and `skills-lock.json`.

**Independent Test**: run `plugin init` with a fake installer and check that
it was called once with the project directory and this build's version, and
that the output names the installed skills.

**Acceptance Scenarios**:

1. **Given** Node is installed and GitHub is reachable, **When** the author
   runs `plugin init`, **Then** the skills are installed and the output lists
   the installed paths.
2. **Given** a released finfocus `v0.4.1`, **When** the skill is installed,
   **Then** the source is `https://github.com/rshade/finfocus/tree/v0.4.1/agent-skills`
   and `skills-lock.json` records `"ref": "v0.4.1"`.

### User Story 2 - Upgrading refreshes the skill (Priority: P1)

A plugin author runs `finfocus plugin upgrade`. After the go.mod and
`SpecVersion` edits, finfocus re-runs `npx skills add` with this build's
tag, which replaces the installed copies. The skill files appear in
`changed_files` and in the same reviewable diff as the code edits.

**Acceptance Scenarios**:

1. **Given** an outdated plugin and a clean tree, **When** the author applies
   the upgrade, **Then** the code edits are made, the skill is reinstalled, and
   the JSON output has a `skill` object with `installed: true`.
2. **Given** an up-to-date plugin, **When** the author runs `plugin upgrade`,
   **Then** no code is edited and the skill is still reinstalled, so plugins
   created before this feature can pick it up.
3. **Given** `--dry-run`, **When** the author runs `plugin upgrade`, **Then**
   nothing runs and the `skill` object carries the command that would run.

### User Story 3 - Install problems never block the author (Priority: P1)

**Acceptance Scenarios**:

1. **Given** `npx` is not on PATH, **When** either command runs, **Then** it
   succeeds, prints a warning, and prints the exact command to run later.
2. **Given** `npx` exits non-zero or exceeds the timeout, **Then** the
   command succeeds and the warning includes the tail of the `skills` output.
3. **Given** `--no-skill`, or `plugin init --offline`, **Then** `npx` is not
   run and the command to run later is printed.

### Edge Cases

- A development build (`git describe` output such as `v0.4.0-3-gabc-dirty`,
  the unstamped default `0.1.0`, or any version at or below `v0.4.0`, the last
  release without the skill) installs from `main` and warns that the skill may
  not match the binary.
- goreleaser stamps versions without the `v` prefix (`0.4.1`); it is added.
- `plugin upgrade --output json` keeps stdout valid JSON: the `skills` CLI
  output is captured, never streamed.
- `plugin init --docker-only` adds Docker files to an existing project and does
  not install the skill.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `agent-skills/finfocus-plugin-dev/` MUST provide a `SKILL.md`,
  `references/`, and a packaged `.skill`, written for a repository created
  by `plugin init`, and verified against finfocus-spec as used by core.
- **FR-002**: `plugin init` MUST install `finfocus-plugin-dev` and
  `finfocus-plugin-upgrade` after generating the project, unless
  `--no-skill`, `--offline`, `--dry-run`, or `--docker-only` is set.
- **FR-003**: `plugin upgrade` MUST install the same skills after a non-dry
  run, whether or not code was edited, unless `--no-skill` is set.
- **FR-004**: The install command MUST be
  `npx -y skills@<pinned> add <source> --skill finfocus-plugin-dev --skill finfocus-plugin-upgrade --agent codex --agent claude-code --copy -y`,
  with the `skills` package version pinned in code.
- **FR-005**: The source MUST be the finfocus tag of a release build newer than
  `v0.4.0`, and `main` otherwise.
- **FR-006**: The install MUST run with no stdin, a timeout, captured output,
  and `DO_NOT_TRACK=1` and `DISABLE_TELEMETRY=1` in its environment.
- **FR-007**: Any install failure MUST be reported as a warning with the
  command to run later, and MUST NOT change the command's exit code.
- **FR-008**: `plugin upgrade` JSON output MUST include a `skill` object
  (`installed`, `source`, `command`, `paths`, `warning`); installed paths MUST
  be added to `changed_files`.
- **FR-009**: `.claude/skills/finfocus-plugin/` and its `.skill` MUST be
  removed in favor of the new skill.

- **FR-010**: The `plugin init` scaffold MUST be correct where the skill
  describes it: `SpecVersion = pluginsdk.SpecVersion`; the EC2 example matches
  `aws:ec2/instance:Instance`, reads `Sku` before the `instanceType` tag, and
  returns not-supported for a missing or unknown instance type instead of a
  default price; `make install` writes a flat JSON `plugin.manifest.json` whose
  name and version match the install directory. An integration test builds,
  tests, installs, and validates a generated project.

### Key Entities

- **Skill install result**: whether the skills were installed, the source URL,
  the command line, the installed paths relative to the project, and a warning.

## Success Criteria *(mandatory)*

- **SC-001**: A plugin scaffolded by a release build contains both skills for
  Claude Code and for agents that read `.agents/skills/`, with no manual step.
- **SC-002**: No `plugin init` or `plugin upgrade` run fails because of the
  skill install.
- **SC-003**: Unit and integration tests never run `npx` or use the network.

## Assumptions

- The `skills` CLI (`vercel-labs/skills`, npm package `skills`) 1.7.0 accepts
  a GitHub tree URL with a tag, `--agent`, `--copy`, and `-y`, and records the
  ref in `skills-lock.json`. Checked by hand on 2026-10-03.
- The first release that contains this skill is newer than `v0.4.0`.
