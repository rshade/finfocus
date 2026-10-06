# Quickstart: Cost Forecast

Price the current plan, then project 12 months with 10% linear growth on
every resource:

```bash
finfocus cost forecast --pulumi-json plan.json --growth-type linear --growth-rate 0.10
```

Leave the plugin growth model in place and supply only the rate. Resources the
plugin marks `none` stay flat. Resources it marks `linear` or `exponential`
use the rate.

```bash
finfocus cost forecast --stack prod --months 6 --growth-rate 0.05 --split-providers
```

Machine-readable points, including history when that stack has a database:

```bash
finfocus cost forecast --pulumi-json plan.json --growth-rate 0.05 --output json
finfocus cost forecast --pulumi-json plan.json --growth-rate 0.05 --output ndjson
```

Hide the budget line or skip history:

```bash
finfocus cost forecast --pulumi-json plan.json --no-budget --no-history --plain
```

`--months` accepts 1 through 36. The chart starts at the current month.
