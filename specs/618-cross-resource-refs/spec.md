# Feature Specification: Cross-Resource References

**Feature Branch**: `618-cross-resource-refs`
**Created**: 2026-10-03
**Status**: Implemented
**Input**: GitHub issue #1610

## User Scenarios & Testing

### User Story 1 - Price a child from its referenced resource (Priority: P1)

An operator runs `finfocus cost projected` on a Pulumi preview. A database, node
pool, or Cosmos child has no region of its own. Core follows `propertyDependencies`
to the resource in the same plan and sends that resource's region on the child
request so a plugin can price the child.

**Why this priority**: Without a region the child never reaches a plugin.

**Independent Test**: Parse the Azure preview fixture and assert the child
request region is the referenced resource's region.

**Acceptance Scenarios**:

1. **Given** a classic SQL database whose `serverId` depends on a server with
   `location: westeurope`, **When** projected cost requests are built, **Then**
   the database `Region` is `westeurope`.
2. **Given** a node pool with `vmSize` and no location, **When** requests are
   built, **Then** `Sku` is the pool's `vmSize` and `Region` is the cluster's
   location.
3. **Given** a native SQL database that already has `location: northeurope`,
   **When** requests are built, **Then** `Region` stays `northeurope`.

### User Story 2 - Tell a plugin where the SKU lives (Priority: P1)

A web app's cost lives on its plan. Core must not price the app as the plan.
It attaches `ref.servicePlanId.sku` or `ref.serverFarmId.sku` and leaves the
app's own `Sku` empty.

**Why this priority**: Copying the plan SKU onto the app would double-count the
plan.

**Independent Test**: A plan priced by its own SKU plus two apps that only carry
`ref.*.sku` totals the plan once.

**Acceptance Scenarios**:

1. **Given** a classic plan with `skuName: S1` and apps pointing at it, **When**
   requests are built, **Then** each app has `ref.servicePlanId.sku=S1` and an
   empty `Sku`.
2. **Given** a native plan whose `sku.name` is `P1v3` and two apps pointing at
   it, **When** their request SKUs are priced, **Then** the total equals the
   plan once.

### User Story 3 - Leave bad references unresolved (Priority: P2)

A property that is not a single in-plan reference produces no `ref.*` tags and
no inferred region. The unknown-value sentinel is never forwarded.

**Why this priority**: Guessing a parent would price the wrong resource.

**Independent Test**: Fixture rows for an empty list, an outside URN, and two
URNs produce no `ref.*` tags.

**Acceptance Scenarios**:

1. **Given** `propertyDependencies` entry `[]`, a missing URN, or several URNs,
   **When** requests are built, **Then** no `ref.*` tags are added for that
   property and no region is inferred from it.
2. **Given** an input equal to `04da6b54-80e4-46f7-96ec-b56ff0331ba9`, **When**
   the request is built, **Then** that value is not a tag, SKU, or region.

### Edge Cases

- Missing, JSON null, and empty-list `propertyDependencies` parse without error.
  Empty lists are not references. Null and missing stay nil.
- A stack export joins by `propertyDependencies`, not by comparing a child input
  to a parent cloud id.
- A delete step reads relationship fields from `oldState`.
- Two properties that point at two resources with different regions do not fill
  the child's `Region`.

## Requirements

### Functional Requirements

- **FR-001**: Plan and state ingestion MUST read `parent`, `dependencies`, and
  `propertyDependencies`. `parent` MUST NOT be used to resolve a pricing reference.
- **FR-002**: `ResourceDescriptor.Refs` MUST map an input property to its URN list,
  dropping empty lists. `ID` remains the resource's own URN.
- **FR-003**: Both the batch and non-batch projected paths MUST build the plugin
  descriptor with `PrepareProjectedDescriptor`.
- **FR-004**: Resolved references MUST be tags `ref.<property>.urn`,
  `ref.<property>.type`, `ref.<property>.region`, and `ref.<property>.sku`.
  A tag is omitted when that value is empty.
- **FR-005**: Core MUST fill `Region` from references only when the child's own
  region is empty and exactly one referenced resource has a region.
- **FR-006**: Core MUST NOT copy a referenced SKU into the child's `Sku`.
- **FR-007**: When `Sku` is empty and `ref.*` tags are present, pre-flight MUST
  use lenient validation. Otherwise it MUST stay strict.
- **FR-008**: Projected cache keys MUST include a hash of `ref.*` properties when
  any exist, and MUST stay unchanged when none exist.
- **FR-009**: The sentinel MUST NOT be sent as a region, SKU, or tag value.

### Pre-flight decision

Children without a SKU (web apps, Cosmos databases) used to die in strict
pre-flight, so a plugin never saw the reference. A placeholder SKU copied from
the plan would price the child as the plan and double-count it.

**Decision**: use `pluginsdk.ValidateProjectedCostRequestLenient` only when the
prepared request has an empty `Sku` and at least one `ref.*` tag. Do not invent
a SKU. The plugin returns a note and no monthly cost when the cost lives on the
referenced resource. Children that already have a SKU, such as a node pool
`vmSize`, stay on the strict validator after region fill.

Classic Azure `skuName` is not a key `ResolveSKUAndRegion` reads. The ref tag's
SKU uses that string when the resolver returns empty. The plan resource's own
`Sku` is unchanged.

### Key Entities

- **Pulumi propertyDependencies**: map from input name to URN list.
- **ResourceDescriptor.Refs**: same map with empty lists removed.
- **ref.* tags**: flat strings on the plugin request.

## Success Criteria

- **SC-001**: The Azure preview fixture covers SQL, both node pools, classic and
  native app plans, and classic and native Cosmos, with the regions and SKUs
  named in the acceptance scenarios.
- **SC-002**: A plan plus two apps totals the plan's SKU price once.
- **SC-003**: Batch and adapter requests for the same properties match
  `PrepareProjectedDescriptor`.
- **SC-004**: `make test` and `make lint` pass.

## Assumptions

- Fixtures reproduce the preview shapes recorded in issue #1610. The original
  provider preview files are not in this repository.
- Plugins ignore unknown tags. No finfocus-spec change is required.
- Which `ref.*` key matters stays in the plugin. Core stays provider-agnostic.
