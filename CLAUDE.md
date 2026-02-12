# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## CRITICAL INSTRUCTIONS

**DO NOT RUN `git commit`** - This is explicitly forbidden. Use `git add`, `git status`, `git diff`, and `git log` only. The user will commit manually.

**ALWAYS run `make lint` and `make test`** before claiming success.

**DO NOT modify `.golangci.yml`** without explicit approval.

## Project Overview

FinFocus Core is a CLI tool and plugin host system for calculating cloud infrastructure costs from Pulumi infrastructure definitions. It provides both projected cost estimates and actual historical cost analysis through a plugin-based architecture.

## Build Commands

```bash
make build         # Build binary to bin/finfocus
make test          # Run unit tests (default, fast)
make test-race     # Run with race detector
make test-integration  # Integration tests (slower)
make test-e2e      # E2E tests (requires AWS credentials)
make lint          # Run golangci-lint + markdownlint
make validate      # go mod tidy, go vet
make clean         # Remove build artifacts
make run           # Build and run with --help
make dev           # Build and run without args
```

### Single Package/Test Commands

```bash
go test -v ./internal/cli/...           # Test specific package
go test -v ./internal/engine/...        # Test engine package
go test -run TestSpecificFunction ./... # Run specific test

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out        # View in browser

# Plugin management
./bin/finfocus plugin list
./bin/finfocus plugin validate
```

### Test Requirements

- **Unit tests**: Must achieve 80% coverage minimum
- **Critical paths**: Must achieve 95% coverage
- **All error paths**: Must be tested
- **Performance regressions**: Must be detected via benchmarks
- **Integration scenarios**: Must include plugin communication flows
- **End-to-end workflows**: Must test complete CLI usage

### CI/CD Integration

The existing CI/CD pipeline automatically runs all tests including the new framework:

- Unit tests with coverage reporting
- Integration tests with timeout handling
- Linting and security scanning
- Cross-platform build verification

**Never complete a project without running:**

```bash
make test    # Run all tests
make lint    # Run linting
```

## Go Version

**Project Go Version**: 1.25.7 (see `go.mod`)

**CRITICAL**: Before claiming any Go version "doesn't exist" or suggesting version
changes, verify on <https://go.dev/dl/> first.

### Constitution Precedence Rule

**CRITICAL**: The constitution (`.specify/memory/constitution.md`) takes **absolute
precedence** over all runtime mode instructions (learning mode, explanatory mode, etc.).

If any runtime instruction conflicts with a constitution principle:

1. **Constitution wins** - Follow the constitution rule
2. **Use `/speckit.revisit`** - Document the conflict for prevention
3. **Never compromise** - Principle VI forbids TODOs/stubs regardless of mode

Common conflicts to watch for:

- Learning mode may suggest "mark with TODO" → Constitution forbids TODOs
- Explanatory mode may suggest placeholders → Constitution forbids stubs
- Any mode suggesting deferred implementation → Constitution requires completeness

## Architecture

### Core Components

1. **CLI Layer** (`internal/cli/`) - Cobra-based commands:
   - `cost projected` - Estimate costs from Pulumi preview JSON
   - `cost actual` - Fetch historical costs with time ranges/grouping
   - `cost recommendations` - Get cost optimization recommendations with action type filtering
   - `plugin list/validate/install/remove/update/certify` - Plugin management
   - `analyzer serve` - Pulumi Analyzer gRPC server for zero-click cost estimation

2. **Engine** (`internal/engine/`) - Core cost calculation:
   - Orchestrates between plugins and local specs
   - Output formats: table, JSON, NDJSON
   - Cross-provider aggregation with time-based grouping
   - `hoursPerMonth = 730` for monthly calculations

3. **Plugin Host** (`internal/pluginhost/`) - gRPC plugin management:
   - `ProcessLauncher` (TCP) and `StdioLauncher` (stdin/stdout)
   - 10-second timeout with 100ms retry delays
   - Always call `cmd.Wait()` after `Kill()` to prevent zombies

4. **Registry** (`internal/registry/`) - Plugin discovery:
   - Scans `~/.finfocus/plugins/<name>/<version>/`
   - Optional `plugin.manifest.json` validation

5. **Ingestion** (`internal/ingest/`) - Pulumi plan parsing:
   - Converts `pulumi preview --json` to resource descriptors
   - **CRITICAL**: Must inspect `newState` to extract `Inputs` correctly

