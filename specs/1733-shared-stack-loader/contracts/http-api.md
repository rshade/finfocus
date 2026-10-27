# Contract: web hook (Constitution Principle VII)

**Feature**: `1733-shared-stack-loader` | **Spec**: [../spec.md](../spec.md)

`finfocus --web` shows the same resources as the command line. The handlers in
`internal/webui` stay transport only and add no engine logic. They reach the
shared code through the callbacks in `webui.SessionOptions`, which
`internal/cli/web.go` fills from `webData` (`internal/cli/web_data.go`).

## No new endpoints, no payload changes

The feature adds no route and no JSON key to any `/api/*` payload. The
resource sets behind existing routes change. The web does not show the stack
total, because it has no projected-total view; the Overview summary strip
already totals the whole stack.

## Hook map

| Endpoint | Handler callback | Go method that loads resources | Resources after this feature |
| --- | --- | --- | --- |
| `GET /api/overview/query`, `POST /api/overview/preview`, `/api/overview/*` | `SessionOptions.Preview` and the overview pipeline | `stackload.Load` then `(*stackload.Snapshot).Rows`, through `autoDetectOverviewLoader`, `plainOverviewLoader`, and `tuiOverviewLoader` | Unchanged: state merged with preview. |
| `GET /api/estimate/resources`, `/api/estimate/baseline`, `POST /api/estimate/recalculate` | `SessionOptions.EstimateResources` | `(*webData).estimateResources`, which calls `(*stackload.Snapshot).Descriptors` | Live inventory. The Estimate view already overlays state on plan; it now does so through the shared step. |
| `POST /api/recommendations/query`, `GET /api/recommendations/item` | `SessionOptions.Recommendations` | `(*webData).recommendations`, which calls `(*stackload.Snapshot).Descriptors` | Live inventory. Gains resources that are pending creation. |
| `POST /api/cost/actual/query`, `GET /api/cost/actual/resource` | `SessionOptions.ActualCosts` | `(*webData).actualCosts`, which calls `(*stackload.Snapshot).ActualDescriptors` and `enrichWithHistoricalResources` | Unchanged: deployed resources plus billing-period history; a saved preview with no state keeps its plan inventory. |

`webData.captureState`, `capturePlan`, and `captureTerraform` collapse into one
store of the snapshot. `loadResources` and `estimateResources` stop merging
state and plan themselves.

## Payload rules

- Payloads keep passing `RedactJSON` and the data-keyed map rules in
  `docs/src/content/docs/architecture/web-ui.md`.
- A state resource's inputs can hold Pulumi secret envelopes. They are dropped
  by the same rule as any other descriptor (`history.IsPulumiSecret`); the new
  path adds no second rule.
- The Overview `run_status` event keeps `stateOnly`, `previewLoaded`, and
  `detectFailed`. The snapshot's state-unavailable category does not reach the
  browser; as with `detectFailed`, the detail stays in the terminal log.

## Render check (required before the design is final)

With `finfocus --web --no-browser` run from `initial-30` `dev` (empty
passphrase, Pulumi Cloud, per `CLAUDE.md`), render the Overview, Estimate,
Recommendations, and Cost views with Playwright, before and after the change,
and record the stack and date in the spec's Web UI Fit section. Compare the
Estimate picker and Recommendations list with `cost estimate` and
`cost recommendations` for the same stack. The check belongs to the tasks, not
to this plan.

## UX-only follow-up

On completion, file one issue labeled `web-ui` for layout, copy, and
accessibility findings only. Engine, API, redaction, or data-shape work found
in the render check is part of this feature.
