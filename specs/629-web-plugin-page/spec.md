# Feature Specification: Web Plugin Page

**Feature Branch**: `629-web-plugin-page`
**Created**: 2026-10-17
**Status**: Draft
**Input**: GitHub issue #1765. Add a Plugins page to the `finfocus --web`
dashboard. The person browses the built-in plugin catalog, installs, updates,
removes, and validates catalog plugins, and fills in each plugin's settings
from a form the plugin describes. Long operations show progress. Settings
take effect the next time that plugin is launched.

## Relationship to the Web Dashboard

This feature amends the web dashboard specification (`specs/627-web-ui/`) and
the config page specification (`specs/628-web-config-page/`).

Specification 627 lists plugin management as out of scope and says the
dashboard adds no persistent state and stays read-only toward infrastructure.
Those clauses no longer cover this page:

- Browsing the catalog and installing, updating, removing, and validating
  catalog plugins are in scope.
- Editing a plugin's settings is in scope.
- The new persistent state is plugin files in the same directory `plugin
  list` and discovery already read, plus plugin settings written through the
  config page's existing project or global configuration file.
- The dashboard stays read-only toward cloud infrastructure. It still does
  not run `pulumi up`, export reports, or change cloud resources.
- Install, update, and remove are not exposed as agent tools.

Specification 628 keeps the tier choice, comment-preserving writes, conflict
checks, and write-only secrets. This page is the dedicated per-plugin
settings form that specification 628 left for a later feature. A plugin
setting is no longer edited only as a single known key on the Config page.
The Config page may still show those keys. This page is where the person
fills in a whole plugin.

The amended text in specifications 627 and 628 must point here so the three
documents do not contradict each other.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See which plugins exist and which are installed (Priority: P1)

A person setting up cost sources opens Plugins and sees every entry in the
built-in catalog. Each entry shows its name, what it is for, which providers
and capabilities it offers, its security level, the installed version if one
is present, and the latest version the catalog can install. An installed
plugin that is not the latest is marked update-available.

**Why this priority**: Without this list the person still has to run
`plugin list --available` and read each plugin's files to learn what can be
installed.

**Independent Test**: With one catalog plugin installed at an older version
and the others absent, open Plugins and compare each card with
`finfocus plugin list` and `finfocus plugin list --available`.

**Acceptance Scenarios**:

1. **Given** a fresh install with no plugins on disk, **When** the person
   opens Plugins, **Then** every built-in catalog entry is listed, none is
   marked installed, and each shows the latest version and its security
   level.
2. **Given** one catalog plugin is installed, **When** the person opens
   Plugins, **Then** that entry shows the installed version and the latest
   version, and the other entries stay not installed.
3. **Given** the installed version is older than the latest catalog version,
   **When** the person opens Plugins, **Then** that entry is marked
   update-available.
4. **Given** the installed version is the latest, **When** the person opens
   Plugins, **Then** that entry is marked installed and is not marked
   update-available.
5. **Given** a plugin was installed from outside the catalog, **When** the
   person opens Plugins, **Then** that plugin is listed as outside the
   catalog, and install and update are not offered for it. Validate,
   settings, and remove remain available.

---

### User Story 2 - Install, update, or remove a catalog plugin (Priority: P1)

The person chooses install, update, or remove. The page names the plugin,
the version, the source repository, and the security level, and waits for an
explicit confirmation. After confirmation, progress appears while the work
runs: resolving the release, downloading (with bytes received and the total
when the total is known), verifying, extracting, validating, and done.
Warnings appear as they happen. The page then says the operation succeeded
or gives a specific failure. The files land in the directory that
`finfocus plugin list` and later dashboard queries read, including for
catalog plugins whose releases use a monorepo tag prefix.

**Why this priority**: Installing from the page is the reason the catalog
exists. A confirmation that names the binary's origin is part of the same
journey, because the page is about to download and run a program.

**Independent Test**: Confirm an install of a catalog plugin from a stand-in
catalog, then run `finfocus plugin list` and see that version. Repeat for
update and remove.

**Acceptance Scenarios**:

1. **Given** a catalog plugin that is not installed, **When** the person
   confirms install of a specific version, **Then** that version is installed
   where `plugin list` finds it, and the page reports success.
2. **Given** an older installed version and a newer catalog version,
   **When** the person confirms update, **Then** the newer version is
   installed and the page reports success.
3. **Given** an installed version, **When** the person confirms removal of
   that version, **Then** that version's files are gone and `plugin list` no
   longer shows it.
