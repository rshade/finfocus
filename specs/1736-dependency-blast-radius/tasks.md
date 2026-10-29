# Tasks: Dependency Blast Radius

**Input**: Design documents from `specs/1736-dependency-blast-radius/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/,
quickstart.md  
**Tests**: Included. The constitution makes TDD non-negotiable and requires
golden files for TUI changes, so each story writes its tests first and sees
them fail.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an
  incomplete task)
- **[Story]**: US1 terminal view, US2 web view, US3 mid-change accuracy
- Every task ships fully implemented: no stubs, no TODO comments
  (constitution Principle VI)

## Phase 1: Setup

- [X] T001 Confirm against a real state file that `dependsOn` a component records the component URN in `dependencies` (quickstart step 2, `../finfocus-demo` stack `dev`), and record the result in `specs/1736-dependency-blast-radius/research.md` R1. If it records the children instead, drop the ancestor walk from R2 and `data-model.md` before T010. Result: it records the children; ancestor walk dropped (2026-10-28).
- [X] T002 Render the existing Overview detail panel with playwright-mcp against a populated stack (`PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port <port>`), save screenshots under `.playwright-mcp/`, and replace the pending "Render evidence" line in `specs/1736-dependency-blast-radius/spec.md` with the stack and date.
- [X] T003 Record the pre-change baseline: run `go vet ./...` and `go test ./internal/engine/... ./internal/stackload/... ./internal/tui/... ./internal/webui/... ./internal/cli/...` and note the failing set (CLAUDE.md: compare the failure set, not the count, because tests that read the real `~/.finfocus` can fail independently).

---

## Phase 2: Foundational (blocks all user stories)

- [X] T004 [P] Write failing tests in `internal/stackload/graph_test.go` for building the graph from deployment resources: `dependencies` and every `propertyDependencies` list merge into one sorted, de-duplicated `DependsOn`; components keep `Component` true and their own edges; `Available` is true only when at least one node has a dependency; a Terraform or OpenTofu snapshot and an empty state return `Available` false.
- [X] T005 [P] Define `DependencyGraph`, `DependencyNode`, `BlastRadius`, `BlastDependent` and `BlastLeftOut` with Godoc on every exported symbol, exactly as in `specs/1736-dependency-blast-radius/data-model.md`, in `internal/engine/blast_radius.go`.
- [X] T006 Implement the graph builder in `internal/stackload/graph.go` (converts `ingest.StackExportResource` into `engine.DependencyGraph`; `internal/engine` cannot import `ingest`), keep every deployment resource available to it in `internal/stackload/load.go` (today only `RawState` and `ComponentURNs` survive), and add the `Graph` field and `Graph()` accessor in `internal/stackload/snapshot.go`. T004 must pass.
- [X] T007 Run `go test -race -shuffle=on -count=3 ./internal/stackload/...` and `go vet ./internal/engine/...`; fix anything T005 or T006 broke before starting a story.

**Checkpoint**: the snapshot carries a graph; nothing renders it yet.

---

## Phase 3: User Story 1 - See what depends on a resource (Priority: P1)

**Goal**: The terminal overview detail view shows a BLAST RADIUS tree with a
count and a monthly total.

**Independent Test**: Run the overview on a stack where a database is used by a
service used by a load balancer; the database's detail view lists the service
at depth 1, the load balancer at depth 2, and a total equal to their summed
monthly cost.

### Tests for User Story 1 (write first, confirm they fail)

- [X] T008 [P] [US1] Table-driven tests for `ComputeBlastRadius` in `internal/engine/blast_radius_test.go`: direct and transitive dependents with depth; a diamond lists once at its shortest depth; a cycle terminates; an unknown URN returns an empty available result; `Available` false returns the unavailable result; a resource whose `DependsOn` lists a component's members is a dependent of each member (R1); `Via` precedes each dependent in the list; output order is stable across runs; `Count` and `MonthlyTotal` over priced Active rows match data-model invariants 1 to 5.
- [X] T009 [P] [US1] Golden tests for the detail view in `internal/tui/overview_golden_test.go` covering a populated tree, an empty radius ("nothing depends on this resource") and an unavailable graph ("dependency information is not available"); add the expected files under `internal/tui/testdata/` as `detail_blast_radius_*.golden`.

### Implementation for User Story 1

- [X] T010 [US1] Implement `ComputeBlastRadius` in `internal/engine/blast_radius.go`: reverse index built once per graph (a node's dependents are the nodes that list it in `DependsOn`; no ancestor walk, R2), breadth-first traversal with a visited set, `Via` set to the first discoverer (URNs sorted within a level), count and total over priced Active rows on `ProjectedCost.MonthlyCost`. T008 must pass.
- [X] T011 [US1] Add the graph-delivery Bubble Tea message in `internal/tui/overview_messages.go` and store the graph on `OverviewModel` (handle the message) in `internal/tui/overview_model.go`.
- [X] T012 [US1] Send `snapshot.Graph()` to the model when the pipeline publishes rows, in `internal/cli/overview_pipeline.go` (the TUI path around the existing `Snapshot` publish at lines 877 to 892).
- [X] T013 [US1] Render the section: add `renderDetailBlastRadius` in a new `internal/tui/overview_blast_radius.go` (indent by depth; box-drawing glyphs with the plain-mode fallback from `internal/tui/accessibility.go`; count and total at the top so scrolling never hides them) and call it from `renderDetailViewForDay` in `internal/tui/overview_view.go` after the cost sections and before the budget block. T009 must pass.
- [X] T014 [US1] Render the detail view for a real fixture, read the full output for column alignment and ordering, then regenerate goldens with `UPDATE_GOLDEN=1 go test -run TestGolden ./internal/tui/...` and review the diff of every golden file.
- [X] T015 [US1] Add a state fixture with a database, a service and a load balancer in `internal/cli/testdata/blast_radius_state.json` and an integration-style test in `internal/cli/overview_blast_radius_test.go` that drives the pipeline into the model and asserts the rendered section for the database (use `t.Setenv("FINFOCUS_HOME", t.TempDir())`; this test is not parallel).

**Checkpoint**: User Story 1 works on its own; the terminal overview is
shippable here.

---

## Phase 4: User Story 2 - See the same blast radius in the web dashboard (Priority: P2)

**Goal**: The web Overview detail panel shows the same dependents, count and
total as the terminal.

**Independent Test**: Open the same stack in both, select the same resource,
and compare: identical dependents, count and total.

### Tests for User Story 2 (write first)

- [X] T016 [P] [US2] Handler tests in `internal/webui/handlers_overview_test.go`: `GET /api/overview/resource` returns `blastRadius` equal to `engine.ComputeBlastRadius` for the same rows and graph; an unknown URN is still 404 `not_found`; an unavailable graph returns `{urn, available:false}`; the response contains no properties, tags or inputs.
- [X] T017 [P] [US2] Parity test in `internal/cli/web_blast_radius_parity_test.go`: load the T015 fixture once, render the terminal section and fetch the web response, and assert the same dependents, count, total and left-out counts for every resource in the fixture (SC-002). Not parallel (builds a root command).

### Implementation for User Story 2

- [X] T018 [US2] Add `SetDependencyGraph(engine.DependencyGraph)` to `Session` in `internal/webui/session.go` (stored under the session mutex, read as a clone); `internal/webui` must not import `cli`, `registry` or `pluginhost`.
- [X] T019 [US2] Add `BlastRadius *engine.BlastRadius` (`json:"blastRadius"`) to `ResourceDetail` and fill it by calling `engine.ComputeBlastRadius` with `s.session.Rows()` and the stored graph in `handleOverviewResource` in `internal/webui/handlers_overview.go`. T016 must pass.
- [X] T020 [US2] Publish the graph to the session wherever the snapshot is published in `internal/cli/web_data.go` (`setPipelineSource`, initial load and after an on-demand preview).
- [X] T021 [US2] Render the block in the detail panel in `internal/webui/static/views/overview.js` as a captioned `.detail-table` (indent by `depth`, numeric columns aligned, total and left-out counts in the caption, the two empty messages), with only the minimum rules needed in `internal/webui/static/styles.css`. T017 must pass.

**Checkpoint**: Stories 1 and 2 both work; the web follows the terminal.

---

## Phase 5: User Story 3 - Trust the numbers when the stack is mid-change (Priority: P3)

**Goal**: New, deleting, unpriced and component dependents are marked, the
total counts only what will exist and has a price, and a preview updates the
section in both views.

**Independent Test**: Run on a preview that creates one dependent, deletes
another and includes an unpriced type; the section marks all three and the
total counts only the priced dependent that will exist.

### Tests for User Story 3 (write first)

- [X] T022 [P] [US3] Overlay tests in `internal/stackload/graph_test.go`: a `create` step adds a node with its new-state edges; `update` and `replace` replace the node's edges; `delete` keeps the node flagged `Deleting`; a `same` step adds nothing; a resource omitted from a default preview keeps its state edges; `WithPlan(nil)` returns the state-only graph.
- [X] T023 [P] [US3] Table-driven tests in `internal/engine/blast_radius_test.go`: Creating counts; Replacing counts; Deleting is listed, excluded and counted in `LeftOut.Deleting`; `ProjectedCost == nil` and `ProjectedPricingFailed` are unpriced; a `$0.00` price is priced; components are listed, uncounted and not in `LeftOut.Unpriced`; a dangling reference increments `LeftOut.Unresolved`; rows with different currencies set `MixedCurrency` and withhold the total; a blank currency is USD.
- [X] T024 [P] [US3] Golden tests in `internal/tui/overview_golden_test.go` for a mid-change tree (new, deleting, unpriced, component, left-out note) and for the view after `p` runs a preview on a state-only run; files `detail_blast_radius_midchange.golden` and `detail_blast_radius_preview.golden` under `internal/tui/testdata/`.

### Implementation for User Story 3

- [X] T025 [US3] Implement the preview overlay in `internal/stackload/graph.go` and have `Snapshot.WithPlan` in `internal/stackload/snapshot.go` rebuild the graph from the new plan. T022 must pass.
- [X] T026 [US3] Extend `ComputeBlastRadius` in `internal/engine/blast_radius.go` with status handling, `Priced`, `LeftOut`, component flags and the currency rule from research R4. T023 must pass.
- [X] T027 [US3] Render the status markers, the unpriced marker and the left-out note ("N deleted and M unpriced dependents are not in the total") in `internal/tui/overview_blast_radius.go`, and resend the graph to the model after the in-TUI preview in `internal/cli/overview_pipeline.go` and `internal/tui/overview_model.go`. T024 must pass.
- [X] T028 [US3] Confirm the web recomputes after an on-demand preview: add a test in `internal/cli/web_blast_radius_parity_test.go` that applies a preview to a state-only session and asserts `blastRadius` reflects the new graph, and fix `internal/cli/web_data.go` if the old graph survives.
- [X] T029 [US3] Render the same markers and left-out note in the detail panel in `internal/webui/static/views/overview.js`.

**Checkpoint**: All three stories work independently and together.

---

## Phase 6: Web Parity (constitution Principle VII)

- [X] T030 Diff the field names in `specs/1736-dependency-blast-radius/contracts/http-api.md` against the real `GET /api/overview/resource` response and correct whichever is wrong; the contract must name `engine.ComputeBlastRadius` and `Snapshot.Graph` as the Go methods the hook calls.
- [X] T031 Render the implemented detail panel with playwright-mcp against the same stack as T002, compare it with the nearest existing view and the spec's Web UI Fit section (slots, `.detail-table`, alignment), and record the stack and date in `specs/1736-dependency-blast-radius/spec.md`.
- [X] T032 [P] Update `docs/src/content/docs/commands/overview.md` (the BLAST RADIUS section: what counts as a dependent, what the total includes and leaves out, the Terraform message).
- [X] T033 [P] Update `docs/src/content/docs/guides/web-ui.md` and `docs/src/content/docs/reference/web-ui.md` (the detail-panel block and the `blastRadius` field).
- [X] T034 Skipped by the user's decision on 2026-10-29: no `web-ui` follow-up issue is filed, because the UX work was built in Phase 6A (T041 to T050) and nothing is left out. This departs from Principle VII.3, which asks for the issue; the departure is recorded in `specs/1736-dependency-blast-radius/spec.md` Web UI Fit.

---

## Phase 6A: Web UX Refinement (US2, constitution Principle VII)

Priority P2, the same as US2: this finishes the web surface, so it runs after
US3 only because it edits the files US3 touched.

**Goal**: The web detail-panel block is polished beyond the first render:
readable on long lists and narrow panels, status never shown by color alone,
dependents reachable by keyboard and click, copy matching the terminal, and
accessible in light and dark themes. Scope is `internal/webui/static/` only;
no engine, API, redaction, or data-shape change (the `blastRadius` payload
stays as in `contracts/http-api.md`).

**Independent Test**: Open a resource with a multi-level tree in the web
dashboard; scroll a long list with the caption and column headers in view,
Tab to a dependent, press Enter to open its detail, and press Back to return;
repeat in light and dark themes at a narrow width.

### Tests for Phase 6A (write first, confirm they fail)

- [X] T041 [P] [US2] Renderer tests in `internal/webui/frontendtest/overview.test.mjs` for `blastRadiusSection`: connector glyphs follow sibling position (`\u251c` for a non-last sibling, `\u2514` for the last); every status (new, deleting, replacing, updating, component, unpriced) shows a text label in its own cell; dependents render as buttons whose activation calls `openDetail` with the dependent URN; the name cell is a row header (`th scope="row"`) and carries visually hidden level text; the two empty messages and the left-out note match the strings pinned in T042; a list over the height bound renders inside the scroll container.
- [X] T042 [P] [US2] Pin the user-visible copy once: add a "Copy" table (heading, count and total summary, the Back button label `Back to NAME`, left-out note, the two empty messages, status labels) to `specs/1736-dependency-blast-radius/contracts/http-api.md`, and add assertions in `internal/tui/overview_golden_test.go` (terminal) and `internal/webui/frontendtest/overview.test.mjs` (web) that both surfaces use those exact strings where the wording is shared (terminal-only indentation and glyph padding are exempt).

### Implementation for Phase 6A

- [X] T043 [US2] Layout in `internal/webui/static/styles.css`: wrap the blast table in a bounded-height scroll container (`.blast-scroll`, `max-height` in `rem`, `overflow: auto`), keep the caption and `thead` visible with `position: sticky`, let the name column wrap and the Type column truncate with a title on narrow panels, and keep the Monthly column right-aligned. Use existing custom properties only; no new colors. T041 must pass for the container assertion.
- [X] T044 [US2] Rendering in `internal/webui/static/views/overview.js` (`blastRadiusSection`, `blastTreeOrder`): derive the connector per row from sibling position and depth (continuing guides for ancestors that have later siblings), render status as its own text-labelled cell with a class per status for styling that never carries meaning alone, and keep the unpriced and component markers distinct from the cost cell. T041 rendering assertions must pass.
- [X] T045 [US2] Interaction in `internal/webui/static/views/overview.js`: render each dependent name as a `button` styled as a link that calls `openDetail(urn)`; keep a small back stack of opened URNs so a `Back to NAME` button in the detail header returns to the previous resource; `closeDetail` still returns focus to the originating table row (`lastFocus`), and the new detail panel moves focus to its heading as `renderDetail` does today. A dependent URN the session no longer has shows the existing 404 detail message without breaking the stack. T041 interaction assertions must pass.
- [X] T046 [US2] Accessibility in `internal/webui/static/views/overview.js` and `internal/webui/static/styles.css`: caption stays the table's accessible name, header cells keep `scope="col"`, each name cell is `th scope="row"` with a visually hidden "level N" prefix so depth is exposed without relying on glyphs, status label text meets WCAG 2.2 AA (4.5:1 text, 3:1 for focus indicators) against its background in both `prefers-color-scheme` palettes, and focus rings on the dependent buttons use the existing focus style. Verify the ratios in T047 by computing them from the rendered styles in both themes and record them in `specs/1736-dependency-blast-radius/spec.md`. T041 must pass in full.

### Verification for Phase 6A

- [X] T047 [US2] Render the refined block with playwright-mcp against a populated stack (`../finfocus-demo` after `pulumi up`, or `finfocus --web --pulumi-state internal/cli/testdata/blast_radius_state.json`): light and dark themes, a narrow viewport, a tree deep enough to scroll, and a keyboard-only pass (Tab, Enter, Back, Escape). Save screenshots under `.playwright-mcp/`, compare with the nearest existing detail block, and record the stack and date in the "Implemented render" entry of `specs/1736-dependency-blast-radius/spec.md`. Run `pulumi destroy` afterwards if anything was deployed.
- [X] T048 [P] [US2] Update `docs/src/content/docs/guides/web-ui.md` and `docs/src/content/docs/reference/web-ui.md` for the scroll behavior, the clickable dependents and Back button, the status labels, and the keyboard use; run `markdownlint` on both and `make docs-lint`.
- [X] T049 [P] [US2] Record the interaction contract in `specs/1736-dependency-blast-radius/contracts/http-api.md`: the dependent buttons call the existing `GET /api/overview/resource` with the dependent URN and add no endpoint, parameter or field. Correct `specs/1736-dependency-blast-radius/plan.md` project-structure notes if the file list changed.
- [X] T050 [US2] Gate: `make test-frontend`, `go test -race -shuffle=on -count=3 ./internal/webui/... ./internal/tui/...`, `make test-e2e-web` when Chromium is available (note it if skipped), `make lint`, and `make test`. Compare any failure with the T003 baseline.

**Checkpoint**: The web block is complete, accessible, and verified in a
browser; Phase 7 can close the feature.

---

## Phase 7: Polish and Cross-Cutting

- [X] T035 [P] Correct `ROADMAP.md` (the entry at issue #366 still says it consumes `CostAllocationLineage` / `ParentResourceID` from `finfocus-spec`; say it uses Pulumi dependencies from the shared stack snapshot).
- [X] T036 [P] Add the non-obvious rules to `CLAUDE.md` under Package-Specific Gotchas (Engine): the graph lives on `stackload.Snapshot`; Pulumi records `dependsOn` a component as its members, so the graph has no parent walk; `OverviewRowResult.ParentURN` is cluster expansion, not dependencies; the total is one basis and excludes deleting, unpriced and components. Do not run `update-agent-context.sh`.
- [X] T037 Check Godoc on every new exported symbol, run `markdownlint` on every touched Markdown file and `make docs-lint`.
- [X] T038 Run `go vet ./...`, `make test`, `go test -race -shuffle=on -count=3 ./internal/engine/... ./internal/stackload/... ./internal/tui/... ./internal/webui/...`, and `make lint` (allow more than five minutes). Compare any failures with the T003 baseline.
- [X] T039 Verify with the real binary in `../finfocus-demo` (stack `dev`, empty passphrase) following `quickstart.md` steps 1, 3 and 4: terminal and web agree, a preview updates the section, a leaf resource shows the empty message. Run `pulumi destroy` afterwards if anything was deployed, and check `ps` for orphaned `finfocus-plugin-aws-public-*` children.
- [X] T051 [US1] Cover SC-004 and FR-011: add a benchmark or test in `internal/engine/blast_radius_test.go` that builds a 500-resource graph with a long chain and a wide fan-out and asserts `ComputeBlastRadius` finishes within the bound recorded in the test (reverse index built once per graph), and a test in `internal/cli/overview_blast_radius_test.go` that opening the detail view makes zero plugin pricing calls (a counting fake).
- [X] T052 [US1] Cover FR-012 and FR-013: assert in `internal/cli/overview_blast_radius_test.go` that `overview --output table|json|ndjson` output for the T015 fixture is byte-identical to the pre-feature golden, and that the JSON redaction path leaves no property, tag or input in the web `blastRadius` payload. Also confirm the TUI detail scroll keeps the count and total in view (`visibleDetail` with `detailScroll` on a long tree) with a golden in `internal/tui/overview_golden_test.go`.
- [X] T053 Re-run the gate after T041 to T052: `go vet ./...`, `make test-frontend`, `make test`, `make lint`, and `go test -race -shuffle=on -count=3 ./internal/engine/... ./internal/stackload/... ./internal/tui/... ./internal/webui/...`. Compare failures with the T003 baseline.
- [X] T040 Dropped by the user's decision on 2026-10-29: no comment is posted on issue #366. The `ROADMAP.md` entry (T035) already carries the corrected premise. Do not edit `CHANGELOG.md` (release-please owns it).

---

## Phase 8: ISS-001 Provider, `deletedWith` and `replaceWith` edges (US1, US3)

**Goal**: the graph follows every link Pulumi's `DependingOn` follows except
`parent` (spec FR-005 and Clarifications 2026-10-28, research R12,
`data-model.md` "Provider reference" and "Source fields", plan.md
"Discovered Constraints"). Only `internal/ingest` and
`internal/stackload/graph.go` change; `engine.ComputeBlastRadius`, the TUI,
the web handler and panel, and `contracts/http-api.md` stay as they are.

**Independent test**: on `initial-30`, selecting `civoCluster` lists 8
dependents (the `civo-k8s` provider and seven Kubernetes resources), up from 1
(quickstart step 5).

### Tests for Phase 8 (write first, confirm they fail)

- [X] T054 [US1] Add state cases to the table in `internal/stackload/graph_test.go` (`BuildDependencyGraph`): a `Provider` of `<provider-urn>::<id>`, where the provider URN itself contains `::`, adds an edge on the provider URN only; a state whose only links are provider references is `Available`; a reference with no `::` and an empty `Provider` add nothing; `DeletedWith` adds an edge; each `ReplaceWith` entry adds an edge; every kind merges with `Dependencies` and `PropertyDependencies` into one sorted, de-duplicated `DependsOn`. Keep the existing "parent alone is not a dependency" and "empty state has no graph" cases unchanged. They fail to compile until T057.
- [X] T055 [US3] Add overlay cases to `internal/stackload/graph_test.go` (`OverlayPlanGraph`): a `create` step whose `NewState.Provider` ends in the unknown-id sentinel `04da6b54-80e4-46f7-96ec-b56ff0331ba9` yields the provider URN; an `update` step replaces the node's whole link set, `DeletedWith` and `ReplaceWith` included; a step with no state uses `PulumiStep.Provider`; a `pulumi:providers:aws` step state with no `custom` field is not `Component`, while a `pulumi:pulumi:Stack` step state with no `custom` still is.
- [X] T056 [US1] Add an end-to-end case to `internal/stackload/graph_test.go` that builds the cluster shape from `ingest.StackExportResource` values and runs `engine.ComputeBlastRadius`: a cluster; a provider whose `PropertyDependencies` names the cluster; seven workloads whose only link is `Provider` on that provider; and one workload with `DeletedWith` the cluster. The cluster's radius has 8 dependents: the provider at depth 1, the workloads at depth 2, each listed once even when two links reach it. With one workload priced in the rows, `MonthlyTotal` equals its cost and `LeftOut.Unpriced` counts the rest. Fails until T058.

### Implementation for Phase 8

- [X] T057 [P] [US1] Add `DeletedWith string` (`json:"deletedWith,omitempty"`) and `ReplaceWith []string` (`json:"replaceWith,omitempty"`), with Godoc, to `ingest.StackExportResource` in `internal/ingest/state.go` and `ingest.PulumiState` in `internal/ingest/pulumi_plan.go`. In the existing ingest tests (`internal/ingest/state_test.go`, `internal/ingest/pulumi_plan_test.go`), assert that a state export and a preview step state carrying both fields decode them. `PulumiStep` gets no new fields.
- [X] T058 [US1] In `internal/stackload/graph.go`, replace `mergeDependencies` with one helper that takes `dependencies`, `propertyDependencies`, the provider reference, `deletedWith` and `replaceWith`, and returns the sorted, de-duplicated `DependsOn` with empty URNs dropped. Add an unexported `providerURN` that cuts the reference at its last `::` (`strings.LastIndex`) and returns `""` when there is none. Use the helper in `BuildDependencyGraph`, and update the Godoc on `BuildDependencyGraph` to name the link kinds. T054 and T056 must pass.
- [X] T059 [US3] In `internal/stackload/graph.go`, have `stepDependencies` pass the step state's `Provider`, `DeletedWith` and `ReplaceWith` to the T058 helper, falling back to the step's own `Provider`, `Dependencies` and `PropertyDependencies` when the state is nil. Have `isStepComponent`'s fallback return false for the `pulumi:providers:` prefix and update its comment. T055 must pass.

### Docs, parity and gate for Phase 8

- [X] T060 [P] Update the "What counts as a dependent" bullet in `docs/src/content/docs/commands/overview.md`: a resource also depends on the provider it was created with and on the resources named in `deletedWith` and `replaceWith`, so a cluster lists the workloads deployed through its provider, and a default provider lists every resource it manages. Keep the component sentence. Run `markdownlint` and `make docs-lint`.
- [X] T061 [P] Update the blast-radius bullet under Package-Specific Gotchas (Engine) in `CLAUDE.md`: the edges are Pulumi's `DependingOn` link set without `parent` (`dependencies`, `propertyDependencies`, `deletedWith`, `replaceWith`, `provider`); a provider reference is cut at its last `::`; the `isStepComponent` fallback excludes `pulumi:providers:`. Do not run `update-agent-context.sh`.
- [X] T062 Render with playwright-mcp against `initial-30` (stack `dev`, empty passphrase) following quickstart step 5: `civoCluster` shows 8 dependents, `civo-k8s` shows 7, and the terminal (`finfocus overview`, Enter on `civoCluster`) shows the same dependents, count and note. Re-run quickstart step 3 on `../finfocus-demo` with **Run preview** and confirm `demo-vpc` still shows "3 dependents, $7.59/month". `handleOverviewResource` still calls `engine.ComputeBlastRadius` over `Snapshot.Graph`, so confirm `contracts/http-api.md` needs no edit. Record the stack and date in the Web UI Fit render-evidence line in `specs/1736-dependency-blast-radius/spec.md`. No `web-ui` follow-up issue: there is no UX change, and the T034 decision stands. Stop each server with an anchored `pkill -f '^<repo-root>/bin/finfocus --web'` and check `ps` for orphaned `finfocus-plugin-*` children.
- [X] T063 Gate: `go vet ./...`, `make test`, `go test -race -shuffle=on -count=3 ./internal/ingest/... ./internal/stackload/... ./internal/engine/...`, and `make lint` (allow more than five minutes). Confirm the T051 bound still holds, since a default provider adds at most one edge per resource. Compare any failures with the T003 baseline, then set ISS-001 to Resolved in `specs/1736-dependency-blast-radius/issues.md`.

**Checkpoint**: a cluster's radius includes everything deployed through its
provider, in the terminal and the web alike, and the total still counts each
priced dependent once.

---

## Dependencies and Execution Order

- **Phase 1** has no dependencies; T001 gates the component rule used in T010.
- **Phase 2** depends on Phase 1 and blocks every story. T004 and T005 are
  parallel; T006 needs both; T007 closes the phase.
- **US1 (Phase 3)** depends on Phase 2. **US2 (Phase 4)** depends on US1's
  `ComputeBlastRadius` (T010). **US3 (Phase 5)** changes the same engine and
  rendering files as US1 and US2, so it follows them; its overlay tasks
  (T022, T025) touch only `stackload` and can start after Phase 2.
- **Phase 6** needs US2 and US3. T034 was skipped by the user (no follow-up
  issue; Phase 6A covers the UX work).
- **Phase 6A** (T041 to T050) needs T021 and T029. T041 and T042 are
  parallel and written first; T043 to T046 touch the same two static files, so
  they run in order; T047 needs all four; T048 and T049 are parallel; T050
  closes the phase.
- **Phase 7** is last; T038 and T039 gate completion, and T050 and T053 must
  pass before T040. T051 and T052 are parallel with Phase 6A.
- **Phase 8** (ISS-001, T054 to T063) follows the completed Phases 1 to 7.
  T054, T055 and T056 share `graph_test.go`, so they run in order; T057 can
  run beside them. T058 needs T054, T056 and T057; T059 needs T055 and T058
  (same file). T060 and T061 can run any time after T058. T062 needs T058 and
  T059 and a fresh `make build`; T063 closes the phase.

### Within a story

Tests first and failing, then engine, then renderers, then goldens or
rendered checks. Each story ends at a checkpoint that can be reviewed alone.

## Parallel Opportunities

- Phase 2: T004 with T005.
- US1: T008 with T009.
- US2: T016 with T017.
- US3: T022, T023, T024 together; T025 can run beside T026 (different files).
- Phase 6: T032 with T033. Phase 6A: T041 with T042; T048 with T049.
  Phase 7: T035 with T036.
- Phase 8: T057 beside T054 to T056 (ingest versus stackload tests); T060
  with T061.

```text
# Example: start US1 test-writing together
T008 engine tests      -> internal/engine/blast_radius_test.go
T009 TUI golden tests  -> internal/tui/overview_golden_test.go
```

## Implementation Strategy

1. **MVP = Phases 1 to 3 (US1).** The terminal view alone answers the issue's
   question. Stop and validate with T014, T015 and a real-binary look.
2. Add US2 so the web reaches the same data (required for the feature to be
   complete under Principle VII).
3. Add US3 for accuracy while a stack is changing. The pure engine rules for
   deleting and unpriced rows land here; shipping US1 alone means the total
   is only right on a settled stack, so say so in the PR if US3 is deferred.
4. Phase 6 and 7 close the feature; the follow-up UX issue is filed last.
5. Phase 8 (ISS-001) widens the edge set after the walkthrough found the
   provider gap. It is one increment: ingest fields, the graph helper and
   the overlay, then the real-stack check on `initial-30`.
