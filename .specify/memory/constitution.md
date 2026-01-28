<!--
Sync Impact Report - Constitution v1.5.0 (Documentation Integrity)
================================================================

Version Change: 1.4.0 → 1.5.0
Change Type: Materially expanded guidance (MINOR)

Changes Made:
- Renamed Principle IV to "Documentation Integrity (Anti-Drift)"
- Added strict requirements for Godoc coverage (80%+)
- Mandated compilable code examples in READMEs
- Required strict function signature matching between docs and code
- Added requirement for SDK package READMEs
- Added requirement for Spec Status Tracking
- Explicitly required documentation validation in CI

Rationale:
- To prevent documentation drift where code evolves but docs lag behind
- Automated validation prevents drift without requiring constant manual vigilance
- Ensures developer trust through verifiable documentation accuracy

Templates Requiring Updates:
- ✅ .specify/templates/plan-template.md (Updated Constitution Check)
- ✅ .specify/templates/tasks-template.md (Updated Principle IV reference)

Follow-up TODOs:
- None

Date: 2026-01-27

---

Sync Impact Report - Constitution v1.4.0 (Documentation Synchronization)
======================================================================

Version Change: 1.3.0 → 1.4.0
Change Type: Expanded guidance (MINOR)

Changes Made:
- Renamed Principle IV to "Documentation Synchronization & Quality"
- Explicitly mandates `README.md` and `docs/` updates concurrently with implementation
- Added rationale regarding documentation drift and single source of truth

Rationale:
- To prevent documentation drift where code evolves but docs lag behind
- Ensures `README.md` and `docs/` are always accurate reflections of the codebase

Templates Requiring Updates:
- ✅ .specify/templates/plan-template.md (Updated Constitution Check)
- ✅ .specify/templates/tasks-template.md (Added Principle IV reference)

Follow-up TODOs:
- None

Date: 2026-01-07

---

Sync Impact Report - Constitution v1.3.0 (Implementation Completeness)
====================================================================

Version Change: 1.2.0 → 1.3.0
Change Type: New principle added (MINOR)

Changes Made:
- Promoted "Implementation Completeness" to Core Principle VI
- Mandates full implementation of code, prohibiting stubs and placeholders
- Forbids TODO comments in committed code
- Requires task list items to be fully implemented before completion
- Enforces that tests MUST exercise real behavior

Rationale:
- Prevents technical debt from stubbed functions and deferred logic
- Ensures AI agents and developers deliver production-ready code
- Maintains high quality standards across the ecosystem

Templates Requiring Updates:
- ✅ .specify/templates/plan-template.md (Added Principle VI to Constitution Check)
- ✅ .specify/templates/tasks-template.md (Added note on Principle VI requirements)

Follow-up TODOs:
- None

Date: 2025-12-23

---

Previous Version - Constitution v1.2.0 (Implementation Completeness & Protocol Updates)
======================================================================================

Version Change: 1.1.0 → 1.2.0
Change Type: Expanded guidance (MINOR)

Changes Made:
- Added "Implementation Completeness" section to Governance
- Forbids stubs, placeholders, and TODO comments in committed code
- Mandates full implementation before marking tasks complete
- Added "Dependency Version Policy" for Pulumi SDK
- Formalized import paths for protobuf types

Rationale:
- Ensures code quality and prevents technical debt
- Resolves compatibility issues with Pulumi SDK v3.210.0+
- Provides clear guidance for Analyzer plugin development

Date: 2025-12-06

---

Previous Version - Constitution v1.1.0 (Quality Gate Enhancements)
====================================================================

Version Change: 1.0.0 → 1.1.0
Change Type: Expanded guidance (MINOR)

Changes Made:
- Added Docstring Coverage quality gate (minimum 80%)
- Enforces Go package and exported symbol documentation
- Added "Linting Protocol" subsection under Quality Gates
- Mandates `make lint` and `make test` before claiming task complete
- Prohibits `.golangci.yml` modifications without approval

