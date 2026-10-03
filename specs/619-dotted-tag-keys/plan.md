# Implementation Plan: Dotted Tag Keys

**Branch**: `619-dotted-tag-keys` | **Date**: 2026-10-03 | **Spec**: [spec.md](./spec.md)
**Input**: GitHub issue #1608

## Summary

`ConvertToProto` keeps today's collapsed keys and adds dotted scalar leaves.
`ConvertValueToString` and `BuildEstimateCostRequest` stay unchanged. The
projected cache key always gains a digest of the flattened tag map.

## Technical Context

**Language**: Go 1.27.1
**Primary dependencies**: existing engine, proto adapter, BoltDB cache
**Storage**: projected cache keys change shape once; the cache file is safe to delete
**Testing**: `go test` on `internal/engine`, `internal/proto`, and `internal/ingest`
**Constraints**: constitution principle I (provider-agnostic). No finfocus-spec change.

## Constitution Check

- No provider-specific pricing. The flattener is generic.
- No TODO or stub. Caps and the denylist are implemented.
- Tests call `ConvertToProto`, `generateProjectedCostResourceKey`,
  `generateRecommendationsCacheKey`, and `BuildEstimateCostRequest`.

## Project Structure

### Source code

- `internal/engine/flatten.go`: walker, caps, tag digest
- `internal/engine/engine.go`: `ConvertToProto` calls the walker; projected keys
  append `/tags-`
- `internal/engine/flatten_test.go`: acceptance tests
- `docs/src/content/docs/plugins/plugin-sdk.md`
- `docs/src/content/docs/guides/cache.md`

## Design decisions

- The cap stays 50 total tags, the conformance `MaxTagCount`. The SDK allows
  256, but the contract suite is stricter and preview inputs peak under 50.
  Deployed state uses the same cap. Collapsed keys are always retained.
  Dotted keys fill remaining room by depth, then by name. One debug log records
  `dotted_kept` and `dotted_dropped` with counts only.
- The credential denylist is fixed: `password`, `secret`, `token`,
  `credential`, `ciphertext`, `privatekey`. It is not configurable.
- `tags`, `tagsAll`, `labels`, and `annotations` are exact container names and
  are not walked. A scalar with one of those names is still emitted.
- The projected digest is always appended, including when the map has no dotted
  keys. Form: `/tags-` plus the first 8 bytes of SHA-256 over sorted
  `key=value` pairs of the `ConvertToProto` result. `/refs-` from #1610 stays
  after it. Existing projected entries miss once.
- Recommendation keys already hash `ConvertToProto`, so dotted keys change the
  hash without a second suffix. Existing recommendation entries miss once.
- A dotted key that collides with a collapsed key keeps the collapsed value.
- Length skips happen before the candidate list and are not counted as
  truncation.
