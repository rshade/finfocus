# CLI Contract: `finfocus cost anomalies`

- Use: `anomalies`, no positional arguments (`cobra.NoArgs`). ax-go tool name
  `finfocus-cost-anomalies`.
- Flags: `--pulumi-json`, `--pulumi-state`, `--from`, `--to`, `--adapter`/`-a`,
  `--filter`/`-f` (repeatable), `--output` (table, json, ndjson). `--stack`,
  `--profile`, and `--budget-scope` come from the `cost` parent; `--format`,
  `--dry-run`, `--yes` come from `ax.Execute`.
- Go: `cli.NewCostAnomaliesCmd`, `engine.SelectAnomalies`,
  `engine.RenderAnomaliesAsTable|JSON|NDJSON`.
- Exit codes: 0 success including an empty list; 2 for an unknown `--output`,
  bad date, or bad filter; 1 for a pipeline failure.
- JSON: `{"considered": N, "anomalies": [ ... ]}`. NDJSON: one anomaly object
  per line. Table: columns RESOURCE, TYPE, BASELINE, OBSERVED, DEVIATION,
  DRIFT %, CURRENCY, with a trailing `N of M resources` line.
