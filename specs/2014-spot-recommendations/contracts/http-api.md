# HTTP API Contract: Spot Recommendations

No new endpoint. Four existing responses gain display strings. Each string is
built by the Go method the CLI and TUI call; `internal/webui` adds no logic.

| Endpoint | New field | Go method |
| --- | --- | --- |
| `POST /api/recommendations/query` | `itemSpot[]` (one per `items[]` entry, `""` when not spot) | `viewmodel.BuildRecommendationPage` → `engine.Recommendation.SpotDisplay` |
| `GET /api/recommendations/item` | `spotDisplay` (omitted when not spot) | `engine.Recommendation.SpotDisplay` |
| `GET /api/overview/resource` | `activeRecommendations[].spotDisplay` (omitted when not spot) | `viewmodel.OverviewDetailRecommendations` → `engine.Recommendation.SpotDisplay` |
| Cost detail (`viewmodel.ActualCostDetail`) | `recommendations[].spotDisplay` (omitted when not spot) | `viewmodel.LinkedCostRecommendations` → `engine.Recommendation.SpotDisplay` |

The raw recommendation in `items[]`, `item`, and
`activeRecommendations[].recommendation` also carries `pricingCategory` and
`spotInterruptionRisk` (`engine.Recommendation`, omitted when empty or zero).
Both are a plugin enum label and a number, so the redaction walk passes them
unchanged.

Label values: `SPOT · risk 0.12`, `SPOT · risk <0.01`, `SPOT · risk unknown`.

## Follow-up #2054 (presentation)

Three more display strings, so the browser still formats nothing:

| Endpoint | New field | Go method |
| --- | --- | --- |
| `POST /api/recommendations/query` | `itemNames[]` (one per `items[]` entry) | `viewmodel.BuildRecommendationPage` → `engine.ExtractResourceDisplayName` |
| `GET /api/overview/resource` | `activeRecommendations[].actionDisplay` | `viewmodel.OverviewDetailRecommendations` → `proto.ActionTypeLabelFromString` |
| Cost detail (`viewmodel.ActualCostDetail`) | `recommendations[].actionDisplay` | `viewmodel.LinkedCostRecommendations` → `proto.ActionTypeLabelFromString` |

`actionDisplay` is the label the Recommendations page already shows in
`itemActions[]` (`Modify`, not `RECOMMENDATION_ACTION_TYPE_MODIFY`).
