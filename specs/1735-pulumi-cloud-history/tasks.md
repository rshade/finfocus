---

description: "Task list for Pulumi Cloud Historical State Snapshots"
---

# Tasks: Pulumi Cloud Historical State Snapshots

**Input**: Design documents from `specs/1735-pulumi-cloud-history/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/cli-contract.md, quickstart.md

**Tests**: Per Constitution Principle II, tests are MANDATORY and are written
BEFORE the implementation they cover. Run each new test and watch it fail
before writing the code. Coverage: 80% overall, 95% for `cost actual`'s
history path and `internal/cloudhistory`. Testify only (`require` to stop,
`assert` to continue); never raw `t.Fatalf` or `if got != want`.

**Completeness**: Per Principle VI, no stubs, no TODOs. Every task is a full
implementation.

**Documentation**: Per Principle IV, docs land with the code that changes
them (Phase 9 collects what is left).

**Web Parity**: The spec's Web UI Fit is an extension of the Config page
(Principle VII). The hook is the existing Config handlers, which call
`config.BuildPage` and `config.PatchFile` (`contracts/http-api.md`); no
handler is added. The tasks are the descriptor entry (T038), a handler test
that the row is served and saved (T060), the Playwright render check (T062),
the docs reference (T053), and the UX-only `web-ui` follow-up issue (T063).
T052 pins that the web never imports the new packages.

**Organization**: Grouped by user story. Paths are repo-relative.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an
  unfinished task)
- **[Story]**: US1 to US6, from spec.md
- Unit tests are colocated (`internal/<pkg>/<name>_test.go`). Parallel tests
  call `t.Parallel()` except where CLAUDE.md lists an exception (`t.Setenv`,
  `t.Chdir`, root-command builds, `ResetGlobalConfigForTest`). Tests that touch
  `FINFOCUS_HOME` use `t.Setenv("FINFOCUS_HOME", t.TempDir())`.

---

## Phase 1: Setup

**Purpose**: Documentation the constitution requires first, fixtures, and
package skeletons.

- [X] T001 Add the Pulumi Cloud import cache (buckets `pulumi_cloud_updates`
  and `pulumi_cloud_snapshots` inside `history.db`) to the Persistence Model
  section of `CONTEXT.md`, stating it holds extracted identity only and is
  safe to delete
- [X] T002 Add the same two buckets to the Permitted Stores table in
  `.specify/memory/constitution.md`, bump the version to 1.8.1 (PATCH), update
  `Last Amended`, and add a Sync Impact Report block at the top in the file's
  existing style. Make the change through `/speckit.constitution` so the
  dependent templates are checked too (depends on T001)
- [X] T003 [P] Capture a real update-list response for a Pulumi Cloud test
  stack with `pulumi api` (`GET /api/stacks/{org}/{project}/{stack}/updates`,
  `pageSize=100`), scrub names and tokens, and commit it as
  `internal/pulumicloud/testdata/updates.json`. If no Pulumi Cloud stack is
  available, ask the user for a scrubbed response; do not invent the shape.
  Capture at least two pages and record in the fixture's README note whether
  the list is newest first, because the conditional-request rule depends on it
  (research R9, contract C5)
- [X] T004 [P] Create `internal/pulumicloud/testdata/export.json`: a small
  export-at-version payload with custom and non-custom resources, a `pulumi:`
  provider, a resource with an empty `id`, and properties holding a fake
  secret envelope (`{"4dabf18193072939515e22adb298388d": ...,
  "ciphertext": "SENTINEL-CIPHERTEXT"}`) so tests can prove it is never kept
- [X] T005 [P] Create `internal/pulumicloud/doc.go` and
  `internal/cloudhistory/doc.go` with package godoc, and
  `internal/pulumicloud/README.md` describing the client, its endpoints
  (contract C5), and the identity-only rule

---

## Phase 2: Foundational (blocking)

**Purpose**: Config enum, store changes, cache, writer, login session, client,
decoder, and detection. No user story can start before this phase is done.

**CRITICAL**: Tests first within each pair.

- [X] T006 [P] Write `internal/config/pulumi_cloud_test.go`: `PulumiCloudMode`
  parses the booleans `false` and `true`, the strings `"false"`, `"auto"`,
  and `"true"`, in both YAML and Hujson; boolean and string forms of the same
  value are equal; an absent key is `Off`; `""`, `"yes"`, `"AUTO"`, and `1`
  are rejected with an error that lists the three allowed values; a value
  written as a string round trips as a string (research R17)
- [X] T007 Implement `internal/config/pulumi_cloud.go` (`PulumiCloudMode` with
  `Off`, `Auto`, `Required`, `Validate`, `UnmarshalYAML`/JSON) and add
  `PulumiCloud PulumiCloudHistory` (tag `pulumi_cloud`) to `HistoryConfig` in
  `internal/config/budget.go`; call the validation from `ValidateConfigSource`
  in `internal/config/validate.go` (depends on T006)
- [X] T008 [P] Write tests in `internal/history/entry_test.go` and
  `internal/history/store_test.go`: `SourcePulumiCloud` validates; merging an
  entry with an earlier `FirstSeen` lowers it and a later `LastSeen` raises
  it; merging a state-snapshot write (`FirstSeen = now`) never changes an
  older `FirstSeen`; source follows the last writer
- [X] T009 Add `SourcePulumiCloud = "pulumi_cloud"` to
  `internal/history/entry.go` (`isValidSource` and the validation message) and
  take the minimum `FirstSeen` in `tryUpdateExisting` in
  `internal/history/store.go` (depends on T008)
- [X] T010 [P] Write `internal/history/cloud_cache_test.go`: round trip an
  `UpdateListRecord` and a `SnapshotRecord`; a missing key reports not found;
  a corrupt value reads as absent; snapshot keys sort by version
  (zero-padded, 10 digits); the two buckets exist after `NewBoltStore`; a
  `NewBoltStore` over a file without the buckets creates them
- [X] T011 Implement `internal/history/cloud_cache.go`: bucket constants,
  `UpdateListRecord`, `SnapshotRecord`, the `CloudCache` interface (contract
  C4), and `BoltStore` methods; create the buckets in `initBuckets` in
  `internal/history/store.go`. Do not add methods to the `Store` interface
  (depends on T009, T010)
- [X] T012 [P] Write tests in `internal/history/writer_test.go` for
  `Writer.RecordCloudObservations`: one entry per identity with
  `FirstSeen = LastSeen = observedAt`, source `pulumi_cloud`, no tags; an
  empty cloud ID is skipped with a debug log; a nil or disabled store is a
  error; a store write failure is returned; a repeat call does not duplicate
- [X] T013 Implement `RecordCloudObservations` and the `CloudIdentity` type in
  `internal/history/writer.go`, returning an error for unavailable stores and
  failed writes (depends on T009, T012)
- [X] T014 [P] Write `internal/esc/session_test.go` for `DiscoverSession`:
  backend from `PULUMI_BACKEND_URL`, then the project `backend.url`, then
  `credentials.json` `current`; token from `PULUMI_ACCESS_TOKEN` over the saved
  one for an environment-selected backend or current saved account; a project
  `backend.url` receives only the token saved for that URL (user clarification
  2026-10-27); a project `backend.url` that differs from every saved account gets no
  saved token; an invalid backend returns a `Failure`; `Session.Org` is the
  org of an `org/stack` name and the default organization for a bare `stack`
- [X] T015 Implement `internal/esc/session.go`: exported `Session`
  (`Backend`, `Token`, `Org`) and
  `DiscoverSession(getenv, projectBackend, stackName)` built on the existing
  `discover`, `validateBackend`, and `orgFor`, adding no second credential
  rule. `orgFromStack` stays unexported. Add the constructor to
  `internal/esc/README.md` (depends on T014)
- [X] T016 [P] Write `internal/pulumicloud/identity_test.go` against
  `testdata/export.json`: only custom, non-`pulumi:`, non-empty-`id`
  resources are returned; the result type has exactly URN, CloudID, Type,
  Provider; the sentinel ciphertext appears nowhere in the decoded values or
  in `fmt.Sprintf("%+v")` of the result; malformed JSON returns an error; a
  body over the size cap returns an error without reading it all
- [X] T017 Implement `internal/pulumicloud/identity.go`: stream-decode into a
  struct limited to `deployment.resources[].{urn,id,type,custom}` through an
  `io.LimitReader` capped at 256 MiB, deriving `Provider` with
  `resourcetype.ExtractProvider` (depends on T016)
- [X] T018 [P] Write `internal/pulumicloud/client_test.go` with `httptest`:
  `Authorization: token <t>` and `Accept: application/vnd.pulumi+8` headers;
  list paging from page 1 at `pageSize=100` until a page adds no new version;
  `testdata/updates.json` decodes (nested and flat field forms) and its order
  matches the note T003 recorded; 401, 403, 404, 400 map to typed errors; a
  redirect to another host is not followed and the token never reaches it;
  error text contains no token, userinfo, or query
- [X] T019 Implement `internal/pulumicloud/client.go`: `Client` interface and
  `NewClient(session esc.Session, httpClient *http.Client)`, `StackRef`,
  `UpdateRecord`, `UpdateList`, typed errors (`ErrUnauthorized`,
  `ErrForbidden`, `ErrNotFound`, `RateLimitError{RetryAfter}`), no-redirect
  transport, and `ExportIdentities` using T017 (depends on T015, T017, T018)
- [X] T020 [P] Write `internal/pulumicloud/detect_test.go`: `GET /api/user`
  2xx is available; 401 is `token_rejected`; 403 is `no_permission`; a network
  error is `unreachable`; a file, S3, GCS, or Azure Blob backend returns
  `non_cloud_backend` with no request; an `http://` backend that is not
  loopback returns `insecure_backend` with no request, while
  `http://127.0.0.1` is accepted; a missing token is `not_logged_in`; an
  unknown org or project is `unknown_stack`; reasons are checked in the
  contract C5 order
