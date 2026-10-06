# Contract: `cost forecast` series

The command does not change finfocus-spec. This is the CLI document #550 and
other clients read.

## Plain

An ASCII chart from the forecast points. Optional provider series and a budget
line. A history note and warnings follow the chart when they exist. `--plain`
and `--no-color` omit ANSI colors.

## JSON

One object:

```json
{
  "currency": "USD",
  "months": 12,
  "budget": 2000,
  "series": [
    {
      "name": "history",
      "points": [{ "time": "2026-06-01T00:00:00Z", "value": 180 }]
    },
    {
      "name": "forecast",
      "points": [{ "time": "2026-10-01T00:00:00Z", "value": 200 }]
    },
    {
      "name": "aws",
      "points": [{ "time": "2026-10-01T00:00:00Z", "value": 200 }]
    }
  ],
  "warnings": ["bucket: growth type linear has no rate; cost stays flat"]
}
```

`budget` is omitted when the line is not drawn. `history` is omitted when it
was not loaded. `warnings` is omitted when empty. Provider series are always
present for providers that had a priced resource.

## NDJSON

One JSON object per line, in this order:

1. `{"kind":"meta","currency":"USD","months":12,"budget":2000}`
2. One `{"kind":"point","series":"forecast","time":"...","value":200}` line per
   point, history first when present, then forecast, then each provider.
3. One `{"kind":"warning","message":"..."}` line per warning.

`budget` is omitted from the meta line when the chart would hide it.

## Future chart (#550)

Plot `history` and `forecast` by `time`. Do not recompute growth. When the
user splits providers, replace the forecast line with the provider series.
Draw `budget` as a horizontal line. Do not parse the ASCII chart.
