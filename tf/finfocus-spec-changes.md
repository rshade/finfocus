# finfocus-spec Changes for Terraform Type Resolution

**Status:** Shipped in finfocus-spec v0.6.1
**Scope:** `github.com/rshade/finfocus-spec` — proto + Go SDK
**Depends on:** Approved design in `docs/superpowers/specs/2026-03-26-terraform-state-ingestion-design.md` (Section 1)
**Consumed by:** finfocus core (`internal/cli/common_execution.go` → `resolveResourceTypes()`)

## Motivation

Plugins own provider-specific type mappings (e.g., `aws_instance` → `aws:ec2/instance:Instance`).
The spec needs to define the RPC contract for the core to ask plugins "what Pulumi tokens do
these Terraform types map to?" — with capability detection so older plugins degrade gracefully.

## Proto Changes

All changes target `proto/finfocus/v1/`.

### 1. New Capability — `enums.proto`

Add to the `PluginCapability` enum (after `PLUGIN_CAPABILITY_BATCH_COST = 12`):

```protobuf
// Plugin implements ResolveResourceTypes RPC for source-format type mapping
PLUGIN_CAPABILITY_RESOLVE_RESOURCE_TYPES = 13;
```

> **Note:** Verify `13` is still the next available value at implementation time.
> Parallel spec PRs could claim it first.

### 2. New Enum — `enums.proto`

```protobuf
// SourceFormat identifies the IaC tool that produced a resource type string.
// Used by ResolveResourceTypes to select the correct mapping table.
enum SourceFormat {
  SOURCE_FORMAT_UNSPECIFIED = 0;
  // Terraform / OpenTofu resource types (e.g., "aws_instance")
  SOURCE_FORMAT_TERRAFORM = 1;
  // AWS CloudFormation resource types (e.g., "AWS::EC2::Instance") — future
  SOURCE_FORMAT_CLOUDFORMATION = 2;
}
```

### 3. New RPC — `costsource.proto`

Add to `CostSourceService` (after `BatchCost`):

```protobuf
// ResolveResourceTypes maps source-format resource types to Pulumi type tokens.
// Enables the core to translate Terraform/CloudFormation types before pricing.
// Plugins that do not implement this RPC return empty mappings (graceful fallback).
rpc ResolveResourceTypes(ResolveResourceTypesRequest) returns (ResolveResourceTypesResponse);
```

### 4. New Messages — `costsource.proto`

```protobuf
// ResolveResourceTypesRequest asks the plugin to map source-format types
// to Pulumi type tokens.
message ResolveResourceTypesRequest {
  // Source-format resource type strings to resolve.
  // Example: ["aws_instance", "aws_s3_bucket", "aws_rds_cluster"]
  repeated string source_types = 1;

  // The IaC tool format these types come from.
  SourceFormat source_format = 2;
}

// ResolveResourceTypesResponse returns mappings keyed by source type string.
// Types the plugin cannot map are omitted from the response (not an error).
message ResolveResourceTypesResponse {
  // Mappings keyed by the source_type string from the request.
  // Only types the plugin can resolve are included.
  map<string, ResourceTypeMapping> mappings = 1;
}

// ResourceTypeMapping describes how a single source-format type maps to Pulumi.
message ResourceTypeMapping {
  // The Pulumi type token (e.g., "aws:ec2/instance:Instance").
  string pulumi_token = 1;

  // Whether this plugin can price resources of this type.
  // A plugin may know the mapping but not support pricing.
  bool supported = 2;

  // Optional property name overrides: source_key -> pulumi_key.
  // When populated, the core uses these instead of mechanical snake_to_camel.
  // Intentionally unused in the initial implementation — designed in for future
  // edge cases where TF and Pulumi property names diverge non-mechanically.
  map<string, string> property_mappings = 3;
}
```

### Design Notes

- **Response omits `source_format`** — the caller already knows what format it sent.
  Avoids redundant bytes on the wire.
- **Unmapped types are omitted**, not returned with `supported = false`. The core
  interprets absence as "try fallback" (raw TF type + `snake_to_camel` properties).
- **`property_mappings` is reserved for future use.** No plugin populates it initially.
  When a plugin does populate it, the core applies overrides on top of the default
  `snake_to_camel` conversion — no core release needed to handle provider quirks.

