# Repository Guidelines

## Project Structure

- `cmd/finfocus`: CLI entrypoint and flag wiring.
- `internal/` packages: core logic (engine, ingest, registry, pluginhost,
  config, logging, analyzer) kept unexported.
- `pkg/version`: shared version/build metadata used by the CLI.
- `examples/` and `testdata/`: Pulumi plan fixtures and sample specs; prefer
  extending these for reproducible tests.
- `test/e2e/fixtures/`: Real Pulumi project fixtures for E2E tests.
- `docs/`: Jekyll site and contributor docs; `scripts/` contains helper
  tooling; `bin/` is populated by builds.

## Build, Test, and Development Commands

- `make build`: Compile the `finfocus` binary to `bin/` with version metadata.
- `make test` | `make test-race`: Run unit tests (optionally with race detector).
- `go test -run TestName ./path/to/package`: Run a specific test in a package.
- `go test -v ./... -run TestFunc`: Run a specific test function across all packages.
- `make lint`: Run `golangci-lint` v2.6.2 plus `markdownlint` (expects
  `AGENTS.md` to pass).
- `make validate`: Run `go mod tidy -diff` and `go vet` to verify module state
  and static checks.
- `make run` / `make dev`: Build then run the CLI; use `make inspect` to launch
  the MCP inspector after a build.

## Code Style Guidelines

### Formatting & Imports

- Go 1.25.5+, tabs for indentation. Format with `gofmt`; imports and line
  length enforced by `goimports`/`golines` via `golangci-lint`.
- Import order: standard library, third-party, internal. Group internal imports together.
- No `init()` functions or global variables (enforced by golangci-lint).
- Use `//nolint:lintername` directives sparingly; include explanation when used.

### Error Handling

- Wrap errors with `fmt.Errorf("%w", err)` for context preservation.
- Define sentinel errors as `var ErrName = errors.New("description")`.
- Return errors with context: `return fmt.Errorf("operation failed: %w", err)`.
- Validate inputs early and return descriptive errors for validation failures.
- Use early returns for error checks; avoid deep nesting.

### Types & Naming

- Package names: lowercase and succinct (e.g., `engine`, `config`, `pluginhost`).
- Custom types for domain values: `type Duration time.Duration`,
  `type ContextKey string`.
- Exported identifiers require Go doc comments when part of the CLI surface.
- Struct fields: use JSON/YAML tags for serialization (`yaml:"field_name"`).
- CLI flags: kebab-case (`--pulumi-json`); config/env keys: uppercase snake
  (`FINFOCUS_PLUGIN_*`).
- Interface definitions before implementations; prefer small interfaces.
- Use `context.Context` throughout for cancellation and timeout handling.

### Testing Patterns

- Use testify/assert and testify/require for assertions.
- Keep table-driven tests focused; avoid redundant test cases.
- Test both success and error paths for all public functions.
- Use fixtures from `testdata/` instead of embedding large data structures.

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

**When to Use `assert.*` (continues test on failure)**:

- Value comparisons after setup is complete
- Multiple property checks on a result
- Non-critical validations where seeing all failures is helpful

**Common Assertion Conversions**:

| Manual Pattern                                   | Testify Replacement           |
| ------------------------------------------------ | ----------------------------- |
| `if err != nil { t.Fatal(err) }`                 | `require.NoError(t, err)`     |
| `if err == nil { t.Error("expected error") }`    | `require.Error(t, err)`       |
| `if x != y { t.Errorf("got %v, want %v", x, y) }`| `assert.Equal(t, y, x)`       |
| `if len(x) != n { t.Errorf(...) }`               | `assert.Len(t, x, n)`         |
| `if !strings.Contains(s, sub) { t.Errorf(...) }` | `assert.Contains(t, s, sub)`  |
| `if x == nil { t.Fatal("nil") }`                 | `require.NotNil(t, x)`        |

## Documentation Standards

- Run `make docs-lint` before committing documentation changes
- Use frontmatter YAML with `title`, `description`, and `layout` fields
- **CRITICAL**: Files with frontmatter must NOT have duplicate H1 - the frontmatter
  `title` serves as the page H1, content should start with H2 or text