6. **Analyzer** (`internal/analyzer/`) - Pulumi Analyzer protocol:
   - Implements `pulumirpc.AnalyzerServer` for zero-click cost estimation
   - ADVISORY enforcement (never blocks deployments)
   - Prints ONLY port number to stdout (Pulumi handshake protocol)

7. **TUI** (`internal/tui/`) - Shared Terminal UI components:
   - Built on Bubble Tea and Lip Gloss
   - Adaptive color schemes (light/dark terminal detection)
   - Reusable progress indicators, styled text, and tables

### Data Flow

```text
Pulumi JSON → Ingestion → Resource Descriptors → Engine
                                                    ↓
                                        Plugins (gRPC) or Local Specs
                                                    ↓
                                              Cost Results → Output Rendering
```

### Plugin Communication

Plugins communicate via gRPC using protocol buffers from `finfocus-spec`:

- `Name()` - Plugin identification

### Action Type Utilities (`internal/proto/action_types.go`)

Shared utilities for recommendation action type handling:

- `ActionTypeLabel(at pbc.RecommendationActionType) string` - Human-readable label for display
- `ActionTypeLabelFromString(actionType string) string` - Label from string (for stored values)
- `ParseActionType(s string) (pbc.RecommendationActionType, error)` - Parse single action type
- `ParseActionTypeFilter(filter string) ([]pbc.RecommendationActionType, error)` - Parse comma-separated filter
- `ValidActionTypes() []string` - List of valid action type names (excludes UNSPECIFIED)
- `MatchesActionType(recType string, types []pbc.RecommendationActionType) bool` - Filter matching

Valid action types: RIGHTSIZE, TERMINATE, PURCHASE_COMMITMENT, ADJUST_REQUESTS, MODIFY,
DELETE_UNUSED, MIGRATE, CONSOLIDATE, SCHEDULE, REFACTOR, INVESTIGATE, OTHER

## Documentation

All documentation lives in `docs/` with GitHub Pages deployment.

### Commands

```bash
make docs-lint     # Lint markdown
make docs-build    # Build Jekyll site
make docs-serve    # Serve at http://localhost:4000/finfocus/
make docs-validate # Validate structure
```

### Structure

```text
docs/
├── guides/           # Audience-specific (User, Developer, Architect, Business)
├── getting-started/  # Quickstart, installation, examples
├── architecture/     # System design, core concepts, roadmap
├── plugins/          # Plugin development, SDK, per-plugin docs
├── reference/        # CLI, API, configuration, error codes
├── deployment/       # Installation, Docker, CI/CD, security
└── support/          # FAQ, troubleshooting, contributing
```

### Key Files

- `docs/README.md` - Documentation hub with navigation
- `docs/llms.txt` - Machine-readable index for AI tools
- `docs/_config.yml` - Jekyll configuration

## Key Patterns

### CLI Package (`internal/cli/`)

- Use `RunE` not `Run` for error handling
- Use `cmd.Printf()` for output (not `fmt.Printf()`)
- Defer cleanup functions immediately after obtaining resources
- Support multiple date formats: "2006-01-02", RFC3339

### Pre-Flight Request Validation (`internal/proto/`)

The adapter layer validates requests using `pluginsdk` validation functions
before making gRPC calls to plugins. This catches malformed requests early
with actionable error messages.

**Validation Pattern**:

```go
// Pre-flight validation: construct proto request and validate before gRPC call
protoReq := &pbc.GetProjectedCostRequest{
    Resource: &pbc.ResourceDescriptor{
        Provider:     resource.Provider,
        ResourceType: resource.Type,
        Sku:          sku,
        Region:       region,
    },
}

if err := pluginsdk.ValidateProjectedCostRequest(protoReq); err != nil {
    log := logging.FromContext(ctx)
    log.Warn().
        Str("resource_type", resource.Type).
        Err(err).
        Msg("pre-flight validation failed")

    // Return placeholder result with VALIDATION: prefix
    result.Results = append(result.Results, &CostResult{
        Currency:    "USD",
        MonthlyCost: 0,
        Notes:       fmt.Sprintf("VALIDATION: %v", err),
    })
    continue  // Skip plugin call for this resource
}
```

**Key Points**:

- Validation happens in `GetProjectedCostWithErrors()` and `GetActualCostWithErrors()`
- Uses "VALIDATION:" prefix to distinguish from plugin errors ("ERROR:")
- Logs at WARN level with resource context for debugging
- Returns placeholder CostResult with $0 cost and descriptive Notes
- Invalid resources are skipped; valid resources still call the plugin

### Logging (Zerolog)

