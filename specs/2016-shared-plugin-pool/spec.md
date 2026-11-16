# Feature Specification: Shared Plugin Processes Across Web Stacks

**Feature Branch**: `2016-shared-plugin-pool`
**Created**: 2026-11-15
**Status**: Implemented
**Input**: GitHub issue rshade/finfocus#2086 — "feat(web): share plugin
processes across stacks whose plugin environment matches". Builds on the
multi-stack dashboard (`specs/2015-multi-stack-web-tabs/`, #2039/#2085) and the
per-process stores of `specs/2013-per-stack-web-data/`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Opening more stack tabs does not multiply plugin processes (Priority: P1)

An engineer runs `finfocus --web --stack dev,prod,test` and opens every tab.
Today each activated stack starts a complete plugin set of its own. On a real
four-stack project this meant four aws-public routers and eight region
children of about 0.9 GiB each, and the host's console locked up. A plugin
that receives the same environment in every stack should run once and serve
every stack that needs it.

**Why this priority**: It is the reported problem. Without it the multi-stack
dashboard is unusable on an ordinary workstation once a few tabs are open.

**Independent Test**: Activate two stacks whose plugins resolve the same
environment and count plugin launches: each installed plugin starts once.

**Acceptance Scenarios**:

1. **Given** `--stack a,b` and a plugin whose delivered environment is the
   same for both stacks, **When** both stacks are activated, **Then** that
   plugin binary is launched once and both stacks price through it.
2. **Given** two stacks share a plugin process, **When** both price resources
   at the same time, **Then** both receive the same answers they receive
   today from separate processes.
3. **Given** two stacks share a plugin process, **When** one stack's request
   arrives while the shared process is still starting, **Then** it waits for
   that start instead of launching a second process.

---

### User Story 2 - Stacks with different plugin environments stay isolated (Priority: P1)

Each stack may import its own ESC environments, so a plugin's environment can
differ between stacks (for example, a credential that only `prod` imports).
Such a plugin must keep one process per distinct environment, and no stack's
value may reach a process another stack uses.

**Why this priority**: The credential rule (no credential or ESC value reaches
another stack's plugin) is a hard security invariant. Sharing must never
weaken it.

**Independent Test**: Give one stack an ESC delivery the other lacks, activate
both, and assert two processes were launched and neither environment holds the
other stack's value.

**Acceptance Scenarios**:

1. **Given** stack `a` delivers `X=1` to a plugin and stack `b` delivers
   nothing extra, **When** both are activated, **Then** two processes run,
   `a`'s has `X=1`, and `b`'s has no `X`.
2. **Given** two stacks whose environments differ only in one value, **When**
   both are activated, **Then** each gets its own process.

---

### User Story 3 - One stack's lifecycle cannot break another's plugins (Priority: P1)

Stopping a stack, cancelling its pipeline, or saving plugin settings in one
stack must not close or kill a plugin process that another stack still uses.
A process stops once nothing uses it, and the dashboard's shutdown stops
every process it started.

**Why this priority**: A shared process launched under one stack's lifetime
would die with that stack and break every other tab.

**Independent Test**: Share a process between two stacks, stop or cancel one,
and assert the other's client still answers; release the last holder and
assert the process was stopped once.

**Acceptance Scenarios**:

1. **Given** stacks `a` and `b` share a process, **When** stack `a`'s
   pipeline is cancelled or the stack is stopped, **Then** the process keeps
   running and stack `b` keeps pricing.
2. **Given** stacks `a` and `b` share a process, **When** a plugin settings
   save in stack `a` reloads its plugins, **Then** stack `a` receives a
   process with the new environment and the old process keeps running for
   `b`; once `b` releases it, the old process is stopped.
3. **Given** the dashboard holds pooled processes, **When** it receives
   SIGINT or SIGTERM, **Then** every pooled process and its process group is
   stopped and no plugin process outlives the dashboard.

---

### User Story 4 - Single-stack and CLI runs are unchanged (Priority: P2)

`finfocus --web` with one stack, the TUI, and every CLI command open plugins
once per process already. They keep today's launch path exactly.

**Why this priority**: Regressions there would affect every user, while the
pool only helps the multi-stack dashboard.

**Independent Test**: Run a single-stack session and a CLI command and assert
no pool is in use and plugins launch as before.

**Acceptance Scenarios**:

1. **Given** `finfocus --web` with one or no `--stack`, **When** it runs,
   **Then** plugins are launched and closed exactly as before.
2. **Given** an unshared plugin client, **When** it prices a resource,
   **Then** its projected cache key is identical to today's.

### Edge Cases

- **A shared process crashes**: the next request for that environment starts a
  fresh process. Stacks that still hold the dead process see failures, as a
  single stack does today, until they reopen plugins.
- **A launch fails** (incompatible spec version, bind timeout): every stack
  waiting on that launch receives the failure, and the next request tries
  again instead of reusing the failure.
- **The requesting stack is cancelled while a launch is in progress**: the
  launch is not killed. If no other stack wants the process when it finishes,
  it is stopped. A launch every requester gave up on is no longer offered, so
  a launch that never answers cannot block later requests.
- **A plugin restarts itself once to apply advertised settings**: the first
  launch of a binary teaches the pool its advertised settings. Later requests
  resolve the exact environment they would receive and launch with it
  directly, so they need no restart.
- **Two stacks ask for a binary before its first launch finishes**: a request
  with a different ESC offer waits (up to 30 seconds) for that launch to
  report the settings, then shares the process if its environment matches.
  This is the order in which a person opens tabs quickly, and the initial-30
  check found it.
- **Plugin probes** (settings read, validate) are request-scoped and launch
  their own short-lived process, as today.
- **Settings changes and other stacks**: a settings save reloads only the
  stack where it was made, as today. Other stacks keep their process and see
  the new settings at their next plugin open.
- **Plugin install or removal in one stack**: a new version is a different
  binary, so it gets its own process. Stacks that hold the old version keep
  it until they reload. Reinstalling the same version at the same path also
  counts as a different binary.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A multi-stack web dashboard (two or more stacks) MUST keep at
  most one running plugin process per plugin binary and exact delivered
  environment, and give it to every stack that requests that combination.
- **FR-002**: Two stacks whose delivered environment for a plugin differs in
  any variable name or value MUST NOT share that plugin's process.
- **FR-003**: A shared process MUST NOT be stopped while any stack holds it.
  Releasing a stack's hold MUST stop the process when it was the last hold.
- **FR-004**: The lifetime of a shared process MUST be owned by the dashboard
  process, not by the stack, pipeline, or request that first asked for it.
  Cancelling that requester MUST NOT kill the process.
- **FR-005**: Concurrent requests for the same binary and environment MUST
  result in a single launch.
- **FR-006**: A failed launch MUST be reported to every requester waiting on
  it and MUST NOT be reused by a later request.
- **FR-007**: A process whose connection has failed or shut down MUST NOT be
  given to a new requester; the next request launches a replacement.
- **FR-008**: The recommendation scorer plugin and plugin reloads after a
  settings save, install, or removal MUST go through the same sharing rule.
- **FR-009**: Dashboard shutdown MUST stop every process the pool started,
  including any still held, together with each one's process group.
- **FR-010**: Single-stack `--web`, the TUI, the analyzer, and every CLI
  command MUST NOT use sharing and MUST launch plugins exactly as before.
- **FR-011**: A shared client MUST report the same delivered-settings digest
  as an unshared client with the same environment, so projected cache keys do
  not change.
- **FR-012**: Each stack MUST receive its own plugin client record, so
  per-stack metadata changes (registry metadata merge) cannot race between
  stacks.
- **FR-013**: Stacks whose ESC environments differ only in values a plugin
  never receives (neither advertised by the plugin nor listed for it in
  `esc.plugins.<name>.variables`) MUST share that plugin's process, including
  when both stacks request it before its first launch has finished.

### Key Entities

- **Plugin pool**: One per multi-stack dashboard process. Holds the shared
  plugin processes and their hold counts, and owns their lifetime.
- **Pool key**: The plugin binary (path and file identity) plus either the
  exact environment its process would receive, once the binary's advertised
  settings are known (final key), or, before that, the launch environment and
  every value the stack's ESC environments offer the plugin (offer key).
- **Hold**: A stack's (or reload's, or scorer's) reference to a shared
  process. Closing the stack's client releases the hold once.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable - this changes how many plugin processes
  the existing multi-stack dashboard starts, not what it shows. Every view,
  value, and interaction stays the same.
- **Web data**: None new.
- **Layout fit**: None.
- **Additions**: None.
- **Render evidence**: Not required for a no-surface change; the existing
  multi-stack web e2e suite (`make test-e2e-web`) covers the unchanged views.
- **Follow-up issue scope**: None; no follow-up issue is filed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With N activated stacks whose plugin environments match, each
  plugin runs exactly once (1 process instead of N).
- **SC-002**: With two stacks whose environments differ, each plugin runs
  once per distinct environment, and 0 values cross between stacks.
- **SC-003**: After dashboard shutdown, 0 plugin processes started by the
  dashboard remain.
- **SC-004**: Stopping or reloading one stack causes 0 failures in another
  stack's plugin calls.
- **SC-005**: Single-stack and CLI runs show 0 changes in plugin launches and
  projected cache keys.
- **SC-006**: On `initial-30` with `--stack dev,test` activated together
  (`dev` imports six ESC environments, `test` one), each plugin runs once in
  repeated runs, and both stacks show the same rows and projected costs as
  before.

## Assumptions

- Plugins already accept concurrent RPCs: one engine issues parallel calls
  during enrichment. Sharing adds callers, not a new concurrency pattern.
- The parent process environment and the global and project config files are
  the same for every stack in one dashboard process. Only ESC deliveries
  differ per stack, and they are part of the pool key.
- The environment after the one advertised-settings restart is a function of
  the launch environment, the stack's ESC offer, and the binary's advertised
  settings. Before the first launch an ESC value for an advertised setting is
  not yet allowed, so two stacks with different ESC credentials resolve the
  same launch environment. Until a binary's settings are known, the key
  therefore covers the stack's whole ESC offer; keying on the launch
  environment alone would hand one stack's credential to another.
- A binary's advertised settings do not change between launches of the same
  file. The pool does not rely on it: a request whose predicted environment
  differs from what the process received retries with the offer key.
- Idle processes of a stack whose tab was left keep running until the stack
  stops or the dashboard exits (stopping idle stacks is out of scope).

## Out of Scope

- Sharing one process between stacks whose environments differ (#1539).
- Memory use of aws-public region children (belongs to the plugin).
- Stopping an idle stack's plugins after its tab is left.
- CLI commands that run one stack per process.
