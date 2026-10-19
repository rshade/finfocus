# Feature Specification: Usage Profile

**Feature Branch**: `630-usage-profile`
**Created**: 2026-10-18
**Status**: Implemented
**Input**: GitHub issue #368. Let a person tell FinFocus whether a projection
is for development, production, or a burst workload, and pass that hint to
pricing plugins. A command flag, an environment variable, or a saved default
can set it. Measured spend does not change.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Ask for a development projection (Priority: P1)

A person estimating a stack they will only run during the work week passes
`--profile=dev`. FinFocus sends that hint with every projected-cost request
and every recommendation request for that run. Plugins may then assume fewer
hours, cheaper instance classes, or shorter retention. FinFocus still shows
whatever the plugin returns, including a plugin that ignores the hint.

**Why this priority**: This is the behavior #368 asks for. Without the hint
on the request, a flag would not change any estimate.

**Independent Test**: Run a projected-cost command with `--profile=dev`
against a plugin that records the request, and confirm the recorded request
carries the development profile. Repeat with `prod` and `burst`.

**Acceptance Scenarios**:

1. **Given** no saved profile, **When** the person runs `finfocus cost projected --profile=dev`, **Then** each projected-cost request for that run carries the development profile.
2. **Given** no saved profile, **When** the person runs the same command with `--profile=prod` or `--profile=burst`, **Then** each projected-cost request carries that profile.
3. **Given** a recommendations command with `--profile=dev`, **When** FinFocus asks plugins for recommendations, **Then** each recommendation request carries the development profile.
4. **Given** a plugin that ignores the profile, **When** the person passes any accepted profile, **Then** the command still returns that plugin's ordinary result.
5. **Given** two runs of the same plan that differ only by profile, **When** a plugin returns a different price for each profile, **Then** FinFocus shows each run's own price and does not reuse the other run's stored projection.

---

### User Story 2 - Keep a default without repeating the flag (Priority: P1)

A person who mostly estimates development stacks sets a default profile in
FinFocus configuration, or exports the profile environment variable, and
later runs `finfocus cost projected` with no flag. The same hint is sent.
An explicit flag on one command overrides the saved default for that command
only and does not rewrite the configuration file.

**Why this priority**: #368 requires a configuration default and an
environment override. Repeating the flag is not the only way to work.

**Independent Test**: Set the configuration default to `dev`, run a
projected-cost command with no flag, and confirm the request carries the
development profile. Then run once with `--profile=prod` and confirm that
run carries production while the file still says `dev`.

**Acceptance Scenarios**:

1. **Given** the cost configuration profile is `dev` and the environment variable is unset, **When** the person runs a projected-cost command with no profile flag, **Then** requests carry the development profile.
2. **Given** the environment variable is `burst` and the configuration profile is `dev`, **When** the person runs with no flag, **Then** requests carry the burst profile.
3. **Given** the environment variable is `dev` and the configuration profile is `prod`, **When** the person passes `--profile=prod`, **Then** that run carries production and neither the file nor the environment variable is changed.
4. **Given** no flag, no environment variable, and no configuration profile, **When** the person runs a projected-cost command, **Then** requests carry no preference and plugins apply their ordinary assumptions.
5. **Given** a configuration profile, an environment variable, or a flag whose value is not one of `dev`, `prod`, `burst`, or `default`, **When** the command or configuration check runs, **Then** it fails as a user-input error and does not call plugins.

---

### User Story 3 - See which profile was used (Priority: P2)

A person reading a table, a JSON result, or a recommendation list can tell
which profile FinFocus sent. `default` and an omitted profile both mean no
preference. The name is the resolved value for that run, after the flag,
environment variable, and configuration have been applied.

**Why this priority**: A saved default changes numbers with no flag on the
command line. The result has to say why.

**Independent Test**: Run once with `--profile=dev` and once with no profile
set, and read both the human output and the JSON output.

**Acceptance Scenarios**:

1. **Given** `--profile=dev`, **When** projected costs or recommendations are printed as text or JSON, **Then** the output names `dev`.
2. **Given** no profile from any source, **When** the same commands print a result, **Then** the output names no preference (`default`).
3. **Given** a configuration profile of `prod` and no flag, **When** the result is printed, **Then** the output names `prod`.

---

### User Story 4 - Leave measured spend alone (Priority: P2)

A person comparing a projection with the bill uses the same configuration
default. The projection follows the profile. The measured month-to-date
cost, cost history, and a historical cluster window do not. Those numbers
describe what was billed, not what a workload is intended to do.

**Why this priority**: Changing actuals because of a planning hint would
make the bill and the tool disagree.

**Independent Test**: With the configuration profile set to `dev`, run a
projected-cost command and an actual-cost command against a recorder plugin.
Only the projected request carries the development profile.

**Acceptance Scenarios**:

1. **Given** a resolved profile of `dev`, **When** the person runs actual cost, cost history, or a historical cluster window, **Then** those requests do not carry a usage profile.
2. **Given** a resolved profile of `dev`, **When** overview, forecast, estimate, a cluster run-rate, or the Pulumi analyzer asks a plugin for a projected monthly cost, **Then** those projected requests carry the development profile. The analyzer reads the environment variable and the configuration profile.
3. **Given** a resolved profile of `dev`, **When** the dashboard asks for the same projected costs or recommendations, **Then** those requests carry the development profile. This feature does not add a profile control to the dashboard.

---

### Edge Cases

- `default` is an accepted value and means no preference. It is not a fourth
  workload.
- Names are accepted in any letter case. `DEV` and `dev` are the same value.
- An empty flag value, an empty environment variable, and an empty
  configuration value are user-input errors, not "no preference". No
  preference is the absence of the setting, or the explicit value `default`.
- A profile applies to the whole run. One command cannot price some
  resources as `dev` and others as `prod`.
- What-if estimate sends the profile on the projected-cost requests it
  makes. It does not invent a second profile field for the what-if call
  itself, because that call has no profile field.
- A plugin that returns the same price for every profile is a valid result.
  FinFocus does not invent a dev-hours discount of its own.
- A stored projection from another profile is not a hit. The run asks the
  plugin again.
- Unknown future profile names are rejected in FinFocus. Accepted names are
  `dev`, `development`, `prod`, `production`, `burst`, `default`, and
  `unspecified`. The last two mean no preference.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: FinFocus MUST accept a profile of `dev`, `prod`, `burst`, or `default` on projected-cost and recommendation commands, ignoring letter case. `development` is `dev`, `production` is `prod`, and `unspecified` is `default`.
- **FR-002**: FinFocus MUST resolve one profile per run in this order: the command flag, then the profile environment variable, then the cost profile in configuration, then no preference.
- **FR-003**: A resolved profile other than no preference MUST be sent on every projected-cost request and every recommendation request in that run.
- **FR-004**: No preference MUST leave the request profile unset so plugins keep their ordinary assumptions.
- **FR-005**: Actual cost, cost history, and historical cluster usage MUST NOT send a usage profile.
- **FR-006**: Forecast, estimate, overview, cluster run-rate, the dashboard, and the Pulumi analyzer MUST use the resolved profile on the projected-cost and recommendation requests they already make. The analyzer has no flag; it uses the environment variable and the configuration profile.
- **FR-007**: An explicit flag MUST NOT rewrite configuration or the environment variable.
- **FR-008**: An unknown, empty, or unsupported profile from the flag, the environment variable, or configuration MUST fail as a user-input error before plugins are called. Configuration validation MUST report the same error.
- **FR-009**: Human and JSON results for projected cost and recommendations MUST name the resolved profile, using `default` for no preference.
- **FR-010**: A stored projection computed under one profile MUST NOT be returned as the result for a different profile.
- **FR-011**: FinFocus MUST NOT change plugin prices itself because of the profile. Hour assumptions, instance class, and retention stay in the plugin.
- **FR-012**: This feature MUST NOT add a dashboard control for choosing a profile.

### Key Entities

- **Usage profile**: The workload hint for one run. One of development, production, burst, or no preference.
- **Profile source**: The place the hint came from for that run: flag, environment, configuration, or absence.
- **Projected request**: A plugin request for a future price or a recommendation. It can carry the hint.
- **Measured request**: A plugin request for billed or historical usage. It does not carry the hint.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A person can switch a projection among development, production, and burst with one flag and no resource edits.
- **SC-002**: With a configuration default of `dev` and no flag, a projected-cost run sends the development profile on every projected-cost request.
- **SC-003**: The same configuration default does not appear on an actual-cost request.
- **SC-004**: Two projections of one plan that differ only by profile each show the price the plugin returned for that profile.
- **SC-005**: An unsupported profile value ends the command as a user-input error and produces no plugin request.
- **SC-006**: A person can read the resolved profile from both the text result and the JSON result.

## Assumptions

- The published plugin contract already defines development, production,
  burst, and "no preference". This feature only teaches FinFocus to send
  that hint. Plugin pricing changes are out of scope.
- The issue's value `default` is "no preference", not a workload of its own.
  `burst` is included because the contract and the roadmap already name it.
- The environment variable is the product's profile variable. The old
  `PULUMICOST_PROFILE` name is not read.
- The configuration value lives with the other cost settings and follows
  the existing project-over-global configuration rules.
- The what-if plugin call has no profile field. Estimate still sends the
  profile on the projected-cost calls it uses.
- The dashboard follows the same resolved profile as the CLI and does not
  grow a new control in this feature.
- `cost projected` and the recommendations command are the commands the
  issue names. The other projected commands share the same resolution so
  one saved default cannot mean two different hints in one sitting.