4. **Given** the confirmation summary, **When** the person has not confirmed,
   **Then** no download starts and no plugin file changes.
5. **Given** a monorepo catalog plugin (one whose releases are tagged with a
   prefix such as `kubernetes-` or `jev-`), **When** the person confirms
   install or update, **Then** the files land in the same place as
   `finfocus plugin install` for that name.
6. **Given** an install, update, or remove in progress for a plugin,
   **When** a second operation is requested for that same plugin, **Then**
   the second request fails with a clear busy message and does not change
   files.
7. **Given** the command-line `plugin install` and `plugin update` output
   from before this feature, **When** the same commands are run after it,
   **Then** their person-facing output is unchanged.

---

### User Story 3 - Trust the download (Priority: P1)

The page only installs names that are in the built-in catalog. It refuses a
repository slug or a URL. It never skips checksum verification. A checksum
mismatch fails the operation. When the release has no checksum file, the
page warns and requires a second confirmation before it continues. When the
requested release has no downloadable files, the page shows the older version
it would use instead and waits for confirmation. It never switches version
silently. When the catalog host rate-limits the request, the page says so
and tells the person they can set a token in the environment the dashboard
was launched from.

**Why this priority**: The page downloads and unpacks executable plugins.
The command line already refuses a bad checksum and already asks before a
version fallback. The page must not be a weaker path.

**Independent Test**: Attempt a URL install, a checksum mismatch, a release
with no checksum file, and a release with no files. Only the confirmed,
verified catalog install changes the plugin directory.

**Acceptance Scenarios**:

1. **Given** a name that is not in the built-in catalog, or a repository
   slug, or a URL, **When** the person tries to install it, **Then** the
   page refuses and no file is downloaded.
2. **Given** a catalog release whose checksum does not match the archive,
   **When** the person confirms install, **Then** the operation fails, the
   failure says the checksum did not match, and the plugin is not installed.
3. **Given** a catalog release with no checksum file, **When** the person
   has confirmed install once, **Then** the page warns and does not continue
   until the person confirms a second time that they accept the missing
   checksum file.
4. **Given** the requested release has no downloadable files and an older
   release does, **When** the person confirmed only the original version,
   **Then** nothing is installed until they confirm the fallback version the
   page names.
5. **Given** the catalog host rate-limits the lookup, **When** the operation
   fails, **Then** the message says the request was rate-limited and that a
   token in the dashboard's environment can raise the limit. The message
   does not include the token.

---

### User Story 4 - Check that an installed plugin starts (Priority: P2)

The person runs Validate on an installed plugin. The page checks the
installed files, then starts the plugin and reads the description the plugin
reports about itself. The result is pass or fail with the reason. Validate
does not run the long conformance or certification suites.

**Why this priority**: Install can succeed and the plugin can still fail to
start. The person needs that answer on the same page. It is second to
install because there is nothing to validate until a plugin is present.

**Independent Test**: Validate a plugin whose files are intact and one whose
files are missing. The first passes. The second fails and names the reason.

**Acceptance Scenarios**:

1. **Given** a healthy installed plugin, **When** the person validates it,
   **Then** the page reports pass.
2. **Given** an installed plugin whose required files are missing or whose
   process fails to start, **When** the person validates it, **Then** the
   page reports fail and includes the reason.
3. **Given** the person is looking at plugin actions, **When** they read the
   page, **Then** conformance, certification, plugin scaffolding, and plugin
   upgrade are not offered.

---

### User Story 5 - Fill in a plugin's settings (Priority: P1)

After a plugin is installed, the person opens its settings. When the plugin
describes its settings, the form follows that description: text, whole
numbers, yes/no, a fixed list of choices, a web address, or a duration.
Required settings are enforced. Each setting shows the description the plugin
supplied. Secrets are write-only: the page shows set or unset, the
replacement field starts empty, and the stored value never comes back to the
browser. When the plugin does not describe its settings, the page offers a
generic list of name and value pairs for that plugin. Names that are
credentials are treated as secrets.

The person chooses the project tier or the global tier, reviews the change,
and confirms. The write uses the same rules as the Config page: only the
chosen tier's configuration file changes, comments and unrelated keys stay,
a stale view is rejected, and a secret left blank is not overwritten. The
page says the new settings apply the next time that plugin is launched, not
to a plugin process that is already running.

**Why this priority**: A plugin that is installed and not configured still
does not produce cost data. The form removes the need to read each plugin's
documentation to learn which variables it reads.

