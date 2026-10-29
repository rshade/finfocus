# Contract: historical carbon report

## Shared Go method

```go
func queryHistoricalCarbon(ctx context.Context, req *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error)
```

`internal/cli/carbon_actual.go`. The command line calls it directly. The web
server receives it as `webui.SessionOptions.HistoricalCarbon`
(`internal/cli/web.go`). The MCP tool is the same command.

## Command

```text
finfocus carbon actual --start YYYY-MM-DD --end YYYY-MM-DD
  [--method market-based|location-based] [--scope total|1|2|3]
  [--account ...] [--region ...] [--service ...] [--output table|json]
```

- `--start` and `--end` are required first-of-month UTC dates; end is exclusive.
- Invalid input exits 2 (`validation_error`).
- A provider failure exits 1 and writes no report to stdout.
- MCP tool: `finfocus-carbon-actual`, read-only.

## HTTP

`POST /api/carbon/actual/query`, active web session required.

Body: `start`, `end`, `method`, `scope`, optional `accounts`, `regions`,
`services`. A success body is the CLI JSON report: resolved `request`,
`records`, and `totalGrams` (null when there are no records).

Errors are `{error, code}` with these codes: `carbon_invalid_query` (400),
`carbon_authentication_required` (401), `carbon_permission_denied` (403),
`carbon_throttled` (429), `carbon_invalid_data` (502), `carbon_provider_error`
(502), `carbon_provider_unavailable` (502), `carbon_disabled` (503),
`carbon_timeout` (504), `carbon_internal_error` (500). Messages are fixed text.

## Plugin

`CostSourceService.GetCarbonEmissions` (finfocus-spec v0.7.10), called only when
`GetPluginInfo` lists `PLUGIN_CAPABILITY_HISTORICAL_CARBON`.
