---

description: "Task list for the shared stack loader (1733)"
---

# Tasks: Shared Stack Loader

**Input**: Design documents from `/specs/1733-shared-stack-loader/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/,
quickstart.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and written BEFORE implementation; each test task must be seen to
fail first. Coverage: 95% for `internal/stackload` and the diff changes
(critical path), 80% elsewhere. This feature changes no `internal/tui`
rendering, so no new golden files; the existing TUI goldens must stay green.

**Completeness**: Per Constitution Principle VI, every task is a full
implementation. No stubs, placeholders, or TODO comments.

**Documentation**: Per Constitution Principle IV, docs change in the same PR
as the code.

**Organization**: Tasks are grouped by user story. Stories are ordered by
priority (P1: US1, US6, US2; P2: US4, US3; P3: US5). The spec lists US6 last,
but it is P1 and its tests guard the totals the other stories feed.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an
  incomplete task)
- **[Story]**: US1 to US6, as numbered in `spec.md`
- Paths are repository-relative

## Go Test Path Conventions

Unit tests are colocated: `internal/<package>/<name>_test.go`. Black-box tests
use `package foo_test`; white-box tests use `package foo`. End-to-end command
tests use `axtest.Run` as in `internal/cli/terraform_real_state_cli_test.go`
and need `isolateConfig(t)` and `isolateFromPulumiProject(t)`. Tests that
touch process-wide state (`t.Setenv`, `t.Chdir`, the `internal/pulumi` runner)
are not parallel and carry `//nolint:paralleltest // <reason>`.

---

## Phase 1: Setup

**Purpose**: Package shell, fixtures, and a recorded v0.5.2 baseline to prove
compatibility against.

- [X] T001 Record the baseline: on the branch before any code change run
  `make test` and `make test-frontend` and note any pre-existing failures in
  the PR description, so later runs compare the failure set, not the count
- [X] T002 Create `internal/stackload/doc.go` with the package comment: what
  the package loads, that it imports only `internal/pulumi`, `internal/ingest`,
  and `internal/engine`, that it has no side effects (no history, dismissals,
  plugins, or ESC), and that backend error text never leaves it
- [X] T003 [P] Add fixtures in `internal/cli/testdata/pulumi/`:
  `stack-export-dev.json` (root stack, one provider, three unchanged custom
  resources, one resource that the preview updates, and one resource whose
  inputs hold a Pulumi secret envelope), `preview-default.json` (root stack
  `same` step plus the one `update`, with `changeSummary` reporting the
  omitted `same` count), `preview-show-sames.json` (same plan listing every
  resource), `preview-mismatch.json` (an `update` whose URN is not in the
  state and a `create` whose URN is), and `preview-create-only.json`;
  derive shapes from `test/fixtures/state/valid-state.json` and the existing
  `unchanged-stack-preview.json`
- [X] T004 [P] Capture v0.5.2 baseline goldens in
  `internal/cli/testdata/pulumi/projected-baseline-default.golden.json`,
  `projected-baseline-show-sames.golden.json`, and
  `projected-baseline-create-only.golden.json` by running the current
  `cost projected --pulumi-json <fixture> --output json` through `axtest.Run`
  with no plugins, normalizing dates as `terraform_real_state_cli_test.go`
  does; commit them before any behavior change (they prove SC-007)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The package, the descriptor origin, and the engine split that
lets state-origin descriptors be priced without moving the plan total.

**CRITICAL**: No user story starts until this phase is complete.

### Tests (write first, confirm they fail)

- [X] T005 [P] Test `ChangeSummary` parsing and its absence in older previews
  in `internal/ingest/pulumi_plan_test.go`
- [X] T006 [P] Test the origin split in `internal/engine/diff_origin_test.go`:
  plan-only descriptors give a result identical to v0.5.2; mixed origins keep
  `Entries`, `TotalAfter`, `Unchanged` over plan-origin entries and put
  state-origin entries in `StackUnchanged`; `StackTotal` counts a resource
  once when preview and state both hold it; `StackComplete=false` gives
  `StackTotal == TotalAfter`; state with no preview (all `same`) gives
  `TotalAfter == StackTotal`
