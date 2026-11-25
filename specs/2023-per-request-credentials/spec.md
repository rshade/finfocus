# Feature Specification: Per-Request Credentials for Pooled Plugins

**Feature Branch**: `2023-per-request-credentials`
**Created**: 2026-11-25
**Status**: Implemented
**Input**: GitHub issue rshade/finfocus#1539, "feat(pluginhost): pool opted-in
plugins and pass per-request credentials". Builds on the shared plugin pool
(`specs/2016-shared-plugin-pool/`, which deferred this case) and on the
credential channel in finfocus-spec (`specs/056-per-request-credentials/` in
that repository, rshade/finfocus-spec#550).

## Overview

A multi-stack dashboard (`finfocus --web --stack dev,prod`) shares a plugin
process between stacks only when both stacks give the plugin the same
environment. Stacks that differ in nothing but a cloud credential each start
their own copy. This feature lets such stacks share one process of a plugin
that accepts credentials per call, when the user asks for it.

A **tenant** here is one stack of a multi-stack dashboard, with its own ESC
environments. It is the only place where one finfocus process serves several
sets of credentials.

## Clarifications

### Session 2026-11-21

- Q: How does finfocus decide which of a stack's values become per-request
  credentials, and under which names? → A: The user declares a map in
  `esc.plugins.<name>.credentials`, from the credential name the plugin reads
  to the ESC variable that holds it. Declaring the map is the host-side
  opt-in. Names are never derived from variable names, because plugins choose
  their own (one existing plugin rejects any name it does not know), and core
  carries no per-plugin name table.

### Session 2026-11-25

- Q: Must a declared variable also be listed in `variables`? → A: No. Naming a
  variable for a plugin under `credentials` allows that plugin to receive it,
  as `variables` does. Otherwise the CLI and a multi-stack dashboard could
  price with different accounts from one configuration.
- Q: What if the plugin does not advertise support? → A: The variable is
  delivered in the plugin's environment, in a process of its own, as today.
- Q: What if a stack's values do not fit the contract's limits (an empty or
  multi-line value, more than 16 entries, over 4096 characters)? → A: That
  stack keeps its own process with the values in the environment. The contract
  is not changed and nothing is truncated.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Stacks with different credentials share one process (Priority: P1)

An engineer opens a dashboard on several stacks. Each stack imports its own
cloud credentials from ESC. The plugin accepts credentials per call, and the
engineer has declared which variables are its credentials. The plugin runs
once, and every call carries only the credentials of the stack that made it.

**Why this priority**: It is the issue's purpose. The rest protects it.

**Independent Test**: Two stacks with different credential values, one
supporting plugin, and a declared map. Count launches, and have the plugin
report the credentials each call carried.

**Acceptance Scenarios**:

1. **Given** a supporting plugin and a declared map, **When** stacks `a` and
   `b` with different credential values open it, **Then** both use one running
   process.
2. **Given** that shared process, **When** both stacks call it at the same
   time, **Then** each call carries exactly the calling stack's credentials.
3. **Given** that shared process, **When** its environment is read, **Then** it
   holds no declared variable's value from either stack.
4. **Given** a stack whose ESC environments supply none of the declared
   variables, **When** it uses the shared process, **Then** its calls carry no
   credentials and still run.

---

### User Story 2 - Everything else stays isolated (Priority: P1)

A plugin that does not advertise support, a plugin with no declared map, a
single-stack dashboard, and every CLI command keep the current behavior: one
process per environment with credentials in that environment.

**Why this priority**: Process isolation is the security default, and a shared
process has a larger blast radius.

**Independent Test**: Remove either opt-in and assert two stacks get two
processes, each holding only its own value, and no call carries credentials.

**Acceptance Scenarios**:

1. **Given** a declared map and a plugin that does not advertise support,
   **When** two stacks open it, **Then** two processes run, each with only its
   own stack's value, and calls carry no credentials.
2. **Given** a supporting plugin and no declared map, **When** two stacks open
   it, **Then** the result is the same.
3. **Given** a declared map and a run without a pool, **When** the plugin
   opens, **Then** the declared variables are in its environment and calls
   carry no credentials.
4. **Given** stacks that also differ in a value that is not a declared
   credential, **When** both open a supporting plugin, **Then** they get
   separate processes.
5. **Given** a process launched without declared variables, **When** that
   process does not advertise support after all, **Then** no stack uses it: it
   is replaced by a process that holds the stack's values in its environment.

---

### User Story 3 - Credentials stay out of records and caches (Priority: P1)

A credential never appears in finfocus logs, error text, or metrics, and one
stack's cached costs are never served to another because they share a process.

**Why this priority**: A leaked secret, or prod costs shown for dev, defeats
the isolation that sharing gives up.

**Independent Test**: A plugin that echoes its credential in an error. Assert
the fixture secret is in no log line and no error string, and that two stacks
on one process have different delivered-settings digests.

**Acceptance Scenarios**:

1. **Given** a plugin whose error repeats a credential value, **When** the call
   fails, **Then** neither the log line nor the returned error contains it, and
   the error keeps its status code.
2. **Given** two stacks with different credentials on one process, **When**
   their cost cache keys are built, **Then** the keys differ. A stack whose
   declared map changes also gets different keys.
3. **Given** a map with an unusable name or variable, **When** it is validated,
   **Then** the message names the setting and repeats neither.
4. **Given** a plugin process that exited, **When** another program binds its
   port, **Then** finfocus does not connect to it.
5. **Given** another program that holds a port before the plugin binds it,
   **When** it answers as a plugin that supports credentials, **Then** it
   receives none.

### Edge Cases

- A stack supplies only some declared variables: those are attached, and the
  plugin answers for an incomplete set.
- A declared variable set in the shell but not by ESC stays in the process
  environment as an ambient value. It is the same for every stack, and it is
  never attached to a call.
- Two names may map to one variable.
- Names that differ only in letter case are rejected by validation.
- Config changes while the dashboard runs: a later open uses the new map, and
  existing holders keep what they resolved.
- ESC reports a new credential on reload: the reload's client carries it, the
  earlier client keeps its own, and no process restarts for it.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users MUST be able to declare, per plugin, a map from credential
  name to ESC variable in `esc.plugins.<name>.credentials`.
- **FR-002**: A declared variable MUST be allowed for that plugin as if listed
  in `esc.plugins.<name>.variables`. No other plugin receives it.
- **FR-003**: With a pool, a declared map, a usable credential set, and a
  plugin that advertises `supports_per_request_credentials=true`, the process
  MUST be launched without ESC values for the declared variables and MUST be
  shared by every stack whose remaining environment matches.
- **FR-004**: Each call by a stack on such a process MUST carry that stack's
  credentials and no others, on every service the plugin serves.
- **FR-005**: A stack with no value for any declared variable MUST be able to
  use the process, and its calls MUST carry no credentials.
- **FR-006**: If any condition of FR-003 is missing, the plugin MUST receive
  the values in its environment, in a process no other stack's credentials
  reach, and calls MUST carry no credentials.
- **FR-007**: A stack MUST NOT use a process that was launched without its
  credentials and does not advertise support.
- **FR-008**: Process environment variables of finfocus itself MUST NOT change.
- **FR-009**: Credential names and values MUST NOT appear in logs, error
  strings, metrics, or telemetry spans written by the host. A value repeated by a plugin error
  MUST be masked with the status code kept.
- **FR-010**: A client's delivered-settings digest MUST cover the stack's full
  environment as a process of its own would receive it and, when its calls
  carry credentials, the map that selects them. A client whose calls carry none
  MUST report the digest an isolated process reports.
- **FR-011**: Validation MUST reject a credential name or variable name the
  contract or environment cannot carry, names equal when letter case is
  ignored, reserved startup variables, and more than 16 entries. Messages MUST
  NOT repeat the entry.
- **FR-012**: A connection MUST NOT be re-established to a plugin's address
  once the plugin process is known to have exited, even while another process
  holds its output open.
- **FR-013**: Without a pool, launches MUST be unchanged except for FR-002.
- **FR-014**: Holds, crash replacement, settings reload, and shutdown MUST
  keep the pool's current behavior.
- **FR-015**: A process MUST receive credentials only after it proved that it
  holds the port the host connects to. A plugin that gives no proof keeps its
  credentials in its environment, in a process of its own.

### Key Entities

- **Credential map**: Per plugin, credential name to ESC variable. Holds no
  secret.
- **Credential set**: One stack's values under the declared names. Held by that
  stack's client for its lifetime and never by a process.
- **Shared process**: A pool process launched without the declared values,
  whose running binary advertises support.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable. The feature changes how many plugin
  processes the multi-stack dashboard starts, not what it shows. The map is
  edited in `config.hujson`, like the other `esc.plugins.<name>` keys, none of
  which the Config page lists.
- **Web data**: None new.
- **Layout fit**: None.
- **Additions**: None.
- **Render evidence**: Not required for a no-surface change. The multi-stack
  web e2e suite (`make test-e2e-web`) covers the unchanged views.
- **Follow-up issue scope**: None.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: N stacks that differ only in declared credentials run 1 process
  of a supporting plugin.
- **SC-002**: Under concurrent calls from two stacks, 0 calls carry another
  stack's credentials.
- **SC-003**: With either opt-in missing, 0 processes are shared between
  stacks with different credentials.
- **SC-004**: 0 log lines and 0 error strings contain a fixture secret.
- **SC-005**: 0 cost cache entries are reused between stacks with different
  credentials or between different maps, and 0 cost cache keys change where no
  call carries credentials.

## Assumptions

- Only ESC values differ between stacks of one dashboard (as in spec 2016).
- A plugin that advertises support separates tenants inside its process,
  including any cache it keeps. The host cannot verify that.
- Plugins keep their own log lines free of credentials, as the SDK requires.
- Calls to plugins use loopback without transport encryption, as before.

## Out of Scope

- Changing finfocus-spec or the wire format.
- Deriving names, or a plugin advertising them.
- `config set` support for the map, and any Config page control.
- Sharing between separate finfocus processes, or CLI runs.
