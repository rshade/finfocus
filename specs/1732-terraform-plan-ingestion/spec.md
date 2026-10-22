# Feature Specification: Terraform Plan Ingestion

**Feature Branch**: `1732-terraform-plan-ingestion`
**Created**: 2026-10-20
**Status**: Implemented
**Input**: `terraform-plan-ingestion-design.md` (owner draft, 2026-10-17).
A person working in a Terraform directory can run the same FinFocus commands
they already run in a Pulumi project and get a projected cost before apply,
without writing a plan file by hand.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Project a Terraform directory with no flags (Priority: P1)

A person is in a directory of Terraform configuration. They run
`finfocus cost projected` with no input file. FinFocus sees the `.tf` or
`.tf.json` files, runs a plan, and prints the projected cost. They do not
pass a var-file, select a workspace, or point at a subdirectory.

**Why this priority**: Today Terraform support starts only after apply, from
a state file. The gap this feature closes is a projection before apply.

**Independent Test**: In an initialized Terraform directory, with credentials
the tool can already use, run `finfocus cost projected` with no input flags.
The result is a projected cost for the planned resources. No plan file
remains in the project directory.

**Acceptance Scenarios**:

1. **Given** the current directory contains `.tf` or `.tf.json` files and no Pulumi project file is nearer, **When** the person runs `finfocus cost projected` with no input file, **Then** FinFocus plans that directory and prints a projected cost.
2. **Given** `tofu` is on the path, **When** FinFocus plans, **Then** it uses `tofu` and does not ask the person to name the binary.
3. **Given** `tofu` is not on the path, **When** FinFocus plans, **Then** it fails with the install hint and does not fall back to `terraform`.
4. **Given** a plan that includes resources inside modules, **When** the projection is printed, **Then** those resources are included and each one is identified by its address, such as `aws_instance.web[0]`.
5. **Given** the plan proposes no creates, updates, deletes, or replaces, **When** the command finishes, **Then** it succeeds, reports zero projected cost, and prints an informational message. A refresh that finds nothing to apply, and a plan that reports only drift (changes made outside Terraform), are this case.
6. **Given** the person interrupts the command, **When** it stops, **Then** the plan process stops and no plan file remains in the project directory.

---

### User Story 2 - Hear what to fix when a plan cannot run (Priority: P1)

The directory is Terraform, but the plan cannot run: the binary is missing,
the directory was never initialized, credentials or the backend cannot be
reached, or the plan fails for another reason. FinFocus stops, prints one
specific hint that says what to run, and prints no cost total. It does not
initialize the directory, apply, import, log in, or store a credential.

**Why this priority**: A raw tool trace, or a command that mutates
infrastructure while estimating, would make the feature unsafe to run.

**Independent Test**: Remove the binary, run in a directory that has `.tf`
files and no initialized working directory, and run with credentials that
the tool rejects. Each run fails with its own hint and leaves the project
unchanged except for what an ordinary plan's default refresh would already
do.

**Acceptance Scenarios**:

1. **Given** `tofu` is not on the path, **When** a projection command runs in a Terraform directory, **Then** it fails with an install hint and does not print a cost total.
2. **Given** the directory has not been initialized, **When** the plan cannot start, **Then** the hint tells the person to run init themselves, and FinFocus does not run init.
3. **Given** the tool reports that credentials or the backend cannot be used, **When** the plan fails for that reason, **Then** the hint says so and FinFocus does not log in, configure a backend, or store a credential.
4. **Given** the plan fails for some other reason, **When** FinFocus reports it, **Then** the message is the plan-failed hint. When the tool's text matches none of the three cases above, the hint includes the last lines of that text.
5. **Given** any of these failures, **When** the command exits, **Then** there is no partial cost result and no plan file left in the project directory.
6. **Given** a plan is running, **When** the person interrupts it, **Then** FinFocus does not keep waiting for a cost result.

---

### User Story 3 - Leave existing Pulumi projects alone (Priority: P1)

A person with a Pulumi project runs the same commands as today. FinFocus
still finds that project, still runs a Pulumi preview when it does today,
and still accepts the same input files. A Pulumi project wins over Terraform
when both markers are equally near. An existing state file flag still prices
Terraform state and does not start a plan.

**Why this priority**: The owner required that Pulumi behavior stay the same
for every existing Pulumi project.

**Independent Test**: Run `finfocus cost projected` in a Pulumi project that
has no `.tf` files in the current directory, and again with an explicit
Pulumi preview file. Compare the detection and the input rules with the
previous release. Then price a Terraform state file with the existing state
flag.

