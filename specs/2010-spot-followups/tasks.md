# Tasks: Spot Follow-ups

**Tests first**: each task writes its test before the code.

- [x] T001 Analyzer: `TestCostToDiagnostic_SpotLabel` in
  `internal/analyzer/diagnostics_spot_test.go`, then the append in
  `formatCostMessage`
- [x] T002 TUI: large-cost tests and golden `cost_table_large_spot` in
  `internal/tui/spot_large_cost_test.go`, then `costWidth` in
  `newResultTable`
- [x] T003 Web: extract `static/spot.js`, link it in the node harness
  (`frontendtest/view_linker.mjs`), keep the existing badge tests passing
- [x] T004 Real-binary analyzer check with a throwaway DYNAMIC plugin kept
  outside the repo, and a browser check of both badges
