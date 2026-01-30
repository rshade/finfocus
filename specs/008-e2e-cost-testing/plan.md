# Implementation Plan: E2E Cost Testing

**Branch**: `008-e2e-cost-testing` | **Date**: 2025-12-03 | **Spec**: [Feature Spec](./spec.md)
**Input**: Feature specification from `/specs/008-e2e-cost-testing/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implement an End-to-End (E2E) testing framework for FinFocus using the Pulumi Automation API. This framework will programmatically deploy AWS resources (e.g., T3 micro instances), run cost calculations, and validate that:

1. Projected costs match AWS list prices within ±5%.
2. Actual costs are proportional to runtime duration.
3. All resources are automatically cleaned up after tests.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**:

- `github.com/pulumi/pulumi-aws/sdk/v7` (AWS Provider v7)
- `github.com/pulumi/pulumi/sdk/v3 v3.210.0` (Pulumi SDK)
- `github.com/stretchr/testify` (Assertions)
- **Storage**: Local Pulumi state (ephemeral), no persistent DB.
- **Testing**: `go test` with `//go:build e2e` tag.
- **Target Platform**: Linux, macOS, Windows.
- **Project Type**: CLI Tool / Testing Framework.
- **Performance Goals**: Tests must complete within 60 minutes (default, configurable).
- **Constraints**: Must not leave orphaned resources. Must run in CI environment. E2E test runner script (run-e2e-tests.sh) requires Bash (Linux/macOS/WSL). Native Windows PowerShell support is deferred to a future release.
- **Scale/Scope**: Covers EC2 and EBS initially.

### Critical Design Decision: Real User Workflow with YAML Projects

**E2E tests MUST follow the exact workflow that users and GitHub Actions will use:**

1. Create a real Pulumi YAML project directory with `Pulumi.yaml` (containing resources)
2. Run `pulumi preview --json > preview.json` via CLI
3. Pass the preview file to `finfocus cost projected --pulumi-json preview.json`

**Why YAML instead of Go?**

- ⚡ **4x faster**: YAML tests complete in ~2.5 min vs 10+ min with Go
- 📦 **No dependencies**: No `go mod tidy` or SDK downloads needed
- 🎯 **Same output**: `finfocus` only needs the preview JSON - doesn't care what language generated it

**Rejected Approaches:**

- ❌ Go projects (too slow due to SDK compilation)
- ❌ Automation API inline programs (no JSON output available)
- ❌ State hacking or internal SDK manipulation
- ❌ Mock/simulated cost values

This ensures tests validate the actual user experience efficiently.

### Critical Design Decision: Plugin Integration E2E Testing (Session 2025-12-04)

**E2E tests validate the complete cost calculation chain in two modes:**

1. **No Plugin Mode** (validates CLI parsing)
   - Run `finfocus cost projected` without plugins installed
   - Validates CLI correctly parses preview JSON and outputs $0.00
   - This is important to ensure CLI works even without plugins

2. **Full Chain Mode** (validates actual cost calculations)
   - Install `aws-public` plugin via `finfocus plugin install aws-public`
   - Run `finfocus cost projected` with the plugin
   - Validate cost output matches expected AWS pricing (~$7.59/month for t3.micro)
   - Optionally cleanup plugin after test

**Plugin Installation Strategy:**

```bash
# Install plugin programmatically in test setup
finfocus plugin install aws-public

# Verify installation
finfocus plugin list

# Optional cleanup after test
finfocus plugin remove aws-public
```

**Test Structure:**

- `TestProjectedCost_EC2_WithoutPlugin` - Validates CLI parsing (expects $0.00)
- `TestProjectedCost_EC2_WithPlugin` - Validates full chain with plugin (expects ~$7.59)

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Is this feature implemented as a plugin or orchestration logic? (Orchestration/Testing)
- [x] **Test-Driven Development**: Are tests planned before implementation? (80% minimum coverage)
- [x] **Cross-Platform Compatibility**: Will this work on Linux, macOS, Windows?
- [x] **Documentation as Code**: Are audience-specific docs planned? (Quickstart for devs)
- [x] **Protocol Stability**: Do protocol changes follow semantic versioning? (N/A)
- [x] **Quality Gates**: Are all CI checks (tests, lint, security) passing?
- [x] **Multi-Repo Coordination**: Are cross-repo dependencies documented? (Depends on AWS plugin)

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/008-e2e-cost-testing/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A for this feature)
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
test/e2e/
├── projects/            # Real Pulumi project directories (user workflow)
│   └── ec2/
│       └── Pulumi.yaml  # Project definition (YAML runtime, no Go compilation)
├── main_test.go         # Test harness (SetupProject, RunFinFocus, Teardown)
├── e2e_white_box_test.go # White-box tests (importing packages)
├── e2e_black_box_test.go # Black-box tests (CLI binary)
├── cleanup.go           # Resource cleanup helpers
├── validator.go         # Cost validation with ±5% tolerance
├── pricing.go           # Expected pricing reference map
├── utils.go             # Helper functions (ULID, etc.)
├── plugin_helpers.go    # Plugin installation/removal helpers (NEW)
└── run-e2e-tests.sh     # Repeatable test execution script
```

**Structure Decision**: Real project directories replace inline programs to match user workflow.
**Plugin Decision**: YAML projects used for speed (~2.5 min vs 10+ min with Go).

## Learnings & Architectural Updates

### 1. Pulumi Plan JSON Structure (`newState`)

We discovered that `pulumi preview --json` nests resource details (including `inputs`, `type`, and `provider`) under a `newState` object for operations like `create`, `update`, and `same`.

- **Impact**: The ingestion logic (`internal/ingest/pulumi_plan.go`) must inspect `newState` to correctly extract these fields.
- **Consequence**: Failing to do so results in empty `Inputs`, which causes property extraction (SKU, Region) to fail in the Core adapter.

### 2. Plugin Resource Type Compatibility

Pulumi uses "Type Tokens" (e.g., `aws:ec2/instance:Instance`), but some plugins or downstream pricing APIs may expect internal service identifiers (e.g., `ec2`).

- **Strategy**: Plugins are responsible for handling standard Pulumi Type Tokens.
- **Fix**: We patched the `aws-public` plugin to normalize `aws:ec2/instance:Instance` -> `ec2` internally, rather than forcing Core to guess the plugin's preferred format. This keeps Core generic.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --------- | ---------- | ------------------------------------ |
| N/A       |            |                                      |
