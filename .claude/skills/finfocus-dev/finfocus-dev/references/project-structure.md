# Project Structure

## Table of Contents

- [Directory Layout](#directory-layout)
- [Internal Packages](#internal-packages)
- [Key Files](#key-files)
- [Test Structure](#test-structure)
- [Dependencies](#dependencies)

## Directory Layout

```text
finfocus/
├── cmd/finfocus/main.go          # CLI entry point
├── internal/
│   ├── analyzer/                 # Pulumi Analyzer gRPC server
│   ├── awsutil/                  # AWS utilities (ARN parsing)
│   ├── cli/                      # Cobra commands (70 files)
│   ├── config/                   # Configuration management
│   ├── conformance/              # Plugin conformance testing
│   ├── constants/                # Shared constants
│   ├── engine/                   # Cost calculation orchestration (40+ files)
│   │   ├── batch/                # Batch processing engine
│   │   └── cache/                # File-based cache store
│   ├── greenops/                 # Green computing operations
│   ├── ingest/                   # Pulumi plan parsing
│   ├── logging/                  # Structured logging setup
│   ├── migration/                # Database migrations
│   ├── pluginhost/               # gRPC plugin lifecycle management
│   ├── proto/                    # Protocol buffer adapters
│   ├── pulumi/                   # Pulumi CLI integration
│   ├── registry/                 # Plugin discovery and management
│   ├── router/                   # Request routing
│   ├── skus/                     # SKU management
│   ├── spec/                     # Local pricing specifications
│   ├── specvalidate/             # Specification validation
│   └── tui/                      # Terminal UI components (Bubble Tea)
├── plugins/recorder/             # Reference plugin implementation
├── test/
│   ├── unit/                     # Unit tests by package
│   ├── integration/              # Cross-component tests
│   ├── e2e/                      # End-to-end tests (separate module)
│   ├── fixtures/                 # Test data (plans, specs, configs)
│   ├── mocks/                    # Mock implementations
│   └── benchmarks/               # Performance tests
├── docs/                         # Jekyll documentation site
├── examples/plans/               # Sample Pulumi plans
└── .claude/                      # Claude Code configuration
    ├── agents/                   # Custom agent definitions
    ├── commands/                 # Slash commands (speckit, code-review)
    └── skills/                   # Skills (this directory)
```

## Internal Packages

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `cli` | Cobra CLI commands | `NewXxxCmd()` constructors |
| `engine` | Cost calculation core | `Engine`, `CostResult`, `ResourceDescriptor` |
| `pluginhost` | gRPC plugin lifecycle | `Client`, `ProcessLauncher`, `StdioLauncher` |
| `registry` | Plugin discovery | `Registry`, `PluginInfo` |
| `ingest` | Pulumi JSON parsing | `LoadPulumiPlan`, `MapResources` |
| `analyzer` | Pulumi Analyzer protocol | `Server` (AnalyzerServer impl) |
| `proto` | Proto adapters | `ActionTypeLabel`, `ParseActionType` |
| `config` | Configuration paths | `Config` (~/.finfocus/) |
| `tui` | Terminal UI components | Bubble Tea models, Lip Gloss styles |
| `logging` | Zerolog setup | `FromContext`, `ComponentLogger` |

## Key Files

- `cmd/finfocus/main.go` - Entry point
- `internal/engine/engine.go` - Core orchestration (104KB)
- `internal/cli/root.go` - Root command and global flags
- `internal/cli/cost_projected.go` - Projected cost command
- `internal/cli/cost_actual.go` - Actual cost command
- `internal/cli/cost_recommendations.go` - Recommendations command
- `internal/pluginhost/host.go` - Plugin client management
- `internal/ingest/pulumi_plan.go` - Pulumi plan parsing
- `internal/analyzer/server.go` - Pulumi Analyzer server
- `examples/plans/aws-simple-plan.json` - Sample plan for testing
- `.specify/memory/constitution.md` - Project principles

## Test Structure

```bash
# Unit tests (fast, for CI/dev)
go test ./internal/cli/...
go test ./internal/engine/...
go test ./plugins/recorder/...

# Specific test
go test -run TestSpecificFunction ./...

# Integration tests (10min timeout)
go test ./test/integration/...

# E2E tests (requires AWS credentials)
make test-e2e
```

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `spf13/cobra` | v1.10.2 | CLI framework |
| `finfocus-spec` | v0.5.6 | Protocol definitions + pluginsdk |
| `pulumi/sdk/v3` | v3.219.0 | Pulumi SDK for Analyzer |
| `google.golang.org/grpc` | v1.78.0 | Plugin communication |
| `rs/zerolog` | v1.34.0 | Structured logging |
| `charmbracelet/bubbletea` | v1.3.10 | TUI framework |
| `charmbracelet/lipgloss` | v1.1.0 | TUI styling |
| `stretchr/testify` | v1.11.1 | Test assertions |

Go version: **1.25.7** (see `go.mod`)
