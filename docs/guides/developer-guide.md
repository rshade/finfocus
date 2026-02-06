---
layout: default
title: Developer Guide
description: Complete guide for engineers - extend FinFocus and build plugins
---

This guide is for **engineers and developers** who want to extend FinFocus by
building plugins or contributing to the core project.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Architecture Overview](#architecture-overview)
3. [Development Setup](#development-setup)
4. [Building Plugins](#building-plugins)
5. [Contributing to Core](#contributing-to-core)
6. [Testing](#testing)
7. [Deployment](#deployment)

---

## Getting Started

### Prerequisites

- Go 1.25.7+ (for core development)
- Git
- Make
- Node.js 18+ (for documentation tools)
- Docker (optional, for containerized testing)

### Quick Setup

```bash
# Clone repository
git clone https://github.com/rshade/finfocus
cd finfocus

# Build
make build

# Test
make test

# Run
./bin/finfocus --help
```

---

## Architecture Overview

### Core Components

```text
┌─────────────────┐
│   Pulumi JSON   │  User's infrastructure definition
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Ingestion     │  Parse resources from Pulumi output
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Engine       │  Orchestrate cost calculation
├─────────────────┤
│  • Resource     │
│    Mapping      │
│  • Cost         │
│    Calculation  │
│  • Aggregation  │
└────────┬────────┘
         │
    ┌────┴─────┐
    ▼          ▼
┌────────┐  ┌──────┐
│Plugins │  │Specs │  Cost sources
└────────┘  └──────┘
    │          │
    └────┬─────┘
         │
         ▼
┌─────────────────┐
│     Output      │  Table, JSON, NDJSON
└─────────────────┘
```

### Key Packages

| Package               | Purpose                        |
| --------------------- | ------------------------------ |
| `internal/cli`        | Command-line interface (Cobra) |
| `internal/engine`     | Core cost calculation logic    |
| `internal/ingest`     | Pulumi plan parsing            |
| `internal/pluginhost` | Plugin gRPC communication      |
| `internal/registry`   | Plugin discovery               |
| `internal/spec`       | Local pricing specifications   |
| `internal/analyzer`   | Pulumi Analyzer gRPC server    |
| `pkg/pluginsdk`       | Plugin SDK for developers      |

### Pulumi Analyzer Integration (Developer Perspective)

The `internal/analyzer` package implements the Pulumi Analyzer gRPC protocol, allowing
FinFocus to act as a "zero-click" cost analysis tool during `pulumi preview`.

Developers extending or debugging the Analyzer should be aware of:

- **gRPC Protocol**: Communication between Pulumi CLI and analyzer occurs via gRPC.
- **Port Handshake**: The analyzer server communicates its dynamic port to the Pulumi
  CLI via stdout. All other logging goes to stderr.
- **Resource Mapping**: The analyzer converts Pulumi resource structures
  (`pulumirpc.AnalyzerResource`) into `engine.ResourceDescriptor` for cost calculation.
- **Diagnostics**: Cost estimates are returned as `ADVISORY` diagnostics.

For a detailed architectural overview of the Analyzer, refer to the [Analyzer Architecture documentation](../architecture/analyzer.md).

---

## Development Setup

### Local Development

```bash
# Install dependencies
go mod download

# Build binary
make build

# Run with example plan
./bin/finfocus cost projected \
  --pulumi-json examples/plans/aws-simple-plan.json

# Run tests
make test

# Run linters
make lint
```

### Documentation Development

```bash
# Install Ruby dependencies
cd docs
bundle install
cd ..

# Serve docs locally
make docs-serve
# Visit http://localhost:4000/finfocus/

# Lint docs
make docs-lint
```

### IDE Setup

**VS Code:**

```json
{
  "go.lintOnSave": "package",
  "go.useLanguageServer": true,
  "[go]": {
    "editor.formatOnSave": true,
    "editor.defaultFormatter": "golang.go"
  }
}
```

---

## Building Plugins

### Quick Plugin Template

Create a new plugin project:

```bash
cd ../
mkdir finfocus-plugin-myservice
cd finfocus-plugin-myservice
go mod init github.com/yourname/finfocus-plugin-myservice
```

### Minimal Plugin

```go
package main

import (
    "context"
    "log"

    pb "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

type MyPlugin struct{}

func (p *MyPlugin) GetProjectedCost(ctx context.Context,
    req *pb.GetProjectedCostRequest) (*pb.GetProjectedCostResponse, error) {

    // Fetch cost from your API
    costs := make([]*pb.Cost, 0)

    for _, resource := range req.Resources {
        cost := &pb.Cost{
            ResourceId: resource.Id,
            TotalCost:  calculateCost(resource),
            Currency:   "USD",
        }
        costs = append(costs, cost)
    }

    return &pb.GetProjectedCostResponse{
        Costs: costs,
    }, nil
}

func calculateCost(resource *pb.Resource) float64 {
    // Your cost calculation logic
    return 0.0
}
```

### Full Plugin Development

See [Plugin Development Guide](../plugins/plugin-development.md) for:

- Complete implementation walkthrough
- gRPC service setup
- Error handling patterns
- Testing strategies
- Deployment instructions

### Example: Vantage Plugin

The Vantage plugin is a complete reference implementation:

```bash
# See implementation at
cat ../finfocus-plugin-vantage/main.go
```

---

## Contributing to Core

### Setting Up Dev Branch

```bash
# Fetch latest
git fetch upstream

# Create feature branch
git checkout -b feature/my-feature upstream/main

# Make changes
# ... edit files ...

# Test changes
make test
make lint
make docs-validate
```

### Code Style

**Go:**

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Run `gofmt` on your code
- Use `golangci-lint` for linting
- Write clear variable names
- Add godoc comments for exported functions

**Example:**

```go
// GetActualCost retrieves actual historical costs for resources.
// It supports filtering by tags and grouping by dimension.
func (e *Engine) GetActualCost(ctx context.Context,
    req *ActualCostRequest) (*ActualCostResponse, error) {
    // Implementation
}
```

**Markdown:**

- Follow [Google style guide](https://developers.google.com/style)
- Use clear headings
- Provide code examples
- Run `make docs-lint` before committing

### Logging Patterns

FinFocus uses zerolog for structured logging with distributed tracing. Follow these patterns:

**Getting a Logger:**

```go
// From context (preferred - includes trace ID)
log := logging.FromContext(ctx)
log.Debug().
    Ctx(ctx).
    Str("component", "engine").
    Str("operation", "get_projected_cost").
    Int("resource_count", len(resources)).
    Msg("starting projected cost calculation")
```

**Component Loggers:**

Each package should identify itself with a component field:

```go
// In CLI package
logger = logging.ComponentLogger(logger, "cli")

// Or inline for context-based logging
log.Info().
    Ctx(ctx).
    Str("component", "engine").
    Msg("operation complete")
```

**Standard Log Fields:**

| Field         | Purpose                             | Example                           |
| ------------- | ----------------------------------- | --------------------------------- |
| `component`   | Package identifier                  | "cli", "engine", "registry"       |
| `operation`   | Current operation                   | "get_projected_cost", "load_plan" |
| `trace_id`    | Request correlation (auto-injected) | "01HQ7X2J3K4M5N6P7Q8R9S0T1U"      |
| `duration_ms` | Operation timing                    | `Dur("duration_ms", elapsed)`     |

**Logging Levels:**

```go
// Trace - Very detailed debugging
log.Trace().Ctx(ctx).Str("component", "engine").Msg("internal detail")

// Debug - Detailed troubleshooting info
log.Debug().Ctx(ctx).Str("component", "engine").Msg("querying plugin")

// Info - Normal operations
log.Info().Ctx(ctx).Str("component", "engine").Msg("calculation complete")

// Warn - Something unexpected but recoverable
log.Warn().Ctx(ctx).Str("component", "engine").Err(err).Msg("plugin timeout, using fallback")

// Error - Something failed
log.Error().Ctx(ctx).Str("component", "engine").Err(err).Msg("calculation failed")
```

**Sensitive Data Protection:**

```go
// Use SafeStr for potentially sensitive key-value pairs
logging.SafeStr(event, "api_key", apiKey)  // Automatically redacts sensitive keys
```

**Trace ID Management:**

```go
// Generate trace ID at entry point
traceID := logging.GetOrGenerateTraceID(ctx)
ctx = logging.ContextWithTraceID(ctx, traceID)

// TracingHook automatically injects trace_id into all log entries
// when using .Ctx(ctx)
```

### Commit Messages

```text
type: Brief description

More detailed explanation of changes.
- What changed
- Why it changed
- Any implementation notes

Closes #123
```

**Types:**

- `feature` - New functionality
- `fix` - Bug fixes
- `docs` - Documentation
- `test` - Tests
- `refactor` - Code restructuring
- `perf` - Performance improvements

### Testing Your Changes

```bash
# Run all tests
make test

# Run specific package
go test -v ./internal/engine/...

# Run with coverage
go test -cover ./...

# Run specific test
go test -run TestActualCost ./internal/engine/...
```

### Pull Request Process

1. **Update from main:**

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all checks:**

   ```bash
   make test
   make lint
   make validate
   make docs-validate
   ```

3. **Push and create PR:**

   ```bash
   git push origin feature/my-feature
   # Create PR on GitHub
   ```

4. **Address feedback:**
   - Respond to comments
   - Push additional commits
   - Rebase if requested

---

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run with coverage
go test -cover ./...

# View coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Structure

```go
func TestGetActualCost(t *testing.T) {
    // Arrange - Set up test data
    request := &ActualCostRequest{
        StartDate: "2024-01-01",
        EndDate:   "2024-01-31",
    }

    // Act - Execute function
    response, err := engine.GetActualCost(context.Background(), request)

    // Assert - Verify results
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if response == nil {
        t.Fatal("expected response, got nil")
    }
}
```

### Integration Testing

For testing with real plugins:

```bash
# Ensure plugins are installed
./bin/finfocus plugin list

# Test with example plan
./bin/finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json
```

### Plugin Certification

Before releasing a plugin, run the certification suite to ensure full protocol compliance:

```bash
finfocus plugin certify ./path/to/your-plugin
```

#### Analyzer Integration Testing

Testing the Analyzer involves running `pulumi preview` against a Pulumi project
configured to use the `finfocus analyzer serve` command.

```bash
# Example: Configure your Pulumi.yaml as described in the Analyzer Setup guide.
# Then, navigate to your Pulumi project directory:
cd your-pulumi-project
pulumi preview
```

Verify the output for cost diagnostics. For detailed debugging, enable verbose logging:

```bash
FINFOCUS_LOG_LEVEL=debug pulumi preview
```

#### Cross-Provider Aggregation Testing

Test cross-provider aggregation by running `finfocus cost actual` with `--group-by daily`
or `--group-by monthly` on a Pulumi plan that includes resources from multiple providers.

```bash
# Example: Daily aggregation
finfocus cost actual --pulumi-json examples/plans/multi-provider-plan.json \
  --from 2024-01-01 --to 2024-01-31 --group-by daily

# Example: Monthly aggregation with JSON output
finfocus cost actual --pulumi-json examples/plans/multi-provider-plan.json \
  --from 2024-01-01 --group-by monthly --output json
```

### Fuzz Testing

FinFocus uses Go's native fuzzing (Go 1.25+) for parser resilience testing:

```bash
# JSON parser fuzzing
go test -fuzz=FuzzJSON$ -fuzztime=30s ./internal/ingest

# YAML parser fuzzing
go test -fuzz=FuzzYAML$ -fuzztime=30s ./internal/spec

# Full plan parsing fuzzing
go test -fuzz=FuzzPulumiPlanParse$ -fuzztime=30s ./internal/ingest
```

**Fuzz test files:**

| Location                       | Purpose                |
| ------------------------------ | ---------------------- |
| `internal/ingest/fuzz_test.go` | JSON parser fuzz tests |
| `internal/spec/fuzz_test.go`   | YAML spec fuzz tests   |

**Adding seed corpus:**

Place interesting inputs in `testdata/fuzz/<TestName>/` directories:

```text
internal/ingest/testdata/fuzz/FuzzJSON/
├── valid_plan.json
├── edge_case_unicode.json
└── malformed_input.json
```

### Performance Benchmarks

Benchmarks test scalability with synthetic data:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./test/benchmarks/...

# Run scale benchmarks only
go test -bench=BenchmarkScale -benchmem ./test/benchmarks/...

# Run with specific iterations
go test -bench=BenchmarkScale1K -benchtime=10x -benchmem ./test/benchmarks/...
```

**Benchmark test files:**

| Location                        | Purpose                  |
| ------------------------------- | ------------------------ |
| `test/benchmarks/scale_test.go` | Scale tests (1K-100K)    |
| `test/benchmarks/generator/`    | Synthetic data generator |

**Performance targets:**

| Scale | Target Time  | Actual (baseline) |
| ----- | ------------ | ----------------- |
| 1K    | < 1 second   | ~13ms             |
| 10K   | < 30 seconds | ~167ms            |
| 100K  | < 5 minutes  | ~2.3s             |

### Synthetic Data Generator

The benchmark generator creates realistic infrastructure plans:

```go
import "github.com/rshade/finfocus/test/benchmarks/generator"

// Use preset configurations
plan, err := generator.GeneratePlan(generator.PresetSmall)   // 1K resources
plan, err := generator.GeneratePlan(generator.PresetMedium)  // 10K resources
plan, err := generator.GeneratePlan(generator.PresetLarge)   // 100K resources

// Custom configuration
config := generator.BenchmarkConfig{
    ResourceCount:   5000,
    MaxDepth:        5,
    DependencyRatio: 0.3,
    Seed:            42,  // Deterministic generation
}
plan, err := generator.GeneratePlan(config)
```

**Generator features:**

- Deterministic output with seed values
- Configurable resource count and nesting depth
- Realistic resource types (AWS, Azure, GCP)
- Dependency graph generation
- JSON export for external tooling

---

## Deployment

### Building Releases

```bash
# Create version tag
git tag v0.1.0
git push origin v0.1.0

# GitHub Actions automatically:
# 1. Builds cross-platform binaries
# 2. Creates release
# 3. Uploads checksums
```

### Plugin Installation

Users install plugins to: `~/.finfocus/plugins/<name>/<version>/`

**Structure:**

```text
~/.finfocus/plugins/
├── myplugin/
│   └── 0.1.0/
│       ├── finfocus-myplugin    # Plugin binary
│       └── plugin.manifest.json    # Metadata
```

### Docker Deployment

```dockerfile
FROM golang:1.25.7 as builder
WORKDIR /app
COPY . .
RUN make build

FROM alpine:latest
COPY --from=builder /app/bin/finfocus /usr/local/bin/
ENTRYPOINT ["finfocus"]
```

---

## Useful Commands

```bash
# Development
make build              # Build binary
make test               # Run tests
make lint               # Code linting
make validate           # Validation
make clean              # Clean artifacts

# Documentation
make docs-lint          # Lint docs
make docs-serve         # Serve locally
make docs-build         # Build site
make docs-validate      # Validate structure

# Git
git fetch upstream      # Get latest changes
git rebase upstream/main # Rebase on main
git push origin branch  # Push changes
```

---

## Resources

- **Plugin Development:** [Plugin Development Guide](../plugins/plugin-development.md)
- **Plugin SDK:** [Plugin SDK Reference](../plugins/plugin-sdk.md)
- **Examples:** [Code Examples](../plugins/plugin-examples.md)
- **Architecture:** [System Architecture](../architecture/system-overview.md)
- **Contributing:** [Contributing Guide](../../CONTRIBUTING.md)
- **Vantage Plugin:** [Vantage Implementation Example](../plugins/vantage/README.md)

---

**Last Updated:** 2025-10-29
