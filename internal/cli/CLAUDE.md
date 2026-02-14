# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## CLI Package Overview

The `internal/cli` package implements the Cobra-based command-line interface for FinFocus. It follows a hierarchical command structure with two main command groups: `cost` (for cost calculations) and `plugin` (for plugin management).

## Command Architecture

### Command Hierarchy

```text
finfocus [--project-dir DIR] [--debug]
├── cost
│   ├── projected       # Calculate estimated costs from Pulumi plan
│   ├── actual          # Fetch historical costs from cloud providers
│   ├── recommendations # Get cost optimization recommendations
│   └── estimate        # Quick cost estimate
├── config
│   ├── init [--global] [--force]  # Initialize config (project-local or global)
│   ├── set             # Set configuration values
│   ├── get             # Get configuration values
│   ├── list            # List all configuration
│   └── validate        # Validate configuration
├── plugin
│   ├── init       # Initialize a new plugin project
│   ├── install    # Install a plugin
│   ├── update     # Update an installed plugin
│   ├── remove     # Remove an installed plugin
│   ├── list       # List installed plugins
│   ├── validate   # Validate plugin installations
│   └── certify    # Run certification tests for a plugin
└── analyzer
    └── serve      # Starts the FinFocus analyzer gRPC server (usually run by Pulumi CLI)
```

### Command Implementation Pattern

Each command follows this consistent pattern:

1. **Constructor Function**: `NewXxxCmd()` returns a `*cobra.Command`
2. **Flag Variables**: Declared as local variables in the constructor
3. **RunE Function**: Contains the main command logic
4. **Flag Registration**: Uses `cmd.Flags().StringVar()` for configuration
5. **Required Flags**: Marked with `cmd.MarkFlagRequired()`

### `analyzer serve` Integration Pattern

The `finfocus analyzer serve` command is unique as it's primarily designed for integration with the Pulumi CLI, not for direct user execution.

1.  **Launched by Pulumi**: The Pulumi CLI launches this command as a gRPC server process, typically configured in `Pulumi.yaml`.
2.  **Port Handshake**: The server prints its dynamically assigned listening port to stdout for the Pulumi CLI to connect.
3.  **Logging**: All diagnostic and operational logs are directed to stderr to avoid interfering with the stdout port handshake.
4.  **Graceful Shutdown**: The server handles `SIGINT`/`SIGTERM` signals for graceful shutdown when the Pulumi CLI terminates the connection.

This indirect execution pattern is crucial for its "zero-click" cost estimation functionality during `pulumi preview`.

### Core Dependencies Flow

```text
CLI Command → Ingest (Pulumi JSON) → Registry (Plugin Discovery) → Engine (Cost Calculation) → Output
```

Key imports:

- `internal/ingest`: Loads and maps Pulumi resources
- `internal/registry`: Discovers and opens plugin connections
- `internal/engine`: Orchestrates cost calculation
- `internal/config`: Manages configuration paths
- `internal/spec`: Loads local pricing specifications

## Testing Commands

Run tests for the CLI package:

```bash
go test ./internal/cli/...
go test -v ./internal/cli/... -run TestCLIIntegration  # Integration tests
go test -race ./internal/cli/...                        # With race detection
```

## Common Development Tasks

### Adding a New Command

1. Create new file: `internal/cli/your_command.go`
2. Follow the constructor pattern with `NewYourCmd() *cobra.Command`
3. Use `RunE` for error handling (not `Run`)
4. Always use `cmd *cobra.Command` as first parameter when command needs access to output streams
5. Add command to parent in `root.go` or appropriate parent command

### Resource Processing Pipeline

All cost commands follow this pipeline:

1. Load Pulumi plan: `ingest.LoadPulumiPlan(planPath)`
2. Map resources: `ingest.MapResources(pulumiResources)`
3. Open plugins: `registry.Open(ctx, adapter)`
4. Calculate costs: `engine.GetProjectedCost()` or `engine.GetActualCost()`
5. Render output: `engine.RenderResults(outputFormat, results)`

### Plugin Management Pattern

Plugin commands interact with the registry:

1. Check plugin directory exists: `os.Stat(cfg.PluginDir)`
2. List plugins: `registry.ListPlugins()`
3. Validate plugins: Custom validation logic in `ValidatePlugin()`
4. Display results: Use `tabwriter` for formatted output

