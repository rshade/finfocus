> **Superseded (2026-09-22):** Option C (core-owned mapping table in the
> adapter) was rejected in favor of plugin-owned mappings behind the
> `ResolveResourceTypes` RPC. See
> `docs/superpowers/specs/2026-03-26-terraform-state-ingestion-design.md`
> for the approved design. This document is kept for historical context only.

# Terraform/OpenTofu State Ingestion Plan

## Overview

Add Terraform and OpenTofu state file parsing to FinFocus, enabling cost
analysis for infrastructure managed by Terraform/OpenTofu — not just Pulumi.

The architecture is already designed for this. The key abstraction is
`engine.ResourceDescriptor` — it's completely IaC-tool-agnostic. The engine,
router, cache, and plugins never know whether their input came from Pulumi or
anything else. Terraform support is purely an **ingestion layer** addition
with CLI wiring — zero engine changes needed.

## Integration Point

The data flow is:

```text
[Input Source] → ingest.Parse*() → ingest.Map*Resource() → engine.ResourceDescriptor → Engine
```

Terraform state parsing slots in at exactly the same level as the existing
Pulumi paths. The engine only sees `ResourceDescriptor{Type, ID, Provider, Properties}`.

## What Needs to Be Built

### 1. `internal/ingest/terraform_state.go` — State Parser

Core new file. Mirrors `state.go` structurally.

**Structs needed** (matching the v4 state JSON):

| Struct | Maps to State JSON |
|---|---|
| `TerraformState` | Root: `version`, `terraform_version`, `serial`, `lineage`, `outputs`, `resources` |
| `TerraformStateResource` | Each entry in `resources[]`: `module`, `mode`, `type`, `name`, `each`, `provider`, `instances` |
| `TerraformStateInstance` | Each entry in `instances[]`: `index_key`, `attributes`, `sensitive_attributes`, `private`, `dependencies` |

**Key functions** (mirroring Pulumi patterns):

- `ParseTerraformState(data []byte) (*TerraformState, error)` — JSON unmarshal
- `LoadTerraformState(path string) (*TerraformState, error)` — file read + parse
- `(*TerraformState).GetManagedResources()` — filter `mode == "managed"`
  (analogous to `GetCustomResources()`)
- `MapTerraformResource(resource TerraformStateResource, instance TerraformStateInstance) (engine.ResourceDescriptor, error)`

### 2. Resource Type Mapping (Hardest Part)

Pulumi uses types like `aws:ec2/instance:Instance`. Terraform uses
`aws_instance`. Plugins currently expect Pulumi-format type tokens.

**Three options:**

| Option | Approach | Pros | Cons |
|---|---|---|---|
| A | Normalize TF→Pulumi at ingestion | Plugins work unchanged | Requires large mapping table, goes stale |
| B | Pass TF types through, plugins handle both | Most flexible long-term | Breaking plugin contract change |
| **C (recommended)** | Normalize at adapter/proto layer | Centralized, adapter already does extraction | Single mapping concern |

**Recommendation: Option C** — Add TF→Pulumi type mapping in
`internal/proto/adapter.go`'s `resolveSKUAndRegion()`. The adapter already
extracts `Provider` from the type string; extending it to normalize TF
resource types fits naturally.

The `extractProvider` function in `map_resource.go:45` currently splits on
`:`, but for TF types it would split on `_` to get `aws` from `aws_instance`.

### 3. `MapTerraformResource` — Property Translation

Mirrors `MapStateResource` at `state.go:186`:

```text
TerraformStateResource + Instance
  → extract provider from type (split on "_" → "aws")
  → instance.attributes becomes Properties
  → inject metadata:
    - "terraform:id"       = attributes["id"] (cloud resource ID)
    - "terraform:arn"      = attributes["arn"] (if present)
    - "terraform:module"   = resource.module path
    - "terraform:provider" = resource.provider config string
  → ResourceDescriptor{Type, ID, Provider, Properties}
```

**Key differences from Pulumi mapping:**

- TF has no URN — use `type.name[index_key]` as the ID
  (e.g., `aws_instance.web[0]`)
- TF `attributes` is a flat JSON object (not split into inputs/outputs) —
  no merge needed
- TF `attributes` includes computed values (like `id`, `arn`, `private_ip`) —
  richer than Pulumi inputs alone
