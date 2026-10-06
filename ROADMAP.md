# FinFocus Strategic Roadmap

This roadmap maps 1:1 with tracked work in GitHub Issues. It outlines the
evolution of `finfocus` while strictly adhering to the technical
guardrails in `CONTEXT.md`.

## Table of Contents

- [Immediate Focus (Post v0.4.0)](#immediate-focus-post-v040)
- [Near-Term Vision (v0.3.x)](#near-term-vision-v03x---forecasting--profiles)
- [Future Vision (Later)](#future-vision-later---notifications-integrations--backlog)
- [Completed Milestones](#completed-milestones)
- [Cross-Repository Feature Matrix](#cross-repository-feature-matrix)
- [Boundary Safeguards](#boundary-safeguards)

## Immediate Focus (Post v0.4.0)

*v0.4.0 released 2026-10-03. Everything below targets the next release.*

The 2026-10-02 Overnight Queue finished: all 22 items closed 2026-10-03 (see
Completed Milestones, 2026-Q4).

- [ ] **Bug Queue**
  - [x] `speckit`: `create-new-feature.sh` breaks when `git fetch --all`
        prints to stdout (two or more remotes)
        ([#1669](https://github.com/rshade/finfocus/issues/1669)) [S]

## Near-Term Vision (v0.3.x - Forecasting & Profiles)

- [ ] **Overview Performance Pipeline** *(deferred from v0.3.2)*
  - [ ] Parallelize plugin opening in `Registry.Open()`
        ([#693](https://github.com/rshade/finfocus/issues/693)) [M]
  - [ ] Start plugin loading concurrently with data loading
        ([#692](https://github.com/rshade/finfocus/issues/692)) [M]
  - [ ] Parallelize stack export and pulumi preview
        ([#691](https://github.com/rshade/finfocus/issues/691)) [M]
- [ ] **Overview TUI Quality** *(deferred from v0.3.2)*
  - [ ] Show phase progress lines sequentially and add preview phase
        ([#714](https://github.com/rshade/finfocus/issues/714)) [M]
  - [ ] Refactor overview to compute-once-render-many architecture
        ([#853](https://github.com/rshade/finfocus/issues/853)) [L]
- [ ] **Contextual Profiles ("Dev Mode")**
      ([#368](https://github.com/rshade/finfocus/issues/368)) [L]
  - [ ] CLI: Implement `--profile` flag (e.g., `dev`, `prod`) to pass hints
        to plugins
  - [ ] Configuration: Allow default profile definition in `finfocus.yaml`
  - *Spec ready:* `UsageProfile` enum (PROD/DEV/BURST) available in
    finfocus-spec v0.5.5 — core-only implementation
- [ ] **Projected Cost Diagnostics**
  - [ ] Report the missing region instead of "no cost data available"
        ([#1670](https://github.com/rshade/finfocus/issues/1670)) [M]
- [ ] **Scale Testing**
  - [ ] Pulumi TypeScript scalability fixture for E2E performance testing
        ([#658](https://github.com/rshade/finfocus/issues/658)) [M]
- [ ] **Spec v0.5.7 Features**
  - *All v0.5.7 consumer items promoted to Immediate Focus*
- [ ] **Time-Series Forecasting Enhancement**
  - [ ] Enhance `cost estimate` with ARIMA + driver-based forecasting
        ([#539](https://github.com/rshade/finfocus/issues/539)) [L]
- [x] **Forecasting & Projections ("Cost Time Machine")**
      ([#364](https://github.com/rshade/finfocus/issues/364)) [L]
  - [x] Projection Math Engine (Linear/Exponential extrapolation)
  - [x] TUI: ASCII Line Chart visualization for 6-12 month forecasts
  - *Status: `cost forecast` projects with the finfocus-spec growth helpers.
    JSON series are timestamped for the interactive history chart (#550).*

- [ ] **Kubernetes Cost Allocation — Pulumi Integration**
  - [ ] `finfocus overview` expansion of Kubernetes clusters in the stack
        ([#1526](https://github.com/rshade/finfocus/issues/1526)) [L]
- [ ] **Kubernetes Cost Allocation — Usage & Pricing**
  - [ ] Prometheus usage source plugin — historical actuals, kind CI
        ([#1529](https://github.com/rshade/finfocus/issues/1529)) [L]
  - *Cross-Repo:* [rshade/finfocus-plugin-aws-public](https://github.com/rshade/finfocus-plugin-aws-public)
    spot pricing for EC2 nodes ([#406](https://github.com/rshade/finfocus-plugin-aws-public/issues/406)) and the finfocus-spec v0.6.2 bump
    ([#407](https://github.com/rshade/finfocus-plugin-aws-public/issues/407)), which must wait for the finfocus v0.3.8 release
- [x] **Azure Resource Resolution — Sparse-SKU Pre-Flight** *(decided in
      #1610)*
  - *Decision:* a projected resource with `ref.*` tags and an empty SKU uses
    `pluginsdk.ValidateProjectedCostRequestLenient` instead of failing closed
    on SKU, so plugins see the dotted tags from #1608.
  - *Transferred:* the per-type Azure SKU table (was #1609) moved to
    [finfocus-plugin-azure-public issue 69](https://github.com/rshade/finfocus-plugin-azure-public/issues/69).
    It conflicted with the "Baked-in Provider Logic" boundary in
    `CONTEXT.md` and duplicated vocabulary the plugin already owns.

## Future Vision (Later - Notifications, Integrations & Backlog)

- [ ] **Governance Overrides ("YOLO Mode")**
      ([#365](https://github.com/rshade/finfocus/issues/365)) [M]
  - [ ] CLI: Implement `--yolo` / `--force` flag to bypass budget gates
  - [ ] UX: "Warning Mode" UI styles for bypassed runs
  - *Blocked:* Requires `BypassReason` enum in
    [finfocus-spec](https://github.com/rshade/finfocus-spec) (not yet defined)
- [ ] **External Notifications**
  - [ ] Webhook and email notifications for budget alerts
        ([#220](https://github.com/rshade/finfocus/issues/220)) [L]
  - *Note:* Requires external service integration to maintain core
    statelessness per CONTEXT.md boundaries
- [ ] **Recommendation Lifecycle Enhancements** *(spec-first)*
  - [x] Add `include_dismissed` field to GetRecommendationsRequest
        ([#545](https://github.com/rshade/finfocus/issues/545)) [S]
  - [ ] Add GetRecommendationHistory RPC to CostSourceService
        ([#546](https://github.com/rshade/finfocus/issues/546)) [M]
- [ ] **Cost Time Machine** *(Phases 1 and 3 and the follow-ups shipped
      2026-10-03; Phase 2 stays here because it needs visual TUI
      verification)*
  - [ ] Phase 2 — Interactive TUI: ntcharts TimeSeriesLineChart with
        pan/zoom, provider split, budget overlay
        ([#550](https://github.com/rshade/finfocus/issues/550)) [L]
- [ ] TUI Lazy Loading & Error Recovery (#483) [L] *Deferred from TUI Phase 7*
- [ ] Plugin integrity verification strategy (#164) [M]
- [ ] Use registry-based plugin install for cross-repo integration tests
      ([#517](https://github.com/rshade/finfocus/issues/517)) [M]
- [ ] **Dependency Visualization ("Blast Radius")**
      ([#366](https://github.com/rshade/finfocus/issues/366)) [L]
  - [ ] TUI: Interactive Dependency Tree view (consuming Lineage Metadata)
  - *Cross-Repo:* Consumes `CostAllocationLineage`/`ParentResourceID` from
    [finfocus-spec](https://github.com/rshade/finfocus-spec)
- [ ] **Spot Market Advisor**
      ([#367](https://github.com/rshade/finfocus/issues/367)) [L]
  - [ ] TUI: Highlight Spot savings in Cyan; show Risk Icon
  - [ ] Display "Savings vs On-Demand" percentage
  - *Cross-Repo:* Requires `PricingTier`/`SpotRisk` enums in
    [finfocus-spec](https://github.com/rshade/finfocus-spec); CE plugin
    implements `DescribeSpotPriceHistory`
- [ ] **Carbon Footprint Estimation**
  - [ ] Carbon footprint estimation plugin using Cloud Carbon Footprint
        methodology
        ([#688](https://github.com/rshade/finfocus/issues/688)) [L]
- [ ] **Mixed-Currency Aggregation Strategy (MCP Alignment)**
  - *Objective*: Implement core-level grouping for multi-currency stacks so
    MCP clients of the built-in server (`finfocus --mcp` /
    `finfocus mcp-server`) receive structured per-currency results.
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
- [ ] **Pulumi Cloud API Integration** *(depends on #934)*
  - [ ] Pulumi Cloud API Integration for Historical State Snapshots
        ([#935](https://github.com/rshade/finfocus/issues/935)) [L]
- [ ] **Agent Skills** *(co-located, tool-dependent)*
  - [ ] `plugin-manage` — Discover, install, update, validate, and
        troubleshoot finfocus plugins via gRPC protocol
        ([#911](https://github.com/rshade/finfocus/issues/911)) [M]
  - *Format:* Agent Skills spec (SKILL.md + references/), installable via
    `npx skills add rshade/finfocus -s <name>`
  - *Generic cost workflow skills (cost-check, cost-drift, cost-optimize,
    budget-setup) live in
    [rshade/agent-skills](https://github.com/rshade/agent-skills)
    as multi-tool skills — see
    [Split-Tier Placement consensus](https://github.com/rshade/agent-skills/blob/main/PUNCHLIST.md)*
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
- [ ] **Kubernetes Cost Allocation — Additional Usage Sources**
  - [ ] Datadog usage source plugin
        ([#1530](https://github.com/rshade/finfocus/issues/1530)) [L]
  - *Transferred:* the OpenCost plugin returning pre-allocated rows (was
    #1531) now lives in
    [finfocus-plugin-kubecost issue 48](https://github.com/rshade/finfocus-plugin-kubecost/issues/48)
- [ ] **Plugin Host Pooling** *(cross-repo)*
  - [ ] Pool opted-in plugins and pass per-request credentials
        ([#1539](https://github.com/rshade/finfocus/issues/1539)) [L]
- [ ] **Cost Anomaly Detection**
  - [ ] Flag-only `cost anomalies` command over actual-cost history, exposed
        as an MCP tool
        ([#1590](https://github.com/rshade/finfocus/issues/1590)) [L]
- [ ] **Recommendation Scoring — Jev Plugin Ideas** *(brainstorm 2026-10-01;
      ideas only, no issues or specs yet)*
  - *Placement rule:* Jev stays in `plugins/jev`. Core and
    [finfocus-spec](https://github.com/rshade/finfocus-spec) get no
    Jev-specific code, names or logic. When an idea needs core or spec work,
    that work is a generic scorer capability any scorer plugin could use.
  - *Cross-cutting question:* each non-recommendation surface (ideas 6 to 9)
    needs either its own spec RPC or one generic "ask yes/no or rating
    questions about these records" RPC. The generic RPC is more powerful but a
    far more open-ended contract. The closed `ScoreSignal` enum and single RPC
    in the spec are the real ceiling on what a scorer can do.
  - [ ] **1. Scores that learn from dismissals** [M] — Send past dismissals
        and their reasons as context, so `false_positive` reflects the team's
        own history (few-shot context, no training).
    - *Placement:* spec gets a generic prior-decisions field on
      `ScoreRecommendationsRequest`; core fills it from the dismissal store
      with the same identifier handling and scrubbing as recommendations; the
      plugin decides how Jev uses it.
    - *Open question:* does sending past dismissal reasons to a hosted model
      feel worse, privacy-wise, than sending the recommendations themselves?
  - [ ] **2. Explain scores by breaking the question into parts** [S-M] — Back
        each headline score with three or four narrow yes/no sub-questions
        (prod-tagged, stateful, utilization present, hard to reverse) and show
        the ones that drove it.
    - *Placement:* sub-questions live in the plugin; the spec may get a
      generic optional "contributing factors" field that core renders.
    - *Open question:* do factors travel over the spec as a new field, or
      does the plugin fold them into an existing free-text field?
  - [ ] **3. Triage inbox in the TUI** [M] — A `cost recommendations
        --triage` mode: one recommendation at a time, sorted by `worth_acting`,
        `needs_review` pinned, single keys to dismiss (with reason, feeding
        idea 1), snooze or open.
    - *Placement:* core TUI over generic scores; works with any scorer.
    - *Open question:* is the user a person clearing a weekly backlog or an
      agent over MCP? The UX differs.
  - [ ] **4. Cost preview before enabling** [S] — Extend `--scoring-dry-run`
        from what would be sent to what it would cost ("47 recommendations,
        about 21k tokens, about $0.001").
    - *Placement:* the plugin knows its price; spec gets a generic optional
      cost estimate the scorer can report; core only displays it.
    - *Open question:* does this need a spec call, or is a plugin-side log
      line enough?
  - [ ] **5. Custom signals from config** [L] — Let operators define their own
        yes/no questions (`"pci": "Does this touch a resource handling card
        data?"`) that become sortable and filterable columns.
    - *Placement:* spec gets a generic custom-signal mechanism alongside the
      closed enum; core passes config through to any scorer; the plugin turns
      them into Jev questions.
    - *Open question:* operator-written question text breaks the plugin rule
      that question text never contains supplied content. Is operator text
      trusted enough to cross that line?
  - [ ] **6. Rank plan changes for PR review** [M] — In `cost projected` and
        the Pulumi Analyzer, rank plan-diff rows by "would a reviewer want to
        look at this?" and surface the top few as advisory warnings, never
        blocking.
    - *Placement:* generic spec RPC for scoring plan changes (or the generic
      record RPC); core calls whichever scorer is configured; Jev questions
      live in the plugin. Analyzer output goes through diagnostics, never
      stdout.
    - *Open question:* does the analyzer's per-preview time budget allow a
      network round trip?
  - [ ] **7. Drift triage: expected or anomaly?** [M] — Ask "probably on
        purpose?" about overview `CostDrift` rows, so a batch job's month-end
        spike ranks below a runaway.
    - *Placement:* same as idea 6, over drift rows.
    - *Open question:* a drift row carries far less context than a
      recommendation. Is there enough for a scorer to judge?
    - *Related:* the statistical `cost anomalies` command (#1590) would
      produce rows a scorer could rank the same way.
  - [ ] **8. Kubernetes idle capacity: headroom or waste?** [M-L] — Ask whether
        idle capacity from the `kubernetes` plugin is intentional headroom (HPA
        burst, PDBs, reserved pools). Combines both differentiators.
    - *Placement:* core composes allocator output into a generic scorer call;
      no plugin-to-plugin calls.
    - *Open question:* core composes plugins only for recommendations today.
      Is this worth a general composition mechanism?
  - [ ] **9. Which resources drove the budget breach?** [M] — When a budget is
        EXCEEDED, rank contributing resources by likely cause so the banner
        reads "EXCEEDED, mostly these 3".
    - *Placement:* same as idea 6, over budget contributors.
    - *Open question:* sorting by cost delta may already do most of this. Does
      a model beat arithmetic here?
  - [ ] **10. Rules baseline scorer and benchmark** [S-M] — A free, offline,
        rules-based scorer plugin implementing the same service, plus a
        shared evaluation dataset, so scoring works without sending data
        anywhere and Jev's quality is measured against a baseline rather than
        asserted.
    - *Placement:* a separate plugin, not core. The labelled dataset moves out
      of `plugins/jev/testdata` to somewhere both plugins can use.
    - *Open question:* what if the rules baseline gets close to Jev? Is that
      worth finding out?
  - [ ] **11. Privacy as the demo** [S] — Lead demos with `--scoring-dry-run`:
        exactly what leaves the machine, pseudonymized, before anything is
        sent. Auditable AI as positioning.
    - *Placement:* docs and positioning only; the dry run is already generic.
      Depends on the TypeSafe data-terms research (PM backlog item 16).
    - *Open question:* do buyers respond to auditable AI, or only engineers?

## Completed Milestones

### 2026-Q4

- [x] #1525 `kubernetes`: projected cost for workloads declared in Pulumi.
      Closed 2026-10-04. [L]
- [x] #1610 `engine`: resolve cross-resource refs from `propertyDependencies`.
      Closed 2026-10-03. [M]
- [x] #1608 `engine`: emit nested resource inputs as dotted tag keys.
      Closed 2026-10-03. [M]
- [x] #1588 `kubernetes`: key allocator nodes by cluster and node name.
      Closed 2026-10-03. [S]
- [x] #808 `cli`: thread budget flag overrides without global mutation.
      Closed 2026-10-03. [M]
- [x] #579 `overview`: add `--output json` support.
      Closed 2026-10-03. [M]
- [x] #638 `engine`: price from plugin GetPricingSpec before local YAML.
      Closed 2026-10-03. [L]
- [x] #549 `history`: add `cost history collect` and `view --plain`.
      Closed 2026-10-03. [L]
- [x] #223 `config`: validate configuration with helpful error messages.
      Closed 2026-10-03. [M]
- [x] #643 `overview`: add the Warn column and `OverviewWarning` type.
      Closed 2026-10-03. [M]
- [x] #637 `tui`: enrich cost estimate with GetPricingSpec discovery.
      Closed 2026-10-03. [M]
- [x] #551 `history`: add history export and inline sparklines.
      Closed 2026-10-03. [M]
- [x] #224 `cli`: add `--no-color`, `--plain` and high-contrast options.
      Closed 2026-10-03. [M]
- [x] #495 `cache`: add an optional LRU tier over BoltDB.
      Closed 2026-10-03. [M]
- [x] #636 `cli`: add `--explain` to `cost projected`.
      Closed 2026-10-03. [M]
- [x] #555 `history`: add history prune and retention policy.
      Closed 2026-10-03. [M]
- [x] #576 `engine`: add the cost diff view to `cost projected`.
      Closed 2026-10-03. [M]
- [x] #645 `tests`: raise overview enrichment and CLI coverage to 80%+.
      Closed 2026-10-03. [L]
- [x] #685 `cli`: add `--show-breakdown` and `--show-confidence` flags.
      Closed 2026-10-03. [M]
- [x] #556 `history`: handle mixed-currency history snapshots.
      Closed 2026-10-03. [M]
- [x] #554 `history`: add `cost history diff` change attribution.
      Closed 2026-10-03. [L]
- [x] #646 `docs`: document the overview command with examples.
      Closed 2026-10-03. [M]
- [x] #553 `docs`: add CI/CD recipes for cost history collection.
      Closed 2026-10-03. [S]
- [x] #248 `plugin-init`: scaffold plugins on core's finfocus-spec version.
      Closed 2026-10-03. [M]
- [x] #270 `plugin`: add `plugin upgrade` for SDK migrations.
      Closed 2026-10-03. [L]
- [x] #913 `skills`: add the finfocus-diagnose agent skill.
      Closed 2026-10-03. [M]
- [x] #1208 `lint`: apply Go modernization lint rules incrementally.
      Closed 2026-10-03. [L]
- [x] #1527 `kubernetes`: link allocated workloads back to Pulumi URNs.
      Closed 2026-10-03. [M]
- [x] #1532 `kubernetes`: price pods running on EKS Fargate.
      Closed 2026-10-03. [M]
- [x] #1528 `cli`: implement `finfocus cost cluster`.
      Closed 2026-10-02. [L]
- [x] #1533 `kubernetes`: share idle and shared-workload cost.
      Closed 2026-10-02. [M]
- [x] #1589 `engine`: truncate decline reasons on a rune boundary.
      Closed 2026-10-02. [S]
- [x] #493 `plugin-init`: generate golangci-lint configuration.
      Closed 2026-10-02. [S]
- [x] #641 `overview`: add `--force-color` and `--no-color` flags.
      Closed 2026-10-02. [S]
- [x] #642 `overview`: add interactive pre-flight confirmation prompt.
      Closed 2026-10-02. [S]
- [x] #914 `skills`: add the finfocus-budget agent skill.
      Closed 2026-10-02. [M]
- [x] #1197 `tests`: migrate internal package tests to external test packages.
      Closed 2026-10-01. [L]
- [x] #1198 `tests`: adopt paralleltest safely across isolated tests.
      Closed 2026-10-01. [M]
- [x] #1506 `terraform-state`: fix v0.3.7 smoke-test findings.
      Closed 2026-10-01. [M]

### 2026-Q3

- [x] #1515 `engine`: surface plugin Supports decline reasons.
      Closed 2026-09-30. [M]
- [x] #1523 `docs`: convert Kubernetes cost-allocation docs to Spec Kit.
      Closed 2026-09-30. [M]
- [x] #1534 `registry`: add kubernetes plugin registry entry.
      Closed 2026-09-30. [S]
- [x] #1512 `engine`: fail open on SDK default Supports response.
      Closed 2026-09-29. [S]
- [x] #1517 `kubernetes`: map API errors to precise gRPC codes.
      Closed 2026-09-29. [S]
- [x] #1518 `kubernetes`: count pod-level spec.resources requests.
      Closed 2026-09-29. [S]
- [x] #1519 `kubernetes`: emit valid GCP and Azure node type tokens.
      Closed 2026-09-29. [S]
- [x] #1199 `tests`: refactor high-complexity tests flagged by gocognit.
      Closed 2026-09-29. [M]
- [x] #1200 `examples`: make mock plugin examples testable.
      Closed 2026-09-29. [M]
- [x] #1203 `cli`: normalize CLI, config and logging constants.
      Closed 2026-09-29. [M]
- [x] #1204 `engine`: normalize engine, router, proto and analyzer constants.
      Closed 2026-09-29. [M]
- [x] #1206 `tests`: adopt stricter testifylint assertions.
      Closed 2026-09-29. [M]
- [x] #1207 `tests`: adopt usetesting helpers for env, tempdir, cwd.
      Closed 2026-09-29. [M]
- [x] #1209 `lint`: audit govet shadow analyzer findings.
      Closed 2026-09-29. [M]
- [x] #1212 `tests`: audit govet unusedwrite findings.
      Closed 2026-09-29. [M]
- [x] #1213 `tests`: migrate mock plugin gRPC helpers off DialContext.
      Closed 2026-09-29. [M]
- [x] #1205 `tests`: clean fixture and benchmark goconst/mnd constants.
      Closed 2026-09-29. [S]
- [x] #1210 `tests`: clean revive unused-parameter findings.
      Closed 2026-09-29. [S]
- [x] #1211 `docs`: add stdlib doc links for godoclint.
      Closed 2026-09-29. [S]
- [x] #1520 `kubernetes`: match China-partition EKS API server hosts.
      Closed 2026-09-28. [S]
- [x] #1516 `kubernetes`: validate label selector keys and values.
      Closed 2026-09-28. [S]
- [x] #1231 `pluginhost`: stop plugins holding inherited stdout/stderr pipes.
      Closed 2026-09-22. [M]
- [x] #979 `engine`: add per-chunk timeout for BatchCost RPC calls.
      Closed 2026-09-22. [S]
- [x] #978 `engine`: executeBatchForPlugin skipped the re-chunked tail.
      Closed 2026-09-22. [M]
- [x] #977 `docs`: document ActualCostData and ResourceError messages.
      Closed 2026-09-22. [S]
- [x] #976 `engine`: budget_tag_filter_test mock BatchCost returns error.
      Closed 2026-09-22. [S]
- [x] #975 `engine`: budget_health_test mock BatchCost returns error.
      Closed 2026-09-22. [S]
- [x] #974 `engine`: batch actual cost mapper kept rate fields.
      Closed 2026-09-22. [M]
- [x] #973 `engine`: test BuildEstimateCostRequest structpb error path.
      Closed 2026-09-22. [S]
- [x] #972 `engine`: add Ctx and component fields to tryEstimateCostRPC logs.
      Closed 2026-09-22. [S]
- [x] #970 `engine`: test modified-response validation fallback path.
      Closed 2026-09-22. [S]
- [x] #968 `history`: extract correct URN hash in filterFullyExpiredURNs.
      Closed 2026-09-22. [S]
- [x] #967 `history`: escape delimiters in BuildTagKey.
      Closed 2026-09-22. [S]
- [x] #965 `history`: newTestEntryWithTime keeps FirstSeen <= LastSeen.
      Closed 2026-09-22. [S]
- [x] #964 `history`: merge tag timestamps in upsertTags.
      Closed 2026-09-22. [S]
- [x] #963 `history`: set enabled=false on BoltStore.Close.
      Closed 2026-09-22. [S]
- [x] #962 `history`: track newest entry per URN hash in GetDeletedResources.
      Closed 2026-09-22. [S]
- [x] #961 `pulumi`: only ignore missing-file errors in GetProjectName.
      Closed 2026-09-22. [S]
- [x] #960 `config`: use pointer type for HistoryConfig.RetentionDays.
      Closed 2026-09-22. [S]
- [x] #959 `cli`: reuse loaded config in overview initHistoryFromConfig.
      Closed 2026-09-22. [S]
- [x] #958 `cli`: return success indicator from detectHistoryStackContext.
      Closed 2026-09-22. [S]
- [x] #957 `cli`: copy tags in convertEngineStateToHistoryState.
      Closed 2026-09-22. [S]
- [x] #956 `cli`: populate tags in convertDescriptorsToHistoryState.
      Closed 2026-09-22. [S]
- [x] #573 `registry`: reimplement plugin installer lock for Windows.
      Closed 2026-09-22. [M]
- [x] #462 `plugin-init`: generate standardized GitHub workflow files.
      Closed 2026-09-22. [S]
- [x] #461 `plugin-init`: add CLI flags for generation control.
      Closed 2026-09-22. [S]
- [x] #460 `plugin-init`: enhanced Makefile template targets.
      Closed 2026-09-22. [S]
- [x] #459 `plugin-init`: add health endpoint to generated main.go.
      Closed 2026-09-22. [S]
- [x] #458 `plugin-init`: add GetPluginInfo and Supports to calculator template.
      Closed 2026-09-22. [S]
- [x] #457 `plugin-init`: generate documentation templates.
      Closed 2026-09-22. [S]
- [x] #456 `plugin-init`: generate Docker support files.
      Closed 2026-09-22. [S]

### 2026-Q2

- [x] #846 `engine`: implement the BatchCost RPC consumer.
      Closed 2026-04-04. [L]
- [x] #847 `engine`: implement the EstimateCost RPC consumer.
      Closed 2026-04-04. [M]
- [x] #934 `history`: add the resource history store.
      Closed 2026-04-04. [L]
- [x] #955 `analyzer`: capture resource properties and tags into history.
      Closed 2026-04-05. [S]
- [x] #980 `engine`: validate resources before BatchCost batching.
      Closed 2026-04-06. [M]
- [x] #971 `engine`: fix type loss in `mergePropertiesWithOverrides`.
      Closed 2026-04-06. [M]
- [x] #966 `engine`: fix double-prefixing in `StackContext.Hash`.
      Closed 2026-04-06. [S]

### 2026-Q1

- [x] #690 add --state-only flag to skip pulumi preview
- [x] #855 clarify budget status visibility in overview output modes
- [x] #909 create finfocus-install agent skill for automated CLI and…
- [x] #910 create finfocus-analyzer-setup agent skill for Pulumi Analyzer integration
- [x] #912 create finfocus-routing agent skill for intelligent plugin routing…
- [x] #895 update aws-public plugin to install router by default
- [x] #845 consume expires_at caching hints from plugin cost responses
- [x] #848 recognize PLUGIN_CAPABILITY_BATCH_COST in capability routing and plugin list
- [x] #844 upgrade finfocus-spec from v0.5.6 to v0.5.7
- [x] #687 Add config routes list and config routes test…
- [x] #552 upgrade to Bubble Tea v2, Lip Gloss v2,…
- [x] #827 upgrade charmbracelet dependencies to v2 (bubbles, bubbletea, lipgloss)
- [x] #760 false-positive drift for resources created mid-month
- [x] #734 implement GetPricingSpec and EstimateCost methods
- [x] #644 add short flags (-s, -f, -a) to overview…
- [x] #717 state guards missing for init-only TUI messages in…
- [x] #720 audit enriched count inaccurate on early TUI exit
- [x] #721 extract progress constant and add goroutine comment in…
- [x] #718 make table separator line extend to terminal width…
- [x] #726 classifyError should handle context.Canceled and context.DeadlineExceeded
- [x] #674 Transition persistent cache from JSON to BoltDB (bbolt)
- [x] #682 BoltStore.Set returns nil when disabled, inconsistent with other…
- [x] #822 eliminate duplicate ResolvePolicyPackDir call in RunChecks
- [x] #746 Bug: AnalyzeStack stack summary always shows $0.00 (0…
- [x] #754 Bug: --force reinstall does not sync policy pack…
- [x] #755 Enhancement: analyzer install should setup policy pack directory…
- [x] #756 Enhancement: analyzer install should print PATH setup instructions…
- [x] #757 Enhancement: Add finfocus analyzer check command for setup…
- [x] #681 compact() leaves store unusable if reopen fails after…
- [x] #809 CLI tests leak real ~/.finfocus config causing JSON…
- [x] #735 add TUI interactive mode integration tests
- [x] #736 add cache system integration tests
- [x] #738 add concurrency and performance regression tests
- [x] #739 add project-local config and config precedence tests
- [x] #740 add analyzer concurrency and partial failure tests
- [x] #741 resolve nightly build tag fragmentation
- [x] #742 add plugin resilience and crash recovery tests
- [x] #716 race between enrichment goroutine and plugin cleanup in…
- [x] #744 display budget status and health in overview command
- [x] #745 add cost caching to speed up enrichment
- [x] #762 detectErr unconditionally overrides --yes flag for isStateOnly in…
- [x] #722 verify defensive copy independence in DataReadyMsg handler
- [x] #783 FINFOCUS_HIDE_ALIAS_HINT should use presence-based check, not value-based
- [x] #787 recognize .tsx, .jsx, and go.work as Pulumi source…
- [x] #698 SBOM action fails to attach to releases —…
- [x] #683 test data quality issues across cache test files
- [x] #684 clean up duplicate doc comments and extract placeholder…
- [x] #758 Docs: Fix analyzer-setup.md — PATH requirement and Pulumi.yaml…
- [x] #737 fix always-skipped integration tests
- [x] #743 cli_helper global log suppression masks plugin errors
- [x] #788 make TestGetProjectedCost_PartialData order-independent
- [x] #786 fix vacuous exit code 0 test in budget_scoped_test.go
- [x] #785 close plugin clients in TestNewClient_Success and TestClient_APIUsage
- [x] #784 stubHome should clear FINFOCUS_HOME for hermetic config tests
- [x] #782 deduplicate env setup and fix fragile assertion in…
- [x] #776 consolidate 5 TestGetPluginInfo_* tests into table-driven
- [x] #775 remove duplicate flat tests in pulumi_plan_test.go, merge into…
- [x] #774 consolidate 4 near-identical cost projected tests into table-driven
- [x] #723 investigate intermittent $0.00 projected costs in TUI overview
- [x] #747 Bug: Recorder plugin returns nil summary on GetRecommendations,…
- [x] #748 Bug: Analyzer JSON logs appear in pulumi preview…
- [x] #749 Bug: analyzer install creates double-v version directory (analyzer-finfocus-vv...)
- [x] #750 Bug: Registry ListPlugins silently skips directory-level symlinks
- [x] #751 Bug: AnalyzerPlugin.Enabled config field is dead code —…
- [x] #752 Bug: FINFOCUS_PLUGIN_DIR env var documented but not implemented
- [x] #753 Bug: plugins.dir config key documented but excluded from…
- [x] #791 consolidate duplicate flat LoadPulumiPlan tests into table-driven suites
- [x] #790 add require.NotNil guard in TestLoadPulumiPlan_ComplexInputs
- [x] #789 remove duplicate TestApplyChangesToRows_NilMap in overview_merge_test.go
- [x] #728 splash screen — figlet banner, phase checklist, passphrase…
- [x] #694 parallelize per-row enrichment sub-calls
- [x] #719 use lipgloss styles in renderInitializingView for consistency
- [x] #761 applyPassphraseEnv uses process-wide os.Setenv (not concurrency-safe)
- [x] #763 replace hardcoded "730h/mo" footnote with engine.HoursPerMonth constant
- [x] #764 TestDetectChanges_StatErrorSkipsFile fails on Windows (no symlink privilege guard)
- [x] #765 missing .Ctx(ctx) on log calls in changedetect.go loses…
- [x] #766 "Recs" table column width too narrow for N(-M)…
- [x] #759 Docs: Document that routing config does not apply…
- [x] #695 add timing instrumentation to overview command
- [x] #689 launch TUI immediately with phase progress feedback
- [x] #680 resolveCacheDir global fallback places cache.db in wrong directory
- [x] #686 add provider/resource_type assertions to tag enrichment tests
- [x] #702 update all version references to v0.3.0
- [x] #710 expand testing guide and fix docker.md phantom Dockerfile
- [x] #599 Install script (curl | sh)
- [x] #601 Checksum verification for plugin installation
- [x] #602 --jobs flag and timing output for cost commands
- [x] #600 Projected cost caching
- [x] #657 add benchmark PR reporting with benchstat regression detection
- [x] #541 extract Cache interface and refactor FileStore
- [x] #542 add caching to GetActualCost with 1-hour TTL
- [x] #543 add caching to GetProjectedCost with SHA-based keys
- [x] #604 Policy-compatible cost output
- [x] #610 consolidate recommendation count and format helpers (DRY)
- [x] #605 isolate auto-detection tests with temp directories
- [x] #590 wire router into cost commands for region-aware plugin…
- [x] #582 Filter pulumi:providers:* synthetic resources from cost plugin routing
- [x] #583 Filter Pulumi component resources from cost plugin routing
- [x] #616 reorder router provider-based region check after feature matching
- [x] #548 split project-local and user-global .finfocus directories
- [x] #611 Neo-friendly CLI fixes
- [x] #612 add Stack field to CostFlags struct and remove…
- [x] #613 add .Ctx(ctx) and structured log fields across multiple…
- [x] #607 Scale benchmarks for cost commands
- [x] #608 add negative test for waitForPluginBindWithFallback when both ports…
- [x] #578 add finfocus overview command — unified cost dashboard…
- [x] #597 finfocus analyzer install/uninstall
- [x] #606 fix state_test.go wantVersion skip and delegation equivalence fragility
- [x] #465 Evaluate GetPricingSpec RPC usage in finfocus core
- [x] #615 support GCP zone normalization in normalizeToRegion
- [x] #609 wrap errors from MapResources, MapStateResources, and resolveOverviewData
- [x] #603 use comma-ok idiom for altMap assertions in common_execution_test.go
- [x] #589 CodeRabbit follow-up: cleanup from #509 Pulumi auto-detect PR
- [x] #614 deep copy CostBreakdown in appendActualCostResults to prevent source…
- [x] #595 clientAdapter.GetActualCost creates phantom $0 results from empty plugin…
- [x] #596 Recorder plugin should not declare ACTUAL_COSTS capability
- [x] #592 Plugin remove fails for manually installed plugins not…
- [x] #591 Log directory not auto-created, causing fallback to stderr…
- [x] #617 move EnsureLogDir() after debug/env overrides in logging_setup.go
- [x] #581 automatic Pulumi project detection for cost commands
- [x] #575 display recommendations in resource detail view for cost…
- [x] #577 document aws-public projected cost gaps for diff support
- [x] #463 Add 'cost estimate' command for what-if scenario modeling
- [x] #533 PR #507 follow-up - docs formatting and validation.go…
- [x] #464 Add recommendation dismissal and lifecycle management
- [x] #410 Multi-Plugin Routing: Intelligent Feature-Based Plugin Selection
- [x] #221 Add flexible budget scoping (per-provider, per-resource-type)
- [x] #302 Feature: Integrate Sustainability Metrics into Engine & TUI
- [x] #303 Feature: GreenOps Impact Equivalencies
- [x] #532 Add tag-based filtering to BudgetFilterOptions
- [x] #185 Add multi-region E2E testing support
- [x] #225 Add performance optimizations and pagination for large datasets
- [x] #219 Add exit codes for budget threshold violations
- [x] #267 Add budget health calculation, threshold alerting, and cross-provider…
- [x] #263 Add provider filtering, currency handling, and summary aggregation…
- [x] #275 Enhance 'plugin init' with Remote Plan Sourcing and…
- [x] #325 Harden Nightly Analysis Workflow security and reliability
- [x] #226 Update documentation for TUI features, budgets, and recommendations
- [x] #217 Add budget status display with threshold alerts
- [x] #333 Add --estimate-confidence flag for actual cost transparency
- [x] #376 Implement GetPluginInfo consumer-side requirements from pulumicost-spec
- [x] #408 Parallel plugin metadata fetching in plugin list command
- [x] #236 Create Cross-Repository Integration Test Workflow
- [x] #218 Upgrade cost commands to Bubble Tea/Lip Gloss for…
- [x] #435 respect strict mode for spec version parse errors
- [x] #434 add Set/Get handlers for plugin_host section
- [x] #432 show installed plugins even when metadata fetch fails…
- [x] #431 add lock acquisition to RemoveOtherVersions for concurrent operation…
- [x] #430 Feature Request: Fallback to latest stable version when…
- [x] #429 replace manual t.* assertions with testify require/assert
- [x] #237 Plugin installer should remove old versions when installing…
- [x] #334 Implement TestE2E_ActualCost end-to-end test
- [x] #181 Set up AWS test account and infrastructure for…
- [x] #326 Improve fuzzing seeds, benchmarks, and validation
- [x] #182 Update documentation for E2E testing and plugin ecosystem
- [x] #349 Docs: Expand Deployment Overview
- [x] #353 Docs: Expand Support Channels
- [x] #380 Implement state-based actual cost estimation for cost actual
- [x] #245 Implement Pulumi Analyzer Plugin Integration
- [x] #177 Implement E2E test with Pulumi Automation API and…
- [x] #228 Add E2E tests for Pulumi Analyzer plugin integration
- [x] #321 Add recommendations to analyzer diagnostics
- [x] #222 Create shared TUI package with Bubble Tea/Lip Gloss…
- [x] #323 Address critical E2E and Conformance test reliability issues
- [x] #324 Fix AWS fallback scope and non-deterministic output

### 2025-Q4

- [x] #163 Implement plugin install/update/remove commands with registry and URL…
- [x] #188 Add UnaryInterceptors support to ServeConfig
- [x] #170 Standardize on zerolog v1.34.0+ with distributed tracing
- [x] #202 Epic: Engine Test Coverage Completion
- [x] #201 Epic: Plugin Ecosystem Maturity
- [x] #160 Implement Supports() gRPC handler in pluginsdk
- [x] #203 Epic: Observability & Operations
- [x] #200 Epic: Test Infrastructure Hardening

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
| Pricing Transparency | GetPricingSpec | --explain + fallback | PricingSpec | N/A |
| K8s Cost Allocation | UsageSource/Allocator (v0.6.2) | kubernetes plugin, cost cluster | Spot pricing (#406) | N/A |

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
