# Research: Shared Stack Loader

**Feature**: `1733-shared-stack-loader` | **Date**: 2026-10-22
**Method**: Read the code paths that load a Pulumi stack, run the real
binary and `pulumi` against a populated stack (`initial-30` `dev`, Pulumi
Cloud, Pulumi 3.268.0), and check each spec requirement against what the code
does today. File and line references are to `main` at the v0.5.2 release.

No `NEEDS CLARIFICATION` remains. Two decisions the owner may want to revisit
are listed under [Open decisions](#open-decisions).

## What the code does today

Several different compositions of "the resources of a stack" exist.

| Consumer | Where it loads | Composition |
| --- | --- | --- |
| `cost projected`, `forecast`, `estimate`, `recommendations` | `loadProjectedResources` (`internal/cli/cost_projected.go`) | Preview only. Plan steps become descriptors through `ingest.MapResources`. |
| `overview` plain CLI | `plainOverviewLoader` (`overview_pipeline.go`) | State merged with preview through `engine.MergeResourcesForOverview`. |
| `overview` terminal screen | `tuiOverviewLoader` | State first, preview on demand, same merge. |
| `finfocus --web` | `autoDetectOverviewLoader`, then `webData` (`web_data.go`) | Same merge for the Overview. Estimate overlays state on plan by resource ID in `estimateResources`. Recommendations and Cost read the captured state only (`loadResources`). |
| `cost actual` | `resolveResourcesFromPulumi` (export mode), then `enrichWithHistoricalResources` | State, plus resources deleted or re-identified in the billing period. |
| Terraform and OpenTofu | `loadTerraformDirectory`, `terraformPlanUnchanged`, `projectedUnchanged` | Their own plan path. Unchanged is a first-class outcome. |

Consequences found while reading:

- The web layer already contains a third implementation of "state plus plan"
  (`estimateResources`), and the capture hooks (`captureState`,
  `capturePlan`, `captureTerraform`) copy data out of the overview pipeline
  into `webData`. This is the drift the spec wants to remove.
- The web Recommendations view reads only the state, so it misses resources
  that are pending creation. The CLI `cost recommendations` reads only the
  preview, so it misses unchanged resources. They disagree in opposite
  directions.
- `MergeResourcesForOverview` silently ignores a plan step whose URN is not in
  the state unless the step is a create. Nothing detects a preview that does
  not match the state.
- A descriptor built from state carries no `Operation`.
  `ResourceDescriptor.Operation` documents that an empty value is treated as a
  create by the diff, so state fed to the projected diff unlabeled would be
  priced as new resources. `cost projected --pulumi-state` does this today.
- `projectedUnchanged` prints its sentence with `cmd.Println` whatever the
  output format, so Terraform's no-change result is not valid JSON.

## Decisions

### R1. A new package owns loading: `internal/stackload`

- **Decision**: Create `internal/stackload`. It runs `pulumi stack export` and
  `pulumi preview --json` (or reads the explicit files), and returns one
  `Snapshot`. Every Pulumi consumer calls it.
- **Rationale**: `engine` cannot import `ingest` (`ingest` imports `engine`),
  so the step cannot live in `engine`. `internal/cli` is not importable by
  `internal/webui`, and `webui` is transport only. A new package that imports
  `pulumi`, `ingest`, and `engine` breaks no existing boundary.
- **Alternatives considered**: Putting it in `internal/cli`. Rejected: it is
  where the divergence started, and the web layer reaches the CLI only through
  callbacks. Putting it in `engine`. Rejected: import cycle.

### R2. Keep the existing intermediate types; add one merge point

- **Decision**: The snapshot holds the types the overview already uses
  (`[]engine.StateResource`, `[]engine.PlanStep`) plus the parsed plan. Two
  derived views come from it: overview rows (unchanged, still
  `MergeResourcesForOverview`) and descriptors (new, one function).
- **Rationale**: The overview, the TUI, the web, and history recording all
  consume these types and have tests. Replacing them would touch every
  consumer for no user benefit. The only genuinely new logic is how
  descriptors are produced from the pair.
- **Alternatives considered**: One new canonical resource type that everything
  converts to. Rejected: large blast radius and a fourth representation.

### R3. Descriptors: plan first, then unchanged resources from the state

- **Decision**: `Descriptors()` returns the plan-derived descriptors exactly as
  `ingest.MapResources` produces them today (same order, same properties), then
  appends one descriptor per custom state resource whose URN is not in the
  plan, built with `ingest.MapStateResource` and `Operation = "same"`. A
  resource in both is taken from the plan. Each descriptor records its origin
  (plan or state).
- **Rationale**: Existing output order, properties, cache keys, and the diff
  stay byte-identical for plan resources, which is what keeps the plan total
  and the budget result unchanged (SC-007). The diff already prices `same`
  once (`planDiffPricing`, FR-012).
- **Alternatives considered**: Build all descriptors from state and overlay the
  plan. Rejected: it changes property shaping for every plan resource and risks
  new cache keys and prices.

### R4. Two totals, one pricing pass

- **Decision**: The projected diff prices every descriptor once, then reports
  two sums. The plan total is the sum over plan-origin entries and keeps the
  field `summary.totalMonthly` and `diff.totalAfter`. The stack total is the
  sum over all entries and is a new field. `resources[]`, `summary` by
  provider and service, and `diff.unchanged` keep counting plan-origin entries
  only. State-added unchanged resources appear in a separate
  `stack.unchanged[]` list.
- **Rationale**: `Summary.TotalAfter` today covers every descriptor in the
  plan, including `same` rows the preview lists, and
  `executeCostProjected` passes it to budget evaluation. The Codex review of
  the spec found that "change total" was the wrong name for it. Keeping
  membership by origin makes the plan total, the budgets, and the exit codes
  identical to v0.5.2 for every input, including saved previews that list
  unchanged resources.
- **Alternatives considered**: Put state-added rows into `resources[]`.
  Rejected: it changes `summary.byProvider` and `totalMonthly` for anyone
  parsing the JSON.

### R5. `cost actual` takes the deployed subset

- **Decision**: `DeployedDescriptors()` returns the state descriptors exactly as
  `ingest.MapStateResources` produces them today (no `Operation`). `cost
  actual`, the web Cost view, and `cost history collect` keep their history
  enrichment (`enrichWithHistoricalResources`, `recordDescriptorHistory`).
  A saved preview and the web Cost view use `ActualDescriptors()`: the deployed
  subset when state was read, otherwise the plan, which is what v0.5.2 priced,
  so a plan file with no state keeps its figures and CLI and web agree.
- **Rationale**: A resource that exists only in the plan has no cloud ID and no
  charges, and a plugin query for it is noise. The Codex review found that the
  billing-period enrichment adds deleted resources that are in neither state
  nor preview, so identical membership with the projected family would drop
  real spend.
- **Alternatives considered**: Give `cost actual` the full live inventory.
  Rejected for the reason above.

### R6. Read the state concurrently with the preview

- **Decision**: `Load` starts the state export and the preview together, like
  `loadOverviewProjectData` does, and cancels one if the other fails fatally.
- **Rationale**: Measured on `initial-30` `dev` (65 stack resources, Pulumi
  Cloud): `pulumi stack export` 0.76 to 0.80 s; `pulumi preview --json` 11.5 to
  13.6 s; both in parallel 10.2 s. The state read adds no measurable wall time.
  Run in sequence it would add under one second. SC-005 (3 seconds) is
  therefore conservative and stays as written.
- **Alternatives considered**: Read the state only when the preview omitted
  unchanged resources. Rejected: the preview's `changeSummary.same` always
  reports a count, so the condition is nearly always true, and the saving is
  under a second.

### R7. State that cannot be read degrades to preview-only with a category

- **Decision**: A failed state export does not fail the command when a preview
  is available. The snapshot records a category (`backend_unreachable`,
  `passphrase_required`, `read_failed`) and no raw error text. Commands print a
  stderr notice and set `stack.complete: false` and `stack.incompleteReason` in
  JSON. When there is no preview either, the error is returned as today.
- **Rationale**: FR-008 and spec User Story 5. Error text from a backend can
  carry URLs or secret-adjacent detail, so only a category is exposed, as the
  redaction rules in `CLAUDE.md` require. A hard failure would block commands
  that work today.
- **Alternatives considered**: Fail the command when state cannot be read.
  Rejected: breaks `cost projected` for CI jobs with a plan file and no state
  access.

### R8. Saved preview with no state: report the omitted count

- **Decision**: `ingest.PulumiPlan` gains `ChangeSummary` (`changeSummary` in the
  JSON). The omitted-unchanged count is `changeSummary.same` minus the `same`
  steps the file lists. When state is unavailable the count is reported in the
  notice and in `stack.omittedUnchanged`.
- **Rationale**: Measured: a default preview reports `same: 59` while listing 1
  `same` step; `--show-sames` lists 59. The count is always present, needs no
  extra call, and is exact. It is the only signal in a plan-file-only run
  (spec User Story 4).
- **Alternatives considered**: Heuristics from step counts. Rejected: the
  summary is authoritative.

### R9. `emptyPlan` keeps its meaning by origin

- **Decision**: `finfocus.emptyPlan` (added in v0.5.2, #1897) is true when no
  plan-origin descriptor is a priceable (non-Pulumi-internal) type. It is
  evaluated before filtering and before pricing, as today. State-added
  resources never make a plan non-empty.
- **Rationale**: FR-006. If the check ran over all descriptors, an unchanged
  stack with readable state would stop being flagged, and the skills and the
  Action that read the flag would see a different answer for the same plan.
- **Alternatives considered**: Drop the flag once the stack total exists.
  Rejected: it would be a breaking change for readers of the flag.

### R10. No-change output for Pulumi follows Terraform in table form only

- **Decision**: For table output with no plan-origin change and a complete
  stack, print `No changes. Projected monthly cost is zero.` (the Terraform
  sentence) followed by one line giving the stack total. JSON and NDJSON stay
  valid documents with `emptyPlan: true`.
- **Rationale**: FR-005 and FR-006. Reusing `projectedUnchanged` unchanged would
  print a bare sentence into JSON, which is what Terraform does today and would
  break JSON readers for Pulumi.
- **Alternatives considered**: Fix Terraform's JSON output in this feature.
  Rejected: out of scope (FR-011). It is listed under Open decisions.

### R11. A preview that does not match the state is reported, not merged

- **Decision**: `Snapshot` records the URNs of plan steps with an operation
  other than create whose URN is absent from the state, and creates whose URN
  is already in the state. Commands print one notice with the count and treat
  those resources as the plan lists them. Nothing is dropped.
- **Rationale**: Spec edge case. Today the overview ignores such steps without
  a word. A count is cheap, needs no new call, and does not block.
- **Alternatives considered**: Compare the state's manifest time with the
  preview. Rejected: the preview carries no revision marker to compare.

### R12. Web: `webData` holds a snapshot, not three copies

- **Decision**: `webData` retains the `Snapshot`. `loadResources`,
  `estimateResources`, `recommendations`, and `actualCosts` derive their input
  from it. `captureState`, `capturePlan`, and `captureTerraform` become one
  store of the snapshot. Handlers in `internal/webui` are unchanged.
- **Rationale**: Principle VII and `architecture/web-ui.md`: the web layer is
  transport only and calls the same Go methods as the CLI. The web Estimate
  overlay and the web Recommendations state-only read are two more divergences
  to remove.
- **Alternatives considered**: Leave the web as is. Rejected: the spec's
  SC-003 requires identical resource sets.

### R13. Side effects stay with the caller

- **Decision**: History recording (`recordHistorySnapshot`,
  `recordHistoryPlanLineage`), dismissal handling, plugin opening, and ESC
  environment handling stay in `internal/cli`. `stackload` has none of them.
- **Rationale**: Constitution Persistence Model: no command requires prior
  local state, and stores are opt-out. A loader with side effects would make
  `cost projected` start writing history, which it does not do today.

### R14. Forecast keeps plan membership for its total

- **Decision**: `cost forecast` derives its inventory from the snapshot but
  keeps its previous total over plan-origin resources. Its totals do not
  change in this feature.
- **Rationale**: The spec's compatibility promise (SC-007) names the
  `cost projected` plan total. Adding unchanged resources to a forward-looking
  growth projection would change an existing figure, and the spec does not
  decide that.
- **Alternatives considered**: Include unchanged resources in the forecast.
  See Open decisions.

### R15. Constitution and cross-repo

- **Decision**: No protocol, proto, or `finfocus-spec` change; no plugin
  change; no new persistent store; no new dependency. Web Parity applies and is
  met through `contracts/http-api.md`.
- **Rationale**: The feature reorganizes how the core gathers resources. It
  sends plugins the same descriptors for resources they already saw, plus
  descriptors for unchanged resources they were not asked about before.

## Risks and how the plan answers them

| Risk | Answer |
| --- | --- |
| A state-derived descriptor prices differently from the plan-derived one for the same resource, so cache keys and plugin results differ | A parity test feeds the same URN through both paths and compares descriptors apart from `Operation`, origin, and the injected `pulumi:` properties (`PropertyPulumiCreated`, `PropertyPulumiCloudID`, `PropertyPulumiURN`). Any difference is a named, documented exclusion. |
| Pulumi secrets in state inputs reach a plugin | Descriptors go through the same credential and secret rules as today (`config.IsCredentialName`, `history.IsPulumiSecret`). A test puts a secret envelope in state inputs and asserts the request omits it. |
| More resources means more plugin calls for `recommendations` and `estimate` | The engine already batches past `batchProcessingThreshold` (100). Measured in the quickstart on a 65-resource stack. |
| Recommendation and estimate figures change for users with deployed stacks | Intended: the resource set grows to the live inventory. Release notes and `docs/` state it. |
| `webData` refactor breaks Estimate or Cost behavior | The existing web handler and frontend tests pin the payloads. The plan runs them before and after each slice. |

## Open decisions

1. **Forecast total.** R14 keeps it unchanged. Including unchanged resources
   would make the forecast describe the whole stack. Recommendation: keep it
   unchanged now and decide with the owner after the stack total has shipped.
   Tracked in #1903.
2. **Terraform JSON.** `projectedUnchanged` prints a sentence into JSON output.
   Recommendation: a separate issue, since FR-011 leaves Terraform alone.
   Tracked in #1904.