## Critical Patterns and Conventions

### Error Handling

- Use `RunE` not `Run` for commands
- Wrap errors with context: `fmt.Errorf("action: %w", err)`
- Return early on errors, don't continue processing

### Resource Cleanup

- Always defer cleanup functions immediately after obtaining resources:

  ```go
  clients, cleanup, err := reg.Open(ctx, adapter)
  if err != nil {
      return fmt.Errorf("opening plugins: %w", err)
  }
  defer cleanup()
  ```

### Command Output

- Use `cmd.Printf()` and `cmd.Println()` for command output (not `fmt.Printf`)
- Use `tabwriter` for table formatting with consistent padding
- Support multiple output formats: table, json, ndjson

### Error Display Pattern

Cost commands display errors in two places:

1. **Inline in Results**: Errors appear in the Notes column with "ERROR:" prefix
2. **Summary After Table**: Aggregated error summary displayed after results

```go
// Display error summary after results
if resultWithErrors.HasErrors() {
    cmd.Println() // Blank line before summary
    cmd.Println("ERRORS")
    cmd.Println("======")
    cmd.Print(resultWithErrors.ErrorSummary())
}
```

This dual display ensures users see both individual resource failures and overall error statistics.

### Date/Time Handling

- Support multiple formats: "2006-01-02", RFC3339
- Default `--to` to `time.Now()` if not provided
- Validate date ranges (to must be after from)

### Testing Patterns

- Use `cli_test` package for black-box testing
- Create temporary directories with `t.TempDir()`
- Mock Pulumi plans as JSON structures
- Test both success and error cases
- Use `cmd.SetOut()` and `cmd.SetErr()` to capture output

### Flag Conventions

- Required flags: `--pulumi-json` for all cost commands
- Optional adapter override: `--adapter`
- Output format: `--output` (table|json|ndjson)
- Global debug flag: `--debug` on root command
- Project directory: `--project-dir` on root command (persistent, available to all subcommands)

### Project Directory Resolution

The `--project-dir` persistent flag overrides automatic Pulumi project detection.
It is wired in `PersistentPreRunE` of the root command:

1. `config.ResolveProjectDir(cmd.Context(), projectDirFlag, cwd)` resolves the project `.finfocus/` path
2. `config.SetResolvedProjectDir(resolvedDir)` stores it for the session
3. `config.InitGlobalConfigWithProject(cmd.Context(), resolvedDir)` loads merged config

This affects `config init` (creates project-local config), `NewDismissalStore`
(stores dismissals per-project), and all config getters via `GetGlobalConfig()`.

### Config Init Behavior

`config init` is project-aware:

- **Inside Pulumi project** (without `--global`): creates `$PROJECT/.finfocus/config.yaml`
  and `$PROJECT/.finfocus/.gitignore`
- **With `--global`**: always creates `~/.finfocus/config.yaml`
- **Outside Pulumi project**: falls back to global init
- `--force`: overwrites existing `config.yaml` but never overwrites `.gitignore`

### DismissalStore Resolution

`loadDismissalStore()` calls `config.NewDismissalStore("")` which uses:

1. `GetResolvedProjectDir()` if set → `$PROJECT/.finfocus/dismissed.json`
2. `ResolveConfigDir()` fallback → `~/.finfocus/dismissed.json`

This means dismissals are automatically project-scoped when running inside a Pulumi project.

## Common Gotchas

1. **Plugin Directory**: Commands gracefully handle missing plugin directory
2. **Empty Plugin List**: Don't error when no plugins installed, show informative message
3. **Date Parsing**: Try multiple formats before failing
4. **Executable Check**: Use `info.Mode()&0111` to check if file is executable
5. **Manifest Validation**: Optional but validated if present
6. **Output Streams**: Always use `cmd.Printf()` not `fmt.Printf()` for testability

## Integration Points

- **Config Package**: Uses `config.New()` for default paths (~/.finfocus/specs, ~/.finfocus/plugins)
- **Registry Package**: `registry.NewDefault()` creates standard plugin registry
- **Engine Package**: Handles both projected and actual cost calculations
- **Proto Adapter**: Real protobuf definitions from finfocus-spec repository

