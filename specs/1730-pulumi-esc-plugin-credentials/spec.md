# Feature Specification: Pulumi ESC Credentials for Plugins

**Feature Branch**: `1730-pulumi-esc-plugin-credentials`
**Created**: 2026-10-19
**Status**: Draft
**Input**: User description: "#1865 — Deliver Pulumi ESC environment
variables to plugins as credentials, with fallback to existing sources"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Plugins use the stack's ESC environment (Priority: P1)

A person keeps cloud and SaaS credentials in a Pulumi ESC environment and
imports it in the stack's configuration file, as Pulumi recommends. They run
a FinFocus cost command, the overview, or the web dashboard from that
project. With no extra flags, wrapper commands, or exported variables, each
plugin receives the credentials it needs from that environment.

A plugin receives only the variables it is allowed to receive: the variables
it names in its own advertised settings. A plugin that has no use for a
variable does not get it, even when the environment contains it.

**Why this priority**: This is the whole journey. Today the person must wrap
every command in `pulumi env run`, which gives every variable to every
plugin, or copy secrets into a plain-text configuration file.

**Independent Test**: In a project whose stack imports an environment that
holds a stand-in plugin's credential, run `finfocus cost actual` with nothing
exported in the shell. The stand-in plugin receives the credential, and a
second plugin that does not advertise that variable does not.

**Acceptance Scenarios**:

1. **Given** the stack's configuration file imports an ESC environment that
   defines a variable a plugin advertises, **When** FinFocus launches that
   plugin, **Then** the plugin receives the environment's value.
2. **Given** the same environment also defines a variable the plugin does
   not advertise, **When** the plugin launches, **Then** the plugin does not
   receive that variable.
3. **Given** the shell already exports a variable that the ESC environment
   also defines for that plugin, **When** the plugin launches, **Then** the
   plugin receives the ESC value.
4. **Given** the stack imports two environments that define the same
   variable, **When** the plugin launches, **Then** the value from the
   environment listed later wins, matching how ESC combines imports.
5. **Given** the person passes `--stack` for a stack other than the selected
   one, **When** plugins launch, **Then** the environments imported by the
   named stack are used.

---

### User Story 2 - Existing setups keep working when ESC is unavailable (Priority: P1)

A person whose credentials come from the shell or from FinFocus
configuration files sees no change. When ESC cannot be used (no Pulumi Cloud
login, a self-managed state backend, a missing environment, denied access, or
a network failure), the command still runs. Plugins get their values from the
shell, the project configuration, the global configuration, and plugin
defaults, as today, and the person sees one warning that names the
environment and the reason category without showing any value.

**Why this priority**: Credentials are on the path of every cost query. A
feature that breaks working setups or turns a network problem into a failed
command is worse than no feature.

**Independent Test**: With no ESC configuration and no stack imports, the
environment each plugin receives is identical to the previous release. Then
point the configuration at an environment that does not exist and confirm
that the command succeeds with shell-provided credentials and one redacted
warning.

**Acceptance Scenarios**:

1. **Given** no ESC environment is configured or imported, **When** any
   command launches plugins, **Then** each plugin receives exactly what it
   received before this feature, and cost answers saved before the upgrade
   are reused.
2. **Given** an imported environment cannot be opened for any reason,
   **When** the command runs, **Then** it completes using the existing
   sources and prints one warning naming the environment reference and the
   failure category.
3. **Given** the person is not logged in to Pulumi Cloud and has no access
   token, **When** a command runs in a project that imports an environment,
   **Then** the warning says that ESC was skipped because no Pulumi Cloud
   login was found.
4. **Given** the project uses a self-managed state backend, **When** a
   command runs, **Then** ESC is skipped with a warning and the command
   proceeds.
5. **Given** a variable the ESC environment does not define, **When** the
   plugin launches, **Then** precedence for that variable is unchanged:
   shell, then project configuration, then global configuration, then the
   plugin's default.

---

### User Story 3 - Choose the environment explicitly (Priority: P2)

A person wants a plugin to use a different ESC environment than the stack
imports, wants all plugins to use one environment when no stack applies (for
example, a plan file analyzed outside a Pulumi project), or wants to turn ESC
off. They set this in FinFocus configuration, globally or per plugin, at the
project or global tier.

