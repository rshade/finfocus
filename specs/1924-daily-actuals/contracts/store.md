# Store Contract: `history.DailyDB`

`internal/history` does not import `engine` or `ingest`.

## API

- `DailyFileName` / `DailyFileNameFor` name the file.
- `PointsFromObservations(resourceID string, observed []ObservedCost, written time.Time) ([]DailyPoint, error)`
  folds timestamped buckets into one point per UTC day and currency. An empty
  result returns no points and no error. A non-finite amount returns an error
  and no points.
- `OpenDailyDB(path, project, stack string) (*DailyDB, error)` creates or
  opens the file. A symlink or other non-regular existing path fails. A
  projected timeline fails with `ErrTimelineAsDaily`.
- `(*DailyDB).Replace(points []DailyPoint) error` upserts those identities
  and leaves every other point in place.
- `(*DailyDB).Points() ([]DailyPoint, error)` returns every point, sorted by
  resource, day, then currency.
- `(*DailyDB).PruneBefore(day time.Time) (int, error)` deletes points whose
  UTC day is strictly before `day`.
- `(*DailyDB).Close() error` closes the file once.

`OpenCostDB` / `OpenCostDBFor` on a daily file returns `ErrDailyFileAsTimeline`
and does not add timeline buckets.

## Schema

`dailySchemaVersion` is 1. A different value fails with
`unsupported daily actuals schema version`. `costSchemaVersion` is unchanged.
