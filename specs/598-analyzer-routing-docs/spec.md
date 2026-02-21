# Feature Specification: Document Routing Limits in Analyzer Mode

**Feature Branch**: `598-analyzer-routing-docs`
**Created**: 2026-02-20
**Status**: Draft
**Input**: User description: "Docs: Document that routing config does not apply in analyzer/policy-pack mode"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer Confused Why Extra Plugin Fires (Priority: P1)

A developer configures routing rules expecting only `aws-public` to respond during
`pulumi preview --policy-pack`. They are surprised when their `recorder` debug plugin
also fires for every resource, creating unwanted side effects (e.g., recorded request
files piling up in CI). They turn to the routing guide for help and find no explanation
for this behavior.

**Why this priority**: This is the primary confusion the issue describes. Users invest
time configuring routing, run `pulumi preview --policy-pack`, and the behavior
contradicts what the routing guide implies. Documenting this prevents support
escalations and wasted debugging time.

**Independent Test**: Can be validated by reading `docs/guides/routing.md` alone and
confirming it contains a clear callout that routing does not apply in analyzer/policy-pack
mode.

**Acceptance Scenarios**:

1. **Given** a developer reads `docs/guides/routing.md` after routing config fails to
   exclude a plugin in analyzer mode, **When** they scroll to the relevant section,
   **Then** they find a clearly labelled callout stating that routing configuration is
   not consulted during `pulumi preview --policy-pack`.
2. **Given** a developer reads the routing callout, **When** they look for how to
   exclude plugins in analyzer mode, **Then** they are directed to the
   `analyzer-integration.md` documentation for the `FINFOCUS_HOME` isolation procedure.

---

### User Story 2 - Operator Needs to Exclude a Plugin in CI Policy-Pack Run (Priority: P2)

A platform engineer sets up a CI pipeline using `pulumi preview --policy-pack`. They
want only the cost-data plugin (e.g., `aws-public`) to run and need to exclude the
`recorder` debug plugin to avoid unnecessary file writes. They need a tested, repeatable
procedure for this.

**Why this priority**: Without actionable isolation instructions, users have no recourse
once they discover routing does not work. The `FINFOCUS_HOME` mechanism is the only path
but is undiscovered and undocumented.

**Independent Test**: Can be validated by reading `docs/analyzer-integration.md` and
confirming it contains a complete, step-by-step "Isolating plugins in analyzer mode"
section with a `FINFOCUS_HOME` example.

**Acceptance Scenarios**:

1. **Given** an operator reads `docs/analyzer-integration.md`, **When** they navigate to
   the "Isolating plugins in analyzer mode" section, **Then** they find step-by-step
   instructions for creating an isolated plugin directory using `FINFOCUS_HOME`.
2. **Given** the operator follows the documented steps using real directory creation and
   file-level symlinks, **When** they run `pulumi preview --policy-pack` with
   `FINFOCUS_HOME` set, **Then** only the plugins present in the isolated directory are
   loaded.
3. **Given** the operator reads the symlink instructions, **When** they attempt the
   directory-level symlink shortcut, **Then** the documentation warns them it does not
   work (due to issue #750) and shows the correct file-level symlink approach.

---

### User Story 3 - Developer Wants to Understand Why Global Plugins Cannot Be Excluded (Priority: P3)

A developer has a plugin with `supported_providers: ["*"]` and expects that omitting it
from the `routing:` config will exclude it. After testing, they find it still fires.
They want documentation explaining why global plugins are always included.

**Why this priority**: This is a secondary discovery path. Users who know about routing
may still be confused by the global plugin behavior. The callout in `routing.md` should
cover this.

**Independent Test**: Can be validated by reading the routing callout and confirming
it mentions that global plugins (`supported_providers: ["*"]`) cannot be excluded via
routing in any mode.

**Acceptance Scenarios**:

1. **Given** a developer reads the analyzer-mode callout in `routing.md`, **When** they
   look for whether routing can exclude a global plugin, **Then** they find an explicit
   statement that global plugins are always included for all resources.

---

### Edge Cases

- What if issue #750 is fixed and directory-level symlinks start working? The docs must
  link to #750 so users can check whether the workaround is still needed.
- What if `FINFOCUS_HOME` contains no plugins at all? The example should clarify that
  finfocus will load no plugins (zero cost data) rather than falling back to `~/.finfocus`.
- What if the user sets `FINFOCUS_HOME` and also has `PULUMI_HOME`? The precedence order
  (`FINFOCUS_HOME` > `PULUMI_HOME/finfocus` > `~/.finfocus`) should be referenced.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `docs/guides/routing.md` MUST contain a clearly-labelled callout, placed
  after the "Common Configuration Patterns" section and before the "Validation" section,
  stating that routing configuration is NOT applied during `pulumi preview --policy-pack`
  (analyzer/policy-pack mode).
- **FR-002**: The callout in `routing.md` MUST state that global plugins
  (`supported_providers: ["*"]`) cannot be excluded via routing in any mode.
- **FR-003**: The callout in `routing.md` MUST direct users to `analyzer-integration.md`
  for the plugin isolation procedure.
- **FR-004**: `docs/analyzer-integration.md` MUST contain a new "Isolating plugins in
  analyzer mode" section with a step-by-step procedure using `FINFOCUS_HOME`.
- **FR-005**: The isolation procedure MUST document the file-level symlink requirement
  and explicitly warn that directory-level symlinks are broken until issue #750 is fixed,
  with a link to the issue.
- **FR-006**: The isolation procedure MUST include an example `FINFOCUS_HOME` directory
  setup showing how to expose only selected plugins.
- **FR-007**: The isolation procedure SHOULD include an inline `bash` code block
  demonstrating how to run `pulumi preview --policy-pack` with `FINFOCUS_HOME` set to
  the isolated directory. No new script files are to be created.
- **FR-008**: Both documents MUST pass `make docs-lint` (markdownlint) without errors
  after the changes are applied.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer encountering unexpected plugin behavior in analyzer mode can
  discover the documented explanation within one page read of `routing.md`.
- **SC-002**: An operator following the isolation procedure step-by-step is able to
  run `pulumi preview --policy-pack` with only their chosen plugins loaded; the procedure
  is complete with no missing steps.
- **SC-003**: Documentation linting (`make docs-lint`) reports zero errors or warnings
  on both modified files after changes are applied.
- **SC-004**: No new documentation pages are created; all changes are additive to the
  two existing files identified in the issue.

## Clarifications

### Session 2026-02-20

- Q: Where in `docs/guides/routing.md` should the analyzer-mode callout be placed? → A: After the "Common Configuration Patterns" section, before "Validation".
- Q: Should FR-007's shell command example be an inline code block or a new script file? → A: Inline `bash` code block inside `analyzer-integration.md`; no new files.

## Assumptions

- The `FINFOCUS_HOME` isolation mechanism is already tested and confirmed working as of
  finfocus v0.3.1 (per issue facts).
- The `analyzer-integration.md` file exists at `docs/analyzer-integration.md` and
  contains existing sections into which the new section will be appended.
- Issue #750 remains open; documentation references it as a known limitation and links
  to it.
- The project's markdownlint configuration permits `> **Note:**` blockquote style
  callouts used elsewhere in the docs.
- The docs directory uses Jekyll with GitHub Pages; standard markdown blockquotes and
  fenced code blocks are acceptable formatting.
- The `## Changelog` section in `docs/guides/routing.md` will be updated with a new
  version entry to document the analyzer-mode callout addition. This is consistent with
  Constitution Principle IV (Documentation Integrity) and the existing changelog
  convention in that file.
