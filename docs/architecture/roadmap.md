---
title: Product Roadmap & Quarterly Priorities
description: Strategic roadmap for finfocus evolution with milestones, vision, and cross-repository dependencies
layout: default
---

<!-- markdownlint-disable MD025 -->

## finfocus Strategic Roadmap

This roadmap maps 1:1 with tracked work in GitHub Issues. It outlines the
evolution of `finfocus` while strictly adhering to the technical
guardrails in `CONTEXT.md`.

## Past Milestones (Done)

- [x] **v0.1.0-v0.1.2: Foundation & Observability**
  - [x] Initial CLI & gRPC Plugin System (#163, #15)
  - [x] Standardized SDK & Interceptors (#188, #189, #191)
  - [x] Zerolog Integration & Structured Logging (#170, #206)
  - [x] Engine Test Coverage Completion (#202, #207)
  - [x] Plugin Ecosystem Maturity (#201, #215)
  - [x] Support for `Supports()` gRPC handler (#160, #165)

## Current Focus (v0.2.0 - Automation & Stability)

- [ ] **Pulumi Analyzer Integration & E2E**
  - [x] Core Analyzer implementation (#245, #229)
  - [x] E2E testing with Pulumi Automation API (#177, #238)
  - [x] Comprehensive E2E tests for Analyzer integration
        ([#228](https://github.com/rshade/finfocus/issues/228))
  - [ ] Cross-Repository Integration Test Workflow
        ([#236](https://github.com/rshade/finfocus/issues/236))
  - [x] Add recommendations to analyzer diagnostics
        ([#321](https://github.com/rshade/finfocus/issues/321))
- [ ] **Developer Experience & Tooling**
  - [ ] Update Plugin Generator Templates (includes gRPC reflection)
        ([#248](https://github.com/rshade/finfocus/issues/248))
  - [ ] Dynamic Data Recording via Integration Plans
        ([#275](https://github.com/rshade/finfocus/issues/275))
- [ ] **Enhanced Visualization**
  - [x] Shared TUI package with Bubble Tea (#222, #258)
  - [ ] Upgrade cost commands to enhanced TUI
        ([#218](https://github.com/rshade/finfocus/issues/218))

## Near-Term Vision (v0.3.0 - Intelligence & Sustainability)

- [ ] **Budgeting & Cost Controls**
  - [ ] Budget health calculation & threshold alerting
        ([#267](https://github.com/rshade/finfocus/issues/267))
  - [ ] Provider filtering & summary aggregation for Budgets
        ([#263](https://github.com/rshade/finfocus/issues/263))
  - [ ] Flexible budget scoping (per-provider, per-resource)
        ([#221](https://github.com/rshade/finfocus/issues/221))
  - [ ] Budget status display in CLI
        ([#217](https://github.com/rshade/finfocus/issues/217))
- [ ] **Sustainability (GreenOps)**
  - [x] Integrate Sustainability Metrics into Engine & TUI
        ([#302](https://github.com/rshade/finfocus/issues/302))
  - [ ] GreenOps Impact Equivalencies
        ([#303](https://github.com/rshade/finfocus/issues/303))
- [ ] **Actionable Insights**
  - [ ] Recommendations command for FinOps optimization
        ([#216](https://github.com/rshade/finfocus/issues/216))
  - [ ] Support extended RecommendationActionType enum
        ([#298](https://github.com/rshade/finfocus/issues/298))
  - [x] Add --estimate-confidence flag for actual cost transparency
        ([#333](https://github.com/rshade/finfocus/issues/333)) *(Completed 2026-01-18)*
- [ ] **Forecasting & Projections ("Cost Time Machine")**
      ([#364](https://github.com/rshade/finfocus/issues/364))
  - [ ] Projection Math Engine (Linear/Exponential extrapolation)
  - [ ] TUI: ASCII Line Chart visualization for 6-12 month forecasts
  - _Cross-Repo:_ Requires `GrowthType`/`GrowthRate` in
    [finfocus-spec](https://github.com/rshade/finfocus-spec)
- [ ] **Governance Overrides ("YOLO Mode")**
      ([#365](https://github.com/rshade/finfocus/issues/365))
  - [ ] CLI: Implement `--yolo` / `--force` flag to bypass budget gates
  - [ ] UX: "Warning Mode" UI styles for bypassed runs
  - _Cross-Repo:_ Requires `BypassReason` in
    [finfocus-spec](https://github.com/rshade/finfocus-spec)
- [ ] **Contextual Profiles ("Dev Mode")**
      ([#368](https://github.com/rshade/finfocus/issues/368))
  - [ ] CLI: Implement `--profile` flag (e.g., `dev`, `prod`) to pass hints
        to plugins
  - [ ] Configuration: Allow default profile definition in `finfocus.yaml`
  - _Cross-Repo:_ Requires `UsageProfile` enum in
    [finfocus-spec](https://github.com/rshade/finfocus-spec)

## Stability & Maintenance

- [x] **Quality Gates**
  - [x] Improve CLI package coverage to 75% (achieved 74.5%)
        ([#269](https://github.com/rshade/finfocus/issues/269))
  - [x] Integration Test Suite for Plugin Communication
        ([#235](https://github.com/rshade/finfocus/issues/235))
- [ ] **Performance & Scale**
  - [ ] Pagination for large datasets
        ([#225](https://github.com/rshade/finfocus/issues/225))
  - [ ] Plugin installer: remove old versions during install
        ([#237](https://github.com/rshade/finfocus/issues/237))
- [ ] **CI/CD & Automation**
  - [ ] Harden Nightly Analysis Workflow
        ([#325](https://github.com/rshade/finfocus/issues/325))
  - [ ] Automated nightly failure analysis with OpenCode
        ([#271](https://github.com/rshade/finfocus/issues/271))

## Icebox / Backlog

- [ ] Plugin integrity verification strategy (#164)
- [ ] Webhook and email notifications for budget alerts (#220) - _Likely
      requires external service integration to maintain core statelessness_
- [ ] Vantage Plugin Integration (#103) - _Deprioritized_
- [ ] **Dependency Visualization ("Blast Radius")**
      ([#366](https://github.com/rshade/finfocus/issues/366))
  - [ ] TUI: Interactive Dependency Tree view (consuming Lineage Metadata)
  - _Cross-Repo:_ Consumes `CostAllocationLineage`/`ParentResourceID` from
    [finfocus-spec](https://github.com/rshade/finfocus-spec)
- [ ] **Spot Market Advisor**
      ([#367](https://github.com/rshade/finfocus/issues/367))
  - [ ] TUI: Highlight Spot savings in Cyan; show Risk Icon
  - [ ] Display "Savings vs On-Demand" percentage
  - _Cross-Repo:_ Requires `PricingTier`/`SpotRisk` enums in
    [finfocus-spec](https://github.com/rshade/finfocus-spec); CE plugin
    implements `DescribeSpotPriceHistory`
- [ ] **Mixed-Currency Aggregation Strategy (MCP Alignment)**
  - _Objective_: Implement core-level grouping for multi-currency stacks to
    support the [finfocus-mcp Mixed-Currency
    Research](https://github.com/rshade/finfocus-mcp/blob/main/ROADMAP.md#1-mixed-currency-aggregation-strategy).
  - _Technical Approach_: Enhance `CostResult` aggregation logic to preserve
    currency codes and provide structured groupings for downstream consumers
    (CLI, TUI, MCP).
  - _Success Criteria_: Orchestrator returns grouped results by currency when
    multi-region/multi-currency resources are encountered.

### Cross-Repository Feature Matrix

| Feature             | spec            | core        | aws-public   | aws-ce      |
| ------------------- | --------------- | ----------- | ------------ | ----------- |
| Cost Time Machine   | GrowthType      | Projection  | GrowthHint   | Historical  |
| YOLO Mode           | BypassReason    | --yolo flag | N/A          | N/A         |
| Blast Radius        | Lineage         | Impact Tree | Parent/child | N/A         |
| GreenOps Receipt    | CarbonFootprint | Converter   | CCF Math     | N/A         |
| Spot Market Advisor | PricingTier     | Cyan style  | N/A          | SpotHistory |
| Dev Mode            | UsageProfile    | --profile   | Burstable    | IOPS warn   |

### Strategic Research Items (The "Detailed Horizon")

- [ ] **Markdown "Cost-Change" Report & CI/CD Bridge**
  - _Objective_: Enable automated PR feedback by providing a Git-native
    visualization of cost deltas.
  - _Technical Approach_: Implement a new `OutputFormatter` that translates
    `CostResult` maps into GFM (GitHub Flavored Markdown) using collapsible
    `<details>` tags for per-resource breakdowns.
  - _Anti-Guess Boundary_: The engine MUST NOT calculate the delta itself if
    it isn't already provided by the input source; it strictly formats data
    returned by the orchestration layer.
  - _Success Criteria_: A valid GFM document is generated that renders
    correctly in a GitHub comment using only data from the `CostResult` array.
- [ ] **Interactive "What-If" Property Tuning**
  - _Objective_: Allow developers to explore pricing alternatives for a
    resource in real-time without modifying Pulumi code.
  - _Technical Approach_: Extend the TUI to allow key-value editing of a
    `ResourceDescriptor.Properties` map and re-triggering the
    `Engine.GetProjectedCost` gRPC call.
  - _Anti-Guess Boundary_: The core MUST NOT contain any logic to determine
    which properties affect price; it must blindly pass the user-modified map
    to the gRPC plugin and display the response.
  - _Success Criteria_: The TUI refreshes a resource's price after an
    in-memory property change by receiving and displaying a new `CostResult`
    from the plugin.
- [ ] **OpenCost Compatibility Mapping**
  - _Objective_: Integrate `finfocus` with the broader FinOps ecosystem by
    supporting standardized data exchange formats.
  - _Technical Approach_: Create a transformation layer that maps the
    `finfocus.CostResult` struct to the JSON schema defined by the
    [OpenCost Specification](https://www.opencost.io/).
  - _Anti-Guess Boundary_: The core MUST NOT attempt to synthesize missing
    OpenCost fields (e.g., Kubernetes metadata); if the data is not present
    in the resource descriptor, the field must remain null.
  - _Success Criteria_: Generated JSON output passes the official OpenCost
    schema validation.
- [ ] **Stateless Cost-Policy Linting**
  - _Objective_: Prevent accidental cost overruns by flagging resources that
    exceed organizational informational thresholds.
  - _Technical Approach_: Compare the `Monthly` field of a `CostResult`
    against a static threshold defined in a local `policy.yaml`.
  - _Anti-Guess Boundary_: This is a comparison-only feature; the core MUST
    NOT attempt to "optimize" or "suggest remediation" for the resource
    configuration.
  - _Success Criteria_: The CLI produces a "Policy Violated" diagnostic when
    a plugin-returned cost exceeds the user-defined threshold.

---

**Verification Questions:**

1. **Statelessness Conflicts**: Issue #138 (Caching) and #137
   (Metrics/Telemetry) suggest persistent state. Should these be reframed to
   focus on _integration_ with external systems (e.g., Prometheus/Redis)
   rather than internal core implementation?
2. **Feature Bloom**: Issue #141 (Distributed calculation) seems out of scope
   for a "Lightweight Orchestrator." Should this be closed as "Will Not Do"?
3. **Budget Health**: Feature #267 assumes thresholds are stored in
   `~/.finfocus/config.yaml`. Does this local config-driven approach
   satisfy our "transient to the process" mandate?
