# HTTP API: Statistical Forecast

Extends `POST /api/forecast/query` from `specs/1737-forecast-web-hook/`.

## Request additions

```json
{
  "months": 12,
  "model": "arima",
  "spendHistory": "date,value\n2026-04-01,52.10\n...",
  "confidence": 0.9,
  "growthCap": 1.25,
  "steps": ["2027-06-01=500"],
  "exclude": ["2025-12-01:2025-12-31"]
}
```

## Response additions

`model`, `modelOrder`, and `confidence` at the top level, and the `lower`,
`upper`, `baseline`, and `spend` series, each with `display` strings, exactly
as `cost forecast --model arima --output json` prints them.

## Errors

A malformed CSV row, driver, confidence, or model combination is 400
`invalid_forecast` with the CLI's message, including the CSV line number.

## Go methods the hook calls

| Step | Method | Shared with the CLI |
| --- | --- | --- |
| Parse | `cli.webForecastSettings` → `cli.parseForecastModel` (`forecast.ParseSpendCSV`, `forecast.ParseExclude`, `forecast.ParseStep`, `forecast.ValidateGrowthCap`, `forecast.ValidateConfidence`) | Yes (`forecastFlagSettings` → `parseForecastModel`) |
| Build | `cli.buildForecast` → `cli.statisticalForecast` (`forecast.Project` baseline, `forecast.Drivers.Exclude`, `forecast.ProjectStatistical`), then `forecast.Drivers.Apply` | Yes |
| Document | `forecast.NewDocument`, `forecast.SeriesList` | Yes |
| Display | `webui.forecastDisplay` (`viewmodel.FormatMoney`) | Formatting only |
