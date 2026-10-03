# Tasks: Dotted Tag Keys

**Input**: [spec.md](./spec.md), [plan.md](./plan.md)
**Issue**: #1608

## Phase 1 - Flattener

- [x] T001 Add `internal/engine/flatten.go` and call it from `ConvertToProto`
  without changing `ConvertValueToString`.
- [x] T002 Cover scalar, nested, array, empty, nil, number, bool, `__`,
  sentinel, credentials, skipped containers, depth, length, collision, and
  the 50-tag order in `internal/engine/flatten_test.go`.
- [x] T003 Add golden coverage for `sku`, `hardwareProfile`, and
  `rootBlockDevice`, plus a test that every collapsed key is unchanged.

## Phase 2 - Cache and estimate

- [x] T004 Append `/tags-` to `generateProjectedCostResourceKey` and test that
  `sku.capacity` 2 and 4 do not share a key.
- [x] T005 Update the recommendation cache regression to record the one-time miss.
- [x] T006 Add `TestBuildEstimateCostRequest_KeepsNestedAttributes`.

## Phase 3 - Docs

- [x] T007 Document dotted keys, caps, and cache invalidation in
  `docs/src/content/docs/plugins/plugin-sdk.md` and
  `docs/src/content/docs/guides/cache.md`.
- [x] T008 Add the changelog entry and the CLAUDE.md note.
