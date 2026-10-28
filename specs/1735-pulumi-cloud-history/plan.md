# Implementation Plan: Pulumi Cloud Historical State Snapshots

**Branch**: `1735-pulumi-cloud-history` | **Date**: 2026-10-27 |
**Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1735-pulumi-cloud-history/spec.md`

## Summary

Add an opt-in import that reads a Pulumi Cloud stack's update list and the
deployment snapshot of each state-changing update, extracts only resource
identity, and records it in the existing resource-history store with source
`pulumi_cloud`. `cost actual` runs the import inline before it reads the
history. A new `--import-cloud-history` root flag runs the long pull ahead of
time so the overview and `finfocus --web`, which never fetch, start fast and
read what was stored. The default is off (`cost.history.pulumi_cloud.enabled`
of `false`); `auto` falls back quietly and `true` fails rather than report
totals that may be short.

The technical approach (research.md): a small REST client in a new
`internal/pulumicloud` package, login discovery reused through one exported
constructor from `internal/esc`, an orchestrator in `internal/cloudhistory`,
two new buckets in the existing `history.db` that cache extracted identities
(never raw exports), and two small changes to the history store (a
`FirstSeen` minimum on merge and the new source value). The setting is one
`config.KnownSettings` entry, which makes it settable through `config set`
and gives the dashboard Config page an editable row (research R17).

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`)
**Primary Dependencies**: standard `net/http`, `encoding/json`,
`golang.org/x/sync/errgroup`, `go.etcd.io/bbolt`, zerolog, Cobra and ax-go,
testify. No new module. Login discovery is `internal/esc`.
**Storage**: Two new buckets (`pulumi_cloud_updates`,
`pulumi_cloud_snapshots`) in the existing BoltDB `history.db`.
**Testing**: Testify only. An `httptest` stand-in for the service, a recorded
update-list fixture, table-driven store tests, and a CLI test through
`ax.Execute`. No live calls.
**Target Platform**: Linux, macOS, Windows (the client uses no
platform-specific code).
**Project Type**: single Go module (CLI plus plugin host)
**Performance Goals**: Disabled default adds under 100 ms and no request
(SC-002). First import of 100 updates under 60 s (SC-004). A repeat run
fetches nothing it already holds (SC-003).
**Constraints**: At most five requests in flight. Token only to the resolved
backend host, no redirects. Nothing but identity is ever decoded or stored.
No command may require the new buckets to succeed.
**Scale/Scope**: Stacks with hundreds of updates and thousands of resources
per snapshot; snapshots are stream-decoded and capped at 256 MiB.

No `NEEDS CLARIFICATION` remains. Unknowns the spec deferred are resolved in
research.md (R1 to R19).

## Constitution Check

*GATE: passed before Phase 0; re-checked after Phase 1 below.*

- [x] **Plugin-First Architecture**: Pulumi Cloud is not a cost data source.
  It supplies resource identity to core's own history store, which already
  lives in core. No provider-specific cost logic is added.
- [x] **Test-Driven Development**: Tests are written first for each unit
  (client, decoder, cache, orchestrator, store merge, config enum, CLI).
  Coverage target is 80%, 95% on the `cost actual` entry path and the new
  orchestrator. No TUI code changes, so no golden files.
- [x] **Cross-Platform Compatibility**: Plain HTTP and BoltDB; no signals, no
  subprocess, no path assumptions.
- [x] **Documentation Integrity**: Updates `docs/.../guides/resource-history.md`,
  `reference/config-reference.md`, `reference/cli-commands.md`, the `esc`
  README (exported constructor), a new package README for `pulumicloud`, and
  godoc on every exported symbol.
- [x] **Protocol Stability**: No protobuf or plugin change. The additive
  `historyImport` field on actual-cost rows is
  omitted unless the import ran incomplete. Incomplete JSON uses a
  results/status envelope; incomplete NDJSON ends with a typed status record
  so even zero-row reports carry status (contract C3). Complete output is unchanged.
- [x] **Implementation Completeness**: No stubs. `import` handling, 304 reuse,
  retry, and clamping are all in scope.
- [x] **Persistence Model**: Two new cache buckets in an existing store.
  Constraint 3 requires documenting new stores before implementation, so the
  first task amends `CONTEXT.md` and the constitution's Persistence Model table
  (PATCH bump to 1.8.1) through the `/speckit.constitution` procedure, so the
  Sync Impact Report and dependent templates stay consistent. The feature works with the buckets deleted
  (constraint 1), and a corrupt database is recreated by the existing
  `openHistoryDB` (constraint 4). Constraint 2 (stores are opt-out) does not
  apply: the store already exists; the network import is the opt-in part.
- [x] **Web Parity**: The Web UI Fit is an extension of the Config page. The
  setting is a `config.KnownSettings` entry, so the existing
  `GET /api/config`, `POST /api/config/preview`, and `POST /api/config`
  handlers serve it through `config.BuildPage` and `config.PatchFile`, the
  same methods `config get` and `config set` use. `contracts/http-api.md`
  names them. No new handler. The Cost page reads the store unchanged. Tasks
  cover the descriptor (T038), a handler test that the row is served (T060),
  a Playwright render of the Config page (T062), the docs reference (T053),
  and the UX-only `web-ui` follow-up issue (T063). The web must not import
  the new packages (import test, T052).