## Testing Guidelines

- Author `_test.go` files with `TestXxx`/`BenchmarkXxx`; keep table-driven tests
  near the code they cover.
- Use `go test ./...` (optionally `-cover` or `-race`) before submitting; add
  fixtures to `examples/` or `testdata/` instead of embedding large literals.
- When adding plugins or adapters, include conformance coverage in
  `internal/conformance` and targeted cases in `internal/engine` or
  `internal/registry`.
- **Error path testing**: Always test error conditions—every error return should
  have a corresponding test. Use table-driven tests with `wantErr`/`errContains`
  fields. Priority: file I/O, network, validation, and resource cleanup errors.
- **Unit Testing Best Practices**: Focus on pure transformation functions,
  stateless logic, and simple methods. Avoid unit testing CRUD operations
  requiring HTTP clients—test those as integration tests in `examples/`.
  Don't mock dependencies that don't provide interfaces. Use testify/assert
  and testify/require for assertions.

## Commit & Pull Request Guidelines

- Commit messages follow Conventional Commits (`feat:`, `fix:`, `chore:`…) as
  enforced by `commitlint.config.js`; keep them scoped and imperative.
- PRs should include: a concise summary, linked issues, a short test plan (e.g.,
  `make test`, `make lint`, `make validate`), and CLI output or screenshots when
  user-facing behavior changes.
- Avoid bundling unrelated changes; keep docs and code changes cohesive. Flag
  breaking changes explicitly in the PR description.
- Always run `make lint`, and `make test` before committing changes.

## Security & Configuration Tips

- Do not commit secrets; prefer env vars (`FINFOCUS_PLUGIN_AWS_*`,
  `FINFOCUS_PLUGIN_VANTAGE_*`, etc.) and `~/.finfocus/config.yaml` for local
  config.
- Plugins live under `~/.finfocus/plugins/`; validate with
  `finfocus plugin validate` before shipping.
- Treat Pulumi plan JSON files as sensitive if they contain identifiers; scrub
  or use redacted fixtures in examples.

## Common Development Patterns

### Plugin Development

- Use `finfocus plugin init` to scaffold new plugin projects
- Implement the plugin protocol from finfocus-spec
- Test plugins with `finfocus plugin certify` before shipping
- Install to `~/.finfocus/plugins/<name>/<version>/`

### Adding New Resource Types

1. Add resource type to `internal/engine/types.go`
2. Implement validation logic in the resource's `Validate()` method
3. Add pricing data to `specs/` or implement plugin support
4. Write unit tests in `internal/engine/types_test.go`
5. Add integration tests in `internal/conformance/`

### Error Recovery

- Use context cancellation for timeouts: `ctx, cancel :=
context.WithTimeout(ctx, timeout)`
- Log warnings for non-critical failures but continue processing
- Return partial results when possible instead of failing completely
- Always check context cancellation in loops: `if err := ctx.Err(); err != nil {
return }`

### Logging Patterns

- Use structured logging from `internal/logging` package
- Retrieve logger from context: `log := logging.FromContext(ctx)`
- Include component and operation fields for traceability:
  `Str("component", "engine")`
- Use appropriate log levels: Debug for detailed flow, Info for key events,
  Warn for recoverable issues, Error for failures
- Always include context in log calls: `Ctx(ctx)`

## Active Technologies

- Go 1.25.5 + github.com/Masterminds/semver/v3, existing plugin
  infrastructure (already in go.mod, 001-latest-plugin-version)
- File system (plugin directory structure:
  `~/.finfocus/plugins/<plugin-name>/<version>/`)
- Go 1.25.5 + existing CLI infrastructure, test helpers
- Go 1.25.5 + pluginsdk from finfocus-spec
- Environment variable constants via pluginsdk
- Go 1.25.5 + charmbracelet/lipgloss v1.0.0, golang.org/x/term v0.37.0

## Recent Changes

- 001-latest-plugin-version: Added Go 1.25.5 +
  github.com/Masterminds/semver/v3, existing plugin infrastructure
