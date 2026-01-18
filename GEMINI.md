# finfocus Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-01-12

## Active Technologies
- Go 1.25.5 + `github.com/spf13/cobra` (CLI), `github.com/spf13/viper` (Config), `github.com/rshade/finfocus-spec` (renamed from `finfocus-spec`) (113-rebrand-to-finfocus)
- Filesystem (`~/.finfocus/config.yaml`, `~/.finfocus/plugins/`) (113-rebrand-to-finfocus)
- Filesystem (Plugin directories) (115-v021-dx-improvements)

- Markdown, Go 1.25.5 (for code verification) + Jekyll (for docs site), GitHub Pages (010-sync-docs-codebase)
- Git repository (docs folder) (010-sync-docs-codebase)
- Pulumi Analyzer integration (finfocus analyzer serve)
- Plugin management commands (finfocus plugin init/install/update/remove)
- GitHub Actions, `gh` CLI, OpenCode CLI/API (019-nightly-failure-analysis)
- Go 1.25.5 + `github.com/stretchr/testify` (assertions), `net/http/httptest` (mocking) (021-plugin-integration-tests)
- Filesystem (mocked via `t.TempDir()`) (021-plugin-integration-tests)
- Go 1.25.5 + `github.com/spf13/cobra` (CLI), `github.com/spf13/pflag` (023-add-cli-filter-flag)
- Pure Go (no external dependencies for filter logic) (023-add-cli-filter-flag)
- Go 1.25.5 + `github.com/Masterminds/semver/v3` (001-latest-plugin-version)
- Filesystem (`~/.finfocus/plugins/`) (001-latest-plugin-version)
- Go 1.25.5 + github.com/rshade/finfocus-spec v0.4.14, github.com/Masterminds/semver/v3 (112-plugin-info-discovery)

- Local Pulumi state (ephemeral), no persistent DB (Stateless operation). (008-e2e-cost-testing, 009-analyzer-plugin, 112-plugin-info-discovery)

- Go 1.25.5
- `github.com/rshade/finfocus-spec`
- `google.golang.org/grpc` (002-implement-supports-handler)

## Project Structure

```text
cmd/
internal/ (now includes analyzer package)
pkg/
test/
testdata/
```

## Commands

```bash
# Build
make build

# Test
make test

# Lint
make lint

# Run
make run

# Plugin Management
finfocus plugin init
finfocus plugin install
finfocus plugin update
finfocus plugin remove

# Analyzer
finfocus analyzer serve
```

## Code Style

Go 1.25.5: Follow standard conventions

## Documentation Standards

- Run `make docs-lint` before committing documentation changes
- Use frontmatter YAML with `title`, `description`, and `layout` fields
- **CRITICAL**: Files with frontmatter must NOT have duplicate H1 - the frontmatter
  `title` serves as the page H1, content should start with H2 or text

## Testing Infrastructure

### Fuzz Tests

Parser resilience testing using Go's native fuzzing (Go 1.25+):

```bash
# JSON parser fuzzing
go test -fuzz=FuzzJSON$ -fuzztime=30s ./internal/ingest

# YAML parser fuzzing
go test -fuzz=FuzzYAML$ -fuzztime=30s ./internal/spec
```

**Locations:**

- `internal/ingest/fuzz_test.go` - JSON parser fuzz tests
- `internal/spec/fuzz_test.go` - YAML spec fuzz tests

### Performance Benchmarks

Scalability testing with synthetic data:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./test/benchmarks/...

