# Repository Guidelines

## Project Snapshot

- Go CLI for finfocus in `cmd/finfocus` with unexported logic in `internal/`.
- Fixtures live in `examples/`, `testdata/`, and `test/e2e/fixtures/`.
- Docs are in `docs/`; scripts in `scripts/`; build artifacts in `bin/`.

## Build, Lint, and Test Commands

- `make build`: Build the `finfocus` binary into `bin/` with version metadata.
- `make run` / `make dev`: Build then run the CLI; `make inspect` launches MCP inspector.
- `make test` | `make test-race`: Run unit tests (optionally with race detector).
- `go test ./...`: Run the full Go test suite.
- `go test -run TestName ./path/to/package`: Run a single test in one package.
- `go test -v ./... -run TestFunc`: Run a single test across all packages.
- `make lint`: Run `golangci-lint` v2.6.2 plus `markdownlint`.
- `make validate`: Run `go mod tidy -diff` and `go vet` checks.
- `make docs-lint`: Lint docs when editing Markdown.

## Go Formatting and Imports

- Go 1.25.5+; use tabs for indentation and run `gofmt` on Go files.
- Imports grouped as: standard library, third-party, internal packages.
- `goimports`/`golines` enforced via `golangci-lint`; keep lines tidy.
- Avoid `init()` and global variables (lint rule).
- `//nolint:lintername` only when required and with justification.

## Types, Naming, and API Surface

- Package names lowercase and short (`engine`, `config`, `pluginhost`).
- Custom domain types preferred (`type Duration time.Duration`).
- Exported identifiers require Go doc comments when part of CLI surface.
- Struct tags use JSON/YAML tags (`yaml:"field_name"`).
- CLI flags are kebab-case; env/config keys uppercase snake (`FINFOCUS_*`).
- Define interfaces before implementations; keep interfaces small.
- Pass `context.Context` through request lifecycles.

## Error Handling and Control Flow

- Wrap errors with `%w`: `fmt.Errorf("operation failed: %w", err)`.
- Sentinel errors are `var ErrName = errors.New("description")`.
- Validate inputs early and return descriptive errors.
- Prefer early returns to reduce nesting.
- Use context cancellation and return partial results where sensible.

## Logging Expectations

- Use `internal/logging` for structured logging.
- Fetch logger from context: `log := logging.FromContext(ctx)`.
- Include `component` and `operation` fields for traceability.
- Use `Debug` for detailed flow, `Info` for milestones, `Warn` for recoverable issues.

## Testing Guidelines

- Table-driven tests with clear `wantErr` / `errContains` fields.
- Test both success and failure paths, especially file I/O and validation.
- Use fixtures from `testdata/` or `examples/` over large literals.
- Use integration tests in `examples/` for HTTP/CRUD flows.
- Plugins: add conformance coverage in `internal/conformance` and targeted tests in
  `internal/engine` or `internal/registry`.

## Testify Assertions (Required)

**CRITICAL**: All Go tests must use `testify/assert` and `testify/require`.

Required imports:

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

Use `require.*` for setup failures and nil checks, `assert.*` for value comparisons.

## Documentation and Markdown

- Docs use frontmatter with `title`, `description`, and `layout`.
- Frontmatter `title` is the H1; content must start with H2 or body text.
- Run `make docs-lint` after Markdown edits.
- `markdownlint` runs as part of `make lint`.

## Commit and PR Guidance

- Conventional Commits required (`feat:`, `fix:`, `chore:`...).
- PRs include summary, linked issues, and a test plan (e.g., `make test`).
- Avoid bundling unrelated changes; call out breaking changes.
- Run `make lint` and `make test` before committing.

## Security and Configuration

- Never commit secrets; use `FINFOCUS_PLUGIN_*` env vars or `~/.finfocus/config.yaml`.
- Plugins live in `~/.finfocus/plugins/<name>/<version>/`.
- Validate plugins with `finfocus plugin validate` and `finfocus plugin certify`.
- Scrub Pulumi plan fixtures if identifiers appear.

## Common Workflows

### Adding Resource Types

1. Add resource type to `internal/engine/types.go`.
2. Implement validation in the resource `Validate()` method.
3. Add pricing data in `specs/` or via plugin support.
4. Add unit tests in `internal/engine/types_test.go`.
5. Add integration tests in `internal/conformance/`.

### Plugin Development

- `finfocus plugin init` scaffolds new plugins.
- Implement the protocol from finfocus-spec.
- Install to `~/.finfocus/plugins/<name>/<version>/`.

## Current Stack

- Go 1.25.5 with `github.com/Masterminds/semver/v3` and finfocus plugin SDK.
- charmbracelet/lipgloss v1.0.0 and golang.org/x/term v0.37.0.
- Plugin directory: `~/.finfocus/plugins/<plugin-name>/<version>/`.

## Cursor/Copilot Rules

- No `.cursor/rules/`, `.cursorrules`, or `.github/copilot-instructions.md` found.