- [x] **Quality Gates**: `make lint`, `make test`, `make validate`, markdownlint.
- [x] **Multi-Repo Coordination**: None. No spec or plugin repo change.

**Violations Requiring Justification**: None.

### Re-check after design

No principle changed state. The cache stores extracted identity only, which
keeps the credential rule (one rule: `history.IsPulumiSecret` and
`config.IsCredentialName`) from needing a new case.

## Project Structure

### Documentation (this feature)

```text
specs/1735-pulumi-cloud-history/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli-contract.md
│   └── http-api.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit.tasks, not created here
```

### Source Code (repository root)

```text
internal/
├── pulumicloud/                  # new: REST client for one Pulumi Cloud backend
│   ├── client.go                 # ListUpdates, ExportIdentities, retry, ETag
│   ├── detect.go                 # Availability and reasons
│   ├── identity.go               # narrow stream decoder
│   ├── README.md
│   └── *_test.go, testdata/updates.json
├── cloudhistory/                 # new: orchestrator
│   ├── import.go                 # Import, range clamp, bounded fetch
│   ├── result.go                 # Result and incomplete rule
│   └── *_test.go
├── esc/
│   └── session.go                # new: DiscoverSession (exports existing logic)
├── history/
│   ├── entry.go                  # SourcePulumiCloud
│   ├── store.go                  # FirstSeen minimum on merge
│   ├── cloud_cache.go            # CloudCache buckets on BoltStore
│   └── writer.go                 # RecordCloudObservations
├── config/
│   ├── budget.go                 # HistoryConfig.PulumiCloud
│   ├── descriptor.go             # KnownSettings enum entry (config set, Config page)
│   ├── view.go                   # defaultValue "false"
│   └── pulumi_cloud.go           # PulumiCloudMode (bool or string), validation
├── engine/
│   ├── types.go                  # CostResult.HistoryImport (omitempty)
│   └── project.go                # table footer for an incomplete import
└── cli/
    ├── cost_actual.go            # one Import call before enrich, --verbose
    ├── history_record.go         # expose the resolved retention period
    ├── import_cloud_history.go   # root flag handler
    └── root.go                   # register flag, validate combinations
CONTEXT.md                        # Persistence Model entry
.specify/memory/constitution.md   # Persistence Model table, v1.8.1
docs/src/content/docs/guides/resource-history.md
docs/src/content/docs/reference/{config-reference,cli-commands}.md
internal/cli/testdata/                # __schema golden
```

**Structure Decision**: Single Go module. The client and orchestrator are
separate packages so `internal/history` keeps its rule of not importing
`engine` or `ingest` and does not learn about HTTP. `internal/webui` imports
none of the new packages.

## Risks and open points

1. **List response shape** (R9): the legacy format is not typed in the OpenAPI
   document. Mitigation: first task captures a real response as a fixture; the
   decoder accepts nested and flat fields.
2. **Resources that predate the period** (R15): under the clarified
   observation-only rule, a long-lived resource destroyed inside the period
   has no in-period observation and is not found by this source. The case the
   spec's story covers works. The user reviewed the bounded `LastSeen`
   extension on 2026-10-27 and kept the rule; the limit is documented (T051)
   and not built.
3. **Source on merge** (R6): a resource seen by both state snapshots and Cloud
   shows the last writer's source. Harmless for cost, visible in history
   output.
4. **Command form** (R12): a root flag follows `--web` but is less discoverable
   than `cost history import-cloud`. Switching is a wiring change.
5. **Import kind** (R8): `import` is added to the spec's three kinds.
6. **Token** (R4): no config token key. The issue's commented example is not
   built.
7. **`history.db` lock or disabled store** (R18): another finfocus process
   holding the database makes `NewBoltStore` return `ErrHistoryLocked`; the
   import then skips with a warning under `auto` and exits 1 under `true` and
   for the import command. A disabled store skips at debug level under `auto`
   and exits 2 under `true` and for the import command. Both are checked
   before any request.
8. **List order** (R9): a conditional list request is only sent when the
   recorded fixture shows the list newest first.
9. **YAML booleans** (R17): `enabled: false` in a hand-written file is a
   boolean; the parser accepts both forms.
10. **`--verbose`** (R16): `cost actual` gains a local flag.

## Complexity Tracking

No constitution violations.

## Import persistence and output refinements

`RecordCloudObservations` returns persistence errors. Each in-range cached
snapshot replays its observations idempotently, skipping only the network
export. Fetches cache extracted identities before writing history; failures
of either write count the update once as unreadable. A cached but unrecorded
snapshot is retried by replay on the next run. This also restores entries
removed by cleanup after retention is widened. Required mode and the import
command fail on any such persistence failure; auto emits incomplete status.

Observation timestamps select identities, including a single observation;
billing plugins supply actual cost for the requested query range. T024 must
assert a known returned cost, not only descriptor membership.

T040 uses the controlled SC-004 workload; throttling tests separately assert
server-requested waits without applying the baseline timing budget.
Phase 3 is a development checkpoint. All stories, quality gates, and Web
Parity work are required before merge.