## SDK Changes (Go)

All changes target `sdk/go/pluginsdk/`.

### 1. New Optional Interface — `sdk.go`

```go
// ResolveResourceTypesProvider is an optional interface for plugins that can
// map non-Pulumi resource types (Terraform, CloudFormation) to Pulumi tokens.
// Plugins that implement this interface automatically gain the
// PLUGIN_CAPABILITY_RESOLVE_RESOURCE_TYPES capability.
type ResolveResourceTypesProvider interface {
    ResolveResourceTypes(ctx context.Context, req *pbc.ResolveResourceTypesRequest) (*pbc.ResolveResourceTypesResponse, error)
}
```

Follows the existing pattern: `RecommendationsProvider`, `BudgetsProvider`, `DismissProvider`, `BatchCostHandler`.

### 2. Server Handler — `sdk.go`

Add RPC handler following the existing type-assertion pattern:

```go
func (s *Server) ResolveResourceTypes(ctx context.Context, req *connect.Request[pbc.ResolveResourceTypesRequest]) (*connect.Response[pbc.ResolveResourceTypesResponse], error) {
    if provider, ok := s.plugin.(ResolveResourceTypesProvider); ok {
        resp, err := provider.ResolveResourceTypes(ctx, req.Msg)
        if err != nil {
            return nil, err
        }
        return connect.NewResponse(resp), nil
    }
    // Graceful fallback: return empty mappings (not Unimplemented).
    // The core interprets empty mappings as "use raw TF type + snake_to_camel".
    return connect.NewResponse(&pbc.ResolveResourceTypesResponse{}), nil
}
```

**Default behavior is empty response**, not `Unimplemented`. This matches the design
principle that older plugins degrade gracefully — the core falls back to raw TF types
with mechanical property conversion.

### 3. Capability Inference — `plugin_info.go`

Update `inferCapabilities()`:

```go
// Add after the BatchCostHandler check:
if _, ok := plugin.(ResolveResourceTypesProvider); ok {
    capabilities = append(capabilities, pbc.PluginCapability_PLUGIN_CAPABILITY_RESOLVE_RESOURCE_TYPES)
}
```

Update constants:

```go
const (
    baseCapabilities     = 4  // unchanged
    optionalCapabilities = 6  // was 5: +RESOLVE_RESOURCE_TYPES
    maxCapabilities      = 10 // was 9: +RESOLVE_RESOURCE_TYPES
)
```

### 4. TypeRegistry Helper — new file `type_registry.go`

The `TypeRegistry` provides a declarative API for plugin authors to register their
type mappings. The SDK auto-implements the `ResolveResourceTypes` RPC from it.

```go
// TypeRegistry holds source-format → Pulumi type token mappings.
// It contains NO provider-specific data — plugins populate it at init time.
// Thread-safe for concurrent reads after initial registration.
type TypeRegistry struct {
    mu       sync.RWMutex
    mappings map[pbc.SourceFormat]map[string]ResourceTypeMapping
}

// NewTypeRegistry creates an empty registry.
func NewTypeRegistry() *TypeRegistry

// Register adds a single type mapping.
func (r *TypeRegistry) Register(format pbc.SourceFormat, sourceType, pulumiToken string)

// RegisterBatch adds multiple mappings at once.
// Keys are source types, values are Pulumi tokens. All are marked supported=true.
func (r *TypeRegistry) RegisterBatch(format pbc.SourceFormat, mappings map[string]string)

// Resolve implements the ResolveResourceTypes logic.
// Returns only types found in the registry; unknown types are omitted.
func (r *TypeRegistry) Resolve(sourceTypes []string, format pbc.SourceFormat) *pbc.ResolveResourceTypesResponse
```

**Plugin author usage:**

```go
func NewPlugin() *MyPlugin {
    reg := pluginsdk.NewTypeRegistry()
    reg.RegisterBatch(pbc.SourceFormat_SOURCE_FORMAT_TERRAFORM, map[string]string{
        "aws_instance":   "aws:ec2/instance:Instance",
        "aws_s3_bucket":  "aws:s3/bucket:Bucket",
        "aws_rds_cluster": "aws:rds/cluster:Cluster",
        // ... hundreds more
    })
    return &MyPlugin{registry: reg}
}

func (p *MyPlugin) ResolveResourceTypes(ctx context.Context, req *pbc.ResolveResourceTypesRequest) (*pbc.ResolveResourceTypesResponse, error) {
    return p.registry.Resolve(req.SourceTypes, req.SourceFormat), nil
}
```

