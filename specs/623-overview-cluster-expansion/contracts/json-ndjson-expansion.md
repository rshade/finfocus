# Contract: Overview JSON/NDJSON cluster expansion

Applies to `finfocus overview --output json` and `--output ndjson`.

## Invariants

1. `resources` remains a flat array of rows. Expansion adds fields; it never
   nests rows.
2. Rows without expansion participation serialize byte-identically to before
   this feature (all new fields are `omitempty`).
3. A cluster row with children contains `"childUrns": ["<child urn>", ...]`
   in display order.
4. A child row contains `"parentUrn": "<cluster urn>"` and
   `"expansionSource": "live" | "projected"`.
5. Live child rows have `"type": "finfocus:k8s/namespace:Allocation"` and
   URNs of the form `<clusterURN>#ns/<namespace>`.
6. When live data suppressed projected rows, `metadata.expansionNotes`
   contains a note naming the preference and the suppressed count.
7. Live child rows are excluded from `summary` totals; projected children
   contribute exactly as they did as flat rows.
8. NDJSON emits parent and child rows as separate lines, parent first,
   children immediately after their parent.

## Example (JSON excerpt)

```json
{
  "urn": "urn:pulumi:prod::app::aws:eks/cluster:Cluster::cluster",
  "type": "aws:eks/cluster:Cluster",
  "status": "active",
  "childUrns": [
    "urn:pulumi:prod::app::kubernetes:apps/v1:Deployment::api",
    "urn:pulumi:prod::app::kubernetes:apps/v1:Deployment::worker"
  ]
}
```

```json
{
  "urn": "urn:pulumi:prod::app::kubernetes:apps/v1:Deployment::api",
  "type": "kubernetes:apps/v1:Deployment",
  "status": "active",
  "parentUrn": "urn:pulumi:prod::app::aws:eks/cluster:Cluster::cluster",
  "expansionSource": "projected",
  "projectedCost": {"monthlyCost": 54.75, "currency": "USD"}
}
```

```json
{
  "urn": "urn:pulumi:prod::app::aws:eks/cluster:Cluster::cluster#ns/payments",
  "type": "finfocus:k8s/namespace:Allocation",
  "status": "active",
  "parentUrn": "urn:pulumi:prod::app::aws:eks/cluster:Cluster::cluster",
  "expansionSource": "live",
  "projectedCost": {"monthlyCost": 120.50, "currency": "USD",
    "breakdown": {"cpu": 80.10, "memory": 40.40}}
}
```
