# pluginsdk API (finfocus-spec v0.7.2)

Import paths:

```go
import (
    "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
    pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)
```

Signatures below were checked against `sdk/go/pluginsdk/` at v0.7.2. The
version your repo uses is the `finfocus-spec` line in `go.mod`; check the
source in the module cache if it differs.

## Required interface

```go
type Plugin interface {
    Name() string
    GetProjectedCost(ctx, *pbc.GetProjectedCostRequest) (*pbc.GetProjectedCostResponse, error)
    GetActualCost(ctx, *pbc.GetActualCostRequest) (*pbc.GetActualCostResponse, error)
    GetPricingSpec(ctx, *pbc.GetPricingSpecRequest) (*pbc.GetPricingSpecResponse, error)
    EstimateCost(ctx, *pbc.EstimateCostRequest) (*pbc.EstimateCostResponse, error)
}
```

`pluginsdk.NewBasePlugin(name)` returns `*BasePlugin`, which implements all of
these. Defaults: `GetProjectedCost` returns `NotSupportedError`,
`GetActualCost` returns `NoDataError`, `GetPricingSpec` and `EstimateCost`
return "not implemented" errors. Override only what you implement.
`BasePlugin` also gives `Matcher()` (`*ResourceMatcher`: `AddProvider`,
`AddResourceType`, `Supports(resource)`) and `Calculator()` (`*CostCalculator`:
`HourlyToMonthly`, `MonthlyToHourly`, `CreateProjectedCostResponse`,
`CreateActualCostResponse`). `pluginsdk.HoursPerMonth` is 730.

## Optional interfaces

The server detects these by type assertion on your plugin and infers
capabilities from them.

| Interface | Method |
| --- | --- |
| `SupportsProvider` | `Supports(ctx, *pbc.SupportsRequest) (*pbc.SupportsResponse, error)` |
| `RecommendationsProvider` | `GetRecommendations(ctx, *pbc.GetRecommendationsRequest)` |
| `BudgetsProvider` | `GetBudgets(ctx, *pbc.GetBudgetsRequest)` |
| `DismissProvider` | `DismissRecommendation(ctx, *pbc.DismissRecommendationRequest)` |
| `PluginInfoProvider` | `GetPluginInfo(ctx, *pbc.GetPluginInfoRequest)` |
| `ResolveResourceTypesProvider` | `ResolveResourceTypes(ctx, *pbc.ResolveResourceTypesRequest)` |
| `BatchCostHandler` | `BatchCost(ctx, *pbc.BatchCostRequest)` |
| `DryRunHandler` | `HandleDryRun(ctx, *pbc.DryRunRequest) (*pbc.DryRunResponse, error)` |

Without `BatchCostHandler` the SDK serves `BatchCost` by fanning out to
`GetProjectedCost`. Without `SupportsProvider` the server answers
`Supported: false` with `pluginsdk.DefaultSupportsNotImplementedReason`.
Specialist services (`UsageSourceProvider`, `AllocatorProvider`,
`RecommendationScorerProvider`, `ContractCommitmentProvider`,
`InvoiceDatasetProvider`) exist for non-pricing plugins; they are out of
scope for a cost plugin.

If you set `PluginInfo.Capabilities` (or return `Capabilities` from
`GetPluginInfo`) the list replaces the inferred set entirely. Leave it empty
to inherit.

## Serving

```go
cfg := pluginsdk.ServeConfig{Plugin: plugin, Port: port}
err := pluginsdk.Serve(ctx, cfg)
```

The scaffold calls `flag.Parse()`, `pluginsdk.ParsePortFlag()` (the `--port`
flag), falls back to `pluginsdk.GetPort()` (`FINFOCUS_PLUGIN_PORT`), and calls
`Serve`. Port 0 picks a free port. `Serve` announces `PORT=<n>` on stdout.
`pluginsdk.Run(config) int` is the newer entry point
(`os.Exit(pluginsdk.Run(cfg))`); it also adds `--help`, `--version`, and a
`dry-run` subcommand. `ServeConfig` also has `PluginInfo`, `Registry`,
`Logger`, `UnaryInterceptors`, `Web`, `Timeouts`, `MaxBatchSize`,
`BatchWorkers`, and `TypeRegistry`.

`pluginsdk.NewPluginInfo(name, version, opts...)` with `WithSpecVersion`,
`WithProviders`, `WithMetadata`, `WithCapabilities` builds the `PluginInfo`.
`pluginsdk.SpecVersion` is `"v0.7.2"` and `ValidateSpecVersion(v)` requires
the `v` prefix.

