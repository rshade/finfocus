# Contract: Logs HTTP API

Both routes sit behind the existing session cookie, Host check, and
`requestHardening` middleware. They are registered only when logs are enabled;
otherwise they do not exist and the server answers its standard
`404 {"error":"unknown path","code":"not_found"}` from `handleRoot`.
Error bodies use the existing `writeError` shape.

Neither route is logged by `requestLog`.

## Go methods called (Constitution Principle VII)

The CLI counterpart is `finfocus logs` (`internal/cli/logs.go`). Both sides use
the same entry builder, so one line yields one entry everywhere:

| Surface | Go method |
| --- | --- |
| `finfocus logs` | `weblog.Decode`, `weblog.Filter.Match`, and `weblog.Display` (used only for an entry that carries no display strings) |
| `GET /api/logs/status` | `weblog.(*Hub).Status` |
| `GET /api/logs/stream` | `weblog.(*Hub).Subscribe` (entries built by `weblog.NewEntry`, which wraps the same unexported builder `Decode` uses; both fill `Entry.Display` through `weblog.Display`) |

`internal/webui` reaches the hub through an interface so it imports neither
`cli`, `registry`, nor `pluginhost`.

`webui.Options.Logs` is `webui.LogSource`:

```go
type LogSource interface {
    Status() weblog.Status
    Subscribe(afterSeq uint64) (snapshot weblog.Snapshot, live <-chan weblog.Entry, cancel func())
}
```

`cli` builds one `*weblog.Hub` in `setupLogging` (through `setupLoggingWithHub`)
when logs are enabled and the command is `--web`, stores it in the context
(`weblog.ContextWithHub`), and `runWebSession` passes
`weblog.HubFromContext(ctx)` as `Options.Logs`. A nil `Logs`, including a typed
nil `*weblog.Hub`, means disabled.

## GET /api/logs/status

Tells the page whether to show the Logs entry and what to expect.

Response `200`:

```json
{
  "enabled": true,
  "capacity": 5000,
  "retained": 412,
  "dropped": 0,
  "sources": [
    { "name": "core", "kind": "core" },
    { "name": "aws-public", "kind": "plugin" }
  ]
}
```

Disabled: the route does not exist (`404`).
Method other than GET: `405` with `Allow: GET` and body
`{"error":"GET only","code":"method_not_allowed"}`. The shared
`requestHardening` check runs first, so a POST without an `Origin` header is
answered `403` and one without a JSON `Content-Type` is answered `415`; only a
same-origin JSON request reaches the `405`.

## GET /api/logs/stream

`Content-Type: text/event-stream`, `X-Accel-Buffering: no`, same write
deadline and 15 s keepalive comment as the overview stream.

Request header `Last-Event-ID: <seq>` is optional. When present, the snapshot
holds only entries with `seq` greater than it. A value that is not an unsigned
decimal integer is treated as absent. Query strings are not read.

Events:

| Event | `id` | Data |
| --- | --- | --- |
| `snapshot` | last `seq` in the snapshot, else the request's `Last-Event-ID`, else omitted | `{ "entries": [Entry, …], "sources": [Source, …], "dropped": n, "capacity": 5000 }` |
| `gap` | none | `{ "missed": n }`, sent right after `snapshot` only when the request carried a `Last-Event-ID` and entries newer than it were already evicted. A first connection to a buffer that has wrapped gets no `gap`. A page that recreates the stream cannot send the header, so it compares the snapshot's first `seq` with the last one it showed |
| `log` | the entry's `seq` | One `Entry` |

`Entry` and `Source` are defined in [data-model.md](../data-model.md).
Example:

```text
id: 413
event: log
data: {"seq":413,"time":"2026-11-02T18:04:11Z","level":"warn","source":"aws-public","sourceKind":"plugin","message":"price lookup slow","fields":{"region":"us-east-1","token":"[redacted]"},"display":{"time":"13:04:11.000","level":"WARN","source":"aws-public"}}

```

Rules:

- Entries are redacted before they reach the hub. The handler does not run
  `RedactJSON` (it would drop `[redacted]` keys the viewer needs to show) and
  does not add a second rule.
- The server closes the stream when the session context ends, the client goes
  away, or the subscriber's channel fills. The browser reconnects with
  `Last-Event-ID`.
- Response to a non-GET method: as for status above.
- Query string: ignored. The session token is never accepted here.

## Cookies, origin, and CSP

Unchanged. `EventSource` is same-origin and sends the session cookie. The CSP
`default-src 'self'` already allows the module and stylesheet served from
`/static/zerolog-viewer/`.

## Static assets

`GET /static/zerolog-viewer/index.js` and `viewer.css` are served from the
viewer package's embedded `src/`. They contain no finfocus data and are served
whether or not logs are enabled; they are inert until the Logs view imports
them, and the view is absent when disabled.

## Redaction guarantees (testable)

For every entry in `snapshot` and `log` events:

- No value under a credential-named key appears; it is `"[redacted]"`.
- No Pulumi secret envelope appears.
- No key starting `__` appears.
