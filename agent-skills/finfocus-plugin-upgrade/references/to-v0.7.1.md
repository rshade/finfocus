# Upgrade to finfocus-spec v0.7.1: additive release, one deprecation

v0.7.1 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade for most plugins.

## Manual: recommendation scorer plugins only

`ScorerInfo.provider_request_id` is deprecated in favor of the repeated
`provider_request_ids`. The old field keeps working until the next major
version, but `staticcheck` (SA1019) now flags code that sets it.

```go
// Before
info := &pbc.ScorerInfo{Name: "jev", Model: model, ProviderRequestId: reqID}

// After
info := &pbc.ScorerInfo{
    Name:               "jev",
    Model:              model,
    ProviderRequestIds: reqIDs,
    ProviderRequestId:  reqIDs[0], //nolint:staticcheck // Hosts that read only the old field.
}
```

Keep setting the deprecated field to the first entry: hosts that predate
v0.7.1 read only that one. If the scorer uses more than one model, list them
in `models`, primary first, with `model` equal to `models[0]`.

## Optional

New fields that need opting in:

| Feature | Where |
| --- | --- |
| Per-region and alternative retail prices | `RegionPrice`, `PriceOption` |
| Allocation period and selector | `AllocateRequest` |
| FOCUS 1.3 provenance on allocation rows | `AllocationRow` |
| Billing account filter for actual costs | `GetActualCostRequest.billing_account_id` |

Do not adopt these as part of an upgrade. Each is a feature with its own
review.