- [X] T021 Implement `internal/pulumicloud/detect.go`: `Availability`,
  `Reason` constants (including `insecure_backend`), and
  `Detect(ctx, session, stack)`, classifying the backend scheme itself rather
  than collapsing every `esc` backend failure into `non_cloud_backend`
  (depends on T019, T020)

**Checkpoint**: Foundation ready. Run
`go test ./internal/config/... ./internal/history/... ./internal/esc/... ./internal/pulumicloud/...`.

---

## Phase 3: User Story 1 - Complete Resource Timeline (Priority: P1) MVP

**Goal**: With Cloud history enabled, `cost actual` imports identities from
state-changing updates inside the period so short-lived resources appear.

**Independent Test**: Against the stand-in service, with `enabled: auto` and
an empty history, run `cost actual` over a period in which a resource was
created and destroyed. The resource's cloud ID is in the history with source
`pulumi_cloud` and in the resource list handed to the engine.

### Tests for User Story 1 (write first, confirm they fail)

- [X] T022 [P] [US1] Write `internal/cloudhistory/import_test.go` with a real
  `BoltStore` and a stand-in `pulumicloud.Client`: identities from an
  in-range `update`, `refresh`, `destroy`, and `import` are recorded; a
  `preview` and `rename` are skipped and counted; a `failed` and a `canceled`
  update are imported; an identity in one update has equal first and last
  seen at that update's end time; an identity in updates 3, 4, 5 spans the
  end of 3 to the end of 5; an existing state-snapshot entry for the same URN
  and cloud ID is extended, not duplicated; a zero-update list records
  nothing; updates outside `[from, to]` are not fetched
