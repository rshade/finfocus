# Data Model: Recommendation History

## Lifecycle event

Stored locally as `config.LifecycleEvent` and returned by the history command.

| Field | Local JSON | Plugin source | Rules |
| --- | --- | --- | --- |
| Action | `action` | `action` string, unchanged | Required for a useful row. Empty is still shown. Not mapped to an enum. |
| Reason | `reason` | `reason` | Copied as text. |
| Note | `custom_reason` | `custom_reason` | Omitted from JSON when empty. |
| Time | `timestamp` | `timestamp` | Missing stays the zero time and sorts after dated events. Present but invalid drops the event. |
| Expiry | `expires_at` | `expires_at` | Absent means no expiry. Present but invalid drops the event. |
| Actor | `actor` | `actor` | New optional field. Omitted from JSON when empty. Older files have no field and load as empty. |

## Local dismissal record

Unchanged aside from events that may now carry `actor`. History reads copy
the event slice before sorting. They do not call save.

## Plugin history response

One recommendation id in, zero or more lifecycle events out. Event order from
a plugin is not significant. Core sorts after the merge.

## Merge identity

Two events are the same change when action, reason, note, time, and expiry
match. Actor is compared only after that match:

- All blank actors collapse to one event.
- A blank actor is dropped when the same change has a named actor.
- Each distinct non-empty actor is kept.

## Ordering

1. Events with a time, oldest first.
2. Events with no time.
3. Ties break by action, reason, note, expiry, then actor, using a stable sort.
