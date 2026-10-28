# Data Model: Pulumi Cloud Historical State Snapshots

All new state lives in the existing resource-history database
(`<history dir>/history.db`, opened by `history.NewBoltStore`). Nothing here is
required for any command to succeed: deleting the buckets only means the next
enabled run fetches again.

## Setting: `cost.history.pulumi_cloud`

| Field | Type | Default | Rule |
| --- | --- | --- | --- |
| `enabled` | boolean or string | `false` | One of `false`, `auto`, `true`. The booleans `false` and `true` and the strings `"false"` and `"true"` are equal; hand-written YAML or Hujson usually holds the boolean, and `config set` and the Config page write the string. An absent key is `false`. Any other value, including an empty string and `"AUTO"`, fails `config validate` and every `cost` pre-run with the three allowed values. |

There is no token field. The token override is `PULUMI_ACCESS_TOKEN`
(research R4). `HistoryConfig` gains `PulumiCloud PulumiCloudHistory` with
`yaml`/`json` tag `pulumi_cloud`. The value is a typed string
(`PulumiCloudMode`) with constants `Off`, `Auto`, `Required`; its
`UnmarshalYAML` and `UnmarshalJSON` accept both forms.

The key is also a `config.KnownSettings` entry (`KindEnum`, enum
`false`/`auto`/`true`, default `"false"` in `config.defaultValue`), which puts
it in `config set`, `config get`, and the dashboard Config page
(`contracts/http-api.md`).

## Entity: Availability

The outcome of detection. Not stored.

| Field | Meaning |
| --- | --- |
| `Available` | bool |
| `Reason` | One of `not_logged_in`, `non_cloud_backend`, `insecure_backend`, `token_rejected`, `no_permission`, `unreachable`, `unknown_stack`. Empty when available. |
| `Session` | Backend URL, token (never logged), and org; present only when available. |

Transitions: detection runs once per command. `auto` treats any `Reason` as a
fallback; `true` turns it into an error naming the reason and a fix
(`non_cloud_backend` exits 2, the rest exit 1).

Two more skip conditions come from the store, not from detection, and are
checked first so they make no request: `history_disabled` (the store is
turned off; `true` and the import command exit 2) and `history_locked`
(`ErrHistoryLocked`; `auto` warns, `true` and the import command exit 1).

## Entity: UpdateRecord

One row of the stack's update list, decoded from the legacy list format.

| Field | Type | Notes |
| --- | --- | --- |
| `Version` | int | Positive; the export key. |
| `Kind` | string | Imported kinds: `update`, `refresh`, `destroy`, `import`. |
| `Result` | string | Recorded for logs only; it does not gate import. |
| `Start` / `End` | time | Unix seconds in the service payload. `End` is the observation time. A zero `End` falls back to `Start`. |

A row with `Version <= 0` or a non-imported kind is dropped and counted as
`skipped`, not as unreadable.

## Entity: SnapshotRecord (bucket `pulumi_cloud_snapshots`)

Key: `<stackHash>/<version, 10 digits zero-padded>`. Value (JSON):

| Field | Type | Notes |
| --- | --- | --- |
| `version` | int | Matches the key. |
| `observed_at` | int64 | The update end time, Unix seconds. |
| `fetched_at` | int64 | When finfocus read it. |
| `resources` | list | Each `{urn, cloud_id, type, provider}`. |

Rules:

- Immutable once written. A present record is never fetched again, but its
  observations are replayed into history on every in-range import. Cache
  presence proves retrieval, not successful persistence into history.
- Resources without `id`, non-custom resources, and `pulumi:` types are never
  stored.
- Properties, outputs, ciphertext, and tags are never stored (research R10).
- A record that fails to decode is treated as absent and refetched.

## Entity: UpdateListRecord (bucket `pulumi_cloud_updates`)

Key: `<stackHash>`. Value (JSON):

| Field | Type | Notes |
| --- | --- | --- |
| `fetched_at` | int64 | Fresh while `now - fetched_at < 3600`. |
| `etag` | string | Sent as `If-None-Match` on refresh; may be empty. |
| `updates` | list | `UpdateRecord` rows for all kinds, so a later range reuses it. |

A `304` response keeps `updates` and resets `fetched_at`.

## Entity: ResourceHistoryEntry (existing)

`internal/history/entry.go` gains `SourcePulumiCloud = "pulumi_cloud"` and
`isValidSource` and the validation message accept it. The record shape does not
change.

Per observation: `FirstSeen = LastSeen = observed_at`. A merge into an existing
entry takes the minimum `FirstSeen` and maximum `LastSeen` (research R6). Tags
are not written by this source.

## Entity: ImportResult

Returned by `cloudhistory.Import` and printed by the import command and
`--verbose`.

| Field | Meaning |
| --- | --- |
| `Listed` | Updates in the list after the range filter. |
| `Skipped` | Listed updates of a non-imported kind. |
| `FromCache` | Imported-kind updates already stored. |
| `Fetched` | Snapshots read from the service this run. |
| `Recorded` | Identities written (after merge, counted per observation). |
| `Unreadable` | Updates that failed fetch, decode, cache write, or history write; counted once per update. |
| `Clamped` | True when the range start was moved to the retention cutoff. |

Incomplete JSON uses the results/status envelope and incomplete NDJSON a final
typed status record, including for zero cost rows (contract C3).

`Incomplete` is `Unreadable > 0`. Under `auto` it sets the output marker
(contract C2); under `true` it fails the command.

## Validation summary

| Spec requirement | Where enforced |
| --- | --- |
| FR-010 enum | `PulumiCloudMode.Validate`, called by `ValidateConfigSource` |
| FR-004 identity only | Narrow decode struct; `SnapshotRecord` has no property fields |
| FR-005 observation model | `Writer.RecordCloudObservations` |
| FR-007 caching | Bucket keys above; list freshness in `cloudhistory` |
| FR-018 retention | `Import` clamp with `Request.RetentionDays` (the value the store was opened with), then store cleanup unchanged |
| FR-017 counters | `Result`, printed by `cost actual --verbose` and always by the import command |
