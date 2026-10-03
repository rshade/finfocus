# Research: Cross-Resource References

## Decision: lenient pre-flight, no placeholder SKU

**Decision**: When a prepared projected request has an empty `Sku` and at least
one `ref.*` tag, call `pluginsdk.ValidateProjectedCostRequestLenient`. Never
copy the referenced SKU into `Sku`.

**Rationale**: Strict validation drops web apps and Cosmos children before a
plugin can read `ref.servicePlanId.sku` or `ref.accountName.region`. A
placeholder SKU equal to the plan would make `AggregateResults` count the plan
once per app. Lenient validation already exists for sparse old-state requests
and still requires provider and resource type.

**Alternatives considered**:

- Placeholder SKU such as `ref` or the plan SKU. Rejected because plugins that
  price from `Sku` would bill the child as the plan.
- Leave strict validation and document a $0 placeholder. Rejected because the
  plugin never sees the tags, so it cannot say where the cost lives.

## Decision: skuName is a ref-tag fallback only

**Decision**: If `ResolveSKUAndRegion` returns an empty SKU, `ref.<property>.sku`
may use a string `skuName` on the referenced resource. The referenced resource's
own request `Sku` is not given that fallback.

**Rationale**: Classic Azure service plans store `S1` or `P1v3` in `skuName`.
The SDK extractor reads `vmSize`, `sku`, and `tier`. Issue #1610 says per-type
SKU key selection is a separate adapter concern, and it also requires
`ref.servicePlanId.sku` for those plans. Limiting the fallback to the ref tag
satisfies the tag contract without pricing every classic plan as `skuName`.

## Decision: propertyDependencies, not parent or cloud id

**Decision**: Resolve only a `propertyDependencies` entry with exactly one URN
that is present in the same slice. Do not use `parent`. Do not join
`child.input == parent.id`.

**Rationale**: `parent` is the stack. Cloud-id equality is a state-only hint and
fails at preview time, when the value is the unknown sentinel. A known literal
such as Cosmos `accountName: cosmos-probe` still has a `propertyDependencies`
URN, so keying off the sentinel would miss it.

## Fixtures

Issue #1610 describes genuine azure 6.40.0 and azure-native 3.28.0 preview
shapes. Those raw preview files are not in the repo. `testdata/azure_property_dependencies.json`
reproduces the documented `newState.propertyDependencies` rows.
