# Research: Recommendation Lifecycle History on the Web

## Decision: Reuse `GetRecommendationHistory`

The command already merges local dismissal events with every connected
plugin and sorts them oldest first. The web callback calls that method and
returns its events. The handler does not sort or merge.

## Decision: Warnings are a second result, not a merge change

Plugin failures are already logged and skipped, including `Unimplemented`,
which contributes nothing. A failure other than `Unimplemented` now also
returns one fixed warning, `<plugin>: recommendation history unavailable`.
The plugin error text stays in the log. `mergeRecommendationHistory` is
unchanged. The command-line renderer ignores the warning slice, so table,
JSON, and NDJSON stay the same.

## Decision: The dashboard engine, not a second plugin open

`cost recommendations history` opens plugins through `historyPluginOpener`.
The dashboard already holds clients on the query engine. The web callback
takes that engine from `queryEngine` and loads the dismissal store with
`loadDismissalStore`. It does not call `historyPluginOpener`.

## Decision: Same id gate as recommendation detail

`GET /api/recommendations/item` only returns an id from the last successful
recommendations query, and loads that query once when none has run. History
uses that same cache and the same row match (id, and the optional key, type,
and description). A row whose recommendation id is empty is not found.
Unknown ids never reach `GetRecommendationHistory`.

## Decision: Redaction stays on `WriteJSON`

Lifecycle fields are fixed struct fields (`action`, `reason`,
`custom_reason`, `timestamp`, `expires_at`, `actor`). Free text is preserved.
`RedactJSON` still drops credential-like map keys and Pulumi secret
envelopes. The handler does not add a second redactor. Lookup failures use
the existing fixed error body, so paths and plugin errors never reach the
page.

## Alternatives rejected

- Opening plugins the way the CLI command does. That would be a second set
  of processes beside the dashboard.
- Putting the plugin error string in the warning. Those strings can carry
  backend detail the page does not need.
- Drawing the history table in this change. The issue keeps rendering as a
  UX-only follow-up.