- [X] T007 [P] Test that a Pulumi secret envelope in a state-origin
  descriptor's inputs never reaches the plugin request, using the existing
  fake cost client helpers, in `internal/engine/diff_secret_test.go`
- [X] T008 [P] Test `Descriptors()` and `DeployedDescriptors()` in
  `internal/stackload/descriptors_test.go` against
  `contracts/loader-api.md` checks 1 to 5: plan-only equals
  `ingest.MapResources`; deployed equals `ingest.MapStateResources`; a
  resource in both comes from the plan; state-only resources are appended in
  state order with `Operation "same"` and `OriginState`; the parity test
  between a state descriptor and a `--show-sames` descriptor for one URN
  (differences limited to `Operation`, `Origin`, and the injected `pulumi:`
  properties); returned slices are copies
- [X] T009 [P] Test mismatch and omitted-unchanged logic in
  `internal/stackload/mismatch_test.go`: non-create step absent from state,
  create already in state, empty when either side is not loaded,
  `OmittedUnchanged = max(0, same - listedSame)`, zero when `changeSummary`
  is absent
- [X] T010 [P] Test `Load`, `LoadState`, `LoadPlan`, and `WithPlan` in
  `internal/stackload/load_test.go` with a fake `internal/pulumi` runner
  (`pulumi.SetRunnerForTest`): export and preview start concurrently; a fatal
  error cancels the sibling; a state failure with a loaded plan is recorded in
  `StateStatus` and returns no error; neither side loadable returns the
  input's error; the passphrase reaches the subprocess environment and
  `os.Setenv` is never called; explicit files skip the subprocesses

### Implementation

- [X] T011 Add `ChangeSummary map[string]int` (`changeSummary`) to
  `PulumiPlan` in `internal/ingest/pulumi_plan.go`
- [X] T012 [P] Add `DescriptorOrigin` with `OriginPlan` (zero value) and
  `OriginState`, and `ResourceDescriptor.Origin` with `json:"-"`, in
  `internal/engine/types.go`
- [X] T013 Add `StackTotal`, `StackResources`, `StackComplete`,
  `StackIncompleteReason`, and `OmittedUnchanged` to `DiffSummary`, and
  `StackUnchanged []DiffEntry` to `DiffResult`, in
  `internal/engine/diff_types.go`
- [X] T014 Split pricing results by origin in `internal/engine/diff.go`:
  price every descriptor once, keep plan-origin entries in `Entries` and the
  existing summary fields, put state-origin entries in `StackUnchanged`, and
  compute the stack fields per `data-model.md` (depends on T012, T013)
- [X] T015 [P] Implement `Source`, `Status`, and the reason categories
  (`none`, `not_requested`, `backend_unreachable`, `passphrase_required`,
  `read_failed`) in `internal/stackload/source.go`
- [X] T016 Implement `Snapshot`, `WithPlan`, `Rows`, and `PendingChanges` in
  `internal/stackload/snapshot.go`, reusing `engine.MergeResourcesForOverview`,
  `engine.NewRowsFromState`, and `engine.DetectPendingChanges` (depends on
  T015)
- [X] T017 Implement `Descriptors()` and `DeployedDescriptors()` in
  `internal/stackload/descriptors.go` per `data-model.md` (depends on T012,
  T016)
- [X] T018 [P] Implement mismatch detection and the omitted-unchanged count in
  `internal/stackload/mismatch.go` (depends on T011, T016)
- [X] T019 Implement `Load`, `LoadState`, and `LoadPlan` in
  `internal/stackload/load.go` using `internal/pulumi` `StackExport` and
  `Preview`, an `errgroup` for the concurrent run, and
  `ingest.ParseStackExportWithContext` / `ParsePulumiPlanWithContext`
  (depends on T015 to T018)
