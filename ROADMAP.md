# FinFocus Strategic Roadmap

This roadmap maps 1:1 with tracked work in GitHub Issues. It outlines the
evolution of `finfocus` while strictly adhering to the technical
guardrails in `CONTEXT.md`.

## Table of Contents

- [Immediate Focus (v0.3.0)](#immediate-focus-v030---install-ux-scale--pulumi-integration)
- [Near-Term Vision (v0.3.x)](#near-term-vision-v03x---forecasting--profiles)
- [Future Vision (v0.4.0+)](#future-vision-v040---notifications-integrations--backlog)
- [Completed Milestones](#completed-milestones)
- [Cross-Repository Feature Matrix](#cross-repository-feature-matrix)
- [Boundary Safeguards](#boundary-safeguards)

## Immediate Focus (v0.3.0 - Install UX, Scale & Pulumi Integration)

- [ ] **Plugin SDK Hardening**
  - [ ] Research: Evaluate GetPricingSpec RPC usage in core
        ([#465](https://github.com/rshade/finfocus/issues/465))
- [ ] **Config Architecture**
  - [ ] Split project-local and user-global `.finfocus/` directories
        ([#548](https://github.com/rshade/finfocus/issues/548))
- [ ] **Pulumi Auto-Detection Follow-up**
  - [ ] CodeRabbit follow-up cleanup from auto-detect PR
        ([#589](https://github.com/rshade/finfocus/issues/589))
- [ ] **Install UX**
  - [ ] Install script (`curl | sh`)
        ([#599](https://github.com/rshade/finfocus/issues/599))
  - [ ] `finfocus setup` one-command bootstrap
        ([#598](https://github.com/rshade/finfocus/issues/598))
  - [ ] `finfocus analyzer install/uninstall` commands
        ([#597](https://github.com/rshade/finfocus/issues/597))
  - [ ] Checksum verification for plugin installation
        ([#601](https://github.com/rshade/finfocus/issues/601))
- [ ] **Scale & Performance**
  - [ ] Scale benchmarks for cost commands
        ([#607](https://github.com/rshade/finfocus/issues/607))
  - [ ] `--jobs` flag and timing output for cost commands
        ([#602](https://github.com/rshade/finfocus/issues/602))
  - [ ] Projected cost caching
        ([#600](https://github.com/rshade/finfocus/issues/600))
- [ ] **CLI Polish**
  - [ ] Neo-friendly CLI fixes
        ([#611](https://github.com/rshade/finfocus/issues/611))
  - [ ] Policy-compatible cost output
        ([#604](https://github.com/rshade/finfocus/issues/604))
- [ ] **Bug Fixes**
  - [ ] Deep copy CostBreakdown in appendActualCostResults to prevent
        source mutation
        ([#614](https://github.com/rshade/finfocus/issues/614))
  - [ ] Phantom $0 results from empty plugin responses
        ([#595](https://github.com/rshade/finfocus/issues/595))
- [ ] **Code Quality & Refactoring**
  - [ ] Reorder router provider-based region check after feature matching
        ([#616](https://github.com/rshade/finfocus/issues/616))
  - [ ] Support GCP zone normalization in normalizeToRegion
        ([#615](https://github.com/rshade/finfocus/issues/615))
  - [ ] Add `.Ctx(ctx)` and structured log fields across packages
        ([#613](https://github.com/rshade/finfocus/issues/613))
  - [ ] Add Stack field to CostFlags struct
        ([#612](https://github.com/rshade/finfocus/issues/612))
  - [ ] Consolidate recommendation count and format helpers (DRY)
        ([#610](https://github.com/rshade/finfocus/issues/610))
  - [ ] Wrap errors from MapResources, MapStateResources, and
        resolveOverviewData
        ([#609](https://github.com/rshade/finfocus/issues/609))
  - [ ] Wire router into cost commands for region-aware plugin selection
        ([#590](https://github.com/rshade/finfocus/issues/590))
- [ ] **Testing Improvements**
  - [ ] Add negative test for waitForPluginBindWithFallback
        ([#608](https://github.com/rshade/finfocus/issues/608))
  - [ ] Fix state_test.go wantVersion skip and delegation fragility
        ([#606](https://github.com/rshade/finfocus/issues/606))
  - [ ] Isolate auto-detection tests with temp directories
        ([#605](https://github.com/rshade/finfocus/issues/605))
  - [ ] Use comma-ok idiom for altMap assertions
        ([#603](https://github.com/rshade/finfocus/issues/603))
- [ ] **Recorder Plugin Fixes**
  - [ ] Recorder plugin should not declare ACTUAL_COSTS capability
        ([#596](https://github.com/rshade/finfocus/issues/596))

## Near-Term Vision (v0.3.x - Forecasting & Profiles)

- [ ] **Resource Filtering Improvements**
  - [ ] Filter `pulumi:providers:*` synthetic resources from cost plugin routing
        ([#582](https://github.com/rshade/finfocus/issues/582))
  - [ ] Filter Pulumi component resources from cost plugin routing
        ([#583](https://github.com/rshade/finfocus/issues/583))
- [ ] **Unified Cost Dashboard**
  - [ ] Add `finfocus overview` command — unified cost dashboard with TUI
        ([#578](https://github.com/rshade/finfocus/issues/578))
- [ ] **Contextual Profiles ("Dev Mode")**
      ([#368](https://github.com/rshade/finfocus/issues/368))
  - [ ] CLI: Implement `--profile` flag (e.g., `dev`, `prod`) to pass hints
        to plugins
  - [ ] Configuration: Allow default profile definition in `finfocus.yaml`
  - *Spec ready:* `UsageProfile` enum (PROD/DEV/BURST) available in
    finfocus-spec v0.5.5 — core-only implementation
- [ ] **Time-Series Forecasting Enhancement**
  - [ ] Enhance `cost estimate` with ARIMA + driver-based forecasting
        ([#539](https://github.com/rshade/finfocus/issues/539))
- [ ] **Forecasting & Projections ("Cost Time Machine")**
      ([#364](https://github.com/rshade/finfocus/issues/364))
  - [ ] Projection Math Engine (Linear/Exponential extrapolation)
  - [ ] TUI: ASCII Line Chart visualization for 6-12 month forecasts
  - *Status: Spec primitives available (GrowthType/GrowthRate)*

## Future Vision (v0.4.0+ - Notifications, Integrations & Backlog)

- [ ] **Governance Overrides ("YOLO Mode")**
      ([#365](https://github.com/rshade/finfocus/issues/365))
  - [ ] CLI: Implement `--yolo` / `--force` flag to bypass budget gates
  - [ ] UX: "Warning Mode" UI styles for bypassed runs
  - *Blocked:* Requires `BypassReason` enum in
    [finfocus-spec](https://github.com/rshade/finfocus-spec) (not yet defined)
- [ ] **External Notifications**
  - [ ] Webhook and email notifications for budget alerts
        ([#220](https://github.com/rshade/finfocus/issues/220))
  - *Note:* Requires external service integration to maintain core
    statelessness per CONTEXT.md boundaries
- [ ] **Recommendation Lifecycle Enhancements** *(spec-first)*
  - [ ] Add `include_dismissed` field to GetRecommendationsRequest
        ([#545](https://github.com/rshade/finfocus/issues/545))
  - [ ] Add GetRecommendationHistory RPC to CostSourceService
        ([#546](https://github.com/rshade/finfocus/issues/546))
- [ ] **Cost Time Machine** *(depends on #548)*
  - [ ] Phase 1 — MVP: `cost history collect` + `view --plain` with
        asciigraph and bbolt storage
        ([#549](https://github.com/rshade/finfocus/issues/549))
  - [ ] Phase 2 — Interactive TUI: ntcharts TimeSeriesLineChart with
        pan/zoom, provider split, budget overlay
        ([#550](https://github.com/rshade/finfocus/issues/550))
  - [ ] Phase 3 — Export & Sparklines: JSON/CSV/NDJSON export + inline
        sparkline indicators
        ([#551](https://github.com/rshade/finfocus/issues/551))
  - [ ] Cost history diff: resource-level change attribution
        ([#554](https://github.com/rshade/finfocus/issues/554))
  - [ ] Prune & retention policy
        ([#555](https://github.com/rshade/finfocus/issues/555))
  - [ ] Mixed-currency snapshot handling
        ([#556](https://github.com/rshade/finfocus/issues/556))
  - [ ] CI/CD automation recipes (documentation)
        ([#553](https://github.com/rshade/finfocus/issues/553))
- [ ] **Bubble Tea v2 Upgrade**
  - [ ] Upgrade to Bubble Tea v2, Lip Gloss v2, Bubbles v2 (after stable
        release)
        ([#552](https://github.com/rshade/finfocus/issues/552))
  - *Blocked: Bubble Tea v2 must exit release candidate status*
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
- [ ] **Plugin Generator Enhancements** *(Deprioritized — solo plugin dev)*
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
- [ ] Plugin developer upgrade command for SDK migrations (#270) — *Research*
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
- [ ] **Cost Diff View**
  - [ ] Add cost diff view to `cost projected` command
        ([#576](https://github.com/rshade/finfocus/issues/576))
- [ ] **Overview Command Enhancements**
  - [ ] Add `--output json` support to `finfocus overview`
        ([#579](https://github.com/rshade/finfocus/issues/579))
- [ ] **Platform Reliability**
  - [ ] Reimplement plugin installer lock for Windows reliability
        ([#573](https://github.com/rshade/finfocus/issues/573))
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

## Completed Milestones

### 2026-Q1

- [x] **Bug Fixes & Stability** *(Completed 2026-02-14)*
  - [x] Fall back to filesystem discovery for plugin removal
        ([#592](https://github.com/rshade/finfocus/issues/592), PR #621)
  - [x] Auto-create log directory before opening log file
        ([#591](https://github.com/rshade/finfocus/issues/591), PR #618)
  - [x] Move EnsureLogDir() after debug/env overrides
        ([#617](https://github.com/rshade/finfocus/issues/617))
- [x] **Pulumi Auto-Detection** *(Completed 2026-02-13)*
  - [x] Automatic Pulumi project detection for cost commands
        ([#581](https://github.com/rshade/finfocus/issues/581), PR #586)
- [x] **TUI & Documentation Polish** *(Completed 2026-02-13)*
  - [x] Display recommendations in resource detail view for cost
        projected/actual
        ([#575](https://github.com/rshade/finfocus/issues/575))
  - [x] Document aws-public projected cost gaps for diff support
        ([#577](https://github.com/rshade/finfocus/issues/577))
- [x] **v0.3.0: Intelligence & Analysis** *(Completed)*
  - [x] Add `cost estimate` command for scenario modeling
        ([#463](https://github.com/rshade/finfocus/issues/463), PR #538)
  - [x] Docs formatting & validation.go fix (PR #507 follow-up)
        ([#533](https://github.com/rshade/finfocus/issues/533))
  - [x] Recommendation dismissal and lifecycle management
        ([#464](https://github.com/rshade/finfocus/issues/464), PR #557)
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
- [x] **v0.2.5: Testing & Stability** *(Released 2026-01-30)*
  - [x] Multi-region E2E testing support (#185, PR #485)
  - [x] Pagination and NDJSON streaming for CI/CD integration (#225, PR #488)
  - [x] Exit codes for budget threshold violations
        ([#219](https://github.com/rshade/finfocus/issues/219))
  - [x] Budget health calculation & threshold alerting (#267, PR #494)
  - [x] Provider filtering & summary aggregation for Budgets (#263, PR #494)
- [x] **v0.2.4: Final Polish** *(Released 2026-01-21)*
  - [x] Dynamic Data Recording via Integration Plans (#275)
  - [x] Harden Nightly Analysis Workflow security and reliability (#325)
  - [x] Documentation for TUI features, budgets, and recommendations (#226)
  - [x] Budget status display in CLI (#217, PR #466)
- [x] **v0.2.1-v0.2.3: Polish & DX Improvements** *(Released 2026-01-17
      through 2026-01-19)*
  - [x] Add `--estimate-confidence` flag for actual cost transparency (#333)
  - [x] Implement GetPluginInfo consumer-side requirements (#376)
  - [x] Parallel plugin metadata fetching in plugin list command (#408)
  - [x] Cross-Repository Integration Test Workflow (#236)
  - [x] Upgrade cost commands to enhanced TUI (#218)
  - [x] Plugin robustness: strict mode, config handlers, metadata
        fallback (#435, #434, #432, #431)
  - [x] Fallback to latest stable version when asset missing (#430)
  - [x] Budget filtering and summary aggregation logic (#446)
  - [x] Replace manual assertions with testify (#429)
  - [x] Plugin installer: remove old versions during install (#237)
  - [x] E2E test for actual cost command (#334)
  - [x] Set up AWS test account and infrastructure (#181)
  - [x] Fuzzing seeds, benchmarks, and validation improvements (#326)
  - [x] Documentation updates (#182, #349-#353, #454)
- [x] **v0.2.0: State-Based Costs & Plugin Maturity** *(Released 2026-01-16)*
  - [x] State-based actual cost estimation for `cost actual` (#380, #382)
  - [x] Plugin info and dry-run discovery (#398)
  - [x] Ecosystem rebrand to FinFocus (#415)
- [x] **v0.1.3-v0.1.4: Analyzer & Recommendations** *(Released 2025-12-27
      through 2026-01-10)*
  - [x] Core Analyzer implementation (#245, #229)
  - [x] E2E testing with Pulumi Automation API (#177, #238)
  - [x] Comprehensive E2E tests for Analyzer integration (#228)
  - [x] Add recommendations to analyzer diagnostics (#321)
  - [x] Shared TUI package with Bubble Tea (#222, #258)
  - [x] E2E and Conformance test reliability fixes (#323)
  - [x] AWS fallback scope and non-deterministic output fixes (#324)

### 2025-Q4

- [x] **v0.1.0-v0.1.2: Foundation & Observability** *(Released 2025-11-26
      through 2025-12-03)*
  - [x] Initial CLI & gRPC Plugin System (#163, #15)
  - [x] Standardized SDK & Interceptors (#188, #189, #191)
  - [x] Zerolog Integration & Structured Logging (#170, #206)
  - [x] Engine Test Coverage Completion (#202, #207)
  - [x] Plugin Ecosystem Maturity (#201, #215)
  - [x] Support for `Supports()` gRPC handler (#160, #165)
  - [x] CLI Filter Flag (#203)
  - [x] Test Infrastructure Hardening (#200)

## Cross-Repository Feature Matrix

| Feature | spec | core | aws-public | aws-ce |
| ------- | ---- | ---- | ---------- | ------ |
| Cost Time Machine | GrowthType | history collect/view | GrowthHint | Historical |
| YOLO Mode | BypassReason (missing) | --yolo flag | N/A | N/A |
| Blast Radius | Lineage | Impact Tree | Parent/child | N/A |
| GreenOps Receipt | CarbonFootprint | Converter | CCF Math | N/A |
| Spot Market Advisor | PricingTier | Cyan style | N/A | SpotHistory |
| Dev Mode | UsageProfile (v0.5.5) | --profile | Burstable | IOPS warn |
| What-If Analysis | EstimateCost | cost estimate | PropertyDelta | N/A |
| Rec Lifecycle | DismissRecommendation | dismiss/snooze | Dismiss | N/A |
| Auto-Detect | N/A | pulumi detect | N/A | N/A |
| Resource Filter | N/A | provider/component filter | N/A | N/A |

## Boundary Safeguards

*Sourced from [CONTEXT.md](CONTEXT.md) — these are architectural hard no's.*

- **No Direct Cloud API Calls**: The core engine MUST NOT call cloud provider
  pricing or usage APIs directly. All provider-specific logic belongs in
  plugins.
- **Minimal Persistent State**: The tool is primarily stateless. Local
  persistence (config, dismissed.json, history DBs) is user-initiated and
  optional — never required for core command execution.
- **Read-Only Infrastructure**: FinFocus MUST NOT perform `pulumi up`,
  `pulumi destroy`, or any operation that modifies cloud state. It reads
  infrastructure definitions only.
- **No Baked-in Provider Logic**: The core engine MUST NOT contain hardcoded
  logic for specific cloud services. This logic is strictly delegated to
  plugins or YAML specs.
- **No Financial Accounting**: The tool handles cost *estimation* and
  *projection*. It is NOT a ledger, invoice matching system, or tax
  calculation engine.
