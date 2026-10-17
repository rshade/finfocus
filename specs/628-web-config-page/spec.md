# Feature Specification: Web Config Page

**Feature Branch**: `628-web-config-page`
**Created**: 2026-10-16
**Status**: Implemented
**Input**: GitHub issue #1763. Add a Config page to the `finfocus --web`
dashboard that shows the effective configuration, where each value comes from,
and lets the user edit a fixed set of known settings in the project tier or
the global tier. Secret values never reach the browser.

## Relationship to the Web Dashboard

This feature amends the web dashboard specification (`specs/627-web-ui/`).
That specification lists configuration editing and budget definition as out
of scope and says the dashboard adds no persistent state. Those clauses no
longer cover this page:

- Viewing and editing the known settings in this specification are in scope.
- The only new persistent state is a write to the resolved project
  configuration file or the resolved global configuration file.
- The dashboard stays read-only toward infrastructure. It still does not
  install plugins, export reports, or change cloud resources.
- Budget amount, currency, and period for the global budget are editable.
  Budget alert destinations stay read-only.

The amended text in specification 627 must point here so the two documents
do not contradict each other.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See which configuration is in effect (Priority: P1)

A person running the web dashboard opens Config and sees every known setting,
the value that is actually in effect, and where that value came from: the
built-in default, the global file, the project file, or the environment.
Each row also shows the value stored in each tier, so the person can tell
why the effective value won.

**Why this priority**: The page has no value if the person still cannot tell
which file or environment variable is winning. Viewing is useful before any
edit exists.

**Independent Test**: Launch the dashboard with a known global file, a known
project file, and one environment override. Open Config and compare each
row's effective value and source with the value the command line would use
for the same project.

**Acceptance Scenarios**:

1. **Given** a global file sets output precision to 2 and the project file
   sets it to 4, **When** the person opens Config, **Then** the effective
   value is 4 and the source is the project tier, and both stored values are
   visible.
2. **Given** an environment override sets the logging level, **When** the
   person opens Config, **Then** the effective value is the environment value
   and the source is the environment, even when a file stores a different
   level.
3. **Given** a setting is absent from both files and the environment,
   **When** the person opens Config, **Then** the row shows the built-in
   default and the source is the default.
4. **Given** the launch used an explicit project directory, or
   `FINFOCUS_PROJECT_DIR`, or the nearest Pulumi project, **When** the person
   opens Config, **Then** the project tier is that same directory's
   configuration.
5. **Given** no project directory resolves, **When** the person opens Config,
   **Then** the project tier is marked unavailable and cannot be selected
   for editing.

---

### User Story 2 - Change a known setting in a chosen tier (Priority: P1)

The person picks Project or Global, changes one or more known settings, and
reviews a before/after preview. Confirming the preview writes only the
chosen tier's resolved configuration file. Cancelling writes nothing. Invalid
input is rejected with a message that names the setting, and the file is
left unchanged.

**Why this priority**: Editing is the reason the page exists. A preview that
writes nothing is part of the same journey, because the person must see the
diff before the file changes.

**Independent Test**: Change output precision in the project tier, confirm
the preview, and read the project file. Repeat with an out-of-range precision
and confirm the file bytes are unchanged.

**Acceptance Scenarios**:

1. **Given** a project file and the Project tier selected, **When** the
   person sets output precision to 3 and confirms the preview, **Then** only
   the project configuration file changes and a later command-line read of
   that project reports precision 3.
2. **Given** the Global tier selected, **When** the person changes the same
   setting and confirms, **Then** only the global configuration file changes.
3. **Given** the person has not confirmed, **When** they view the preview,
   **Then** they see the before and after value for each changed setting and
   neither file changes.
4. **Given** precision 11, **When** the person tries to save, **Then** the
   page names the invalid setting, and the target file is byte-for-byte
   unchanged.
5. **Given** a successful save, **When** the page confirms it, **Then** it
   states that the running dashboard session keeps its startup configuration
   and that the new values apply to the next dashboard launch and to
   command-line runs.
