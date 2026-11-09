# Upgrade to finfocus-spec v0.7.12: capability-scoped Supports

v0.7.12 removed and renamed nothing in the protocol. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for most plugin binaries.
The release adds one optional request field. A plugin that ignores it keeps
answering as before.

## What changed

- `SupportsRequest.capability` names the operation the host is about to call
  for the resource, such as `PLUGIN_CAPABILITY_PROJECTED_COSTS` or
  `PLUGIN_CAPABILITY_ACTUAL_COSTS`. `PLUGIN_CAPABILITY_UNSPECIFIED` (the
  default, and what older hosts send) means any capability. Hosts treat an
  answer to an unspecified request as covering every capability.
- `pluginsdk.RequestedCapability(req)` returns the requested capability. It
  maps a value the SDK does not recognize to `UNSPECIFIED`, so a newer host
  keeps working, and logs a warning once per distinct value, for at most 64
  values.
- `pluginsdk.Client.SupportsFor` asks about one capability. `Client.Supports`
  still asks about any capability.
- Errors returned by `pluginsdk.Client` keep the gRPC status of the failed
  call. A plugin that forwards one from its own handler, such as a router that
  calls a child plugin, now returns the child's code instead of `Unknown`.
- Conformance adds `RPCCorrectness_SupportsCapability` at the Standard level.
  It sends an unspecified `Supports` request, then one per declared
  resource-scoped capability, and checks only that each answer is a valid
  response. A plugin that ignores the field passes.
- The recommendation spot check now fails a plugin that declares
  `PLUGIN_CAPABILITY_RECOMMENDATIONS` and answers `GetRecommendations` with
  `Unimplemented`. A plugin that declares no capabilities and answers
  `Unimplemented` still passes.

## Plugins whose support differs by operation

Read the capability in `SupportsProvider.Supports` and answer for it:

```go
switch pluginsdk.RequestedCapability(req) {
case pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS:
    return &pbc.SupportsResponse{Supported: false, Reason: "no billing data for this type"}, nil
default:
    return &pbc.SupportsResponse{Supported: true}, nil
}
```

Keep the answer for `UNSPECIFIED` true when any capability covers the
resource, because older hosts send only that value.

## Plugins that declare recommendations

If `GetPluginInfo` lists `PLUGIN_CAPABILITY_RECOMMENDATIONS`, implement
`GetRecommendations`, or remove the capability from the declared list.

## Plugins that forward errors from `pluginsdk.Client`

The error message is unchanged. Code that read `status.Code(err)` and expected
`Unknown` now sees the child's code. Branch on that code instead.
