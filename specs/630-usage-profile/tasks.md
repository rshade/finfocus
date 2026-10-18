# Tasks: Usage Profile

**Input**: [plan.md](plan.md), [spec.md](spec.md)
**Prerequisites**: plan.md, data-model.md, contracts/usage-profile.md, research.md

## Phase 1: Setup

- [x] T001 Confirm finfocus-spec v0.7.6 already has `UsageProfile` on `GetProjectedCostRequest` and `GetRecommendationsRequest` and that `BatchCostRequest` does not

## Phase 2: Foundational

- [x] T002 Parse profile labels in `internal/config/profile.go` and reject unknown or empty values from `cost.profile` in `internal/config/budget.go`
- [x] T003 Test parsing and config validation in `internal/config/profile_test.go`

## Phase 3: User Story 1 - Development projection (Priority: P1)

- [x] T004 [US1] Carry `usage_profile` on the internal projected and recommendation requests in `internal/proto/adapter.go` and test it in `internal/proto/adapter_test.go`
- [x] T005 [US1] Set the profile from `Engine.WithUsageProfile` on unary projected and recommendation RPCs, skip projected batching when it is set, and suffix cache keys in `internal/engine/engine.go`
- [x] T006 [US1] Test the RPC field, the batch skip, the cache suffix, and the actual-cost hourly lookup in `internal/engine/profile_test.go`

## Phase 4: User Story 2 - Saved default (Priority: P1)

- [x] T007 [US2] Resolve `--profile`, then `FINFOCUS_PROFILE`, then `cost.profile` in `internal/cli/usage_profile.go`, and add the persistent `cost` flag plus the `overview` flag in `internal/cli/root.go` and `internal/cli/overview.go`
- [x] T008 [US2] Apply the resolved profile on projected, recommendations, forecast, estimate, cluster, overview, the dashboard engine, and `analyzer serve` (`internal/cli/cost_projected.go`, `cost_recommendations.go`, `cost_forecast.go`, `cost_estimate.go`, `cost_cluster.go`, `overview_pipeline.go`, `web_data.go`, `analyzer_serve.go`)
- [x] T009 [US2] Test resolution order and the exit-2 invalid value in `internal/cli/usage_profile_test.go`

## Phase 5: User Story 3 - Show the profile (Priority: P2)

- [x] T010 [US3] Print `Profile:` and add `profile` to projected and recommendation JSON in `internal/engine/diff_render.go` and `internal/cli/cost_recommendations.go`
- [x] T011 [US3] Test the table line and both JSON documents in `internal/engine/diff_render_test.go` and `internal/cli/cost_recommendations_test.go`

## Phase 6: User Story 4 - Measured spend (Priority: P2)

- [x] T012 [US4] Leave `cost actual` and `cost history collect` on an unspecified engine, and cover that with the engine test from T006

## Phase 7: Polish

- [x] T013 Document the flag, `FINFOCUS_PROFILE`, and `cost.profile` in `docs/src/content/docs/reference/config-reference.md`, `docs/src/content/docs/commands/overview.md`, `docs/src/content/docs/commands/forecast.md`, and `ROADMAP.md`
- [x] T014 Update `internal/cli/testdata/mcp/tools.golden` if the new flag changes the MCP tool schema
- [x] T015 Run `make validate`, `make test`, `make lint`, and `make docs-lint`

## Dependencies

T002 before T007. T004 before T005. T005 before T008 and T010. T012 is the
decision not to call `WithUsageProfile` from actual and history. Docs after
the flag exists.

## Parallel

T003 can be written with T002. T006 can be written with T005. T011 can be
written with T010.

## MVP

User Story 1: a `--profile=dev` projected run sends `USAGE_PROFILE_DEV` and
does not reuse a `prod` cache entry.
