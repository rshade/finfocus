# Feature Specification: Pulumi Cloud Historical State Snapshots

**Feature Branch**: `1735-pulumi-cloud-history`
**Created**: 2026-10-27
**Status**: Draft
**Input**: GitHub Issue #935 - Pulumi Cloud API Integration for Historical State Snapshots

## Clarifications

### Session 2026-10-27

- Q: How should an imported resource's lifetime be bounded? → A: Record only
  the update times where the resource appeared; infer no lifetime before its
  first or after its last observed update.
- Q: Should Cloud history be on by default? → A: No. The default is `false`
  (opt-in); users enable it with `auto` or `true`. Automated tests enable it
  explicitly.
- Q: Which commands trigger the import? → A: First answered as "every actual
  cost view", then narrowed: `cost actual` imports inline, and a dedicated
  `--import-cloud-history` command does the long pull ahead of time. The
  overview and `finfocus --web` never fetch or wait on Pulumi Cloud; they read
  the stored history. This replaces the earlier manual-refresh follow-up.
- Q: Which updates are imported? → A: Any `update`, `refresh`, or `destroy`
  whose snapshot shows resources, whatever its result (including failed or
  canceled). Previews and other kinds that cannot change state are skipped.
  Planning added `import` to the three kinds, because an import adds
  resources with physical IDs.
- Q: How is an unreadable update reported under `auto`? → A: A warning on
  stderr, and the JSON and human output carry an "incomplete Cloud history"
  flag with the count of updates that could not be read. The exit code is
  unchanged.
- Q: The setting appears on the dashboard Config page through the shared
  setting list. How is that recorded? → A: Web UI Fit is an extension of the
  Config page (render check and UX-only follow-up apply), not `Not applicable`.
- Q: Should a long-lived resource destroyed inside the period be caught by
  extending its last-seen time? → A: No. Keep observation-only and document
  the limit.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Complete Resource Timeline for a Billing Period (Priority: P1)

A user whose stack lives in Pulumi Cloud, and who has enabled Cloud history,
runs `finfocus cost actual` for a past period (for example March 1-31). A short-lived resource was created and
destroyed between two finfocus runs, so the local resource history never saw
it. Its cloud costs appear on the bill but not in the report.

With this feature, finfocus asks Pulumi Cloud for the stack's recorded
updates inside the period, reads the resources each update contained, and adds
their cloud identities to the local resource history before costs are
calculated. The report then includes the short-lived resource.

**Why this priority**: This is the whole reason for the feature. The local
history only knows what finfocus has observed, so every resource that lived and
died between runs is silently missing from actual-cost totals. Pulumi Cloud
holds the complete record.

**Independent Test**: With Cloud history enabled, deploy a resource, destroy
it, and run no finfocus command in between. Then run `cost actual` over the period. The report must
include the destroyed resource's cost, and the local history must show it with
source `pulumi_cloud`.

**Acceptance Scenarios**:

1. **Given** a Pulumi Cloud stack with a resource created on day 5 and
   destroyed on day 9 that finfocus never observed, **When** the user runs
   `cost actual` for the full month, **Then** its cloud identity is included
   in the actual-cost query and the billing plugin's returned cost is included
   in the total. The import does not infer its lifetime through day 9.
2. **Given** the stack had several updates in the period, **When** the user
   runs `cost actual`, **Then** every cloud identity that appeared in any
   state-changing update inside the period, whatever its result, is recorded in the local history.
3. **Given** the same resource and cloud identity already exists in the local
   history from a state snapshot, **When** the cloud data is imported, **Then**
   the existing entry is extended (earliest first-seen and latest last-seen
   win) and no duplicate is created.
4. **Given** a resource appeared in exactly one update, **When** its identity
   is imported, **Then** its first-seen and last-seen are both that update's
   end time, and nothing before or after is inferred.
5. **Given** identities were imported earlier by the import command,
   **When** the user views actual cost for the same stack and period in the
   overview or the `--web` Cost page, **Then** the totals include them, and no
   Pulumi Cloud request is made or waited on by those views.

---

### User Story 2 - Automatic Fallback When Pulumi Cloud Is Unavailable (Priority: P1)

