# Upgrade to finfocus-spec v0.7.0: additive release

v0.7.0 removed and renamed nothing. Bumping `go.mod`, which
`finfocus plugin upgrade` does, is the whole upgrade.

## Optional

These are new and need opting in. None is needed to keep working:

| Feature | Interface |
| --- | --- |
| FOCUS 1.4 columns, `cost_breakdown`, lineage metadata | Fields on existing messages |
| Recommendation scoring | `RecommendationScorerProvider` |
| Contract commitments and invoices | `ContractCommitmentProvider`, `InvoiceDatasetProvider` |
| Per-request credentials | `PerRequestCredentialConsumer` |

Do not adopt these as part of an upgrade. Each is a feature with its own
review.