FinFocus uses zerolog for structured logging with distributed tracing support.

#### Enabling Debug Output

```bash
# CLI flag
finfocus cost projected --debug --pulumi-json plan.json

# Environment variable
export FINFOCUS_LOG_LEVEL=debug
export FINFOCUS_LOG_FORMAT=json    # json or console
export FINFOCUS_TRACE_ID=external-trace-123  # inject external trace ID
```

#### Configuration Precedence

1. CLI flags (`--debug`)
2. Environment variables (`FINFOCUS_LOG_LEVEL`)
3. Config file (`~/.finfocus/config.yaml`)
4. Default (info level, console format)

#### Logging Patterns for Developers

```go
// Get logger from context (preferred - includes trace_id)
log := logging.FromContext(ctx)
log.Debug().
    Ctx(ctx).
    Str("component", "engine").
    Str("operation", "get_projected_cost").
    Int("resource_count", len(resources)).
    Msg("starting projected cost calculation")

// Create component sub-logger
logger = logging.ComponentLogger(logger, "registry")

// Log with duration
start := time.Now()
// ... operation ...
log.Info().
    Ctx(ctx).
    Dur("duration_ms", time.Since(start)).
    Msg("operation complete")
```

#### Standard Log Fields

| Field         | Purpose             | Example                      |
| ------------- | ------------------- | ---------------------------- |
| `trace_id`    | Request correlation | "01HQ7X2J3K4M5N6P7Q8R9S0T1U" |
| `component`   | Package identifier  | "cli", "engine", "registry"  |
| `operation`   | Current operation   | "get_projected_cost"         |
| `duration_ms` | Operation timing    | 245                          |

#### Log Levels

- **TRACE**: Property extraction, detailed calculations
- **DEBUG**: Function entry/exit, retries, intermediate values
- **INFO**: High-level operations (command start/end)
- **WARN**: Recoverable issues (fallbacks, deprecations)
- **ERROR**: Failures needing attention

#### Trace ID Management

```go
// Generate trace ID at entry point (usually in CLI PersistentPreRunE)
traceID := logging.GetOrGenerateTraceID(ctx)
ctx = logging.ContextWithTraceID(ctx, traceID)

// TracingHook automatically injects trace_id when using .Ctx(ctx)
```

## CI/CD Pipeline

Complete CI/CD pipeline setup with GitHub Actions for automated testing, building, and release management.

### CI Pipeline (.github/workflows/ci.yml)

Triggered on pull requests and pushes to main branch:

**Test Job:**

- Go 1.25.7 setup with caching
- Unit tests with race detection and coverage reporting
- Coverage threshold check (minimum 61%)
- Artifacts uploaded for coverage reports

**Lint Job:**

- golangci-lint with project-specific configuration
- Security scanning with gosec included
- Timeout set to 5 minutes

**Security Job:**

- govulncheck for dependency vulnerability scanning
- Checks for known vulnerabilities in Go dependencies

**Validation Job:**

- gofmt formatting checks
- go mod tidy verification
- go vet static analysis

**Build Job:**

- Cross-platform builds (Linux, macOS, Windows)
- Support for amd64 and arm64 architectures
- Build artifacts uploaded with proper naming

### Release Pipeline (.github/workflows/release.yml)

Triggered on version tags (v\*):

**Multi-Platform Binaries:**

- Linux: amd64, arm64
- macOS: amd64, arm64
- Windows: amd64
- Naming convention: `finfocus-v{version}-{os}-{arch}`

**Release Features:**

- Automatic changelog generation from git history
- SHA256 checksums for all binaries
- GitHub Release creation with proper metadata
- Asset upload with verification instructions
- Pre-release detection for tags containing hyphens

### Quality Gates

**Code Quality:**

- golangci-lint with essential linters (errcheck, govet, staticcheck, gosec, etc.)
- Security scanning integrated into CI pipeline
- Formatting and import organization enforced

**Coverage Requirements:**

- Minimum 20% code coverage (adjustable as project matures)
- Coverage reports generated and uploaded as artifacts
- Automatic threshold validation in CI

**Build Verification:**

- Cross-platform compilation verification
- Binary naming consistency
- Version information embedded in binaries

### Troubleshooting Commands

```bash
# Fix parallel linting conflicts
pkill golangci-lint || true

# Check coverage details
go tool cover -html=coverage.out

# Test release build locally
GOOS=linux GOARCH=amd64 make build

# Validate workflow syntax
gh workflow validate .github/workflows/ci.yml
```

## Testing

### Test Directory Structure