- [X] T023 [US1] Add range tests to the same file: a `from` earlier than
  `now - Request.RetentionDays` is moved to the cutoff and `Result.Clamped`
  is true; the clamp uses `Request.RetentionDays` and never reads the config
  (a request with 10 days clamps at 10 even when the config says 90); a range
  wholly in the future fetches nothing; a range before the first update
  fetches nothing
- [X] T024 [P] [US1] Write `internal/cli/cost_actual_cloud_history_test.go`
  (build the root through `ax.Execute`, `FINFOCUS_HOME` in a temp dir,
  `httptest` service via `PULUMI_BACKEND_URL` and `PULUMI_ACCESS_TOKEN`,
  `enabled: auto` in the project config): after `cost actual` the history
  holds the destroyed resource's cloud ID and the descriptors passed to the
  engine include it; the import ran before `enrichWithHistoricalResources`;
  with `FINFOCUS_HISTORY_RETENTION_DAYS=10` and `--from` 30 days ago, the
  import's range starts 10 days ago; for a resource observed exactly once,
  assert equal first/last timestamps, the full requested billing range passed
  to the stand-in plugin, and inclusion of its known returned cost in totals

### Implementation for User Story 1

- [X] T025 [US1] Implement `internal/cloudhistory/result.go`: `Result` with
  `Listed`, `Skipped`, `FromCache`, `Fetched`, `Recorded`, `Unreadable`,
  `Clamped`, an `Incomplete()` method, and JSON tags matching contract C3
- [X] T026 [US1] Implement `internal/cloudhistory/import.go`: `Deps`,
  `Request` (with `RetentionDays`), `Import`; range clamp to the retention
  cutoff computed from `Request.RetentionDays`; keep updates of
  kind `update`, `refresh`, `destroy`, `import` inside the range; skip the
  network export for a version with a `SnapshotRecord` and replay its cached
  identities; otherwise fetch identities,
  store a `SnapshotRecord`, and call `Writer.RecordCloudObservations` at the
  update end time (sequential for now; US4 adds the pool) (depends on T013,
  T021, T022, T023, T025)
- [X] T027 [US1] Wire the import into `internal/cli/cost_actual.go`: after
  `ParseTimeRange` and before `enrichWithHistoricalResources`, resolve the
  stack (`pulumi.LoadStackContext`, then `esc.DiscoverSession` with the stack
  name, using `Session.Org`) and call `cloudhistory.Import` when the mode is
  not `Off`. In `internal/cli/history_record.go`, split the retention
  resolution (env, then config, then 90) into a helper that both the store
  opener and the import use, and add a helper that builds `Deps` and
  `Request.RetentionDays` from the opened store, which must also satisfy
  `CloudCache` (depends on T026, T024)

**Checkpoint**: US1 works end to end with `enabled: auto` against the stand-in.

---

## Phase 4: User Story 2 - Fallback When Unavailable (Priority: P1)

**Goal**: Under `auto`, anything that stops the import leaves `cost actual`
exactly as it is today, and a partial import is reported honestly.

**Independent Test**: With `enabled: auto`, run `cost actual` on a file-backed
stack, with no login, with a rejected token, and with the service dropping
connection mid-fetch. Output and exit code match the pre-feature run, except
that the partial case adds the stderr warning and the `historyImport` marker.

### Tests for User Story 2 (write first)