**Independent Test**: Install a plugin that describes a secret and a required
address. Save the address in the project tier and replace the secret. Read
the project file and confirm the address is stored, the secret is stored, and
the page's loaded data does not contain the secret. Repeat with a plugin
that describes nothing and a credential-shaped name.

**Acceptance Scenarios**:

1. **Given** a plugin that describes a required web address and a secret,
   **When** the person opens settings, **Then** the address is a web-address
   field, the secret shows set or unset, and a save without the required
   address is rejected.
2. **Given** a plugin that describes a fixed list of choices, **When** the
   person opens settings, **Then** those choices are the only values the
   field accepts.
3. **Given** a stored secret, **When** the person loads settings or replaces
   a different field, **Then** the secret's value is not in the page and is
   not written back over itself. Submitting a new secret replaces it.
4. **Given** a plugin that describes no settings, and a stored value whose
   name is a credential, **When** the person opens settings, **Then** they
   can edit name and value pairs for that plugin, and the credential is
   write-only like a described secret.
5. **Given** the project tier is selected, **When** the person confirms a
   settings save, **Then** only the project configuration file changes.
   Choosing global changes only the global file. Comments and keys that were
   not edited remain.
6. **Given** a successful save, **When** the page confirms it, **Then** it
   states that the settings apply the next time that plugin is launched.

---

### User Story 6 - Saved settings reach the plugin (Priority: P1)

The next time FinFocus launches that plugin, the saved settings are present
for the plugin to read, as if the person had exported them in the shell.
Precedence is: a variable already set in the environment that launched
FinFocus, then the project configuration, then the global configuration, then
the default the plugin declared. A shell export still wins, so an existing
setup keeps working. A setting reaches only the plugin that owns it. A
secret is never written to logs, errors, or another plugin's environment.
A cost answer computed before the setting changed is not reused after the
change.

The plugins shipped with FinFocus that already read settings (the
recommendation scorer, the Prometheus usage source, and the Kubernetes usage
source) describe those settings, so their forms are typed rather than
generic.

**Why this priority**: A form that saves values the plugin never sees does
not change the setup journey. Delivery is what makes the form true.

**Independent Test**: Save a stand-in plugin's mock response through the
form, run a dashboard query, and observe that the plugin received that
value. Export a different value in the parent environment and observe that
the export wins. Confirm a second plugin's environment does not contain the
first plugin's secret.

**Acceptance Scenarios**:

1. **Given** a project setting for one plugin and no matching environment
   variable, **When** FinFocus next launches that plugin, **Then** the plugin
   receives the project value.
2. **Given** the same setting in the project file and the global file,
   **When** the plugin launches, **Then** the project value wins.
3. **Given** the parent environment already sets that plugin's variable,
   **When** the plugin launches, **Then** the environment value wins over
   the project file, the global file, and the plugin's declared default.
4. **Given** no environment variable and no stored value, and the plugin
   declares a default, **When** the plugin launches, **Then** the plugin
   receives the default.
5. **Given** a secret saved for plugin A, **When** plugin B launches,
   **Then** plugin B's environment does not contain plugin A's secret, and
   the secret does not appear in logs or error text.
6. **Given** a cost result was saved while a plugin setting had one value,
   **When** the person changes that setting and runs the same cost request,
   **Then** the result is computed again with the new setting rather than
   replayed from before the change.
7. **Given** the recommendation scorer, the Prometheus usage source, or the
   Kubernetes usage source is installed, **When** the person opens its
   settings, **Then** the form lists the settings that plugin reads today,
   with secrets marked write-only.

### Edge Cases

- Removing a version that is not installed fails with a clear message and
  does not delete other versions or other plugins.
- Updating a plugin that is not installed is the same action as installing
  the latest catalog version, after the same confirmation.
- A plugin with several installed versions lists each version. Remove names
  the version it will delete.
- The person cannot type a filesystem path. Install location is the
  directory discovery already uses for this launch.
- A second confirmation is required only for a missing checksum file. A
  mismatch is a failure, not a confirmation.
- Progress for a failed operation ends in that failure, including a failure
  while downloading. A partial download is not left as an installed plugin.
- A validate request for a plugin that is not installed fails with a clear
  reason and does not install it.
- Settings for a plugin that is neither in the catalog nor installed are
  rejected. The generic editor does not create settings for an unknown name.
- An empty settings save writes nothing.
- A required setting that the plugin's default already satisfies can be
  saved without typing that setting again.