# Scale tests (1K, 10K, 100K resources)
go test -bench=BenchmarkScale -benchmem ./test/benchmarks/...
```

**Locations:**

- `test/benchmarks/scale_test.go` - Scale benchmarks
- `test/benchmarks/generator/` - Synthetic data generator

**Performance targets:**

- 1K resources: < 1s (actual: ~13ms)
- 10K resources: < 30s (actual: ~167ms)
- 100K resources: < 5min (actual: ~2.3s)

### Validation Tests

Configuration validation with >85% coverage:

- `internal/config/validation_test.go` - Table-driven validation tests

### Error Path Testing

**Always test error conditions when writing new code:**

1. Test every error return path
2. Validate error messages are descriptive
3. Test boundary conditions (empty, nil, invalid ranges)
4. Test partial failures in batch operations
5. Test resource cleanup on errors

**Pattern**: Use table-driven tests with `wantErr` and `errContains` fields.

**Priority paths**: File I/O, network, validation, resource exhaustion, concurrency.

### CI Integration

- PRs: 30-second fuzz smoke tests, benchmark smoke tests
- Nightly: 6-hour deep fuzzing, full benchmark suite, cross-platform matrix

<!-- MANUAL ADDITIONS START -->

## Workflow Restrictions

- **NEVER COMMIT**: Do not execute `git commit`. Always stop after `git add` and ask the user to review/commit.
<!-- MANUAL ADDITIONS END -->

## Integration & Testing

- **E2E Testing**: Uses `pulumi preview --json` against real AWS infrastructure (ephemeral stacks). Project fixtures are located in `test/e2e/fixtures/`.
- **Pulumi Plan JSON**: The structure of `pulumi preview --json` output nests resource state under `newState` (for creates/updates). Ingest logic MUST check `newState` to correctly extract `Inputs` and `Type`.
- **Resource Type Compatibility**: Plugins must handle Pulumi-style resource types (e.g., `aws:ec2/instance:Instance`) or Core must normalize them. Currently, plugins are expected to handle the mapping.
- **Property Extraction**: Core (`adapter.go`) relies on populated `Inputs` to extract SKU and Region. If `Inputs` are empty (due to ingest issues), pricing lookup fails.

## Recent Changes
- 115-v021-dx-improvements: Added Go 1.25.5
- 113-rebrand-to-finfocus: Added Go 1.25.5 + `github.com/spf13/cobra` (CLI), `github.com/spf13/viper` (Config), `github.com/rshade/finfocus-spec` (renamed from `finfocus-spec`)

- 112-plugin-info-discovery: Added Go 1.25.5 + github.com/rshade/finfocus-spec v0.4.14, github.com/Masterminds/semver/v3

  plus Jekyll (for docs site), GitHub Pages
  plan JSON.

## Session Analysis - Recommended Updates

Based on recent development sessions, consider adding:

### Go Version Management

- **Version Consistency**: When updating Go versions, update both `go.mod` and ALL markdown files simultaneously
- **Search Pattern**: Use `grep "Go.*1\." --include="*.md"` to find all version references in documentation
- **Files to Check**: go.mod, all .md files in docs/, specs/, examples/, and root-level documentation
- **Docker Images**: Update Docker base images (e.g., `golang:1.24` → `golang:1.25.5`) in documentation examples

### Systematic Version Updates

- **Process**: 1) Update go.mod first, 2) Find all references with grep, 3) Update each file systematically, 4) Verify with final grep search
- **Common Patterns**: Update both specific versions (1.24.10 → 1.25.5) and minimum requirements (Go 1.24+ → Go 1.25.5+)
- **CI Workflows**: Update GitHub Actions go-version parameters in documentation examples

This ensures complete version consistency across the entire codebase and documentation.

## AI Agent File Maintenance

This file (GEMINI.md) provides guidance for Gemini AI assistants. To maintain its effectiveness:

### Update Requirements:

- **Review regularly** when significant codebase changes occur
- **Update version information** immediately when technologies change
- **Document new active technologies** as they are introduced
- **Update workflow restrictions** if development processes change
- **Maintain current project structure** documentation
- **Keep build and test commands** accurate and functional

### When to Update:

- New technologies are adopted
- Build processes change
- Project structure evolves
- Workflow restrictions change
- New dependencies are added
- Testing frameworks change

### Integration with GitHub Copilot:

- This file is automatically read by GitHub Copilot via `.github/instructions/ai-agent-files.instructions.md`
- Use it as reference for Gemini AI assistants
- Follow the documented workflow restrictions
- Keep information current for consistent AI assistance

### Maintenance Checklist:

- [ ] Active technologies list is current
- [ ] Project structure reflects reality
- [ ] Build commands work as documented
- [ ] Workflow restrictions are accurate
- [ ] Integration and testing information is up to date
- [ ] Recent changes section is maintained