6. **Given** a project file that contains comments, a key the page does not
   edit, and a custom key order, **When** the person changes one known
   setting, **Then** the comments, the unknown key, and the relative order of
   untouched keys remain, and settings that were not already in the file are
   not added.

---

### User Story 3 - Replace a secret without revealing it (Priority: P1)

Notification destinations and plugin credential settings are visible only as
set or unset. The person can replace a secret. The page never shows the
stored secret, never fills the replacement field with it, and never writes
the masked placeholder back to disk.

**Why this priority**: A config page that displayed webhook URLs, authorization
headers, or plugin credentials would be a new leak. Write-only replacement
is required for the page to be safe to open.

**Independent Test**: Store a plugin credential and a notification URL, open
Config, and confirm the response and the rendered page contain neither value.
Replace the credential, confirm the file contains the new value, then save
again without entering a replacement and confirm the new value is still
there.

**Acceptance Scenarios**:

1. **Given** a notification URL and a plugin credential are stored, **When**
   the person opens Config, **Then** each is shown as set, and neither the
   page nor the data behind it contains the URL, the header value, or the
   credential.
2. **Given** a secret is unset, **When** the person opens Config, **Then**
   it is shown as not set.
3. **Given** the person enters a new credential and confirms, **When** the
   save completes, **Then** the file contains the new value and the page
   still shows only that the secret is set.
4. **Given** a secret is set, **When** the person saves without entering a
   replacement, **Then** the stored secret is unchanged.
5. **Given** the person submits the masked placeholder text, **When** the
   save completes, **Then** that placeholder is not written and the stored
   secret is unchanged.
6. **Given** a save of any setting, **When** the dashboard writes its audit
   and request records, **Then** those records name the setting and the tier
   and do not contain the submitted value.

---

### User Story 4 - Survive concurrent and stale edits (Priority: P2)

Two browser tabs, or a tab and a command-line change, cannot silently
overwrite each other. A save based on an outdated view is rejected and the
person is asked to reload. A crash during the write does not leave a
truncated configuration file.

**Why this priority**: The files are also edited by `config set` and by hand.
Silent overwrite would discard a budget or a credential. The view is still
useful without this story, so it follows the primary journeys.

**Independent Test**: Load Config, change the same file from another writer,
then save from the page. Confirm the save is rejected and the other writer's
bytes remain. Repeat with two overlapping saves and confirm both values are
intact.

**Acceptance Scenarios**:

1. **Given** the file changed after the page loaded, **When** the person
   saves, **Then** the save is rejected, the page explains the conflict, and
   the file keeps the newer contents.
2. **Given** two saves of different settings arrive together, **When** both
   complete, **Then** each setting has the value its save requested and the
   file is valid configuration.
3. **Given** the write is interrupted, **When** the file is read, **Then** it
   is either the previous complete file or the new complete file.

---

### User Story 5 - Command line matches the page (Priority: P2)

`config get` and `config list` hide plugin credential values the same way
the page does. `config set` writes the global file through the same safe
update used by the page: it does not persist environment-only values, it
does not drop comments, and a flat budget amount is stored on the global
budget. A legacy YAML configuration file in the chosen tier is converted to
the current configuration format before it is patched.

**Why this priority**: Divergent masking or a command-line save that still
writes environment secrets would undo the page's guarantees. The page can
ship its own view first, so this story is second.

**Independent Test**: Set a plugin credential, run `config get` and
`config list`, and confirm the credential value is absent. Set an output
value with an environment override of a different setting present, and
confirm the environment value was not written. Repeat against a legacy YAML
file and confirm the result is the current format and still contains the
original comments that the format can carry.

**Acceptance Scenarios**:

1. **Given** a plugin credential in the global file, **When** the person runs
   `config get` or `config list`, **Then** the output shows that the
   credential is set and does not show its value.
2. **Given** an environment override for a setting that is not being changed,
   **When** the person runs `config set` for a different setting, **Then**
   the environment value is not written into the file.
