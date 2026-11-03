# Tasks: Spot Exposure Totals

**Tests first**: each phase writes its tests before the code.

- [x] T001 Table-driven tests for `ComputeSpotExposure` (mixed, all-spot,
  no-spot, all-unknown, some-unknown, zero total, invalid risk, mixed
  currency) in `internal/engine/spot_exposure_test.go`
- [x] T002 Tests for `SpotExposureFromCosts` (error rows skipped) and
  `ComputeSpotExposureChange` (create, replacement, delete, unchanged)
- [x] T003 Implement `internal/engine/spot_exposure.go`
- [x] T004 Test and wire `CostSummary.Spot` and the `cost projected` summary
  row; JSON `summary.spot`
- [x] T005 Test and wire the diff table rows and JSON `spotChange`
- [x] T006 Test and wire `OverviewTotals.Spot`, the overview footer row, and
  `OverviewSummary.Spot`
- [x] T007 Wire the web totals bar (`overview.js`), with a Go test of the
  payload and a node test
- [x] T008 Docs: user guide Spot pricing, overview page
- [x] T009 Real-binary run with a spot-answering plugin: table, JSON,
  overview plain and JSON, web