A user who has set Cloud history to `auto` runs finfocus against a stack that
uses a local file or object-storage backend, is not logged in, has an expired
token, or is offline. Nothing about
their experience changes: finfocus uses the local history alone, exactly as
it does today.

**Why this priority**: The enhancement is optional. Breaking or slowing the
existing flow for users without Pulumi Cloud would be a regression for the
majority of installs.

**Independent Test**: With `enabled: auto`, run `cost actual` with no Pulumi
login, with a file-backed stack, and with the Cloud endpoint unreachable. Output and exit
code must match the pre-feature behavior in each case.

**Acceptance Scenarios**:

1. **Given** the backend is not Pulumi Cloud (file, S3, GCS, or Azure Blob),
   **When** the user runs `cost actual` with `enabled: auto`, **Then** no
   network request is made to any Pulumi Cloud endpoint and the result is
   unchanged.
2. **Given** the user is not logged in or the saved token is rejected, **When**
   the user runs `cost actual` with `enabled: auto`, **Then** finfocus
   continues with local history and may note at debug level why Cloud data was
   skipped.
3. **Given** Pulumi Cloud is reachable at detection time but fails partway
   through a fetch, **When** the user runs `cost actual` with
   `enabled: auto`, **Then** the entries already imported are kept, a warning names
   the incomplete import, and the cost report still completes.

---

### User Story 3 - Control Over Whether Cloud History Is Used (Priority: P2)

A user controls the integration with a single setting,
`cost.history.pulumi_cloud.enabled`, with three values. `false` (the default)
never contacts Pulumi Cloud for history, so nothing changes until the user opts
in. `auto` detects Pulumi Cloud and uses it when present. `true` requires it: if Cloud history cannot be
retrieved, the command fails with a clear error instead of reporting totals
that might be incomplete.

**Why this priority**: Opt-in keeps the default run free of new network
calls. Auditors and finance teams need the "complete or fail" guarantee of
`true`. This ranks below the core import because it governs how the import is
turned on.

**Independent Test**: Run `cost actual` with each of the three values against
a reachable Cloud stack and against an unreachable one, and compare behavior.

**Acceptance Scenarios**:

1. **Given** `enabled` is unset or `false`, **When** the user runs
   `cost actual`, **Then** finfocus makes no Pulumi Cloud history request even
   when logged in, and output and exit code are unchanged.
2. **Given** `enabled: true` and Pulumi Cloud cannot be reached or
   authenticated, **When** the user runs `cost actual`, **Then** the command
   exits 1 with an error that says what failed and how to fix it, and reports
   no cost totals.
3. **Given** `enabled: true` and the backend is a file or object-storage
   backend, **When** the user runs `cost actual`, **Then** the command exits
   with an error stating that Cloud history requires a Pulumi Cloud backend.
4. **Given** an unrecognized value for `enabled`, **When** configuration is
   validated, **Then** `config validate` and every `cost` command reject it
   and list the three allowed values.
5. **Given** the user sets `enabled` with `config set` or on the dashboard
   Config page, **When** the value is saved, **Then** it is stored and read
   back the same way as a value written by hand, and an unrecognized value is
   refused with the three allowed values.

---

### User Story 4 - Fast Repeat Runs and Respectful API Use (Priority: P2)

A user runs `cost actual` repeatedly over the same month, or a team shares a
large stack with hundreds of updates. Repeat runs should not re-download data
that cannot change, and a first run on a large stack must not overwhelm the
Pulumi Cloud API or hit its rate limits.

**Why this priority**: Without reuse, every run pays the full download cost
and a busy stack can trip rate limits, which would make the feature unusable
in exactly the cases where it matters most.

**Independent Test**: Run `cost actual` twice over the same period. The second
run must issue no per-update snapshot requests for updates already stored and
must only re-list updates once the cached list is more than an hour old.

**Acceptance Scenarios**:

1. **Given** a snapshot for an update was fetched earlier, **When** the user
   runs `cost actual` again, **Then** that snapshot is read from the local
   cache and not requested again.
2. **Given** the update list was fetched less than one hour ago, **When** the
   user runs `cost actual` again, **Then** the list is read from the cache.
