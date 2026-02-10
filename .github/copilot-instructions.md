---
title: "FinFocus Development Instructions"
description: "Development guidelines and instructions for the FinFocus CLI tool"
layout: "docs"
---

## FinFocus Development Instructions

FinFocus is a CLI tool for calculating cloud infrastructure costs from Pulumi infrastructure definitions. It uses a plugin-based architecture to query multiple cost data sources.

## Build, Test, and Lint Commands

### Building

```bash
make build              # Build binary to bin/finfocus
make build-all          # Build binary + all plugins
make clean              # Remove build artifacts
```

### Testing

```bash
# Unit tests (fast, default)
make test               # Run unit tests
go test ./internal/cli/...          # Test specific package
go test -run TestName ./...         # Run single test by name

# Other test types
make test-race          # Run with race detector
make test-integration   # Integration tests (slower)
make test-e2e           # E2E tests (requires AWS credentials)

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Linting & Validation

```bash
make lint               # Run golangci-lint + markdownlint
make validate           # go mod tidy + go vet
make docs-lint          # Lint documentation only
```

**CRITICAL**: Always run `make lint` and `make test` before committing.

## Architecture

### Core Components

```text
Pulumi JSON → Ingestion → Resource Descriptors → Engine → Plugins/Specs → Output
```

1. **CLI** (`internal/cli/`) - Cobra-based commands:
   - `cost projected` - Estimate costs from Pulumi preview
   - `cost actual` - Historical costs with time ranges
   - `cost recommendations` - Cost optimization insights
   - `cost budget` - Budget tracking and alerts
   - `plugin *` - Plugin management (install/list/validate/etc)
   - `analyzer serve` - Pulumi Analyzer gRPC server

2. **Engine** (`internal/engine/`) - Core orchestration:
   - Queries plugins via gRPC or falls back to local YAML specs
   - Supports table, JSON, NDJSON output formats
   - Cross-provider aggregation with time-based grouping
   - Budget health monitoring and forecasting
   - Uses `hoursPerMonth = 730` for monthly calculations

3. **Plugin Host** (`internal/pluginhost/`) - gRPC plugin management:
   - `ProcessLauncher` (TCP) and `StdioLauncher` (stdin/stdout)
   - 10-second timeout with 100ms retry delays
   - **CRITICAL**: Always call `cmd.Wait()` after `Kill()` to prevent zombies

4. **Registry** (`internal/registry/`) - Plugin discovery:
   - Scans `~/.finfocus/plugins/<name>/<version>/`
   - Optional `plugin.manifest.json` validation

5. **Ingestion** (`internal/ingest/`) - Pulumi plan parsing:
   - **CRITICAL**: Must inspect `newState` in Pulumi JSON to extract `Inputs`

6. **Analyzer** (`internal/analyzer/`) - Pulumi Analyzer integration:
   - Implements `pulumirpc.AnalyzerServer`
   - Prints ONLY port number to stdout (Pulumi handshake protocol)
   - All logs go to stderr

7. **TUI** (`internal/tui/`) - Terminal UI components:
   - Bubble Tea + Lip Gloss
   - Adaptive color schemes (light/dark detection)

### Plugin Communication

- Plugins communicate via gRPC using protocol buffers from [finfocus-spec](https://github.com/rshade/finfocus-spec)
- Environment variables use constants from `pluginsdk`:
  - `pluginsdk.EnvPort` - "FINFOCUS_PLUGIN_PORT"
  - `pluginsdk.EnvLogLevel` - "FINFOCUS_LOG_LEVEL"
  - `pluginsdk.TraceIDMetadataKey` - "x-finfocus-trace-id" (gRPC metadata)

## Key Conventions

### Go Standards

- **Go Version**: 1.25.7
- **Imports**: Standard library → third-party → internal packages
- **Error Handling**: Always wrap with `%w`: `fmt.Errorf("operation failed: %w", err)`
- **Logging**: Use `internal/logging` with structured fields (`component`, `operation`)
  - Get logger: `log := logging.FromContext(ctx)`
  - Use `Debug` for flow, `Info` for milestones, `Warn` for recoverable issues
- **Context**: Pass `context.Context` through request lifecycles

### Testing Requirements

- **Testify Required**: All tests MUST use `testify/assert` and `testify/require`
  - Use `require.*` for setup failures (stops test)
  - Use `assert.*` for value comparisons (continues test)
- **Coverage**: 80% minimum, 95% for critical paths (project goal; the enforced CI gate is 61%)
- **Table-driven tests**: Use `wantErr` and `errContains` fields
- **Test both paths**: Success and error cases
- **Error messages**: Use `assert.Contains(t, err.Error(), "expected text")`

Example:
```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFunction(t *testing.T) {
    result, err := Function()
    require.NoError(t, err)         // Setup check
    assert.Equal(t, expected, result)  // Value check
}
```

### CLI Command Pattern

Each command follows this pattern:
1. Constructor: `NewXxxCmd() *cobra.Command`
2. Use `RunE` (not `Run`) for error handling
3. Use `cmd.Printf()` for output (not `fmt.Printf()`)
4. Defer cleanup immediately after resource acquisition
5. Support multiple date formats: "2006-01-02", RFC3339

### Naming Conventions

- Package names: lowercase, short (`engine`, `config`, `pluginhost`)
- CLI flags: kebab-case (`--pulumi-json`)
- Environment/config keys: uppercase snake (`FINFOCUS_*`)
- Exported types: require Go doc comments

### Documentation

- Files use frontmatter with `title`, `description`, `layout`
- **CRITICAL**: Frontmatter `title` is the H1; content starts with H2
- Run `make docs-lint` after Markdown edits

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):
```text
type(scope): description