```text
test/
├── unit/           # Unit tests by package (engine, config, spec)
├── integration/    # Cross-component tests (plugin communication)
├── e2e/            # End-to-end tests (separate Go module)
│   ├── fixtures/   # Pulumi project fixtures for E2E tests
│   └── ...
├── fixtures/       # General test data (plans, specs, configs, responses)
├── mocks/          # Mock implementations (plugin server)
└── benchmarks/     # Performance tests
```

### E2E Testing

**Location**: `test/e2e/` (separate Go module)

**Project Fixtures**: `test/e2e/fixtures/` (Real Pulumi projects)

**Prerequisites**: AWS session or profile configured, Pulumi CLI, `make build`

```bash
export PATH="$HOME/.pulumi/bin:$PATH"
export PULUMI_CONFIG_PASSPHRASE="e2e-test-passphrase"
make test-e2e
```

**CRITICAL**: E2E tests MUST call actual finfocus CLI binary.
Never simulate cost values or stub CLI execution.

**Actual cost integration coverage**: `test/e2e/actual_cost_test.go` (build tag `integration`) runs `finfocus cost actual` against static Pulumi state fixtures and validates JSON output structure and edge cases.

### Expected Failure Test Patterns

**IMPORTANT**: Tests that intentionally create failing plugin scenarios (e.g., mock
plugins that exit immediately, nonexistent binaries, timeout scenarios) must follow
these patterns to avoid false CI failures:

**Pattern for Expected Errors**:

```go
// CORRECT: Use t.Logf() for expected failures - test passes
client, err := pluginhost.NewClient(ctx, launcher, mockPlugin)
if client != nil {
    client.Close()
}
if err != nil {
    t.Logf("Expected failure (handled): %v", err)  // Informational, not a failure
}

// INCORRECT: Using t.Errorf() would cause CI to fail
if err != nil {
    t.Errorf("Failed: %v", err)  // DON'T DO THIS for expected errors
}
```

**Pattern for Required Errors**:

```go
// When an error MUST occur for the test to pass
_, err := launcher.Start(ctx, "/nonexistent/command")
if err == nil {
    t.Fatalf("expected error for invalid command")  // Only fail if NO error
}
if !strings.Contains(err.Error(), "starting plugin") {
    t.Errorf("unexpected error: %v", err)  // Fail for WRONG error type
}
```

**Test Types That Use Expected Failures**:

- `TestIntegration_ProcessLauncherWithClient` - Tests mock plugin startup
- `TestIntegration_StdioLauncherWithClient` - Tests stdio communication
- `TestIntegration_ConcurrentClients` - Concurrent mock plugin handling
- `TestIntegration_RapidCreateDestroy` - Rapid teardown scenarios
- `TestIntegration_ErrorRecovery` - Various error conditions

**Common Expected Error Types**:

- `context deadline exceeded` - Timeout waiting for plugin
- `connection refused` - Plugin not listening on port
- `broken pipe` / `EOF` - Plugin crashed or disconnected
- `no such file or directory` - Plugin binary not found

**CI Troubleshooting**:

If CI shows these errors in logs but tests are marked PASS, the behavior is correct.
Tests log these messages with `t.Logf()` for debugging visibility while passing.
Only investigate if tests actually FAIL (exit code 1) with these messages.

### Error Path Testing Guidelines

**When writing new code, always include tests for error conditions:**

1. **Test every error return**: If a function can return an error, write a test that
   triggers that error path
2. **Validate error messages**: Use `assert.Contains(t, err.Error(), "expected text")`
   to ensure errors are descriptive and actionable
3. **Test boundary conditions**: Empty inputs, nil pointers, invalid ranges, malformed data
4. **Test partial failures**: What happens when one item in a batch fails?
5. **Test resource cleanup**: Verify cleanup runs even when errors occur (defer patterns)

**Table-driven error tests pattern**:

```go
func TestFunction_Errors(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        wantErr     bool
        errContains string
    }{
        {"empty input", "", true, "input required"},
        {"invalid format", "bad", true, "invalid format"},
        {"valid input", "good", false, ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Function(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errContains)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

**Priority error paths to test**:

- File I/O errors (missing files, permission denied, disk full)
- Network errors (connection refused, timeout, DNS failure)
- Validation errors (invalid input, out of range, type mismatch)
- Resource exhaustion (memory, file handles, goroutines)
- Concurrent access errors (race conditions, deadlocks)

### Testify Assertion Standards

**CRITICAL**: All Go tests MUST use testify's `require` and `assert` packages.
NEVER use manual `if x != y { t.Errorf(...) }` patterns.

**Required Imports**:

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**When to Use `require.*` (stops test on failure)**:

- Setup operations that must succeed for test to be valid
- Error checks where continuing would cause panics or misleading failures
- Non-nil checks for required objects before using them
- File/directory creation in test setup

```go
// Setup must succeed - use require
require.NoError(t, os.MkdirAll(dir, 0755))
require.NoError(t, os.WriteFile(path, data, 0644))

