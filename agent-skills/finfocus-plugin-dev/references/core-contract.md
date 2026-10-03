# How finfocus core calls a plugin

Derived from core's documented behavior. Use it to predict what your plugin
receives and how core treats what it returns.

## Discovery and launch

- Plugins live at `~/.finfocus/plugins/<name>/<version>/`. Core launches the
  binary named `finfocus-plugin-<name>`.
- If `plugin.metadata.json` in that directory has a `region` key, core looks
  for `finfocus-plugin-<name>-<region>` first, then the standard name.
- Core allocates a free port and starts the plugin with `--port <n>`. It does
  not set the generic `PORT` variable. `FINFOCUS_PLUGIN_PORT` also works.
- The plugin prints `PORT=<n>` on stdout once listening (the SDK does this).
  Keep stdout clean.
- `finfocus plugin list` shows each plugin's name, version, and spec version
  (from `GetPluginInfo`); a bare `0.x.y` spec version shows `N/A`.
  `finfocus plugin validate` starts the plugins and checks they respond.

## Request shape

- `ResourceDescriptor.Provider` is the billing cloud (`aws`, `azure`, `gcp`).
  Pulumi packages such as `aws-native` or `azure-native` are normalized for
  routing, but `ResourceType` keeps the raw package prefix.
- `ResourceType` is a Pulumi type token: `aws:ec2/instance:Instance`. Other
  spellings (`aws:ec2:Instance`, `ec2`) are not guaranteed. Normalize.
- Core fills `Sku` and `Region` from resource inputs
  (`instanceType`, `type`; `availabilityZone`, `region`). If ingestion could
  not find them they are empty.
- `Tags` carries the resource inputs. Nested maps and arrays also appear as
  dotted keys (`sku.capacity`, `rootBlockDevice.0.volumeType`) beside the
  collapsed key. Credential-like segments, `tags`, `labels`, and `__`
  internal keys are dropped. The map is capped at 50 entries.
- When a property points at exactly one other resource in the plan, core adds
  `ref.<property>.urn`, `.type`, `.region`, `.sku` tags. A child with `ref.*`
  tags and an empty SKU is validated with
  `ValidateProjectedCostRequestLenient`, so your plugin sees the request.
  The unknown-value sentinel `04da6b54-80e4-46f7-96ec-b56ff0331ba9` is never
  sent.
- Core validates requests with `pluginsdk` helpers before the call; invalid
  resources are skipped with a `VALIDATION:` note and never reach you.

## Supports

- Core calls `Supports` with provider, type, SKU, and region, and caches the
  answer per client, provider, type, region, SKU, and feature.
- `Supported: false` removes your plugin from that resource's chain.
- An RPC error, or the SDK default reason
  `pluginsdk.DefaultSupportsNotImplementedReason`, is treated as supported
  (fail-open), so a plugin that skips `Supports` is still called.
- Region-bound plugins should return `Supported: false` for other regions.

## Results and fallback

- Routing is configured by the user; with no routing config core queries all
  plugins. Higher routing priority wins.
- A `$0` cost is a valid result and does **not** trigger fallback. Only a nil
  or empty result (an error or "no data") moves on to the next plugin.
- For an unpriced resource, return an error. A `$0` from an unknown instance
  type is read as "free" and also hides the resource from other plugins.
- Core derives monthly totals from `CostPerMonth` and uses 730 hours/month
  when it converts `UnitPrice`.
- If `GetPricingSpec` fallback is enabled by the user, resources no plugin
  priced are priced from your `GetPricingSpec` response (5 s deadline); unknown
  billing modes and errors fall through to local YAML specs.
- Cache: results are cached by resource. Set `ExpiresAt` to control per-entry
  lifetime; a past time skips caching and values over 7 days are capped.
- Errors are shown to the user as `PLUGIN_ERROR`, `TIMEOUT_ERROR`, or
  `VALIDATION_ERROR` rows. Prefer a clear error message over silence.

## Recommendations

Core routes recommendations per plugin and keeps the full record (`ID`,
category, priority, confidence, `ActionDetail`, reasons). `Recommendation.ID`
is the dismissal key, so keep it stable across calls.