- [X] T028 [P] [US2] Extend `internal/cloudhistory/import_test.go`: under
  `auto`, each unavailable reason returns no error and records nothing; a
  snapshot that fails after retries leaves earlier identities in the store,
  counts `Unreadable`, sets `Incomplete()`, and a second run fetches only the
  missing versions; injected cache/history write failures count once per
  update, preserve earlier writes, and return incomplete under `auto` and an
  error under `true` or the import command; a second run replays any cached
  but unrecorded observations without exporting them again; a `history.db` lock (`ErrHistoryLocked`, nil store) skips
  with a warning and reason `history_locked`; a disabled store
  (`cost.history.enabled: false`) skips at debug level with reason
  `history_disabled`; both make zero requests to the stand-in (research R18)
- [X] T029 [P] [US2] Write tests in `internal/engine/project_test.go` and
  `internal/engine/types_test.go`: `CostResult.HistoryImport` is omitted from
  JSON and NDJSON when nil; present as
  `{"incomplete":true,"unreadableUpdates":N}` on every row when set; the table
  renderer prints one footer line naming the count; output with the field nil
  is byte-identical to today's golden output; incomplete JSON uses the C3
  envelope and incomplete NDJSON ends with the C3 typed status record,
  including with zero rows
- [X] T030 [P] [US2] Extend `internal/cli/cost_actual_cloud_history_test.go`:
  a file-backed stack makes zero requests to the stand-in service; no login
  and a 401 token produce unchanged stdout and exit code with the reason only
  at debug level; a mid-run failure prints a stderr warning, marks rows in
  JSON, and leaves the exit code unchanged; with zero rows, JSON still has
  the status envelope, NDJSON the final status record, and human output the
  footer

### Implementation for User Story 2

- [X] T031 [US2] Implement the fallback and partial-failure paths in
  `internal/cloudhistory/import.go`: an `Availability` reason under `auto`
  returns a `Skipped` outcome with the reason; a failed version increments
  `Unreadable` and the loop continues; cache/history write failures follow
  that same path and are counted once per update; the store check (disabled or locked)
  runs before detection and is reported, not fatal (depends on T028)
- [X] T032 [US2] Add `HistoryImport *HistoryImportStatus` (tag
  `historyImport,omitempty`) to `CostResult` in `internal/engine/types.go` and
  render it in `internal/engine/project.go` for table (footer), JSON, and
  NDJSON, including the C3 envelope/final status record and zero-row footer
  (depends on T029)
- [X] T033 [US2] In `internal/cli/cost_actual.go`, copy an incomplete
  `Result` onto every result row, print the stderr warning, and log the
  skipped reason at debug (depends on T027, T031, T032, T030)

**Checkpoint**: US1 and US2 establish `auto` behavior for development
verification. The merge boundary remains completion of all stories and gates.

---

## Phase 5: User Story 3 - Control Over the Setting (Priority: P2)

**Goal**: `false` (and absent) never contacts Pulumi Cloud; `true` fails
instead of reporting possibly short totals; bad values are rejected everywhere.

**Independent Test**: Run `cost actual` with each value against a reachable
and an unreachable stand-in and compare against contract C2.

### Tests for User Story 3 (write first)

- [X] T034 [P] [US3] Write `internal/cli/cost_actual_cloud_mode_test.go`: an
  unset setting and `false` make zero requests with a logged-in token present
  and leave output and exit code unchanged (pins the opt-in default); `true`
  with an unreachable service exits 1 with the reason and prints no totals;
  `true` with a rejected token exits 1; `true` with a file backend exits 2
  with `Cloud history requires a Pulumi Cloud backend`; `true` with a
  non-loopback `http://` backend exits 1 with `must use HTTPS`; `true` with
  one unreadable update exits 1; `true` with `cost.history.enabled: false`
  exits 2 naming both settings; `true` with a locked `history.db` exits 1
  naming the lock
- [X] T035 [P] [US3] Extend `internal/config/validate_test.go` and add a
  `config set` test in `internal/cli/config_set_test.go`: `config validate`
  and a `cost` pre-run reject `enabled: maybe` and list the allowed values;
  `config set cost.history.pulumi_cloud.enabled auto` patches `config.hujson`
  through `config.PatchFile` and keeps comments; an invalid value is a
  validation error (exit 2); `config get cost.history.pulumi_cloud.enabled`
  prints `false` when unset and `false` when the file holds the boolean
  `false`

### Implementation for User Story 3

- [X] T036 [US3] Implement `Required` semantics in
  `internal/cloudhistory/import.go`: any `Availability` reason, a locked
  store, or any `Unreadable` count returns a typed
  `*RequiredError{Reason, Fix}`; the `non_cloud_backend` and
  `history_disabled` reasons carry the exit-2 marker (depends on T034)
- [X] T037 [US3] Map the errors in `internal/cli/cost_actual.go`: a
  `*RequiredError` to exit 1 with the message, the exit-2 cases through
  `toValidationError`; make sure no totals print first (depends on T033,
  T036)