Rationale:
- Ensures comprehensive Go documentation for maintainability
- Repeated lint failures in CI/CD from incomplete local verification
- Enforces quality gate compliance at development time
- Prevents wasted CI cycles from preventable lint errors

Date: 2025-11-24
-->

# PulumiCost Core Constitution

## Core Principles

### I. Plugin-First Architecture

All cost data sources MUST be implemented as gRPC plugins. The core system
MUST remain provider-agnostic and serve only as an orchestration layer.
Plugins MUST communicate via the protocol buffer definitions in
`pulumicost-spec` repository. Direct provider integrations within core are
forbidden.

**Rationale**: Enables extensibility without modifying core, supports
third-party plugin development, and maintains clear separation of concerns
between orchestration and data sourcing.

### II. Test-Driven Development (NON-NEGOTIABLE)

Tests MUST be written before implementation. All code changes MUST maintain
minimum 80% test coverage. Critical paths (CLI entry points, cost calculation
engine, plugin communication) MUST achieve 95% coverage. Tests MUST pass in
CI before any pull request merges.

**Rationale**: Ensures reliability in cost calculations where accuracy is
paramount. TDD prevents regressions and serves as living documentation of
system behavior.

### III. Cross-Platform Compatibility

All code MUST compile and run correctly on Linux (amd64, arm64), macOS
(amd64, arm64), and Windows (amd64). CI MUST verify cross-platform builds on
every commit. Platform-specific code MUST be isolated and documented.

**Rationale**: Infrastructure teams use diverse operating systems.
Cross-platform support is essential for adoption and prevents vendor lock-in.

### IV. Documentation Integrity (Anti-Drift)

Documentation MUST remain synchronized with implementation and MUST be
verifiable:

- **Godoc Coverage**: All exported functions, types, and methods in Go SDK
  packages MUST have godoc comments. CI MUST verify 80%+ documentation
  coverage.
- **README ↔ Code Sync**: Code examples in README files MUST compile. Function
  signatures, type names, and method names in documentation MUST match exported
  symbols exactly.
- **Semantic Versioning for Docs**: When a documented API changes, the
  corresponding documentation MUST be updated in the same PR.
- **Package Completeness**: Every SDK package with 3+ exported symbols MUST
  have a `README.md`.
- **Doc Validation in CI**: A documentation linter MUST run in CI checking
  markdown link validity, code block syntax, and function signature accuracy.
- **Spec Status Tracking**: Specifications in `specs/` MUST have status
  indicators. Specs missing `plan.md` and `tasks.md` for >30 days SHOULD be
  alerted.

**Rationale**: Documentation drift erodes developer trust. Automated validation
prevents drift without requiring constant manual vigilance. Synchronous updates
ensure the codebase and documentation remain a single source of truth.

### V. Protocol Stability

Breaking changes to protocol buffer definitions MUST follow semantic
versioning. Protocol changes MUST be coordinated across all three
repositories (core, spec, plugin). Backward compatibility MUST be maintained
for at least two minor versions.

**Rationale**: Plugins are developed by third parties. Protocol stability
prevents breaking existing integrations and ensures ecosystem reliability.

### VI. Implementation Completeness

Code changes MUST be complete implementations, not stubs or placeholders.
"Stubbing out" functionality for future implementation is strictly prohibited
in the main codebase.

1.  **No TODOs**: TODO comments are FORBIDDEN in committed code. Implement
    the feature fully or track it via a GitHub issue.
2.  **No Stubs**: Stub implementations that bypass actual functionality or
    return hardcoded/simulated results where real logic is required are
    FORBIDDEN.
3.  **Task Finality**: All task list items in `tasks.md` MUST be fully
    implemented, tested, and verified before being marked as complete `[x]`.
4.  **Real Tests**: Tests MUST exercise real behavior and logic. Mocking is
    permitted only for external systems (e.g., cloud APIs, gRPC services)
    as defined in the testing framework.

