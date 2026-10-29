# HTTP Contract: Recommendation Lifecycle History

## Endpoint

| Method | Path | Query | Success |
| --- | --- | --- | --- |
| GET | `/api/recommendations/history` | `id` required. Optional `key`, `type`, and `description` match the detail endpoint | 200 `recommendationHistoryResponse` |

`recommendationHistoryResponse`:

| Field | JSON | Source |
| --- | --- | --- |
| Recommendation id | `recommendation_id` | The matched `Recommendation.ID` |
| Events | `events` | `(*engine.Engine).GetRecommendationHistory` |
| Event count | `event_count` | `len(events)` |
| Warnings | `warnings` | The warning slice from that same call |

Empty `events` and `warnings` are arrays, not null. The body goes through
`Server.WriteJSON` (`RedactJSON`).

## Go methods

The handler does not merge or sort.

1. Resolve `id` against the session's presented recommendations, the same
   cache as `GET /api/recommendations/item`. That cache is filled by
   `SessionOptions.Recommendations`.
2. Call `SessionOptions.RecommendationHistory`.
3. The CLI wires that callback to `(*webData).recommendationHistory`.
4. `recommendationHistory` takes the engine from `(*webData).queryEngine`,
   loads the dismissal store with `loadDismissalStore`, and calls
   `(*engine.Engine).GetRecommendationHistory`.
5. The handler writes the events and warnings from that call.

`loadDismissalStore` is the helper `cost recommendations history` uses.
`queryEngine` is the dashboard engine after plugin reload. The callback does
not call `historyPluginOpener`.

## Status

| Condition | Status | Code | Message |
| --- | --- | --- | --- |
| Missing `id` | 400 | `invalid_query` | `recommendation id required` |
| Session or either callback is not ready | 503 | `not_ready` | `recommendation data is loading` |
| Id is not in the presented list, or the row has no recommendation id | 404 | `not_found` | `recommendation not found` |
| `GetRecommendationHistory` returns an error | 502 | `history_unavailable` | `recommendation history unavailable` |
| Events returned, with or without warnings | 200 | - | Response body |

A 502 body is only `{error, code}`. It does not include the Go error.

## CLI parity

`events` equals the `events` array of
`finfocus cost recommendations history <id> --output json` for the same
dismissal store and the same plugin clients. The command-line document has
no `warnings` field. Web adds `warnings` without changing event order.
