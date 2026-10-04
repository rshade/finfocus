---
title: Contributing
description: Development setup, guidelines, and workflow for contributing to FinFocus.
---


Thank you for your interest in contributing to FinFocus Core! This document
provides guidelines and instructions for contributing code, documentation, and
feedback.

## Table of Contents

- [License](#license)
- [Contribution Types](#contribution-types)
- [Development Environment Setup](#development-environment-setup)
- [Feature Development with SpecKit](#feature-development-with-speckit)
- [Minor Bug Fixes](#minor-bug-fixes)
- [Quality Requirements](#quality-requirements)
- [Submitting Changes](#submitting-changes)
- [Agentic Workflows](#agentic-workflows)
- [Issue Labels and Decision Tracking](#issue-labels-and-decision-tracking)
- [Getting Help](#getting-help)

## License

This project is licensed under the **Apache License 2.0**.

By contributing to FinFocus Core, you agree that your contributions will be
licensed under the same terms. See the [LICENSE](https://github.com/rshade/finfocus/blob/main/LICENSE) file for the full
license text.

## Contribution Types

### New Features (Requires SpecKit)

New features, capabilities, and architectural changes **must** use
[SpecKit](https://github.com/github/spec-kit) for specification-driven
development. This ensures features are well-planned, documented, and testable.

### Minor Bug Fixes (SpecKit Optional)

Minor bug fixes and small improvements **do not require** SpecKit. You may
submit a pull request directly.

**What qualifies as a minor bug fix:**

- Typo corrections in code or documentation
- Small logic fixes that don't change APIs
- Documentation corrections or clarifications
- Dependency updates for security patches
- Test improvements and additional test coverage
- Performance optimizations without API changes

**What requires SpecKit:**

- New CLI commands or flags
- New features or capabilities
- API changes or new endpoints
- Architectural modifications
- Changes affecting the plugin protocol
- New integrations or data sources

## Development Environment Setup

### Prerequisites

| Tool             | Version    | Purpose             |
| ---------------- | ---------- | ------------------- |
| Go               | 1.27.1+    | Core development    |
| golangci-lint    | v2.12.2    | Go linting          |
| markdownlint-cli | v0.45.0    | Markdown linting    |
| Git              | Latest     | Version control     |
| Make             | Latest     | Build automation    |
| Node.js          | Latest LTS | Documentation tools |

### Installing Development Tools

**Install Go** (if not already installed):

Download from [go.dev/dl](https://go.dev/dl/)

**Install golangci-lint:**

```bash
curl -sSfL "https://golangci-lint.run/install.sh" | sh -s -- -b "$HOME/go/bin" v2.12.2
```

**Install markdownlint-cli:**

```bash
npm install -g markdownlint-cli@0.45.0
```

### Clone and Build

```bash
git clone https://github.com/rshade/finfocus.git
cd finfocus

go mod download

make build

./bin/finfocus --help
```

### Make Targets Reference

Run `make help` for a complete list. All available targets:

#### Core Development Targets

| Target           | Description                                         |
| ---------------- | --------------------------------------------------- |
| `make build`     | Build the `finfocus` binary to `bin/finfocus`   |
| `make test`      | Run all unit tests                                  |
| `make test-race` | Run tests with Go race detector enabled             |
| `make lint`      | Run Go linters (golangci-lint) and Markdown linters |
| `make validate`  | Run `go mod tidy`, `go vet`, and format validation  |
| `make clean`     | Remove build artifacts (`bin/` directory)           |
| `make run`       | Build and run binary with `--help` flag             |
| `make dev`       | Build and run binary without arguments              |
| `make inspect`   | Launch MCP Inspector for interactive testing        |

#### Documentation Targets

| Target               | Description                                       |
| -------------------- | ------------------------------------------------- |
| `make docs-lint`     | Lint documentation markdown files                 |
| `make docs-build`    | Build documentation site with Jekyll              |
| `make docs-serve`    | Serve documentation locally (localhost:4000)      |
| `make docs-validate` | Validate documentation structure and completeness |

### Verifying Your Setup

```bash
make build      # Should complete without errors
make test       # All tests should pass
make lint       # No linting errors
make validate   # Module and vet checks pass
```

## Feature Development with SpecKit

New features **must** follow specification-driven development using
[SpecKit](https://github.com/github/spec-kit). This workflow ensures features
are properly designed before implementation.

### Why SpecKit?

- **Better planning**: Features are fully specified before coding begins
- **Clearer requirements**: User stories and acceptance criteria defined upfront
- **Consistent quality**: Follows project constitution and quality gates
- **Easier review**: Reviewers understand the intent and scope

### Installing SpecKit

```bash
uv tool install specify-cli --from git+https://github.com/github/spec-kit.git

pipx install git+https://github.com/github/spec-kit.git
```

### SpecKit Workflow

1. **Create specification** - Define what you're building:

   ```text
   /speckit.specify [your feature description]
   ```

2. **Clarify ambiguities** - Resolve underspecified areas (recommended):

   ```text
   /speckit.clarify
   ```

   This asks targeted questions to fill gaps in your spec before planning.

3. **Plan implementation** - Design the technical approach:

   ```text
   /speckit.plan
   ```

4. **Generate tasks** - Break down into actionable items:

   ```text
   /speckit.tasks
   ```

5. **Analyze consistency** - Validate artifacts before implementation:

   ```text
   /speckit.analyze
   ```

   This performs read-only analysis across spec, plan, and tasks to catch
   inconsistencies, gaps, and constitution violations.

6. **Implement** - Build the feature:

   ```text
   /speckit.implement
   ```

### Project Specifications Directory

All specifications, plans, and templates are stored in the `.specify/` directory:

```text
.specify/
├── memory/
│   └── constitution.md    # Project principles and quality gates
├── scripts/               # Helper scripts for SpecKit workflows
│   └── bash/
│       ├── check-prerequisites.sh
│       ├── create-new-feature.sh
│       └── setup-plan.sh
└── templates/             # Templates for specs, plans, tasks
    ├── spec-template.md
    ├── plan-template.md
    ├── tasks-template.md
    └── checklist-template.md
```

**Important**: Review [.specify/memory/constitution.md](https://github.com/rshade/finfocus/blob/main/.specify/memory/constitution.md)
before starting any feature work. It defines the core principles and quality
gates that all contributions must follow.

## Minor Bug Fixes

For minor bug fixes that don't require SpecKit:

1. **Create a branch:**

   ```bash
   git checkout -b fix/brief-description main
   ```

2. **Make your changes** and write tests

3. **Run quality checks:**

   ```bash
   make test    # Must pass
   make lint    # Must pass
   ```

4. **Submit a pull request** with:
   - Clear description of the bug
   - Explanation of the fix
   - Tests demonstrating the fix works

## Quality Requirements

All contributions must meet these quality gates, which are enforced by CI:

### Code Quality

- **Test Coverage**: Minimum 80% overall, 95% for critical paths
- **Linting**: `golangci-lint` must pass with zero errors
- **Security**: `govulncheck` must report no high/critical vulnerabilities
- **Formatting**: All Go code must be formatted with `gofmt`
- **Documentation**: Minimum 80% docstring coverage for exported symbols

### Pre-Submission Checklist

**Always run these commands before submitting:**

```bash
make lint      # Must pass with no errors
make test      # All tests must pass
make validate  # Module and vet checks must pass
```

### Test Requirements

For detailed testing instructions, see the [Testing Guide](../testing/guide.md).

- Write tests before implementation (TDD approach)
- Include tests for all new code paths
- Test error conditions and edge cases
- Use table-driven tests where appropriate
- Run tests with race detector: `make test-race`

### Running Fuzz Tests

FinFocus uses Go's native fuzzing (Go 1.26+) to test parser resilience:

```bash
go test -fuzz=FuzzJSON$ -fuzztime=30s ./internal/ingest

go test -fuzz=FuzzYAML$ -fuzztime=30s ./internal/spec

go test -fuzz=. -fuzztime=30s ./internal/ingest
```

**Fuzz test locations:**

| Package           | Test Function        | Target               |
| ----------------- | -------------------- | -------------------- |
| `internal/ingest` | `FuzzJSON`           | JSON parser          |
| `internal/ingest` | `FuzzPulumiPlanParse`| Full plan parsing    |
| `internal/spec`   | `FuzzYAML`           | YAML spec parsing    |
| `internal/spec`   | `FuzzSpecFilename`   | Spec filename parser |

**Seed corpus:**

Fuzz tests use seed corpora in `testdata/fuzz/` directories. Add new
interesting inputs discovered during fuzzing to these directories.

**CI integration:**

- PRs run 30-second fuzz smoke tests
- Nightly builds run 6-hour deep fuzzing sessions

### Running Benchmarks

FinFocus includes performance benchmarks for scalability testing:

```bash
go test -bench=. -benchmem ./test/benchmarks/...

go test -bench=BenchmarkScale -benchmem ./test/benchmarks/...

go test -bench=BenchmarkScale1K -benchtime=10x -benchmem ./test/benchmarks/...
```

**Available benchmarks:**

| Benchmark                    | Description             | Target    |
| ---------------------------- | ----------------------- | --------- |
| `BenchmarkScale1K`           | 1,000 resources         | < 1s      |
| `BenchmarkScale10K`          | 10,000 resources        | < 30s     |
| `BenchmarkScale100K`         | 100,000 resources       | < 5min    |
| `BenchmarkDeeplyNested`      | Deep nesting complexity | < 1s      |
| `BenchmarkJSONParsing`       | JSON parsing at scale   | Baseline  |
| `BenchmarkGeneratorOverhead` | Generator overhead      | Baseline  |

**Benchmark guidelines:**

- Run benchmarks on a quiet system for consistent results
- Use `-benchtime=10x` for quick checks, `-benchtime=1m` for accurate results
- Compare results between commits to detect performance regressions
- CI runs smoke benchmarks (1x) on every PR

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
type(scope): description

[optional body]

[optional footer]
```

**Types:** `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`

**Examples:**

```text
feat(cli): add --format flag for cost output
fix(engine): correct monthly cost calculation rounding
docs(contributing): add SpecKit workflow documentation
test(registry): add plugin discovery edge case tests
```

## Submitting Changes

### Pull Request Process

1. **Ensure your branch is current:**

   ```bash
   git fetch origin
   git rebase origin/main
   ```

2. **Run all quality checks:**

   ```bash
   make test
   make lint
   make validate
   ```

3. **Push and create PR:**

   ```bash
   git push origin your-branch-name
   ```

4. **Fill in PR template** with:
   - Description of changes
   - Type of change (feature, fix, docs, etc.)
   - Testing performed
   - Related issues

### CI Checks

All pull requests must pass:

- Go Tests with race detection
- Code Coverage (minimum threshold)
- golangci-lint
- Security scanning (govulncheck)
- Documentation validation
- Cross-platform builds (Linux, macOS, Windows)

### Releases

#### Monorepo plugin releases

Plugins shipped from this monorepo (under `plugins/<name>/`) release
independently from the CLI, using a `<plugin>-vX.Y.Z` tag format (for
example, `kubernetes-v0.1.0`) rather than the CLI's bare `vX.Y.Z` tags:

- Each plugin has its own `release-please` component so its changelog and
  version bump are tracked separately from the CLI.
- `.github/workflows/release-monorepo-plugin.yml` builds and uploads release
  archives (`scripts/release-plugin-assets.sh`) for a published `<plugin>-v*`
  release; the CLI's `goreleaser.yml` and `nightly.yml` release-triggered jobs
  are guarded to skip these tags.
- The plugin's registry entry needs **both** `tag_prefix` set (for example,
  `"kubernetes-"`) so the installer can resolve prefixed tags to a canonical
  version, **and** `asset_hints.asset_prefix` set (for example,
  `"finfocus-plugin-kubernetes"`). Without `asset_prefix` the installer falls
  back to looking for assets named `<plugin>_v…` and never matches the
  `finfocus-plugin-<plugin>_v…` archives that
  `release-monorepo-plugin.yml`/`scripts/release-plugin-assets.sh` produce.
- Monorepo plugin names (and thus `tag_prefix` values) must not start with
  `v`. The CLI's own release workflows distinguish a bare CLI tag from a
  plugin tag with `startsWith(tag, 'v')`; a plugin prefix starting with `v`
  (e.g. `vantage-v0.1.0`) would defeat that guard, so
  `ValidateRegistryEntry` rejects it.
- A published `<plugin>-vX.Y.Z` release is never allowed to become the
  repo's "Latest" release (that slot must stay on the newest CLI `vX.Y.Z`
  release, since `scripts/install.sh` and `plugin_init_fixtures.go` read
  `/releases/latest`). `release-monorepo-plugin.yml` unmarks the plugin
  release as latest and re-marks the newest stable CLI release after
  uploading assets.

### Automated Nightly Failure Analysis

To assist with debugging, the project employs an automated nightly failure
analysis workflow:

- **Trigger**: When a nightly build fails, an issue is created with the
  `nightly-failure` label.
- **Analysis**: A workflow automatically runs to fetch logs, analyze them
  using OpenCode, and post a triage report as a comment.
- **Goal**: Provide immediate root cause hypothesis and suggested fixes to
  reduce manual investigation time.
- **Hardening**: The workflow enforces strict security and reliability standards:
  - **Pinned Dependencies**: Uses specific versions of CLI tools and actions.
  - **Timeouts**: Enforces a 59-minute execution limit to prevent resource exhaustion.
  - **Error Handling**: Fails explicitly on any command error to avoid silent failures.

### Agentic Workflows

The repository uses [GitHub Agentic Workflows (gh-aw)](https://github.com/github/gh-aw)
to automate common maintenance tasks. These workflows run on schedules or are
triggered by slash commands in pull request comments.

#### Daily Scheduled Workflows

| Workflow | Description |
| -------- | ----------- |
| `daily-doc-updater` | Scans merged PRs and updates documentation to reflect new features and changes. |
| `daily-malicious-code-scan` | Reviews code changes from the last 3 days for suspicious or malicious patterns. |
| `code-simplifier` | Analyzes recently modified code and opens PRs with readability improvements while preserving behavior. |
| `issue-arborist` | Links related open issues as sub-issues to improve organization. |
| `sub-issue-closer` | Automatically closes a parent issue when all of its sub-issues are resolved. |
| `audit-workflows` | Audits all agentic workflow runs from the last 24 hours and surfaces errors or improvement opportunities. |

#### Weekly Scheduled Workflows

| Workflow | Description |
| -------- | ----------- |
| `weekly-issue-summary` | Posts a weekly summary of issue activity including trends and insights every Monday. |

#### Dependency Management Workflows

| Workflow | Description |
| -------- | ----------- |
| `dependabot-pr-bundler` | Groups compatible Dependabot updates into a single PR, runs tests, and creates a draft PR with the bundled changes. |

#### On-Demand Slash Commands

These workflows are triggered by a maintainer comment on a pull request:

| Command | Description |
| ------- | ----------- |
| `/pr-fix` | Analyzes failing CI checks in a PR, identifies the root cause, implements fixes, and pushes a corrected commit to the branch. |
| `/mergefest` | Merges the `main` branch into the current PR branch to resolve conflicts or bring it up to date. |

#### CI Failure Investigation

| Workflow | Description |
| -------- | ----------- |
| `ci-doctor` | Triggers automatically when a monitored workflow fails and performs deep log analysis to surface root causes and remediation steps. |

## Project Architecture

FinFocus operates as a three-repository ecosystem:

| Repository                  | Purpose                                     |
| --------------------------- | ------------------------------------------- |
| [finfocus][core]     | CLI tool, plugin host, orchestration engine |
| [finfocus-spec][spec]     | Protocol buffer definitions, SDK generation |
| [finfocus-plugin][plugin] | Plugin implementations (Kubecost, Vantage)  |

[core]: https://github.com/rshade/finfocus
[spec]: https://github.com/rshade/finfocus-spec
[plugin]: https://github.com/rshade/finfocus-plugin

Cross-repository changes require coordination. See the
[constitution](https://github.com/rshade/finfocus/blob/main/.specify/memory/constitution.md) for the cross-repo change
protocol.

## Issue Labels and Decision Tracking

### The `decision` Label

Issues labeled with `decision` represent **deliberate architectural or product
decisions** with documented reasoning. These serve as lightweight Architecture
Decision Records (ADRs) without the overhead of separate markdown files.

**When to use `decision`:**

- Thorough analysis was performed
- Reasoning is documented in the issue body
- A deliberate choice was made (implement, defer, or reject)

**Finding past decisions:**

```bash
gh issue list --repo rshade/finfocus --state closed --label decision

```

**Why this matters:**

- Prevents re-litigating the same issues
- New contributors can understand *why* things are the way they are
- Creates searchable institutional knowledge

### Standard Labels

| Label              | Purpose                                   |
| ------------------ | ----------------------------------------- |
| `decision`         | Deliberate architectural/product decision |
| `enhancement`      | New feature or request                    |
| `bug`              | Something isn't working                   |
| `documentation`    | Documentation improvements                |
| `good first issue` | Good for newcomers                        |

## Getting Help

### Documentation

- [Developer Guide](../guides/developer-guide.md) - Complete developer docs
- [Architecture](../architecture/README.md) - System design and diagrams
- [Plugin Development](../plugins/plugin-development.md) - Building plugins

### Support Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and community discussion

## Code of Conduct

Be respectful and constructive in all interactions. We welcome contributors of
all experience levels and backgrounds.

---

Thank you for contributing to FinFocus Core!
