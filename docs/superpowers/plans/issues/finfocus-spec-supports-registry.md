# fix(pluginsdk): Supports always fails with the default registry, so plugin Supports is never consulted

## Summary

`Server.Supports` validates the request's provider/region against `ServeConfig.Registry` before
it consults the plugin. The default, `DefaultRegistryLookup`, always returns `""`, so every
`Supports` call returns `InvalidArgument("no plugin registered for provider … and region …")`.
No plugin in the ecosystem sets `ServeConfig.Registry` (checked: finfocus-plugin-aws-public,
finfocus's `plugins/recorder`). The result is that **no plugin's `SupportsProvider`
implementation is ever reached by a host.**

## Where

- `sdk/go/pluginsdk/sdk.go:245-253` — `DefaultRegistryLookup.FindPlugin` returns `""`; its doc
  comment says "Use a real RegistryLookup implementation in production", but nothing does.
- `sdk/go/pluginsdk/sdk.go:576-595` — `Server.Supports` step 1 returns `InvalidArgument` when
  `FindPlugin(provider, region) == ""`, before step 2 (`SupportsProvider`).
- `NewServerWithOptions` installs `&DefaultRegistryLookup{}` when `registry == nil`.

## Impact

- finfocus-plugin-aws-public implements `Supports` (`internal/plugin/supports.go:13`) to decline
  resources outside its compiled region; hosts never see that answer.
- finfocus core calls `Supports` before routing (`internal/engine/engine.go:194-232`,
  `checkPluginSupports`) with only `resource_type` set, and **fails open** on any error, caching
  "supported". So every plugin is treated as supporting every resource type, and a plugin that
  cannot price a resource is still called. Each such call records a per-resource plugin error in
  `cost projected`, and with routing `fallback: false` it can end the fallback chain early.
- Usage-only plugins (#505) and allocator plugins (#506) cannot opt out of pricing through
  `Supports` without shipping their own `RegistryLookup` stub — a workaround every author would
  have to rediscover.

## Proposed fix

1. In `Server.Supports`, treat the default registry as "no registry configured": when
   `s.registry` is the default lookup (or nil), skip the provider/region check and go straight to
   the plugin's `SupportsProvider` (or the existing not-implemented default response). A
   configured `RegistryLookup` keeps today's validation.
2. Keep returning `Supported: false` with `DefaultSupportsNotImplementedReason` when the plugin
   does not implement `SupportsProvider` — unchanged.
3. Update the `DefaultRegistryLookup` doc comment and the README / developer guide section on
   `Supports` to describe the new default.

Alternative considered: make `DefaultRegistryLookup.FindPlugin` return the plugin's own name.
Rejected — it hides the "no registry" state instead of modelling it, and `FindPlugin("", "")`
succeeding would be surprising to any caller that uses the lookup directly.

## Companion change (finfocus core, separate issue)

`checkPluginSupports` sends a `ResourceDescriptor` with only `resource_type`. After this fix, it
should also send `provider` and `region` so plugins such as aws-public can answer region
questions. Tracked in rshade/finfocus once this lands.

## Acceptance criteria

- [ ] A plugin served with no `ServeConfig.Registry` that implements `SupportsProvider` has its
  `Supports` called, over gRPC and Connect (tests for both)
- [ ] A plugin with no `ServeConfig.Registry` and no `SupportsProvider` returns
  `Supported: false` with `DefaultSupportsNotImplementedReason` and no error
- [ ] A plugin with a configured `RegistryLookup` keeps today's provider/region validation
  (existing tests unchanged)
- [ ] `Supports` with an empty provider and region (what hosts send today) no longer errors under
  the default registry
- [ ] Capabilities auto-population in the `Supports` response is unchanged
- [ ] README / developer guide updated; `DefaultRegistryLookup` doc comment corrected
- [ ] Conventional commit `fix(pluginsdk): …` so release-please lists it under Fixed

## Related

- #504 — capability backfill for `PluginInfoProvider` (same area, different bug)
- #505 / #506 — usage-source and allocator plugins, which rely on declining `Supports`
