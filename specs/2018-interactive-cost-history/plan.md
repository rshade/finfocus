# Implementation plan

## Summary

Keep the existing history reads and date/currency selection. Build a Bubble
Tea v2 model with ntcharts v2. The approved v2 dependency matches the current
FinFocus TUI.

## Design

- `internal/history` provides provider normalization, chart values and
  applicable budget thresholds. Reuse `history.DiffSnapshots` for details.
- `internal/tui/history_model.go` owns selection, viewport and toggles.
  `internal/tui/history_view.go` renders the chart, markers, detail and help.
- `internal/cli/cost_history_view.go` runs the model when the existing
  accessibility resolver and input checks allow it.
- `internal/cli/web_history.go` and `internal/webui/handlers_history.go`
  expose matching budget thresholds through the existing History route.

## Constitution check

The view adds no storage, protocol or provider pricing changes. Write tests
before production code and inspect golden renders. Verify web parity via the
existing History routes, parity tests and a populated dashboard render.
