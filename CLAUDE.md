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
make docs-lint     # Lint markdown docs
make docs-build    # Build Jekyll site
make docs-serve    # Serve docs at http://localhost:4000/finfocus/
make build-recorder    # Build recorder plugin to bin/finfocus-plugin-recorder
make install-recorder  # Build and install recorder to ~/.finfocus/plugins/recorder/0.1.0/
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
./bin/finfocus plugin list --output json  # JSON array for machine consumption
./bin/finfocus plugin validate
```

### Troubleshooting

```bash
pkill golangci-lint || true             # Fix parallel linting conflicts
GOOS=linux GOARCH=amd64 make build      # Test release build locally
gh workflow validate .github/workflows/ci.yml  # Validate workflow syntax
```

### Test Requirements

- **Unit tests**: Must achieve 80% coverage minimum
- **Critical paths**: Must achieve 95% coverage
- **All error paths**: Must be tested
- **Performance regressions**: Must be detected via benchmarks
- **Integration scenarios**: Must include plugin communication flows
- **End-to-end workflows**: Must test complete CLI usage

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

## Architecture

Core components and their directories:

1. **CLI** (`internal/cli/`) - Cobra commands: cost projected/actual/recommendations, plugin management, analyzer
2. **Engine** (`internal/engine/`) - Cost calculation orchestration, output rendering (table/JSON/NDJSON)
3. **Plugin Host** (`internal/pluginhost/`) - gRPC plugin lifecycle (launch, connect, cleanup)
4. **Registry** (`internal/registry/`) - Plugin discovery in `~/.finfocus/plugins/<name>/<version>/`
5. **Ingestion** (`internal/ingest/`) - Pulumi plan JSON parsing
6. **Analyzer** (`internal/analyzer/`) - Pulumi Analyzer gRPC protocol for zero-click cost estimation
7. **TUI** (`internal/tui/`) - Bubble Tea terminal UI components
8. **Router** (`internal/router/`) - Intelligent plugin routing with priority and fallback
9. **Config** (`internal/config/`) - Two-tier configuration (project-local overrides global)

### Configuration Resolution

**Project-specific settings** (config, dismissals) precedence:

1. `--project-dir` flag (explicit override)
2. `FINFOCUS_PROJECT_DIR` env var
3. Walk up from CWD to find `Pulumi.yaml`, use `$PROJECT/.finfocus/`
4. Fall back to `~/.finfocus/` (backward compatible)

**Global resources** (plugins, cache, logs) precedence:

1. `FINFOCUS_HOME` env var
2. `PULUMI_HOME/finfocus`
3. `~/.finfocus/`

**Config Merge**: Project `config.yaml` overrides global at the **top-level key** level
(shallow merge). Keys absent in project config inherit from global defaults.

## Key Patterns

### CLI Conventions

- Use `RunE` not `Run` for error handling
- Use `cmd.Printf()` for output (not `fmt.Printf()`)
- Defer cleanup functions immediately after obtaining resources
- Support multiple date formats: "2006-01-02", RFC3339

### Pre-Flight Request Validation

The adapter layer (`internal/proto/`) validates requests using `pluginsdk` validation
before gRPC calls. Key points:

- Validation happens in `GetProjectedCostWithErrors()` and `GetActualCostWithErrors()`
- Uses "VALIDATION:" prefix to distinguish from plugin errors ("ERROR:")
- Logs at WARN level with resource context for debugging
- Returns placeholder CostResult with $0 cost and descriptive Notes
- Invalid resources are skipped; valid resources still call the plugin

### Logging

```bash
# Enable debug output
finfocus cost projected --debug --pulumi-json plan.json
export FINFOCUS_LOG_LEVEL=debug
export FINFOCUS_LOG_FORMAT=json    # json or console
export FINFOCUS_TRACE_ID=external-trace-123  # inject external trace ID
```

Precedence: CLI flags (`--debug`) > env vars > config file > default (info, console).

## Testing

### E2E Testing

**Location**: `test/e2e/` (separate Go module)

**Prerequisites**: AWS session or profile configured, Pulumi CLI, `make build`

```bash
export PATH="$HOME/.pulumi/bin:$PATH"
export PULUMI_CONFIG_PASSPHRASE="e2e-test-passphrase"
make test-e2e
```

**CRITICAL**: E2E tests MUST call actual finfocus CLI binary.
Never simulate cost values or stub CLI execution.

### Expected Failure Test Patterns

**IMPORTANT**: Tests that intentionally create failing plugin scenarios must follow
these patterns to avoid false CI failures:

- **Expected errors**: Use `t.Logf()` (informational, test passes), NOT `t.Errorf()`
- **Required errors**: Use `t.Fatalf("expected error")` only if error is absent
- **Common expected errors**: `context deadline exceeded`, `connection refused`,
  `broken pipe`/`EOF`, `no such file or directory`

If CI shows these errors in logs but tests are marked PASS, the behavior is correct.
Only investigate if tests actually FAIL (exit code 1).

### Error Path Testing

When writing new code, always include tests for error conditions:

1. Test every error return path
2. Validate error messages with `assert.Contains(t, err.Error(), "expected text")`
3. Test boundary conditions: empty inputs, nil pointers, invalid ranges
4. Test partial failures in batch operations
5. Test resource cleanup runs even when errors occur (defer patterns)

### Testify Assertion Standards

**CRITICAL**: All Go tests MUST use testify's `require` and `assert` packages.
NEVER use manual `if x != y { t.Errorf(...) }` patterns.

**`require.*`** (stops test on failure): Setup operations, error checks where
continuing would panic, non-nil checks before use.

**`assert.*`** (continues on failure): Value comparisons, multiple property
checks, non-critical validations.

| Manual Pattern | Testify Replacement |
| --- | --- |
| `if err != nil { t.Fatal(err) }` | `require.NoError(t, err)` |
| `if err == nil { t.Error("expected error") }` | `require.Error(t, err)` |
| `if x != y { t.Errorf("got %v, want %v", x, y) }` | `assert.Equal(t, y, x)` |
| `if len(x) != n { t.Errorf(...) }` | `assert.Len(t, x, n)` |
| `if !strings.Contains(s, sub) { t.Errorf(...) }` | `assert.Contains(t, s, sub)` |
| `if x == nil { t.Fatal("nil") }` | `require.NotNil(t, x)` |

### Local Plugin Development

1. Clone the plugin repository (e.g., `finfocus-plugin-aws-public`)
2. Modify the plugin code (add logging, fix type mapping)
3. Build: `make build-region REGION=us-east-1`
4. Install: Copy binary to `~/.finfocus/plugins/<plugin>/<version>/`
5. Run Core E2E tests to verify

## Important Files

- `cmd/finfocus/main.go` - CLI entry point (semantic exit codes: 0=success, 1=error, 2=budget exceeded)
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

## Common Error Types

- `ErrNoCostData`: No cost data available for a resource
- `ErrMixedCurrencies`: Multiple currencies detected in cross-provider aggregation
- `ErrInvalidGroupBy`: Invalid grouping type used for time-based aggregation
- `ErrEmptyResults`: Attempted aggregation on empty results
- `ErrInvalidDateRange`: Invalid date range (end date before start date)
- `ErrResourceValidation`: Internal resource validation failed
- `ErrConfigCorrupted`: Configuration file is malformed

Structured error codes for JSON/NDJSON output: `PLUGIN_ERROR`, `VALIDATION_ERROR`,
`TIMEOUT_ERROR`, `NO_COST_DATA` (see `internal/engine/types.go`).

## Recorder Plugin

Reference plugin for inspecting Core-to-plugin data shapes and contract testing.

| Variable | Default | Description |
| --- | --- | --- |
| `FINFOCUS_RECORDER_OUTPUT_DIR` | `./recorded_data` | Directory for recorded JSON files |
| `FINFOCUS_RECORDER_MOCK_RESPONSE` | `false` | Enable randomized mock responses |

## Package-Specific Gotchas

Non-obvious behaviors that can cause subtle bugs if you don't know about them.

### Plugin Host (`internal/pluginhost/`)

- **Port allocation**: Uses allocate→hold→release→bind pattern. Race window between
  release and plugin bind is mitigated by `StartWithRetry`
- **Zombie prevention**: Always call `cmd.Wait()` after `cmd.Process.Kill()`
- **`PORT` env var removed**: Core does NOT set `PORT` (avoids Cloud Run conflicts).
  Plugins must use `--port` flag or `FINFOCUS_PLUGIN_PORT`

### CLI (`internal/cli/`)

- **`analyzer serve` stdout**: Prints ONLY the port number to stdout (Pulumi handshake
  protocol). ALL logging must go to stderr exclusively
- **DismissalStore**: Uses `GetResolvedProjectDir()` → project `dismissed.json`;
  falls back to `~/.finfocus/dismissed.json`
- **`config init`**: Without `--global`, inside a Pulumi project creates
  `$PROJECT/.finfocus/config.yaml` + `.gitignore`. Outside Pulumi project → global init
- **`config routes`**: `config routes list` shows effective routing source/path;
  `config routes test <type> [region]` simulates per-feature plugin selection without loading plugin binaries

### Registry (`internal/registry/`)

- **Region-specific binaries**: When `plugin.metadata.json` has a `region` key, registry
  looks for `finfocus-plugin-<name>-<region>` first, then falls back to standard names
- **Checksum verification**: Only a confirmed hash mismatch is fatal. Missing
  `checksums.txt`, download failures, or unlisted assets produce warnings and continue
- **`--skip-checksum`** flag available on `plugin install` and `plugin update`

### Router (`internal/router/`)

- **Fallback chain**: `$0.00` cost is a VALID result (does NOT trigger fallback).
  Only nil/empty results trigger fallback to the next plugin
- **Priority**: Higher number = higher priority (sorted descending by `sortByPriority`)
- **No config = no routing**: `createRouterForEngine()` returns nil if no routing config;
  engine falls back to querying all plugins

### Engine (`internal/engine/`)

- **`hoursPerMonth = 730`** for monthly cost calculations
- **Budget health thresholds**: OK (<80%), WARNING (80-89%), CRITICAL (90-100%),
  EXCEEDED (>100%). Aggregation uses worst-case status
- **Cache hits**: Append ` (cached)` to the Adapter field for visual feedback
- **Cache corruption**: Auto-detected and auto-recovered (delete + recreate)

### Overview Field Semantics (`internal/engine/overview_*.go`)

Understanding what each field *means* prevents the most common overview bugs.
Every field on `OverviewRow` has a specific temporal basis, population rule,
and set of valid comparisons. Violating these invariants produces subtle bugs
(misleading deltas, nonsensical drift, UI garbage).

#### Cost Fields — Temporal Basis

| Field | Struct | Temporal Basis | Unit | Source |
| --- | --- | --- | --- | --- |
| `MTDCost` | `ActualCostData` | Partial month (day 1 → today) | Dollars spent so far | Actual cost plugin |
| `MonthlyCost` | `ProjectedCostData` | Full canonical month (730h) | Dollars if run all month | Projected cost plugin |
| `ExtrapolatedMonthly` | `CostDriftData` | Full calendar month (28-31d) | Projected from MTD trend | Calculated by `CalculateCostDrift` |
| `Delta` | `CostDriftData` | Full calendar month | ExtrapolatedMonthly - Projected | Calculated by `CalculateCostDrift` |

**Key rule**: `MTDCost` and `MonthlyCost` are **different units**. You cannot
subtract one from the other. To compare them, you must first extrapolate
`MTDCost` to a full month using `getExtrapolatedActual()` (30-day standard)
or `CalculateCostDrift()` (calendar-accurate).

#### When Fields Are Nil vs Populated

Fields start nil after merge and get populated during enrichment. What gets
populated depends on the resource's `Status`:

| Status | `ActualCost` | `ProjectedCost` | `CostDrift` | `PropertyDiffs` |
| --- | --- | --- | --- | --- |
| Active | Yes (has billing history) | Yes (current config) | Maybe (nil if < 10% or day < 3) | No (no changes) |
| Updating | Yes (still running) | Yes (new config pricing) | Maybe | Yes (what changed) |
| Replacing | Yes (old resource billing) | Yes (new resource pricing) | Maybe | Yes (what changed) |
| Creating | No (doesn't exist yet) | Yes (new resource pricing) | No (no history) | No |
| Deleting | Yes (still running) | No (will be removed) | No (no projection) | No |

This table is the **source of truth** for which cost computations are valid
per status. If a formula assumes a field is non-nil, check this table first.

#### Delta Column — What It Means Per Status

The "Delta" TUI column answers: "how will this change affect my monthly bill?"
Use `CalculateRowDelta()` — it encodes status-aware logic:

| Status | Delta Formula | Meaning |
| --- | --- | --- |
| Updating/Replacing | `projected - extrapolatedActual` | Cost impact of the config change |
| Creating | `+projected` | New cost being added |
| Deleting | `-extrapolatedActual` | Cost being removed |
| Active (with drift) | `CostDrift.Delta` | How much actual spend deviates from projection |
| Active (no drift) | `-` (no delta shown) | Spend is tracking projection (< 10% off) |

#### Extrapolation — Two Methods, Intentionally Different

| Function | Month Basis | Used For |
| --- | --- | --- |
| `getExtrapolatedActual()` | 30-day standard | Delta calculations (consistent cross-month) |
| `CalculateCostDrift()` | Calendar days (28-31) | Drift % (calendar-accurate precision) |

Do not unify these. Delta uses 30-day for stable comparisons across months.
Drift uses calendar days because a February drift % must account for 28 days.

#### Drift Nil Cases

`CostDrift` is nil (not populated) in these cases — all intentional:

- **Day 1-2 of month** (`driftMinDay = 3`): insufficient data
- **Drift < 10%** (`driftWarningThreshold`): not significant enough to show
- **New resource** (has projected, no actual): nothing to extrapolate from
- **Deleted resource** (has actual, no projected): nothing to compare against
- **Recently created** (`CreatedAt` within billing window, < 3 days old)

Code must always handle `row.CostDrift == nil` as a normal case, not an error.

#### Pulumi Plan Data — What to Filter

`PulumiStep.OldState.Inputs` and `NewState.Inputs` contain both user-specified
properties and Pulumi internal metadata. When displaying to users:

- **Filter keys prefixed with `__`** (e.g., `__defaults`, `__provider`) —
  these are Pulumi SDK internals, not user properties
- **Truncate values** in TUI to prevent wrapping (max 40 chars via
  `truncateDiffValue()`) — Pulumi inputs can contain large arrays/objects
- **PropertyDiff data flow**: Plan JSON → `diffInputs()` (CLI) →
  `PlanStep.PropertyDiffs` → merge → `OverviewRow.PropertyDiffs` → TUI view

#### State-Only Mode

When no preview is provided, overview shows state resources with `*` footnote
on projected costs. The `p` key triggers on-demand preview; when it completes,
`ApplyChangesToRows()` and `ApplyPropertyDiffsToRows()` update rows in-place.

### GitHub Actions (`.github/workflows/`)

- **OpenCode Action** (`sst/opencode/github@dev`) ONLY works with `issue_comment` events.
  For other triggers, use CLI installation
- **`/opencode-review-fix`** comment on a PR triggers automatic fix of all review issues

## Recent Changes

- 604-charm-v2-upgrade: Added Go 1.25.7 (see `go.mod`)

## Active Technologies

- Go 1.25.7 (see `go.mod`) (604-charm-v2-upgrade)
- Bubble Tea v2 (`charm.land/bubbletea/v2 v2.0.0`) (604-charm-v2-upgrade)
- Bubbles v2 (`charm.land/bubbles/v2 v2.0.0`) (604-charm-v2-upgrade)
- Lip Gloss v2 (`charm.land/lipgloss/v2 v2.0.0`) (604-charm-v2-upgrade)
- CLI commands using Cobra, tabwriter, and Viper for config parsing