**Acceptance Scenarios**:

1. **Given** a Pulumi project file in the current directory, **When** the person runs a projection command with no input file, **Then** FinFocus uses Pulumi even if that directory also contains `.tf` or `.tf.json` files.
2. **Given** a Pulumi project file only in a parent directory, and `.tf` or `.tf.json` files in the current directory, **When** the person runs a projection command, **Then** FinFocus uses Terraform, because the Terraform marker is nearer.
3. **Given** a Pulumi project file in a parent directory and no `.tf` or `.tf.json` files in the current directory, **When** the person runs a projection command, **Then** FinFocus uses that Pulumi project, as it does today.
4. **Given** neither a Pulumi project nor Terraform files, **When** a command needs a project, **Then** the error names both Pulumi and Terraform.
5. **Given** the existing Terraform state flag and a state file, **When** the person runs a command that already accepts that flag, **Then** FinFocus prices that state and does not run a plan.
6. **Given** a Pulumi project and the same flags as today, **When** the person passes no Terraform input, **Then** required flags, defaults, and the preview path are unchanged.

---

### User Story 4 - Price a plan file the person already has (Priority: P2)

A person has already produced plan JSON. They pass that file and FinFocus
prices it without running Terraform or OpenTofu. The flag wins over
auto-detection, including inside a Pulumi project. It cannot be combined
with a Pulumi preview file, a Pulumi state file, or a Terraform state file.

**Why this priority**: Auto-detection covers the common case. A saved plan
is how someone reproduces a projection without touching a backend.

**Independent Test**: Pass a real plan JSON file and no other input flag.
FinFocus prints a projection and does not invoke `terraform` or `tofu`.

**Acceptance Scenarios**:

1. **Given** `--terraform-plan` points at plan JSON, **When** the command runs, **Then** FinFocus prices that file and does not run a plan.
2. **Given** the current directory is a Pulumi project, **When** `--terraform-plan` is set, **Then** the plan file is used and Pulumi detection does not run a preview.
3. **Given** `--terraform-plan` is combined with `--pulumi-json`, `--pulumi-state`, or `--terraform-state`, **When** the command starts, **Then** it fails as a user-input error and does not run a plan or a preview.
4. **Given** the plan file is missing or unreadable, **When** the command starts, **Then** it fails as a user-input error and does not search the directory for a project.

---

### User Story 5 - See plan actions and unknown values in the projection (Priority: P2)

A plan names resources that will be created, updated, deleted, replaced, or
left as they are. Some values are not known until apply. The projection
treats those actions the way a Pulumi plan's actions already affect the
cost diff. Unknown values are not presented as blank values the person
cleared on purpose. Read-only data sources are not priced.

**Why this priority**: A total with no create, delete, or replace is not
enough to judge a change. Confusing "unknown" with "absent" would send
plugins the wrong inputs.

**Independent Test**: Price a plan JSON file that contains one create, one
update, one delete, one replace, one unchanged resource, one data source,
and one unknown value. Compare the diff and the values sent for pricing
with a Pulumi plan that has the same actions.

**Acceptance Scenarios**:

1. **Given** a resource action of create, update, delete, or no-op, **When** the projection is built, **Then** that action reaches the cost diff as create, update, delete, or unchanged.
2. **Given** a resource the plan will delete and then create, **When** the projection is built, **Then** its operation is replace and the cost diff counts it as an update, exactly as it counts a Pulumi replace.
3. **Given** a read-only data source, **When** the projection is built, **Then** that data source is omitted.
4. **Given** a value the plan marks unknown until apply, **When** the resource is priced, **Then** that value is left out of the resource properties and its name is reported separately, so a plugin can tell unknown from absent.
5. **Given** an unknown name, **When** it is reported, **Then** it is a unique, case-sensitive, dot-separated path relative to the resource attributes, with no list index.
6. **Given** a property whose planned value is null, **When** the resource is priced, **Then** null is kept as a present value and is not reported as unknown.
7. **Given** a plugin result that does not complain about a missing price input, **When** FinFocus prints the projection, **Then** FinFocus does not add its own warning that a price input is missing.
8. **Given** a value the plan marks sensitive in the before or after state, **When** the resource is priced, **Then** that value is left out of both the properties and the old properties sent to plugins, and it is not reported as unknown.
9. **Given** a resource from a utility provider with no billing relation (`random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`), **When** the projection is built, **Then** the resource is not priced, contributes zero cost, and is reported once as skipped. The plan does not fail.
10. **Given** a resource from any other provider outside AWS, Azure, GCP, and Kubernetes (for example `cloudflare_*`), **When** the projection is built, **Then** the resource is kept with provider `custom` and sent to plugins, so a plugin that prices it can, and one that does not reports no cost data, as it does for Terraform state.

