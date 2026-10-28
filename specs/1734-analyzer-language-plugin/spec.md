# Feature Specification: Analyzer Language Plugin

**Feature Branch**: `1734-analyzer-language-plugin`
**Created**: 2026-10-22
**Status**: Draft
**Input**: Issue #1895. FinFocus analyzer (policy pack) mode fails on Pulumi
3.270.0 and later with `no language plugin 'pulumi-language-finfocus' found`.
Pulumi 3.270.0 (pulumi/pulumi#24784) removed the fallback that ran
`pulumi-analyzer-policy-<runtime>` directly; every policy pack runtime now
needs a `pulumi-language-<runtime>` plugin. A spike on 2026-10-22 confirmed
that adding a FinFocus language plugin restores analyzer mode on 3.271.0 and
keeps 3.268.0 working.

## Clarifications

### Session 2026-10-22

- Q: Where should the analyzer resolve its project from when Pulumi starts
  it through the language plugin? → A: The Pulumi project directory, which
  Pulumi passes as `RunPluginRequest.pwd`, so policy-pack mode reads
  project-tier config and writes summaries and history where Pulumi before
  3.270.0 did (FR-009, FR-014). First answered as "the policy pack
  directory, as today" on 2026-10-22; changed on 2026-10-28 after a demo
  preview showed that answer drops the project's `.finfocus/` config.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Get cost diagnostics in `pulumi preview` on current Pulumi (Priority: P1)

A person who follows the analyzer setup guide runs
`pulumi preview --policy-pack ~/.finfocus/analyzer` with a current Pulumi
release. Today the preview stops with an error that names a plugin they have
never heard of. After this change the preview runs and shows FinFocus cost
diagnostics, as it does on Pulumi 3.268.0.

**Why this priority**: Analyzer mode is unusable for everyone on Pulumi
3.270.0 or later, which includes GitHub's hosted runners. It is the whole
reason for the fix.

**Independent Test**: Run the documented setup against a project with no
resources on Pulumi 3.271.0 and check that the preview succeeds and that the
FinFocus analyzer served it.

**Acceptance Scenarios**:

1. **Given** Pulumi 3.270.0 or later and a policy pack set up by
   `finfocus analyzer install`, **When** the person runs
   `pulumi preview --policy-pack <pack dir>`, **Then** the preview succeeds
   and the FinFocus analyzer runs inside it.
2. **Given** the same setup on Pulumi 3.268.0, **When** the person runs the
   same preview, **Then** it succeeds as it does today.
3. **Given** a project with resources FinFocus can price, **When** the
   preview runs on either version, **Then** the cost diagnostics and stack
   summary are the same as they are on Pulumi 3.268.0 today.
4. **Given** stack configuration set on the Pulumi stack, **When** the
   analyzer runs through the new path, **Then** it receives the same
   project, stack, and configuration it received before.

---

### User Story 2 - Fix an existing installation with one command (Priority: P1)

A person who set up analyzer mode with an older FinFocus release upgrades
FinFocus and Pulumi. Their policy pack directory has only the old entry.
They re-run `finfocus analyzer install`, and the pack works on every
supported Pulumi version. The command prints the pack location and the
preview command to run.

**Why this priority**: Every existing user has the old layout. Without a
one-command upgrade the fix does not reach them.

**Independent Test**: Create a policy pack directory with the old layout
only, run `finfocus analyzer install`, and run the preview on Pulumi 3.271.0.

**Acceptance Scenarios**:

1. **Given** a policy pack directory created by an older FinFocus, **When**
   the person runs `finfocus analyzer install` (with or without `--force`),
   **Then** the directory gains the language plugin entry and keeps the
   legacy entry.
2. **Given** the analyzer is already installed at the current version,
   **When** the person re-runs `finfocus analyzer install`, **Then** the
   policy pack is still brought up to the new layout.
3. **Given** a completed install, **When** the person reads the output,
   **Then** it names the policy pack directory, says that directory must be on
   `PATH`, and prints the exact `pulumi preview --policy-pack` command.
4. **Given** Windows, where the pack holds copies of the binary rather than
   links, **When** the person runs the install, **Then** both entries are
   current copies and the preview works.

---

### User Story 3 - Hear about the next Pulumi break before users do (Priority: P2)

A maintainer wants a Pulumi release that changes how policy packs start to
turn a FinFocus check red before a user or the demo repository reports it.

**Why this priority**: This break reached users through the demo CI, after
the hosted runner image moved to a new Pulumi. Earlier warning is valuable,
but it does not fix analyzer mode by itself.

**Independent Test**: Run the automated check against the newest Pulumi
release with the fix reverted and see it fail; restore the fix and see it
pass.

**Acceptance Scenarios**:

1. **Given** the automated policy pack check, **When** it runs against
   Pulumi 3.268.0 and against the newest Pulumi release, **Then** both runs
   pass with this feature in place.
2. **Given** the current `main` branch without this feature, **When** the
   check runs against Pulumi 3.271.0, **Then** it fails with the missing
   language plugin error (break check).
3. **Given** a policy pack with the language plugin entry removed, **When**
   the check runs against Pulumi 3.268.0, **Then** it still passes, which
   keeps the legacy entry covered.

---

### User Story 4 - Follow docs that match what the tools do (Priority: P2)

A person setting up analyzer mode for the first time, or in CI, reads the
FinFocus docs and gets a setup that works on current Pulumi.

**Why this priority**: Every analyzer doc describes the old naming. Wrong
docs send new users into the same error.

**Independent Test**: Follow the getting-started guide from a clean machine
on Pulumi 3.271.0 and reach a working preview without steps outside the
guide.

**Acceptance Scenarios**:

1. **Given** the analyzer setup guide, **When** a person follows it on the
   newest Pulumi release, **Then** the preview works.
2. **Given** a person who sees `no language plugin 'pulumi-language-finfocus'
   found`, **When** they search the docs or the FAQ for that message,
   **Then** they find the cause and the fix (re-run
   `finfocus analyzer install`).

---

### Edge Cases

- The policy pack directory is not on `PATH`: the preview fails with Pulumi's
  missing-plugin error, and the docs and install output already tell the
  person to add it. FinFocus cannot detect this, because it never starts.
- Automatic plugin download is enabled (Pulumi's default): Pulumi tries to
  download a `finfocus` plugin from a repository that does not exist and
  prints a 404 next to the missing-plugin error. With the language plugin
  present, Pulumi never tries.
- The language plugin entry is a link to the FinFocus binary: starting the
  analyzer through it must run the analyzer, not the general CLI. This must
  hold for links (Linux, macOS) and copies (Windows).
- Pulumi cancels or ends the preview: the analyzer stops, and on Linux and
  macOS any cost plugin processes it started are stopped too. On Windows the
  analyzer is stopped forcibly, as Pulumi itself does there, so a cost plugin
  it started can outlive it.
- The analyzer exits with an error during the preview: Pulumi reports the
  failure and its exit code instead of hanging.
- Pulumi 3.268.0 prefers the language plugin whenever one is present (seen
  in the spike), so on that release the legacy entry runs only when the
  language plugin entry is missing. Releases before 3.268.0 are not verified
  and are expected to behave the same; see Assumptions.
- A Pulumi project not written in YAML (TypeScript, Python, Go): the policy
  pack runtime is independent of the project language, so behavior is the
  same.
- The analyzer installed as a direct analyzer plugin under the Pulumi plugin
  directory (`analyzer-finfocus-v<version>`): Pulumi did not change that path,
  and this feature leaves it alone.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The FinFocus release MUST let Pulumi 3.268.0 and every later
  Pulumi release start the FinFocus analyzer from the policy pack directory
  that `finfocus analyzer install` creates.
- **FR-002**: The language plugin MUST be a mode of the existing FinFocus
  binary, selected by the name it is started under, like the two analyzer
  names it already answers to. The release MUST NOT add a second artifact.
- **FR-003**: Starting the analyzer through the language plugin MUST run the
  same analyzer as today, with the same diagnostics, stack summary, and
  stack configuration, and MUST print nothing but the plugin handshake on
  standard output.
- **FR-004**: `finfocus analyzer install` MUST put a `pulumi-language-finfocus`
  entry in the policy pack directory beside the existing
  `pulumi-analyzer-policy-finfocus` entry, on every run, including runs where
  the analyzer is already installed. On Windows each entry MUST be a current
  copy with the platform's executable suffix.
- **FR-005**: `finfocus analyzer install` MUST print the policy pack directory,
  the language plugin entry, the instruction to put the directory on `PATH`,
  and the exact `pulumi preview --policy-pack` command. Its JSON output MUST
  carry the policy pack directory and the language plugin entry's path.
- **FR-006**: The legacy `pulumi-analyzer-policy-finfocus` entry MUST remain
  in the policy pack directory and MUST keep working on Pulumi releases that
  still support it.
- **FR-007**: When Pulumi stops the analyzer on Linux or macOS, the analyzer
  MUST get the chance to shut down cleanly, so that the cost plugin processes
  it started do not outlive the preview. The language plugin MUST finish its
  own shutdown inside Pulumi's grace period. On Windows, which has no
  graceful signal for a child process, the analyzer MUST still stop; its cost
  plugin processes are not guaranteed to stop with it (see Assumptions).
- **FR-008**: When the analyzer exits with an error, Pulumi MUST receive that
  exit status.
- **FR-009**: The analyzer MUST resolve its project directory (project-tier
  config, summary location, cost history) from the Pulumi project the
  preview runs in, on every Pulumi version. Before 3.270.0 Pulumi starts the
  analyzer in that project; from 3.270.0 the language host passes it as
  `RunPluginRequest.pwd` (FR-014). Summaries and history MUST land under that
  project, where Pulumi before 3.270.0 already wrote them.
- **FR-010**: An automated check MUST run `pulumi preview --policy-pack`
  against a project with no resources on Pulumi 3.268.0 and on the newest
  Pulumi release, and MUST fail when the FinFocus analyzer does not serve the
  preview. It MUST also cover the legacy entry with the language plugin entry
  removed on Pulumi 3.268.0.
- **FR-011**: The check MUST run on a schedule against the newest Pulumi
  release, so that a new break shows up without a code change in FinFocus.
- **FR-012**: The analyzer docs (getting started, integration, architecture,
  deployment, FAQ) MUST describe the new policy pack layout, and the FAQ MUST
  quote the missing-language-plugin error with its fix.
- **FR-013**: `rshade/finfocus-action` MUST write both policy pack entries
  in its analyzer setup, which writes only the old layout. The change is made
  in that repository; the draft in `issue-drafts.md` is its PR text if a
  tracking issue is wanted.
- **FR-014**: The language host MUST give the analyzer `RunPluginRequest.pwd`
  as `FINFOCUS_PROJECT_DIR`, the variable the CLI already reads, so the
  analyzer resolves the Pulumi project instead of the policy pack directory.
  An explicit `FINFOCUS_PROJECT_DIR` (in the host environment or in
  `RunPluginRequest.env`) MUST win, and a `pwd` that is empty, relative,
  missing, or not a directory MUST be ignored. The existing
  `cost.history.directory` safety check applies unchanged.

### Key Entities

- **Policy pack directory**: The directory passed to `--policy-pack`
  (`$FINFOCUS_HOME/analyzer`, default `~/.finfocus/analyzer`). It holds the
  policy pack description, the legacy analyzer entry, and the new language
  plugin entry.
- **Language plugin entry** (`pulumi-language-finfocus`): What current Pulumi
  releases look for to start a `finfocus` policy pack. A link to, or copy of,
  the FinFocus binary.
- **Legacy analyzer entry** (`pulumi-analyzer-policy-finfocus`): What Pulumi
  3.268.0 and earlier start when no language plugin is present.
- **Language host**: The FinFocus process Pulumi starts through the language
  plugin entry. It does not analyze anything itself; it starts the analyzer
  for Pulumi and relays its output. The plan, contracts, and tasks use this
  name.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable - analyzer mode runs inside
  `pulumi preview`/`pulumi up` as a Pulumi plugin and produces Pulumi
  diagnostics, not a FinFocus result. The constitution names
  `analyzer serve` as an example of a feature with no web surface. This
  change only restores how Pulumi starts it.
- **Web data**: None.
- **Layout fit**: None.
- **Additions**: none.
- **Render evidence**: Not applicable (no web surface).
- **Follow-up issue scope**: None; exempt under Principle VII.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A person who follows the setup guide gets a working analyzer
  preview on Pulumi 3.268.0 and on the newest Pulumi release, with no steps
  beyond the guide. Linux and macOS are verified automatically on both
  versions; Windows is verified by native unit tests and one recorded manual
  preview on the newest release.
- **SC-002**: An existing user restores analyzer mode with one command
  (`finfocus analyzer install`) and no manual file changes.
- **SC-003**: The `finfocus-demo` analyzer workflow passes on the newest
  Pulumi release once the interim version pin is removed. Checked after the
  FinFocus release, in `finfocus-demo`; it is not a completion gate for this
  feature.
- **SC-004**: A future Pulumi release that breaks policy pack startup turns
  the scheduled FinFocus check red within one scheduled run of that release,
  before any user report.
- **SC-005**: On Linux and macOS, after a cancelled or completed preview, no
  FinFocus analyzer or cost plugin process is left running. On Windows, no
  analyzer or language plugin process is left running.
- **SC-006**: The cost diagnostics a priced project shows are the same on
  Pulumi 3.268.0 before and after this change.

## Assumptions

- Pulumi will not bring back the `pulumi-analyzer-policy-<runtime>` fallback.
  The upstream PR calls the change intended and keeps it only in a public SDK
  copy that the CLI no longer calls.
- Users already put the policy pack directory on `PATH`, as the current docs
  require; Pulumi searches `PATH` for language plugins, so the setup steps do
  not change.
- The analyzer gets its project, stack, and configuration from the Pulumi
  stack configuration call, not from the environment variables Pulumi stopped
  setting. The spike confirmed that analyzer mode works without them.
- With automatic plugin download disabled, Pulumi behaves the same once the
  language plugin entry is present; the entry removes any need to download.
  Both settings are tested.
- On Windows a child process cannot be asked to shut down gracefully, so
  stopping the analyzer there skips its plugin cleanup. Pulumi's own
  forcible stop has the same effect on Windows today. Making Windows cleanup
  graceful (for example through a job object) is not part of this feature.
- Pulumi releases before 3.268.0 that FinFocus supported keep working: the
  legacy entry stays in place for them (FR-006). Only 3.268.0 and the newest
  release are tested.
- The newest Pulumi release is whatever Pulumi's own installer resolves as
  latest on the day the scheduled check runs.
- The interim mitigation, pinning Pulumi 3.268.0 in the `finfocus-demo`
  analyzer workflow, is done in that repository and is not part of this
  feature.

## Dependencies

- Pulumi 3.270.0 behavior (pulumi/pulumi#24784) and the
  `pulumi-policy-opa` v1.1.2 language plugin as the reference design.
- `rshade/finfocus-action` must adopt the new layout through its own issue
  (FR-013) for its users to benefit.
- `rshade/finfocus-demo` removes its Pulumi pin once a FinFocus release
  ships this feature (SC-003).

## Out of Scope

- The direct analyzer plugin install under the Pulumi plugin directory
  (`analyzer-finfocus-v<version>`); Pulumi did not change that path.
- Publishing a `finfocus` plugin that Pulumi can download automatically
  (`pulumi plugin install language finfocus`).
- Changes to `finfocus-action` and `finfocus-demo` beyond the follow-up
  issue; each repository tracks its own change.
- Any change to what the analyzer computes or reports.
- Graceful cost plugin cleanup on Windows (FR-007, Assumptions).
