# Research: Dotted Tag Keys

## Decision: Keep the 50-tag ceiling

**Rationale**: Issue #1608 measured preview inputs at 37 keys or fewer. The
conformance suite rejects more than `MaxTagCount` (50). The production SDK
allows 256, but a resource that passed core and then failed the contract suite
would be a worse outcome. Deployed state was not measured separately, so it
uses the same cap rather than a second limit. Collapsed keys stay even when
they already exceed 50, because removing them would change current behavior.

**Alternatives considered**: Use the SDK limit of 256. Rejected because the
contract suite is the stricter gate plugins already meet.

## Decision: Fixed credential denylist

**Rationale**: The issue asks whether the list should be configurable. A
config switch would let a deployment publish `adminPassword` as its own tag.
The list stays in code: `password`, `secret`, `token`, `credential`,
`ciphertext`, `privatekey`, matched case-insensitively as substrings.

**Alternatives considered**: A config denylist. Rejected. Operators do not need
to opt in to hiding those segments.

## Decision: Always hash the flattened tag map

**Rationale**: The projected key is `provider/type/region/sku` from top-level
strings only. An object `sku` does not fill the SKU segment, so `sku.capacity`
2 and 4 share an entry today. Once a plugin prices from the dotted key, that
shared entry is a wrong price. Hashing `ConvertToProto` output covers collapsed
keys, dotted keys, and `ref.*` tags. Appending it for every resource
invalidates old projected entries once. The cache is already documented as
safe to delete. `/refs-` stays so a reference-only change remains visible in
the key shape introduced by #1610.

**Alternatives considered**: Hash only dotted keys. Rejected because two
collapsed values with no nested leaves would still need a stable suffix, and a
conditional suffix would make the no-nest key look like the old key while a
later nested edit changed the shape.

## Decision: Do not change ConvertValueToString

**Rationale**: `estimate.go` also calls it, and `BuildEstimateCostRequest`
keeps a nested `Struct`. Flattening only inside `ConvertToProto` limits the
new keys to Supports, projected, batch, and recommendations.
