# Research: Pulumi Analyzer Plugin Integration

**Feature**: 009-analyzer-plugin
**Date**: 2025-12-05
**Status**: Complete

## Executive Summary

This research document consolidates findings on implementing a Pulumi Analyzer plugin for cost estimation. The Pulumi Analyzer protocol is well-documented and stable, with clear patterns for plugin handshake, resource analysis, and diagnostic reporting.

## Research Topics

### 1. Proto Definitions Source

**Decision**: Add `github.com/pulumi/pulumi/sdk/v3` as a dependency

**Rationale**: The `pulumirpc` package is the standard Go implementation for Pulumi's gRPC interfaces. Reimplementing or vendoring manual protos is error-prone and hard to maintain. `go get github.com/pulumi/pulumi/sdk/v3` is the standard way to build Pulumi plugins in Go.

**Alternatives Considered**:

- *Manual Proto Generation*: Generating Go code from raw `.proto` files found in Pulumi repo. Rejected due to maintenance burden.
- *Vendoring*: Copying generated files. Rejected as it breaks easily with upstream updates.

### 2. Pulumi Analyzer Protocol

**Decision**: Implement `pulumirpc.Analyzer` gRPC service interface

**Rationale**: The Pulumi SDK defines a stable Analyzer protocol in `proto/pulumi/analyzer.proto`. This protocol is used by policy packs and analyzers to inspect and validate resources during preview/update operations.

**Key Protocol Methods**:

| Method | Purpose | Required for MVP |
|--------|---------|------------------|
| `Handshake` | Exchange engine address and directories | Yes |
| `ConfigureStack` | Receive stack context (name, project, org) | Yes |
| `GetAnalyzerInfo` | Return plugin metadata and policies | Yes |
| `GetPluginInfo` | Return plugin version info | Yes |
| `AnalyzeStack` | Analyze all resources at end of preview | Yes |
| `Analyze` | Analyze single resource (inputs) | Optional |
| `Remediate` | Transform resource properties | No (MVP) |
| `Configure` | Receive policy configuration | Optional |
| `Cancel` | Graceful shutdown signal | Yes |

**Alternatives Considered**:

1. **Custom plugin protocol**: Would require changes to Pulumi CLI - rejected
2. **Resource provider approach**: Not suitable for cross-resource analysis - rejected
3. **External tool integration**: Breaks "zero-click" experience - rejected

### 3. Plugin Handshake Mechanism

**Decision**: Use random TCP port with stdout handshake

**Rationale**: Pulumi expects analyzer plugins to:

1. Listen on a random available TCP port
2. Print ONLY the port number to stdout as the first output
3. Use stderr for all logging

**Implementation Pattern**:

```go
// Start gRPC server on random port
listener, err := net.Listen("tcp", ":0")
port := listener.Addr().(*net.TCPAddr).Port

// Print port to stdout (handshake)
fmt.Println(port)  // ONLY this goes to stdout

// All other logging goes to stderr
logger := zerolog.New(os.Stderr)
```

**Critical Constraints**:

- ANY output to stdout before/after the port number breaks handshake
- zerolog must be configured with `os.Stderr` writer
- No `fmt.Println()` except for port number

**Alternatives Considered**:

1. **Unix socket**: Not cross-platform - rejected
2. **Named pipe**: Complex on Windows - rejected
3. **Environment variable**: Not supported by Pulumi engine - rejected

### 4. Resource Mapping Strategy

**Decision**: Create new `internal/analyzer/mapper.go` for pulumirpc → ResourceDescriptor conversion

**Rationale**: The spec clarified (Q: How should resource mapping be implemented? → A: New Separate Mapper). This provides clean separation and testability.

**Mapping Schema**:

| pulumirpc.AnalyzerResource | ResourceDescriptor | Transformation |
|---------------------------|-------------------|----------------|
| `type` | `Type` | Direct copy (e.g., "aws:ec2/instance:Instance") |
| `urn` | `ID` | Extract resource ID from URN |
| `properties` | `Properties` | Convert protobuf Struct to map[string]interface{} |
| `provider` | `Provider` | Extract provider name from provider URN |
| - | `Region` | Extract from properties["region"] or provider config |
| - | `SKU` | Extract from properties (instance type, size, etc.) |

**Technical Detail**: `structpb.Struct` -> `map[string]interface{}` conversion is standard but needs careful handling of numeric types (float vs int) which matters for cost calculations.

