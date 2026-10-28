# Research: Pulumi Cloud Historical State Snapshots

Phase 0 findings for spec `1735-pulumi-cloud-history`. Every unknown in the
Technical Context is resolved here. Endpoint facts come from Pulumi's
published OpenAPI document (`https://api.pulumi.com/api/openapi/pulumi-spec.json`,
read 2026-10-27), and code facts from this repository at `main`.

## R1. Which endpoint returns a historical snapshot

- **Decision**: List with `GET /api/stacks/{org}/{project}/{stack}/updates`
  and read each snapshot with
  `GET /api/stacks/{org}/{project}/{stack}/export/{version}`
  (`ExportStackAtVersion`).
- **Rationale**: Issue #935 says `GET .../updates/{version}` returns the full
  deployment. The OpenAPI document shows that path is `GetStackUpdate`, which
  returns update metadata. The deployment (`UntypedDeployment`) comes from the
  `export/{version}` path.
- **Alternatives considered**: Calling `.../updates/{version}` as written in
  the issue (wrong payload). Shelling out to `pulumi stack export --version`
  (see R2).
- **Details**:
  - `GetStackUpdates` takes `page` (0-indexed, `0` returns everything) and
    `pageSize` (default 10, ignored when `page` is 0). The default of 10 is the
    same trap `cost history collect` already hit.
  - Requests need `Authorization: token <token>` and
    `Accept: application/vnd.pulumi+8`.
  - `ExportStackAtVersion` answers 400 for a bad version and 404 for a missing
    one. Its `deployment` field is opaque JSON and contains secret ciphertext.

## R2. REST client or the existing Pulumi CLI exporter

- **Decision**: A small REST client in a new `internal/pulumicloud` package.
- **Rationale**: `cost history collect` already reads per-version exports
  through `pulumiExporter` (`internal/cli/cost_history_collect.go`), which runs
  `pulumi stack export --version N`. That works, but it needs the `pulumi`
  binary on the path, spawns one process per version, cannot send conditional
  requests, cannot honor a `Retry-After`, and offers no token override. The
  spec requires the last three (FR-009, FR-012), and CI runners often have a
  token and no CLI.
