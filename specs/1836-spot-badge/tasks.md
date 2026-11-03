# Tasks: Spot Badge

**Input**: `spec.md`, `plan.md`, `research.md`, `contracts/fields.md`
**Tests first**: each phase writes its tests before the code.

## Phase 1: Data

- [x] T001 Test `SpotDisplay` and the `PricingCategory` JSON shape
  (`internal/engine/spot_test.go`)
- [x] T002 Test mapping for DYNAMIC, STANDARD, UNSPECIFIED, and risk 0.0 on
  the adapter, the projected path, and the estimate path
- [x] T003 Add `CostResult` fields, `internal/engine/spot.go`, adapter fields
  and `PricingCategoryLabel`
- [x] T004 Remove the `Pricing:` and `Spot risk:` notes in `estimate.go`
- [x] T005 Test cache round-trip and the old-entry miss; add the
  `/pricing-v1` key suffix; update `TestProjectedCacheMissesPreGrowthEntries`

## Phase 2: Projected and estimate output

- [x] T006 Test the `cost projected` table Notes column (`TestRenderCostTable
  ShowsSpotTag`); prefix the label in `formatResourceNotes`
- [x] T007 Test the `cost estimate` table and JSON
  (`internal/cli/cost_estimate_spot_test.go`); add the tag to the table

## Phase 3: Overview

- [x] T008 Test `ProjectedCostData` copy, `OverviewRowResult.SpotDisplay`,
  the overview table cell, and the JSON fields
- [x] T009 Add the fields, `enrichProjectedCost` copy, `SpotDisplay`, and the
  table cell

## Phase 4: TUI

- [x] T010 Goldens for the spot list (`overview_spot_list`) and details
  (`detail_spot_resource`, `detail_spot_risk_unknown`); read each golden
- [x] T011 `ColorSpot`, `SpotStyle`, Projected cell tag, conditional column
  width, detail `Pricing` line
- [x] T012 `cost` TUI list cost cell and detail `Pricing` line; estimate TUI
  comparison (`RenderCostComparisonWithSpot`)

## Phase 5: Web parity (Principle VII)

- [x] T013 Node tests for the Overview cell, detail panel, and Estimate
  comparison
- [x] T014 `viewmodel.EstimateDisplay` spot pair with a table test
- [x] T015 `overview.js`, `estimate.js`, `styles.css` (light and dark)
- [x] T016 Render in a browser from a real run and record the evidence in
  `spec.md`

## Phase 6: Polish

- [x] T017 Verify with the real binary: `cost projected` (table, JSON,
  NDJSON, `NO_COLOR`, `--plain`), cache hit, `cost estimate`, `overview`,
  and `--web`, using a throwaway plugin that answers DYNAMIC
- [x] T018 `make lint`, `make test`, `go vet ./...`, markdownlint, and
  `go test -race -shuffle=on -count=3` on the changed packages

## Dependencies

Phase 1 blocks every later phase. Phases 3 to 5 can run in parallel once
Phase 1 is done. Issue #1837 reads the fields in `contracts/fields.md`.
