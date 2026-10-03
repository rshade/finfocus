# Feature Specification: Dotted Tag Keys

**Feature Branch**: `619-dotted-tag-keys`
**Created**: 2026-10-03
**Status**: Implemented
**Input**: GitHub issue #1608

## User Scenarios & Testing

### User Story 1 - Read a nested pricing field (Priority: P1)

A plugin prices an Azure resource from `sku.capacity` or `hardwareProfile.vmSize`.
Core still sends the collapsed top-level string and also sends one dotted key
per scalar leaf so the plugin can read the field by name.

**Why this priority**: Collapsed map text hides the dimensions plugins need.

**Independent Test**: Convert an azure-native `sku` object and assert both
`sku` and `sku.capacity`.

**Acceptance Scenarios**:

1. **Given** `sku` is `{name: P1v3, capacity: 2, tier: PremiumV3}`, **When**
   tags are built, **Then** `sku` stays `P1v3` and `sku.name`, `sku.capacity`,
   and `sku.tier` are added.
2. **Given** `hardwareProfile` is `{vmSize: Standard_D4s_v5}`, **When** tags
   are built, **Then** the collapsed value stays `Standard_D4s_v5` and
   `hardwareProfile.vmSize` is added.
3. **Given** `rootBlockDevice` is a one-element list of `{volumeSize, volumeType}`,
   **When** tags are built, **Then** the collapsed Go map text is unchanged and
   `rootBlockDevice.0.volumeSize` and `rootBlockDevice.0.volumeType` are added.

### User Story 2 - Keep a safe, bounded tag map (Priority: P1)

Nested input must not publish secrets, Pulumi internals, or an unbounded tag
map. The same rules apply to preview inputs and deployed state.

**Why this priority**: A new key that is easier to read than collapsed map text
must not expose a password, and the conformance suite rejects more than 50 tags.

**Independent Test**: A resource with credential fields, `__` segments, tag
maps, a depth of 7, and more than 50 leaves keeps every collapsed key and only
the allowed dotted keys.

**Acceptance Scenarios**:

1. **Given** a path segment that starts with `__` or contains `password`,
   `secret`, `token`, `credential`, `ciphertext`, or `privatekey`, **When**
   tags are built, **Then** that leaf is not a dotted key.
2. **Given** `tags`, `tagsAll`, `labels`, or `annotations`, **When** tags are
   built, **Then** those containers are not walked.
3. **Given** more leaves than the remaining room under 50 total tags, **When**
   tags are built, **Then** collapsed keys are kept and dotted keys are added
   by increasing depth, then by name.

### User Story 3 - Stop sharing a projected price across nested SKUs (Priority: P2)

Two resources of the same type can differ only by `sku.capacity`. The projected
cache key includes a digest of the flattened tag map so they do not share an
entry. Recommendation cache keys hash the same map, so those entries miss once.

**Why this priority**: After plugins read dotted keys, a shared projected entry
would return the wrong price.

**Independent Test**: `generateProjectedCostResourceKey` for capacity 2 differs
from capacity 4, and both keys start with the existing projected prefix plus
`/tags-`.

**Acceptance Scenarios**:

1. **Given** two plans that differ only by `sku.capacity`, **When** projected
   cache keys are built, **Then** the keys differ and both contain `/tags-`.
2. **Given** a resource with no `ref.*` properties, **When** its projected key
   is built, **Then** the key has no `/refs-` suffix.
3. **Given** a resource with `ref.*` properties, **When** its projected key is
   built, **Then** `/refs-` is still appended after `/tags-`.
4. **Given** `EstimateCost` attributes for a nested `sku`, **When** the request
   is built, **Then** `sku.capacity` stays inside the struct and no dotted key
   is added.

### Edge Cases

- Nil leaves, empty maps, and empty arrays add no dotted key. An empty string
  leaf is kept.
- A leaf equal to `04da6b54-80e4-46f7-96ec-b56ff0331ba9` is omitted.
- A dotted key that collides with an existing key keeps the existing value.
- A scalar at six segments is kept. A seventh segment is not.
- New keys longer than 128 characters and new values longer than 256 characters
  are omitted. Collapsed values that are already longer stay as they are.
- A scalar whose name is `tags` is emitted. Only map and array containers with
  that exact name are skipped.

## Requirements

### Functional Requirements

- **FR-001**: `ConvertToProto` MUST keep every key and value it produces today.
- **FR-002**: `ConvertToProto` MUST add one dotted key per scalar leaf of each
  non-empty top-level map or array, using `ConvertValueToString` for the leaf.
- **FR-003**: `ConvertValueToString` MUST NOT change.
- **FR-004**: Dotted keys MUST stop at 6 segments, 128-character keys,
  256-character values, and 50 total tags. Collapsed keys are retained first.
- **FR-005**: The credential denylist is fixed, not configurable. The 50-tag
  cap applies to preview and deployed state.
- **FR-006**: Projected cache keys MUST always append `/tags-` plus the first
  8 bytes of SHA-256 over the sorted flattened tag map, then `/refs-` when
  `ref.*` properties exist.
- **FR-007**: `BuildEstimateCostRequest` MUST keep nested attributes.

### Key Entities

- **Flattened tag map**: `map[string]string` passed as `ResourceDescriptor.tags`.
- **Tag digest**: 16 hex characters appended to the projected cache key.

## Success Criteria

- **SC-001**: Table tests cover scalar, nested map, array index, empty map and
  array, nil, number and bool formatting, `__` segments, the unknown sentinel,
  credential-like keys, skipped tag containers, depth, key and value length,
  and deterministic truncation.
- **SC-002**: Golden objects cover azure-native `sku`, `hardwareProfile`, and
  an AWS `rootBlockDevice` list.
- **SC-003**: A large resource stays at 50 tags and keeps its collapsed keys.
- **SC-004**: `make test` and `make lint` pass.

## Assumptions

- Plugins are not taught to read the new keys in this change.
- finfocus-spec SKU extractors are not extended.
- The projected cache is safe to delete. Entries miss once.
- Traversal is sorted because the tag map feeds cache hashing.