3. **Given** a global file with comments, **When** the person runs
   `config set` for one known setting, **Then** the comments and untouched
   keys remain.
4. **Given** the person sets a budget amount with the existing flat budget
   command, **When** the file is read back, **Then** the amount is stored on
   the global budget, which is the field later commands honor.
5. **Given** the target tier still has a legacy YAML file, **When** the
   person saves from the page or runs `config set`, **Then** the tier is
   converted to the current format and the requested change is applied to
   that file.

### Edge Cases

- A write request that does not come from the dashboard page itself, or that
  is not a configuration document, is rejected and writes nothing.
- The person cannot name a file path. The only writable locations are the
  two files the dashboard already resolved.
- Clearing a setting in a tier removes that setting from that tier's file so
  a lower tier or the default can apply. It does not delete the other tier's
  value. An empty overview cluster context removes that cluster's mapping.
- Submitting no changes previews an empty diff and writes nothing.
- A project file that contains an environment-variable reference inside a
  notification destination is still rejected, because project files are
  committed. The rejection names the setting and leaves the file unchanged.
- Unknown keys and settings outside the editable set are shown when they
  already exist, with secrets masked, and have no edit control.
- The global budget is edited at its scoped fields (amount, currency, and
  period). The flat budget fields that commands warn about and ignore are
  not offered as separate settings.
- Plugin install, a dedicated per-plugin settings form, a raw configuration
  text editor, and exposing configuration get or set to the agent tool
  surface are out of scope. A known plugin setting can still be edited as
  one of the known keys.
- Routing, analyzer settings, and budget alert destinations are read-only on
  this page.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The dashboard has a Config page that lists every known setting.
  Each row shows the effective value, the source tier (default, global,
  project, or environment), and the value stored in each tier.
- **FR-002**: The project tier uses the same project resolution as the rest
  of the program: an explicit project directory, then `FINFOCUS_PROJECT_DIR`,
  then the nearest Pulumi project. When none resolves, the project tier is
  unavailable and cannot be written.
- **FR-003**: The person can choose the project tier or the global tier. A
  save writes only that tier's resolved configuration file. The request
  never carries a file path, and no other path can be written.
- **FR-004**: The editable settings are exactly those the command line can
  set today: output format and precision; logging level and log file; plugin
  host strict compatibility; the global budget amount, currency, and period;
  scoring enablement, plugin, identifier mode, timeout, field allowlist, and
  needs-review thresholds; overview cluster-context mappings; and individual
  plugin settings (`plugins.<name>.<key>`). Each control accepts only values
  that the existing validation accepts for that setting.
- **FR-005**: Budget alert destinations, routing, and analyzer settings are
  shown read-only. Secrets among them are masked. A later feature may add
  editors; this feature does not.
- **FR-006**: The page offers a preview of the before/after diff for the
  pending edits. Preview writes nothing. Confirming the preview performs
  the save.
- **FR-007**: A save is validated with the same rules as `config validate`
  for that tier before the file is replaced. Invalid input returns an error
  that identifies the setting and leaves the file unchanged.
- **FR-008**: A save changes only the edited settings in the target file. It
  preserves comments, key order, and unknown keys, and it does not add
  built-in defaults that were not already present.
- **FR-009**: Values that come from the environment are never written to
  either configuration file, whether the save comes from the page or from
  `config set`.
- **FR-010**: Notification URLs, notification headers, and plugin settings
  whose names are credentials never reach the browser. They are shown as set
  or unset. The same masking applies to `config get` and `config list`.
- **FR-011**: Replacing a secret writes the new value. Omitting the secret,
  or submitting the masked placeholder, leaves the stored value unchanged.
  The masked placeholder is never written.
- **FR-012**: Each loaded view has a version. A save whose version no longer
  matches the file is rejected with a conflict message and a prompt to
  reload. Overlapping saves of the same file are applied one at a time. An
  interrupted write leaves a complete previous file or a complete new file.