**Rationale**: Maintains a high standard of code readiness, prevents
technical debt accumulation, and ensures that every merged feature is
genuinely functional.

## Quality Gates

All pull requests MUST pass the following automated checks before merging:

- **Go Tests**: All unit, integration, and contract tests pass with `-race`
  flag
- **Code Coverage**: Minimum 80% overall, 95% for critical paths (enforced
  via CI)
- **Linting**: `golangci-lint` passes with project configuration
  (`.golangci.yml`)
- **Security**: `govulncheck` reports no high/critical vulnerabilities
- **Formatting**: All Go code formatted with `gofmt` and `goimports`
- **Documentation**: Markdown linting passes (`markdownlint-cli2`)
- **Docstring Coverage**: Minimum 80% Go package and exported symbol documentation
- **Cross-Platform Build**: Successful compilation on Linux, macOS, Windows

### Linting Protocol

**Before claiming any task complete**: ALWAYS run `make lint` and `make test`.
Never push code or claim success if either fails.

**DO NOT** modify `.golangci.yml` without explicit approval. The project uses
maratori's golden config v2.5.0 intentionally for strict quality enforcement.

## Multi-Repo Governance

PulumiCost operates as a three-repository ecosystem:

- **pulumicost-core**: CLI tool, plugin host, orchestration engine (this
  repository)
- **pulumicost-spec**: Protocol buffer definitions, SDK generation
- **pulumicost-plugin**: Plugin implementations (Kubecost, Vantage, etc.)

**Cross-Repo Change Protocol**:

1. Protocol changes MUST originate in `pulumicost-spec` with version bump
2. Core MUST update dependencies and adapt to new protocol
3. Plugins MUST update to new SDK version with compatibility testing
4. Changes MUST be coordinated via GitHub Project board (project #3)
5. Documentation MUST be updated across all affected repositories

**Dependency Rules**:

- Core depends on spec (protocol definitions)
- Plugins depend on spec (SDK)
- Core MUST NOT depend on specific plugin implementations
- Spec MUST remain independent (no external dependencies)

## Governance

This constitution supersedes all other development practices and conventions.
All code reviews, pull requests, and architectural decisions MUST verify
compliance with these principles.

**Amendment Process**:

- Amendments require documentation of rationale and impact analysis
- Breaking changes to principles require MAJOR version bump
- New principles or expanded guidance require MINOR version bump
- Clarifications and non-semantic changes require PATCH version bump
- All amendments MUST update this document and propagate changes to dependent
  templates

**Complexity Justification**:

Any violation of principles MUST be explicitly justified with:

1. Clear explanation of why the principle cannot be followed
2. Description of simpler alternatives considered and rejected
3. Documented plan to remediate in future versions

**Implementation Completeness**:

Code changes MUST be complete implementations, not stubs or placeholders:

1. TODO comments are FORBIDDEN in committed code - implement the feature or
   create a tracked issue
2. Stub implementations that bypass actual functionality are FORBIDDEN
3. All task list items MUST be fully implemented before marking complete
4. Tests MUST exercise real behavior, not simulated/mocked results where
   real implementation is required

**Runtime Development Guidance**:

For day-to-day development conventions, tooling preferences, and
project-specific patterns, refer to `CLAUDE.md` in the repository root. That
file provides operational context while this constitution defines
non-negotiable architectural principles.

## Dependency Version Policy

### Pulumi SDK

When using the Pulumi SDK (`github.com/pulumi/pulumi/sdk/v3`):

- **ALWAYS use v3.210.0 or later** for Analyzer plugin development
- The correct import path for protobuf types is:
  `pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"`
  (NOT `github.com/pulumi/pulumi/sdk/v3/proto/go/pulumirpc`)
- Earlier versions may have different package structures or missing types
- This ensures compatibility with the current Pulumi Analyzer protocol

**Version**: 1.5.0 | **Ratified**: 2025-11-06 | **Last Amended**: 2026-01-27