- TF has no `Created`/`Modified` timestamps — no time-based confidence
  estimation possible
- For `for_each`/`count` resources: one `TerraformStateResource` has
  *multiple* instances, each becoming its own `ResourceDescriptor`

### 4. SKU/Region Extraction

The adapter's `resolveSKUAndRegion()` at `adapter.go:825` extracts SKU from
property keys like `instance_type`, `instance_class`, `vm_size`. Terraform
state `attributes` uses **these exact property names** — they're the
Terraform argument names that Pulumi normalizes to camelCase.

So `instance_type` (TF) vs `instanceType` (Pulumi).

**Two approaches:**

1. **Normalize TF snake_case keys to camelCase at ingestion** — single
   conversion point, all downstream code works (recommended)
2. **Extend `mapping.ExtractAWSSKU()` to check both** — no key
   transformation, but scattered dual-format awareness

### 5. CLI Integration

Add a `--terraform-state` flag to both `cost projected` and `cost actual`
commands:

- `cost_projected.go`: Add `--terraform-state` flag, mutually exclusive with
  `--pulumi-json`
- `cost_actual.go`: Add `--terraform-state` flag, mutually exclusive with
  `--pulumi-state` and `--pulumi-json`
- `common_execution.go`: Add `loadAndMapTerraformResources()` function
  (mirrors `loadAndMapResources()` at line 72)

For projected costs from TF state: the state contains the **current**
resource configuration, which is what you'd price. Equivalent to Pulumi's
"same" operation — resources exist and should be priced as-is.

For actual costs from TF state: the `attributes` contain cloud IDs and ARNs
needed for billing API lookups, just like Pulumi's `MapStateResource` injects
`pulumi:cloudId`.

### 6. OpenTofu Encrypted State Support

If the input is an encrypted OpenTofu state (detected by presence of
`encryption_version` field instead of `terraform_version`), the parser should
return a clear error:

> "encrypted OpenTofu state detected — decrypt with `tofu state pull` first"

Decrypting in-process would require importing OpenTofu's encryption
libraries, which is a much larger dependency. Better to document the workflow.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/ingest/terraform_state.go` | **New** | Parse TF/OTF state JSON, filter managed resources |
| `internal/ingest/terraform_state_test.go` | **New** | Unit tests with sample state fixtures |
| `internal/ingest/terraform_map.go` | **New** | `MapTerraformResource()`, snake→camelCase, metadata injection |
| `internal/ingest/terraform_map_test.go` | **New** | Mapping tests |
| `internal/cli/cost_projected.go` | **Edit** | Add `--terraform-state` flag |
| `internal/cli/cost_actual.go` | **Edit** | Add `--terraform-state` flag |
| `internal/cli/common_execution.go` | **Edit** | Add `loadAndMapTerraformResources()` |
| `examples/plans/` | **New files** | Sample TF state fixtures for testing |

## What Does NOT Change

- `internal/engine/` — zero changes (works on `ResourceDescriptor`)
- `internal/proto/adapter.go` — minimal (possibly extend SKU extraction keys)
- `internal/pluginhost/` — zero changes
- `internal/router/` — zero changes
- Proto/spec — zero changes
- Cache — zero changes (keys based on provider/type/region/sku, not source)

## Open Design Questions

1. **Type format**: Should TF resource types pass through as `aws_instance`
   or be normalized to Pulumi format `aws:ec2/instance:Instance`? Affects
   whether plugins need changes.

2. **Key normalization**: Should `instance_type` → `instanceType` happen at
   ingestion, or should the adapter handle both?

3. **Resource ID format**: For TF resources, use `aws_instance.web[0]`
   (readable) or generate a synthetic URN-like ID?

4. **Scope**: Just state parsing for now, or also TF plan
   (`terraform plan -out=plan.bin && terraform show -json plan.bin`) for
   projected costs from pending changes?

## Reference

- See `tf_state.md` in this repo for the full Terraform/OpenTofu state
  format reference (v4 schema, encryption envelope, backend details, version
  history)
- Existing Pulumi ingestion patterns: `internal/ingest/state.go`,
  `internal/ingest/map_resource.go`
- Engine contract: `internal/engine/types.go` (`ResourceDescriptor`)
- Adapter layer: `internal/proto/adapter.go` (SKU/region extraction)
