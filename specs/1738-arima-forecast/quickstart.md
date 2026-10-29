# Quickstart: Statistical Cost Forecast

## Spend CSV

```text
date,value
2026-04-01,52.10
2026-04-02,51.80
```

One row per consecutive day, or one per consecutive month, at least ten
rows, values in the plan's currency.

## CLI

```bash
finfocus cost forecast --pulumi-json plan.json --model arima \
  --spend-history spend.csv --months 12 \
  --exclude 2025-12-01:2025-12-31 --step 2027-06-01=500 --growth-cap 1.25 \
  --output json
```

## Tests

```bash
go test ./internal/forecast/ -run 'Spend|Driver|Statistical'
go test ./internal/cli/ -run 'Forecast'
go test ./internal/webui/ -run 'Forecast'
```

## Render evidence

2026-10-28, Playwright (headless Chromium) against
`finfocus --web --no-browser --port 18766` with the `testdata/overview`
mixed-changes plan and state (aws-public v0.1.7). `../finfocus-demo` was not
usable: the local Pulumi login is an expired ephemeral agent account.

- The Overview view rendered with its header links and budget panel; the
  Forecast view's model controls fit its toolbar as the spec's Web UI Fit
  describes.
- `POST /api/forecast/query` with 180 days of spend CSV text,
  `{"months":6,"model":"arima","steps":["2027-01-01=300"],"growthCap":1.5,
  "noHistory":true}` returned 200. With `display` removed, the body equaled
  `finfocus cost forecast --model arima --spend-history spend.csv --months 6
  --step 2027-01-01=300 --growth-cap 1.5 --no-history --output json` on the
  same files (`SARIMA(4,1,2)(0,1,1)[7]`, 95% confidence). Display strings
  read `Oct 2026` / `$2,019.18 USD`.
- A CSV with a missing day returned 400 `invalid_forecast` with
  `spend history line 3: 2026-04-03 does not follow 2026-04-01; rows must be
  consecutive days or months`.
- The fixture plan has no priced resources, so the baseline series was zero.
