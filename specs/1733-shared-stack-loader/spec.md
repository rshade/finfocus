# Feature Specification: Shared Stack Loader

**Feature Branch**: `1733-shared-stack-loader`
**Created**: 2026-10-22
**Status**: Draft
**Input**: User description: "One shared stack loader for every command that
reads a Pulumi stack, so the same stack gives the same answer in cost
projected, forecast, estimate, recommendations, overview, actual and the web
dashboard."

A person asking FinFocus about one Pulumi stack should get one list of
resources, whichever command or screen they ask. Today they do not. On a real
stack (`initial-30` `dev`: 65 stack resources, 1 pending update, Pulumi 3.268)
`cost projected` priced 1 resource while `overview` listed 61. Both were
working as built: they read the stack differently.

## Background (observed 2026-10-22)

- `pulumi preview --json` leaves out unchanged resources by default. It still
  reports how many it left out (59 on the stack above), and
  `pulumi preview --json --show-sames` includes them (60 steps).
- `overview` reads the stack's recorded state and lays the preview on top of
  it, so unchanged resources always appear.
- `cost projected`, `cost forecast`, `cost estimate`, and
  `cost recommendations` read only the preview, so they see only the resources
  the preview lists: the changing ones, plus unchanged ones only when the
  preview was made to include them.
- The `cost projected` total covers every resource the preview lists, so it
  already includes unchanged resources when a saved preview lists them.
  Budgets, exit codes, and the GitHub Action read that total.
- `cost actual` reads the recorded state, then adds resources that were
  deleted, or whose cloud identity changed, during the billing period, taken
  from the history store. The web Cost view does the same. Those resources are
  absent from today's state and preview but still incurred charges.
- `overview` has more than one way of loading a stack (plain, terminal screen,
  and web dashboard), and the web dashboard keeps its own copies of the state
  and the plan. Its Estimate view already overlays the state on the plan, so it
  lists unchanged resources that `cost estimate` does not; its Recommendations
  and Cost views read only the state, so they miss resources that are pending
  creation.
- For Terraform and OpenTofu, `cost projected` already treats "nothing is
  changing" as a real outcome and says so. For Pulumi it gives a `$0` total
  with no explanation. Pull request #1897 (patch release) adds a warning and a
  `finfocus.emptyPlan` JSON flag for that case without changing any total.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See the same resources in every command (Priority: P1)

A person runs `finfocus overview`, `finfocus cost projected`,
`finfocus cost estimate`, and `finfocus cost recommendations` against the same
Pulumi stack and gets the same list of live resources from each, each command
then showing its own figures for those resources. The live inventory is every
resource that exists in the stack now or is pending a change.

**Why this priority**: This is the defect. A cost tool whose commands disagree
about what is in the stack cannot be trusted, and a person cannot tell which
answer is the complete one.

**Independent Test**: Run all four commands against one stack that has both
unchanged resources and a pending change. Compare the resource identities each
one reports. The sets match. Delivers a trustworthy answer even if no other
story ships.

**Acceptance Scenarios**:

1. **Given** a stack with 60 unchanged resources and 1 pending update,
   **When** the person runs `overview` and `cost projected`, **Then** both
   report the same 61 resources and each resource carries whether it is
   unchanged, created, updated, replaced, or deleted.
2. **Given** a stack with only pending creates, **When** the person runs any
   of the four commands, **Then** the created resources appear in each, marked
   as creates.
3. **Given** a resource that is being replaced, **When** the person runs any
   command, **Then** it appears once, as a replacement, in every command.
4. **Given** a resource that was deleted earlier in the billing period and is
   absent from the current state and preview, **When** the person runs
   `cost actual` for that period, **Then** its charges are still reported, and
   the live commands do not list it.

---

### User Story 2 - Be told when nothing is changing (Priority: P1)

A person runs `cost projected` on a stack with no pending changes and gets a
clear statement of that, in the same words and shape as the Terraform and
OpenTofu "no changes" result, instead of a `$0` total that looks like an
estimate.

**Why this priority**: A silent `$0` led a skill to report a made-up result.
The explanation must exist whatever answer is chosen for what the total
covers.

**Independent Test**: Run `cost projected` on a stack with no pending changes,
once with table output and once with JSON. Delivers an unambiguous result on
its own.

**Acceptance Scenarios**:

