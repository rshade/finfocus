# Contract: Web Hook for "Declared at"

No new endpoint.

- `GET /api/overview/resource?urn=...` (`Server.handleOverviewResource`)
  already returns `row` as `engine.OverviewRowResult`, built by
  `engine.ComputeOverviewResult` from `Session.Rows()`, and `display` from
  `viewmodel.BuildOverviewDetailDisplay`. `OverviewRowResult` gains `Location`
  (`json:"location,omitempty"`), copied from `OverviewRow.Location`, the same
  value the CLI's JSON overview prints. The handler itself does not change.
- `viewmodel.OverviewDetailDisplay` gains `declaredAt`: `file:line` from that
  location, empty when there is none. The server formats it, as it formats
  every other detail field.
- `internal/webui/static/views/overview.js` renders a "Declared at" row from
  `display.declaredAt` when it is non-empty, and no row otherwise. The browser
  formats nothing.
- `RedactJSON`: `location` is a plain struct of a project-relative path and
  integers, and `declaredAt` is a string built from it; neither carries
  property data, so no new rule is needed.
- UX follow-up issue (Principle VII): copy and placement of the row only.