3. **Given** the cached update list is older than one hour, **When** the user
   runs `cost actual`, **Then** the list is refreshed and only updates not
   seen before are fetched.
4. **Given** a stack with many updates in the period, **When** finfocus
   fetches snapshots, **Then** no more than five requests are in flight at
   once, and a rate-limit response is retried after the server's requested
   delay rather than failing the run.

---

### User Story 5 - Self-Hosted Pulumi Cloud and Token Override (Priority: P3)

A user whose organization runs a self-hosted Pulumi Cloud, or who runs finfocus
in CI with a service token, gets the same history import as users of
`app.pulumi.com`. The token comes from the existing Pulumi login by default
and can be overridden explicitly.

**Why this priority**: Important for enterprise and CI adoption, but the
default login path covers most users first.

**Independent Test**: Point finfocus at a stand-in self-hosted endpoint with
an override token and confirm the import works and the token goes only to that
endpoint.

**Acceptance Scenarios**:

1. **Given** the logged-in backend is a self-hosted Pulumi Cloud URL, **When**
   the user runs `cost actual`, **Then** history is imported from that
   endpoint.
2. **Given** `PULUMI_ACCESS_TOKEN` is set, **When** finfocus calls Pulumi
   Cloud, **Then** that token is used instead of the saved login token.
3. **Given** a token of any kind, **When** finfocus makes any request,
   **Then** the token is sent only to the resolved Pulumi Cloud backend host
   and never to any other host, including redirect targets.

---

### User Story 6 - Pre-Load History Without Slowing Interactive Views (Priority: P2)

A user with a large or busy stack runs `finfocus --import-cloud-history` once
(for example from a scheduled job or before a review), and later opens the
overview or `finfocus --web`. The long Pulumi Cloud pull happens in the import
command, so the interactive views start as fast as they do without Cloud
history.

**Why this priority**: A first import on a stack with hundreds of updates can
take a minute or more. Making a dashboard or TUI wait for that on startup
would make the opt-in feature feel broken.

**Independent Test**: Run the import command, then start the overview and
`--web` with the Cloud endpoint unreachable. Both start normally and their
totals include the imported identities.

**Acceptance Scenarios**:

1. **Given** Cloud history is enabled and the user runs the import command for
   a stack, **When** it finishes, **Then** the identities for the period are in
   the local history, the command prints how many updates were listed, skipped
   as not state-changing, served from cache, fetched, and unreadable, and how
   many identities were recorded, and no cost is calculated.
2. **Given** the import command is run again over the same period, **When** it
   finishes, **Then** only updates not already stored are fetched.
3. **Given** `enabled` is unset or `false`, **When** the user runs the import
   command, **Then** it exits with an error that names the setting and how to
   enable it, and makes no Cloud request.
4. **Given** the import command cannot reach Pulumi Cloud, **When** it runs,
   **Then** it exits non-zero with the cause, because importing was its only
   purpose.
5. **Given** the user starts the overview or `--web` and has not run the
   import, **When** the views load, **Then** they start without any Cloud
   request and use whatever is stored.

---

### Edge Cases

- A stack has zero updates in the requested period: nothing is imported, no
  error, and the cost report is unchanged.
- An update failed or was canceled partway: its snapshot is still imported,
  because resources it left behind are billed. Only resources present in the
  snapshot are recorded.
- An update is a preview or another kind that cannot change state: it is
  skipped.
- The requested range starts before the history retention window: the start is
  moved to the retention cutoff and a note says so, because older imported
  entries would be removed by the next cleanup.
- A resource appears in an update's snapshot with no physical cloud ID (for
  example a component or provider resource): it is skipped, not recorded.
- The requested period is entirely older than the stack's first update, or
  entirely in the future: no fetch for the missing range, no error.
- A deployment snapshot contains secret or encrypted property values: only
  resource identity fields are extracted and stored; no property values,
  outputs, or secrets are written to the history store, cache, or logs.
- The stack identity (organization, project, stack) cannot be determined, as
  with a stack-less state file or Terraform directory: Cloud history is
  skipped as unavailable, and `enabled: true` reports why.
- The Cloud cache was written by an older finfocus or is corrupt: it is
  discarded and rebuilt, never fatal.