1. **Given** a Pulumi stack with no pending changes, **When** the person runs
   `cost projected`, **Then** the output says there are no changes and does
   not present `$0` as an estimate.
2. **Given** the same stack and `--output json`, **When** the command runs,
   **Then** the output is still valid JSON that says the plan is empty, and a
   program reading it can tell "nothing to price" from "priced at zero".
3. **Given** a Terraform or OpenTofu project with no changes, **When** the
   person runs `cost projected`, **Then** the result is the same kind of
   statement as in Scenario 1.

---

### User Story 3 - The terminal and the web dashboard cannot disagree (Priority: P2)

A person looks at the same stack in the terminal overview and in
`finfocus --web` and sees the same resources, because both come from the one
loader.

**Why this priority**: `finfocus --web` was built to reuse the command line's
methods so the two stay in step. Two loaders inside one command breaks that
promise.

**Independent Test**: Load one stack in the terminal overview and in the web
Overview, Estimate, and Recommendations views. Compare resource identities.
They match.

**Acceptance Scenarios**:

1. **Given** a stack with unchanged and pending resources, **When** the person
   opens the web Estimate picker and the web Recommendations view, **Then**
   both offer the same resources as `cost estimate` and
   `cost recommendations` on the command line.
2. **Given** the person runs a preview from the web Overview, **When** it
   completes, **Then** the rows match what the terminal shows after the same
   preview.

---

### User Story 4 - A saved plan file still works, and says what it cannot show (Priority: P2)

A person who passes a saved preview file (`--pulumi-json`) with no stack state
gets results for what the file contains, plus a plain note of how many
unchanged resources the file left out.

**Why this priority**: Continuous integration jobs and the GitHub Action feed
saved plans. They have no stack to read, and they must keep working.

**Independent Test**: Run `cost projected --pulumi-json plan.json` with a
default preview file from a stack that has unchanged resources. The note names
the omitted count taken from the file itself.

**Acceptance Scenarios**:

1. **Given** a default preview file that reports 59 unchanged resources it did
   not list, **When** the person runs `cost projected --pulumi-json` on it,
   **Then** the listed resources are priced and the output says 59 unchanged
   resources are not included and how to include them.
2. **Given** a preview file made with unchanged resources included, **When**
   the person runs the command, **Then** no such note appears.

---

### User Story 5 - A stack that cannot be read is reported, never guessed (Priority: P3)

A person whose stack state cannot be read (no access to the backend, a
missing passphrase, a failed read) gets the preview-only result together with
a clear statement that unchanged resources could not be included and why.

**Why this priority**: Reading state is the new dependency. Its failure must
not turn into a silent partial answer or a hard failure that blocks commands
that worked before.

**Independent Test**: Run a command with the stack state unreadable. The
command still returns what the preview shows and names the reason.

**Acceptance Scenarios**:

1. **Given** the state cannot be read, **When** the person runs
   `cost projected`, **Then** the preview resources are priced and the output
   says the stack state was unavailable and that unchanged resources are
   missing.
2. **Given** the failure, **When** the person reads JSON output, **Then** the
   incompleteness is a field a program can test.

---

### User Story 6 - Existing totals and budgets keep their meaning (Priority: P1)

A person whose budget, exit-code check, or GitHub Action reads the
`cost projected` total gets the same total and the same budget result as in the
previous release, and can also see the stack total.

**Why this priority**: Changing what the total means would silently change
who passes and fails a budget.

**Independent Test**: Run `cost projected` with a default preview, and with a
preview that lists unchanged resources, against the same stack, with and
without readable state, and compare against the previous release.

**Acceptance Scenarios**:

1. **Given** a default preview that lists only the changing resources and a
   readable state, **When** the person runs `cost projected`, **Then** the
   plan total equals the previous release's total, and the stack total is
   larger by the cost of the unchanged resources.
2. **Given** a saved preview that lists unchanged resources, **When** the
   person runs `cost projected`, **Then** the plan total equals the previous
   release's total, which includes those resources, and the stack total counts
   each of them once.
3. **Given** a budget threshold set between the plan total and the stack
   total, **When** the person runs `cost projected`, **Then** the budget result
   is the one the previous release gave, which is the result for the plan
   total.
4. **Given** a preview and no readable state, **When** the person runs
   `cost projected`, **Then** the plan total is unchanged and the output says
   the stack total could not be completed.

---

### Edge Cases

