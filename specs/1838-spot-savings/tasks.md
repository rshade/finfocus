# Tasks: Spot Savings vs On-Demand

**Tests first**: each phase writes its tests before the code.

- [x] T001 Tests for `proto.OnDemandEquivalent` (present, zero, absent, NaN,
  infinity, negative, non-DYNAMIC) and the projected mapping in
  `internal/proto/spotrisk_test.go`
- [x] T002 Table-driven tests for `engine.SpotSavingsPercent` and
  `SpotDisplay` with savings (cheaper, equal, dearer, zero on-demand, unset)
  in `internal/engine/spot_savings_test.go`
- [x] T003 Implement `internal/engine/spot_savings.go`, the `CostResult` and
  `ProjectedCostData` fields, and the mapping in `engine_batch.go` and
  `estimate.go`
- [x] T004 Tests for the overview row (label, JSON, table), the enrichment
  copy, and the diff note
- [x] T005 Cache round-trip test (`TestProjectedCacheKeepsOnDemandEquivalent`)
- [x] T006 TUI: carry the fields in the estimate pricing snapshot; regenerate
  and read the detail goldens
- [x] T007 Viewmodel and node tests render the longer label verbatim
- [x] T008 Docs: user guide Spot pricing, overview page, CLAUDE.md note
- [x] T009 Real-binary run with a throwaway plugin answering DYNAMIC with and
  without the field: `cost projected` table and JSON, `cost estimate`,
  overview, web
