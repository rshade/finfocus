# Data Model: v0.2.1 DX Improvements

**Feature**: v0.2.1 Developer Experience Improvements
**Status**: Draft

## In-Memory Structures

### Plugin List Representation

Used by `internal/cli/plugin_list.go` to aggregate data for display.

```go
type PluginListRow struct {
    Name        string // from Registry
    Version     string // from Registry
    Path        string // from Registry
    SpecVersion string // from GetPluginInfo RPC (or "Legacy", "Error")
    Status      string // "Installed", "Invalid", "Incompatible"
}
```

### Shared Filter Helper

Located in `internal/cli/filters.go`.

```go
// ApplyFilters validates and applies a slice of filter strings to a resource set.
// It handles logging and validation errors centrally.
func ApplyFilters(
    ctx context.Context, 
    resources []engine.ResourceDescriptor, 
    filters []string,
) ([]engine.ResourceDescriptor, error)
```

## RPC Interfaces (Existing)

**Service**: `CostSource` (defined in `finfocus-spec`)

```protobuf
rpc GetPluginInfo(GetPluginInfoRequest) returns (GetPluginInfoResponse);

message GetPluginInfoRequest {}

message GetPluginInfoResponse {
    string name = 1;
    string version = 2;
    string spec_version = 3; // SemVer of the spec implemented
    repeated string supported_clouds = 4;
    map<string, string> metadata = 5;
}
```

## File System Layout

### Plugin Registry

```text
~/.finfocus/plugins/
  ├── aws-public/
  │   ├── v0.0.6/   <-- Candidate for removal with --clean
  │   └── v0.0.7/   <-- Newly installed
  └── ...
```