// Must have result before checking properties
result, err := SomeFunction()
require.NoError(t, err)
require.NotNil(t, result)
```

**When to Use `assert.*` (continues test on failure)**:

- Value comparisons after setup is complete
- Multiple property checks on a result
- Non-critical validations where seeing all failures is helpful

```go
// Value assertions - use assert
assert.Equal(t, "expected", result.Name)
assert.Len(t, result.Items, 3)
assert.Contains(t, result.Message, "success")
assert.True(t, result.IsValid)
```

**Common Assertion Conversions**:

| Manual Pattern                                   | Testify Replacement           |
| ------------------------------------------------ | ----------------------------- |
| `if err != nil { t.Fatal(err) }`                 | `require.NoError(t, err)`     |
| `if err == nil { t.Error("expected error") }`    | `require.Error(t, err)`       |
| `if x != y { t.Errorf("got %v, want %v", x, y) }`| `assert.Equal(t, y, x)`       |
| `if len(x) != n { t.Errorf(...) }`               | `assert.Len(t, x, n)`         |
| `if !strings.Contains(s, sub) { t.Errorf(...) }` | `assert.Contains(t, s, sub)`  |
| `if x == nil { t.Fatal("nil") }`                 | `require.NotNil(t, x)`        |

**Error Message Validation**:

```go
// Check error exists AND contains expected message
err := SomeFunction(invalidInput)
require.Error(t, err)
assert.Contains(t, err.Error(), "invalid input")
```

### Local Plugin Development

To debug plugin issues during Core development:

1. Clone the plugin repository (e.g., `finfocus-plugin-aws-public`)
2. Modify the plugin code (add logging, fix type mapping)
3. Build: `make build-region REGION=us-east-1`
4. Install: Copy binary to `~/.finfocus/plugins/<plugin>/<version>/`
5. Run Core E2E tests to verify

## Important Files

- `cmd/finfocus/main.go` - CLI entry point
- `internal/engine/engine.go` - Core orchestration
- `internal/pluginhost/host.go` - Plugin client management
- `internal/ingest/pulumi_plan.go` - Pulumi plan parsing
- `.specify/memory/constitution.md` - Project principles and quality gates
- `examples/plans/aws-simple-plan.json` - Sample plan for testing

## Pulumi Integration Notes

### Plan JSON Parsing

The `pulumi preview --json` output nests resource details under `newState`.
Ingestion MUST inspect `newState` to extract `inputs` and `type`. Without this,
property extraction fails and plugins return `InvalidArgument` errors.

### Property Extraction

The adapter (`internal/proto/adapter.go`) relies on the `Inputs` map to extract:

- **SKU**: from `instanceType`, `type`, etc.
- **Region**: from `availabilityZone`, `region`

If ingestion fails to populate `Inputs`, these fields are empty.

### Resource Type Compatibility

Pulumi provides types like `aws:ec2/instance:Instance` (Type Token). Plugins may
expect `aws:ec2:Instance` or just `ec2`. Plugins should handle the standard
Pulumi format or normalize internally.

### Pulumi SDK Import Path

For Analyzer development, use the correct import:

```go
pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
// NOT: github.com/pulumi/pulumi/sdk/v3/proto/go/pulumirpc
```

## Multi-Repository Ecosystem

FinFocus operates across three repositories:

- **finfocus** (this repo) - CLI tool, plugin host, orchestration
- **finfocus-spec** - Protocol buffer definitions, SDK generation
- **finfocus-plugin** - Plugin implementations (Kubecost, Vantage, etc.)

Cross-repo changes follow the protocol in `.specify/memory/constitution.md`.

## Dependencies

Key dependencies (see `go.mod` for versions):

- `github.com/spf13/cobra` - CLI framework
- `google.golang.org/grpc` - Plugin communication
- `github.com/rs/zerolog` - Structured logging
- `github.com/rshade/finfocus-spec` - Protocol definitions and pluginsdk
- `github.com/pulumi/pulumi/sdk/v3` - Pulumi SDK for Analyzer (v3.210.0+)
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - TUI styling
- `golang.org/x/term` - Terminal detection utilities

## Environment Variable Constants (pluginsdk)

FinFocus uses standardized environment variable constants from `pluginsdk` for
consistency between core and plugins.

### Available Constants

```go
import "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