- **Alternatives considered**: Reuse `pulumiExporter` (less code, fails the
  spec's rate-limit and token requirements). Use the official generated Pulumi
  Cloud SDK, `github.com/pulumi/pulumi-cloud-sdk/go/apiclient`. An earlier
  version of this entry said no generated client covers stack export. That was
  wrong: the SDK is generated from the published OpenAPI spec
  (`https://api.pulumi.com/api/openapi/pulumi-spec.json`) and has
  `GetStackUpdates` and `ExportStackAtVersion`. The `pulumi-pulumiservice`
  provider is not an alternative: its `pulumiservice:api/*` resources are also
  generated from that spec but manage resources and are still preview.
- **SDK fit** (read from `v1.20261027.1310`; the module is date-versioned, needs
  Go 1.26.6, and adds `go-jose`, `blang/semver`, and `yaml.v3`):
  - `CloudClient{BaseURL, Executor, Interceptor}`. Every request goes through
    `Executor`, so the SDK does not set auth in Go. Redirect refusal, the HTTPS
    and loopback check on the backend, and the token all stay ours, set there.
  - It has no retry. A 4xx or 5xx becomes `*APIError`, which keeps the status
    and `ResponseHeader()`, so `Retry-After` is readable. Retry and the shared
    three-attempt budget stay ours, either in `Executor` or around the call.
  - It sends `Accept: application/vnd.pulumi+8` by default, as we do.
  - `GetStackUpdates` returns `*any`, so the page still needs hand decoding.
    `ExportStackAtVersion` returns `AppUntypedDeployment` with `deployment` as
    `json.RawMessage`, read with `io.ReadAll` and `json.Unmarshal`. There is no
    size bound and no streaming, so the whole deployment, secret ciphertext
    included, sits in memory before our identity decoder runs. (Today's
    `DecodeIdentities` is not a stream either: `json.Decoder.Decode` reads the
    whole top-level value first. The T065 spike below measures both.) Neither SDK method returns
    response headers, so a conditional request cannot read `ETag` from them.
  - Conditional requests work only through `Executor`: `If-None-Match` goes in
    through `extraHeaders`, but a `304` is a status below 400, so the SDK
    treats it as a body and fails `json.Unmarshal` on an empty one. `Executor`
    must record the `ETag` and turn a `304` into a sentinel itself.
- **Open decision**: The maintainer prefers the SDK, so the client inherits
  upstream API changes. Moving to it removes URL building and the error
  envelope parse, and keeps retry, ETag, `304`, the token and redirect rules,
  and the size bound in an `Executor` wrapper. For the export it also needs
  the unbounded read accepted or bypassed (an `Executor` can wrap
  `resp.Body` in `io.LimitReader`). Until that migration lands, the REST
  client above is what ships. The migration is not verified against a live
  backend for `429` or `304`.
- **T065 spike** (SDK `v1.20261027.1310`, Go 1.27.3, linux/amd64, a 1 KB
  `inputs` pad per resource, server in the same process, `Executor` wrapping
  `resp.Body` in a reader that fails at a 256 MiB cap):

  | Payload | SDK path max RSS | Current `DecodeIdentities` max RSS |
  | --- | --- | --- |
  | 1 MiB | 14 MiB | not run |
  | 64 MiB | 240 MiB | 185 MiB |
  | 250 MiB | 899 MiB | 685 MiB |
  | 300 MiB (over cap) | 666 MiB, fails with the cap error | not run |

  The SDK path costs about 1.3 times the current memory at the cap, and both
  are several times the payload. The cap reader must return a non-EOF error:
  `io.ReadAll` hands it back unchanged and the SDK returns it, so the
  "exceeds limit" message survives. A reader that returned `io.EOF` at the
  cap would instead surface as a JSON unmarshal error. The rows are single runs
  on one machine, so read them as an order of magnitude.
- **T066 decision**: migrate both calls. The extra memory is bounded by the
  same 256 MiB cap, the existing path is already a whole-value read, and a
  split transport would keep two code paths for a 1.3 times difference.
- **Follow-up**: The two paths can share the identity decoder later. They stay
  separate now because collect prices whole resources and the import must never
  decode properties.

## R3. Reusing login discovery

- **Decision**: Export one constructor from `internal/esc`
  (`esc.DiscoverSession`) and have `internal/pulumicloud` call it.
- **Rationale**: `internal/esc/auth.go` already finds the backend URL and token
  (`PULUMI_BACKEND_URL`, `PULUMI_ACCESS_TOKEN`, `credentials.json`, the project
  `backend.url`), validates the backend, and never sends a token to a host that
  does not match. CLAUDE.md says credential rules have one home. A second copy
  in the new package would be a second rule. `esc` imports `internal/pulumi`,
  so the client cannot live in `internal/pulumi` without an import cycle.
- **Alternatives considered**: Move discovery into `internal/pulumi` (a larger
  refactor of spec 1730 code with its own tests). Duplicate it (rejected above).
- **Org resolution**: `authSession.orgFor` and `orgFromStack` already turn
  `org/stack` or the default organization into the org name. Both are
  unexported, so `DiscoverSession` takes the stack name and returns the
  resolved org in `Session.Org`; `internal/cli` never calls them. The project
  name comes from `pulumi.GetProjectName`, the stack from
  `pulumi.LoadStackContext`.

## R4. Token override

- **Decision**: The override is the environment (`PULUMI_ACCESS_TOKEN`, which
  discovery already reads). There is no `token` key in finfocus configuration.
- **Rationale**: FR-012 allows "user-global configuration or the environment".
  The project `config.hujson` replaces the global one at the top-level key
  (`cost`), so a global `cost.history.pulumi_cloud.token` would vanish when a
  project defines any `cost` key, and a literal secret in a file is the exact
  risk FR-012 exists to prevent. The environment satisfies the requirement
  with no new secret store.
- **Alternatives considered**: A global-tier literal token with a project-tier
  rejection (needs the same gate machinery as `cost.history.directory`, plus a
  secret on disk).
- **Spec impact**: The issue's commented-out `token:` example is not built.
  Noted in the quickstart.

## R5. Where the Cloud cache lives

- **Decision**: Two new buckets in the existing resource-history database
  (`history.db`): `pulumi_cloud_updates` (the update list per stack, with a
  fetch time) and `pulumi_cloud_snapshots` (one record per stack and version).
- **Rationale**: Matches the issue ("separate BoltDB bucket") and reuses
  `NewBoltStore` corruption recovery, locking, and the history directory gate.
  The Persistence Model section of the constitution requires new stores to be
  documented first, so the plan includes that amendment (Constitution Check).
- **What a snapshot record holds**: The extracted identities only (URN, cloud
  ID, type, provider) and the update end time, never the raw export. Raw
  exports are large and carry ciphertext. This keeps SC-006 (no secrets in any
  cache) true by construction and keeps the cache small.
- **Alternatives considered**: Cache the raw export (large, contains
  secrets). A separate database file (a second lock and a new store to
  document). In-memory only (fails FR-007).
- **Resumption**: A record's presence is the "already stored" test. A separate
  `last fetched version` counter adds nothing, because an earlier range run
  later would be skipped wrongly.

## R6. Merge semantics in the history store

- **Decision**: Change `BoltStore.tryUpdateExisting` so an existing entry takes
  the minimum `FirstSeen` and the maximum `LastSeen` of the old and new values.
- **Rationale**: Today it only raises `LastSeen`. Importing an older
  observation over a newer entry would leave `FirstSeen` too late, which breaks
  User Story 1 scenario 3. For existing state-snapshot writes `FirstSeen` is
  always `now`, so taking the minimum changes nothing for them.
- **Source on merge**: The existing code already overwrites `Source` with the
  latest writer. The import follows that rule; the plan does not add a
  per-source history. A resource seen by both sources shows whichever wrote
  last.
- **Store interface**: `history.Store` is not widened (CLAUDE.md warns that a
  widened interface breaks other doubles). The cache buckets sit behind a
  separate `CloudCache` interface that `BoltStore` implements.

## R7. Retention can delete what the import writes

- **Decision**: Clamp the import's start to `now - retention days`
  (`cost.history.retention_days`, default 90) and print a note when the
  requested range starts earlier.
- **Which retention value**: The one the store was opened with. The CLI
  resolves it in `internal/cli/history_record.go` as
  `FINFOCUS_HISTORY_RETENTION_DAYS`, then `cost.history.retention_days`, then
  90, and passes it to `Import` as `Request.RetentionDays`. Re-reading the
  config file would ignore the environment override and import entries the
  next cleanup removes.
- **Rationale**: `cleanupExpiredEntries` deletes entries whose `LastSeen` is
  older than the cutoff. An import of March data run in October would write
  entries that the next cleanup removes, wasting every request. FR-018 already
  says imported entries expire like any other. The spec's import-command
  default ("the existing history retention window") matches this clamp.
- **Alternatives considered**: Exempt imported entries from cleanup (a second
  retention rule). Fail the command (hostile for an optional feature).

## R8. Which update kinds to read

- **Decision**: `update`, `refresh`, `destroy`, and `import`, whatever the
  result. `preview`, `rename`, and unknown kinds are skipped.
- **Rationale**: The spec's clarified rule is "state-changing kinds". An import
  adds resources with physical IDs, so it belongs with them. A rename moves a
  URN without changing a cloud ID.
- **Spec impact**: FR-003 and the clarification list three kinds; the plan adds
  `import`. The spec is updated to match.

## R9. Response shape of the update list

- **Decision**: Decode defensively and pin the shape with a recorded fixture.
  Read `version`, and the `kind`, `result`, `startTime`, `endTime` fields from
  the `info` object when present, falling back to top-level fields.
- **Rationale**: The OpenAPI schema for the legacy format is only
  `{"type": "object"}`, and the issue's example is flat while the service has
  historically nested these under `info`. This cannot be settled from the
  document alone.
- **First task**: Capture a real response with the Pulumi CLI (`pulumi api`
  exists per the docs) against a test stack and commit it, scrubbed, as the
  fixture. No test calls the live service (FR-019).
- **Order**: The fixture also settles the list order. A conditional request on
  page 1 is sound only if the list is newest first, because only then does a
  new update change page 1. The client sends `If-None-Match` only in that case
  (contract C5); otherwise it relies on the one-hour freshness rule alone.

## R10. Reading a snapshot safely

- **Decision**: Stream-decode the response into a struct with only
  `deployment.resources[].{urn,id,type,custom}`. Properties are never decoded.
  The body is capped (256 MiB) and read through a size-limited reader.
- **Rationale**: Decoding into a narrow struct means inputs, outputs, and
  ciphertext are never materialized, which makes FR-004 and SC-006 structural.
  A resource with an empty `id`, `custom == false`, or a `pulumi:` type is
  skipped, as `resourcesFromExport` already does for pricing.

## R11. Fetch concurrency, retries, and conditional requests

- **Decision**: `errgroup` with `SetLimit(5)`. A 429 or 503 retries up to three
  times, waiting the `Retry-After` value when present and a bounded jittered
  back-off otherwise, using the caller's context. The update list is sent with
  `If-None-Match` when a stored ETag exists. Snapshots are immutable and need
  no conditional request.
- **Rationale**: Matches FR-009. Mirrors `retryRateLimited` in the engine
  without importing it.
- **Redirects and credentials**: The client reuses the ESC rule: no redirects
  and the token only on the resolved backend host (`withoutRedirects` in
  `internal/esc/client.go`).

## R12. The import command

- **Decision**: A root-level flag `--import-cloud-history`, next to `--web` and
  `--mcp`, handled in `rootRunE`. It takes local `--from` and `--to` flags and
  the existing `--stack`.
- **Rationale**: The user named it as a flag. A root flag follows the `--web`
  precedent, shares its flag-combination validation, and exits after the work.
  It is not an MCP tool (the root is excluded from `tools/call`), so
  `tools.golden` does not change; the `__schema` output gains the flag and its
  golden updates.
- **Alternatives considered**: `cost history import-cloud` (consistent with
  `cost history collect`, easier to discover, and would become an MCP tool,
  which the single mutex would then block on). Recorded for the user in the
  plan summary. Switching later changes only the cobra wiring.
- **`enabled` rule**: The command requires `auto` or `true` (FR-021).

## R13. Incomplete-history marker in `cost actual` output

- **Decision**: Keep the optional status on rows, and use a results/status
  envelope for incomplete JSON and a final typed status record for incomplete
  NDJSON (contract C3). Always emit the human footer, even with no rows.
- **Rationale**: Row-only status disappears when every snapshot fails and
  there are no local resources. Complete output stays byte-identical.
- **Alternatives considered**: Row-only status (cannot represent empty
  incomplete reports); changing the shape of complete runs (unnecessary).

## R14. Where `cost actual` and the other readers call in

- **Decision**: Add one call, `cloudhistory.Import`, in `cost actual` between
  `recordDescriptorHistory` and `enrichWithHistoricalResources`
  (`internal/cli/cost_actual.go:205-221`). `web_data.go:363-364` and the
  overview pipeline keep reading the store and add nothing (FR-006).
- **Rationale**: The import must finish before the enrich step reads the
  store, and the range (`from`, `to`) is known only after `ParseTimeRange`.
  The call goes after it and before `enrichWithHistoricalResources`.

## R15. Known limit of "no inferred lifetime" (for review)

- **Finding**: With observation-only history, an entry for a resource that
  existed before the period and was destroyed inside it carries only
  timestamps before the period. `overlaps` in the store then excludes it, and
  the destroy update's own snapshot no longer lists it. The case the spec
  covers (created and destroyed inside the period, seen in at least one
  in-period update) works.
- **Decision**: The plan implements the clarified rule as written. On
  2026-10-27 the user reviewed the bounded last-seen extension and kept the
  observation-only rule. The limit is stated in the spec's Assumptions, the
  quickstart, and the user guide (T051). No code is built for it.

## R16. `--verbose` on `cost actual`

- **Finding**: `cost actual` has no `--verbose` flag. Only
  `cost recommendations`, `config validate`, and `plugin list` declare one,
  each locally.
- **Decision**: Declare a local `--verbose` bool on `cost actual` that prints
  the import counters to stderr. Without it the counters go to the debug log.
- **Alternatives considered**: `--debug` only (buries six numbers in every
  log line of the run). A persistent root `--verbose` (would collide with the
  three existing local declarations, and `config validate` binds `-v`).

## R17. Config page surface

- **Finding**: `config set` accepts only keys that `config.LookupSetting`
  knows (`internal/config/patch.go`), and no `cost.history.*` key is in
  `config.KnownSettings` today. The dashboard Config page builds its rows from
  the same list (`config.BuildPage`).
- **Decision**: Add one `KindEnum` entry. This makes `config set` work and
  gives the Config page an editable row with no `internal/webui` change. The
  Web UI Fit is therefore an extension of the Config page, with a render
  check and a UX-only follow-up issue (user decision, 2026-10-27).
- **YAML booleans**: A hand-written `enabled: false` is a boolean, while the
  enum control and `config set` write the string `"false"`. `PulumiCloudMode`
  accepts both forms, and the Config page shows a stored boolean as its
  string so the enum control has a matching option.

## R18. When the history store cannot be used

- **Finding**: The import writes only to the resource history store. That
  store can be turned off (`cost.history.enabled: false`,
  `FINFOCUS_HISTORY_ENABLED=false`) or held by another process
  (`ErrHistoryLocked`), and the CLI then has a nil store.
- **Decision**: Check the store before detection, so these cases make no
  request. Disabled: `auto` logs at debug level; `true` and the import command
  exit 2 naming both settings (a configuration contradiction). Locked: `auto`
  warns on stderr; `true` and the import command exit 1 naming the lock.
- **Spec impact**: The concurrency edge case now says neither process corrupts
  the store, rather than that both complete.

## R19. Replay and persistence failures

- **Decision**: Cache hits replay observations. The writer returns errors;
  failed cache/history writes count once per update as unreadable and follow
  the existing auto/required/import-command failure rules.
- **Rationale**: Cache presence proves retrieval, not recording. Replay
  restores observations after cleanup or interrupted writes without exports.
- **Alternatives considered**: Skip cached versions entirely (loses history);
  mark versions permanently imported (cannot recover removed observations).

## Implementation clarification: credential trust (2026-10-27)

The user explicitly chose to preserve the existing credential protection.
A project-controlled `backend.url` receives only a token saved for that URL,
never `PULUMI_ACCESS_TOKEN`. To override the token for a project backend,
explicitly select the backend with `PULUMI_BACKEND_URL`. The current saved
account continues to accept the environment token override. This clarification
supersedes broader override wording elsewhere in these artifacts; there is
no change to the shared ESC credential rule.

## Implementation clarification: overview inventory (2026-10-27)

The user explicitly chose to keep overview's current state/plan inventory.
Unlike the web Cost path, the existing overview loaders record observations
but do not read historical-only identities into rows. This feature verifies
that overview makes zero Pulumi Cloud requests and retains its inventory;
it does not add historical rows. The web Cost page and `cost actual` read
stored identities. This supersedes wording that assumed both views already
included imported identities.