- [X] T038 [US3] Add the setting to `config.KnownSettings` in
  `internal/config/descriptor.go` (`Key: "cost.history.pulumi_cloud.enabled"`,
  `Kind: KindEnum`, `Enum: []string{"false", "auto", "true"}`, a one-line
  `Help`), return `"false"` for it from `defaultValue` in
  `internal/config/view.go`, and make the Config page and `config get` show a
  stored boolean as its string. `config.LookupSetting` gates both
  `config set` (`config.PatchFile`) and the Config page, so no `internal/cli`
  allowlist change is needed. Confirm the project tier may set it (depends on
  T035)
- [X] T060 [P] [US3] Extend `internal/webui/handlers_config_test.go` with a
  `config.BuildPage`-backed
  `ConfigView`: `GET /api/config` lists `cost.history.pulumi_cloud.enabled`
  with kind `enum`, the three values, and effective value `"false"`;
  `POST /api/config/preview` with `auto` returns a diff; `maybe` returns 400
  (contract `http-api.md`) (depends on T038)

---

## Phase 6: User Story 4 - Fast Repeat Runs, Respectful API Use (Priority: P2)

**Goal**: Repeat runs reuse stored data, first runs on large stacks stay
inside the rate limits.

**Independent Test**: Run the import twice over 100 updates against a
stand-in that counts requests: the second run makes no snapshot request, the
peak in-flight count never exceeds five, and a 429 with `Retry-After` is
waited out.

### Tests for User Story 4 (write first)

- [X] T039 [P] [US4] Extend `internal/pulumicloud/client_test.go`: a 429 with
  `Retry-After: 2` waits that long (inject a clock/sleep seam) and retries; a
  503 with no header waits a bounded jittered delay; three failures return
  `RateLimitError`; a canceled context stops the wait; when the fixture order
  is newest first, the list request sends `If-None-Match` when given an ETag
  and a 304 returns the caller's list; otherwise no `If-None-Match` is ever
  sent (contract C5)
- [X] T040 [P] [US4] Extend `internal/cloudhistory/import_test.go`: with 100
  updates the stand-in's peak concurrent exports is at most 5 (use a gate
  channel); a second run over the same range makes no export request
  (`FromCache` equals the imported-kind count); a list fetched under an hour
  ago is not re-requested; one older than an hour is refreshed with the stored
  ETag, a 304 keeps the stored list and resets `fetched_at`, a 200 replaces it;
  a snapshot record written by an older run is reused for a later, wider range;
  after retention cleanup removes history entries, widen retention and prove
  cached observations restore them with zero exports; against the SC-004
  stand-in with 100 updates, 100 identities and at most 64 KiB per snapshot,
  100 ms response latency and at least 10 MiB/s aggregate throughput, the
  first import finishes in under 60 s of wall time and the second makes zero
  export requests (SC-003, SC-004)

### Implementation for User Story 4

- [X] T041 [US4] Implement retry, `Retry-After`, jittered back-off, and
  `If-None-Match` in `internal/pulumicloud/client.go`, using the caller's
  context and an injectable sleep (depends on T039)
- [X] T061 [P] [US4] Write a test in
  `internal/cli/cost_actual_cloud_history_test.go`: `cost actual --verbose`
  prints the six C1 counters to stderr in C1 order; without `--verbose` they
  are absent from stderr and present in the debug log; stdout is identical
  with and without the flag
- [X] T042 [US4] In `internal/cloudhistory/import.go` fetch snapshots through
  `errgroup.WithContext` with `SetLimit(5)`, read and write the
  `UpdateListRecord` with the one-hour freshness rule, and report the six
  counters through `Result`. In `internal/cli/cost_actual.go` declare a local
  `--verbose` bool (research R16) that prints them to stderr, and log them at
  debug level otherwise (depends on T040, T041, T061)

---

## Phase 7: User Story 5 - Self-Hosted and Token Override (Priority: P3)

**Goal**: Same behavior on a self-hosted Pulumi Cloud and with a service
token, with the token confined to its backend host.

**Independent Test**: Point the session at an `httptest.NewTLSServer` as a
self-hosted backend with `PULUMI_ACCESS_TOKEN` set, run the import, and scan
every sink for the token.

### Tests for User Story 5 (write first)

- [X] T043 [P] [US5] Write `internal/cloudhistory/security_test.go`: a
  self-hosted backend (TLS test server, its CA in the client) imports
  normally; `PULUMI_ACCESS_TOKEN` wins over the saved token; a redirect from
  the backend to a second server never delivers the token; a project
  `backend.url` that is not the saved account's backend never receives the
  saved token; after a run that fails with the token in scope, the token and
  the sentinel ciphertext are absent from captured zerolog output, returned
  error strings, the `Result` JSON, the history file bytes, and both cache
  buckets
- [X] T044 [US5] Fix whatever T043 finds in `internal/pulumicloud/client.go`,
  `internal/pulumicloud/detect.go`, or `internal/esc/session.go`. If T043
  passes unchanged, record that in the commit notes and add no code
  (depends on T043)

---

## Phase 8: User Story 6 - Pre-Load Without Slowing Interactive Views (Priority: P2)

**Goal**: `finfocus --import-cloud-history` does the long pull; the overview
and `--web` never contact Pulumi Cloud.

