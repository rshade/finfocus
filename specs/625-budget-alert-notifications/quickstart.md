# Quickstart: Budget Alert Notifications

## 1. Add a destination to a budget alert

In the global `~/.finfocus/config.hujson`. Variable references only expand
from the global config, and only for `FINFOCUS_NOTIFY_*` variables; a
project config is committed, so it may hold literal destinations only:

```jsonc
{
  "cost": {
    "budgets": {
      "global": {
        "amount": 100,
        "currency": "USD",
        "alerts": [
          {
            "threshold": 80,
            "type": "actual",
            "notifications": [
              {"type": "slack", "url": "${FINFOCUS_NOTIFY_SLACK_URL}"}
            ]
          }
        ]
      }
    }
  }
}
```

Check it:

```bash
finfocus config validate
```

## 2. Preview without sending

```bash
export FINFOCUS_NOTIFY_SLACK_URL=https://hooks.slack.com/services/T000/B000/XXXX
finfocus cost projected --pulumi-json plan.json --notify --dry-run
# stderr: dry-run: would notify slack for global budget, 80% actual threshold
```

## 3. Send from CI

```yaml
env:
  FINFOCUS_NOTIFY: "true"
  FINFOCUS_NOTIFY_SLACK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
steps:
  - run: finfocus cost projected --pulumi-json plan.json
```

A run without `--notify` or `FINFOCUS_NOTIFY` never sends. When it would
have sent, it prints one hint on stderr.

## 4. Generic webhook

```jsonc
{"type": "webhook", "url": "https://api.example.com/budget-alert",
 "headers": {"Authorization": "Bearer ${FINFOCUS_NOTIFY_API_TOKEN}"}}
```

The receiver gets the `budget.threshold.exceeded` event described in
`contracts/webhook-event.schema.json`.

## Verify the feature (developers)

```bash
go test ./internal/notification/... ./internal/config/... ./internal/cli/...
go test -run 'TestBudgetNotifications' ./test/integration/...
make lint && make test
```
