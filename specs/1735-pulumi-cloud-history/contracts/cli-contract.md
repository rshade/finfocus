# Contract: CLI Behavior and Internal Interfaces

The feature adds no HTTP endpoint to `finfocus --web`. Its web surface is one
row on the existing Config page; [http-api.md](http-api.md) names the Go
methods behind it. The web Cost page and the overview keep calling
`enrichWithHistoricalResources` and read the store.

## C1. Import command

```text
finfocus --import-cloud-history [--from <date>] [--to <date>] [--stack <name>]
```

| Rule | Behavior |
| --- | --- |
| Setting | Requires `cost.history.pulumi_cloud.enabled` of `auto` or `true`. `false` or absent exits 2 with `Pulumi Cloud history is disabled; set cost.history.pulumi_cloud.enabled to auto or true`. No request is made. |
| Dates | Same formats as `cost actual` (`2006-01-02`, RFC3339). `--to` defaults to now. `--from` defaults to now minus the resolved retention period (`FINFOCUS_HISTORY_RETENTION_DAYS`, then `cost.history.retention_days`, then 90; the same value the store's cleanup uses). A `--from` earlier than that is moved to the cutoff and a note says so. |
| History store | Requires the resource history store. `cost.history.enabled: false` (or `FINFOCUS_HISTORY_ENABLED=false`) exits 2 naming both settings. A store held by another process (`ErrHistoryLocked`) exits 1 naming the lock. |
| Flag use | `--from` and `--to` are valid only with `--import-cloud-history`. Combining the flag with `--web`, `--mcp`, or a subcommand exits 2 through `toValidationError`. |
| Work | Detects Pulumi Cloud, imports, prints the summary, calculates no cost, starts no plugin. |
| Exit | 0 when the import ran, including a no-op. 1 when it cannot run or any update is unreadable (its only purpose failed). 2 for flag, setting, or date errors. |
| Output | Human: one line per counter. JSON (`--format json`): the `ImportResult` object (C3). |

Human summary lines, in this order: `Updates listed`, `Skipped (not state
changing)`, `Served from cache`, `Fetched`, `Identities recorded`, `Unreadable`.

## C2. `cost actual` behavior

Order inside `runCostActual`: `recordDescriptorHistory`, resolve dates,
**`cloudhistory.Import`**, `enrichWithHistoricalResources`.

| Setting | Detection | Result |
| --- | --- | --- |
| absent or `false` | not run | No request. Output, exit code, and timing unchanged. |
| `auto` | unavailable | Debug log with the reason. Local history only. |
| `auto` | available, all updates read | Import completes; normal output. |
| `auto` | available, some unreadable | Stderr warning naming the count. Rows carry `historyImport` (C3). Table gets a footer. Exit code unchanged. |
| `auto` | history store disabled or locked | Disabled: debug log. Locked: stderr warning. Local history only; exit code unchanged. |
| `true` | unavailable, store locked, or any update unreadable | Exit 1 with the reason and a fix. No totals printed. |
| `true` | `non_cloud_backend` | Exit 2 (`Cloud history requires a Pulumi Cloud backend`). |
| `true` | `insecure_backend` | Exit 1 (`Pulumi Cloud backend must use HTTPS`). |
| `true` | history store disabled | Exit 2 naming `cost.history.enabled` and `cost.history.pulumi_cloud.enabled`. |

`cost actual` gains a `--verbose` flag (bool, default false). With it, the C1
counters go to stderr in C1 order after the import. Without it they are logged
at debug level only. The flag changes nothing on stdout.

## C3. Output fields

Additive and omitted unless the import ran incomplete (research R13).

```json
{
  "historyImport": {
    "incomplete": true,
    "unreadableUpdates": 2
  }
}
```

On each `CostResult` row (`json:"historyImport,omitempty"`). Incomplete JSON
also uses an envelope, including for zero rows:

```json
{"results": [], "historyImport": {"incomplete": true, "unreadableUpdates": 2}}
```

Incomplete NDJSON ends with exactly one status record, even with zero rows:

```json
{"type": "historyImport", "historyImport": {"incomplete": true, "unreadableUpdates": 2}}
```

The table status footer is emitted even with no rows. Complete runs retain
the existing JSON array and NDJSON row stream byte-for-byte. Consumers must
handle the explicit incomplete shapes when opting into Cloud history.

The import command's JSON is the `ImportResult` object:

```json
{
  "listed": 41,
  "skipped": 6,
  "fromCache": 30,
  "fetched": 5,
  "recorded": 212,
  "unreadable": 0,
  "clamped": false
}
```

Neither shape contains a token, URL userinfo, query string, property value, or
response body.

## C4. Go interfaces

```go
package pulumicloud

// Client reads stack updates and snapshots from one Pulumi Cloud backend.
type Client interface {
    ListUpdates(ctx context.Context, stack StackRef, etag string) (UpdateList, error)
    ExportIdentities(ctx context.Context, stack StackRef, version int) ([]Identity, error)
}

// StackRef is org, project, and stack name.
type StackRef struct{ Org, Project, Stack string }

// Identity is the only data read from a snapshot.
type Identity struct{ URN, CloudID, Type, Provider string }
```

```go
package cloudhistory

// Import runs detection, then lists, fetches, caches, and records.
func Import(ctx context.Context, deps Deps, req Request) (Result, error)

// Deps carries the store, cache, client factory, clock, and logger.
// Request carries the mode, stack context, the range, and RetentionDays:
// the resolved value the store was opened with, never re-read from config.
```

```go
package history

// CloudCache is implemented by *BoltStore, not added to Store.
type CloudCache interface {
    GetUpdateList(stackHash string) (UpdateListRecord, bool, error)
    PutUpdateList(stackHash string, rec UpdateListRecord) error
    GetSnapshot(stackHash string, version int) (SnapshotRecord, bool, error)
    PutSnapshot(stackHash string, rec SnapshotRecord) error
}

// RecordCloudObservations writes observations and returns persistence errors.
func (w *Writer) RecordCloudObservations(
    stackCtx StackContext, observedAt int64, ids []CloudIdentity) error
```

```go
package esc

// Session is the active backend, its token (never logged), and the
// organization resolved for the stack.
type Session struct{ Backend, Token, Org string }

// DiscoverSession exposes login discovery to other packages. stackName is
// the selected stack as `org/stack` or `stack`; Org comes from the same
// rules as authSession.orgFor (explicit org, else the default organization).
func DiscoverSession(
    getenv func(string) string, projectBackend, stackName string,
) (Session, *Failure)
```

A snapshot cache hit replays its identities at the cached observation time;
it skips only the network export. Newly fetched identities are cached before
recording. A cache-write or history-write failure increments `Unreadable`
once for that update; `auto` continues with a warning and incomplete status,
while `true` and the import command fail. A retained cache record is retried
through replay on the next run. A nil or disabled writer returns an error;
the orchestrator handles disabled mode before calling it. Successful writes
are idempotent, so partial writes can safely be replayed.

`internal/cli` uses `Session.Org` and never calls the unexported
`orgFromStack`.

`cloudhistory` may import `history`, `pulumicloud`, `config`, and `logging`.
`history` imports neither `pulumicloud` nor `cloudhistory`. `webui` imports
none of them.

## C5. Service requests

| Purpose | Request |
| --- | --- |
| Detection | `GET /api/user` with the token (any 2xx is accepted; 401 is `token_rejected`, 403 is `no_permission`). |
| List | `GET /api/stacks/{org}/{project}/{stack}/updates?page=<n>&pageSize=100`, pages from 1 until a page adds no new version. |
| Conditional list | `If-None-Match` on page 1 only when an ETag is stored and the recorded fixture shows the list newest first. Then a new update always lands on page 1, so a 304 proves the whole list is unchanged. If the fixture shows another order, no conditional request is sent and the one-hour freshness rule alone applies. |
| Snapshot | `GET /api/stacks/{org}/{project}/{stack}/export/{version}` |
| Headers | `Authorization: token <token>`, `Accept: application/vnd.pulumi+8`. |
| Redirects | None followed. |
| Retry | 429 and 503, three attempts, `Retry-After` else bounded jitter. |

Detection uses `GET /api/user` because it is cheap and needs no stack. The
shape and order of the update list are pinned by a recorded fixture (research
R9).

Detection reasons, checked in this order with no request until the last
three: `not_logged_in` (no token), `non_cloud_backend` (`file`, `s3`, `gs`,
`azblob` schemes), `insecure_backend` (`http://` that is not loopback),
`unknown_stack` (no org, project, or stack), then from `GET /api/user`:
`token_rejected` (401), `no_permission` (403), `unreachable` (network error or
5xx after retries).