**Independent Test**: Run the command, then start the overview and the web
data path with the stand-in stopped: both start without Cloud requests. The
web Cost path includes imported identities; overview keeps its current
state/plan inventory (user clarification, 2026-10-27).

### Tests for User Story 6 (write first)

- [X] T045 [P] [US6] Write `internal/cli/import_cloud_history_test.go`
  (through `ax.Execute`): unset or `false` exits 2 with the contract message
  and makes no request; a run prints the six counters in the contract order
  and, with `--format json`, the `ImportResult` object; a second run reports
  `Fetched: 0`; an unreachable service exits 1 with the cause; any unreadable
  update exits 1; `--from` older than the retention window prints the clamp
  note; `cost.history.enabled: false` exits 2 naming both settings; a locked
  `history.db` exits 1 naming the lock; `--from` or `--to` without the flag
  exits 2; combining the flag with
  `--web`, `--mcp`, or a subcommand exits 2; the command starts no plugin and
  calculates no cost
- [X] T046 [P] [US6] Write `internal/cli/web_data_cloud_history_test.go` and an
  overview-pipeline test: with `enabled: auto` and the stand-in server
  counting requests, building the web data path and the overview pipeline
  makes zero requests; the web Cost path includes identities stored earlier
  by the import, while overview keeps its state/plan inventory (user
  clarification, 2026-10-27).
  For SC-008, compare the time to build each with `enabled: auto` against
  `enabled: false` over the same fixture (median of 5 runs) and require the
  difference to stay under 100 ms

### Implementation for User Story 6

- [X] T047 [US6] Implement `internal/cli/import_cloud_history.go`: flag
  registration, `--from`/`--to` parsing with the `cost actual` date helpers,
  the default range, the `enabled` rule (FR-021), the summary printer, and
  exit-code mapping per contract C1 (depends on T037, T042, T045)
- [X] T048 [US6] Register the flag and its validation in
  `internal/cli/root.go` and `rootRunE` next to `registerWebFlags`: route to
  T047 after the `lifecycle.serving` and `--web` checks, reject the
  combinations in T045 through `toValidationError` (depends on T047)
- [X] T049 [US6] Regenerate the `__schema` golden with `UPDATE_GOLDEN=1`, read
  the diff, and confirm `internal/cli/testdata/mcp/tools.golden` is unchanged
  because the root is not a tool (depends on T048)
- [X] T050 [US6] Confirm by test (T046) that `internal/cli/web_data.go` and
  `internal/cli/overview_pipeline.go` call no `cloudhistory` function; if they
  do, remove the call (depends on T046)

---

## Phase 9: Polish and Cross-Cutting

**Purpose**: Boundaries, docs, and the quality gates.

- [X] T051 [P] Update `docs/src/content/docs/guides/resource-history.md`: the
  `pulumi_cloud` source, opt-in setting, `--import-cloud-history`, the
  retention clamp (including `FINFOCUS_HISTORY_RETENTION_DAYS`), that
  overview makes no Cloud requests and the web Cost page reads what is stored,
  the Config page row, that the
  feature needs `cost.history.enabled`, `cost actual --verbose`, and the known
  limit kept on 2026-10-27 (a resource created before the period and
  destroyed inside it is not added); state that the token override is the
  `PULUMI_ACCESS_TOKEN` environment variable (no config key)
- [X] T052 [P] Write `internal/cloudhistory/boundary_test.go`: `go list -deps`
  shows `internal/webui` and `internal/history` import neither
  `internal/pulumicloud` nor `internal/cloudhistory`, and
  `internal/pulumicloud` does not import `internal/engine`
- [X] T053 [P] Update `docs/src/content/docs/reference/config-reference.md`
  (the `pulumi_cloud.enabled` row, default `false`, boolean or string form,
  settable from `config set` and the Config page) and
  `docs/src/content/docs/reference/cli-commands.md` (the root flag, its exit
  codes, `cost actual --verbose`, and the `historyImport` field). If the web
  Config page has its own docs section, list the new row there
- [X] T062 Playwright render check (Principle VII): build `bin/finfocus`, run
  `PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port <port>` from
  a populated project (`initial-30` or `../finfocus-demo`), open the Config
  page with playwright-mcp, and confirm the `cost.history.pulumi_cloud.enabled`
  row shows the enum control with `false`, `auto`, `true` and effective value
  `false`; save `auto` on the project tier and confirm the row and
  `config.hujson` both show it. Save screenshots under `.playwright-mcp/`
  only, then stop the server with an anchored `pkill -f '^<path>/finfocus --web'`
  (depends on T038, T060)
- [X] T063 After the CLI feature is complete, file one GitHub issue labeled
  `web-ui` for the Config page row, scoped to layout, rendering, interaction,
  copy, and accessibility only (for example help-text wording and how
  `false`/`auto`/`true` read in the control). No engine, API, redaction, or
  data-shape work; anything of that kind found later goes back into this
  feature (depends on T062)