- A resource that appears in the preview but not yet in the state (a create)
  appears once.
- A resource in the state with a pending delete appears once, as a delete.
- A replacement has an old and a new identity in the preview and one identity
  in the state; it appears once.
- Providers, the root stack, and other bookkeeping resources are not counted
  as priceable resources in any command.
- Imported or externally created resources keep the treatment each command
  gives them today.
- A stack with no custom resources at all is an empty result, not an error.
- A filter that removes every resource is a filtered result, not an empty
  stack.
- An encrypted stack with no passphrase: the failure names the passphrase, as
  today, and never prints a secret value.
- Large stacks: loading must stay usable for stacks of several hundred
  resources.
- Terraform and OpenTofu sources continue through their own plan path; this
  feature must not change them beyond using the same "no changes" wording.
- A preview and a state that disagree (the preview was made for a different
  stack revision): the result says so and does not merge rows it cannot
  match.
- A resource deleted earlier in the billing period, or one whose cloud
  identity changed in it, is not in the live inventory. `cost actual` and the
  web Cost view still report its charges.
- State supplied with no preview (`--pulumi-state` on its own): there is no
  preview to list resources, so the plan total covers the state resources, as
  it does today, and they are labeled unchanged and not created.
- A saved preview that lists unchanged resources, with a readable state that
  also holds them: each resource is counted once in the stack total and as the
  preview lists it in the plan total.
- A budget threshold between the plan total and the stack total passes or fails
  by the plan total only.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: For a given Pulumi stack, `cost projected`, `cost forecast`,
  `cost estimate`, `cost recommendations`, `overview`, `cost actual`, and
  `finfocus --web` MUST derive their live inventory (the resources that exist
  in the stack now or are pending a change) from one shared loading step, so
  the set of live resource identities is the same in all of them. `cost actual`
  takes the deployed part of it, the resources that are in the stack state: a
  resource that is only planned has no charges to report.
- **FR-002**: The loading step MUST combine the recorded stack state with the
  preview, and every resource MUST carry its change kind: unchanged, create,
  update, replace, or delete.
- **FR-003**: Each command MUST show its own figures for those resources and
  MUST NOT compute live membership itself. A command MAY add resources beyond
  the live inventory only where its meaning requires it, as FR-013 does for
  `cost actual`.
- **FR-004**: The terminal overview and the web dashboard MUST use the same
  loading step, with no second path inside `overview`.
- **FR-005**: When the plan has no changes, `cost projected` MUST say so in
  the same form for Pulumi as for Terraform and OpenTofu, and MUST NOT present
  a `$0` total as an estimate.
- **FR-006**: JSON and NDJSON output MUST remain machine-readable in the
  no-changes case, and the `finfocus.emptyPlan` flag from the patch release
  MUST keep its meaning: the plan held nothing to price.
- **FR-007**: When only a saved preview file is available, the commands MUST
  work on the resources it lists and MUST report how many unchanged resources
  the file omitted, using the count the file reports.
- **FR-008**: When the stack state cannot be read, the commands MUST return the
  preview result, MUST say that unchanged resources are missing and why, and
  MUST expose that as a field in JSON output.
- **FR-009**: Credentials and Pulumi secret values MUST NOT reach a plugin,
  a log, or an error, as today; reading the state MUST NOT widen what is sent.
- **FR-010**: `cost projected` MUST report two totals side by side:
  - The plan total is the cost of exactly the resources the preview itself
    lists, as today. It includes unchanged resources when, and only when, the
    preview lists them. Resources the shared loading step adds from the stack
    state MUST NOT enter it.
  - The stack total is the cost of the whole live inventory, with each
    resource counted once even when both the preview and the state hold it.

  Budgets, exit codes, and the GitHub Action MUST keep reading the plan total,
  so no existing result changes. In JSON output the existing total keeps its
  name and meaning, and the stack total is an added field.
- **FR-011**: Behavior for Terraform and OpenTofu projects MUST be unchanged,
  apart from sharing the "no changes" wording in FR-005.
- **FR-012**: Resources whose change kind is unchanged MUST be priced once
  (not as a before and an after), as the projected diff already does.
- **FR-013**: `cost actual` and the web Cost view MUST keep adding, for the
  requested billing period, the resources deleted in it and the earlier cloud
  identities of resources that changed identity in it, so historical charges
  are not dropped. They MUST add them to the live inventory from the shared
  loading step.