- [X] T020 Checkpoint: run
  `go test -race ./internal/stackload/ ./internal/engine/ ./internal/ingest/`
  and confirm coverage of `internal/stackload` and the changed `diff.go`
  paths is at least 95%

**Checkpoint**: The package and the engine split exist and are proven. No
command uses them yet and no command output has changed.

---

## Phase 3: User Story 1 - Same resources in every command (P1) 🎯 MVP

**Goal**: `overview`, `cost projected`, `cost estimate`,
`cost recommendations`, and `cost forecast` (inventory) take their live
inventory from `stackload`; `cost actual` takes the deployed subset; the
three overview loaders become one.

**Independent Test**: Against the `stack-export-dev.json` and
`preview-default.json` fixtures, the overview rows, the `cost estimate`
no-match selector error list, and the descriptors `cost recommendations`
requests all name the same resources (the live inventory). `cost projected`
carries the same set in its stack total once US6 renders it; until then its
output is byte-identical to v0.5.2.

### Tests for User Story 1 (write first, confirm they fail)

- [X] T021 [P] [US1] Test that the plain, terminal, and web overview loaders
  produce rows identical to today's for the fixtures, in
  `internal/cli/overview_loader_parity_test.go`
- [X] T022 [P] [US1] Test that the projected family sees the live inventory
  (plan resources plus unchanged state resources) in
  `internal/cli/stack_membership_test.go`, comparing the descriptors
  `loadProjectedResources` returns with the overview rows' URNs
- [X] T023 [P] [US1] Test that a `cost estimate` selector matching nothing
  exits 2 and lists the live inventory in
  `internal/cli/cost_estimate_membership_test.go`
- [X] T024 [P] [US1] Test that `cost recommendations` requests recommendations
  for the live inventory (fake plugin records the resource IDs it receives) in
  `internal/cli/cost_recommendations_membership_test.go`
- [X] T025 [P] [US1] Test that `cost actual` uses the deployed subset, omits a
  plan-only create, and still reports a resource deleted earlier in the
  billing period from the history store (SC-009), in
  `internal/cli/cost_actual_history_test.go`
- [X] T026 [P] [US1] Test that `cost forecast` takes its inventory from the
  snapshot and its total equals the v0.5.2 value (plan-origin resources only)
  in `internal/cli/cost_forecast_total_test.go`

### Implementation for User Story 1

- [X] T027 [US1] Add the CLI adapter `internal/cli/stack_source.go`: build a
  `stackload.Source` from flags (`--stack`, `--pulumi-json`, `--pulumi-state`,
  passphrase), resolving the project with `detectPulumiProject`, and keep ESC
  context, history recording, and dismissals in `internal/cli`
- [X] T028 [US1] In `internal/cli/cost_projected.go`, make
  `loadProjectedResources` and `loadDetectedProjected` call `stackload`:
  `Descriptors()` for a project, `LoadPlan` for `--pulumi-json` with no
  project, `LoadState` for `--pulumi-state` alone (state resources are
  `same`, not `create`); leave the Terraform branches untouched (depends on
  T027)
- [X] T029 [US1] In `internal/cli/cost_forecast.go`, keep the inventory from
  the snapshot but compute the forecast total over `OriginPlan` descriptors
  only (research R14); confirm `internal/cli/cost_estimate.go` and
  `internal/cli/cost_recommendations.go` need no further change beyond the
  shared `loadProjectedResources` and adjust them if they do (depends on T028)
- [X] T030 [US1] In `internal/cli/cost_actual.go` and
  `internal/cli/common_execution.go`, replace the export branch of
  `resolveResourcesFromPulumi` with `stackload.LoadState` and
  `DeployedDescriptors()`; keep `enrichWithHistoricalResources` and
  `recordDescriptorHistory` unchanged (depends on T027)
