# Upgrade to finfocus-spec v0.7.9: capability-aware conformance

v0.7.9 removed and renamed nothing in the protocol. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for the plugin binary. The
conformance suite behaves differently, so run it again after the bump.

## What changed

The built-in conformance suite reads `GetPluginInfo.capabilities` once per run.
A nonempty list is authoritative for actual costs, projected costs, pricing
specs, and budgets:

- For each advertised capability, the suite runs the existing response
  validation checks.
- For each unadvertised capability, the suite sends a valid request and requires
  `Unimplemented`. These are executed checks, not skipped tests.
- Pricing spec support is independent of projected-cost support.

An empty list, or an `Unimplemented` `GetPluginInfo`, keeps the legacy contract:
pricing and actual-cost checks stay mandatory at their existing levels, and
budgets stay optional. Other discovery errors fail the capability-dependent
checks. Unknown capabilities do not disable checks for the known ones.

## Plugins that list capabilities: list every implemented service

If `GetPluginInfo` sets `Capabilities` by hand, make sure it lists every service
the plugin implements. A plugin that implements `GetBudgets` but does not list
`PLUGIN_CAPABILITY_BUDGETS` now fails conformance, because the suite expects
`Unimplemented` from it. The reverse also fails: an advertised budgets
capability must succeed and cannot use the legacy optional-RPC exception.

Plugins that leave `Capabilities` empty need no change: the SDK fills it with
the set it infers from the implemented interfaces, which already matches what
the plugin answers. Only a hand-set list, for example from
`pluginsdk.WithCapabilities`, can disagree with the implementation.

## Running the suite

Use `RunConformance` with `ConformanceLevelStandard` to include the positive
actual-cost checks. Supply `WithSampleResource` when the default descriptor does
not suit the plugin. Check both the returned error and `result.Passed()`:
validation failures are recorded in the result.
