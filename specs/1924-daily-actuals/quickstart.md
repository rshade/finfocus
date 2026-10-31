# Quickstart: Daily Actuals

Record the last 30 complete UTC days after a deploy:

```bash
finfocus cost history collect --stack dev --versions 1
finfocus cost history collect-daily --stack dev
```

Correct one week without dropping older days:

```bash
finfocus cost history collect-daily --stack dev --from 2026-10-01 --to 2026-10-08
```

`--to` on a calendar date is midnight at the start of that date, matching
`finfocus cost actual`.

JSON for a job log:

```bash
finfocus cost history collect-daily --stack dev --output json
```

The file is `~/.finfocus/history/<project>@<stack>.daily.db`. The projected
file `<project>@<stack>.history.db` is not modified. Set
`cost.history.retention.auto_prune` and `cost.history.retention.max_age_days`
to drop daily points older than that many days. `max_snapshots` still applies
only to projected checkpoints.

A plugin that returns only a window total is counted as skipped. Nothing is
invented for the missing days.