**Why this priority**: Stack imports cover the common case. Explicit
selection covers billing-account credentials that do not belong to any one
stack and gives an off switch.

**Independent Test**: Configure one plugin to use a named environment and
confirm that it receives that environment's value while other plugins keep
using the stack's imports. Then configure an empty list and confirm that no
environment is opened.

**Acceptance Scenarios**:

1. **Given** a per-plugin environment setting, **When** that plugin launches,
   **Then** it uses only the named environments and ignores the stack's
   imports. Other plugins are unaffected.
2. **Given** a global environment setting and no per-plugin setting, **When**
   any plugin launches, **Then** the global setting replaces the stack's
   imports.
3. **Given** an environment setting that is an empty list, **When** plugins
   launch, **Then** no ESC environment is opened for the plugins it covers.
4. **Given** an environment reference with a version or tag, **When** the
   plugin launches, **Then** values come from that version.
5. **Given** an environment reference that is not well formed, **When** the
   person runs `config validate` or any cost command, **Then** it is reported
   as a configuration error that names the key, before any plugin launches.

---

### User Story 4 - Allow additional variables for a plugin (Priority: P2)

Some plugins read standard provider variables (for example, the AWS SDK's
credential variables) that they do not advertise as settings. A person lists
those variable names for that plugin in configuration, and only that plugin
receives them from ESC.

**Why this priority**: Without it, plugins that rely on provider SDK defaults
cannot use ESC at all. It is P2 because advertised settings cover FinFocus's
own plugins.