- Two finfocus processes run at once against the same history: neither
  corrupts the store. The process that cannot open the locked store skips the
  import under `auto` with a warning and completes; under `true`, and for the
  import command, it exits 1 naming the lock.
- Cloud history is `auto` or `true` while the resource history store itself is
  disabled (`cost.history.enabled: false` or `FINFOCUS_HISTORY_ENABLED=false`):
  there is nowhere to record identities. Under `auto` the import is skipped
  with a debug note; under `true`, and for the import command, the command
  exits 2 naming both settings.
- The logged-in backend is a plain `http://` URL that is not loopback: it is
  treated as unavailable with its own reason (the backend must use HTTPS), not
  as a non-Cloud backend.
- The user lacks permission to read the stack's updates: treated as
  unavailable (`auto` falls back, `true` fails with a permission message).
- An update is large (thousands of resources): import completes without
  holding every update's snapshot in memory at once.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST determine whether Pulumi Cloud history is available
  before any history request, by checking that the user is logged in, that the
  active backend is Pulumi Cloud (`app.pulumi.com` or a self-hosted Pulumi
  Cloud) and not a file or object-storage backend, and that the token is
  accepted.
- **FR-002**: System MUST fall back to local-only history, without failing the
  command, when `enabled` is `auto` and Pulumi Cloud is unavailable for any
  reason.
- **FR-003**: System MUST retrieve the list of updates for the target stack
  that fall within the requested date range, and for each state-changing update (`update`, `refresh`, `destroy`, or `import`, whatever
  its result) not already stored retrieve the resources recorded in that
  update. Previews and other kinds that cannot change state MUST be skipped.
- **FR-004**: System MUST extract from each retrieved resource only its URN,
  physical cloud ID, type, and provider, and MUST skip resources with no
  physical cloud ID.
- **FR-005**: System MUST record imported identities in the existing resource
  history store with source `pulumi_cloud`, treating each update in which the
  resource appears as one observation at that update's end time, and MUST NOT
  infer existence before the first or after the last observed update. These
  observations select identities for the requested billing query; they do not
  manufacture a billable duration. An
  existing entry for the same URN and cloud ID is extended to cover the
  observation instead of duplicated.
- **FR-006**: System MUST complete the import in `cost actual` before its
  history-enhanced actual-cost query reads the history, so imported identities
  affect that same run. The overview and `finfocus --web` MUST NOT contact
  Pulumi Cloud or wait on an import; they use the stored history only.
- **FR-007**: System MUST cache update lists for one hour and individual
  update snapshots indefinitely, and MUST never fetch again a version whose
  snapshot is already stored, whatever range an earlier run covered. Cached
  observations MUST be replayed idempotently into history, including when
  entries have expired and the retention window is subsequently widened.
- **FR-008**: System MUST keep the Cloud cache separate from the resource
  history entries, and MUST discard and rebuild it when it is unreadable.
- **FR-009**: System MUST limit snapshot retrieval to five concurrent
  requests, MUST honor rate-limit responses by waiting the server-specified
  delay (or a bounded back-off when none is given), and MUST send a
  conditional request for the update list only when the list order lets an
  unchanged first page prove that the whole list is unchanged.
- **FR-010**: System MUST support the setting
  `cost.history.pulumi_cloud.enabled` with values `false` (default), `auto`,
  and `true`, written either as a boolean or as a string, and MUST reject any
  other value at configuration validation with an error listing the allowed
  values. The setting MUST be settable through `config set` and the dashboard
  Config page.
- **FR-011**: With `enabled` unset or `false`, System MUST NOT contact Pulumi Cloud for
  history. With `enabled: true`, System MUST fail the command with an
  actionable error when Cloud history cannot be fully retrieved, before
  reporting any cost totals.
- **FR-012**: System MUST use the saved Pulumi login token by default and MUST
  accept an explicit token override, which takes precedence. The override is
  the `PULUMI_ACCESS_TOKEN` environment variable. finfocus configuration has
  no token key at any tier, so a literal token never sits in a file shared
  with the repository.
