---
name: finfocus-plugin
description: >
  Guide for building FinFocus cost plugins using the pluginsdk and gRPC protocol.
  Use when creating a new plugin, implementing CostSourceService methods, setting up
  plugin scaffolding, debugging plugin communication, understanding the plugin lifecycle,
  or working with the recorder reference plugin. Triggers on: "create a plugin",
  "implement GetProjectedCost", "plugin gRPC", "pluginsdk", "plugin development",
  "recorder plugin", or any task involving finfocus plugin architecture.
---

# FinFocus Plugin Development

## Plugin Architecture

Plugins communicate with finfocus-core via gRPC using protocol buffers from `finfocus-spec`.
Each plugin is a standalone binary that implements the `CostSourceService`.

```text
Core CLI -> gRPC -> Plugin Binary
                    (CostSourceService)
```

**Two launcher types**:

- `ProcessLauncher` (TCP) - Plugin listens on a port
- `StdioLauncher` (stdin/stdout) - Plugin communicates over stdio

## Quick Start: New Plugin

1. Create Go module with `finfocus-spec` dependency
2. Embed `pluginsdk.BasePlugin` for common functionality
3. Implement required gRPC methods
4. Build binary and install to `~/.finfocus/plugins/<name>/<version>/`

```go
import (
    "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
    pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

type MyPlugin struct {
    *pluginsdk.BasePlugin
    logger zerolog.Logger
    mu     sync.Mutex
}

func (p *MyPlugin) Name() string { return "my-plugin" }
```

## Required Methods

Every plugin must implement these `CostSourceService` methods:

| Method | Purpose |
|--------|---------|
| `GetProjectedCost` | Estimate future costs from resource descriptors |
| `GetActualCost` | Return historical costs for a time range |
| `GetRecommendations` | Return cost optimization recommendations |
| `GetPluginInfo` | Return plugin metadata (name, version, providers) |
| `Shutdown` | Clean up resources on termination |

## Request Validation

Use pluginsdk validation helpers before processing:

```go
func (p *MyPlugin) GetProjectedCost(ctx context.Context, req *pbc.GetProjectedCostRequest) (*pbc.GetProjectedCostResponse, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if err := pluginsdk.ValidateProjectedCostRequest(req); err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
    }
    // Process request...
}
```

## Plugin Installation

Plugins install to `~/.finfocus/plugins/<name>/<version>/`:

```bash
# Build and install
make build-recorder
make install-recorder  # -> ~/.finfocus/plugins/recorder/0.1.0/

# Verify
./bin/finfocus plugin list
./bin/finfocus plugin validate
```

## Environment Variables

Plugins receive these from core via `pluginsdk` constants:

| Constant | Env Var | Purpose |
|----------|---------|---------|
| `pluginsdk.EnvPort` | `FINFOCUS_PLUGIN_PORT` | gRPC port |
| `pluginsdk.EnvLogLevel` | `FINFOCUS_LOG_LEVEL` | Log verbosity |
| `pluginsdk.EnvLogFormat` | `FINFOCUS_LOG_FORMAT` | json or text |
| `pluginsdk.EnvLogFile` | `FINFOCUS_LOG_FILE` | Log file path |
| `pluginsdk.EnvTraceID` | `FINFOCUS_TRACE_ID` | Distributed trace ID |

## Trace ID Propagation

```go
// Server-side: extract trace ID from incoming gRPC metadata
server := grpc.NewServer(
    grpc.UnaryInterceptor(pluginsdk.TracingUnaryServerInterceptor()),
)

// In handlers: get trace ID from context
traceID := pluginsdk.TraceIDFromContext(ctx)
```

## Reference Implementation

The recorder plugin at `plugins/recorder/` demonstrates all patterns.
See [references/recorder-reference.md](references/recorder-reference.md) for
the complete implementation walkthrough.

## Plugin Communication Details

See [references/pluginsdk-api.md](references/pluginsdk-api.md) for the full
pluginsdk API, proto message types, and gRPC patterns.

## Testing Plugins

```bash
go test ./plugins/recorder/...                       # Unit tests
go test ./test/integration/recorder_test.go          # Integration
go test -bench=BenchmarkRecorder ./plugins/recorder/... # Perf (<10ms)
```

## Common Pitfalls

- Always use `sync.Mutex` for thread safety in gRPC handlers
- Return `status.Errorf(codes.InvalidArgument, ...)` for validation errors
- Call `cmd.Wait()` after `Kill()` to prevent zombie processes
- 10-second timeout with 100ms retry delays for plugin startup
- Platform-specific binary detection (Unix permissions vs Windows .exe)
