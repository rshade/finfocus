# History API contract

`GET /api/history/view` uses `cli.readHistoryView` for the requested dates
and currency and `history.ChartBudgets` for monthly threshold amounts. The
CLI uses both functions. The response carries global and provider thresholds
with labels ready for display, along with existing redacted snapshots and
annotations.

`GET /api/history/diff?from=v1&to=v2` uses `history.DiffSnapshots` through
`cli.readHistoryDiff`. The terminal detail uses the same calculation for
adjacent checkpoints in the selected history. The route applies existing
redaction to deployment resource data.
