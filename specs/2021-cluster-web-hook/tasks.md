# Tasks: Cluster Allocation Web Hook

**Input**: Design documents from `specs/2021-cluster-web-hook/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http-api.md

**Tests**: Mandatory under Constitution Principle II; each test task comes
before the code it covers. Go tests are colocated and use testify.

**Organization**: Grouped by user story (spec.md). US1 and US2 are both P1.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Record the `cost cluster` baseline: run `go test ./internal/cli/ -run 'CostCluster|Cluster'` and keep the pass list for SC-003

## Phase 2: Foundational (blocking)

- [X] T002 [P] Test `engine.NewClusterReport` (idle omitted when scoped, capacity never nil, fields copied) and the JSON tag set in internal/engine/cluster_report_test.go
- [X] T003 [P] Test `engine.ClusterInputError` (message is the inner error's, `errors.As` and `Unwrap`) and `engine.ClusterPluginMissingError` (message equals the CLI's install hint, capability field) in internal/engine/cluster_errors_test.go
- [X] T004 Move `clusterOutput`, `clusterPolicyOutput`, `newClusterOutput`, `capacityOrEmpty` to `engine.ClusterReport`, `engine.ClusterPolicy`, `engine.NewClusterReport` in internal/engine/cluster_report.go and update internal/cli/cost_cluster_render.go and its tests to use them
- [X] T005 Add `engine.ClusterInputError` and `engine.ClusterPluginMissingError` in internal/engine/cluster_errors.go
- [X] T006 [P] Test `config.ParseAllocationPolicy` (valid text, `null`, empty text, syntax error naming the source and position, size limit) in internal/config/allocation_policy_test.go
- [X] T007 Add `config.ParseAllocationPolicy(ctx, io.Reader, source)` sharing parse, size limit, and error rewriting with `readAllocationPolicy` in internal/config/allocation_policy.go
- [X] T008 [P] Test `viewmodel.ClusterModeLine`, `viewmodel.SummarizeClusterWarnings`, `viewmodel.ClusterIncompleteLines`, and `viewmodel.BuildClusterReportDisplay` (money, shares, scoped idle, zero total) in internal/viewmodel/cluster_report_display_test.go
- [X] T009 Move the warning grouping and incomplete-footer text from internal/cli/cost_cluster_warnings.go into internal/viewmodel/cluster_report_display.go and add `BuildClusterReportDisplay`; the CLI keeps only stderr printing and table rendering, output unchanged

**Checkpoint**: `go test ./internal/engine/ ./internal/config/ ./internal/viewmodel/ ./internal/cli/ -run 'Cluster|Policy'` passes and existing cluster render tests pass unchanged.

## Phase 3: User Story 1 - Query cluster allocation (P1) MVP

**Goal**: the session builds the CLI's report for a dashboard query.

**Independent Test**: parity test comparing CLI JSON with the endpoint body.

- [X] T010 [US1] Test `parseClusterQuery` (defaults, selectors, window, file policy, text policy) and `buildClusterReport` with in-process `bufconn` usage-source and allocator fakes and a fake pricer (run-rate, window, namespace-scoped) in internal/cli/cost_cluster_build_test.go
- [X] T011 [US1] Add `clusterQuery`, `clusterInput`, `parseClusterQuery`, and `buildClusterReport(ctx, clients, pricer, input)` in internal/cli/cost_cluster_build.go; rewrite `runCostCluster` in internal/cli/cost_cluster.go over them with `clusterCommandError` mapping `ClusterInputError` to `toValidationError`
- [X] T012 [P] [US1] Test `handleClusterQuery` (ok body has `status` and `display`, not-ready 503, generic failure 502 with no plugin text, bad JSON 400, and a body carrying a `policyPath` field is rejected without reading a file) in internal/webui/handlers_cluster_test.go
- [X] T013 [US1] Add `ClusterQuery`, `ClusterResponse`, `SessionOptions.Cluster`, `registerCluster`, and `POST /api/cluster/query` calling `SessionOptions.Cluster` and `viewmodel.BuildClusterReportDisplay` in internal/webui/handlers_cluster.go, internal/webui/session.go, internal/webui/server.go
- [X] T014 [US1] Add `(*webData).cluster` (query gate through `queryEngine`, `d.clients`, `buildClusterReport`) in internal/cli/web_cluster.go and wire `Cluster` in `webSessionOptions` in internal/cli/web.go
- [X] T015 [US1] Parity test: same fakes, `renderClusterResult` JSON versus the `/api/cluster/query` body minus `status` and `display`, for run-rate, window, selector-scoped, and incomplete (usage warning plus an unpriced node) requests, in internal/cli/web_cluster_test.go

## Phase 4: User Story 2 - Unavailable answer (P1)

**Goal**: a missing capability answers 200 `status: "unavailable"`.

**Independent Test**: query with each capability missing.

- [X] T016 [US2] Test that `selectCapablePlugin` returns `*engine.ClusterPluginMissingError` with the unchanged message and that `TestCostCluster_NoPluginsInstalled` still exits 1, in internal/cli/cost_cluster_test.go
- [X] T017 [US2] Return `*engine.ClusterPluginMissingError` from `selectCapablePlugin` in internal/cli/cost_cluster.go
- [X] T018 [US2] Map `ClusterPluginMissingError` to the unavailable body (`code: cluster_plugin_missing`, `capability`, `error`) and test both capabilities in internal/webui/handlers_cluster.go and internal/webui/handlers_cluster_test.go

## Phase 5: User Story 3 - CLI error text for bad input (P2)

**Goal**: every exit-2 input returns 400 with the CLI's message.

**Independent Test**: each invalid input against the CLI message.

- [X] T019 [US3] Test that group-by, selector, window, plugin choice, policy syntax, policy rejection, usage-request rejection, and ambiguous source errors are `ClusterInputError` with the CLI's text, and that `TestCostCluster_*` exit codes are unchanged, in internal/cli/cost_cluster_build_test.go
- [X] T020 [US3] Wrap every user-input failure in `parseClusterQuery` and `buildClusterReport` with `engine.ClusterInputError`, keeping the existing messages, in internal/cli/cost_cluster_build.go
- [X] T021 [US3] Map `ClusterInputError` to 400 `invalid_cluster_query` with its message and test it in internal/webui/handlers_cluster.go and internal/webui/handlers_cluster_test.go

## Phase 6: User Story 4 - Effective policy (P3)

**Goal**: `--show-policy` from the dashboard.

**Independent Test**: policy with no text and with text versus the CLI.

- [X] T022 [US4] Test `buildClusterPolicy` (built-in defaults, text source `request body`, allocator only) in internal/cli/cost_cluster_build_test.go
- [X] T023 [US4] Add `buildClusterPolicy(ctx, clients, input)` in internal/cli/cost_cluster_build.go and use it for `--show-policy` in internal/cli/cost_cluster.go
- [X] T024 [US4] Add `ClusterPolicyQuery`, `ClusterPolicyResponse`, `SessionOptions.ClusterPolicy`, and `POST /api/cluster/policy` with tests in internal/webui/handlers_cluster.go and internal/webui/handlers_cluster_test.go
- [X] T025 [US4] Add `(*webData).clusterPolicy` in internal/cli/web_cluster.go, wire it in internal/cli/web.go, and add a policy parity case in internal/cli/web_cluster_test.go

## Phase 7: Web Parity (Principle VII)

- [X] T026 Confirm contracts/http-api.md names every Go method the handlers call (`cli.parseClusterQuery`, `cli.buildClusterReport`, `cli.buildClusterPolicy`, `engine.RunClusterAllocation`, `engine.GroupClusterRows`, `engine.ShowAllocationPolicy`, `engine.NewClusterReport`, `viewmodel.BuildClusterReportDisplay`) in specs/2021-cluster-web-hook/contracts/http-api.md
- [X] T027 Add both endpoints and their errors to the API table, and a note that the Cluster view is pending, in docs/src/content/docs/reference/web-ui.md
- [X] T028 Render the nearest existing view (Overview with cluster expansion) with Playwright against a populated stack, call `/api/cluster/query` from that page, compare with the Web UI Fit section; when a kind or opencost cluster with the kubernetes plugin is reachable, also compare the endpoint with `cost cluster --output json` there; record the result, including whether a live cluster was available, in specs/2021-cluster-web-hook/quickstart.md and spec.md
- [X] T029 Run `make test-e2e-web`
- [X] T030 After the PR opens, file one UX-only `web-ui` follow-up issue for the Cluster view (#2280) (layout, rendering, interaction, copy, accessibility; no engine, API, redaction, or data-shape work)

## Phase 8: Polish

- [X] T031 [P] Add cluster hook gotchas (shared builder, typed errors, no page paths) to CLAUDE.md
- [X] T032 Run `make validate`, `make test`, `make test-race`, `make lint`, `make docs-lint`; set spec status to Implemented

## Dependencies

- Phase 2 blocks every story. US1 (T010-T015) comes before US2-US4,
  which reuse its builder, handler, and wiring. US2, US3, and US4 are
  independent of each other after US1.
- Within a story: test task, then implementation, then wiring.

## Parallel Opportunities

- T002, T003, T006, T008 (different packages).
- T012 alongside T010-T011 (webui versus cli).
- After US1: US2, US3, US4 touch the same files, so run them in sequence.

## Implementation Strategy

MVP is Phases 1-3: the query endpoint with parity. US2 is the next most
valuable because most installations lack cluster plugins. Then US3, US4,
the web parity phase, and polish.