**URN Parsing**:

```text
Format: urn:pulumi:stack::project::type::name
Example: urn:pulumi:dev::myproject::aws:ec2/instance:Instance::webserver
         │       │    │          │                         │
         └stack  │    └project   └type                     └name
```

**Provider URN Parsing**:

```text
Format: urn:pulumi:stack::project::pulumi:providers:aws::default
        Yields: "aws" as provider name
```

**Alternatives Considered**:

1. **Modify existing ingest package**: Would couple pulumi-plan and analyzer - rejected
2. **Generic adapter pattern**: Over-engineered for single use case - rejected

### 5. Server Implementation

**Decision**: Create `internal/analyzer/server.go` implementing `pulumirpc.AnalyzerServer`

**Rationale**: Keeps the gRPC handler logic separate from the core engine. The server will wrap `internal/engine.Engine` to delegate cost calculations.

### 6. Diagnostic Message Format

**Decision**: Return costs as `AnalyzeDiagnostic` with `ADVISORY` enforcement level

**Rationale**: The spec mandates (FR-005) that cost diagnostics use INFO/WARNING severity, never ERROR in MVP. This ensures costs never block deployments.

**Diagnostic Structure**:

```go
&pulumirpc.AnalyzeDiagnostic{
    PolicyName:        "cost-estimate",
    PolicyPackName:    "finfocus",
    PolicyPackVersion: version.Version,
    Description:       "Resource cost estimation",
    Message:           "Estimated Monthly Cost: $25.50 USD",
    EnforcementLevel:  pulumirpc.EnforcementLevel_ADVISORY,
    Urn:              resource.Urn,
    Severity:         pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW,
}
```

**Message Format Patterns**:

| Scenario | Severity | Message |
|----------|----------|---------|
| Cost calculated | LOW | "Estimated Monthly Cost: $X.XX USD (source: plugin-name)" |
| Fallback to spec | LOW | "Estimated Monthly Cost: $X.XX USD (source: local-spec)" |
| No pricing data | MEDIUM | "Unable to estimate cost: unsupported resource type" |
| Plugin error | MEDIUM | "Cost estimation unavailable: [error summary]" |

**Stack Summary Diagnostic**:

```go
&pulumirpc.AnalyzeDiagnostic{
    PolicyName:  "stack-cost-summary",
    Description: "Total stack cost summary",
    Message:     "Total Estimated Monthly Cost: $150.00 USD (10 resources analyzed)",
    // No URN - stack-level diagnostic
}
```

### 7. Logging & Handshake Safety

**Decision**: Enforce `internal/logging` configuration to use `stderr` by default for the Analyzer command

**Rationale**: The Pulumi engine initiates the plugin by executing the binary and reading the port from `stdout`. Any other output on `stdout` corrupts this handshake. `zerolog` (used by `internal/logging`) is already configured to write to `stderr` by default, but we must explicitly ensure no `fmt.Printf` usage in the analyzer startup path except for the port.

**Logger Configuration**:

```go
// In analyzer serve command
func configureAnalyzerLogging() zerolog.Logger {
    // CRITICAL: All logs to stderr, never stdout
    return zerolog.New(os.Stderr).
        With().
        Str("component", "analyzer").
        Timestamp().
        Logger()
}
```

**Log Levels for Analyzer**:

