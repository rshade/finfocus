# CLI Contract: `finfocus cost history collect-daily`

The Go entry point is `cli.NewCostHistoryCollectDailyCmd`. The run function
is `runDailyCollect`. Mapping is `collectDailyPoints`. Storage is
`history.OpenDailyDB` and `(*history.DailyDB).Replace`.

## Flags

| Flag | Source | Meaning |
| --- | --- | --- |
| `--stack` | parent `cost` | Same stack identity as the other history commands |
| `--from`, `--to` | this command | `YYYY-MM-DD` or RFC3339. Both omitted: last 30 complete UTC days, `[today-30d, today)` |
| `--pulumi-json`, `--pulumi-state`, `--terraform-state` | this command | Mutually exclusive. Discovery is `loadActualResources` |
| `--output` | this command | `table` (default) or `json`. Checked before any stack read |

`--to` alone is a validation error. Only `--from` prices through now.

## Pricing

`(*engine.Engine).GetActualCostWithOptionsAndErrors` with `Resources`,
`From`, and `To` set. Profile is unspecified. No group-by, no
`FallbackEstimate`, no Cloud-history import, no `recordDescriptorHistory`.

## Exit

| Situation | Code |
| --- | --- |
| Summary printed, no resource errors | 0 |
| At least one resource error, after successes are stored | 1 |
| Bad flags, bad dates, bad input combination | 2 (`toValidationError`) |

## Human summary

Stdout states the window, stored resources, stored points, skipped
resources, and failures. Skips and length mismatches also warn on stderr.
JSON stdout is the collect report. JSON field names are `storedResources`,
`storedPoints`, `skippedResources`, `prunedPoints`, `errors`, `from`, `to`,
`stack`, and `project`.

## MCP

`mcpExcludedCommands` includes `{cost, history, collect-daily}` and still
includes `{cost, history, collect}`. Neither name appears in
`internal/cli/testdata/mcp/tools.golden`.

## Web

No method in `internal/webui`. No HTTP route. Not applicable.