feat(cli): add --format flag for output
fix(engine): correct monthly cost calculation
test(registry): add plugin discovery tests
```

## Multi-Repository Ecosystem

FinFocus operates across three repositories:

- **[finfocus](https://github.com/rshade/finfocus)** (this repo) - CLI tool, plugin host
- **[finfocus-spec](https://github.com/rshade/finfocus-spec)** - Protocol definitions, SDK
- **[finfocus-plugin](https://github.com/rshade/finfocus-plugin)** - Plugin implementations

Cross-repo changes require coordination per `.specify/memory/constitution.md`.

## Common Development Tasks

### Adding a New CLI Command

1. Create `internal/cli/your_command.go`
2. Follow constructor pattern with `NewYourCmd() *cobra.Command`
3. Use `RunE` for error handling
4. Add to parent in `root.go`
5. Write tests in `internal/cli/your_command_test.go`

### Adding Resource Types

1. Declare type in `internal/engine/types.go`
2. Implement `Validate()` method
3. Provide pricing in `specs/` or via plugin
4. Create unit tests in `internal/engine/types_test.go`
5. Create integration tests in `internal/conformance/`

### Local Plugin Development

1. Clone plugin repo (e.g., `finfocus-plugin-aws-public`)
2. Modify plugin code
3. Build: `make build-region REGION=us-east-1`
4. Install: Copy binary to `~/.finfocus/plugins/<plugin>/<version>/`
5. Run Core E2E tests to verify

## Important Files & Paths

- `cmd/finfocus/main.go` - CLI entry point
- `internal/engine/engine.go` - Core orchestration
- `internal/pluginhost/host.go` - Plugin client management
- `internal/ingest/pulumi_plan.go` - Pulumi plan parsing
- `.specify/memory/constitution.md` - Project principles
- `examples/plans/` - Sample plans for testing
- `~/.finfocus/config.yaml` - User configuration
- `~/.finfocus/plugins/` - Plugin installation directory

## Configuration

Configuration precedence: CLI flags → Environment variables → Config file → Defaults

Key environment variables:
- `FINFOCUS_LOG_LEVEL` - Log verbosity (debug/info/warn/error)
- `FINFOCUS_LOG_FORMAT` - Log format (json/console)
- `FINFOCUS_TRACE_ID` - External trace ID for distributed tracing
- `FINFOCUS_PLUGIN_*` - Plugin-specific credentials

## Debugging

```bash
# Enable debug output
finfocus --debug cost projected --pulumi-json plan.json

# Or via environment
export FINFOCUS_LOG_LEVEL=debug
export FINFOCUS_LOG_FORMAT=json
```

## CI/CD Pipeline

All PRs must pass:
- Unit tests with race detection
- Code coverage (61% minimum, enforced CI gate)
- golangci-lint v2.6.2
- Security scanning (govulncheck)
- Cross-platform builds (Linux, macOS, Windows)

## Key Dependencies

- `github.com/spf13/cobra v1.10.2` - CLI framework
- `google.golang.org/grpc v1.78.0` - Plugin communication
- `github.com/rs/zerolog v1.34.0` - Structured logging
- `github.com/rshade/finfocus-spec` - Protocol definitions
- `github.com/pulumi/pulumi/sdk/v3 v3.218.0` - Pulumi SDK
- `github.com/charmbracelet/bubbletea v1.3.10` - TUI framework
- `github.com/charmbracelet/lipgloss v1.1.0` - TUI styling

## Feature Development

New features MUST use [SpecKit](https://github.com/github/spec-kit) workflow:
1. Create specification: `/speckit.specify`
2. Clarify ambiguities: `/speckit.clarify`
3. Plan implementation: `/speckit.plan`
4. Generate tasks: `/speckit.tasks`
5. Analyze consistency: `/speckit.analyze`
6. Implement: `/speckit.implement`

Minor bug fixes don't require SpecKit.

## Constitution

The constitution (`.specify/memory/constitution.md`) takes **absolute precedence** over all instructions. Key principles:
- No TODOs or stubs allowed
- Complete implementation required
- High test coverage mandatory