- [X] T064 [P] Write a CLI test for SC-002 in
  `internal/cli/cost_actual_cloud_mode_test.go`: with `enabled` unset and
  with a file backend under `auto`, time the import helper T027 added (median
  of 5 calls) and require under 100 ms, with no client constructed and zero
  requests to a stand-in listening on `PULUMI_BACKEND_URL`
- [X] T054 Verify godoc on every exported symbol in `internal/pulumicloud`,
  `internal/cloudhistory`, the new `internal/history` and `internal/esc`
  symbols, and the README example in `internal/pulumicloud/README.md`
  compiles
- [X] T055 Run `go test -race -shuffle=on -count=3` on
  `./internal/pulumicloud/... ./internal/cloudhistory/... ./internal/history/...
  ./internal/esc/... ./internal/config/...` and `./internal/cli/` for the new
  tests; fix any order dependence (a test that fails alone with
  `HOME=$(mktemp -d)` is order-dependent)
- [X] T056 Check coverage with `go test -cover` for the new packages and
  `cost_actual.go`'s history path against the 80% and 95% targets; add tests
  for any gap
- [X] T057 Run `make lint`, `make test`, `make validate`, and
  `markdownlint` on every changed Markdown file; fix findings. Run
  `go vet ./...` first (a widened interface breaks other doubles)
- [X] T058 Run the `quickstart.md` flow against the stand-in end to end and,
  with the user's Pulumi Cloud stack, once for real: enable, import, `cost
  actual`, then disable and confirm zero requests
- [X] T059 Replace the "(planned)" 1735 entry in `CLAUDE.md` with the durable
  rules learned (opt-in default, export endpoint, identity-only cache, the
  `FirstSeen` minimum, retention clamp from the resolved retention value,
  overview/web never fetch, the `KnownSettings` entry being the only gate for
  `config set` and the Config page, boolean-or-string enum parsing, and the
  `update-agent-context.sh` script damaging `CLAUDE.md`), keeping the file's
  style and no machine paths

---

## Phase 10: Follow-up - Official Pulumi Cloud SDK (R2 open decision)

**Purpose**: Replace the hand-written `restClient` with
`github.com/pulumi/pulumi-cloud-sdk/go/apiclient` so upstream API changes
arrive through a dependency bump. `Client`, `DecodeIdentities`, the cache, and
the CLI stay as they are. Not part of the merge gate for the opt-in release;
run it after Phase 9 or as its own PR. The existing `client_test.go` and
`detect_test.go` are the contract: they must pass unchanged.

- [X] T065 Spike, no production edit: add the SDK at the latest release in a
  scratch module and run `ExportStackAtVersion` through an `Executor` against
  `internal/pulumicloud/testdata/export.json` and a generated 256 MiB-class
  fixture. Record peak resident memory and whether `Executor` can wrap
  `resp.Body` in `io.LimitReader` (256 MiB) before the SDK's `io.ReadAll`.
  Write the result in `research.md` R2 under "Open decision"
- [X] T066 Gate on T065. If the bounded buffered read is acceptable, migrate
  both calls. If not, keep `ExportIdentities` on the current streaming path
  (same transport and token rules) and migrate only `ListUpdates` and
  `detect.go`'s `GET /api/user` (`GetCurrentUser`). Record the choice in R2
  (depends on T065)
- [X] T067 [P] Add tests to `internal/pulumicloud/client_test.go` that pin what
  the SDK does not do, written first and failing against any naive wrapper: a
  `304` becomes `UpdateList{NotModified: true}` and not an unmarshal error; the
  `ETag` is read from the first page; `429` and `503` retry up to the shared
  three attempts and honor `Retry-After` through `APIError.ResponseHeader()`;
  a redirect is not followed and the token never reaches another host; an
  `http://` non-loopback backend is refused before any request; error text has
  no token, userinfo, or query; export errors map to the typed errors; stack
  names with spaces and slashes are escaped; a backend with a path prefix keeps
  it. Most already existed in `client_test.go`; T067 added the gaps as contract
  tests that pass on the REST client and must pass unchanged on the SDK. The
  export byte-cap test needs the limit seam T068 creates, so T068 writes it
  first
- [X] T068 Implement `Executor` in `internal/pulumicloud/client.go`: set
  `Authorization: token <t>` from `esc.Session`, use the no-redirect
  `http.Client`, validate the backend scheme and host as `get` does today,
  retry with `retryDelay`, capture `ETag` and `304` into the call's result, and
  bound the body. Build `apiclient.CloudClient{BaseURL, Executor}` in
  `NewClient`; map `*apiclient.APIError` to `ErrUnauthorized`, `ErrForbidden`,
  `ErrNotFound`, and `RateLimitError` with no body text (depends on T066, T067)
- [X] T069 Replace `decodeUpdatePage` input: decode the SDK's `*any` result (or
  re-marshal it) through the existing nested-or-flat field rules and the
  `{"updates":null}` end marker. Keep `wireUpdateInfo`. `testdata/updates.json`
  must decode identically (depends on T068)