// Plugin communication
pluginsdk.EnvPort        // "FINFOCUS_PLUGIN_PORT" - Primary port for gRPC (Note: "PORT" is NOT set)

// Logging configuration
pluginsdk.EnvLogLevel    // "FINFOCUS_LOG_LEVEL" - Log verbosity
pluginsdk.EnvLogFormat   // "FINFOCUS_LOG_FORMAT" - Log format (json/text)
pluginsdk.EnvLogFile     // "FINFOCUS_LOG_FILE" - Log file path

// Distributed tracing
pluginsdk.EnvTraceID     // "FINFOCUS_TRACE_ID" - Trace ID for request correlation
```

### Usage Patterns

**Setting environment variables (core → plugin):**

```go
cmd.Env = append(os.Environ(),
    fmt.Sprintf("%s=%d", pluginsdk.EnvPort, port),
)
// Note: PORT is NOT set (issue #232) - plugins use --port flag or pluginsdk.GetPort()
```

**Reading environment variables (in tests):**

```go
os.Setenv(pluginsdk.EnvLogLevel, "debug")
defer os.Unsetenv(pluginsdk.EnvLogLevel)
```

**Note**: Config-specific variables like `FINFOCUS_OUTPUT_FORMAT` and
`FINFOCUS_CONFIG_STRICT` are NOT in pluginsdk as they are core-specific.

## gRPC Metadata Keys (pluginsdk)

FinFocus uses standardized gRPC metadata keys for distributed tracing across
plugin boundaries.

### Available Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `pluginsdk.TraceIDMetadataKey` | `"x-finfocus-trace-id"` | gRPC metadata key for trace ID propagation |

### Trace ID Propagation Flow

1. **Core (client-side)**: `TraceInterceptor()` in `internal/pluginhost/grpc.go`
   injects the trace ID from context into outgoing gRPC metadata
2. **Plugins (server-side)**: `TracingUnaryServerInterceptor()` from pluginsdk
   extracts the trace ID from incoming metadata and adds it to the context

### Usage in Core

```go
import "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

// TraceInterceptor automatically uses pluginsdk.TraceIDMetadataKey
// to inject trace IDs into outgoing gRPC calls
conn, err := grpc.DialContext(ctx, addr,
    grpc.WithUnaryInterceptor(pluginhost.TraceInterceptor()),
)
```

### Usage in Plugins

```go
import "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

// TracingUnaryServerInterceptor extracts trace ID from metadata
server := grpc.NewServer(
    grpc.UnaryInterceptor(pluginsdk.TracingUnaryServerInterceptor()),
)

