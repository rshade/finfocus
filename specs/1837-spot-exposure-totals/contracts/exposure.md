# Contract: Spot exposure

## `engine.SpotExposure` (JSON)

| Key | Type | Present | Meaning |
| --- | --- | --- | --- |
| `spotCost` | number | always | Sum of the cost of DYNAMIC rows |
| `totalCost` | number | always | Sum of the cost of every row counted |
| `spotShare` | number | when `totalCost` > 0 | `spotCost / totalCost` (0 to 1) |
| `weightedRisk` | number | when a known-risk spot row carries cost | `sum(cost*risk) / sum(cost)` over known-risk spot rows. Absent never means 0 |
| `spotResources` | integer | always | DYNAMIC rows |
| `riskUnknownResources` | integer | always | DYNAMIC rows excluded from `weightedRisk` |
| `currency` | string | always | The common currency |
| `display` | string | always | Text shown after the `Spot exposure` label |

`weightedRisk` is also absent when every known-risk spot row costs $0.

The whole block is absent when there is no DYNAMIC row or the currencies
differ.

## Where it appears

| Surface | Location |
| --- | --- |
| `cost projected` table | `Spot exposure` row (no colon) in the COST DIFF block, which is what the command prints. `RenderCostTable` also adds a `Spot exposure:` COST SUMMARY row for API callers |
| `cost projected` JSON | `finfocus.summary.spot` |
| `cost projected` diff | table `Spot change` row; JSON `finfocus.spotChange` |
| Overview table | `Spot exposure:` row under the SUMMARY line |
| Overview JSON | `summary.spot` |
| Web overview | `totals.spot` (same object), shown in the totals bar |

## `engine.SpotExposureChange` (JSON `spotChange`)

`beforeSpotCost`, `afterSpotCost`, `deltaSpotCost`, `currency`, `display`
(`+300.00 USD (before 0.00, after 300.00)`). Absent when neither side has a
spot row or the currencies differ.

## Display examples

- `400.00 USD (40.0% of 1000.00 USD), weighted risk 0.18`
- `1000.00 USD (50.0% of 2000.00 USD), weighted risk 0.40 (1 of 2 spot
  resources have no risk score and are excluded)`
- `150.00 USD (75.0% of 200.00 USD), weighted risk unknown (no spot resource
  has a risk score)`

## Go API (package `engine`)

```go
func ComputeSpotExposure(items []SpotExposureItem) *SpotExposure
func SpotExposureFromCosts(results []CostResult) *SpotExposure
func ComputeSpotExposureChange(diff *DiffResult) *SpotExposureChange
```
