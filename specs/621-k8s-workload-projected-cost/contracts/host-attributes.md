# Contract: core sends `ResourceDescriptor.attributes`

finfocus-spec v0.7.3 defines the field; this is core's side of it.

| RPC | Attributes |
| --- | --- |
| `GetProjectedCost` | yes |
| `BatchCost` (projected) | yes |
| `BatchCost` (actual) | no |
| `Supports` | yes |
| `GetPricingSpec` | yes |
| `GetActualCost`, `GetRecommendations`, `EstimateCost` | unchanged |

Redaction, size, and cache rules: see `data-model.md`. Tags are unchanged.
