# Data Model: Recommendation Lifecycle History on the Web

## Recommendation identity

The history id is `Recommendation.ID`, the same value stored as the
dismissal id. The browser may only ask for an id that the last successful
recommendations query presented. Optional `key`, `type`, and `description`
query parameters use the detail endpoint's match. An empty `Recommendation.ID`
is not a history id.

## Lifecycle event

Unchanged `config.LifecycleEvent`:

| Field | Meaning |
| --- | --- |
| `action` | Recorded action, such as dismissed or snoozed |
| `reason` | Reason code |
| `custom_reason` | Free-text note, omitted when empty |
| `timestamp` | When the event was recorded |
| `expires_at` | Snooze expiry, omitted when unset |
| `actor` | Who recorded it, omitted when empty |

Order is the order `GetRecommendationHistory` already returns: oldest first.
The web handler does not sort.

## History warning

One string per plugin that was asked and failed, in client order. A plugin
that returns `Unimplemented` is absent. The string is
`<plugin name>: recommendation history unavailable` and does not contain the
plugin error.

## Response

| Field | Meaning |
| --- | --- |
| `recommendation_id` | The id that was looked up |
| `events` | Lifecycle events, empty array when none |
| `event_count` | Length of `events` |
| `warnings` | Plugin warnings, empty array when none |

The command-line JSON document stays `recommendation_id`, `events`, and
`event_count` only.

## State

No new store. The dismissal file is read and not written. The presented
recommendation list stays the session cache recommendation detail already
uses.
