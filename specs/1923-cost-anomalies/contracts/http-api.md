# HTTP Contract: Anomalies hook

Principle VII hook. Transport only.

- **Endpoint**: `GET /api/overview/anomalies`, handler
  `(*Server).handleAnomalies` in `internal/webui/handlers_anomalies.go`.
- **Go methods called**: `(*webui.Session).Rows`, `engine.SelectAnomalies`, and
  `viewmodel.BuildAnomalyDisplay`. No other engine call, no new overview
  pipeline, no import of `internal/cli`.
- **Response 200**: `webui.AnomalyPage` as JSON: `considered`, `summary`, and
  `anomalies`, each an `engine.Anomaly` (same fields as the CLI) plus a
  `display` object with `baseline`, `observed`, `deviation`, `drift`, and
  `period` text. Written with `(*Server).WriteJSON`, which applies `RedactJSON` like the other
  overview payloads.
- **Before the session is ready**: rows are installed before enrichment sets
  their drift, so an empty list would be wrong. The handler answers
  `503 not_ready` (`overview data is loading`) until `(*Session).IsReady` is
  true, as `/api/cost/actual/query` and `/api/recommendations/query` do. A
  ready session with no drifting rows returns `200` with an empty list.
- **Auth**: same session cookie or bearer rules as `/api/overview/resource`.

## Page contract

The Anomalies view (`static/views/anomalies.js`) renders only what this
endpoint returns.

- Route `#/anomalies`, header link `Anomalies`.
- Fetch on mount, and again on the overview stream events `ready` and
  `snapshot`. A response from a superseded request is dropped.
- Slots: heading, status line (`role="status"`), alert (`role="alert"`) for a
  failed request, captioned table in a scrollable wrapper (`tabindex="0"`).
- Columns: Resource, Type, Period, Baseline, Observed, Deviation, Drift %,
  Currency. Numeric columns are right-aligned. A missing period renders `—`.
- States: loading before the first response, empty (`No anomalies found.`
  with the considered count), error.
- The view does no arithmetic and no formatting; every cell is a string from
  the payload.
