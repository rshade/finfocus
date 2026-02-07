# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## GitHub Actions Workflows Overview

This directory contains GitHub Actions workflows for CI/CD, automated code review, and Claude Code integration.

## Workflow Files

### CI/CD Workflows

**ci.yml** - Complete CI/CD pipeline triggered on PRs and main branch pushes:

- **Test Job**: Go 1.25.7 setup, unit tests with race detection, coverage reporting (20% minimum threshold)
- **Lint Job**: golangci-lint with project-specific configuration, security scanning with gosec
- **Security Job**: govulncheck for dependency vulnerability scanning
- **Validation Job**: gofmt formatting checks, go mod tidy verification, go vet static analysis
- **Build Job**: Cross-platform builds (Linux/macOS/Windows, amd64/arm64), artifact upload

**release.yml** - Multi-platform binary releases triggered on version tags (v\*):

- Builds for Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
- Automatic changelog generation, SHA256 checksums, GitHub Release creation
- Binary naming: `finfocus-v{version}-{os}-{arch}`

### OpenCode Integration Workflows

**IMPORTANT: Understanding OpenCode GitHub Action vs CLI**

OpenCode can be used in two ways in GitHub Actions:

1. **GitHub Action** (`sst/opencode/github@dev`):
   - Only works with `issue_comment` events (triggered by comments)
   - Accepts parameters: `model`, `share`, `token` (NO `prompt` parameter)
   - The prompt comes FROM the comment that triggers the workflow
   - Example trigger: User comments `/opencode please fix the linting issues`

2. **OpenCode CLI** (manual installation):
   - Can work with any event type
   - Allows passing prompts directly via command line
   - Requires manual installation step
   - More flexible but requires more setup

**opencode-review-fix.yml** - Systematic review issue fixing:

- **Trigger**: Comments containing `/opencode-review-fix` on PRs
- **Type**: Uses OpenCode CLI (manual installation with embedded prompt)
- **Purpose**: Automatically fixes ALL review issues from CodeRabbit, OpenCode, and other reviewers
- **Features**:
  - Embedded comprehensive prompt for systematic issue fixing
  - Concurrency control to prevent overlapping runs per PR
  - Automatic PR branch checkout
  - Post-execution Go validation (go mod tidy, go build, make test)
  - golangci-lint integration
- **Requirements**: `XAI_API_KEY` secret and `gh` CLI
- **Usage**: Simply comment `/opencode-review-fix` on any PR to trigger automatic fix workflow
- **Note**: Uses CLI approach to allow embedded prompt - no manual instructions needed

**opencode-code-review.yml** - Automated PR code review:

- **Trigger**: PRs with `opencode-review` label (opened, synchronize, labeled, reopened)
- **Type**: Uses OpenCode CLI (manual installation)
- **Purpose**: Provides automated code review focusing on code quality, security, and best practices
- **Scope**: Reviews all changes in the PR with special attention to workflow files
- **Requirements**: `XAI_API_KEY` secret must be configured, `gh` CLI for PR operations
- **Permissions**: Read-only access to repository, write access for PR comments
- **Usage**: Add the `opencode-review` label to any PR to trigger automatic review
- **Note**: This workflow uses CLI installation, not the GitHub action, allowing custom prompts

**opencode.yml** - General OpenCode assistance:

- **Trigger**: Comments containing `/oc` or `/opencode` in issues or PRs
- **Type**: Uses OpenCode GitHub Action
- **Purpose**: General development assistance and question answering
- **Flexibility**: Responds to issue comments and PR comments
- **Usage**: Comment `/oc [your instructions]` or `/opencode [your instructions]` to get assistance
- **Permissions**: Read access to contents, id-token for OIDC
- **Requirements**: `XAI_API_KEY` secret must be configured
- **Examples**:

  ```
  /oc please explain the engine architecture
  /opencode add unit tests for the new function
  /oc refactor this code for better readability
  ```

### Common OpenCode Usage Patterns

**Review Fixing Workflow** (using opencode-review-fix.yml):

