# Data Model: Cost in the Editor for VS Code and Its Forks

## Core (Go)

### Location (`internal/sourcepos`)

| Field | Type | Rule |
| --- | --- | --- |
| `File` | string | Project-relative, forward slashes, no `..`, names a regular file under the project root |
| `Line` | int | 1-based, > 0 |
| `Column` | int | 1-based; 0 when the runtime records none (Go) |

JSON: `{"file":"lib/box.js","line":8,"column":5}`; `column` omitted when 0.
Omitted when no frame resolves (FR-003).

### Raw position fields (`internal/ingest`)

`StackExportResource` and `PulumiState` gain `SourcePosition string` and
`StackTrace []StackFrame{SourcePosition string}`, read as Pulumi writes them.
They never leave `stackload`.

### Engine records

`StateResource`, `PlanStep`, and `OverviewRow` gain
`Location *sourcepos.Location` (`json:"location,omitempty"`).

- State rows take the state resource's location.
- A plan step's location wins over state for `create`, `update`, `replace`,
  and `same` (the preview reflects the current code); a `delete` step keeps
  the state location.
- A preview step with no resolvable current location omits it rather than
  using a deployed location that may now identify a different line.
- `StateResource` and `OverviewRow` also carry `ModifiedAt *time.Time`,
  serialized as `modifiedAt,omitempty` on rows. Copy from deployed state
  through merge; preserve for updates/replacements, omit for new resources.
  It is not inferred from arbitrary provider properties.
- Cluster expansion children have no location or modification timestamp.
- Not part of any cache key or plugin request.

### JSON overview additions

See [contracts/overview-json.md](contracts/overview-json.md).

## Extension (JavaScript)

### OverviewResult

Parsed from one CLI run.

| Field | Source |
| --- | --- |
| `stack`, `generatedAt`, `stateOnly` | `metadata.stackName`, `metadata.generatedAt`, `metadata.isStateOnly` (absent means false) |
| `rows[]` | `resources[]`: `urn`, `type`, `status`, `location`, `display`, `modifiedAt` |
| `summaryDisplay` | `summary.display` |
| `budgetDisplay` | `budgetDisplay` |
| `errors[]` | `errors[]` (row-level, shown in the tree) |
| `producedAt` | time the run started (for stale comparison) |

### CacheEntry (per ContextKey, state-only only)

| Field | Meaning |
| --- | --- |
| `contextKey` | canonical project URI plus stack reference |
| `lastUpdate`, `resourceCount` | from discovery; absent or `updateInProgress` means no entry |
| `stackConfigHash`, `projectConfigHash` | SHA-256 of `Pulumi.<stack>.yaml` and `.finfocus/config.hujson` (empty when absent) |
| `finfocusVersion`, `stateOnly` | as run |
| `storedAt`, `producedAt` | expiry clock; original run start (stale comparison) |
| `confirmedGaps` | URNs that were unpriced (gaps) in two runs agreed on |
| `stdout` | finfocus's JSON, verbatim |

The last stored entry's `confirmedGaps` is also the context's baseline, even
when the entry has expired. Valid when every key field equals the current value, `now - storedAt` is within
the TTL, and `stdout` parses. Never holds a passphrase or environment.

### Completeness

Derived per result: `gaps` (URNs whose `status` is not `deleting` and whose
`display.projected` is `-`), `total` (rows), and one of `complete` (no gaps),
`confirmed` (gaps equal to the baseline or to a second run's), or `incomplete`
(gaps differ between runs, or include rows the baseline had priced). Only
`incomplete` changes what the card says about the total, and only `complete`
and `confirmed` results are written to the cache.

### RunFailure

From the ax-go envelope on stderr (stdout is empty) when the exit code is non-zero:
`message` (shown verbatim), `category` (the parenthesized `stackload` token,
for example `passphrase_required`), `version`.

### ContextKey and RequestSnapshot

`ContextKey = (canonical project URI including remote authority, stack
reference returned by discovery)`. Never key a map by the short stack name
alone. Results, passphrases, and scheduler state live in separate maps per
context. Selections live in memory per project. A backend/configuration change
invalidates that project's contexts and credentials and reruns discovery.

Each request captures a monotonically increasing request ID, selection epoch,
per-file edit generations and dirty flags, and the stack-config generation.
Generations increase for editor edits and external file changes, including
renames/deletions; stack settings changes invalidate all rows. Recheck file
state when a result arrives. Only the latest request for the still-selected
context may publish a result or show a passphrase prompt.

### StaleSet

Per context: map of URN to reasons (`edited`, `unsaved`, `config changed`,
`freshness unknown`). It never triggers a run itself.

1. File edits mark every row declared in that file stale, including unsaved
   edits. Stack settings changes mark every row stale.
2. A deployed result recomputes initial freshness from declaring-file mtime
   versus valid `modifiedAt`. Missing/invalid timestamps or missing/unreadable
   files mean freshness unknown. Unsaved changes always remain stale.
3. A successful preview clears a covered URN only if its declaring file was
   clean at request start, remains clean, its edit generation is unchanged,
   and the stack-config generation is unchanged. Any edits during the run
   stay stale even if saved before completion. Rows without a location cannot
   establish file freshness; show freshness unknown.
4. State-only refresh cannot establish that local changes were priced. It
   preserves edit/config marks until a covering preview, while recomputing
   timestamp-based freshness. Failed/cancelled/superseded runs clear nothing.
5. Manual Save all and run waits for successful saves before capturing the
   request snapshot; failed saves abort the run. Run saved files leaves dirty
   resources stale. On-save captures the just-saved file and does not save
   other files. A newer save supersedes the older request.

### RunState (per ContextKey)

```text
idle ──refresh──▶ cache hit? ──yes──▶ idle(result)        (no process)
idle ──refresh──▶ cache miss ──▶ running(state-only) ──ok──▶ idle(result)
  │                    │──fail──▶ idle(previous result, failure shown)
  └──preview──▶ running(preview) ── same transitions
running + new request ──▶ cancelling ──exit──▶ running(new request)
passphrase_required ──▶ awaitingPassphrase ──answer──▶ running(same request)
                                          └─dismiss──▶ idle(previous result)
```

A state-only result with unpriced rows and no matching baseline goes
`running` once more before it is published (see Completeness). Only one
overview process per context. Selection changes invalidate the old
request immediately, then run bounded cleanup before starting the next
selection. Discovery and version checks obey the same trust and disposal
rules. A cleanup failure enters `cleanupFailed` and blocks replacement runs
until cleanup succeeds or the extension is reloaded. Untrusted workspace: no
transitions out of `idle`. See the CLI contract for deadlines and escalation.