- A value that does not match the described type (a non-numeric whole
  number, a yes/no value other than true or false, a web address without a
  scheme and host, a duration the program does not accept, or a choice
  outside the list) is rejected and the file is unchanged.
- The page does not offer custom catalog URLs, a remote catalog, signature
  approval, routing-priority editing, per-request credentials, or any action
  that scaffolds a plugin or runs a package installer.
- Operations and settings saves that do not come from the dashboard page, or
  that are not the expected document type, are rejected and change nothing.
- Audit and error text name the plugin, the operation, and the setting
  names. They do not include setting values, tokens, or download URLs that
  carry secrets.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The dashboard has a Plugins page, linked from the dashboard
  navigation, that lists every built-in catalog entry with its description,
  providers, capabilities, security level, installed version when one
  exists, and latest catalog version. An installed plugin older than the
  latest version is marked update-available. A plugin that is installed but
  not in the catalog is listed too, marked as outside the catalog. It can be
  validated, configured, and removed. It cannot be installed or updated from
  the page.
- **FR-002**: From that page the person can install a catalog version,
  update a catalog plugin to the latest installable version, and remove one
  installed version. Each action shows a confirmation that names the plugin,
  version, source repository, and security level. Nothing is downloaded or
  deleted until that confirmation is submitted, and a confirmation that does
  not match the resolved target is rejected.
- **FR-003**: After confirmation, the page shows progress through resolving,
  downloading, verifying, extracting, and validating, including bytes
  received and the total when known, and any warning. The page then reports
  success or a specific failure. The command-line install and update text
  stays the same as before this feature.
- **FR-004**: Installed files are placed in the directory that plugin
  discovery and `finfocus plugin list` read for this launch, including
  catalog plugins whose releases use a monorepo tag prefix. The page does
  not accept a directory override of its own.
- **FR-005**: The page accepts only built-in catalog names. A repository
  slug or a URL is rejected. Checksum verification cannot be skipped. A
  checksum mismatch fails the operation and leaves the plugin uninstalled.
  A release with no checksum file continues only after a second confirmation
  of that warning.
- **FR-006**: When the requested release has no downloadable files, the page
  names the fallback version and waits for confirmation. It never installs a
  different version silently. An explicit version that cannot be installed
  fails rather than being replaced without that confirmation.
- **FR-007**: A second operation on a plugin that already has an install,
  update, or remove in progress fails with a busy message and does not
  change files.
- **FR-008**: When the catalog host rate-limits a request, the failure tells
  the person the request was rate-limited and that a token in the
  environment that launched the dashboard can raise the limit. The token
  value is never shown.
- **FR-009**: Validate checks the installed files and then starts the plugin
  and reads the plugin's own description. The result is pass or fail with
  the reason. The page does not offer conformance, certification, plugin
  scaffolding, or plugin upgrade.
- **FR-010**: A plugin that describes its settings gets a form driven by
  that description. The form supports text, whole numbers, yes/no, a fixed
  list of choices, web addresses, and durations. Required settings are
  enforced. Descriptions from the plugin are shown. A plugin that describes
  no settings gets a generic name and value editor for that plugin only.
  Names that the program already treats as credentials are write-only
  secrets in that editor.
- **FR-011**: Settings save to the project tier or the global tier through
  the same write path as the Config page: tier selection, preservation of
  comments and unrelated keys, a version check, one-at-a-time writes, and
  write-only secrets. No secret value reaches the browser, logs, audit
  entries, or error messages. A blank secret does not overwrite a stored
  secret. The masked placeholder is never written.
- **FR-012**: On the next launch of a plugin, each saved setting is delivered
  to that plugin as an environment variable. Precedence is the parent
  environment, then project configuration, then global configuration, then
  the plugin's declared default. A variable the parent environment already
  set is left unchanged.
- **FR-013**: A setting is delivered only to the plugin that owns it. One
  plugin's secret is never placed in another plugin's environment.
- **FR-014**: A stored cost answer from before a plugin setting changed is
  not reused for the same request after that setting changed. The answer is
  computed again with the settings the plugin receives now.
- **FR-015**: The recommendation scorer, the Prometheus usage source, and
  the Kubernetes usage source each describe the settings they read today,
  including which of those settings are secrets.
- **FR-016**: Requests that change plugins or settings follow the dashboard's
  existing rules for which page originated the request and which document
  type is accepted. A request that fails those rules changes nothing.
