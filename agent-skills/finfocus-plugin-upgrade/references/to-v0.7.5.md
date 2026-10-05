# Upgrade to finfocus-spec v0.7.5: dismissed recommendations and handler status codes

v0.7.5 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for most plugins.

## What changed

- `RecommendationsRequest` gains `include_dismissed`. Plugins that store
  dismissals omit dismissed recommendations unless the host sets it.
  `excluded_recommendation_ids` still wins over `include_dismissed`.
- The conformance harness can take a plugin-supplied sample resource, so a
  plugin whose priced types need specific attributes can describe one instead
  of relying on the generic sample.
- `pluginsdk` no longer rewrites a handler's gRPC status to `Internal`: the
  code a handler returns reaches the host, and a wrapped status carries only
  its own message.

## Recommendation plugins: honor include_dismissed

If your plugin tracks dismissals, filter them out by default and return them
only when asked:

```go
if !req.GetIncludeDismissed() {
    recs = filterDismissed(recs)
}
```

Plugins that do not store dismissals need no change.

## Handlers: check the statuses you return

Because handler status codes now pass through unchanged, review error paths
that forward statuses from upstream calls (cloud APIs, other services). Return
your own codes instead of an upstream status as is: an upstream `NotFound` or
`PermissionDenied` usually means something different inside your plugin.
