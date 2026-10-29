# Quickstart: Recommendation Lifecycle History on the Web

## Command line

```bash
finfocus cost recommendations history <recommendation-id> --output json
```

The JSON document has `recommendation_id`, `events`, and `event_count`.
Events are oldest first. A plugin that does not implement history is skipped.
A plugin that fails is logged and does not remove local events.

## Browser session

After Recommendations has loaded, request:

```text
GET /api/recommendations/history?id=<recommendation-id>
```

The `events` array matches the command-line JSON. `warnings` is empty when
every plugin answered or skipped as unimplemented. A plugin failure adds
`<plugin>: recommendation history unavailable` and still returns the events.

An id that was not in the latest recommendations list is `404`. A missing id
is `400`. The Recommendations page does not render this list yet.

## Checks

```bash
go test -count=1 -run 'RecommendationHistory|GetRecommendationHistory' ./internal/engine/ ./internal/webui/ ./internal/cli/
make test
make lint
make validate
make test-e2e-web
```
