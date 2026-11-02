# Research: Daily Actual-Cost Series

## Decision: Separate file, schema version 1 stays on the timeline

`CostDB` (`internal/history/cost_db.go`) is schema version 1 and refuses any
other version. `CostSnapshot` is one projected checkpoint. Adding a bucket
there would force every reader through a migration.

The daily series is `<project>@<stack>.daily.db` beside
`<project>@<stack>.history.db`. Its own meta key is `daily_schema` (version
1). `OpenCostDB` returns an error when that key is present, before it creates
timeline buckets. `OpenDailyDB` returns an error when the file has the
timeline `version` key or a `snapshots` bucket. An old timeline still opens,
and a file whose timeline version is not 1 still reports
`unsupported cost history schema version`.

## Decision: Reuse the history-directory jail

`historyDatabaseFilesSafe` already rejects a symlink or non-regular file
whose name is `history.db*` or contains `.history.db`. The same function
gains the `.daily.db` names. Directory resolution stays
`resolveHistoryDir` plus `projectHistoryDirectoryRecheckFails` (the path
`initHistoryFromConfig` uses). No second copy of the absolute / `..` /
symlink rules.

## Decision: Store plugin timestamps, not the even split

`deriveActualCostWindow` fills `CostResult.DailyCosts` with equal shares of
`TotalCost` whenever the window is at least one day. That is what
`finfocus cost actual` shows. It is not the plugin series. The plugin sends
`ActualCostResult` buckets with `timestamp`, `cost`, and a FOCUS currency.
The adapter keeps those buckets on `Observations`. The collector groups them
by UTC day and currency. A bucket with no timestamp is not a day. An empty
observation list stores nothing and does not split `TotalCost`.

`RetainDailyObservations` on the actual-cost request adds `daily_observations=v1`
to the cache key. `cost actual` leaves the flag unset, so its key is unchanged.

## Decision: Retention uses `cost.history.retention` only for age

`CostHistoryRetention.MaxAgeDays` is "delete older than N days" and
`AutoPrune` already gates projected collect. When both say to prune, daily
points whose UTC day is strictly before today minus `max_age_days` are
deleted. `MaxSnapshots` counts checkpoints, so it does not apply.
`retention_days` remains resource-observation cleanup.

## Decision: Command shape

`cost history collect-daily` sits next to `collect`. Flags are `--from`,
`--to`, `--pulumi-json`, `--pulumi-state`, `--terraform-state`, and
`--output` (`table` or `json`). `--stack` is the parent `cost` flag.
Discovery calls `loadActualResources`. Pricing calls
`GetActualCostWithOptionsAndErrors` with resources and the window only: no
group-by, no fallback estimate, no usage profile, no Cloud-history import,
and no observation-history write.

The command is added to `mcpExcludedCommands` for the same reason as
`cost history collect`: it runs for minutes and would stall the one MCP
dispatcher lock.

## Decision: Web parity

Not applicable. No `internal/webui` handler. No follow-up issue.

## Alternatives considered

- A bucket inside `*.history.db`: rejected. It bumps or overloads schema
  version 1 and couples billing failure to checkpoint collection.
- Spreading `TotalCost` when `DailyCosts` is absent: rejected. A gap must
  stay "no observation" for #1863.
- A new `cost.history.daily_retention` tree: rejected. `max_age_days` already
  expresses the age rule.
- Running collection from the dashboard: rejected by the issue and by
  Principle VII's exemption for a command with no web surface.