- **FR-013**: System MUST send a token only to the resolved Pulumi Cloud
  backend host, MUST NOT follow redirects to another host with it, and MUST
  NOT write the token, or any secret from a snapshot, to logs, error text,
  cache, or history.
- **FR-014**: System MUST work against both `app.pulumi.com` and a
  self-hosted Pulumi Cloud, using the backend URL the user is logged in to.
- **FR-015**: Whenever the import does not run or completes with every update
  read, `cost actual` output MUST be byte-identical to the pre-feature output
  for the same resources, and the exit code MUST be unchanged; the only
  output additions are the incomplete marker of FR-016 and the `--verbose`
  counters of FR-017.
- **FR-016**: System MUST keep partial imports: identities recorded before a
  mid-run failure remain in the store, and a later run resumes from what is
  missing. Under `auto`, when any update could not be read, System MUST print
  a warning on stderr and MUST mark the JSON and human output of that run as
  incomplete Cloud history, with the count of unreadable updates, without
  changing the exit code. Persistence failures count as unreadable updates
  and MUST NOT be reported as successful imports. Incomplete output MUST
  remain machine-readable when there are zero cost rows: JSON uses an
  envelope with `results` and `historyImport`; NDJSON ends with a typed
  `historyImport` status record. Human output always prints the status footer.
- **FR-017**: `cost actual` MUST gain a `--verbose` flag that prints to stderr
  how many updates were listed, skipped, served from cache, fetched, and
  unreadable, and how many identities were recorded. Without the flag these
  counts are logged at debug level only.
- **FR-018**: System MUST apply the existing history retention rules to
  imported entries, so they expire like any other entry, and MUST clamp the
  import range with the same resolved retention period the store's cleanup
  uses (including the `FINFOCUS_HISTORY_RETENTION_DAYS` override).
- **FR-019**: System MUST be fully testable without real Pulumi Cloud access;
  automated tests use a stand-in service and never call a live endpoint.
  Tests of the import enable it explicitly (`auto` or `true`); a separate test
  pins that the unset default makes no request.
- **FR-020**: System MUST provide an import command, the root flag
  `finfocus --import-cloud-history`, that performs
  the import for the resolved stack and a date range, shares the same cache and
  import path as `cost actual`, calculates no cost, and exits non-zero when the
  import cannot run or fails. The date range uses the same date flags as
  `cost actual` and defaults to the existing history retention window.
- **FR-021**: The import command MUST require `enabled` to be `auto` or
  `true`, and with `false` or unset MUST exit with a validation error and make
  no request. It MUST always report the FR-017 counts on completion, without
  needing `--verbose`.

### Key Entities

- **Pulumi Cloud availability**: The outcome of the detection step: available,
  or unavailable with a reason (not logged in, non-Cloud backend, insecure
  backend URL, token rejected, no permission, unreachable, unknown stack,
  history store disabled, history store locked). Drives whether `auto` falls
  back and what `true` reports.
- **Stack update record**: One entry in the stack's update list: version,
  kind, result, start and end time, and resource-change counts. Selected by
  the requested date range.
- **Deployment snapshot**: The full set of resources a stack held after one
  update. Used only to read resource identity.
- **Resource history entry**: The existing record of a URN, cloud ID, type,
  provider, first-seen, and last-seen times, and source. Gains the new source
  value `pulumi_cloud`.
- **Cloud cache**: Locally stored update lists (valid one hour) and update
  snapshots (valid indefinitely). A stored snapshot is the record that a
  version has been read.
- **Cloud history setting**: `cost.history.pulumi_cloud.enabled`. It has no
  token field; the token override is `PULUMI_ACCESS_TOKEN`.

## Web UI Fit *(mandatory)*

- **Disposition**: Extension of the existing Config page. The new setting is
  one more entry in the shared setting list (`config.KnownSettings`), which
  both `config set` and the Config page read, so the page gains an editable
  `cost.history.pulumi_cloud.enabled` row with the three allowed values. The
  Cost page needs no change: it reads the history store, which now also holds
  imported identities. The web never fetches from Pulumi Cloud and has no
  import trigger.