### Key Entities *(include if feature involves data)*

- **Live inventory**: The resources of one stack at one moment that exist now
  or are pending a change, from its recorded state combined with a preview.
  Each resource has an identity, a type, its properties, and a change kind.
  Resources that existed only earlier in a billing period are not part of it.
- **Change kind**: One of unchanged, create, update, replace, delete. The
  single field every command reads to decide how to treat a resource.
- **Omitted-unchanged count**: How many unchanged resources a preview left out
  without listing them; used when no state is available.
- **Plan total and stack total**: The plan total is the cost of the resources
  the preview lists, which is today's `cost projected` total and the one
  budgets read. The stack total is the cost of the whole live inventory, each
  resource counted once.

## Web UI Fit *(mandatory)*

- **Disposition**: Extends the existing Overview, Estimate, and
  Recommendations views. No new view.
- **Web data**: The same live resource list the command line reports for the
  stack, with each resource's change kind. The command line changes to match
  what the web Estimate view already offers; the Recommendations view gains the
  resources that are pending creation, which it lacks because it reads only
  the state. The Overview already shows both.
- **Layout fit**: The added resources fill the existing captioned tables and
  the Estimate resource picker. No new slot, control, or column. The Cost view
  keeps the historical resources it reports today.
- **Additions**: None. Any "unchanged resources not included" notice reuses the
  status line already used for the state-only run.
- **Render evidence**: Not yet captured. Planning must render the Overview,
  Estimate, and Recommendations views against a populated stack
  (`initial-30` `dev`) with Playwright and compare them with the terminal
  before the design is final.
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility). No engine, API, redaction, or data-shape work.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On `initial-30` `dev` (65 stack resources, 1 pending update),
  `overview`, `cost projected`, `cost estimate`, and `cost recommendations`
  report identical live resource sets: 100% agreement, where today they
  report 61 and 1.
- **SC-002**: A stack with no pending changes never produces a `$0` total
  without an explanation, in table, JSON, or NDJSON output, across 100% of
  runs.
- **SC-003**: Opening the same stack in the terminal overview and the web
  Overview, Estimate, and Recommendations views shows identical resource sets.
- **SC-004**: Running a command with the stack state unreadable still returns a
  result and a stated reason in 100% of runs, with no new hard failure.
- **SC-005**: Time to the first result on a 65-resource stack increases by no
  more than 3 seconds compared with today's `cost projected`.
- **SC-006**: No existing Terraform or OpenTofu test changes its expected
  output.
- **SC-007**: For every input, including saved previews that already list
  unchanged resources, the `cost projected` plan total, and the budget and
  exit-code results derived from it, equal what the previous release produced
  for the same inputs. The stack total is the only new figure.
- **SC-008**: When the stack state is readable, the stack total is the same
  whether the preview lists the unchanged resources or leaves them out, and
  counts each resource once.
- **SC-009**: For a billing period in which a resource was deleted,
  `cost actual` reports the same charges for it as the previous release, in
  100% of runs.

## Assumptions

- Reading the stack state needs the same backend access and passphrase that
  running a preview already needs, so no new setup is asked of the person.
- `cost actual` already reads state and adds historical resources for the
  billing period; it adopts the shared step for its live inventory and keeps
  that history step, so its figures do not change.
- A saved plan file with no state keeps working exactly as it does now, plus
  the omitted-count note.
- The 3 second budget in SC-005 is a target to confirm during planning against
  a measured state read.
- The patch-release warning and `finfocus.emptyPlan` flag (#1897) ship first;
  this feature keeps them valid and may later fold them into the
  no-changes result.
- The owner chose option C for the totals question on 2026-10-22: both totals
  are reported, and budgets keep reading the existing total. A review the same
  day showed the existing total covers every resource the preview lists, not
  only the changing ones, so it is called the plan total and keeps its
  membership exactly.
- Constitution Principle VII applies; the Web UI Fit above is the record.

## Out of Scope

- Changing how any plugin prices a resource.
- New output formats or new commands.
- Terraform and OpenTofu planning, apart from the shared wording.
- Changing budget thresholds or exit codes. Budgets keep reading the plan
  total (FR-010).
- Making the stack total the figure budgets read. That would be a deliberate,
  separate change with a migration note.
- Reading state from anything other than the active Pulumi backend.
