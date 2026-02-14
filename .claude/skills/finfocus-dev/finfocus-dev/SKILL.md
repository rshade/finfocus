---
name: finfocus-dev
description: >
  FinFocus Core development workflow guide for contributors. Use when implementing features,
  fixing bugs, writing tests, running CI checks, or following project conventions in the
  finfocus Go codebase. Triggers on: adding CLI commands, writing Go tests, running make
  targets, debugging CI failures, understanding project structure, or any Go development
  task within finfocus-core.
---

# FinFocus Development Workflow

## Quick Reference

```bash
make build          # Build binary to bin/finfocus
make test           # Unit tests (fast, default)
make lint           # golangci-lint + markdownlint (5+ min, use extended timeout)
make validate       # go mod tidy + go vet
make test-race      # Race detector
make test-integration  # Cross-component tests (10min timeout)
make build-all      # Build binary + all plugins
```

**Critical rules**: Always run `make lint` and `make test` before claiming success.
Never run `git commit` (user commits manually). Never modify `.golangci.yml`.

## Adding a CLI Command

1. Create `internal/cli/your_command.go`
1. Follow constructor pattern:

```go
func NewYourCmd() *cobra.Command {
    var flagVar string
    cmd := &cobra.Command{
        Use:   "your-command",
        Short: "Description",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Use cmd.Printf() not fmt.Printf()
            return nil
        },
    }
    cmd.Flags().StringVar(&flagVar, "flag", "", "description")
    return cmd
}
```

1. Register in parent command (e.g., `root.go` or `cost.go`)
1. Use `RunE` (not `Run`) for error handling
1. Defer cleanup immediately: `defer cleanup()`

## Resource Processing Pipeline

All cost commands follow:

```text
ingest.LoadPulumiPlan(path) -> ingest.MapResources() -> registry.Open(ctx, adapter)
  -> engine.GetProjectedCost/GetActualCost() -> engine.RenderResults(format, results)
```

## Testing Standards

- Use `testify/assert` and `testify/require` exclusively (never manual `if/t.Errorf`)
- `require.*` for setup that must succeed; `assert.*` for value checks
- Table-driven tests for variations
- Package suffix `_test` for black-box testing
- `t.TempDir()` for temporary files (auto-cleanup)
- Capture output: `cmd.SetOut(&buf)` and `cmd.SetErr(&buf)`
- Target 80% coverage minimum, 95% for critical paths

See [references/testing-patterns.md](references/testing-patterns.md) for detailed
test patterns and examples.

## Project Structure

See [references/project-structure.md](references/project-structure.md) for the
complete package map and key file locations.

## Error Handling

- Wrap errors: `fmt.Errorf("context: %w", err)`
- Return early on errors
- Plugin failures don't stop processing (graceful degradation)
- Validation prefix: `"VALIDATION: %v"`, plugin error prefix: `"ERROR:"`

## Logging (zerolog)

```go
log := logging.FromContext(ctx)
log.Debug().Ctx(ctx).Str("component", "engine").Msg("message")
```

Standard fields: `trace_id`, `component`, `operation`, `duration_ms`.
Enable debug: `--debug` flag or `FINFOCUS_LOG_LEVEL=debug`.

## Date Handling

Support both `"2006-01-02"` and RFC3339. Default `--to` to `time.Now()`.
Validate ranges (to must be after from).

## Output Formats

Three formats via `--output` flag: `table` (default), `json`, `ndjson`.
Always use `cmd.Printf()` for testability.
