# Quickstart: Pulumi Cloud Historical State Snapshots

Use this to check the feature by hand once it is built. It needs a Pulumi
Cloud stack that has had at least one resource created and destroyed, and
`pulumi login` against Pulumi Cloud.

## Enable

```bash
finfocus config set cost.history.pulumi_cloud.enabled auto
```

`auto` falls back silently when Pulumi Cloud is unavailable. `true` fails
instead of reporting totals that might be short. `false` (the default) never
contacts Pulumi Cloud. The same setting is a row on the `finfocus --web`
Config page. Cloud history needs the resource history store, so
`cost.history.enabled` must stay on.

## Pre-load the history

```bash
finfocus --import-cloud-history --from 2026-10-01 --to 2026-10-31
```

Expected: six counters. A second run reports `Fetched: 0` for the same range.
A `--from` older than the retention period (`cost.history.retention_days`,
90 by default, or `FINFOCUS_HISTORY_RETENTION_DAYS`) is moved to the cutoff
and the output says so.

## See it in cost actual

```bash
finfocus cost actual --from 2026-10-01 --to 2026-10-31
```

Expected: resources that were created and destroyed between your finfocus runs
appear in the billing query with the plugin's returned cost. Even a single
observation selects an identity; it does not infer lifetime through destruction. If any update could not be read, stderr has a warning
and JSON uses a `results`/`historyImport` envelope, including with no rows.
NDJSON ends with a typed status record (contract C3). Add `--verbose` to see
the six counters on stderr.

A resource that existed before `--from` and was destroyed inside the range is
not added by this source: no update inside the range lists it.

## Check it is opt-in

```bash
finfocus config set cost.history.pulumi_cloud.enabled false
finfocus cost actual --from 2026-10-01 --to 2026-10-31 --debug 2>&1 | grep -c api.pulumi.com
```

Expected: `0`.

## Overview and web

`finfocus` (overview) and `finfocus --web` never wait on Pulumi Cloud. They show
whatever the import command or `cost actual` stored. On the web Config page,
the `cost.history.pulumi_cloud.enabled` row offers `false`, `auto`, and `true`.

## Token and backend

The token comes from your Pulumi login. To override it, set
`PULUMI_ACCESS_TOKEN` in the environment; there is no token key in the
configuration file. Self-hosted Pulumi Cloud works through `pulumi login <url>`
or `PULUMI_BACKEND_URL`.

## Tests

```bash
go test ./internal/pulumicloud/... ./internal/cloudhistory/... ./internal/history/...
go test -run 'TestCostActual.*CloudHistory' ./internal/cli/
make test && make lint
```

The tests use an `httptest` stand-in server and a recorded update-list fixture.
They enable the setting explicitly, and one test pins that the unset default
makes no request.

## Recovery and empty-result validation

Against the stand-in, import once, remove the corresponding history entries
while retaining snapshot cache, then repeat with a wider retention window.
Expected: identities are restored and no export request is made.

Inject a history or cache write failure. Expected: `auto` reports incomplete
history; `true` and the import command fail. A subsequent successful run
replays cached observations safely. With no cost rows, incomplete JSON still
contains `results: []` and `historyImport`; NDJSON still emits its final typed
status record, and human output still prints its footer.

Use T040's controlled SC-004 workload for the import timing check. Run retry
and throttling checks separately from that baseline.