1. Create/update PR with code changes
2. Wait for automated reviews (CodeRabbit, etc.)
3. Comment on PR: `/opencode-review-fix` (no additional instructions needed)
4. OpenCode automatically:
   - Reads all review comments
   - Checks out the PR branch
   - Fixes each issue one at a time with validation
   - Commits and pushes changes
5. Post-execution validation runs automatically (Go-specific: go mod tidy, go build, make test)
6. golangci-lint runs to ensure no new issues
7. Review and merge when complete

**General Assistance** (using opencode.yml):

- Use `/oc` or `/opencode` in any GitHub comment for development help
- Ask about architecture decisions, debugging, or implementation guidance
- Request code explanations or suggestions for improvements
- Get help with testing strategies or CI/CD configuration

**Automated Code Review** (using opencode-code-review.yml):

- Add `opencode-review` label to any PR
- Automated review triggers on PR events
- Reviews focus on code quality, security, and best practices
- Integrates with existing CI pipeline results
- Provides actionable feedback via PR comments

## Workflow Architecture Patterns

### Security and Permissions

- All workflows use least-privilege permission models
- OIDC token authentication for secure access
- Separate permissions for read vs write operations
- Concurrency controls to prevent resource conflicts

### Error Handling and Reliability

- Timeout configurations for long-running operations
- Artifact upload with proper naming conventions
- Context cancellation handling for interrupted workflows
- Rollback mechanisms in review-fix workflow

### Integration Points

- Workflows can read CI results from other workflows
- Cross-workflow artifact sharing capabilities
- GitHub API integration for issue/PR management
- External tool integrations (golangci-lint, govulncheck)

## Development Commands for Workflows

```bash
# Validate workflow syntax
gh workflow validate .github/workflows/ci.yml
gh workflow validate .github/workflows/release.yml
gh workflow validate .github/workflows/claude-review-fix.yml

# Trigger workflows manually (if configured)
gh workflow run ci.yml
gh workflow run claude-code-review.yml

# View workflow runs and status
gh run list
gh run view <run-id>
gh run watch <run-id>

# Check workflow permissions and secrets
gh secret list
gh auth status
```

## Workflow Development Guidelines

### Always Review Existing Workflows First

**CRITICAL**: Before creating new workflows, always examine existing workflows to understand established patterns, tool versions, and configurations. Inconsistency leads to maintenance issues and different behavior across workflows.

**Required Review Process**:

1. Examine `ci.yml` for current tool versions and patterns
2. Check action versions used (e.g., `actions/checkout@v5`, `actions/setup-go@v5`)
3. Identify official actions vs manual installations
4. Match timeout, caching, and configuration patterns
5. Ensure consistent permission models

### Established Tool Patterns

**golangci-lint**:

- ✅ **Use**: `golangci/golangci-lint-action@v8` with `version: v2.3.1` and `args: --timeout=5m`
- ❌ **Avoid**: Manual installation via curl scripts
- **Rationale**: Official action provides better caching, error handling, and maintenance

**Go Setup**:

- ✅ **Standard**: `actions/setup-go@v5` with `go-version: '1.25.7'` and `cache: true`
- **Consistency**: All workflows should use identical Go version and caching

**Checkout**:

- ✅ **Standard**: `actions/checkout@v5`
- **Special cases**: Use `fetch-depth: 0` only when git history is needed

### Concurrency Control Patterns

Always add concurrency control to prevent resource conflicts:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.event.issue.number }} # For PR-based workflows
  cancel-in-progress: true
```

## Common Workflow Patterns

### Multi-Platform Builds

- Use matrix strategies for OS/architecture combinations
- Proper binary naming with platform suffixes
- LDFLAGS for version information embedding
- Artifact organization by platform

### Coverage and Quality Gates

- Minimum coverage thresholds with flexibility for project maturity
- Security scanning integrated into CI pipeline
- Multiple validation layers (formatting, linting, testing)
- Fail-fast approach with clear error reporting

### Dependency Management Integration

- Works with Renovate and Dependabot configurations
- Automated security vulnerability detection
- Semantic commit formatting for changelog generation
- Rate limiting to prevent notification spam
