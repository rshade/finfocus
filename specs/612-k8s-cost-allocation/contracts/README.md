# Contracts: Kubernetes In-Cluster Cost Allocation

The wire contracts for this feature are owned by **finfocus-spec** and released
in **v0.6.2** (2026-09-28). They are referenced here rather than copied, so the
proto definitions remain the single source of truth.

## Protos (finfocus-spec v0.6.2)

- `proto/finfocus/v1/usage.proto` — `UsageSourceService.GetStats`,
  `GetStatsRequest`/`GetStatsResponse`, `UsageRow`, `StatsMode`.
  Added by rshade/finfocus-spec#505 (speckit folder
  `specs/051-usage-source-getstats/` in finfocus-spec).
- `proto/finfocus/v1/allocation.proto` — `AllocatorService.Allocate`,
  `AllocateRequest`/`AllocateResponse`, `PricedResource`, `AllocationRow`.
  Added by rshade/finfocus-spec#506 (speckit folder
  `specs/052-allocator-allocate/` in finfocus-spec).
- `proto/finfocus/v1/enums.proto` — `PLUGIN_CAPABILITY_USAGE_STATS = 14`,
  `PLUGIN_CAPABILITY_ALLOCATION = 15`.
- `Supports` default-registry fix: rshade/finfocus-spec#507, shipped via #510.

Entity semantics (subject keys, kinds, metrics, conservation invariant, policy
rules) are documented in [../data-model.md](../data-model.md).

## Generated Go API (consumed by core and plugins)

- `pbc.GetStatsRequest`/`GetStatsResponse`, `pbc.UsageRow`,
  `pbc.StatsMode_STATS_MODE_RUN_RATE`, `pbc.StatsMode_STATS_MODE_HISTORICAL`,
  `pbc.NewUsageSourceServiceClient`, `pbc.UsageSourceServiceServer`
- `pbc.AllocateRequest`/`AllocateResponse`, `pbc.PricedResource`,
  `pbc.AllocationRow`, `pbc.NewAllocatorServiceClient`,
  `pbc.AllocatorServiceServer`
- `pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS`,
  `pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION`

## pluginsdk API (finfocus-spec v0.6.2)

- Provider interfaces: `pluginsdk.UsageSourceProvider`,
  `pluginsdk.AllocatorProvider` — when implemented, `pluginsdk.Serve`
  registers the service in gRPC mode, Connect mode, and the grpchealth static
  checker.
- Shared rules: `pluginsdk.DecodePolicy`, `pluginsdk.ValidateAllocateRequest`,
  `pluginsdk.ValidateAllocateResponse`, `pluginsdk.ResolveCurrency`,
  `pluginsdk.CheckConservation`, `pluginsdk.DefaultConservationEpsilon`.
- Constants: `Subject*`/`Kind*`/`Metric*`/`Unit*` (`pluginsdk/subjects.go`);
  `pluginsdk.DefaultSupportsNotImplementedReason`.
- Testing: `plugintesting.ValidateStatsResponse`,
  `plugintesting.RunAllocatorConformance`.

## Capability names in core

`internal/pluginhost` maps the enum values to the capability strings
`"usage_stats"` and `"allocation"` (used by registry entries and plugin
selection).
