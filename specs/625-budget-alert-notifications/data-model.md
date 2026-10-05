# Data Model: Budget Alert Notifications

## NotificationDestination (config, `internal/config`)

One delivery target attached to an alert threshold.

| Field | Type | Required | Applies to | Rules |
| --- | --- | --- | --- | --- |
| `type` | string | yes | all | `slack` or `webhook` |
| `url` | string | yes | all | Literal value: `https://` with a host. With `${NAME}`: checked after expansion at send time |
| (unexported) `source` | global or project | set by loader | all | Not read from JSON. `project` when the destination came from a project config overlay |
| `channel` | string | no | `slack` | Optional channel override, sent as written |
| `method` | string | no | `webhook` | `POST` (default) or `PUT`, case-insensitive |
| `headers` | map string→string | no | `webhook` | Header names must be non-empty; values may use `${NAME}` |

Validation errors use paths such as
`cost.budgets.global.alerts[0].notifications[1].url`. `channel` on a webhook,
or `method`/`headers` on a Slack destination, is an error naming the field
and the type it belongs to.

Secret fields: `url` and every `headers` value.

Variable rules: `${NAME}` is valid only when `NAME` starts with
`FINFOCUS_NOTIFY_`. A destination whose source is `project` must not
contain any `${...}` reference.

## AlertConfig (existing, extended)

| Field | Change |
| --- | --- |
| `notifications` | New: `[]NotificationDestination`, optional. Empty means no notifications for that threshold |

Applies to the alerts of every scope: `global`, `providers.<name>`,
`tags[i]`, and `types.<pattern>`. Default thresholds that a scope receives
when it lists no alerts have no notifications.

## ThresholdStatus (existing engine type, extended)

| Field | Change |
| --- | --- |
| `Notifications` | New: copy of the alert's destinations, `json:"-"` so it never appears in JSON output |

## BudgetAlertEvent (`internal/notification`)

Built once per exceeded threshold in an opted-in run.

| Field | Source |
| --- | --- |
| `Scope` | `global`, `provider`, `tag`, or `type` (`global` for legacy budgets) |
| `ScopeKey` | Provider name, tag selector, or type pattern; empty for global |
| `Amount`, `Currency`, `Period` | Budget configuration (period is always `monthly`) |
| `ThresholdPercent`, `AlertType` | The exceeded alert |
| `ThresholdValue` | `Amount × ThresholdPercent / 100` |
| `Spend`, `SpendPercent` | Current spend and percentage, or forecasted spend and forecast percentage when `AlertType` is `forecasted` |
| `Timestamp` | Evaluation time, UTC |
| `Version` | `version.GetVersion()` |

## DeliveryResult (`internal/notification`)

| Field | Meaning |
| --- | --- |
| `Type` | Destination type |
| `Scope`, `ScopeKey`, `ThresholdPercent`, `AlertType` | Which alert it belongs to |
| `Outcome` | `sent`, `skipped` (unresolved or disallowed variable, project destination with a reference, non-HTTPS after expansion), `failed` (error, non-2xx, 3xx, timeout), or `dry-run` |
| `Reason` | Redacted, secret-free text; empty when sent |

## State transitions

Per destination in one run: `pending → sent | skipped | failed | dry-run`.
Nothing is persisted between runs.