**Independent Test**: Allow `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and
`AWS_SESSION_TOKEN` for one plugin. That plugin receives them from ESC, and
the recommendation scorer does not.

**Acceptance Scenarios**:

1. **Given** a plugin with an allowed-variable list, **When** it launches,
   **Then** it receives each listed variable that the ESC environment
   defines.
2. **Given** another plugin without that list, **When** it launches, **Then**
   it receives none of those variables from ESC.
3. **Given** an allowed name is one FinFocus reserves for plugin startup (the
   port, log file, or trace id variable), **When** the plugin launches,
   **Then** that variable is never taken from ESC.

---

### User Story 5 - See where a plugin setting comes from (Priority: P3)

In the dashboard's plugin settings, a person can see that a setting's value
is supplied by an ESC environment and which environment it is, so they know
why editing the file has no effect. The value itself is never shown.

**Why this priority**: Visibility helps troubleshooting, and it is required
for web parity, but delivery works without it.

**Independent Test**: With a setting supplied by ESC, open the plugin's
settings in the dashboard. The row says it comes from ESC, names the
environment reference, and reveals no value.

**Acceptance Scenarios**:

1. **Given** a setting supplied by ESC, **When** the person opens that
   plugin's settings, **Then** the row states that ESC supplies the value and
   names the environment, and the help text says ESC overrides the shell and
   the files.
2. **Given** ESC could not be opened, **When** the person opens the settings,
   **Then** rows show their usual source, and no ESC source is claimed.

---

### Edge Cases

- An ESC variable whose value is not a string (an object or a list) is not
  delivered. A debug log line names the variable, not the value.
- The environment defines a variable for FinFocus's own startup (port, log
  file, or trace id). It is never delivered from ESC.
- Several plugins use the same environment in one command. The environment
  is opened once, not once per plugin.
- A long-running session (the web dashboard, the MCP server) relaunches
  plugins after a settings change. The environment is opened again so that
  short-lived credentials are fresh. Plugins already running keep the values
  they started with.
- Short-lived credentials (for example, OIDC session keys) change every time
  the environment is opened. Repeat runs at the same environment revision
  still reuse saved cost answers. A new environment revision or a changed set
  of delivered variables does not reuse them.
- Opening the environment takes longer than the time limit. The command
  continues on the fallback path, and the wait adds no more than that limit.
- No stack can be resolved and no explicit environment is configured. ESC is
  not used, and no warning is shown, because nothing asked for it.
- A plan or state file is analyzed outside a Pulumi project. Only explicitly
  configured environments apply.
- The person runs the command inside `pulumi env run` as well. The process
  environment and ESC agree, and ESC wins on any difference.
- The stack's `environment:` block is an inline definition (it has `values:`,
  or an import that turns off merging) rather than a list of named
  environments. FinFocus does not reproduce inline definitions. It skips ESC
  for that stack with the "unsupported definition" warning, and plugins fall
  back. An explicit environment setting still works.
- No stack is named and the Pulumi CLI has not selected one for the project.
  Stack imports are not used.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When a command launches plugins and the stack's configuration
  file imports ESC environments, FinFocus MUST open those environments and
  use their environment variables as a source of plugin settings. The stack
  is the one named by `--stack`, else the one the Pulumi CLI has selected for
  the project. Finding it MUST NOT run the Pulumi CLI.
- **FR-002**: An explicit environment setting MUST replace the stack's
  imports for what it covers. A per-plugin setting applies to that plugin and
  takes precedence over a global setting, which applies to all plugins. Both
  MUST be settable at the project and global configuration tiers. A project
  ESC section replaces the global one as a whole, as other top-level
  configuration sections do.
- **FR-003**: An explicit empty environment list MUST disable ESC for what it
  covers.
- **FR-004**: Several environments MUST be combined in listed order, with
  later environments winning for the same variable.
- **FR-005**: A plugin MUST receive an ESC variable only when the variable is
  named by one of that plugin's advertised settings or is listed in that
  plugin's allowed-variable configuration. Nothing else is delivered.
- **FR-006**: For a variable that ESC supplies and the plugin is allowed to
  receive, the ESC value MUST take precedence over the shell, the project
  configuration, the global configuration, and the plugin's default.
- **FR-007**: For every variable ESC does not supply, precedence MUST be
  unchanged from the previous release.
- **FR-008**: Variables FinFocus reserves for plugin startup (port, log file,
  trace id) MUST never be taken from ESC.
- **FR-009**: When an environment cannot be opened, FinFocus MUST continue
  with the existing sources and emit one warning per environment per command.
  The warning names the environment reference and one category: not logged
  in, unsupported backend, not found, access denied, timed out, unsupported
  definition, or unavailable.
- **FR-010**: Opening an environment MUST have a time limit of 5 seconds per
  command. When it expires, the fallback in FR-009 applies.
- **FR-011**: FinFocus MUST authenticate to Pulumi Cloud with the
  `PULUMI_ACCESS_TOKEN` variable when it is set. Otherwise it MUST use the
  login the Pulumi CLI saved for the current backend when that login is
  stored in plain text. When the saved login is encrypted with the operating
  system's keychain, or is missing, ESC is skipped with the "not logged in"
  warning, which tells the person to set `PULUMI_ACCESS_TOKEN`. FinFocus MUST
  NOT require the Pulumi CLI to be installed in order to open an environment,
  and it MUST NOT write to Pulumi's own files.
- **FR-012**: Each environment MUST be opened at most once per command,
  whatever the number of plugins. A long-running session MUST open it again
  when it relaunches plugins.
- **FR-013**: ESC values MUST NOT appear in logs at any level, error text,
  command output in any format, dashboard responses, or files written by
  FinFocus, including the cost cache. They are held in memory only for the
  launch of the plugin processes.
- **FR-014**: Saved cost answers MUST NOT be reused after the environment
  revision or the set of delivered ESC variable names changes for a plugin.
  They MUST still be reused when only the values of the same variables at the
  same revision change.
- **FR-015**: A malformed environment reference or allowed-variable list in
  configuration MUST be a validation error (exit code 2) that names the key.
  It MUST be reported by `config validate` and before any plugin launches.
- **FR-016**: With `--debug`, FinFocus MUST log which environment references
  supplied which variable names to which plugin, without any values.
- **FR-017**: The dashboard's plugin settings MUST show, for each setting,
  when ESC supplies the effective value and which environment reference
  supplied it. The value MUST NOT be shown. This information MUST come from
  the same resolution the plugin launch uses.
- **FR-018**: The new configuration keys MUST be readable and settable with
  `config get` and `config set`, and editable on the dashboard Config page,
  like other configuration keys.
- **FR-019**: The feature MUST NOT require a change to the plugin protocol or
  to any plugin. A plugin reads ESC-supplied values exactly as it reads values
  from the shell today.
- **FR-020**: FinFocus MUST send a Pulumi access token only to the backend it
  belongs to. `PULUMI_ACCESS_TOKEN` goes only to the URL from
  `PULUMI_BACKEND_URL`, the Pulumi CLI's current login, or
  `https://api.pulumi.com`. A backend URL taken from the project's
  `Pulumi.yaml` uses only a token the Pulumi CLI saved for exactly that URL.
  Otherwise ESC is skipped as an unsupported backend. A cloned repository
  therefore cannot redirect the person's token by naming its own backend.