### 5. Server Integration with TypeRegistry — `sdk.go`

Update `NewServerWithRegistry` and `NewServerWithOptions` to accept an optional
`TypeRegistry`. When provided AND the plugin does not implement
`ResolveResourceTypesProvider` directly, the server auto-implements the RPC
using the registry:

```go
func NewServerWithRegistry(plugin Plugin, registry *TypeRegistry) *Server
```

This means plugin authors can choose:
- **Direct interface**: Implement `ResolveResourceTypesProvider` for full control
- **Registry shortcut**: Pass a `TypeRegistry` to the server constructor, SDK handles the RPC

## Mapping Data Ownership

Provider-specific mapping tables live in the **primary plugin** for that provider:

```text
finfocus-plugin-aws-public/
  types/
    types.go    // RegisterAWS(registry) — all TF→Pulumi mappings for AWS

finfocus-plugin-aws-ce/
  go.mod        // requires github.com/rshade/finfocus-plugin-aws-public
  main.go       // imports "finfocus-plugin-aws-public/types"
```

The spec SDK provides the `TypeRegistry` mechanism. It contains **no provider-specific
mapping data**. The primary plugin is the authority on its provider's resource types.
Consuming plugins import from the primary as a Go dependency.

## Testing Requirements (spec repo)

### Unit Tests

| Test | Scope |
| --- | --- |
| `TypeRegistry.Register` / `RegisterBatch` | Registration and retrieval |
| `TypeRegistry.Resolve` with known/unknown types | Omission behavior for unmapped types |
| `TypeRegistry.Resolve` with wrong `SourceFormat` | Returns empty for unregistered formats |
| Capability inference with `ResolveResourceTypesProvider` | Auto-detection of new capability |
| Server handler with/without provider interface | Graceful empty response fallback |
| Server with `TypeRegistry` (no direct interface) | Auto-implementation via registry |

### Integration Tests

| Test | Scope |
| --- | --- |
| Full RPC round-trip via Connect | Request → handler → response |
| `GetPluginInfo` includes new capability | Capability reported correctly |

## Files Changed in finfocus-spec

| File | Action | Change |
| --- | --- | --- |
| `proto/finfocus/v1/enums.proto` | Edit | Add `PLUGIN_CAPABILITY_RESOLVE_RESOURCE_TYPES = 13`, add `SourceFormat` enum |
| `proto/finfocus/v1/costsource.proto` | Edit | Add `ResolveResourceTypes` RPC, add 3 new messages |
| `sdk/go/pluginsdk/sdk.go` | Edit | Add `ResolveResourceTypesProvider` interface, add server handler |
| `sdk/go/pluginsdk/plugin_info.go` | Edit | Update `inferCapabilities()`, update constants |
| `sdk/go/pluginsdk/type_registry.go` | New | `TypeRegistry` helper |
| `sdk/go/pluginsdk/type_registry_test.go` | New | TypeRegistry unit tests |
| `sdk/go/pluginsdk/sdk_test.go` | Edit | Add ResolveResourceTypes handler tests |
| `sdk/go/pluginsdk/capabilities_test.go` | Edit | Add capability inference test for new interface |

## Cross-Repo Coordination

### Execution Order

1. **finfocus-spec** — Proto + SDK changes (this spec)
2. **finfocus** — Core ingestion + CLI (`tf/tf_ingest_plan.md`)
3. **finfocus-plugin-aws-public** — Mapping data + `ResolveResourceTypesProvider` impl

Steps 1 and 2 can proceed in parallel: the core uses capability detection, so it works
with or without the new RPC. Step 3 requires step 1 (spec release with new proto).

### Version Bump

These changes warrant a **minor version bump** of finfocus-spec (e.g., v0.7.0):
- New RPC (additive, non-breaking)
- New enum (additive)
- New capability value (additive)
- New SDK interface (optional, non-breaking)
