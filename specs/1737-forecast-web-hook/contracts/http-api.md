# HTTP API: Cost Forecast

## `POST /api/forecast/query`

Authenticated like every other dashboard POST: session cookie, same
`Origin`, `Content-Type: application/json`, `Sec-Fetch-Site` same-origin.

### Request

```json
{
  "months": 12,
  "growthType": "linear",
  "growthRate": "0.10",
  "noHistory": false
}
```

Every field is optional. Omitted `months` is 12. Omitted `growthType` uses
each plugin's growth model.

### Response 200

```json
{
  "currency": "USD",
  "months": 12,
  "budget": 500,
  "series": [
    {"name": "history", "points": [{"time": "2026-09-01T00:00:00Z", "value": 120}]},
    {"name": "forecast", "points": [{"time": "2026-10-01T00:00:00Z", "value": 130}]},
    {"name": "aws", "points": [{"time": "2026-10-01T00:00:00Z", "value": 130}]}
  ],
  "warnings": ["..."],
  "display": {
    "budget": "$500.00 USD",
    "series": [
      {"name": "forecast", "points": [{"month": "Oct 2026", "value": "$130.00 USD"}]}
    ]
  }
}
```

The top-level fields are exactly the `cost forecast --output json` document
for the same stack and settings. `display.series` follows `series` order.

### Errors

| Status | `code` | When | `error` text |
| --- | --- | --- | --- |
| 400 | `invalid_request` | Body is not JSON | Server's standard text |
| 400 | `invalid_forecast` | Months out of range, unknown growth type, bad or missing rate | The CLI's message for the same flags |
| 422 | `no_forecast` | No resource was priced, or priced resources use two currencies | The CLI's message (`no cost data available`, or the mixed-currency message naming the codes). When unchanged resources were left out of the forecast, the web adds how many to `no cost data available`. |
| 503 | `not_ready` | Session data not loaded | `forecast data is loading` |
| 502 | `forecast_unavailable` | Pricing or history setup failed | `forecast data unavailable` |

## Go methods the hook calls

| Step | Method | Shared with the CLI |
| --- | --- | --- |
| Handler | `(*webui.Server).handleForecastQuery` | No (transport only) |
| Callback | `webui.SessionOptions.Forecast` = `(*cli.webData).forecast` | No (session wiring) |
| Pricing | `(*engine.Engine).GetProjectedCostDiff(ctx, resources).AfterCosts()` on the session query engine | Yes (`forecastCosts`) |
| Membership | `cli.planOriginResources`, `cli.ApplyFilters`, `cli.resolveResourceTypes` | Yes |
| Input parse | `forecast.ValidateMonths`, `forecast.ParseGrowth` | Yes (`runForecast`) |
| Build | `cli.buildForecast(costs, forecastSettings, readHistory)` | Yes (`runForecast`) |
| Projection | `forecast.Project(resources, months, now)` via `cli.forecastResources` | Yes |
| History | `cli.readForecastHistory(stack)`, `cli.forecastHistoryFrom` | Yes |
| Budget | `cli.forecastBudgetFor(cfg, currency, noBudget)` | Yes |
| Document | `forecast.NewDocument(projection, forecast.SeriesList(...), budget)` | Yes (CLI JSON) |
| Display | `viewmodel.FormatMoney` | Formatting only |