// In handlers, get trace ID from context
func (s *Server) GetProjectedCost(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    traceID := pluginsdk.TraceIDFromContext(ctx)
    // Use traceID for logging correlation
}
```

## Package-Specific Documentation

### internal/cli

The CLI package implements the Cobra-based command-line interface. Key patterns:

- Use `RunE` not `Run` for error handling
- Always use `cmd.Printf()` for output (not `fmt.Printf()`)
- Defer cleanup functions immediately after obtaining resources
- Support multiple date formats: "2006-01-02", RFC3339
- See `internal/cli/CLAUDE.md` for detailed CLI architecture and patterns

### internal/engine

The engine package orchestrates cost calculations between plugins and specs:

- Tries plugins first, falls back to local YAML specs
- Supports three output formats: table, JSON, NDJSON
- Uses `hoursPerMonth = 730` for monthly calculations
- Always returns some result, even if placeholder
- **Actual Cost Pipeline Features**:
  - `GetActualCostWithOptions()` - Advanced querying with time ranges and filters
  - Resource filtering with `matchesTags()` helper for tag-based filtering
  - Cost aggregation logic for daily/monthly breakdowns
  - Grouping support (resource, type, provider, date)
  - Multiple date format parsing ("2006-01-02", RFC3339)
- **Cross-Provider Aggregation Features**:
  - `CreateCrossProviderAggregation()` - Time-based multi-provider cost analysis
  - Currency validation system with `ErrMixedCurrencies` protection
  - Advanced input validation (empty results, invalid date ranges, grouping types)
  - GroupBy type safety with `IsValid()`, `IsTimeBasedGrouping()`, `String()` methods
  - Intelligent cost calculation (actual vs projected with time period conversion)
  - Provider extraction from resource types ("aws:ec2:Instance" → "aws")
  - Sorted chronological output for trend analysis
- See `internal/engine/CLAUDE.md` for detailed calculation flows

**Error Types for Cross-Provider Aggregation**:

- `ErrMixedCurrencies`: Different currencies detected (USD vs EUR)
- `ErrInvalidGroupBy`: Non-time-based grouping used for cross-provider aggregation
- `ErrEmptyResults`: Empty or nil results provided for aggregation
- `ErrInvalidDateRange`: EndDate before StartDate in cost results

### internal/pluginhost

The pluginhost package manages plugin communication via gRPC:

- Two launcher types: ProcessLauncher (TCP) and StdioLauncher (stdin/stdout)
- 10-second timeout with 100ms retry delays
- Platform-specific binary detection (Unix permissions vs Windows .exe)
- Always call `cmd.Wait()` after `Kill()` to prevent zombies
- See `internal/pluginhost/CLAUDE.md` for detailed plugin lifecycle

### internal/registry

The registry package handles plugin discovery and lifecycle:

- Scans `~/.finfocus/plugins/<name>/<version>/` structure
- Optional `plugin.manifest.json` validation
- Graceful handling of missing directories and invalid binaries
- Platform-specific executable detection
- See `internal/registry/CLAUDE.md` for detailed discovery patterns

### internal/analyzer

The analyzer package implements the Pulumi Analyzer gRPC protocol for zero-click cost estimation during `pulumi preview`:

- **Server**: Implements `pulumirpc.AnalyzerServer` interface with `AnalyzeStack`, `Handshake`, `ConfigureStack`, `Cancel`, `GetAnalyzerInfo`, `GetPluginInfo` RPCs
- **Resource Mapping**: Converts `pulumirpc.AnalyzerResource` to `engine.ResourceDescriptor` for cost calculation
- **Diagnostics**: Generates `pulumirpc.AnalyzeDiagnostic` from cost results with ADVISORY enforcement (never blocks deployments)
- **Graceful Degradation**: Errors produce warning diagnostics, preview continues even if cost calculation fails

**Key Components**:

- `Server` - gRPC server implementing Pulumi Analyzer protocol
- `MapResource/MapResources` - Convert Pulumi resources to internal format
- `CostToDiagnostic` - Convert cost results to Pulumi diagnostics
- `StackSummaryDiagnostic` - Aggregate stack-level cost summary
- `WarningDiagnostic` - Generate warning for error conditions

**CLI Integration**:

- `finfocus analyzer serve` - Starts gRPC server on random TCP port
- Prints ONLY port number to stdout (Pulumi handshake protocol)
- All logging goes to stderr exclusively
- Graceful shutdown on SIGINT/SIGTERM

**Protocol Requirements**:

- Port handshake: Server prints port to stdout, Pulumi engine connects
- Stack configuration: Engine sends stack/project context before analysis
- Resource analysis: Engine sends resources, analyzer returns diagnostics
- Cancellation: Engine can request analysis cancellation

**Testing**:

```bash
# Run analyzer unit tests
go test ./internal/analyzer/...

# Run integration tests
go test ./test/integration/...

