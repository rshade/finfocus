# Implementation Plan: Spot Exposure Totals

**Branch**: `1837-spot-exposure-totals` | **Date**: 2026-11-02 | **Spec**: `spec.md`
**Stacked on**: `1836-spot-badge`

## Summary

Add `engine.ComputeSpotExposure` and call it from `AggregateResults`
(`CostSummary.Spot`), the diff renderers (`SpotExposureChange`), and
`summarizeOverviewRows` (`OverviewTotals.Spot`). Renderers print the
`Display` string the function builds. See `research.md` and
`contracts/exposure.md`.

## Technical Context

- **Language**: Go 1.27.3; no new dependency, no spec bump.
- **Testing**: Testify, table-driven, `t.Parallel()`; a node test for the
  totals bar.

## Constitution Check

- Plugin domain knowledge stays in the plugin: core reads the category
  DYNAMIC only. PASS.
- Principle VII: see `Web UI Fit` in `spec.md`. PASS.
- No TODOs or stubs; tests first. PASS.

## Source Code

```text
internal/engine/spot_exposure.go        aggregation, display text
internal/engine/types.go                CostSummary.Spot
internal/engine/engine.go               AggregateResults
internal/engine/project.go              COST SUMMARY row
internal/engine/diff_render.go          diff table rows, JSON spotChange
internal/engine/overview_result.go      OverviewTotals.Spot
internal/engine/overview_render.go      footer row, OverviewSummary.Spot
internal/webui/session.go               mixed-currency totals drop spot
internal/webui/static/views/overview.js totals bar item
docs/src/content/docs/guides/user-guide.md
docs/src/content/docs/commands/overview.md
```

## Complexity Tracking

None.

## Verification

2026-11-02/03, `make build` binary, isolated `FINFOCUS_HOME`, a throwaway
plugin (not committed) answering DYNAMIC with risk 0.2 for one resource, DYNAMIC
with risk 0.0 for another, and STANDARD for the rest, against
`examples/plans/aws-simple-plan.json`:

- `cost projected` table: `Spot exposure  200.00 USD (80.0% of 250.00 USD),
  weighted risk 0.20 (1 of 2 spot resources have no risk score and are
  excluded)` and `Spot change  +200.00 USD (before 0.00, after 200.00)`.
- `cost projected --output json`: `summary.spot` and `spotChange` with the same
  figures; `weightedRisk` is absent when both spot rows have unknown risk, and
  the table then reads `weighted risk unknown (no spot resource has a risk
  score)`. With no spot row, the JSON holds no `spot` key.
- `overview --plain` prints the `Spot exposure:` footer row; `overview
  --output json` carries `summary.spot`.
- `--web` (Playwright): the totals bar read `Spot exposure: 200.00 USD (80.0%
  of 250.00 USD), weighted risk 0.20 (1 of 2 spot resources have no risk score
  and are excluded)` after Potential savings.

Not verified: a real cloud stack in `finfocus-demo` (no spot-priced plugin
there); a replacement diff against the real binary (covered by unit tests).
