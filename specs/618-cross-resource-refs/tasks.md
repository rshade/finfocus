# Tasks: Cross-Resource References

**Input**: [spec.md](spec.md), [plan.md](plan.md)

## Implementation

- [x] T001 Add `parent`, `dependencies`, and `propertyDependencies` to plan and state structs
- [x] T002 Carry non-empty property dependencies on `ResourceDescriptor.Refs`
- [x] T003 Resolve refs into `ref.*` properties and inherited region in one helper
- [x] T004 Use that helper from the batch and non-batch projected request paths
- [x] T005 Lenient pre-flight only when `Sku` is empty and `ref.*` tags exist
- [x] T006 Include `ref.*` tags in the projected cache key
- [x] T007 Document the tag contract and the pre-flight decision

## Tests

- [x] T008 Missing, null, and empty-list `propertyDependencies`
- [x] T009 Azure preview fixture: SQL, apps, node pools, Cosmos, native SQL
- [x] T010 State export uses `propertyDependencies` and does not join on cloud id
- [x] T011 Plan plus two apps totals the plan SKU once
- [x] T012 Batch and adapter descriptors match `PrepareProjectedDescriptor`
- [x] T013 Sentinel never appears as a tag, SKU, or region
