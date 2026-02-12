# FinFocus Development Instructions

FinFocus is a CLI tool for calculating cloud infrastructure costs from Pulumi infrastructure definitions. It uses a plugin-based architecture to query multiple cost data sources via gRPC.

## Build, Test, and Lint Commands

```bash
make build                              # Build binary to bin/finfocus
make build-all                          # Build binary + all plugins (recorder, Pulumi tool)
make clean                              # Remove build artifacts

make test                               # Run unit tests (fast, default)
go test ./internal/cli/...              # Test specific package
go test -run TestName ./...             # Run single test by name
go test -v -run TestName ./internal/engine/...  # Single test in one package

make test-race                          # Run with race detector
make test-integration                   # Integration tests (slower)
make test-e2e                           # E2E tests (requires AWS credentials)

make lint                               # golangci-lint v2.9.0 + markdownlint + actionlint
make validate                           # go mod tidy -diff + go vet
make docs-lint                          # Lint documentation only

go test -coverprofile=coverage.out ./...  # Coverage report
go tool cover -html=coverage.out
```

**Always run `make lint` and `make test` before committing.**

## Architecture

```text
Pulumi JSON → Ingestion → Resource Descriptors → Router → Engine → Plugins (gRPC) / Specs (YAML) → Output
```

### Dual-Mode Binary

The `finfocus` binary runs as both a standalone CLI and a Pulumi tool plugin. Mode is detected via binary name (`pulumi-tool-finfocus`) or `FINFOCUS_PLUGIN_MODE=true`. This changes the `Use` string and examples but shares all subcommands.

### Core Components

1. **CLI** (`internal/cli/`) — Cobra commands: `cost projected|actual|recommendations|budget|estimate`, `plugin *`, `config *`, `analyzer serve`
2. **Engine** (`internal/engine/`) — Orchestrates cost calculation. Tries plugins first, falls back to local YAML specs in `specs/`. Uses `hoursPerMonth = 730`. Supports table, JSON, NDJSON output. Includes batch processing (threshold: 100 resources), caching, and budget forecasting.
3. **Router** (`internal/router/`) — Routes resource types to the correct plugin based on provider patterns, priority rules, and config-driven routing from `~/.finfocus/config.yaml`.
4. **Proto Adapter** (`internal/proto/`) — Bridge between engine and plugins. Converts `ResourceDescriptor` to protobuf requests, performs pre-flight validation via `pluginsdk`, extracts SKU/Region from resource `Inputs`, and aggregates errors with `CostResultWithErrors`.
5. **Plugin Host** (`internal/pluginhost/`) — gRPC plugin lifecycle. `ProcessLauncher` (TCP) and `StdioLauncher` (stdin/stdout). 10-second timeout, 100ms retry. **Always call `cmd.Wait()` after `Kill()` to prevent zombies.**
6. **Registry** (`internal/registry/`) — Discovers plugins at `~/.finfocus/plugins/<name>/<version>/`. Optional `plugin.manifest.json` validation.
7. **Ingestion** (`internal/ingest/`) — Parses `pulumi preview --json`. **Must inspect `newState` to extract `Inputs`** — without this, property extraction fails and plugins return `InvalidArgument`.
8. **Analyzer** (`internal/analyzer/`) — Implements `pulumirpc.AnalyzerServer` for zero-click cost estimation during `pulumi preview`. Prints ONLY port number to stdout (Pulumi handshake). All logs go to stderr. ADVISORY enforcement only.
9. **Config** (`internal/config/`) — Manages `~/.finfocus/config.yaml` including plugin routing rules, budget definitions, and dismissed recommendation state (`~/.finfocus/dismissed.json`).
10. **TUI** (`internal/tui/`) — Bubble Tea + Lip Gloss with adaptive color schemes.

### Plugin Communication

