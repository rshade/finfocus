# Contract: Terraform-compatible web project sources

`finfocus --web` detects a Terraform-compatible project and generates its
plan with OpenTofu. It keeps the existing localhost authentication, request
validation, redaction, session lifecycle, and endpoint shapes. No additional
endpoint is required.

## Shared source and engine methods

`cli.runWebSession` calls `cli.DetectProjectSource` for source selection.
`cli.autoDetectOverviewLoader` routes Terraform projects through
`cli.tuiOverviewLoader`, `cli.plainOverviewLoader`, and
`cli.loadTerraformOverview`. Those call `terraform.RunPlan` and
`ingest.(*TerraformPlanParser).Parse`, then the same engine adapter used by
the CLI. Planning and workspace commands invoke only `tofu`.

`cli.(*webData).captureTerraform` retains the validated current/planned
resource descriptors. It preserves Terraform addresses, operations,
before-values, custom providers, and unknown-property metadata. Existing
plugin type resolution applies to these values. Queries use the captured
snapshot and do not start another plan or fall back to Pulumi discovery.

| Endpoint | Handler and shared Go method |
| --- | --- |
| `GET /api/overview/stream` | Existing `webui.(*Server).handleOverviewStream` publishes pipeline updates from `engine.EnrichOverviewRows` |
| `POST /api/overview/query` | Existing `webui.(*Server).handleOverviewQuery` reads enriched rows from that pipeline |
| `GET /api/overview/resource` | Existing `webui.(*Server).handleOverviewResource` reads the same resource details |
| `POST /api/cost/actual/query` | `cli.(*webData).actualCosts` calls `engine.(*Engine).GetActualCostWithOptionsAndErrors` with before-values |
| `POST /api/recommendations/query` | `cli.(*webData).recommendations` uses `engine.(*Engine).GetRecommendationsForResources` through the existing recommendation workflow |
| `GET /api/estimate/resources` | `cli.(*webData).estimateResources` supplies captured planned descriptors |
| `GET /api/estimate/baseline` | Existing estimate handler calls `engine.(*Engine).EstimateBaseline` |
| `POST /api/estimate/recalculate` | Existing estimate handler calls `engine.(*Engine).EstimateCost` |

`POST /api/overview/preview` retains its existing state-only session gate.
A detected Terraform web launch has already planned and is not a Pulumi
state-only session. Terraform discovery never prompts for a Pulumi stack
passphrase; Pulumi sessions retain their existing decryption/retry behavior.

## Validation

The web integration test starts a session in a nearer Terraform project,
uses a fake `tofu` process and a real gRPC mock pricing plugin, asserts
priced rows, then calls overview, actual, recommendations, and estimate
endpoints. It asserts that neither those requests nor startup invoke Pulumi
or perform a second plan. Live validation generates a genuine OpenTofu
plan and inspects a populated browser dashboard.