- **Web data**: The Config page row (key, help text, allowed values, global
  and project tier values, effective value), served by the existing
  `GET /api/config` through `config.BuildPage`, and saved by the existing
  `POST /api/config/preview` and `POST /api/config` through
  `config.PatchFile`. The Cost page totals come from the existing
  `enrichWithHistoricalResources` path. See `contracts/http-api.md`.
- **Layout fit**: One row in the Config page's captioned settings table,
  rendered with the enum control the page already uses for `logging.level`.
  No new view, slot, or endpoint.
- **Additions**: None beyond the row; it appears through the setting
  descriptor and needs no `internal/webui` handler change.
- **Render evidence**: A Playwright render of the Config page from a populated
  project shows the row, its three values, and a saved change.
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility). No engine, API, redaction, or data-shape work.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a stack with a resource created and destroyed between
  finfocus runs, the actual-cost report for the period includes that
  resource's identity in the billing query and includes the billing plugin's
  returned cost in 100% of runs where Cloud history is enabled and available.
  This includes an identity observed in exactly one update; no duration beyond
  its observations is inferred by the importer.
- **SC-002**: Users who have not enabled Cloud history, and users without a
  Pulumi Cloud backend, see no change in output, exit code, or measurable run
  time (under 100 ms added) in 100% of runs, and no network traffic to Pulumi
  Cloud.
- **SC-003**: A second run over the same period issues zero repeat
  requests for previously stored update snapshots.
- **SC-004**: A first run over a month with 100 updates completes the import
  in under 60 seconds against a controlled service with 100 identities per
  snapshot, payloads up to 64 KiB each, 100 ms response latency, and aggregate
  throughput of at least 10 MiB/s. This baseline excludes throttling; a
  separate rate-limit test proves retries honor server delays without treating
  those delays as a baseline performance failure.
- **SC-005**: With `enabled: true`, 100% of runs where Cloud history is
  unavailable end with an error naming the cause, and none print cost totals.
- **SC-006**: No token or secret value appears in any log, error message,
  cache, or history file in any tested scenario.
- **SC-007**: The full automated test suite passes with no network access to
  Pulumi Cloud.
- **SC-008**: Overview and `--web` startup time with Cloud history enabled is
  unchanged from startup with it disabled, and neither makes a Pulumi Cloud
  request.

## Assumptions

- The Resource History Store (#934, spec `608-resource-history-store`) is
  already in place, so this feature adds a new source to it rather than
  building storage.
- The stack identity (organization, project, stack) comes from the same
  stack resolution `cost actual` already uses; the active backend and token
  come from the existing Pulumi login discovery, also used by the ESC
  integration.
- A snapshot lists only resources that exist after its update, including
  those a failed or canceled update left in place, so result does not
  decide whether to import; the update kind does.
- The feature is opt-in: the default is `false`, so existing users see no new
  network traffic or latency until they set `auto` or `true`. A later release
  may revisit the default once the import has proven itself.
- Because no lifetime is inferred beyond observed updates, days between a
  resource's creation and the end of its first update, or between its last
  observed update and its destruction, are not asserted as lifetime by this
  source. Billing plugins remain responsible for actual cost over the requested
  period; observation timestamps are evidence for identity selection.
  Reviewed on 2026-10-27 and kept: a resource created before the period and
  destroyed inside it has no in-period observation, because the destroying
  update's snapshot no longer lists it, so this source does not add it. The
  user guide states this limit.
- Only `cost actual` and the import command start a fetch. The overview, the
  web views, `cost history view`, and other readers benefit from stored
  identities but never fetch. `cost history collect` stays unspecified.
- Until the import command or `cost actual` has run, the overview and web
  totals can be lower than `cost actual` for the same period. This is the
  accepted price of never blocking those views.
- Pulumi Cloud rate limits are not published as fixed numbers; the five-request
  concurrency cap and retry behavior are the contract.

## Out of Scope

- Pulling cost data from Pulumi Cloud (only resource identity is read).
- Non-Cloud backends (file, S3, GCS, Azure Blob) having an equivalent history
  import.
- Inspecting or listing stored Cloud history on demand.
- An import trigger or progress display inside the dashboard or overview.
- Any dashboard control for Cloud history beyond the Config page row that
  the setting descriptor provides.
