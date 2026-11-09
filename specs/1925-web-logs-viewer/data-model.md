# Data Model: Web UI Log Viewer

All state is in memory for the life of one `finfocus --web` process. Nothing
is written to disk by this feature.

## Entry

One captured log line after parsing and redaction.

| Field | Type | Rules |
| --- | --- | --- |
| `seq` | uint64 | Starts at 1 and increases within one hub or one `Decode`. The hub never reuses a number, and that number is the SSE `id`. A positive `finfocus logs --tail` numbers `seq` inside the decoded suffix, unless a line longer than 64 KiB forces a decode from the start of the file. |
| `time` | string | RFC 3339 UTC. Taken from the line's `time` field when it parses; otherwise the receive time. |
| `level` | string | One of `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`. Unknown or missing becomes `info`. |
| `source` | string | See Source. Never empty. |
| `sourceKind` | string | `core` or `plugin`. |
| `message` | string | The line's `message`; for a raw line, the whole line. Cut to 64 KiB. |
| `fields` | object | Remaining fields after redaction. Omitted when empty. `time`, `level`, `message` are not repeated. |
| `raw` | bool | True when the line was not JSON. A raw entry has level `info` and no `fields`. |
| `truncated` | bool | True when the line or message exceeded the cap. |
| `display` | object | `{ "time": "15:04:05.000", "level": "WARN", "source": "aws-public" }`, produced by `weblog.Display`. Local time, fixed-width upper-case level label. Used as is by `finfocus logs` and the page. |

Redaction (applies to `fields`, recursively through objects and arrays):

- A key for which `config.IsCredentialName` is true keeps its key and gets the
  value `"[redacted]"`.
- A value for which `history.IsPulumiSecret` is true gets `"[redacted]"`.
- A key beginning `__` is removed.

## Source

The label an entry is filtered by. Resolved in this order:

1. The line's `plugin_name` field (`sourceKind` = `plugin`).
2. The tag of the per-plugin writer that received the line (`plugin`).
3. The line's `component` field (`core`).
4. The literal `core` (`core`).

When a file is decoded (`finfocus logs`) there is no writer tag, so step 2 does
not apply and a raw line gets source `unknown` (`sourceKind` = `core`).

A source is registered the first time an entry names it and stays until the
process ends. The set is sent with the snapshot so a new tab can show every
chip before any line matches.

## Buffer

| Property | Value |
| --- | --- |
| Capacity | 5000 entries |
| Order | By `seq` |
| Eviction | Oldest first when full; `dropped` counts evicted entries |
| Subscribers | Each has a channel of 256; a full channel closes it (the page reconnects) |

Operations: `Append(entry)`, `Subscribe(afterSeq)` returning the retained
entries after `afterSeq`, a live channel, and a cancel function; `Status()`
returning capacity, retained count, dropped count, and sources.

State transitions of a subscriber: `connected` then either `cancelled` by the
client or `closed` by the buffer when it lags. A closed subscriber is not
resumed; the client reconnects with its last `seq`.

## Line assembler

One per writer (one for core's tee, one per plugin). It holds the unterminated
tail between writes.

| Event | Result |
| --- | --- |
| Write with complete lines | Each line parsed and appended; the tail is kept |
| Write without a newline | Bytes kept in the tail |
| Tail over 64 KiB | Flushed as a raw, truncated entry |
| `Close` | Tail flushed as one entry if non-empty |
| Line `\r\n` | The `\r` is trimmed |
| Empty line | Ignored |

## Filter

Shared by `finfocus logs` (Go) and mirrored by the viewer package (JavaScript).
`Match(entry)` is true when all hold: `source` is in the selected set (empty
set means all); the level rank is at least `MinLevel`; and, when `Text` is
set, a case-insensitive substring of the message or any field value matches.
Level order: `trace` < `debug` < `info` < `warn` < `error` < `fatal` < `panic`.
A shared fixture, `internal/weblog/testdata/filter_cases.json`, is the canonical
copy. The viewer package vendors it, and `sample.ndjson`, under
`packages/zerolog-viewer/test/fixtures/`. `TestVendoredViewerFixturesMatch`
fails when a copy differs, so the two implementations cannot disagree and the
package tests still run with no finfocus source present.

## Decoder

`weblog.Decode(r io.Reader, tag string)` streams a log file through the same line
assembler and entry builder as the live writers and yields entries in file
order. `finfocus logs --tail 0` reads the whole file that way. A positive
`--tail` calls `weblog.TailDecode`, which seeks backward until it has N
matching entries and then `Decode`s only that suffix. Memory for a tail is
bounded by N and the line cap, not by file size. `seq` on a tailed read is
the position in that suffix, because the prefix is not scanned. A line longer
than 64 KiB inside the window is decoded from byte 0, and `seq` then stays
the whole-file index.

## Viewer state (browser, per tab)

Not sent to the server and not shared between tabs.

| Field | Default |
| --- | --- |
| Selected sources | All |
| Minimum level | `trace` |
| Search text | Empty |
| Following | True |
| New while paused | 0 |

The page holds at most 2000 rows; older rows are removed from the top and the
status line says how many.

## Setting

`web.logs.enabled`: bool, default false, in the `web` config section. Env
`FINFOCUS_WEB_LOGS` overrides it.
