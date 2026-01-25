# FinFocus Strategic Roadmap

This roadmap maps 1:1 with tracked work in GitHub Issues. It outlines the
evolution of `finfocus` while strictly adhering to the technical
guardrails in `CONTEXT.md`.

## Table of Contents

- [Past Milestones](#past-milestones-done)
- [Current Focus (v0.2.5)](#current-focus-v025---testing--stability)
- [Near-Term Vision (v0.3.0)](#near-term-vision-v030---budgeting--intelligence)
- [Future Vision (v0.4.0)](#future-vision-v040---notifications--integrations)
- [Stability & Maintenance](#stability--maintenance)
- [Documentation](#documentation)
- [Icebox / Backlog](#icebox--backlog)

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
- [x] **v0.2.1-v0.2.3: Polish & DX Improvements** *(Released 2026-01-19)*
  - [x] Add `--estimate-confidence` flag for actual cost transparency (#333)
  - [x] Implement GetPluginInfo consumer-side requirements (#376)
  - [x] Parallel plugin metadata fetching in plugin list command (#408)
  - [x] Cross-Repository Integration Test Workflow (#236)
  - [x] Upgrade cost commands to enhanced TUI (#218)
  - [x] Plugin robustness: strict mode, config handlers, metadata fallback (#435, #434, #432, #431)
  - [x] Fallback to latest stable version when asset missing (#430)
  - [x] Budget filtering and summary aggregation logic (#446)
  - [x] Replace manual assertions with testify (#429)
  - [x] Plugin installer: remove old versions during install (#237)
  - [x] E2E test for actual cost command (#334)
  - [x] Set up AWS test account and infrastructure (#181)
  - [x] Fuzzing seeds, benchmarks, and validation improvements (#326)
  - [x] Documentation updates (#182, #349-#353, #454)
- [x] **v0.2.4: Final Polish** *(Completed 2026-01-24)*
  - [x] Dynamic Data Recording via Integration Plans (#275)
  - [x] Harden Nightly Analysis Workflow security and reliability (#325)
  - [x] Documentation for TUI features, budgets, and recommendations (#226)

## Current Focus (v0.2.5 - Testing & Stability)

- [x] **Multi-Region Testing** *(Completed 2026-01-24)*
  - [x] Multi-region E2E testing support (#185, PR #485)
- [x] **Performance** *(Completed 2026-01-24)*
  - [x] Pagination and NDJSON streaming for CI/CD integration (#225, PR #488)

## Near-Term Vision (v0.3.0 - Budgeting & Intelligence)

- [ ] **Plugin Intelligence**
  - [ ] Multi-Plugin Routing: Intelligent Feature-Based Plugin Selection
        ([#410](https://github.com/rshade/finfocus/issues/410))
- [ ] **Budgeting & Cost Controls** *(Budget Health Suite)*
  - [x] Budget health calculation & threshold alerting (#267) *(Completed)*
  - [x] Provider filtering & summary aggregation for Budgets (#263) *(Completed)*
  - [x] Budget status display in CLI (#217) *(Completed)*
  - [ ] Flexible budget scoping (per-provider, per-resource)
        ([#221](https://github.com/rshade/finfocus/issues/221))
  - [ ] Exit codes for budget threshold violations
        ([#219](https://github.com/rshade/finfocus/issues/219))
  - [ ] Namespace filtering & Kubecost metadata handling
        ([#266](https://github.com/rshade/finfocus/issues/266))
  - [ ] Budget health aggregation and multi-provider summary validation tests
        ([#265](https://github.com/rshade/finfocus/issues/265))
  - [ ] Kubecost budget mapping and namespace filtering validation tests
        ([#264](https://github.com/rshade/finfocus/issues/264))
- [ ] **Sustainability (GreenOps)**
  - [x] Integrate Sustainability Metrics into Engine & TUI (#302) *(Completed 2025-12-24)*
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
- [ ] **What-If Analysis**
  - [ ] Add 'cost estimate' command for scenario modeling
        ([#463](https://github.com/rshade/finfocus/issues/463))
        *Cross-Repo: Uses `EstimateCost` RPC from finfocus-spec v0.5.2*
- [ ] **Recommendation Lifecycle**
  - [ ] Add recommendation dismissal and snooze management
        ([#464](https://github.com/rshade/finfocus/issues/464))
        *Cross-Repo: Uses `DismissRecommendation` RPC from finfocus-spec v0.5.2*
- [ ] **Contextual Profiles ("Dev Mode")**
      ([#368](https://github.com/rshade/finfocus/issues/368))
  - [ ] CLI: Implement `--profile` flag (e.g., `dev`, `prod`) to pass hints
        to plugins
  - [ ] Configuration: Allow default profile definition in `finfocus.yaml`
  - *Cross-Repo:* Requires `UsageProfile` enum in
    [finfocus-spec](https://github.com/rshade/finfocus-spec)

## Future Vision (v0.4.0 - Notifications & Integrations)

- [ ] **External Notifications**
  - [ ] Webhook and email notifications for budget alerts
        ([#220](https://github.com/rshade/finfocus/issues/220))
  - *Note:* Requires external service integration to maintain core
    statelessness per CONTEXT.md boundaries

## Stability & Maintenance

- [x] **Quality Gates**
  - [x] Improve CLI package coverage to 75% (achieved 74.5%) (#269)
  - [x] Integration Test Suite for Plugin Communication (#235)
- [x] **Integration Testing Expansion** *(Completed 2026-01-19)*
  - [x] Integration tests for resource filtering and output formats (#319)
  - [x] Integration tests for cross-provider aggregation (#251)
  - [x] Integration tests for `--group-by` flag (#250)
  - [x] Integration tests for `cost actual` command scenarios (#252)
  - [x] Integration tests for config management commands (#254)
  - [x] E2E test for actual cost command
        ([#334](https://github.com/rshade/finfocus/issues/334)) *(Completed 2026-01-19)*
  - [x] Set up AWS test account and infrastructure for E2E testing
        ([#181](https://github.com/rshade/finfocus/issues/181)) *(Completed 2026-01-19)*
- [x] **Fuzzing & Security** *(Completed 2026-01-19)*
  - [x] Create fuzz test skeleton for JSON parser
        ([#330](https://github.com/rshade/finfocus/issues/330))
  - [x] Improve fuzzing seeds, benchmarks, and validation
        ([#326](https://github.com/rshade/finfocus/issues/326))
- [ ] **Plugin SDK Hardening**
  - [ ] Research: Evaluate GetPricingSpec RPC usage in core
        ([#465](https://github.com/rshade/finfocus/issues/465))
- [x] **Code Quality Refactoring** *(Completed 2026-01-18)*
  - [x] Extract shared applyFilters helper (#337) *(Completed 2026-01-18)*
  - [x] Remove redundant .Ctx(ctx) calls in ingest/state.go
        ([#338](https://github.com/rshade/finfocus/issues/338))
  - [x] Pre-allocate slice in GetCustomResourcesWithContext
        ([#339](https://github.com/rshade/finfocus/issues/339))
  - [x] Simplify map conversion in state_test.go
        ([#340](https://github.com/rshade/finfocus/issues/340))

## Documentation

- [x] **User & Developer Guides** *(Completed 2026-01-19)*
  - [x] Expand Support Channels documentation (#353)
  - [x] Expand Troubleshooting Guide (#352)
  - [x] Expand Configuration Guide (#351)
  - [x] Expand Security Guide (#350)
  - [x] Expand Deployment Overview (#349)
  - [x] Update documentation for E2E testing and plugin ecosystem
        ([#182](https://github.com/rshade/finfocus/issues/182)) *(Completed 2026-01-19)*

## Icebox / Backlog

- [ ] TUI Lazy Loading & Error Recovery (#483) *Deferred from TUI Phase 7*
- [ ] Plugin integrity verification strategy (#164)
- [ ] Accessibility options (--no-color, --plain, high contrast) (#224)
- [ ] Configuration validation with helpful error messages (#223)
- [ ] **Plugin Generator Enhancements** *(Deprioritized - solo plugin dev)*
  - [ ] Update Plugin Generator Templates for spec v0.5.x (#248)
  - [ ] Generate Docker support files (#456)
  - [ ] Generate documentation templates (#457)
  - [ ] Add GetPluginInfo and Supports to calculator template (#458)
  - [ ] Add health endpoint to generated main.go (#459)
  - [ ] Enhanced Makefile template with new targets (#460)
  - [ ] Add new CLI flags for generation control (#461)
  - [ ] Generate standardized GitHub workflow files (#462)
  - [ ] Generate .golangci-lint.yml configuration (#493)
- [x] Registry should pick latest version when multiple versions installed (#140)
      *(Completed 2026-01-09)*
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
| What-If Analysis | EstimateCost | cost estimate | PropertyDelta | N/A |
| Rec Lifecycle | DismissRecommendation | dismiss/snooze | Dismiss | N/A |

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
- [x] **Interactive "What-If" Property Tuning**
      ([#463](https://github.com/rshade/finfocus/issues/463)) *Tracked*
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
