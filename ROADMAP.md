# FinFocus Strategic Roadmap

This roadmap maps 1:1 with tracked work in GitHub Issues. It outlines the
evolution of `finfocus` while strictly adhering to the technical
guardrails in `CONTEXT.md`.

## Table of Contents

- [Past Milestones](#past-milestones-done)
- [Current Focus (v0.3.0)](#current-focus-v030---intelligence--analysis)
- [Near-Term Vision (v0.3.x)](#near-term-vision-v03x---forecasting--governance)
- [Future Vision (v0.4.0)](#future-vision-v040---notifications--integrations)
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
  - [x] Budget status display in CLI (#217, PR #466)
- [x] **v0.2.5: Testing & Stability** *(Released 2026-01-30)*
  - [x] Multi-region E2E testing support (#185, PR #485)
  - [x] Pagination and NDJSON streaming for CI/CD integration (#225, PR #488)
  - [x] Exit codes for budget threshold violations
        ([#219](https://github.com/rshade/finfocus/issues/219))
  - [x] Budget health calculation & threshold alerting (#267, PR #494)
  - [x] Provider filtering & summary aggregation for Budgets (#263, PR #494)
- [x] **v0.2.6: Routing & Budget Enhancements** *(Released 2026-02-02)*
  - [x] Intelligent Multi-Plugin Routing with feature-based plugin selection
        ([#410](https://github.com/rshade/finfocus/issues/410), PR #507)
  - [x] Flexible budget scoping (per-provider, per-type, per-tag)
        ([#221](https://github.com/rshade/finfocus/issues/221), PR #509)
  - [x] Sustainability metrics integration in Engine & TUI (#302)
  - [x] GreenOps carbon emission equivalency calculations
        ([#303](https://github.com/rshade/finfocus/issues/303), PR #515)
  - [x] Tag-based budget filtering
        ([#532](https://github.com/rshade/finfocus/issues/532), PR #535)

## Current Focus (v0.3.0 - Intelligence & Analysis)

- [ ] **Multi-Plugin Routing Polish**
  - [ ] Docs formatting & validation.go fix (PR #507 follow-up)
        ([#533](https://github.com/rshade/finfocus/issues/533))
- [x] **What-If Analysis**
  - [x] Add `cost estimate` command for scenario modeling
        ([#463](https://github.com/rshade/finfocus/issues/463), PR #538)
  - *Uses `EstimateCost` RPC from finfocus-spec v0.5.2+*
- [ ] **Recommendation Lifecycle**
  - [ ] Add recommendation dismissal and snooze management
        ([#464](https://github.com/rshade/finfocus/issues/464))
  - *Uses `DismissRecommendation` RPC from finfocus-spec v0.5.2+*
- [ ] **Plugin SDK Hardening**
  - [ ] Research: Evaluate GetPricingSpec RPC usage in core
        ([#465](https://github.com/rshade/finfocus/issues/465))

## Near-Term Vision (v0.3.x - Forecasting & Governance)

- [ ] **Time-Series Forecasting Enhancement**
  - [ ] Enhance `cost estimate` with ARIMA + driver-based forecasting
        ([#539](https://github.com/rshade/finfocus/issues/539))
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

## Icebox / Backlog

- [ ] **Cache Architecture Improvements**
  - [ ] Extract Cache interface and refactor FileStore
        ([#541](https://github.com/rshade/finfocus/issues/541))
  - [ ] Add caching to GetActualCost with 1-hour TTL
        ([#542](https://github.com/rshade/finfocus/issues/542))
  - [ ] Add caching to GetProjectedCost with SHA-based keys
        ([#543](https://github.com/rshade/finfocus/issues/543))
  - [ ] Add optional LRU in-memory cache layer to complement FileStore
        ([#495](https://github.com/rshade/finfocus/issues/495))
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
- [ ] Use registry-based plugin install for cross-repo integration tests
      ([#517](https://github.com/rshade/finfocus/issues/517))
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
