# Upgrade to finfocus-spec v0.7.10: historical carbon inventory

v0.7.10 removed and renamed nothing in the protocol. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for the plugin binary. The
release adds an optional RPC; a plugin that does not report carbon inventory
needs no change.

## What changed

- `CostSourceService.GetCarbonEmissions` returns provider-reported monthly
  emissions inventory. It is independent of the projected `CARBON` impact
  metrics and of resource cost rows.
- `PLUGIN_CAPABILITY_HISTORICAL_CARBON` is the new capability, with the legacy
  name `supports_historical_carbon`.
- `pluginsdk.HistoricalCarbonProvider` is the interface a provider implements.
  Without one, the RPC returns `Unimplemented`.
- `ValidateGetCarbonEmissionsRequest`, `ApplyGetCarbonEmissionsDefaults`,
  `ValidateGetCarbonEmissionsResponse`, and `ValidateCarbonEmissionRecord`
  check the exchange.

## Plugins that report carbon inventory

The SDK never infers the capability. Existing plugins that embed the generated
unimplemented server also satisfy the method interface, so inference would
advertise a capability they do not have. Implement the provider interface and
list `PLUGIN_CAPABILITY_HISTORICAL_CARBON` in the `GetPluginInfo` capabilities
(for example through `pluginsdk.WithCapabilities`) yourself.

Requests use an inclusive start and an exclusive end, both at midnight UTC on
the first of a month. Account, region, and service filters use OR within a
dimension and AND across dimensions. The SDK resolves an unspecified method to
market-based and an unspecified scope to total. Page size defaults to 50 and is
bounded at 1000.

Each `CarbonEmissionRecord` carries a stable ID, the original monthly period,
the resolved method and scope, the model version, and a finite, nonnegative
`gCO2e` value. The `value` field must be present, including an explicit zero:
use `proto.Float64(value)`. Never invent a zero for missing data. The stable ID
excludes the model version, so a revised estimate replaces the earlier row
instead of adding to it.

## Plugins that do not report carbon inventory

Leave `GetCarbonEmissions` unimplemented and do not list the capability. Core
checks the capability before it calls the RPC.
