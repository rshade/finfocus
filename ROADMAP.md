---
title: finfocus Strategic Roadmap
description: >-
  Strategic roadmap mapping 1:1 with GitHub Issues, outlining the evolution
  of finfocus while adhering to technical guardrails.
layout: docs
---

This roadmap maps 1:1 with tracked work in GitHub Issues. It outlines the
evolution of `finfocus` while strictly adhering to the technical
guardrails in `CONTEXT.md`.

## Table of Contents

- [Past Milestones](#past-milestones-done)
- [Immediate Priority](#immediate-priority-bug-fixes)
- [Current Focus (v0.2.1)](#current-focus-v021---polish--dx-improvements)
- [Near-Term Vision (v0.3.0)](#near-term-vision-v030---budgeting--intelligence)
- [Stability & Maintenance](#stability--maintenance)
- [Documentation](#documentation)
- [Icebox / Backlog](#icebox--backlog)
- [Naming & Branding](#naming--branding)

## Past Milestones (Done)

- [x] **v0.1.0-v0.1.2: Foundation & Observability**
  - [x] Initial CLI & gRPC Plugin System (#163, #15)
  - [x] Standardized SDK & Interceptors (#188, #189, #191)
  - [x] Zerolog Integration & Structured Logging (#170, #206)
  - [x] Engine Test Coverage Completion (#202, #207)
  - [x] Plugin Ecosystem Maturity (#201, #215)
  - [x] Support for `Supports()` gRPC handler (#160, #165)
  - [x] CLI Filter Flag (#203)
  - [x] Test Infrastructure Hardening (#200)
- [x] **v0.1.3-v0.1.5: Analyzer & Recommendations**
  - [x] Core Analyzer implementation (#245, #229)
  - [x] E2E testing with Pulumi Automation API (#177, #238)
  - [x] Comprehensive E2E tests for Analyzer integration (#228)
  - [x] Add recommendations to analyzer diagnostics (#321)
  - [x] Shared TUI package with Bubble Tea (#222, #258)
  - [x] E2E and Conformance test reliability fixes (#323)
  - [x] AWS fallback scope and non-deterministic output fixes (#324)
- [x] **v0.2.0: State-Based Costs & Plugin Maturity** *(Released 2026-01-16)*
  - [x] State-based actual cost estimation for `cost actual` (#380, #382)
  - [x] Plugin info and dry-run discovery (#398)
  - [x] Ecosystem rebrand to FinFocus (#415)

## Immediate Priority (Bug Fixes)

- [x] **Test Reliability & CI Stability** *(Completed 2026-01-17)*
  - [x] Nightly test failures resolved (#417, #414, #424)

## Current Focus (v0.2.1 - Polish & DX Improvements)

- [ ] **Cost Estimation Enhancements**
  - [ ] Add `--estimate-confidence` flag for actual cost transparency
        ([#333](https://github.com/rshade/finfocus/issues/333))
- [ ] **Plugin Ecosystem Maturity**
  - [ ] Implement GetPluginInfo consumer-side requirements
        ([#376](https://github.com/rshade/finfocus/issues/376))
        *Status: Unblocked (Spec v0.4.12+)*
  - [ ] Update Plugin Generator Templates (includes gRPC reflection)
        ([#248](https://github.com/rshade/finfocus/issues/248))
- [ ] **Developer Experience & Tooling**
  - [ ] Dynamic Data Recording via Integration Plans
        ([#275](https://github.com/rshade/finfocus/issues/275))
  - [ ] Cross-Repository Integration Test Workflow
        ([#236](https://github.com/rshade/finfocus/issues/236))
  - [ ] Multi-Plugin Routing: Intelligent Feature-Based Plugin Selection
        ([#410](https://github.com/rshade/finfocus/issues/410))
  - [x] Parallel plugin metadata fetching in plugin list command
        ([#408](https://github.com/rshade/finfocus/issues/408)) *(Completed 2026-01-17)*
- [x] **Enhanced Visualization**
  - [x] Upgrade cost commands to enhanced TUI (#218) *(Completed)*
- [x] **Code Quality**
  - [x] Fix CodeRabbit issues from #398 (#412) *(Completed 2026-01-17)*
- [ ] **Deferred from v0.1.x**
  - [ ] Pagination for large datasets
        ([#225](https://github.com/rshade/finfocus/issues/225))
  - [ ] Plugin installer: remove old versions during install
        ([#237](https://github.com/rshade/finfocus/issues/237))

## Near-Term Vision (v0.3.0 - Budgeting & Intelligence)

- [ ] **Budgeting & Cost Controls** *(Budget Health Suite)*
  - [ ] Budget health calculation & threshold alerting
        ([#267](https://github.com/rshade/finfocus/issues/267))
  - [ ] Provider filtering & summary aggregation for Budgets
        ([#263](https://github.com/rshade/finfocus/issues/263))
  - [ ] Budget status display in CLI
        ([#217](https://github.com/rshade/finfocus/issues/217))
  - [ ] Flexible budget scoping (per-provider, per-resource)
        ([#221](https://github.com/rshade/finfocus/issues/221))
  - [ ] Exit codes for budget threshold violations
        ([#219](https://github.com/rshade/finfocus/issues/219))
  - [ ] Namespace filtering & Kubecost metadata handling
        ([#266](https://github.com/rshade/finfocus/issues/266))
- [ ] **Sustainability (GreenOps)**
  - [x] Integrate Sustainability Metrics into Engine & TUI (#302)
  - [ ] GreenOps Impact Equivalencies
        ([#303](https://github.com/rshade/finfocus/issues/303))
- [ ] **Forecasting & Projections ("Cost Time Machine")**
      ([#364](https://github.com/rshade/finfocus/issues/364))
  - [ ] Projection Math Engine (Linear/Exponential extrapolation)
  - [ ] TUI: ASCII Line Chart visualization for 6-12 month forecasts
  - *Status: Spec primitives available (GrowthType/GrowthRate)*
- [ ] **Governance Overrides ("YOLO Mode")**
      ([#365](https://github.com/rshade/finfocus/issues/365))
  - [ ] CLI: Implement `--yolo` / `--force` flag to bypass budget gates
  - [ ] UX: "Warning Mode" UI styles for bypassed runs
  - *Cross-Repo:* Requires `BypassReason` in
    [finfocus-spec](https://github.com/rshade/finfocus-spec)
- [ ] **Interoperability & Data Exchange**
  - [ ] Implement JSON-LD export for FOCUS cost records
        *(Issue TBD - Cross-Repo: Uses sdk/go/jsonld from spec 032)*
  - [ ] OpenCost Compatibility Mapping
        *(Issue TBD)*
- [ ] **Contextual Profiles ("Dev Mode")**
      ([#368](https://github.com/rshade/finfocus/issues/368))
  - [ ] CLI: Implement `--profile` flag (e.g., `dev`, `prod`) to pass hints
        to plugins
  - [ ] Configuration: Allow default profile definition in `finfocus.yaml`
  - *Cross-Repo:* Requires `UsageProfile` enum in
    [finfocus-spec](https://github.com/rshade/finfocus-spec)

## Stability & Maintenance

- [x] **Quality Gates**
  - [x] Improve CLI package coverage to 75% (achieved 74.5%) (#269)
  - [x] Integration Test Suite for Plugin Communication (#235)
- [ ] **Integration Testing Expansion**
  - [ ] Integration tests for resource filtering and output formats
        ([#319](https://github.com/rshade/finfocus/issues/319))
  - [ ] Integration tests for cross-provider aggregation
        ([#251](https://github.com/rshade/finfocus/issues/251))
  - [ ] Integration tests for `--group-by` flag
        ([#250](https://github.com/rshade/finfocus/issues/250))
  - [ ] Integration tests for `cost actual` command scenarios
        ([#252](https://github.com/rshade/finfocus/issues/252))
  - [ ] Integration tests for config management commands
        ([#254](https://github.com/rshade/finfocus/issues/254))
  - [ ] E2E test for actual cost command
        ([#334](https://github.com/rshade/finfocus/issues/334))
- [ ] **Fuzzing & Security**
  - [ ] Create fuzz test skeleton for JSON parser
        ([#330](https://github.com/rshade/finfocus/issues/330))
  - [ ] Improve fuzzing seeds, benchmarks, and validation
        ([#326](https://github.com/rshade/finfocus/issues/326))
- [ ] **CI/CD & Automation**
  - [ ] Harden Nightly Analysis Workflow
        ([#325](https://github.com/rshade/finfocus/issues/325))
- [ ] **Plugin SDK Hardening**
  - [ ] Implement configurable CORS & Security headers for plugin servers
        *(Issue TBD)*
  - [ ] Adopt enhanced ARN provider type safety
        *(Issue TBD)*
- [ ] **Code Quality Refactoring**
  - [ ] Extract shared applyFilters helper
        ([#337](https://github.com/rshade/finfocus/issues/337))
  - [ ] Remove redundant .Ctx(ctx) calls in ingest/state.go
        ([#338](https://github.com/rshade/finfocus/issues/338))
  - [ ] Pre-allocate slice in GetCustomResourcesWithContext
        ([#339](https://github.com/rshade/finfocus/issues/339))
  - [ ] Simplify map conversion in state_test.go
        ([#340](https://github.com/rshade/finfocus/issues/340))

## Documentation

- [ ] **User & Developer Guides**
  - [ ] Expand Support Channels documentation
        ([#353](https://github.com/rshade/finfocus/issues/353))
  - [ ] Expand Troubleshooting Guide
        ([#352](https://github.com/rshade/finfocus/issues/352))
  - [ ] Expand Configuration Guide
        ([#351](https://github.com/rshade/finfocus/issues/351))
  - [ ] Expand Security Guide
        ([#350](https://github.com/rshade/finfocus/issues/350))
  - [ ] Expand Deployment Overview
        ([#349](https://github.com/rshade/finfocus/issues/349))
  - [ ] Update documentation for TUI features, budgets, and recommendations
        ([#226](https://github.com/rshade/finfocus/issues/226))

## Icebox / Backlog

- [ ] Plugin integrity verification strategy (#164)
- [ ] Webhook and email notifications for budget alerts (#220) - *Likely
      requires external service integration to maintain core statelessness*
- [ ] Accessibility options (--no-color, --plain, high contrast) (#224)
- [ ] Configuration validation with helpful error messages (#223)
- [ ] Registry should pick latest version when multiple versions installed (#140)
- [ ] Plugin developer upgrade command for SDK migrations (#270) - *Research*
- [ ] **Dependency Visualization ("Blast Radius")**
      ([#366](https://github.com/rshade/finfocus/issues/366))
  - [ ] TUI: Interactive Dependency Tree view (consuming Lineage Metadata)
  - *Cross-Repo:* Consumes `CostAllocationLineage`/`ParentResourceID` from
    [finfocus-spec](https://github.com/rshade/finfocus-spec)
- [ ] **Spot Market Advisor**
      ([#367](https://github.com/rshade/finfocus/issues/367))
  - [ ] TUI: Highlight Spot savings in Cyan; show Risk Icon
  - [ ] Display "Savings vs On-Demand" percentage
  - *Cross-Repo:* Requires `PricingTier`/`SpotRisk` enums in
    [finfocus-spec](https://github.com/rshade/finfocus-spec); CE plugin
    implements `DescribeSpotPriceHistory`
- [ ] **Mixed-Currency Aggregation Strategy (MCP Alignment)**
  - *Objective*: Implement core-level grouping for multi-currency stacks to
    support the [finfocus-mcp Mixed-Currency
    Research](https://github.com/rshade/finfocus-mcp/blob/main/ROADMAP.md#1-mixed-currency-aggregation-strategy).
  - *Technical Approach*: Enhance `CostResult` aggregation logic to preserve
    currency codes and provide structured groupings for downstream consumers
    (CLI, TUI, MCP).
  - *Success Criteria*: Orchestrator returns grouped results by currency when
    multi-region/multi-currency resources are encountered.

### Cross-Repository Feature Matrix

| Feature | spec | core | aws-public | aws-ce |
| ------- | ---- | ---- | ---------- | ------ |
| Cost Time Machine | GrowthType | Projection | GrowthHint | Historical |
| YOLO Mode | BypassReason | --yolo flag | N/A | N/A |
| Blast Radius | Lineage | Impact Tree | Parent/child | N/A |
| GreenOps Receipt | CarbonFootprint | Converter | CCF Math | N/A |
| Spot Market Advisor | PricingTier | Cyan style | N/A | SpotHistory |
| Dev Mode | UsageProfile | --profile | Burstable | IOPS warn |

## Naming & Branding

- [ ] **Project Rename**
  - *Objective*: Replace the dry `finfocus` name with a stronger brand identity.
  - *Current Leader*: **Tailly** (CLI: `tally`)
    - *Vibe*: Developer-friendly, ecosystem-native (the "Tail" of the Pulumi Platypus).
    - *Command*: `tally cost projected` (avoiding `tail` conflict while
      preserving clear verb structure).
    - *Mascot Potential*: High (Platypus).
  - *Alternative*: **FinFocus** (CLI: `fin`)
    - *Vibe*: Enterprise, compliance-focused (FinOps FOCUS spec).
    - *Command*: `fin cost projected`.
  - *Decision*: Leaning towards **Tailly** for better DX; **FinFocus**
    offers stronger enterprise signaling.

### Strategic Research Items (The "Detailed Horizon")

- [ ] **Markdown "Cost-Change" Report & CI/CD Bridge**
  - *Objective*: Enable automated PR feedback by providing a Git-native
    visualization of cost deltas.
  - *Technical Approach*: Implement a new `OutputFormatter` that translates
    `CostResult` maps into GFM (GitHub Flavored Markdown) using collapsible
    `<details>` tags for per-resource breakdowns.
  - *Anti-Guess Boundary*: The engine MUST NOT calculate the delta itself if
    it isn't already provided by the input source; it strictly formats data
    returned by the orchestration layer.
  - *Success Criteria*: A valid GFM document is generated that renders
    correctly in a GitHub comment using only data from the `CostResult` array.
- [ ] **Interactive "What-If" Property Tuning**
  - *Objective*: Allow developers to explore pricing alternatives for a
    resource in real-time without modifying Pulumi code.
  - *Technical Approach*: Extend the TUI to allow key-value editing of a
    `ResourceDescriptor.Properties` map and re-triggering the
    `Engine.GetProjectedCost` gRPC call.
  - *Anti-Guess Boundary*: The core MUST NOT contain any logic to determine
    which properties affect price; it must blindly pass the user-modified map
    to the gRPC plugin and display the response.
  - *Success Criteria*: The TUI refreshes a resource's price after an
    in-memory property change by receiving and displaying a new `CostResult`
    from the plugin.
- [ ] **Stateless Cost-Policy Linting**
  - *Objective*: Prevent accidental cost overruns by flagging resources that
    exceed organizational informational thresholds.
  - *Technical Approach*: Compare the `Monthly` field of a `CostResult`
    against a static threshold defined in a local `policy.yaml`.
  - *Anti-Guess Boundary*: This is a comparison-only feature; the core MUST
    NOT attempt to "optimize" or "suggest remediation" for the resource
    configuration.
  - *Success Criteria*: The CLI produces a "Policy Violated" diagnostic when
    a plugin-returned cost exceeds the user-defined threshold.

---

**Verification Questions:**

1. **Statelessness Conflicts**: Issue #138 (Caching) and #137
   (Metrics/Telemetry) suggest persistent state. Should these be reframed to
   focus on *integration* with external systems (e.g., Prometheus/Redis)
   rather than internal core implementation?
2. **Feature Bloom**: Issue #141 (Distributed calculation) seems out of scope
   for a "Lightweight Orchestrator." Should this be closed as "Will Not Do"?
3. **Budget Health**: Feature #267 assumes thresholds are stored in
   `~/.finfocus/config.yaml`. Does this local config-driven approach
   satisfy our "transient to the process" mandate?
