# Upgrade to finfocus-spec v0.7.3: structured attributes, longer tag values

v0.7.3 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade.

## What changed

- `ResourceDescriptor.attributes` (field 12, `google.protobuf.Struct`) carries
  a resource's declared properties without flattening. Hosts redact keys that
  start with `__`, credential-like keys, and secret values before sending it,
  and cap it at 65536 bytes (`pluginsdk.MaxAttributesBytes`).
- `pluginsdk.MaxTagValueLength` and the conformance suite's tag value limit
  rose from 256 to 2048 bytes.

## Optional: read nested inputs

A plugin that needs a value deeper than the flattened tags carry can read it
from `attributes` and fall back to tags when the host sent none:

```go
attrs := req.GetResource().GetAttributes()
if v, ok := pluginsdk.AttributeValue(attrs, "sku.capacity"); ok {
    capacity := v.GetNumberValue()
    // ...
}
```

`AttributeValue` returns `(nil, false)` for a missing path, never an error.
Do not log `attributes` verbatim.