---

### User Story 6 - Use the same detection for every projection command (Priority: P2)

The person runs overview, a forecast, an estimate, or recommendations from
the same Terraform directory, with no input file. Each of those commands
plans once through the same detection rules as projected cost. Bare
`finfocus` opens the overview in that directory, the way it opens the
overview in a Pulumi project today. The overview does not try to skip the
plan by comparing file times.

**Why this priority**: A projection that works only for `cost projected`
would leave the rest of the product on the old "Pulumi or a state file"
rule.

**Independent Test**: In one Terraform directory, run overview, forecast,
estimate, and recommendations with no input file, and run bare `finfocus`.
Each one plans. Repeat the same commands in a Pulumi project and confirm
they still preview.

**Acceptance Scenarios**:

1. **Given** a Terraform directory, **When** the person runs `finfocus overview`, `finfocus cost forecast`, `finfocus cost estimate`, or `finfocus cost recommendations` with no input file, **Then** each command uses a Terraform plan.
2. **Given** a Terraform directory, **When** the person runs `finfocus` with no subcommand, **Then** FinFocus opens the overview and that overview uses a Terraform plan.
3. **Given** a Terraform directory, **When** the person runs `finfocus overview`, **Then** FinFocus runs the plan on every invocation in this version, including when source files are older than the last apply.
4. **Given** `finfocus cost recommendations dismiss` or `finfocus cost recommendations snooze` in a Terraform directory, **When** the person runs it, **Then** the command behaves as it does today: it never runs a plan, because it loads no project resources and uses `--pulumi-json` only to decide whether to open plugins.
5. **Given** a Pulumi project and no nearer Terraform files, **When** the person runs any command in this story, **Then** it still uses Pulumi.

---

### User Story 7 - Keep history under a stable project identity (Priority: P3)

Terraform has no project name, and a plan does not carry a backend,
workspace, or lineage. History for a Terraform directory is stored under an
identity derived from the git remote and the directory's path inside that
repository, plus a readable label. The workspace is not part of that
identity unless the person opts in. Two clones of the same repository record
the same identity for the same relative path. A directory that is not a git
checkout, or has no remote, still gets an identity, and the label says it is
tied to that machine's path.

**Why this priority**: Projected cost is useful before history is. History
commands need an identity that survives a different absolute path in CI.

**Independent Test**: Record a projection from two clones that share a
remote and the same relative path, then list history in each clone. The
records are the same project. Repeat with the workspace opt-in on and off.

**Acceptance Scenarios**:

1. **Given** a git checkout with a remote, **When** a Terraform projection records history, **Then** the identity is a hash of the normalized remote URL plus the path relative to the git root, and the stored label is that relative path.
2. **Given** the same remote and relative path in another clone, **When** history is listed, **Then** both clones use the same identity.
3. **Given** the workspace opt-in is off, **When** history is recorded, **Then** the identity and the label omit the workspace.
4. **Given** the workspace opt-in is on, **When** history is recorded, **Then** the workspace name is part of the identity and of the label.
5. **Given** no git repository, or a repository with no remote, **When** history is recorded, **Then** the identity is a hash of the absolute path and the label says that this identity is tied to that path.
6. **Given** a Terraform directory, **When** the person runs `finfocus cost history view`, `list`, `diff`, `export`, or `prune`, **Then** those commands resolve this identity.
7. **Given** a Terraform directory, **When** the person runs `finfocus cost history collect`, **Then** FinFocus does not invent past applies. It explains that checkpoint collection is for a Pulumi project.

### Edge Cases

- The current directory has both a Pulumi project file and Terraform files.
  Pulumi is used.
- A parent directory has the Pulumi project and the current directory has
  Terraform files. Terraform is used.
- Terraform files exist only in a subdirectory. The current directory is not
  a Terraform root, so those files are not selected.
- The plan JSON the person passes contains sensitive values. FinFocus prices
  it and does not write those values to logs. FinFocus does not move that
  file.
- The plan FinFocus itself runs contains sensitive values. The plan file is
  created outside the project, in a directory only the current user can
  read, is held for pricing only in memory, is never logged, and is deleted
  on success, failure, and cancellation.
