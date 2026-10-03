# Implementation Plan: Cross-Resource References

**Branch**: `618-cross-resource-refs` | **Date**: 2026-10-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/618-cross-resource-refs/spec.md`

## Summary

Ingest Pulumi `propertyDependencies` on plan steps and stack exports, carry them
on `ResourceDescriptor.Refs`, and attach `ref.*` tags before projected cost
requests. Fill `Region` only in the single-referenced-region case. Use lenient
pre-flight when a child has those tags and no SKU of its own. Never copy the
referenced SKU into the child's `Sku`.

## Technical Context

**Language/Version**: Go 1.27.1
**Primary Dependencies**: finfocus-spec v0.7.1 (`pluginsdk` validation and SKU/region mapping)
**Storage**: N/A
**Testing**: `go test` with testify
**Target Platform**: Linux, macOS, Windows CLI
**Project Type**: single Go module
**Performance Goals**: one linear pass over the resource slice per projected query
**Constraints**: no finfocus-spec change; provider-agnostic core
**Scale/Scope**: ingest structs, descriptor field, one resolver, request helper

## Constitution Check

- [x] **Plugin-First Architecture**: Core only forwards references. Pricing stays in plugins.
- [x] **Test-Driven Development**: Fixture and request tests cover the new behavior.
- [x] **Cross-Platform Compatibility**: Pure Go, no OS-specific code.
- [x] **Documentation Integrity**: Guide, changelog, and CLAUDE.md describe the tag contract.

## Project Structure

### Documentation

```text
specs/618-cross-resource-refs/
├── spec.md
├── plan.md
├── research.md
└── tasks.md
docs/src/content/docs/guides/resource-references.md
```

### Source Code

```text
internal/ingest/pulumi_plan.go
internal/ingest/state.go
internal/ingest/map_resource.go
internal/engine/refs.go
internal/engine/engine.go
internal/engine/engine_batch.go
internal/proto/refs.go
internal/proto/adapter.go
```

## Design notes

`PrepareProjectedDescriptor` is the single request builder for the batch path
(`buildBatchCostRequest`) and the non-batch path (`clientAdapter.GetProjectedCost`
and `GetProjectedCostWithErrors`). `ApplyCrossResourceRefs` runs once on the
full slice in `GetProjectedCost` and `GetProjectedCostWithErrors` before either
path reads properties.

Cache keys call `cache.BuildProjectedKey` as before and append `/refs-<hash>`
only when `ref.*` properties exist.