- [X] T031 [US1] In `internal/cli/overview.go`, delete `exportStateFromProject`,
  `exportStateForStack`, `loadOverviewProjectData`, and `resolveOverviewPlan`,
  and call `stackload`; keep the capture hooks for now (removed in US3)
  (depends on T027)
- [X] T032 [US1] In `internal/cli/overview_pipeline.go`, make
  `plainOverviewLoader`, `tuiOverviewLoader`, and `autoDetectOverviewLoader`
  build rows with `Snapshot.Rows` and `PendingChanges`, keeping history
  recording and filters in place (depends on T031)
- [X] T033 [US1] Remove code made dead by T028 to T032 (including the preview
  branch of `resolveResourcesFromPulumi`), update tests that named the removed
  helpers, and run `go vet ./...`

**Checkpoint**: US1 is complete when T021 to T026 pass and the independent
test above holds.

---

## Phase 4: User Story 6 - Existing totals keep their meaning (P1)

**Goal**: `cost projected` keeps its plan total, budgets, and exit codes
exactly, and adds the stack total, the unchanged list, and the table section.

**Independent Test**: Run `cost projected` on `preview-default.json` and on
`preview-show-sames.json`, each with and without readable state, and compare
`summary.totalMonthly`, `diff.totalAfter`, and the exit code against the T004
goldens. They match; `stack.totalMonthly` is the only new figure.

### Tests for User Story 6 (write first, confirm they fail)

- [X] T034 [P] [US6] Test that plan-only runs are byte-identical to the T004
  goldens in `internal/cli/cost_projected_baseline_test.go`
- [X] T035 [P] [US6] Test the four acceptance scenarios in
  `internal/cli/cost_projected_totals_test.go`: default preview plus state;
  saved preview listing unchanged resources plus state (each counted once);
  a budget threshold between the plan total and the stack total gives the
  baseline exit code; unreadable state gives `stack.complete: false` and
  `stack.totalMonthly` equal to the plan total
- [X] T036 [P] [US6] Test the output contract in
  `internal/engine/diff_render_test.go`: the `stack` object and its keys, no
  `stack` key for Terraform sources, the `UNCHANGED RESOURCES` table section
  and stack-total line, and no new NDJSON lines

### Implementation for User Story 6

- [X] T037 [US6] Render the `stack` object (`totalMonthly`, `currency`,
  `resources`, `complete`, `incompleteReason`, `omittedUnchanged`,
  `unchanged[]`) in `renderDiffJSON` and the unchanged section and stack-total
  line in the table renderer, in `internal/engine/diff_render.go`
- [X] T038 [US6] In `internal/cli/cost_projected.go`, keep budget evaluation,
  `summary`, and the audit total on `diff.Summary.TotalAfter` and
  `diff.AfterCosts()` (plan-origin entries), and pass the stack fields to the
  renderer (depends on T037)
- [X] T039 [US6] Document the plan total, the stack total, and the new JSON
  keys in `docs/src/content/docs/reference/cli-commands.md` under
  `cost projected`

**Checkpoint**: US1 and US6 together fix the user-visible defect without
changing any existing total. This is the suggested MVP.

---

## Phase 5: User Story 2 - Be told when nothing is changing (Priority: P1)

**Goal**: An unchanged Pulumi stack gets the Terraform sentence in table form
and a valid JSON document with `emptyPlan: true`, even when state is readable.

**Independent Test**: `cost projected` on `stack-export-dev.json` with a
preview holding only the root stack prints the no-change sentence and the
stack total (table) and `emptyPlan: true` (JSON).

### Tests for User Story 2 (write first, confirm they fail)

- [X] T040 [P] [US2] Test in `internal/cli/cost_projected_empty_stack_test.go`
  (beside the v0.5.2 `cost_projected_empty_plan_test.go`):
  table output prints `No changes. Projected monthly cost is zero.` and the
  stack total line for a complete stack; JSON and NDJSON stay valid;
  `emptyPlan` is true when only state-origin resources exist; a Terraform
  project with no changes still prints its own result unchanged (SC-006)

