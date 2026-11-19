# History API contract

`GET /api/history/view` returns checkpoints, their provider amounts,
annotations, and budget thresholds through `cli.readHistoryView` and
`history.ChartBudgets`.

`GET /api/history/diff?from=v35&to=v36` returns the comparison through
`cli.readHistoryDiff` and `history.DiffSnapshots`. Deployment selection
uses these existing routes. Both retain the dashboard's redaction.