- [X] T070 Remove the code the SDK replaced (`stackPath`, `get`, the status
  mapping in `serviceError`) only after T067 passes unchanged. The code removal
  landed with T068 and T069, since the unused `get` and `stackPath` fail lint;
  `serviceError` stays as the status mapping. Remaining: the README. Keep
  `retryDelay`, `sleepContext`, and `isLoopback`. Update
  `internal/pulumicloud/README.md` and its compiling example (depends on T069)
- [X] T071 [P] Dependency hygiene: confirm the SDK's Go requirement is at or
  below the root `go.mod`, run `go mod tidy -diff`, check `renovate.json` so a
  date-versioned module (`v1.YYYYMMDD.N`) is grouped and not opened as a daily
  PR, run `govulncheck` if the repo has it, and add a one-line note to
  `CLAUDE.md` that `internal/pulumicloud` wraps the official SDK and the
  `Executor` owns the credential and retry rules (depends on T068)
- [X] T072 Run `go vet ./...`, `go test -race -shuffle=on -count=3
  ./internal/pulumicloud/... ./internal/cloudhistory/...`, `make lint`,
  `make test`, `make validate`, and `markdownlint` on changed Markdown. Repeat
  the T058 real-stack import once against the user's Pulumi Cloud stack and
  confirm the same 4 updates, 113 identities, and an empty repeat run
  (depends on T070, T071)

---

## Dependencies and Execution Order

### Phase dependencies

- Phase 1 (Setup): none. T001 before T002.
- Phase 2 (Foundational): after Phase 1. T003 and T004 feed T018 and T016.
- Phases 3 to 8: after Phase 2.
- Phase 9: after all six stories; required before merge.
- Phase 10 (SDK follow-up): after Phase 9. Optional for the opt-in release;
  T065 gates T066, T067 can run beside T065, then T068 to T072 in order.

### Story dependencies

- **US1** (P1): needs Phase 2 only. This is the MVP.
- **US2** (P1): builds on US1's `Import` and the `cost actual` wiring
  (T026, T027).
- **US3** (P2): builds on US2 (T033) for error mapping; the config enum is
  already in Phase 2. T038 (descriptor) comes before T060 (Config handler
  test).
- **US4** (P2): independent of US2 and US3; changes `import.go`, the client,
  and `cost_actual.go` (`--verbose`, T061 then T042), so sequence it after US1
  to avoid conflicts.
- **US5** (P3): independent; its tests pass or drive small fixes in Phase 2
  code.
- **US6** (P2): needs US3's error mapping (T037) and US4's counters (T042).

### Within a story

Tests first and failing, then types, then the orchestrator, then the CLI.

### Parallel opportunities

- Phase 1: T003, T004, T005.
- Phase 2: the eight test tasks T006, T008, T010, T012, T014, T016, T018, T020
  touch different packages and can be written together; implementations that
  share `internal/history` (T009, T011, T013) run in order.
- US1 tests T022 and T024; T023 follows T022 in the same file; US2 tests T028, T029, T030; US3 tests T034,
  T035; US4 tests T039, T040, T061.
- US5 can run alongside US4 by a second person.
- Polish: T051, T052, T053, T064. T062 needs a built binary and T063 runs
  last, once the feature is complete.

### Parallel example: Phase 2 tests

```text
Task: internal/config/pulumi_cloud_test.go            (T006)
Task: internal/history/{entry,store}_test.go          (T008)
Task: internal/history/cloud_cache_test.go            (T010)
Task: internal/esc/session_test.go                    (T014)
Task: internal/pulumicloud/identity_test.go           (T016)
```

---

## Implementation Strategy

### MVP first (User Story 1)

1. Phase 1 and Phase 2.
2. Phase 3. Stop and run the US1 independent test with `enabled: auto`.
3. This is an internal development checkpoint, not a merge boundary. Before
   merge, complete all six stories, Phase 9 quality gates, the Config handler
   coverage and Playwright render, and the UX-only `web-ui` follow-up issue.
   The opt-in default does not waive constitutional requirements.

### Incremental delivery

1. Develop US1, then US2 so `auto` handles failures correctly.
2. US3 for the `true` guarantee, US4 so big stacks are usable.
3. US6 so interactive views never wait.
4. US5 hardens the credential and self-hosted paths.

### Notes

- The plan's risk 2 (resources that predate the period) is not built. The
  user reviewed it on 2026-10-27 and kept the observation-only rule; T051
  documents the limit.
- Commit only when asked. Task IDs T001-T059 are the order of execution.
  T060-T064 were added by `/speckit.analyze`; each runs where its phase
  places it and its `depends on` note says.
- Each task ends with the new tests passing; the full gate is T057.

## Implementation handoff (2026-10-27)

- Phases 1 through 8 and the polish tasks are implemented on this branch.
- The live Pulumi Cloud update list ends with `{"updates":null}`. A deployment
  snapshot with no `resources` list is an empty snapshot.
- `initial-30` / `dev` imported 4 in-range updates, recorded 113 identities,
  and a repeat run fetched nothing. One snapshot has manifest metadata only.
- UX follow-up: GitHub issue 1912.