### Implementation for User Story 2

- [X] T041 [US2] In `internal/cli/cost_projected.go`, evaluate
  `warnIfPlanEmpty` over `OriginPlan` descriptors only, print the no-change
  sentence plus the stack total for table output on a complete stack, and keep
  the v0.5.2 stderr warning otherwise (contracts/cli-output.md)

**Checkpoint**: US2 holds for Pulumi and the Terraform result is untouched.

---

## Phase 6: User Story 4 - A saved plan file says what it omits (Priority: P2)

**Goal**: `--pulumi-json` with no state reports how many unchanged resources
the file omitted.

**Independent Test**: `cost projected --pulumi-json preview-default.json`
prints the omitted count and sets `stack.omittedUnchanged`;
`preview-show-sames.json` prints no such note.

### Tests for User Story 4 (write first, confirm they fail)

- [X] T042 [P] [US4] Test the `not_requested` notice text, the
  `stack.omittedUnchanged` key, the absent-note case for
  `preview-show-sames.json`, and a preview with no `changeSummary`, in
  `internal/cli/cost_projected_totals_test.go`

### Implementation for User Story 4

- [X] T043 [US4] Wire `Snapshot.OmittedUnchanged` into
  `DiffSummary.OmittedUnchanged` and print the `Note:` line from
  `contracts/cli-output.md` for `not_requested` runs in
  `internal/cli/cost_projected.go`

**Checkpoint**: US4 holds for the plan-file-only path used by CI and the
GitHub Action.

---

## Phase 7: User Story 3 - Terminal and web cannot disagree (P2)

**Goal**: `webData` holds one snapshot; the Estimate, Recommendations, and
Cost callbacks derive their resources from it; the capture hooks are gone.

**Independent Test**: For the fixtures, the resources `webData.estimateResources`
and `webData.recommendations` use equal those `cost estimate` and
`cost recommendations` use, and `webData.actualCosts` uses the deployed subset.

### Tests for User Story 3 (write first, confirm they fail)

- [X] T044 [P] [US3] Test in `internal/cli/web_data_snapshot_test.go` that
  `estimateResources` equals the CLI live inventory, `recommendations` gains
  plan-only creates, `actualCosts` keeps the history enrichment, and an
  on-demand preview replaces the snapshot through `WithPlan`

### Implementation for User Story 3

- [X] T045 [US3] In `internal/cli/web_data.go`, store one `*stackload.Snapshot`
  and rewrite `loadResources`, `estimateResources`, and `recommendations` to
  read it; remove `captureState`, `capturePlan`, `captureTerraform`, `raw`,
  `planned`, and `planSteps`, and rewrite `appendPreviewRows` to use
  `Snapshot.WithPlan` and the engine's `AppendPlanOnlyRows` as today
- [X] T046 [US3] In `internal/cli/overview.go` and `internal/cli/web.go`,
  remove the `captureState`, `capturePlan`, and `captureTerraform` parameters
  and fields from `overviewParams`, and wire the snapshot from the overview
  pipeline's result (depends on T045)
- [X] T047 [US3] Run `go test ./internal/webui/... ./internal/cli/...` and
  `make test-frontend`; the web handler and view tests must pass unchanged

**Checkpoint**: US3 holds; `internal/webui` has no diff.

---

## Phase 8: User Story 5 - A stack that cannot be read is reported (Priority: P3)

**Goal**: A failed state read degrades to the preview with a category, never
raw backend text.

**Independent Test**: With the fake runner failing the export, `cost projected
--pulumi-json` plus a project still prices the preview resources, sets
`stack.complete: false` and the reason, and prints no URL or backend name.

### Tests for User Story 5 (write first, confirm they fail)

- [X] T048 [P] [US5] Test error classification and redaction in
  `internal/stackload/status_test.go`: a passphrase error maps to
  `passphrase_required`, a network or login error to `backend_unreachable`,
  anything else to `read_failed`, and no category or notice carries a URL, a
  backend name, a token, or the raw error