- **FR-013**: A write that does not come from the dashboard page, or that is
  not a configuration document, is rejected. These checks are the same ones
  the dashboard already applies to other state-changing requests.
- **FR-014**: Audit and request records for a configuration save contain the
  setting names and the tier only. They do not contain values.
- **FR-015**: A legacy YAML file in the target tier is migrated to the
  current configuration format before it is patched, using the program's
  existing migration.
- **FR-016**: `config set` uses the same safe file update as the page. It
  still writes the global file only. The existing flat budget commands store
  the amount, currency, and period on the global budget.
- **FR-017**: After a successful save the page states that the running
  dashboard session does not reload configuration. The new values apply to
  the next dashboard launch and to command-line runs.
- **FR-018**: The web dashboard guides (architecture, reference, and how-to)
  describe the Config page, the tier choice, the secret behavior, and the
  next-launch limitation. They no longer say that configuration editing is
  unavailable. The configuration reference no longer tells the reader that
  the only file is YAML.
- **FR-019**: Specification 627 is updated so its persistence rule and its
  out-of-scope list match this feature: configuration writes to the two
  resolved configuration files are allowed, and configuration viewing and
  editing of the known settings are no longer excluded.
- **FR-020**: The dashboard remains read-only toward infrastructure. This
  feature adds no other persistent state.

### Key Entities

- **Known setting**: A named configuration item the person can view. It has
  a type, the values it accepts, a secret flag, and short help. The editable
  set is the list in FR-004.
- **Tier value**: The value of a setting as stored in one tier (default,
  global, project, or environment), distinct from the effective value.
- **Effective configuration**: The value that wins for each setting after
  the same shallow merge the program already uses, with environment
  overrides applied on top.
- **Config view**: The list of known settings, their tier values, their
  effective values, and whether the project tier is available. It carries a
  version used to detect a stale save.
- **Pending edit**: A chosen tier plus one or more setting changes that can
  be previewed and then confirmed. Secret changes are either a replacement
  value or "leave unchanged".

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a setup with a global file, a project file, and one
  environment override, a person can identify the winning value and its
  source for every known setting without opening either file.
- **SC-002**: A person can change one known setting in a chosen tier,
  including the preview, in under two minutes, and the other tier's file is
  unchanged.
- **SC-003**: In 100% of automated cases, no secret value (notification URL,
  notification header, or plugin credential) appears in the page or in the
  data the page loads. Command-line get and list match that masking.
- **SC-004**: In 100% of automated cases, an invalid save or a stale save
  leaves the target file byte-for-byte unchanged, and a successful save of
  one setting preserves comments and keys the person did not edit.
- **SC-005**: Two overlapping saves of different settings both persist, and
  an interrupted write never leaves a partial configuration file, across the
  automated cases for those races.
- **SC-006**: The published web dashboard architecture, reference, and
  how-to pages, and the configuration reference, describe this page and do
  not tell the reader that configuration editing is unavailable or that YAML
  is the current file format.

## Assumptions

- The running dashboard session loads configuration once at startup. Reloading
  it live is a separate feature.
- Command-line `config set` continues to target the global file. Choosing the
  project tier is a dashboard action. No new command-line flag is added.
- "The same way the command line can set today" includes the flat budget
  aliases, which write the global budget's scoped fields. The page shows the
  scoped fields, not the ignored flat fields.
- Clearing a tier value removes that key from that tier. It is not a write of
  an empty string unless an empty string is a meaningful stored value.
- Read-only display of alert destinations, routing, and analyzer settings uses
  the same secret masking as the editable secrets. It does not add editors.
- A dedicated plugin-install or per-plugin settings form is a separate page
  that will reuse this feature's tier choice and write-only secret handling.
  This feature still edits a single `plugins.<name>.<key>` setting.
- Specification 627's other rules still apply: loopback only, per-session
  token, and no exposure beyond the local machine.
- The configuration schema and the known-setting list must stay in agreement.
  A test guards that agreement; the person-facing behavior is that the page
  does not offer a setting the program rejects, and does not omit a setting
  the command line can set.
