# Implementation Plan: Analyzer E2E Tests

**Branch**: `012-analyzer-e2e-tests` | **Date**: 2025-12-08 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/012-analyzer-e2e-tests/spec.md`

## Summary

Add end-to-end tests that verify the FinFocus Analyzer plugin works correctly with the real Pulumi CLI, ensuring the complete workflow from `pulumi preview` to cost diagnostic output functions as expected. Tests will follow existing E2E patterns in `test/e2e/` and integrate with `nightly.yml` workflow.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: testing (stdlib), github.com/stretchr/testify, github.com/oklog/ulid/v2
**Storage**: Local Pulumi state (`file://` backend), temp directories for test fixtures
**Testing**: Go test with `-tags e2e` build constraint
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single project - tests in existing `test/e2e/` directory
**Performance Goals**: Tests complete within 15 minutes additional runtime in nightly job
**Constraints**: No cloud credentials required for analyzer-specific tests, local backend only
**Scale/Scope**: 4-6 new test functions, 1 new fixture project, ~300 lines of test code

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Tests validate the analyzer plugin integration - aligns with plugin architecture
- [x] **Test-Driven Development**: This IS the test implementation - tests are the deliverable
- [x] **Cross-Platform Compatibility**: Tests use `exec.Command` and standard Go patterns - cross-platform by design
- [x] **Documentation as Code**: Test code is self-documenting; test names describe functionality
- [x] **Protocol Stability**: Tests validate existing analyzer gRPC protocol, no protocol changes
- [x] **Quality Gates**: Tests will pass CI checks (lint, test, security)
- [x] **Multi-Repo Coordination**: No cross-repo dependencies - tests are internal to finfocus-core

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/012-analyzer-e2e-tests/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (test entities)
├── quickstart.md        # Phase 1 output (running tests)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
test/e2e/
├── main_test.go                # Existing TestContext, SetupProject, Teardown
├── e2e_white_box_test.go       # Existing E2E tests (EC2, EBS, cost validation)
├── e2e_black_box_test.go       # Existing CLI execution test
├── analyzer_e2e_test.go        # NEW: Analyzer integration tests
├── utils.go                    # Existing utilities (GenerateStackName)
├── validator.go                # Existing cost validators
├── cleanup.go                  # Existing cleanup manager
├── plugin_helpers.go           # Existing plugin management
├── pricing.go                  # Existing pricing references
└── projects/
    ├── ec2/
    │   └── Pulumi.yaml         # Existing EC2 fixture
    └── analyzer/               # NEW: Analyzer test fixture
        └── Pulumi.yaml         # YAML project with analyzer config
```

**Structure Decision**: Extend existing `test/e2e/` directory with new test file and fixture project. Follows established patterns from existing E2E tests.

## Critical Design Decisions

### 1. Analyzer Configuration via Pulumi.yaml (Smart Binary Approach)

**Decision**: The `finfocus` binary will be made "smart" to detect if it's being invoked by the Pulumi engine as an analyzer plugin. If the executable name matches the Pulumi Analyzer convention (`pulumi-analyzer-<name>`), it will automatically start its gRPC server (`finfocus analyzer serve`) without requiring explicit subcommand arguments. Otherwise, it will function as the standard CLI tool.

**Configuration Format**:

```yaml
name: finfocus-analyzer-e2e
runtime: yaml
description: E2E test for analyzer plugin

plugins:
  analyzers:
    - name: finfocus
      path: /path/to/plugin/directory # This directory will contain 'pulumi-analyzer-finfocus'
      version: 0.0.0-dev
```

**Rationale**:

- **Seamless Integration**: Allows `finfocus` to act as both a standalone
  CLI and a Pulumi Analyzer plugin from a single binary.
- **No Wrapper Scripts**: Eliminates the need for platform-specific shell
  scripts (e.g., `.sh` for Linux, `.cmd` for Windows) within the test harness
  or for local development.
- **Pulumi Naming Convention**: Adheres to Pulumi's expectation for analyzer
  plugin executables (`pulumi-analyzer-<name>`).
- **Automatic Handshake**: The binary will automatically initiate the gRPC
  server and output the port number for the Pulumi engine handshake when
  detected as a plugin.

### 2. Real AWS Resources with aws-public Plugin

**Decision**: Use real AWS resources (t3.micro EC2) with the `aws-public` plugin, consistent with existing E2E tests.

**Rationale**:

- Only way to guarantee analyzer output matches real pricing data
- Validates complete chain: Pulumi → Analyzer → Engine → Plugin → AWS Pricing
- Consistent with existing E2E patterns (`TestProjectedCost_EC2_WithPlugin`)
- AWS credentials already configured in `nightly.yml`
- Expected cost: ~$7.59/month for t3.micro (validated with ±5% tolerance)

### 3. Output Validation Strategy

**Decision**: Parse `pulumi preview` stdout for diagnostic patterns.

**Expected Patterns**:

```text
# Per-resource diagnostic
"Estimated Monthly Cost: $"

# Stack summary
"Total Estimated Monthly Cost: $"

# Policy attribution
"finfocus"
```

**Rationale**:

- Diagnostics appear in stdout as part of preview output
- Simple string/regex matching is sufficient
- The analyzer server writes logs to stderr only (doesn't pollute stdout)

## Complexity Tracking

No violations - all principles satisfied.