### Key Entities

- **ESC environment reference**: Names one Pulumi ESC environment, as
  `[organization/]project/environment`, optionally with `@version` or
  `@tag`. When the organization is omitted, it is the stack's organization
  for stack imports, or the logged-in user's default organization for
  configured references.
- **Environment selection**: The ordered list of references that applies to
  one plugin. It comes from the per-plugin setting, else the global setting,
  else the stack's imports.
- **Allowed variables**: For one plugin, the union of the variable names its
  advertised settings declare and the names listed in its allowed-variable
  configuration.
- **Setting source**: Where a plugin variable's effective value comes from:
  ESC (with the reference), shell, project configuration, global
  configuration, or plugin default.

## Web UI Fit *(mandatory)*

- **Disposition**: Extends Plugins → Settings (the per-plugin settings form)
- **Web data**: For each setting row, whether the effective value is supplied
  by ESC and the environment reference that supplied it. These are the same
  facts the CLI uses at plugin launch and logs with `--debug`. No value is
  sent. The new configuration keys appear on the existing Config page with no
  special handling.
- **Layout fit**: Fills the existing **Status** cell of the "Plugin settings"
  captioned table, which today shows `not set`, `set`, or the tier value, and
  the row's help paragraph, which today says "An environment value is set and
  overrides the file." An ESC-supplied row reads, for example, "from ESC
  (`team/aws-billing`)", and its help paragraph says that ESC overrides the
  shell and both files.
- **Additions**: none
- **Render evidence**: `../finfocus-demo`, stack `dev`, 2026-10-20. Playwright
  opened `finfocus --web` with `esc.plugins.recorder.environments` set to
  `acme/billing/recorder` on a temporary home and a loopback ESC server.
  Plugins → recorder → Settings shows the help sentence "Supplied by Pulumi
  ESC environment `acme/billing/recorder`, which overrides the shell and both
  files." The Status cell still reads `not set`. No variable value is shown.
  Screenshot: `.playwright-mcp/esc-recorder-settings.png`. The demo
  `Pulumi.dev.yaml` already had local edits, so it was not rewritten.
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility). No engine, API, redaction, or data-shape work. Filed as
  #1867.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A person whose stack already imports an ESC environment can get
  plugin credentials from it with zero added flags, wrapper commands, or
  exported variables.
- **SC-002**: With no ESC configured or imported, 100% of existing plugin
  environments are unchanged, and saved cost answers from before the upgrade
  are still reused.
- **SC-003**: In every ESC failure category, the command completes
  successfully on the fallback sources with exactly one warning per
  environment.
- **SC-004**: When ESC is unreachable, a command takes no more than 6 seconds
  longer than the same command with ESC disabled.
- **SC-005**: A fixture secret placed in an ESC environment is found in zero
  log lines, error messages, output formats, dashboard responses, or files
  that FinFocus writes.
- **SC-006**: A plugin that is not allowed a variable receives it in 0% of
  launches, even when every environment in use defines it.
- **SC-007**: Two consecutive runs at the same environment revision, with
  rotating short-lived credentials, reuse all saved cost answers on the
  second run.

## Assumptions

- ESC requires Pulumi Cloud (hosted or self-hosted). A file-based or
  object-storage state backend has no ESC, so FR-009 applies there.
- Only the environment's `environmentVariables` output is used. The
  `pulumiConfig` and `files` outputs are ignored.
- FinFocus only reads ESC environments. It never creates, edits, or rotates
  them.
- Plugins keep one process per plugin. Per-request credentials and shared
  plugin processes are #1539 and out of scope.
- The Pulumi Analyzer path (`analyzer serve`) runs inside a Pulumi
  operation that already sets the stack environment's variables in the
  process environment, which plugins inherit (confirmed in research R10).
  The analyzer therefore applies only explicitly configured environments,
  not the stack's imports.
- The scorer and other third-party-facing plugins get ESC values only through
  FR-005, so a shared environment does not widen what reaches those services.
- Precedence where ESC wins over the shell is intentional and is called out
  in user documentation and release notes as a behavior change, limited to
  variables that ESC supplies.
