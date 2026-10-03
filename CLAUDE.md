# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## CRITICAL INSTRUCTIONS

**DO NOT RUN `git commit`** - This is explicitly forbidden. Use `git add`, `git status`, `git diff`, and `git log` only. The user will commit manually.

**ALWAYS run `make lint` and `make test`** before claiming success.

**DO NOT modify `.golangci.yml`** without explicit approval.

## Project Overview

FinFocus Core is a CLI tool and plugin host system for calculating cloud infrastructure costs from Pulumi infrastructure definitions. It provides both projected cost estimates and actual historical cost analysis through a plugin-based architecture.

## Specs

Feature specs, plans, and tasks live in `specs/NNN-*/` and are produced by the
**Spec Kit** pipeline (`.specify/` templates, `.specify/memory/constitution.md`,
and the `/speckit-*` skills). Spec Kit is the ONLY spec/plan/task pipeline:
design or plan output from other tools (e.g. superpowers brainstorming or
writing-plans) MUST be converted into Spec Kit feature folders under `specs/`
before implementation, not committed elsewhere in the repo.

## Build Commands

```bash
make tools         # Install toolchain pinned in mise.toml (first-time setup)
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
make test-jev          # Test the jev scorer plugin module (plugins/jev, separate go.mod)
make install-jev       # Build and install the jev scorer plugin
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

**Project Go Version**: 1.27.1 (see `go.mod`)

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

**Config Merge**: Project `config.hujson` overrides global `config.hujson` at the **top-level key** level
(shallow merge). Keys absent in project config inherit from global defaults. Legacy `config.yaml` files are auto-migrated to Hujson format on first read.

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
- A projected resource with `ref.*` tags and an empty SKU uses lenient validation
  (`pluginsdk.ValidateProjectedCostRequestLenient`) instead of failing closed on SKU

### Logging

```bash
# Enable debug output
finfocus cost projected --debug --pulumi-json plan.json
export FINFOCUS_LOG_LEVEL=debug
export FINFOCUS_LOG_FORMAT=json    # json or console
export FINFOCUS_TRACE_ID=external-trace-123  # inject external trace ID
```

Precedence: CLI flags (`--debug`) > env vars > config file > default (info, console).

### Global Agentic Flags

The following flags are automatically mounted by `ax.Execute` on all commands:

- **`--format json|human`**: Output format for machine vs human consumption (JSON for agent/tooling, human for terminal). When set to `json`, some commands automatically output machine-readable JSON. Defaults to auto-detection via TTY.
- **`--dry-run`**: Preview changes without making them (skip real side effects). Affects mutating commands: `plugin install/update/remove`, `analyzer install/uninstall`, `config init/set`, and recommendation operations (dismiss/snooze/undismiss), and `plugin upgrade`.
- **`--yes`**: Skip confirmation prompts (equivalent to `-y` or `--force` on individual commands). Automatically confirms operations that normally require user approval.
- **`--idempotency-key string`**: Opaque retry-deduplication key for preventing duplicate-create operations in distributed systems.
- **`--debug`**: Enable debug logging (also available as persistent flag for CLI-specific control).

Additionally, the following utility commands are available:

- **`__schema [--as=ax|mcp]`**: Emit machine-discoverability schema in AX (default) or MCP format for tool discovery.
- **`mcp-server [--transport=stdio|http] [--addr] [--allow-non-loopback]`**: Expose the entire CLI as a live Model Context Protocol server.
- **`--mcp`** (root-local flag): stdio alias for `mcp-server`; same tool list. User docs: `docs/src/content/docs/guides/mcp.md`.

## Testing

### TUI Visual Verification

After modifying `internal/tui/` code, ALWAYS:

1. Render the affected view and read the full output (not just test pass/fail)
2. Verify column alignment, section ordering, and data population visually
3. Run golden file tests: `go test -run TestGolden ./internal/tui/...`
4. Regenerate golden files if layout intentionally changed:
   `UPDATE_GOLDEN=1 go test -run TestGolden ./internal/tui/...`

String-only assertions (`assert.Contains`) are NOT sufficient for TUI testing.

### Parallel Tests

Tests call `t.Parallel()` (top level and each `t.Run` subtest) unless they touch
process-wide state. Parallel tests only run after every sequential test in the
package has finished, so the hazard is parallel tests interfering with each other.

- Not parallel: `t.Setenv`, `t.Chdir`, `os.Setenv`, swapping a package-level
  variable, `config.ResetGlobalConfigForTest`/`SetGlobalConfig`, or anything that
  builds a root command (`NewRootCmd*` sets the process-wide resolved project
  directory through `config.SetResolvedProjectDir`). This includes helpers that call
  those, such as `stubHome` or `WithEnv`.
- Not parallel: a test that writes a script or binary (`#!/bin/sh`, `os.WriteFile`
  or `os.Chmod` with exec bits) and then executes it, directly or through a helper
  or launcher. A concurrent fork in the same process lets the child inherit the open
  write fd, so the exec fails with `text file busy` (ETXTBSY, golang/go#22315).
  Only the executing test needs to be sequential, because sequential tests never
  overlap parallel ones.
- Mark such a test with `//nolint:paralleltest // <specific reason>` on the line
  above `func`. Put the directive on the `t.Run` or `for` line instead when only
  some subtests must stay sequential. `nolintlint` requires the reason.
- Table-driven subtests must not share a mutable fixture. Production code mutates
  proto messages and slices in place (for example `Engine.GetBudgets` on a shared
  `*pbc.Budget`), so build the fixture inside the subtest.
- Do not use `defer` in a parent test that starts parallel subtests; use
  `t.Cleanup`. The defer runs before the paused subtests resume.
- Verify with `go test -race -shuffle=on -count=3 ./<pkg>/...`. A flake or race
  means the test is not safe to parallelise.

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

- `cmd/finfocus/main.go` - CLI entry point, routed through `ax.Execute` (ax-go). Exit codes:
  0=success, 1=generic/internal error (ax `ExitInternal`), and a user-configurable code (default 1,
  0-255 via `cost --exit-code`/`--exit-on-threshold`) for budget-exceeded, preserved through
  `ax.Execute` via `internal/cli.toAxExitError`. `ax.ExitValidation`(2)/`ExitNetwork`(3)/`ExitAuth`(4)
  are ax-go's own reserved exit codes for new error classifications, not yet used anywhere in
  finfocus, and not retrofitted onto existing exit-1 paths.
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

`newState` and stack-export resources also carry `parent`, `dependencies`, and
`propertyDependencies`. `parent` is the Pulumi stack, not a pricing reference.
`ApplyCrossResourceRefs` follows `propertyDependencies` only: a property that
points at exactly one in-plan resource becomes `ref.<property>.{urn,type,region,sku}`
tags. The Pulumi unknown sentinel `04da6b54-80e4-46f7-96ec-b56ff0331ba9` is never
sent as a tag, SKU, or region. A child's own region is filled from the referenced
resource only when the child has none and exactly one referenced resource has a
region. The referenced SKU is never copied into the child's `Sku`. A child with
`ref.*` tags and an empty SKU uses `ValidateProjectedCostRequestLenient` so the
plugin still sees the tags. Classic Azure `skuName` is copied only into
`ref.<property>.sku` (the provider resolver does not treat `skuName` as the
resource's own SKU). Projected cache keys append `/refs-<hash>` when those tags
exist. Spec: `specs/618-cross-resource-refs/`.

`ConvertToProto` also emits dotted keys for nested maps and arrays
(`sku.capacity`, `rootBlockDevice.0.volumeType`) beside each collapsed key.
It skips `__` segments, credential-like segments (`password`, `secret`,
`token`, `credential`, `ciphertext`, `privatekey`, `apikey`, `accesskey`,
`connectionstring`, with the snake_case forms), a top-level input named `ref`
(so it cannot flatten into the `ref.*` reference namespace), and the containers
`tags`, `tagsAll`, `labels`, and `annotations`. Depth is capped at 6
segments, new keys at 128 characters, new values at 256 characters, and the
whole tag map at 50 entries. Existing collapsed keys are kept first.
Projected cache keys always append `/tags-<digest>` of that map, then
`/refs-<hash>` when reference tags exist. `EstimateCost` attributes stay
nested. Spec: `specs/619-dotted-tag-keys/`.

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

### Agent Skills Placement

- **Tool-specific skills** (`finfocus-install`, `finfocus-budget`, `finfocus-diagnose`, etc.) → live in
  `rshade/finfocus` under `agent-skills/` — these are product skills tightly coupled to
  finfocus CLI commands, file paths, and architecture
- **Generic cost workflow skills** (`cost-check`, `cost-drift`, `cost-optimize`,
  `budget-setup`) → live in `rshade/agent-skills` as multi-tool skills not tied to any
  specific cost tool

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

## Jev Scorer Plugin

`plugins/jev/` is a separate module (like `plugins/kubernetes`) implementing
`RecommendationScorerService` with TypeSafe AI's Jev API. It must not import
finfocus core packages (`make check-plugin-boundaries`). Design notes:

- Off unless `TYPESAFE_API_KEY` is set; without it every scoring call returns
  `UNAUTHENTICATED`. The key is read only from the environment and must never
  appear in logs, errors, tests or committed files.
- Thin `net/http` client in `plugins/jev/internal/jevapi`; no third-party Jev SDK.
- Question names are `<signal>:#<position>` over an ordered state list; the
  recommendation id never leaves the plugin (answers map back by position via
  `batch.indexes`). Question text never contains recommendation content. Free
  text is stripped and capped in `internal/scoring/record.go`. Tests find a
  record's key with `questionKey()` in `helpers_test.go`, because every
  one-record `priority` request uses `priority:#0`.
- `jevapi.Client` does not start a retry whose wait outlasts the caller's
  deadline (core's `scoring.timeout_seconds`); it returns the `*APIError`.
- Live tests (`TestLive*`, `TestEvaluation` with `JEV_EVAL=1`) are skipped without
  a key. Batching lowers `priority` rank quality (Spearman about 0.55 at batch
  25 against 0.77 at batch 1), so priority is always one record per request.
- Scores are ranking-only. Never gate an irreversible action on one.

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
  `$PROJECT/.finfocus/config.hujson` + `.gitignore`. Outside Pulumi project → global init
- **`config routes`**: `config routes list` shows effective routing source/path;
  `config routes test <type> [region]` simulates per-feature plugin selection without loading plugin binaries
- **`config validate` and cost pre-run**: both call `ValidateConfigSource`.
  `--file` selects a document. `--output json` prints the report. An unknown
  `--output` is rejected before the file is read. A missing default file stays
  valid. `cost projected`, `cost actual`, `cost recommendations`,
  `cost estimate`, and `cost cluster` fail in pre-run when a present file is
  invalid, and they ignore a missing file. Flat `cost.budgets.amount` is warned
  and not applied; the on-disk field is `cost.budgets.global.amount`. Period
  stays monthly. Threshold stays 0–1000. Amount 0 disables a scope. Unknown
  keys warn, with a suggestion when the name is close. There is no
  notifications section
- **Unit tests leak into the real `~/.finfocus`**: any test that executes a
  mutating command (`dismiss`, `snooze`, `config set`) writes to the developer's
  and the CI runner's actual home unless it sets
  `t.Setenv("FINFOCUS_HOME", t.TempDir())`. `NewDismissalStore("")` falls back to
  `ResolveConfigDir()`, which honors `FINFOCUS_HOME` first. This has produced
  tests that pass *only* because a sibling test seeded the shared store — green
  as a package, red when run alone. Detect with
  `HOME=$(mktemp -d) go test -run TestName ./internal/cli/`; a test that fails
  in isolation but passes in the package is order-dependent, not flaky
- **Validate `--output` before loading state**: commands that early-return on an
  empty result (e.g. history's `len(events) == 0`) must reject an unknown format
  *first*, or an invalid `--output` silently exits 0. See `config_routes.go`,
  `analyzer_check.go`, `plugin_list.go` for the up-front pattern
- **Overview docs path**: the published page is
  `docs/src/content/docs/commands/overview.md`. Root `README.md` links there.
  `docs/commands/overview.md` is not a file. Plain headers are
  `ACTUAL(MTD)` and `PROJECTED` (`PROJECTED*` in state-only mode). Delta comes
  from `CalculateRowDelta`, not projected minus month-to-date. The Warn column
  repeats a shown drift as `drift` and also lists `error` and `new`. `error`
  covers a plugin error reported on the result (`PLUGIN_ERROR`, `TIMEOUT_ERROR`,
  `VALIDATION_ERROR`, an `ERROR:`/`VALIDATION:` note) as well as a returned error; `NO_COST_DATA`
  is not an error. `ApplyChangesToRows` re-derives warnings when a status changes. The TUI
  cell uses `name+N` when that list does not fit in 7 columns. `estimate` and
  `stale` exist on `OverviewWarning` and are not derived
- **Cost table flags**: `--show-breakdown` on `cost projected` and `cost actual`
  adds alphabetical component sub-rows in the plain table (`├─`, last row `└─`).
  Empty maps, nil maps, and empty keys add no sub-rows. `--show-confidence`
  adds the actual-cost Confidence column. Neither flag changes JSON or NDJSON.
  `--estimate-confidence` still sets the plugin request and decides whether
  JSON/NDJSON keep `confidence`. Either new actual flag skips the interactive
  TUI so the table is what the user sees. Time-based `--group-by` stays on the
  cross-provider aggregation table, which has no per-resource sub-rows
- **Accessibility flags are local to output commands**: `overview`, `cost projected`, `cost actual`, and `cost recommendations` accept `--no-color`, `--plain`, `--color`, and `--high-contrast`. `overview` also keeps `--force-color` as an alias of `--color`. Do not put `--plain` on the `cost` parent: `cost history view` already defines a local `--plain`, and Cobra rejects the redefined flag. `FINFOCUS_PLAIN`, a non-empty `NO_COLOR`, `FORCE_COLOR`, and `FINFOCUS_HIGH_CONTRAST` fill unset flags (`strconv.ParseBool`; invalid values are ignored). Explicit `--plain` or `--no-color` wins over color flags. Those env vars win over `FORCE_COLOR` and `FINFOCUS_HIGH_CONTRAST` when the color mode was not set by a flag. `--plain` and `--no-color` stay `OutputModePlain`. High contrast does not change `ColorOK` and the other package color variables; `tui.Palette(true)` is ANSI 46, 226, 196, and 231, used by the styled budget box. Plain budget status text is `[OK] Within budget` or `[WARNING] Exceeds N% threshold`. Call `tui.DetectResolvedOutputMode` with the resolved struct, never `DetectOutputModeFor`,
  which re-reads `NO_COLOR` and would undo an explicit `--color`. `RenderScopedBudgetStatus` takes the
  resolved `tui.Accessibility` like the global box. `cost projected` prints its diff table as plain text in
  every mode, so its four flags only change the budget box (the help text says so). `FINFOCUS_PLAIN` fills
  in `cost history view`'s default format only: an explicit `--output`, `--format json`, or `AGENT_MODE`
  keeps JSON, and an explicit `--plain` still wins. `FORCE_COLOR` accepts `1`, `2`, `3`, or `true`
- **Budget CLI flags stay off the global config**: `--exit-on-threshold` and
  `--exit-code` are `BudgetFlagOverrides` on the command context, passed into
  `evaluateBudgetStatus` and `legacyBudgetConfig`. They are not written onto
  `config.GetGlobalConfig()`. Precedence is the CLI pointer, then the scoped
  budget field, then parent `BudgetsConfig`, then the zero value. Only a copy
  of the global scope receives the overlay. Provider, tag, and type scopes do not.

### Registry (`internal/registry/`)

- **Region-specific binaries**: When `plugin.metadata.json` has a `region` key, registry
  looks for `finfocus-plugin-<name>-<region>` first, then falls back to standard names
- **Checksum verification**: Only a confirmed hash mismatch is fatal. Missing
  `checksums.txt`, download failures, or unlisted assets produce warnings and continue
- **`--skip-checksum`** flag available on `plugin install` and `plugin update`
- **Monorepo plugins (`tag_prefix`)**: an entry with `tag_prefix: "kubernetes-"` installs
  from `rshade/finfocus` tags like `kubernetes-v0.1.0`. "Latest" is the highest semver among
  prefixed stable releases (scans 100), never `/releases/latest`; install dir, asset names, and
  recorded version use the *canonical* version (`v0.1.0`, via `CanonicalVersion`). The entry
  also needs `asset_hints.asset_prefix`, and prefixes must not start with `v` (CLI workflows
  treat `v*` tags as CLI releases)

### Router (`internal/router/`)

- **Fallback chain**: `$0.00` cost is a VALID result (does NOT trigger fallback).
  Only nil/empty results trigger fallback to the next plugin
- **Priority**: Higher number = higher priority (sorted descending by `sortByPriority`)
- **Provider is the billing cloud, not the package**: `resourcetype.NormalizeProvider`
  holds the only alias table (`aws-native`→`aws`; `azure-native`, `azurerm`→`azure`;
  `google-native`, `google`→`gcp`; anything else passes through lowercased).
  `resourcetype.ExtractProvider` returns the cloud and `ExtractPackage` the raw
  prefix. Never split a type token for a provider anywhere else. `resource_type`
  keeps the package, so routing `patterns` such as `aws-native:*` still match.
  `ProviderMatches` normalizes both sides, so a plugin that still lists
  `azure-native` keeps matching. Provider budget keys, `ByProvider`,
  `GroupByProvider`, the `provider=` filter and history provider lookups compare
  normalized names, which also lets history entries stored under a package name
  match a cloud filter
- **No config = no routing**: `createRouterForEngine()` returns nil if no routing config;
  engine falls back to querying all plugins

### Engine (`internal/engine/`)

- **`hoursPerMonth = 730`** for monthly cost calculations
- **Projected cost diff**: `cost projected` calls `GetProjectedCostDiff`.
  An empty operation is a create (`$0` before, current price after); Terraform
  state maps to `same` (existing infrastructure, no delta). Internal `pulumi:`
  types that no plugin priced are left out of the entries, and a projected cache
  hit is re-keyed to the requesting resource's ID (identical resources share one
  cache entry). `update`,
  `replace`, and `create-replacement` price `OldProperties` then `Properties`.
  `delete` and `delete-replaced` price old properties and a `$0` after.
  `same` is priced once. Both sides use the same basis: `OldProperties` is outputs
  merged under the old inputs (like `Properties`), and `ref.*` tags are resolved
  once over the whole plan (`resolveDiffRefTags`) and given to both descriptors,
  because the before and after slices hold different resources. A replacement's
  `create-replacement`, `replace`, and `delete-replaced` steps share a URN and
  are collapsed into one `update` entry first (`collapseReplacements`), or the
  old resource would be counted twice. `summary.totalMonthly` and budget evaluation use the
  after total. Table output is the diff table (TTY included). JSON adds
  `finfocus.diff`. NDJSON is one diff entry per line. `--show-breakdown`
  adds component sub-rows on that table only. Recommendations are merged
  onto the after cost and skipped for deletes
- **Cost history** (`cost history collect|view|list`): snapshots live in
  `~/.finfocus/history/<project>@<stack>.history.db` (`@` cannot appear in a
  Pulumi name, so names never collide). The project is the `Pulumi.yaml` in the
  working directory or the middle part of `--stack org/project/stack`
  (`resolveHistoryTarget`). A database records its project in `meta` and
  `OpenCostDBFor` errors on a different one. A legacy `<stack>.history.db`
  (slashes become dashes) is kept by the project its snapshot URNs name
  (`CostDB.InferProject`), or adopted when they name none; any other project
  gets its own file. With no project, a bare `--stack` uses the legacy file,
  else a single `@` match, else errors. That file is not the
  resource-observation `history.db`. `collect`
  prices each successful `update` through `GetProjectedCostWithErrors` and
  stores the result; a successful `destroy` is `$0` with annotation
  `Stack destroyed` and does not call the pricer. `view` and `list` are
  read-only. A real plugin `$0` is stored. Adapter `none`, the unpriced note,
  or a result error fails that checkpoint (fail fast, per the #549 design: no
  inaccurate totals), and that includes a type an installed plugin declines.
  `pulumi:` internal types (provider resources are `custom: true`) are skipped
  before pricing, because the engine never prices them. `internal/history` must not import
  `engine` or `ingest` (`engine` already imports `history`). The parent
  `cost --stack` flag is reused; history subcommands do not redeclare it
- **Cost history collect details**: `pulumiExporter.History` reads
  `pulumi stack history` page by page (page size 100), stops on a
  short page or a page with no new versions, and asks once without paging flags
  if the CLI rejects them, because the default page is 10 updates. `import` and
  `refresh` updates are not collected (a checkpoint is a deployment; their cost
  shows at the next update). `history list` skips an unreadable or locked database
  with a stderr warning (`ListCostDBsLenient`), and `Stats` reads only the key
  count and the two end snapshots. `view --provider` errors on an unknown provider.
- **Cost history currencies**: `cost history view` warns and keeps the
  dominant currency (most snapshots; a tie keeps the newer timestamp, then
  the earlier code). `--currency` filters and does not warn. `--strict`
  returns `history.ErrMixedCurrencies`. An empty currency is `(unset)`.
  One currency produces no warning. `cost history export` uses the same
  default and prints the warning on stderr
- **Cost history prune**: `cost history prune` deletes per-stack snapshots
  (`--keep`, `--older-than`, `--force`). `--dry-run` is the global ax flag.
  Compaction uses `bolt.Compact` after the database is closed. This is not
  `BoltStore.cleanupExpiredEntries`. `cost.history.retention.auto_prune`
  runs that policy after `collect` when the command loaded config
- **Cost history export**: `cost history export` writes JSON, CSV, or NDJSON
  from the per-stack database. `--format` is the root ax flag. The command
  does not declare a second `--format`, because Cobra rejects the redefined
  flag. `csv` and `ndjson` are not agent modes (`ParseMode` allows `json` and
  `human` only). The export command's `Args` saves those two values and sets
  the flag to `json` before `ax.Execute` resolves the mode. Other commands
  still reject them. `--from`
  and `--to` are inclusive dates. `--provider` keeps every snapshot and sets
  `total_monthly` to that provider, or 0 when the snapshot has none.
  Sparklines are `history.Sparkline` (U+2581 through U+2588, width 7). A flat
  series is the low block. Empty input is an empty string. `cost projected`
  and `cost actual` add a Trend column only when that stack's history file
  exists. A missing file does not fail the cost command. There is no ntcharts
  dependency. The interactive `d` key from #550 is not part of this command
- **Cost history diff**: `cost history diff` compares two stored snapshots
  by URN (`internal/history/diff.go`). It reads bbolt only. A cost move of
  $0.01 or less is unchanged. `--threshold` hides smaller impacts.
  `--from` and `--to` take `vN` or `YYYY-MM-DD` (nearest snapshot). Empty
  `--to` is the newest snapshot. The interactive `d` key from #550 is not
  part of this command
- **Cost history in CI**: `cost history collect --versions` defaults to 0
  (every successful checkpoint not already stored). `--versions 1` keeps
  the newest. Recipes are
  `docs/src/content/docs/guides/ci-cd-cost-tracking.md`. There is no
  `docs/guides/ci-cd-cost-tracking.md` page. Release assets are versioned
  archives (`finfocus-v0.4.0-linux-amd64.tar.gz`); the install script is
  `scripts/install.sh`
- **`cost projected --explain`** calls `GetPricingSpec` after
  `GetProjectedCostDiff` and stores the view on `DiffEntry.PricingSpec`
  (`pricing_spec` in JSON and NDJSON). It does not change `Monthly`. The
  default is off, and a disabled run does not call the RPC. A delete is
  explained from the old properties; every other operation uses the new
  ones. An RPC error or an empty spec omits the field and keeps the cost.
  The next selected plugin is tried when the first has no spec. Unpriced
  `pulumi:` rows are not explained because they are not diff entries
- **Plugin pricing spec fallback** is off by default. `newEngineWithCache`
  copies `cost.pricing_spec_fallback`. `cost projected --pricing-spec-fallback`
  overrides that for the command when the flag is set. When on, a resource
  whose plugins returned no projected price is priced from `GetPricingSpec`
  before local YAML. Disabled runs do not call the RPC. Notes start with
  `Calculated from plugin pricing spec` and the adapter is `plugin-spec`.
  A `$0` rate is a priced result. Unknown billing modes and RPC errors fall
  through to YAML. Each `GetPricingSpec` call (fallback and `DiscoverPricingSpec`)
  has a 5s deadline (`perResourceTimeout`). A chain that stopped at a plugin
  whose router `Fallback` is false asks no plugin for a spec. With the flag on,
  projected cache keys end in `/pricing-spec`, so a run with it off never reads a
  `plugin-spec` result (the exported `ProjectedResourceCacheKey` stays the
  default key). `per_hour` and `per_cpu_hour` use 730 hours (the AWS Pricing Calculator
  basis, 365 × 24 / 12), `per_day` uses `daysPerMonth` (30, the month AWS's billing
  examples use: the ELB page's `$/hour * 24 hours * 30 days` and CloudWatch's
  `30 days * 24 hours = 720 hours`). Do not "fix" the 720 against 730 by arithmetic:
  both are AWS's own conventions for different purposes. `per_gb_month`
  multiplies by storage size or 1 GB. A billing mode the engine does not know that
  still has a usable unit, such as `per_hour_plus_data`, is priced from the unit
  alone and the note says `<mode>: hourly rate only, other charges not included`.
  A tiered spec is graduated, as finfocus-spec defines it
  (`docs/ADVANCED_PATTERNS.md`: usage in each tier is `min(usage, max) - min`; the S3
  example is "first 50 TB at $0.023, next 400 TB at $0.022"): each tier bills the part of
  the quantity inside it (`graduatedTierCost`). Tiers whose ranges do not cover the
  whole quantity (a gap, a first tier above zero, a bounded last tier below the
  quantity) are unusable and fall through, since the uncovered part has no rate. A `$0` rate stays a price, so a plugin must answer `zero_cost` or an error
  for "no price", not a zero rate (#1639 reports aws-public returning a zero
  `per_hour` rate for a type it does not find, which core prices as `$0`; unverified
  here, since it is a plugin-side issue)
- **Estimate TUI pricing discovery** calls `GetPricingSpec` when
  `cost estimate --interactive` starts. `DiscoverPricingSpec` caches by
  resource type, SKU, and region on the engine for that session, so an edited SKU
  asks the plugin again. The view lists each plugin
  billing mode, its tiers, assumptions, and usage hints. Left and right move
  between modes. `not_implemented`, an empty mode, and RPC errors hide that
  section and leave property editing in place. The lookup does not replace
  `EstimateCost`
- **Budget health thresholds**: OK (<80%), WARNING (80-89%), CRITICAL (90-100%),
  EXCEEDED (>100%). Aggregation uses worst-case status
- **Cache hits**: Append `(cached)`, including the leading space, to the Adapter field for visual feedback
- **Cache corruption**: Auto-detected and auto-recovered (delete + recreate)
- **Plugin TTL hints**: Plugins can set `expires_at` on responses to control per-entry
  cache TTL. The engine extracts `ExpiresAt` from `CostResult`, calls
  `cache.CalculatePluginTTL()`, and uses `SetWithTTL()` when a plugin hint is present.
  Past timestamps skip caching entirely. TTLs exceeding `MaxTTLSeconds` (604800 = 7 days)
  are capped. Debug logs record TTL overrides; warn logs record caps and skips
- **Optional LRU tier**: `cost.cache.lru_enabled` (default false) wraps the
  BoltDB cache with `cache.TieredStore` inside `initCacheFromConfig`, so every
  `newEngineWithCache` caller gets it. Reads hit memory first and promote a
  disk hit. Writes update disk, then refresh memory. Expired memory entries
  fall through. `lru_max_items` of 0 uses 256. `FINFOCUS_CACHE_LRU_ENABLED`
  and `FINFOCUS_CACHE_LRU_MAX_ITEMS` override the file. A wrap failure keeps
  the Bolt store. The engine field stays `cache.Cache`
- `checkPluginSupports` sends the descriptor `proto.PrepareProjectedDescriptor` builds
  (the same one `GetProjectedCost` sends, including a region inherited from a
  referenced resource), and caches per
  client+provider+type+region+sku+feature (SKU is part of the key: a first SKU-less
  resource in a region must not poison the cached answer for every other SKU there);
  plugins on finfocus-spec ≥ v0.6.2 answer `Supports` for real, so a region-bound plugin
  (aws-public) declines other regions instead of being called and failing. A plugin that
  never implements `SupportsProvider` gets the SDK's generic fallback response
  (`Supported:false`, `Reason: pluginsdk.DefaultSupportsNotImplementedReason`); the engine
  treats that reason as fail-open (cached `true`), same as an RPC error, so such a plugin
  isn't silently dropped from routing
- **Recommendations**: `convertProtoRecommendation` keeps the full plugin record
  (`ID`, category, priority, confidence, source, metadata, `ImpactDetail`, `ResourceInfo`,
  `ActionDetail`, primary/secondary reasons);
  `Recommendation.ID` is the dismissal ID. The cache key is
  `recommendations/multi/{types}/{hash}` where the hash covers resource id/provider/type/
  properties plus dismissed IDs (`cache.HashRecommendationInputs`). Resources are routed per
  plugin via `routeRecommendationTargets` (router feature `Recommendations`)
- **Recommendation scoring** (`internal/scoring`, opt-in via `scoring.*` config, off by default):
  `scoring.Service.Score` runs after the fetch in `cost recommendations`, sends recommendations
  to a scorer plugin (`PLUGIN_CAPABILITY_RECOMMENDATION_SCORING`, normalized
  `recommendation_scoring`) through `pbc.RecommendationScorerServiceClient`. Core applies
  `identifier_mode` (default pseudonymized: per-request HMAC key, opaque `rec-N` ids, raw ids
  scrubbed from text) and `field_allowlist` before sending; identifier handling also covers
  identifiers inside `action_detail` (Kubernetes cluster/namespace/controller/container names) and
  free text there; the first call probes batch size
  (20, halving on INVALID_ARGUMENT), later batches run up to 8 concurrent. Scorer failure only
  warns. Scores are cached in the `scores` bucket (extracted values only). Scores never dismiss
  or hide anything. Scorer-only plugins are skipped by `routeRecommendationTargets`

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

### Cluster allocation (`internal/engine/cluster*.go`, `plugins/kubernetes/`)

- **Core never interprets Kubernetes**: nodes arrive from `GetStats` as ordinary
  `ResourceDescriptor`s (`sku`/`region` passed as properties) and are priced by
  `GetProjectedCostWithErrors`; grouping is string-map aggregation
- **`$0` price = unpriced**: aws-public returns `$0` (not an error) for unknown
  instance types, so `priceResources` treats `Monthly <= 0` as unpriced
- **Conservation is enforced in core** (`VerifyConservation`, rel 1e-6, delegating to
  `pluginsdk.CheckConservation`) even though the conformance suite also checks it —
  third-party allocators exist, and one shared SDK rule keeps core and plugins in agreement
- **`plugins/kubernetes` is a nested module** and must not import core packages
  (`make check-plugin-boundaries`); it declines `Supports` so `cost projected`
  skips it (finfocus-spec ≥ v0.6.2 delivers plugin `Supports` answers to hosts;
  earlier SDKs errored and the engine failed open)
- **Allocator policy v2** (`specs/614-allocator-policy-v2/`): `idle` and
  `system_workloads` accept `share` as well as the default `separate`. Idle
  CPU and memory are shared in proportion to each workload's already-computed
  CPU cost and memory cost. System rows (`namespace=kube-system` or
  `controller_kind=DaemonSet`) are then folded into the other workloads on
  the same node. The idle row stays even at `$0`, because
  `ValidateAllocateResponse` requires exactly one idle row per priced node.
  When both fields are `share`, idle is distributed first (including onto
  system rows) and system rows are folded after that
- **EKS Fargate pods** (`specs/615-eks-fargate-pods/`): a node labeled
  `eks.amazonaws.com/compute-type=fargate` still has no capacity rows and no
  node descriptor. Each running pod on it is a priceable
  `aws:eks/fargate:Pod` (sku `fargate`, tags `kind=fargate`, `cpu`,
  `memory_gib`). The allocator assigns that priced cost to the pod and never
  folds it into `__idle__`. `Monthly <= 0` stays unpriced (`Fargate pod has
  no price`, or the pricer's note). A `fargate-` node name with no fargate
  priced entry still notes `Fargate pricing not supported yet`. Rates are
  aws-public's job
  ([finfocus-plugin-aws-public#409](https://github.com/rshade/finfocus-plugin-aws-public/issues/409));
  kind cannot simulate Fargate, so `make test-e2e-kind` does not cover it
- **Pulumi URN linking** (`specs/616-k8s-pulumi-urn-linking/`): the provider
  does not write a URN onto objects. Users set annotation
  `finfocus.dev/pulumi-urn` (not a label: URN values contain `:` and exceed
  63 characters). The collector copies it to subject
  `label.finfocus.dev/pulumi-urn`, which stats validation allows. `cost
  cluster --group-by pulumi-stack` groups by `<stack>/<project>`; a missing
  or malformed value is `<none>`. Group JSON includes `pulumi_urns`

### MCP Server (`internal/cli/mcp.go`, `internal/cli/output_mode.go`)

- **Per-call lifecycle**: ax-go's dispatcher re-executes the *shared* root once
  per `tools/call` (nested `ExecuteContext`), so root `PersistentPreRunE`/
  `PersistentPostRunE` run per call. `commandLifecycle.serving` marks dispatched
  calls: they reuse the server's `loggingSession` via `attach` and never close
  it. Opening/closing logging per call previously leaked handles and ended in
  `close .../finfocus.log: file already closed`. `loggingSession.Close` is
  idempotent. Config resolution still runs per call
- **Serialized dispatch**: one mutex covers every call, so a blocking command
  stalls the whole server. That is why `analyzer serve` is excluded
- **Output resolver**: `resolveOutputFormat` gives an explicitly set `--output`
  first priority, then `--format` (the dispatcher injects `json` on every call),
  then `AGENT_MODE`, resolved via `ax.ResolveMode(..., stdoutIsTTY=true)`, else the
  flag default. It deliberately ignores `ax.ModeFromContext` and TTY detection:
  those report JSON whenever stdout is piped, which would flip redirected human
  output. New commands with an output flag must call it,
  and commands without one should emit JSON when `machineOutputRequested(cmd)`
- **Flag stickiness**: the dispatcher resets flag values *and* `Changed` per
  call, but only on a real server. Nested `ExecuteContext` in unit tests does
  not reset, so assert cross-call `--output` behavior in the integration test
- **Exclusion**: `applyMCPExclusions` marks `mcpExcludedCommands` (root,
  `analyzer serve`, `cost history collect` (minutes-long, would stall the serialized
  dispatcher), `setup`, `plugin init`) with ax-go's node-only `mcp.Exclude`
  at tree build time; they stay in `--help`. ax-go (v0.7.0+) skips `help` and
  non-runnable groups itself. Never use `Hidden` to exclude a root/group: ax-go
  prunes the whole subtree. `withPositionalCommandsHidden` still hides
  positional-arg leaves while an MCP tool list is built so the static
  `__schema --as=mcp` matches the live server, which cannot pass positional args
- **Tool allow-list golden**: `internal/cli/testdata/mcp/tools.golden` pins the
  tool list for both the static schema (unit) and live `tools/list` of both
  entry points (`test/integration/mcp_server_test.go`). A new command must make
  an expose-or-exclude decision; regenerate with `UPDATE_GOLDEN=1`
- **Version**: the MCP handshake rejects `dev`/`unknown` versions, so binaries
  built for MCP tests need `-ldflags -X .../pkg/version.version=...`

### Plugin Upgrade (`internal/pluginupgrade/`, `agent-skills/finfocus-plugin-upgrade/`)

- **Bumping finfocus-spec in core needs a hop**: `TestHopsReachCoreSpecVersion`
  fails until `hops.go` has an entry whose `To` is at least
  `pluginsdk.SpecVersion`, and `TestHopsMatchGuides` requires a matching
  `references/to-vX.Y.Z.md`. Add both in the bump PR, even for an additive
  release (a guide that says "no changes required" is fine)
- **Hops are releases that need author action, not minor lines**: v0.5.7 (a
  patch) changed `HandleDryRun`; v0.6.1 raised Go to 1.27.1
- **`SpecVersion` must be `v`-prefixed**: `pluginsdk.ValidateSpecVersion`
  rejects `"0.6.1"`, so `GetPluginInfo` returns `Internal` and `plugin list`
  shows `N/A`. `plugin init` still writes the bare form (#248); the upgrade
  rewrites it
- **Minimum supported version is v0.5.0**; there are no pre-rename plugins
- Rebuild `agent-skills/finfocus-plugin-upgrade/finfocus-plugin-upgrade.skill`
  (a zip of `SKILL.md` + `references/`) after editing either

### Integration Tests (`test/integration/`)

- **The CLI helper must go through `ax.Execute`**: `helpers.CLIHelper.Execute`
  runs commands via `ax.Execute`, the same entry point as `cmd/finfocus`. Calling
  `cli.NewRootCmd().Execute()` directly does NOT mount the persistent agentic flags
  (`--format`, `--dry-run`, `--yes`, `--idempotency-key`), so those flags fail with
  "unknown flag" in integration tests while working in production and in unit tests
  (which use `axtest.Run`). `ax.Execute` returns an exit code rather than an error,
  so the helper converts non-zero codes back into an error from stderr
- **Many integration tests do not isolate `HOME`**: tests that call
  `helpers.NewCLIHelper(t)` without `WithEnv` read the developer's real
  `~/.finfocus/config.hujson`, so local config can change their results. (A local
  `cost.budgets` entry used to inject a "BUDGET STATUS" banner into JSON/NDJSON
  output and fail ~57 tests locally; the banner now renders only for table output.)
  Always diff the integration failure *set* against a pre-change baseline rather
  than comparing failure counts
- **`config.yaml` fixtures are intentional**: most `config.yaml` references in
  `test/integration/` write legacy YAML as *input* to exercise auto-migration. Only
  assertions that `config init` *creates* a file should expect `config.hujson`

## Recent Changes

- 608-batch-cost-consumer: Added Go 1.27.1 (see `go.mod`) + finfocus-spec v0.6.0 (proto definitions with `BatchCost` RPC), Cobra (CLI), gRPC, zerolog (logging)
- 608-resource-history-store: Added Go 1.27.1 (see `go.mod`) + BoltDB (`go.etcd.io/bbolt` — already in `go.mod`).
- 608-estimate-cost-rpc: Added Go 1.27.1 (see `go.mod`) + finfocus-spec v0.6.0 (proto definitions), gRPC, Cobra, zerolog
- 607-state-only-flag: Added Go 1.27.1 (see `go.mod`) + Cobra (CLI framework), Bubble Tea (TUI), zerolog (logging)
- 606-cache-expires-at: Added Go 1.27.1 (see `go.mod`) + finfocus-spec v0.6.0 (provides `expires_at` proto fields), BoltDB (cache storage), zerolog (logging)

## Active Technologies

- Go 1.27.1 (see `go.mod`) + finfocus-spec v0.6.0 (proto definitions with `BatchCost` RPC), Cobra (CLI), gRPC, zerolog (logging) (608-batch-cost-consumer)
- BoltDB cost cache (`~/.finfocus/cache/cache.db`) — `Engine.GetProjectedCost` caches batch projected results per-resource via `storeProjectedCostCache`; `Engine.GetActualCostWithOptions` caches actual-cost results by full request key via `storeActualCostCacheIfClean` (608-batch-cost-consumer)
- Go 1.27.1 (see `go.mod`) + BoltDB (`go.etcd.io/bbolt` — already in `go.mod`) (608-resource-history-store)
- BoltDB at `~/.finfocus/history/history.db` (separate from cache) (608-resource-history-store)
- Go 1.27.1 (see `go.mod`) + finfocus-spec v0.6.0 (proto definitions), gRPC, Cobra, zerolog (608-estimate-cost-rpc)
- N/A (no new persistent state) (608-estimate-cost-rpc)
