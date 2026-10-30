# Data Model: Cost History on the Web

All values come from `internal/history` and `internal/cli`. The web adds a
status and display strings only.

## history.ViewDocument (moved from `cli.costHistoryView`)

| Field | JSON | Notes |
| --- | --- | --- |
| Stack | `stack` | Short stack name stored in the database |
| Snapshots | `snapshots` | `[]CostSnapshot` after date and currency selection |
| Annotations | `annotations` | `[]CostAnnotation` for the kept snapshot versions |
| CurrencyWarning | `currency_warning,omitempty` | `SelectCurrency` warning |

`CostSnapshot.ByProvider`, `CostSnapshot.ByType`, and
`CostAnnotation.ResourceChanges` carry `datakey:"true"`.

## history.DiffDocument (moved from `cli.costDiffJSON`)

Unchanged fields: `stack`, `from`/`to` (`version`, `timestamp`,
`total_monthly`), `delta`, `currency`, `added`/`removed`/`changed` (`urn`,
`type`, `monthly_cost`, `delta`, `sku`, `region`), `unchanged_count`,
`unchanged_total`. Built by `history.NewDiffDocument(stack, CostDiff)`.

## Errors (internal/history)

| Value | Meaning | Web result |
| --- | --- | --- |
| `ErrNoCostHistory` | No database for the stack, or no stack | 200 `status: no_history` |
| `ErrNoSnapshots` | Database has no snapshots to diff | 200 `status: no_history` |
| `*QueryError` | Caller mistake; `Error()` is the CLI message | 400 `invalid_history_query` |
| other | Open, read, or stack-mismatch failure | 502 `history_unavailable` |

## webui queries

- `HistoryViewQuery`: `From`, `To`, `Currency` (strings), `Strict` (bool).
- `HistoryDiffQuery`: `From`, `To` (selectors), `Threshold` (float64).

## webui responses

- `HistoryViewResult` (callback): `Document`, `Budget` (nil when unset),
  `BudgetCurrency`.
- `HistoryViewResponse`: `status`, embedded `history.ViewDocument`,
  `budget` (omitted when no global budget), `display` (`budget`, and per
  snapshot `version`, `date`, `total`).
- `HistoryDiffResponse`: `status`, embedded `history.DiffDocument`, `display`
  (`delta`, `from`, `to`, `unchangedTotal`, and per item `monthlyCost`,
  `delta`).
- `HistoryResourceResponse`: `status`, `id`, `points` (`timestamp`,
  `amount`, `currency`), `display.points` (`date`, `amount`).

With `status: no_history`, only `status` is present, plus `id` for the
resource endpoint.

## Lifecycle

Read-only. No entity is created, updated, or deleted by the dashboard.
