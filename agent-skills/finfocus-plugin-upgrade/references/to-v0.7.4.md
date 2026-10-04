# Upgrade to finfocus-spec v0.7.4: a resource descriptor on actual cost

v0.7.4 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade.

## What changed

- `GetActualCostRequest.resource` (field 11, `ResourceDescriptor`) has the
  same meaning as `GetProjectedCostRequest.resource`, with attributes and host
  redaction included. When it is set, pricing dimensions come from it and not
  from tags, and tags stay the resource's cloud tags. The same resource is sent
  on every page.
- Unset means the host sent none. Plugins fall back to `tags`, `resource_id`,
  and `arn`, as before.
- `pluginsdk.ValidateActualCostRequest` validates `resource` when it is set.
- The new Standard conformance test `RPCCorrectness_GetActualCostWithResource`
  sends a valid descriptor. A plugin that ignores it or reports no data passes.
  A plugin that rejects it fails.

## Optional: price actual cost from the descriptor

A list-price plugin can derive actual cost from the same inputs it uses for
projected cost, and fall back to tags when the host sent no descriptor:

```go
if res := req.GetResource(); res != nil {
    sku, region := res.GetSku(), res.GetRegion()
    // ...
} else {
    sku, region := req.GetTags()["sku"], req.GetTags()["region"]
    // ...
}
```

Do not log `attributes` verbatim.