| Level | Usage |
|-------|-------|
| TRACE | Property extraction, per-resource details |
| DEBUG | Resource mapping, plugin selection |
| INFO | Stack analysis start/complete, total cost |
| WARN | Plugin failures, fallback usage, timeouts |
| ERROR | Fatal errors (still won't block preview) |

### 8. Cost Plugin Integration

**Decision**: Use existing `internal/engine` and `internal/pluginhost` infrastructure

**Rationale**: The spec requires (FR-004, FR-009) reusing existing engine and registry packages. No new cost plugin protocol needed.

**Integration Flow**:

```text
1. AnalyzeStack RPC receives []AnalyzerResource
2. Mapper converts to []ResourceDescriptor
3. Engine.GetProjectedCost() calculates costs via plugins
4. If plugins fail, engine falls back to local specs
5. CostResults converted to []AnalyzeDiagnostic
6. Response sent back to Pulumi engine
```

**Configuration Path** (from spec clarifications):

```yaml
# ~/.finfocus/config.yaml
analyzer:
  plugins:
    vantage:
      path: ~/.finfocus/plugins/vantage/v1.0.0/finfocus-plugin-vantage
      enabled: true
      env:
        VANTAGE_API_KEY: "${VANTAGE_API_KEY}"
    kubecost:
      path: ~/.finfocus/plugins/kubecost/v1.0.0/finfocus-plugin-kubecost
      enabled: true
```

### 9. Error Handling Strategy

**Decision**: Graceful degradation with WARNING diagnostics

**Rationale**: The spec mandates (FR-005, SC-004) that cost estimation failures never block deployments in MVP mode.

**Error Handling Matrix**:

| Error Type | Behavior | Diagnostic |
|------------|----------|------------|
| Plugin timeout | Use next plugin or spec | WARNING: "Plugin X timed out, using fallback" |
| Network failure | Use local specs | WARNING: "Network unavailable, using cached specs" |
| Unsupported resource | Skip silently | (debug log only) |
| Invalid resource data | Skip with warning | WARNING: "Could not parse resource properties" |
| All plugins fail | Return empty costs | WARNING: "No cost data available" |

**Timeout Configuration** (from spec Q&A):

```yaml
# ~/.finfocus/config.yaml
analyzer:
  timeout:
    per_resource: 5s      # Per-resource timeout
    total: 60s            # Overall stack analysis timeout
    warn_threshold: 30s   # Log warning if analysis exceeds this
```

### 10. CLI Subcommand Design

**Decision**: Distinct `finfocus analyzer serve` subcommand (non-hidden)

**Rationale**: The spec clarifies (Q: Which approach for invoking analyzer mode? → A: Distinct Subcommand, non-hidden).

**Command Structure**:

```bash
finfocus analyzer serve [flags]

Flags:
  --debug         Enable debug logging
  --config        Path to config file (default: ~/.finfocus/config.yaml)
  --timeout       Overall analysis timeout (default: 60s)

# Usage in Pulumi.yaml:
# analyzers:
#   - cost
# (Pulumi will run: finfocus analyzer serve)
```

**Plugin Naming Convention**:

- Binary: `pulumi-analyzer-cost` (or `finfocus` with analyzer subcommand)
- Install path: `~/.pulumi/plugins/analyzer-cost-vX.Y.Z/`
- Or: symlink `pulumi-analyzer-cost` → `finfocus`

## Clarifications Resolved

- **Config Discovery**: Delegated to `internal/registry` and `internal/config`.
- **Mapper**: Validated as necessary.
- **Interface**: `pulumirpc.Analyzer` confirmed as target.
- **Plugin Config**: Environment variables for child plugin configuration.
- **Diagnostics**: Both per-resource and stack summary required.
- **Logging**: Use existing zerolog with stderr output.

## Dependencies and Risks

### New Dependencies

| Dependency | Purpose | Risk |
|------------|---------|------|
| `github.com/pulumi/pulumi/sdk/v3/proto/go` | Analyzer protocol | Low - stable API |
| None new | Reusing existing deps | N/A |

### Technical Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Pulumi protocol changes | Low | Version lock, integration tests |
| Port collision on startup | Low | OS handles with `:0` binding |
| Large stack timeouts | Medium | Configurable timeouts per spec |
| Plugin process leaks | Medium | Proper cleanup on Cancel RPC |

## Implementation Recommendations

1. **Start with `AnalyzeStack`**: The MVP only needs stack-level analysis at preview end
2. **Skip `Remediate`**: Not needed for cost estimation
3. **Implement `Cancel` early**: Prevents zombie processes
4. **Test with real Pulumi CLI**: Integration tests against actual `pulumi preview`
5. **Use table-driven tests**: For resource mapping edge cases

## References

- Pulumi Analyzer Protocol: `$GOPATH/src/github.com/pulumi/pulumi/proto/pulumi/analyzer.proto`
- Pulumi SDK Implementation: `$GOPATH/src/github.com/pulumi/pulumi/sdk/go/common/resource/plugin/analyzer_plugin.go`
- FinFocus Spec Protocol: `$GOPATH/src/github.com/rshade/finfocus-spec/proto/finfocus/v1/costsource.proto`
- Existing Engine: `$GOPATH/src/worktrees/analyzer-plugin/internal/engine/engine.go`