# Check coverage (target: 80%, achieved: 92.7%)
go test -coverprofile=coverage.out ./internal/analyzer/...
go tool cover -func=coverage.out
```

### plugins/recorder (Reference Plugin)

The recorder plugin is a reference implementation demonstrating how to build a FinFocus plugin using the pluginsdk v0.4.6. It captures all gRPC requests to JSON files and optionally returns mock cost responses.

**Location**: `plugins/recorder/`

**Purpose**:

- Developer tool for inspecting Core-to-plugin data shapes
- Reference implementation for pluginsdk patterns
- Contract testing support for integration tests

**Key Files**:

- `plugins/recorder/plugin.go` - Main plugin implementation with CostSourceService

**Build Commands**:

```bash
make build-recorder    # Build to bin/finfocus-plugin-recorder
make install-recorder  # Build and install to ~/.finfocus/plugins/recorder/0.1.0/
```

**Configuration (Environment Variables)**:

| Variable | Default | Description |
| --- | --- | --- |
| `FINFOCUS_RECORDER_OUTPUT_DIR` | `./recorded_data` | Directory for recorded JSON files |
| `FINFOCUS_RECORDER_MOCK_RESPONSE` | `false` | Enable randomized mock responses |

**Usage Examples**:

```bash
# Record requests to inspect data shapes
export FINFOCUS_RECORDER_OUTPUT_DIR=./debug
./bin/finfocus cost projected --pulumi-json plan.json
cat ./debug/*.json | jq .

# Enable mock mode for testing
export FINFOCUS_RECORDER_MOCK_RESPONSE=true
./bin/finfocus cost projected --pulumi-json plan.json
```

**Recorded File Format**:

```json
{
  "timestamp": "2025-12-11T14:30:52Z",
  "method": "GetProjectedCost",
  "requestId": "01JEK7X2J3K4M5N6P7Q8R9S1T2",
  "request": { /* protobuf request as JSON */ }
}
```

**Testing**:

```bash
go test ./plugins/recorder/...                      # Unit tests
go test ./test/integration/recorder_test.go         # Integration tests
go test -bench=BenchmarkRecorder ./plugins/recorder/...  # Performance (<10ms overhead)
```

**Implementation Patterns Demonstrated**:

- `pluginsdk.BasePlugin` embedding with wildcard provider matcher
- Request validation using pluginsdk v0.4.6+ helpers
- `protojson.Marshal` for human-readable JSON serialization
- ULID for time-ordered, collision-free filenames
- Graceful shutdown with context cancellation
- Thread-safe recording with `sync.Mutex`
- Zerolog structured logging

## Common Error Types

- `ErrNoCostData`: No cost data available for a resource.
- `ErrMixedCurrencies`: Multiple currencies detected in cross-provider aggregation.
- `ErrInvalidGroupBy`: Invalid grouping type used for time-based aggregation.
- `ErrEmptyResults`: Attempted aggregation on empty results.
- `ErrInvalidDateRange`: Invalid date range (end date before start date).
- `ErrResourceValidation`: Internal resource validation failed.
- `ErrConfigCorrupted`: Configuration file is malformed.

## CodeRabbit Configuration

### Setup

The repository includes a comprehensive `.coderabbit.yaml` configuration optimized for Go development with the following key settings:

**PR Blocking Configuration:**

- `fail_commit_status: true` - Blocks PR merging on critical issues
- `request_changes_workflow: true` - Formally requests changes for issues
- `profile: assertive` - Uses stricter analysis profile

**Comment Management:**

- `auto_reply: true` - Enables automatic comment responses
- `abort_on_close: true` - Stops processing when PR is closed
- `auto_incremental_review: true` - Reviews new commits automatically

**Go-Specific Settings:**

- Custom path instructions for `**/*.go` files focusing on Go best practices
- Enhanced test review instructions for `**/*_test.go` files
- Enabled golangci-lint, gitleaks, yamllint, and markdownlint
- Docstring and unit test generation enabled

**Tool Configuration:**

- `golangci-lint: enabled: false` - Integrates with project's existing linting
- `markdownlint: enabled: true` - Validates documentation
- `gitleaks: enabled: true` - Scans for secrets
- `actionlint: enabled: true` - Validates GitHub Actions
- `semgrep: enabled: true` - Advanced security analysis

### Usage

CodeRabbit now:

1. **Blocks PRs** with critical issues by setting commit status to failed
2. **Updates comments** automatically on new commits
3. **Resolves outdated comments** when issues are fixed
4. **Provides detailed Go-specific feedback** on code quality
5. **Integrates with existing CI/CD** tools and workflows

## Active Technologies
- Go 1.25.7 + Cobra v1.10.2 (CLI), gRPC v1.78.0 (plugins), finfocus-spec v0.5.5 (protocol):
  - zerolog v1.34.0 (logging), testify v1.11.1 (testing) (508-recommendation-dismissal)
  - Bubble Tea v1.3.10 (TUI), Lip Gloss v1.1.0 (styling) (223-cost-estimate)
- Local JSON file (`~/.finfocus/dismissed.json`) for dismissal state; plugin-side storage delegated to plugins (508-recommendation-dismissal)
- State Management: N/A (stateless command design) (223-cost-estimate)
- Go 1.25.7 + Cobra v1.10.2 (CLI), zerolog v1.34.0 (logging), testify v1.11.1 (testing). No new dependencies. (509-pulumi-auto-detect)
- N/A (stateless CLI invocation) (509-pulumi-auto-detect)

## Recent Changes

- 223-cost-estimate: Added Go 1.25.7 + Cobra v1.10.2 (CLI), gRPC v1.78.0 (plugins), finfocus-spec v0.5.5 (protocol), Bubble Tea v1.3.10 (TUI), Lip Gloss v1.1.0 (styling)