- **FR-017**: The plugin system guide, the command reference's description of
  the plugin configuration section, and the web dashboard how-to, reference,
  and architecture guides describe this page. The command reference no
  longer implies that stored plugin settings reach a plugin when they do
  not, and it describes the delivery rule in FR-012. The guides no longer
  say that plugin management is unavailable in the dashboard.
- **FR-018**: Specifications 627 and 628 are updated so plugin install,
  update, remove, validate, and per-plugin settings are in scope for this
  page, and so install, update, and remove are allowed local side effects.
  Specification 627's rule that the dashboard does not change cloud
  infrastructure still holds.

### Key Entities

- **Catalog entry**: A plugin the built-in catalog can install. It has a
  name, description, providers, capabilities, security level, source
  repository, and latest version. It is not a user-supplied URL.
- **Installed version**: One version of a plugin present in the discovery
  directory, distinct from the latest catalog version.
- **Plugin operation**: A confirmed install, update, or remove. It has a
  target plugin and version, a progress sequence, and a success or failure
  outcome. Only one operation per plugin runs at a time.
- **Setting description**: One setting a plugin declares: a key, a kind of
  value, whether it is required, whether it is secret, an optional default,
  a description, the environment variable the plugin reads, and the allowed
  choices when the kind is a fixed list.
- **Plugin settings view**: The described settings, or the generic pairs,
  plus the stored project and global values, with secrets reduced to set or
  unset, and a version used to detect a stale save.
- **Delivered setting**: The value a plugin process actually receives after
  parent environment, project, global, and default precedence.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A person can see every built-in catalog entry, whether it is
  installed, and whether an update is available, without leaving the
  dashboard or reading a plugin document. An installed plugin from outside
  the catalog is visible and can be removed or configured, and it cannot be
  updated from the page.
- **SC-002**: A person can install, update, or remove a catalog plugin from
  the page, including a monorepo catalog plugin, and `plugin list` shows the
  same files afterward. Each action requires a confirmation that names the
  plugin, version, source, and security level.
- **SC-003**: In 100% of automated cases, a non-catalog name, a repository
  slug, a URL, a checksum mismatch, a missing checksum file without the
  second confirmation, and a missing-file release without the fallback
  confirmation leave the plugin directory unchanged.
- **SC-004**: In 100% of automated cases, a settings response and the
  dashboard's logs and errors contain no secret value. A secret saved for
  one plugin is absent from every other plugin's environment.
- **SC-005**: A person can set a stand-in plugin's response through the
  settings form and see the next dashboard query deliver that value to the
  plugin. An exported parent value for the same setting still wins.
- **SC-006**: Changing a delivered plugin setting causes the next identical
  cost request to be computed again. The previous cached answer is not
  returned.
- **SC-007**: The published plugin guide, command reference, and three web
  dashboard guides describe this page and the setting-delivery rule, and do
  not claim that stored plugin settings fail to reach plugins or that the
  dashboard cannot manage plugins.

## Assumptions

- The built-in catalog shipped with FinFocus is the only catalog. Custom
  registry URLs are a separate feature.
- Signature verification, per-request credentials, routing-priority editing,
  and agent-tool exposure of install, update, and remove are separate
  features. This page does not add them.
- Conformance, certification, plugin scaffolding, and plugin upgrade stay on
  the command line because they are slow or they invoke a package installer.
- Plugin settings are delivered as environment variables because that is how
  the plugins shipped today already read their configuration. A later
  per-request credential channel does not replace this page.
- The parent environment wins so that a shell export set up before this page
  existed keeps working. The page does not unset or overwrite it.
- "Apply on next launch" means the next time FinFocus starts that plugin
  process. The dashboard starts plugins when a query needs them. A process
  that is already running keeps the environment it was given.
- A required setting is satisfied when a parent variable, a stored value, or
  the plugin's declared default is present. The form does not force the
  person to retype a default.
- The recommendation scorer, Prometheus usage source, and Kubernetes usage
  source are the plugins in this repository. Plugins published from other
  repositories describe their own settings in those repositories.
- Specification 627's other rules still apply: loopback only, per-session
  token, and no exposure beyond the local machine.
- Install, update, and remove remain available on the command line. This
  page calls the same install behavior the command line uses, with the
  stricter catalog and checksum rules above. The command line's text does
  not change.
- The config page's masking, tier files, and conflict checks are reused.
  This feature does not invent a second way to write configuration files.
- A remembered cost answer must follow the settings that were delivered,
  because those settings can change what the plugin answers. Plugins that
  also depend on state the program cannot see still mark their prices as
  expiring immediately, which is an existing plugin duty and is not changed
  here.
