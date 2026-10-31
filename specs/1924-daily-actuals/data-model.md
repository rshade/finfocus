# Data Model: Daily Actual-Cost Series

## DailyPoint

| Field | Meaning |
| --- | --- |
| `ResourceID` | The actual-cost result id. For Pulumi this is the URN. |
| `Day` | UTC midnight of the calendar day. |
| `Currency` | Plugin currency, or `USD` when the plugin sent a blank. |
| `Amount` | The day's amount. Zero is a real observation. |
| `WrittenAt` | When this collect wrote the point. |

Identity is `(ResourceID, Day, Currency)`. A later put of the same identity
replaces `Amount` and `WrittenAt`. A different currency is a different point.

## File

`<project>@<stack>.daily.db` in the history directory. With no project, the
legacy name is `<stack>.daily.db`, using the same sanitizing as
`CostFileName`.

Bolt layout:

| Bucket / key | Value |
| --- | --- |
| `meta` / `daily_schema` | uint64 `1` |
| `meta` / `project`, `meta` / `stack` | owner, same rules as the timeline |
| `daily_points` / `resource \x00 YYYY-MM-DD \x00 currency` | JSON `{amount, written_at}` |

The timeline keys `version`, `snapshots`, and `annotations` are absent.
`costSchemaVersion` stays `1`.

## Collect report

| Field | Meaning |
| --- | --- |
| `storedResources` | Resources that gained or replaced at least one point |
| `storedPoints` | Points written by this run |
| `skippedResources` | Resources with no daily series |
| `prunedPoints` | Points deleted by the age policy |
| `errors` | `{resource, message}` for plugin failures and bad series |
| `from`, `to` | Window that was priced |

## Relationships

- One stack has one daily file and one projected timeline. They do not share
  rows.
- #1863 reads points. It treats a missing day as no observation.
- Prune deletes a point when its day is strictly before the UTC date
  `today - max_age_days`, and only when `auto_prune` is true and
  `max_age_days > 0`.

## Validation

- Empty resource id, a NUL in the id or currency, or a non-finite amount
  rejects that resource.
- Series length must equal `int(end.Sub(start).Hours() / 24)` and be at
  least 1. Otherwise the resource is an error and none of its points in this
  response are written.
- A symlink at the daily path is rejected before Bolt opens it.
