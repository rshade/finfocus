# Interactive browser cost history

**Status**: Implemented

**Issue**: [#2264](https://github.com/rshade/finfocus/issues/2264)

## User scenarios and testing

A user opens the Cost view and explores the loaded checkpoint history.
They zoom in, move earlier or later, and reset to the applied date range.
They show provider costs or budget thresholds, read the legend, and
select a deployment to see its annotation and previous-checkpoint diff.

## Requirements

- **FR-001** Provide keyboard and pointer controls to pan, zoom and reset the
  chart. Keep the applied date and currency controls and captioned tables.
- **FR-002** Show active provider series and applicable budget thresholds.
  Use the budget amounts and display strings supplied by the server.
- **FR-003** Let users select deployment markers and reach the same selection
  using keyboard-accessible controls. Show the annotation and compare with the
  immediately preceding checkpoint in the applied range.
- **FR-004** Keep controls and details usable at mobile widths and in both
  color schemes. Use accessible names, visible focus, and text state.

## Web UI Fit

Extend the existing Cost history chart in slot 7 of the Cost view. Place the
chart controls above the chart and its legend and deployment selection beside
the existing range and diff workflow. The existing history and diff endpoints
provide the data. This issue is the UX follow-up required by #550.

## Success criteria

- Navigation and series changes update the chart without another history read.
- Activation from the chart or deployment list selects the same detail.
- A populated History view passes desktop and mobile browser acceptance.