## Environment variables

Read through helpers in `env.go`; legacy `PULUMICOST_*` names are still
accepted as fallbacks.

| Variable | Helper | Meaning |
| --- | --- | --- |
| `FINFOCUS_PLUGIN_PORT` | `GetPort()` | gRPC port; `--port` wins |
| `FINFOCUS_LOG_LEVEL` | `GetLogLevel()` | trace, debug, info, warn, error |
| `FINFOCUS_LOG_FORMAT` | `GetLogFormat()` | json or console |
| `FINFOCUS_LOG_FILE` | `GetLogFile()` | log file path |
| `FINFOCUS_TRACE_ID` | `GetTraceID()` | externally injected trace id |
| `FINFOCUS_TEST_MODE` | `GetTestMode()` / `IsTestMode()` | SDK test-mode flag |

`pluginsdk.NewPluginLogger(name, version, level, writer)` and
`pluginsdk.NewLogWriter()` build a stderr/file logger. gRPC metadata key
`pluginsdk.TraceIDMetadataKey` (`x-finfocus-trace-id`) carries the trace id;
`pluginsdk.TraceIDFromContext(ctx)` reads it and
`pluginsdk.WithTrace(ctx, logger)` stamps logs with it. The field name is
`trace_id` (`pluginsdk.FieldTraceID`); do not rename it.

## Validation helpers

| Function | Use |
| --- | --- |
| `ValidateProjectedCostRequest(req)` | strict: needs provider, type, SKU, region |
| `ValidateProjectedCostRequestLenient(req)` | provider and type only; for sparse or `ref.*` requests |
| `ValidateActualCostRequest(req)` | actual-cost request check |
| `ValidateGetProjectedCostResponse(resp)` | check your own response in tests |
| `ValidateSupportsResponse(resp)` | check a `Supports` answer |
| `ValidateRecommendation(rec)` | check a recommendation |

After lenient validation, check what you extract; report missing inputs in
`BillingDetail` instead of returning a misleading cost.

## Response builders and errors

- `NewGetProjectedCostResponse(opts...)` with `WithProjectedCostDetails(unit,
  currency, monthly, detail)`, `WithProjectedCostExpiresAt(t)`,
  `WithProjectedCostBreakdown(map[string]float64)`,
  `WithProjectedCostPricingCategory`, `WithProjectedCostSpotRisk`,
  `WithPredictionInterval(lower, upper, confidence)`,
  `WithProjectedCostPriceOptions`, `WithProjectedCostRegionPrices`.
- `NewActualCostResponse(opts...)` with `WithResults`, `WithFallbackHint`,
  `WithNextPageToken`, `WithTotalCount`; `ValidateActualCostResponse`.
- `NewEstimateCostResponse(opts...)` with `WithEstimateCost(currency,
  monthly)` and related options.
- `NotSupportedError(resource)`, `NoDataError(resourceID)`.
- `NewResourceDescriptor(provider, type, opts...)` with `WithSKU`,
  `WithRegion`, `WithTags`, `WithID`, `WithARN` for building test inputs.
- Expiry helpers in `expires_at.go`: `ProjectedCostExpiresAt`,
  `IsProjectedCostExpired`, `WithActualCostResultExpiresAt`,
  `WithEstimateCostExpiresAt`.

## Mapping helpers (`pluginsdk/mapping`)

`ExtractAWSSKU(props)`, `ExtractAWSRegion(props)`,
`ExtractAWSRegionFromAZ(az)`, and the Azure and GCP equivalents
(`ExtractAzureSKU`, `ExtractAzureRegion`, `ExtractGCPSKU`,
`ExtractGCPRegion`, `ExtractGCPRegionFromZone`). They return `""` when the
property is absent, so always check the result.

## Dry run

`HandleDryRun` returns which FOCUS fields the plugin fills for a resource
type, without external calls (target under 100 ms). Build the response with
`pluginsdk.NewDryRunResponse(opts...)`, `WithResourceTypeSupported`,
`WithFieldMappings`, `NewFieldMapping(field, status, opts...)`,
`AllFieldsWithStatus(status)`, `SetFieldStatus`. Its signature takes a
`context.Context` first since v0.5.7; if the build fails on it, follow the
`finfocus-plugin-upgrade` skill.