- [X] T049 [P] [US5] Test the notice text and the JSON fields end to end in
  `internal/cli/cost_projected_totals_test.go`

### Implementation for User Story 5

- [X] T050 [US5] Implement the classification in
  `internal/stackload/status.go` and use it in `Load` (depends on T019)
- [X] T051 [US5] Print the `Warning: stack state unavailable (<reason>)` notice
  and set `stack.complete` and `stack.incompleteReason` in
  `internal/cli/cost_projected.go`; print the mismatch notice from the
  snapshot's `Mismatches` (depends on T050)

**Checkpoint**: All six stories hold.

---

## Phase 9: Web Parity (Constitution Principle VII)

**Purpose**: Prove the web shows what the CLI shows. This feature adds no
route and no payload key, so the hook is the existing callbacks.

- [X] T052 [P] Add a parity test in `internal/cli/web_stack_parity_test.go`
  asserting that the resource sets behind `/api/estimate/resources` and
  `/api/recommendations/query` (through `webData.estimateResources` and
  `webData.recommendations`) equal the sets the CLI commands use for the same
  fixtures
- [X] T053 Confirm `internal/webui` is unchanged (`git diff --stat main --
  internal/webui` is empty) and update
  `specs/1733-shared-stack-loader/contracts/http-api.md` if any Go method name
  changed during implementation
- [X] T054 Render the Overview, Estimate, Recommendations, and Cost views with
  Playwright against `initial-30` `dev` (`finfocus --web --no-browser --port
  <port>`, empty passphrase, Pulumi Cloud) before and after the change,
  compare them with the CLI, and record the stack and date under Render
  evidence in `specs/1733-shared-stack-loader/spec.md`
- [X] T055 [P] Update `docs/src/content/docs/reference/web-ui.md` to say the
  Estimate and Recommendations views list the live inventory
