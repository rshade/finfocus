# HTTP Contract: Cost History

## Endpoints

| Method | Path | Query | Success |
| --- | --- | --- | --- |
| GET | `/api/history/view` | Optional `from`, `to` (`YYYY-MM-DD` or RFC 3339), `currency`, `strict` (`true`/`false`) | 200 `HistoryViewResponse` |
| GET | `/api/history/diff` | `from` required (`v35`, `35`, or `YYYY-MM-DD`); optional `to` (default newest), `threshold` (number, default 0) | 200 `HistoryDiffResponse` |
| GET | `/api/history/resource` | `id` required (the row's resource id, usually a URN) | 200 `HistoryResourceResponse` |

Every body goes through `Server.WriteJSON` (`RedactJSON`). Empty lists are
arrays, not null.

### HistoryViewResponse

| Field | JSON | Source |
| --- | --- | --- |
| Status | `status` | `ok` or `no_history` |
| Document | `stack`, `snapshots`, `annotations`, `currency_warning` | `history.ViewDocument` from `readHistoryView` |
| Budget | `budget` | `globalBudget(cfg)`; omitted when unset |
| Display | `display.budget`, `display.snapshots[]` (`version`, `date`, `total`) | `viewmodel.FormatMoney` per snapshot currency |

### HistoryDiffResponse

| Field | JSON | Source |
| --- | --- | --- |
| Status | `status` | `ok` or `no_history` |
| Document | the `cost history diff --output json` fields | `history.NewDiffDocument` from `readHistoryDiff` |
| Display | `display.delta`, `display.from`, `display.to`, `display.unchangedTotal`, `display.added[]`/`removed[]`/`changed[]` (`monthlyCost`, `delta`) | `viewmodel.FormatMoney`; a positive delta gets `+` |

### HistoryResourceResponse

| Field | JSON | Source |
| --- | --- | --- |
| Status | `status` | `ok` or `no_history` |
| Id | `id` | The request `id` |
| Points | `points[]` (`timestamp`, `amount`, `currency`) | `loadDetailHistory` |
| Display | `display.points[]` (`date`, `amount`) | `viewmodel.FormatMoney` |

## Go methods

The handlers do no history arithmetic. Each one parses the query, calls one
`SessionOptions` callback inside `Session.beginQuery`, and formats display
strings.

| Endpoint | `SessionOptions` callback | CLI wiring | Shared Go function (also called by the CLI) |
| --- | --- | --- | --- |
| `/api/history/view` | `HistoryView` | `(*webData).historyView` | `readHistoryView` → `historyViewBounds`, `(*history.CostDB).Snapshots`, `.Annotations`, `selectViewCurrency` → `history.SelectCurrency`, `annotationsForSnapshots` (`runView`) |
| `/api/history/diff` | `HistoryDiff` | `(*webData).historyDiff` | `readHistoryDiff` → `checkDiffInputs`, `(*history.CostDB).Snapshots`, `history.ResolveCostSnapshot`, `history.DiffSnapshots`, `history.NewDiffDocument` (`runDiff`) |
| `/api/history/resource` | `HistoryResource` | `(*webData).historyResource` | `loadDetailHistory` (`overviewHistoryLoader`, `costActualHistoryLoader`) |

`historyView` and `historyDiff` resolve the database with
`resolveHistoryTargetFor(dir="", stack, currentProjectName())`, which applies the history-directory
rules (`historyDirectory`: `FINFOCUS_HISTORY_DIR`, then
`cost.history.directory` with the project-tier gate, then
`<config dir>/history`) and Terraform identity detection. They open it with
`openHistoryTargetReadOnly` (`history.OpenCostDBRead` plus
`diffStackMatches`), never with the CLI's labeling opener. The stack is
`(*webData).forecastStack`, read under the query gate and released before
any file is opened. Terraform identity comes from
`terraformHistoryTargetContext` with `cost.terraform.include_workspace`,
as `resolveHistoryCommandTarget` does without the flag.

## Status

| Condition | Status | Code | Message |
| --- | --- | --- | --- |
| `GET /api/history/resource` with no `id` | 400 | `invalid_query` | `resource id required` |
| Non-boolean `strict` or non-numeric `threshold` | 400 | `invalid_history_query` | `invalid strict value "<v>" (use true or false)` / `invalid threshold "<v>"` |
| `history.IsQueryError` (bad date, reversed range, missing `from`, negative threshold, bad or unknown selector, strict mixed currencies, diff across currencies) | 400 | `invalid_history_query` | The CLI's message for the same input |
| Session or callback not ready | 503 | `not_ready` | `cost history is loading` |
| `history.IsNoHistory` (`ErrNoCostHistory`, `ErrNoSnapshots`) | 200 | - | `{status: "no_history"}`, plus `id` for `/api/history/resource` |
| Any other error (unreadable or corrupt database, stack mismatch) | 502 | `history_unavailable` | `cost history unavailable` |

A 502 body is only `{error, code}`. The Go error, which can name a file path,
goes to the terminal log at warn and never into the response.

## CLI parity

- The view document equals `finfocus cost history view --stack <stack>
  --output json` with the same `--from`, `--to`, `--currency`, `--strict`.
- The diff document equals `finfocus cost history diff --stack <stack>
  --output json` with the same `--from`, `--to`, `--threshold`.
- The resource points equal what the overview and cost TUI detail views load.

## Not applicable

`cost history collect`, `prune`, and `export` have no endpoint: they write or
delete the store, or write a file. The dashboard is read-only toward history.
