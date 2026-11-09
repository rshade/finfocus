# Feature Specification: Per-Stack-Safe Web Data Path

**Feature Branch**: `2013-per-stack-web-data`
**Created**: 2026-11-08
**Status**: Draft
**Input**: GitHub issue rshade/finfocus#2038 — "web: make cache/history stores,
ESC and passphrase per-stack-safe for multi-stack". Step 3 of 4 (#2036, #2037,
\#2038, then #2039). This step is a prerequisite refactor: it adds no
user-visible feature, and single-stack behavior must not change. The
`--stack a,b,c` flag, tab bar, and lazy per-stack loading belong to #2039.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Several stacks share one live cache and history (Priority: P1)

A developer building the multi-stack dashboard (#2039) runs a pipeline and
engine per stack in one process. Every stack's engine reads and writes the same
cost cache and resource history. Today each engine opens its own database
files; the second open in the process times out and that stack silently runs
with no cache and no history.

**Why this priority**: Without it, stacks 2..N lose caching and history with
only a warning in the log, which corrupts the accuracy the web promises.

**Independent Test**: Build two engines and two pipelines in one process with
caching and history enabled and assert both hold a live store (the same shared
instance), that a value written through one is visible through the other, and
that the test fails on the previous behavior.

**Acceptance Scenarios**:

1. **Given** caching and history are enabled, **When** two stacks' engines are
   built in one process, **Then** both use a live cache and a live history
   store, and neither logs "proceeding without cache" or "proceeding without
   history".
2. **Given** the shared stores were created once, **When** the last stack
   shuts down, **Then** the stores are closed exactly once, after every stack
   has finished with them.
3. **Given** caching or history is disabled or fails to open, **When** stacks
   are built, **Then** every stack degrades the same way it does today for one
   stack: the failed store is nil and the other store stays live.
4. **Given** the overview pipeline records resource history and plan lineage,
   **When** it runs for a second stack, **Then** it writes through the shared
   history store instead of opening the file again.

---

### User Story 2 - Each stack resolves its own ESC environments (Priority: P1)

Each stack imports its own ESC environments through the `environment:` list in
its `Pulumi.<stack>.yaml`. In the example project `dev` imports six and
`tailscale` imports five. A stack's plugins must receive that stack's
environment values and no other stack's.

**Why this priority**: Today ESC is attached once from the raw `--stack` flag
value. A list such as `dev,tailscale` names no settings file, the not-found is
swallowed, and every stack silently loses its ESC environments.

**Independent Test**: Create a project with two stack settings files that list
different environments, resolve ESC for each stack, and assert each source
holds only its own environments.

**Acceptance Scenarios**:

1. **Given** stacks `dev` and `tailscale` with different `environment:` lists,
   **When** each stack's source is built, **Then** each resolves exactly its
   own list.
2. **Given** a stack with no settings file or no `environment:` key, **When**
   its source is built, **Then** it has no stack environments and the
   global/per-plugin ESC configuration still applies, as for a single stack.
3. **Given** the web is started with a stack list value, **When** ESC is
   attached for the command, **Then** the raw list is never used as a stack
   name.
4. **Given** one stack's source is refreshed after a plugin settings change,
   **When** another stack's source is read, **Then** the other is untouched.

---

### User Story 3 - A passphrase prompt reaches the right stack (Priority: P1)

Stacks may be encrypted with different passphrases. When stack `prod` needs a
passphrase, the prompt appears for `prod` and the entered text unlocks `prod`
only.

**Why this priority**: One shared unbuffered channel and one session mean a
second encrypted stack would take the first stack's answer or block it.

**Independent Test**: Wire two stacks with separate sessions and pipelines,
trigger the prompt on one, submit through its session, and assert only that
pipeline receives the text and only that session shows the prompt.

**Acceptance Scenarios**:

1. **Given** two stacks both need a passphrase, **When** the user submits for
   `prod`, **Then** only `prod`'s pipeline receives it and `dev` keeps
   waiting.
2. **Given** a stack's pipeline has stopped, **When** a passphrase is
   submitted for it, **Then** the request ends with that stack's cancellation,
   not another stack's.
3. **Given** a single stack, **When** it needs a passphrase, **Then** the
   prompt, the retry on rejection, and the text sent are exactly as today.

---

### User Story 4 - Requested stacks are validated against real stacks (Priority: P2)

The multi-stack caller turns `--stack a,b,c` into stack names. A helper lists
the project's stacks so a misspelled or missing name is reported with the
available names before any pipeline starts.

**Why this priority**: #2039 needs it, but the pipelines work without it.

**Independent Test**: Stub the Pulumi runner with a stack listing and assert
the helper returns every name, the current marker, and a clear error when the
listing fails or is malformed.

**Acceptance Scenarios**:

1. **Given** a project with stacks `dev`, `prod`, **When** the helper runs,
   **Then** it returns both names and marks the current one.
2. **Given** the listing command fails, **When** the helper runs, **Then** it
   returns a wrapped error, and a canceled context is preserved in it.
3. **Given** a requested name not in the listing, **When** validated, **Then**
   the error names the unknown stack and lists the available ones.

---

### User Story 5 - Single-stack use is unchanged (Priority: P1)

A user running `finfocus --web`, `finfocus overview`, or any CLI command sees
no difference: same files opened, same ESC resolution from `--stack`, same
passphrase flow, same warnings.

**Independent Test**: The existing CLI, web, and integration suites pass with
no assertion changed other than those that pin the removed per-engine opens.

**Acceptance Scenarios**:

1. **Given** one stack, **When** `finfocus --web` starts, **Then** it opens
   one cache and one history store, as before.
2. **Given** any non-web command, **When** it runs, **Then** it constructs its
   own stores and ESC source exactly as before.

### Edge Cases

- A store that is locked by another finfocus process still degrades to "no
  cache" or "no history" with one warning. Sharing only removes the
  in-process collision.
- History directory resolution depends on the project directory that built
  the config. Stacks of one project resolve to the same directory, so one
  shared store is correct; the project-tier safety gate runs once.
- A stack added after startup (lazy loading, #2039) receives the existing
  stores; it never opens a second handle.
- Closing a stack's pipeline must not close the shared stores.
- A stack name with no settings file is an absent environment, not an error,
  using the existing candidate-name rules.
- An ESC failure for one stack does not fail another stack's plugin launch.
- Per-stack plugin sets, query gates, and snapshots stay per stack; no shared
  mutable state is added beyond the two stores.
- Secrets: no passphrase or ESC value is logged, stored, or shared between
  stacks.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The web data path MUST create the cost cache and the resource
  history store once per process and inject them into every stack's engine and
  overview pipeline. No stack's engine or pipeline MAY open either file itself.
- **FR-002**: The shared stores MUST be closed exactly once, after every
  consumer has finished.
- **FR-003**: The overview pipeline's own history writes (state snapshot, plan
  lineage) MUST use the injected store when one is supplied, and open one only
  when none is, so non-web callers keep today's behavior.
- **FR-004**: Disabled or failed stores MUST yield the same degraded behavior
  and warning as today, applied once at startup rather than once per stack.
- **FR-005**: ESC resolution MUST be buildable for a named stack from that
  stack's own settings file. The web path MUST NOT read the raw `--stack`
  value to choose ESC environments.
- **FR-006**: A stack with an absent settings file or `environment:` key MUST
  resolve without stack environments and without error.
- **FR-007**: The shared global and per-plugin ESC configuration MUST be
  identical across stacks; only the stack environments differ.
- **FR-008**: Each stack MUST have its own passphrase channel, and the web
  session's submit callback and prompt MUST reach only that stack's pipeline
  and session.
- **FR-009**: A passphrase submission MUST end on its own stack's request or
  pipeline cancellation only.
- **FR-010**: The Pulumi package MUST provide a stack listing helper returning
  each stack's name and current marker, with the same cancellation handling
  as `GetCurrentStack`, and a validator for requested names that reports the
  unknown name and the available ones.
- **FR-011**: The config editor, log hub, audit context, dismissals, and
  plugin catalog MUST remain shared and unchanged.
- **FR-012**: Single-stack `--web`, `overview`, and every other CLI command
  MUST behave as before.

### Key Entities

- **Shared stores**: the one cost cache and one resource history for the
  process, with a single close.
- **Stack ESC source**: the ESC resolver for one stack, built from the shared
  global configuration and that stack's own environment list.
- **Stack passphrase wiring**: the channel and session callbacks owned by one
  stack.
- **Stack listing**: the names Pulumi reports for the project, with the
  current marker.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable — this is a prerequisite refactor of the web
  data path. No view, control, or data is added and no CLI feature gains a
  web hook. The tab bar and stack label are #2039's.
- **Web data**: None new.
- **Layout fit**: Unchanged.
- **Additions**: none
- **Render evidence**: A single-stack `finfocus --web` run against
  `../finfocus-demo` shows the Overview unchanged and a warm cache and
  history; recorded in the PR.
- **Follow-up issue scope**: None. #2039 carries the UX.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With two stacks in one process, 100% of engines hold a live
  cache and history store, and the regression test fails on the previous
  behavior.
- **SC-002**: Two stacks with different environment lists each resolve only
  their own list (automated test).
- **SC-003**: A passphrase submitted for one of two stacks reaches exactly
  that stack's pipeline and session (automated test).
- **SC-004**: The existing CLI, web, and integration suites pass with no
  change in single-stack behavior, and `make lint` and `make test` pass.
- **SC-005**: A single-stack web run logs zero new warnings and opens each
  database file exactly once.

## Assumptions

- Stacks of one project share one cache directory and one resource-history
  directory; per-stack cost history files (`<project>@<stack>.history.db`)
  are separate and out of scope.
- The shared stores are safe for concurrent use by several stacks' engines;
  the plan verifies this and adds guards only where it does not hold.
- #2039 supplies the stack list and the per-stack session registration; this
  feature provides the per-stack building blocks and keeps the single-stack
  caller wired through them.
- Plugin sets remain per stack because ESC-derived environment differs per
  stack; lazy loading is decided in #2039.
- Resource-history rows are keyed by the workspace-selected stack today
  (`detectHistoryStackContext`), not the pipeline's stack. That is existing
  behavior and out of scope here; #2039 must give each stack's context its own
  history stack key before several stacks write to the shared store.