- [X] T056 Final task: file one `web-ui` follow-up issue for layout, copy, and
  accessibility findings from T054 only, citing the Web UI Fit section of
  `specs/1733-shared-stack-loader/spec.md`; engine, API, redaction, or
  data-shape findings belong to this feature (filed as #1905)

---

## Phase 10: Polish & Cross-Cutting Concerns

- [X] T057 [P] Update `docs/src/content/docs/commands/overview.md` to say the
  overview, `cost projected`, `estimate`, and `recommendations` share one
  loading step
- [X] T058 [P] Update `agent-skills/cost-check/SKILL.md` and
  `agent-skills/cost-check/references/failure-modes.md` so the `empty-plan`
  rule keys off `finfocus.emptyPlan` and mentions `finfocus.stack`, rebuild the
  `.skill` zip (`SKILL.md` plus `references/`), and run
  `go test ./internal/cli -run Skills`
- [X] T059 [P] Add `internal/stackload/` to the architecture table in
  `CLAUDE.md` and one Recent Changes line for `1733-shared-stack-loader`,
  edited by hand (do not run `update-agent-context.sh`; it corrupts the file)
- [X] T060 [P] Confirm godoc on every exported symbol in `internal/stackload`
  and on the new `engine` symbols
- [X] T061 Run coverage and enforce 95% for `internal/stackload` and the
  changed `internal/engine/diff*.go` paths, 80% for the rest of the diff
- [X] T062 Run `go test -race -shuffle=on -count=3 ./internal/stackload/...
  ./internal/engine/... ./internal/cli/...` for parallel-safety
- [X] T063 Run `make lint`, `make test`, `make validate`, `make docs-lint`,
  `make test-frontend`, and `make test-integration`; compare the integration
  failure set with the T001 baseline
- [X] T064 Measure timing against SC-005: `time finfocus cost projected --stack
  dev` on `initial-30` `dev` before and after; the difference must be within 3
  seconds (baseline: preview 11.5 to 13.6 s, export about 0.8 s, parallel
  10.2 s)
- [X] T065 Run `quickstart.md` scenarios 1 to 7 and record the results in
  `specs/1733-shared-stack-loader/validation.md`
- [X] T066 File two follow-up issues from `research.md` Open decisions: whether
  `cost forecast` should include unchanged resources, and Terraform's no-change
  sentence appearing in JSON output (use `/create-issue`; research the code
  first) (filed as #1903 and #1904)
- [X] T067 Set the spec status to Implemented, keep the generated changelog
  untouched (release-please owns `CHANGELOG.md`), and put the behavior change
  (recommendations and estimate cover the live inventory) in the PR
  description

### Review fixes (after `/speckit.analyze`, 2026-10-26)

- [X] T068 Keep pricing failures of state-origin resources out of
  `DiffResult.Errors` (`finfocus.errors`, the table ERRORS block) in
  `internal/engine/diff.go`; carry them in `DiffResult.StackErrors` and list
  them in the table only with the stack section. Test:
  `TestProjectedDiffStateErrorsStayOutOfPlanErrors` in
  `internal/engine/diff_origin_test.go`
- [X] T069 Return an explicit `--pulumi-state` file that cannot be read as an
  error from `stackload.Load`, cancelling the preview, instead of degrading to
  the plan. Test: the explicit-file cases in `TestLoadFiles`
- [X] T070 Add `(*Snapshot).ActualDescriptors` and use it for
  `cost actual --pulumi-json` and the web Cost view, so a saved preview keeps
  its v0.5.2 inventory without state and CLI and web agree with it. Tests:
  `TestActualDescriptors`, `TestActualSavedPreviewUsesSharedInventory`
- [X] T071 Keep Pulumi's message on a failed preview (`stackload.Error`
  unwraps it), as in v0.5.2; state-export failures stay categories. Test:
  the `fatal` case in `TestLoadDegradedAndFatalCancellation`
- [X] T072 Let `cost recommendations` load a detected Pulumi project with no
  input flag (`validateRecommendationsInputs`), as the docs and loader already
  allowed; fix the help text and `guides/recommendations.md`. Tests:
  `TestRecommendationsInputsAcceptDetectedPulumiProject`,
  `TestRecommendationsInputsRequireSourceOutsideProject`
- [X] T073 Record deployed component URNs on the snapshot so a preview step
  for a component resource is not reported as a mismatch (found in the T065
  run). Test: `TestLoadComponentStepIsNotMismatch`

### PR #1909 review fixes (2026-10-27)

- [X] T074 Price a saved preview of a different stack or project on its own
  (`not_requested`) instead of merging the surrounding project's state, by
  comparing URN stack and project in `stackload.Load`. An explicit state and
  preview pair is merged as given. Tests:
  `TestLoadProjectStateOfAnotherStackIsNotMerged`, the `other stack preview`
  case in `TestProjectedStackTotalsAndAvailability`
- [X] T075 Reprice plan resources whose references resolve differently against
  the full inventory, for the stack total only, so it does not depend on
  `--show-sames`; plan requests and cache keys stay as before. Test:
  `TestProjectedStackTotalUsesFullInventoryReferences`
- [X] T076 Accept `cost projected --pulumi-json` with `--pulumi-state`, as the
  docs state, so an unreadable explicit state file is an input error (exit 2).
  Test: `TestProjectedExplicitStateAndPreviewPair`
- [X] T077 Explain an unchanged plan with a complete stack total on stderr for
  NDJSON (`engine.NoChangesStackSummary`). Test:
  `TestProjectedUnchangedStackFormats`
- [X] T078 Return the caller's cancellation from `stackload.Load` instead of
  degrading to the preview. Test: `TestLoadCallerCancellationAfterSavedPreview`
- [X] T079 Render an empty daily or monthly `cost actual` result as empty, not
  an internal error, matching the web Cost view (nothing deployed yet); give
  the web E2E Cost-view tests a deployed state. Tests:
  `TestRenderActualCostOutputEmptyTimeGrouping`, `make test-e2e-web`
- [X] T080 Skip state-origin resources in `cost forecast` pricing; use
  `--stack` for the initial terminal overview preview. Test:
  `TestOverviewPipeline_TUIInitialPreviewHonorsStackFlag`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (1)**: none. T003 and T004 can run together after T002.
- **Foundational (2)**: after Setup. Blocks every story. Tests T005 to T010
  first, then T011 to T019, then the T020 checkpoint.
- **US1 (3)**: after Foundational. No dependency on other stories.
- **US6 (4)**: after US1 (it renders what US1 feeds through the diff) and
  after T004.
- **US2 (5)**: after US6 (it reuses the stack total line).
- **US4 (6)**: after US6 (it fills `stack.omittedUnchanged`).
- **US3 (7)**: after US1 (it replaces the capture hooks that US1 leaves).
- **US5 (8)**: after US6 and Foundational T019.
- **Web Parity (9)**: after US1 and US3.
- **Polish (10)**: after the stories it documents.

### Within Each Story

Tests first and failing, then implementation. Tasks that edit the same file
(`internal/cli/cost_projected.go`: T028, T038, T041, T043, T051) are
sequential in that order.

### Parallel Opportunities

- Foundational tests T005 to T010 touch different files and run together;
  so do T012, T015, and T018 once their inputs exist.
- US1 tests T021 to T026 are all in different files and run together.
- US6 tests T034 to T036 run together.
- After Foundational, US1 implementation tasks on different files
  (T029, T030, T031) can run together once T027 exists.
- Docs tasks T039, T055, T057, T058, T059, and T060 run together.

---

## Parallel Example: User Story 1

```bash
# Write all US1 tests together (different files):
Task: "Overview loader parity test in internal/cli/overview_loader_parity_test.go"
Task: "Projected-family membership test in internal/cli/stack_membership_test.go"
Task: "Estimate selector test in internal/cli/cost_estimate_membership_test.go"
Task: "Recommendations membership test in internal/cli/cost_recommendations_membership_test.go"
Task: "Actual-cost history test in internal/cli/cost_actual_history_test.go"
Task: "Forecast total test in internal/cli/cost_forecast_total_test.go"

# After T027, run these implementation tasks together:
Task: "cost forecast plan-origin total in internal/cli/cost_forecast.go"
Task: "cost actual deployed subset in internal/cli/cost_actual.go"
Task: "overview helpers in internal/cli/overview.go"
```

---

## Implementation Strategy

### MVP First (User Story 1, then User Story 6)

1. Complete Phase 1 (Setup) and Phase 2 (Foundational).
2. Complete Phase 3 (US1) and Phase 4 (US6) together: US1 makes the commands
   agree, US6 makes `cost projected` show the stack and proves no existing
   total moved.
3. **Stop and validate** with quickstart scenarios 1 and 2 on `initial-30`
   `dev`: 61 resources everywhere, `plan` unchanged, `stack` larger.
4. Ship if ready; the rest are increments.

### Incremental Delivery

1. Setup + Foundational: nothing user-visible changes.
2. US1 + US6: the defect is fixed (MVP).
3. US2: the unchanged-stack sentence.
4. US4: the saved-plan note.
5. US3: the web reads the shared snapshot.
6. US5: unreadable-state reporting.
7. Web Parity and Polish.

Each increment keeps `make test` and `make lint` green and leaves the plan
total, budgets, and exit codes as v0.5.2.

---

## Notes

- [P] tasks touch different files and have no incomplete dependency.
- [Story] labels map each task to a user story in `spec.md`.
- Confirm each test fails before implementing.
- Commit after each task or logical group, on a branch off `main` (main is
  protected).
- Do not run `.specify/scripts/bash/update-agent-context.sh`; it deletes
  existing `CLAUDE.md` content.
- Avoid: edits to `internal/webui` (this feature changes none), changes to
  `.golangci.yml`, hand edits to `CHANGELOG.md`.
