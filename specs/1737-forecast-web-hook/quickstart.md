# Quickstart: Cost Forecast Web Hook

## Run the tests

```bash
go test -run 'Forecast' ./internal/forecast/ ./internal/cli/ ./internal/webui/
make test
```

## Query a running session

```bash
cd ../finfocus-demo
PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port 8765
```

Open the printed URL once in a browser to set the session cookie, then call
the endpoint from the browser console:

```js
await (await fetch('/api/forecast/query', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({months: 6, growthType: 'linear', growthRate: '0.05'}),
})).json()
```

Compare with the CLI from the same project:

```bash
finfocus cost forecast --stack dev --months 6 \
  --growth-type linear --growth-rate 0.05 --output json
```

The `series`, `currency`, `budget`, and `warnings` fields must match.

## Render evidence

2026-10-28, Playwright (headless Chromium) against
`finfocus --web --no-browser --port 18765` started with
`--pulumi-json testdata/overview/plan-mixed-changes.json` and
`--pulumi-state testdata/overview/state-mixed-changes.json` (aws-public
v0.1.7 installed). `../finfocus-demo` was not usable: the local Pulumi login
is an expired ephemeral agent account.

- The Overview view rendered with the header links Overview, Cost,
  Recommendations, Estimate, Plugins, Config and the "Budget health" panel
  (no budget configured). A Forecast view fits as a new header link with the
  slots listed in the spec's Web UI Fit section.
- `POST /api/forecast/query` with `{"months":3,"growthType":"linear",
  "growthRate":"0.10","noHistory":true}` returned 200. With `display`
  removed, the body matched `finfocus cost forecast --months 3
  --growth-type linear --growth-rate 0.10 --no-history --output json` on the
  same files field for field, including all eight warnings.
- `{"months":37}` returned 400 `invalid_forecast` with
  `forecast months must be from 1 to 36`, the CLI's text.
- The fixture resources carry no region, so aws-public declined to price
  them and every value was `$0.00 USD`; priced values are covered by
  `TestWebForecast_MatchesCLIJSON`.