- The binary is present but too old to emit the plan JSON this version
  reads. The result is the plan-failed hint, and the hint names the minimum
  version.
- The tool's error text matches none of the three specific cases. The
  plan-failed hint includes the last lines of that text.
- `finfocus cost actual` is run in a Terraform directory with no input file.
  FinFocus does not run a plan. It tells the person to pass a state file
  with the existing state flag.
- The person passes var-files, individual variables, a workspace name, or a
  subdirectory. This version has no such options. The plan runs in the
  current directory with the tool's own defaults.
- A plan fails halfway through pricing. No partial total is shown.
- A resource from a provider with no cloud billing relation (`random`,
  `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`) is skipped with one informational note on stderr that names
  the provider and count. The note never changes stdout or the exit code.
  Any other unrecognized provider is kept as `custom`, so adding a plugin that
  prices it needs no core change.
- A plan JSON larger than 32 MiB. A file the person passed fails as a
  user-input error that names the limit. A plan FinFocus ran itself uses the
  plan-failed hint, which names the limit.
- A plan with sensitive values that also names them unknown. The value is
  dropped once, and its path is not reported as unknown unless the plan marks
  it unknown.
- Detection finds neither Pulumi nor Terraform. The error names both.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: FinFocus MUST detect a Pulumi project by walking upward from the current directory for `Pulumi.yaml` or `Pulumi.yml`, as it does today.
- **FR-002**: FinFocus MUST detect a Terraform root only in the current directory, by the presence of `.tf` or `.tf.json` files.
- **FR-003**: When both markers are found, FinFocus MUST use the marker in the nearer directory. A tie in the same directory MUST use Pulumi.
- **FR-004**: When neither marker is found, the error MUST name both Pulumi and Terraform.
- **FR-005**: For a Terraform root, a projection command with no input file MUST run a plan in the current directory and price it. The plain case has no var-file, no individual variables, no workspace selection, and no subdirectory.
- **FR-006**: FinFocus MUST plan with `tofu` only (see FR-034) and MUST NOT fall back to `terraform`.
- **FR-007**: The plan run MUST NOT prompt for input, MUST NOT take the state lock, and MUST NOT color the tool's output. Refresh MUST stay the tool's default.
- **FR-008**: FinFocus MUST write its plan file outside the project, in a directory only the current user can read, MUST keep the JSON form only in memory, MUST NOT log the plan or sensitive plan values, and MUST delete the plan file on every exit, including failure and cancellation.
- **FR-009**: Cancelling the command MUST stop the plan process.
- **FR-010**: FinFocus MUST NOT run init, apply, or import, and MUST NOT store or supply credentials.
- **FR-011**: A plan that cannot run MUST fail with exactly one of these hints: binary missing, directory not initialized, credentials or backend access failed, or plan failed. Each hint MUST say what the person should run. Unrecognized tool text MUST be shown only as the last lines inside the plan-failed hint. A binary too old for the plan JSON MUST use the plan-failed hint and name the minimum version.
- **FR-012**: A failed plan MUST produce no partial cost result.
- **FR-013**: A plan with no creates, updates, deletes, or replaces, including one that reports only drift, MUST succeed, report zero projected cost, and print an informational message.
- **FR-014**: `--terraform-plan` MUST exist on every command in FR-020 except bare `finfocus`, MUST accept an existing plan JSON file, MUST skip running Terraform or OpenTofu, and MUST win over auto-detection. It MUST be mutually exclusive with `--pulumi-json`, `--pulumi-state`, and `--terraform-state`.
- **FR-015**: `--terraform-state` MUST keep its current meaning. FinFocus MUST NOT run a plan to satisfy it.
- **FR-016**: Resource actions MUST reach the cost diff as they do for a Pulumi plan: no-op as unchanged, create as create, update as update, delete as delete, and delete-and-create as replace. The cost diff counts replace as an update, as it does for Pulumi.
- **FR-017**: Read-only data sources MUST be omitted. Resources inside modules MUST be included. The resource address MUST be the resource's identity.
- **FR-018**: Values unknown until apply MUST be omitted from the priced properties and reported by name. Names MUST be unique, case-sensitive, dot-separated paths relative to the attributes, without list indexes. A null value MUST stay a present value. FinFocus MUST NOT add a missing-price-input warning of its own.
- **FR-019**: FinFocus MUST NOT keep its own table of Terraform type names. Type resolution stays with the pricing plugins, as it does for Terraform state today.
- **FR-020**: The commands that gain an automatic Terraform plan are `finfocus` with no subcommand, `finfocus overview`, `finfocus cost projected`, `finfocus cost forecast`, `finfocus cost estimate`, and `finfocus cost recommendations`. They MUST share one detection and plan resolution with each other. `finfocus cost recommendations dismiss` and `snooze` load no project resources and MUST NOT run a plan.
- **FR-021**: In this version, overview MUST treat every Terraform run as needing a plan. It MUST NOT skip the plan by comparing source file times with a last deployment time.
- **FR-022**: In a Pulumi project with no nearer Terraform root, detection, preview, required flags, and defaults MUST stay as they are today.
- **FR-023**: `finfocus cost actual` in a Terraform directory MUST NOT run a plan. With no input file it MUST tell the person to pass a state file with `--terraform-state`.
- **FR-024**: Terraform history identity MUST default to a hash of the normalized git remote URL plus the path relative to the git root, with a readable label of that relative path. It MUST NOT be a hash of the absolute path when a remote exists.
- **FR-025**: The Terraform workspace MUST be omitted from the history identity and the label unless the person opts in. Opt-in MUST add the current workspace name to both. The default MUST be off.
- **FR-026**: With no git repository or no remote, the identity MUST be a hash of the absolute path and the label MUST say that the identity is tied to that path.
- **FR-027**: `finfocus cost history view`, `list`, `diff`, `export`, and `prune` MUST resolve the Terraform identity in a Terraform directory. Snapshots recorded from a Terraform projection MUST use that identity.
- **FR-028**: `finfocus cost history collect` MUST NOT invent Terraform apply history. In a Terraform directory it MUST explain that checkpoint collection is for a Pulumi project.
- **FR-030**: Sensitive values MUST NOT reach a plugin. Every leaf the plan marks sensitive in the before or after state MUST be removed from the priced properties and the old properties. A removed sensitive leaf is not an unknown property. This sits beside, and does not replace, the existing credential-name rule.
- **FR-031**: A resource whose provider is `random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, or `http` MUST be skipped, contribute zero cost, and be reported once as an informational note. A resource from any other provider outside AWS, Azure, GCP, and Kubernetes MUST be kept with provider `custom`. FinFocus MUST NOT keep a table of resource types for this; the skipped set is these nine provider names only.
- **FR-032**: A `--terraform-plan` file larger than 32 MiB MUST fail as a user-input error that names the limit.
- **FR-033**: A plan document whose `format_version` major is not 1 MUST fail. For `--terraform-plan` it is a user-input error. For a plan FinFocus ran itself it uses the plan-failed hint.
- **FR-029**: User-facing help and command docs for the commands in FR-020 and FR-027 MUST describe Terraform plan mode and the unchanged state flag.

- **FR-034**: Automatic planning, version checks, and workspace lookup MUST
  invoke only OpenTofu (`tofu` >= 1.6.0). FinFocus MUST NOT bundle, import, or
  execute Terraform core. Terraform-compatible input formats remain supported.
- **FR-035**: `finfocus --web` in a detected Terraform project MUST load and
  price the OpenTofu plan through the existing overview pipeline and support
  the existing web query surfaces without invoking Pulumi.

### Key Entities

- **Project marker**: The signal that selects a mode. A Pulumi project file found by walking upward, or `.tf` / `.tf.json` files in the current directory. The nearer marker wins. The same directory is a tie and uses Pulumi.
- **Plan**: The proposed changes for the current directory. Either produced by the plan run or read from a plan JSON file the person supplied.
- **Planned resource**: One managed resource in that plan. Its identity is its address. It has an action (create, update, delete, replace, or unchanged) and properties, some of which may be unknown until apply.
- **Unknown property**: The name of a value the plan could not know yet. Reported separately from properties that are present, including properties whose value is null.
- **Failure hint**: One of four messages: binary missing, not initialized, credentials or backend access failed, or plan failed. The plan-failed hint covers an old binary and any unrecognized tool text.
- **Project identity**: The history key for a Terraform directory. A hash plus a readable label. By default the hash is the git remote and the relative path. The workspace is included only when opted in. Without a remote, the hash is the absolute path and the label says so.

## Web UI Fit *(mandatory)*

- **Disposition**: Required. The owner extended this feature to detect
  Terraform-compatible projects in `finfocus --web` and use OpenTofu only.
- **Layout fit**: Extend the existing overview. Keep its heading, controls,
  loading status, summary strip, and captioned resource table. No new page.
- **API hook**: Existing overview, actual-cost, recommendations, and estimate
  endpoints consume the same validated descriptors and Go engine methods as
  the CLI. The method and endpoint mapping is in `contracts/http-api.md`.
- **Render evidence**: Run a fresh OpenTofu plan with priced resources and
  inspect the populated browser dashboard, resource identities, totals,
  errors, and browser console. Record the result in `validation.md`.
- **Follow-up issue scope**: File one issue labeled `web-ui` for layout,
  interaction, copy, and accessibility improvements only. Engine, API,
  redaction, and data-shape fixes remain part of this implementation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In an initialized Terraform directory, a person gets a projected cost from `finfocus cost projected` with no input file and no hand-written plan file.
- **SC-002**: For every existing Pulumi project fixture, detection and the preview path stay on Pulumi, including a project file that sits next to `.tf` files.
- **SC-003**: Each of the four failure situations (missing binary, not initialized, credentials or backend rejected, and any other plan failure) ends in a non-zero exit, one hint, and no cost total.
- **SC-004**: Across those failures, cancellation, and success, the project directory contains no plan file created by FinFocus, and logs contain no plan JSON.
- **SC-005**: A plan with no creates, updates, deletes, or replaces exits successfully and reports zero projected cost.
- **SC-006**: On a plan JSON fixture that contains create, update, delete, replace, unchanged, a data source, a sensitive leaf, and an unknown value, the cost diff shows creates, updates (including the replace), and deletes, omits the data source and the sensitive leaf, and reports the unknown name separately from absent properties.
- **SC-010**: On a fixture with `random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`, and `cloudflare` resources beside an AWS one, the first nine are skipped with one note, the `cloudflare` resource reaches plugins with provider `custom`, and the AWS resource is priced.
- **SC-007**: Overview, forecast, estimate, recommendations, and bare `finfocus` each produce a Terraform projection from the same directory with no input file.
- **SC-008**: Two clones with the same remote and the same relative path record history under one identity. Turning the workspace opt-in on changes that identity, and turning it off leaves the workspace out.
- **SC-009**: A command that prices `--terraform-state` still prices that state file and does not run a plan.

## Assumptions

- `--terraform-plan` is the flag name. It pairs with the existing
  `--terraform-state` flag, and no flag of that name exists today. Planning
  may rename it only if the name collides with a flag already on those
  commands.
- The minimum tool version is the oldest `terraform` and `tofu` release
  whose `show -json` output this feature accepts. Planning recorded it in
  `contracts/plan-json.md` (OpenTofu 1.6.0). An older
  binary uses the plan-failed hint.
- The workspace opt-in is one saved setting, default off, and one command
  flag can turn it on for a single run without changing a Pulumi stack
  flag. Planning names the setting and the flag. An override name that
  keeps history across a directory rename is a later feature.
- `cost actual` reads state, not a plan. Automatic Terraform support for
  that command stays the existing state flag plus a hint, rather than a
  plan run dressed up as measured spend.
- `cost history collect` reads Pulumi checkpoints. A plan has no checkpoint
  list, and this version does not talk to a remote backend to invent one.
- Dismiss and snooze use `--pulumi-json` only to decide whether to open
  plugins; they load no resources from it. They stay unchanged, as do
  commands that only read local dismissal records.
- Terraform plans carry no cross-resource references in the form Pulumi's
  `propertyDependencies` do, so Terraform rows get no `ref.*` tags in this
  version.
- Analyzer installation and Pulumi ESC credential loading stay Pulumi-only.
  They are not projection commands.
- Pricing plugins already accept Terraform types for state. This feature
  sends planned resources through that same resolution and does not add a
  type map of its own.
- Examples and acceptance fixtures come from real `terraform show -json`
  output. Hand-written JSON with invented property names does not count.
- Var-files, `-var`, workspace selection, a subdirectory target,
  Terragrunt, remote-backend setup, and reading HCL without running the
  tool are out of scope. So is serving this ingestion through a plugin
  parse API. FinFocus reads Terraform and Pulumi itself.

## Out of Scope

- Running init, apply, or import, and storing or typing credentials.
- Changing Pulumi detection, preview, or defaults for a directory whose
  nearest marker is a Pulumi project.
- A name the person can set to keep history after a directory rename.
- Importing past Terraform applies into `cost history collect`.
- Cross-resource reference tags (`ref.*`) for Terraform rows.
- A Terraform plan mode for `cost recommendations dismiss` and `snooze`.
- Skipping providers beyond those nine. Other utility
  providers can be added later without a core mapping table by sending them as
  `custom`.