Plugins communicate via gRPC using protocol buffers from [finfocus-spec](https://github.com/rshade/finfocus-spec). Always use `pluginsdk` constants for environment variables and metadata keys:

```go
pluginsdk.EnvPort              // "FINFOCUS_PLUGIN_PORT"
pluginsdk.EnvLogLevel          // "FINFOCUS_LOG_LEVEL"
pluginsdk.EnvLogFormat         // "FINFOCUS_LOG_FORMAT"
pluginsdk.TraceIDMetadataKey   // "x-finfocus-trace-id" (gRPC metadata)
```

Trace IDs propagate automatically: `TraceInterceptor()` in pluginhost injects them into outgoing gRPC calls; plugins extract via `pluginsdk.TracingUnaryServerInterceptor()`.

### Pre-Flight Validation Pattern

The proto adapter validates requests before making gRPC calls. Invalid resources get a `$0` placeholder with `"VALIDATION: ..."` notes (distinct from plugin errors prefixed `"ERROR:"`):

```go
if err := pluginsdk.ValidateProjectedCostRequest(protoReq); err != nil {
    log.Warn().Str("resource_type", resource.Type).Err(err).Msg("pre-flight validation failed")
    result.Results = append(result.Results, &CostResult{
        Currency: "USD", MonthlyCost: 0,
        Notes: fmt.Sprintf("VALIDATION: %v", err),
    })
    continue
}
```

### Recorder Plugin

`plugins/recorder/` is a reference plugin for development. It records all gRPC requests to JSON files and optionally returns mock responses (`FINFOCUS_RECORDER_MOCK_RESPONSE=true`). Build with `make build-recorder`, install with `make install-recorder`.

## Key Conventions

### Go Standards

- **Go Version**: 1.25.7
- **Imports**: Standard library → third-party → internal packages (enforced by `goimports` in golangci-lint)
- **Error Handling**: Wrap with `%w`: `fmt.Errorf("operation failed: %w", err)`. Sentinel errors: `var ErrName = errors.New("description")`
- **Logging**: Use `internal/logging` with `logging.FromContext(ctx)`. Include `component` and `operation` fields. Use `Debug` for flow, `Info` for milestones, `Warn` for recoverable issues.
- **Context**: Pass `context.Context` through all request lifecycles

### Testing

- **Testify required**: Use `require.*` for setup (stops test), `assert.*` for values (continues test)
- **Coverage**: 80% goal, 95% critical paths; CI enforces 61% minimum
- **Table-driven tests**: Use `wantErr` and `errContains` fields
- **Test both paths**: Success and error cases
- **Error messages**: `assert.Contains(t, err.Error(), "expected text")`

### CLI Command Pattern

1. Constructor: `NewXxxCmd() *cobra.Command`
2. Use `RunE` (not `Run`) for error handling
3. Use `cmd.Printf()` for output (not `fmt.Printf()`)
4. Defer cleanup immediately after resource acquisition
5. Support multiple date formats: `"2006-01-02"`, RFC3339

### Pulumi SDK Import

Use the correct import path for Analyzer development:

```go
pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
// NOT: github.com/pulumi/pulumi/sdk/v3/proto/go/pulumirpc
```

### Naming

- Package names: lowercase, short (`engine`, `config`, `pluginhost`)
- CLI flags: kebab-case (`--pulumi-json`)
- Environment/config keys: uppercase snake (`FINFOCUS_*`)
- Exported types: require Go doc comments

### Documentation

- Markdown frontmatter: `title`, `description`, `layout`. The `title` is the H1; content starts with H2.
- Run `make docs-lint` after Markdown edits.

### Commit Messages

[Conventional Commits](https://www.conventionalcommits.org/): `feat(cli): add --format flag`, `fix(engine): correct monthly cost`, `test(registry): add discovery tests`

## Multi-Repository Ecosystem

| Repository | Purpose |
|---|---|
| [finfocus](https://github.com/rshade/finfocus) (this repo) | CLI tool, plugin host, engine |
| [finfocus-spec](https://github.com/rshade/finfocus-spec) | Protocol buffers, pluginsdk |
| [finfocus-plugin](https://github.com/rshade/finfocus-plugin) | Plugin implementations |

Cross-repo changes require coordination per `.specify/memory/constitution.md`.

## Configuration

Precedence: CLI flags → Environment variables → Config file (`~/.finfocus/config.yaml`) → Defaults

Key environment variables: `FINFOCUS_LOG_LEVEL`, `FINFOCUS_LOG_FORMAT`, `FINFOCUS_TRACE_ID`, `FINFOCUS_PLUGIN_*`

```bash
finfocus --debug cost projected --pulumi-json plan.json   # Debug output
```

## CI/CD

All PRs must pass: unit tests with race detection, 61% coverage gate, golangci-lint v2.9.0, govulncheck, cross-platform builds (Linux/macOS/Windows, amd64/arm64).

## Feature Development

New features MUST use [SpecKit](https://github.com/github/spec-kit) workflow (`/speckit.specify` → `/speckit.clarify` → `/speckit.plan` → `/speckit.tasks` → `/speckit.analyze` → `/speckit.implement`). Minor bug fixes don't require SpecKit.

## Constitution

The constitution (`.specify/memory/constitution.md`) takes **absolute precedence**. Key principles: no TODOs or stubs, complete implementation required, high test coverage mandatory.
