# Contract: Notification Configuration

Location: `cost.budgets.<scope>.alerts[].notifications[]` in `config.hujson`
(global or project).

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
              {"type": "slack", "url": "${FINFOCUS_NOTIFY_SLACK_URL}", "channel": "#finops-alerts"},
              {
                "type": "webhook",
                "url": "https://api.example.com/budget-alert",
                "method": "POST",
                "headers": {"Authorization": "Bearer ${FINFOCUS_NOTIFY_API_TOKEN}"}
              }
            ]
          }
        ]
      },
      "providers": {
        "aws": {
          "amount": 50,
          "alerts": [
            {"threshold": 100, "type": "forecasted",
             "notifications": [{"type": "webhook", "url": "${FINFOCUS_NOTIFY_OPS_HOOK}"}]}
          ]
        }
      }
    }
  }
}
```

## Variable and source rules

| Rule | Behavior |
| --- | --- |
| `${NAME}` with `NAME` starting `FINFOCUS_NOTIFY_`, global config | Expanded at send time |
| `${NAME}` with any other name | Validation error; never expanded; destination skipped at send time |
| Any `${...}` in a project config destination | Validation error ("move this destination to the global config"); destination skipped at send time |
| Literal values in a project config destination | Sent as written |

## CLI surface

| Surface | Contract |
| --- | --- |
| `--notify` | Boolean. Persistent on `cost` (used by `cost projected` and `cost actual`); local on `overview` |
| `FINFOCUS_NOTIFY` | Fills an unset `--notify` (`strconv.ParseBool`). Invalid values warn once and count as false |
| `--dry-run` | With an opted-in run: no requests; one stderr line per would-be delivery |
| stderr hint (not opted in) | `budget notifications for N exceeded threshold(s) were not sent; pass --notify or set FINFOCUS_NOTIFY=true` |
| stderr warning (failure) | `warning: <type> notification for <scope> budget (<threshold>% <alert type>) failed: <redacted reason>` |
| stdout | Never written by notifications |
| exit code | Never changed by notifications |
| `config get` / `config list` | `url` and header values under `notifications` are shown as `[REDACTED]` unless the value is exactly one `${NAME}` reference |
