# Tasks: Recommendation Lifecycle History on the Web

**Input**: Design documents from `/specs/1921-recommendation-history-web/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http-api.md

**Tests**: Principle II requires tests before implementation.

## Phase 1: User Story 1 - Same lifecycle as the command line (Priority: P1)

**Goal**: The dashboard returns `GetRecommendationHistory` events for a listed id.

**Independent Test**: Events and order match a direct engine call on the query engine.

- [x] T001 [US1] Return plugin-failure warnings from `GetRecommendationHistory` without changing `mergeRecommendationHistory` in `internal/engine/recommendation_history.go`
- [x] T002 [US1] Keep CLI history output unchanged in `internal/cli/cost_recommendations_history.go`
- [x] T003 [US1] Add `SessionOptions.RecommendationHistory` and `GET /api/recommendations/history` in `internal/webui/session.go`, `internal/webui/handlers_recommendations.go`, and `internal/webui/server.go`
- [x] T004 [US1] Wire `(*webData).recommendationHistory` through `queryEngine` and `loadDismissalStore` in `internal/cli/web_data.go` and `internal/cli/web.go`
- [x] T005 [US1] Handler and wiring tests in `internal/webui/handlers_recommendations_test.go` and `internal/cli/web_recommendation_history_test.go`

## Phase 2: User Story 2 - Refuse unknown ids (Priority: P1)

**Goal**: History uses the presented-recommendation cache.

**Independent Test**: Missing id is 400. An id outside the cache is 404 and the history callback is not called.

- [x] T006 [US2] Resolve id with the item endpoint's match before calling history in `internal/webui/handlers_recommendations.go`
- [x] T007 [US2] Cover 400, 404, and the one-time list fallback in `internal/webui/handlers_recommendations_test.go`

## Phase 3: User Story 3 - Warnings (Priority: P2)

**Goal**: A failing plugin keeps local events and adds one fixed warning.

**Independent Test**: Engine and wiring tests show the local event, one warning, and no plugin error text.

- [x] T008 [US3] Assert warnings for a failing plugin and none for `Unimplemented` in `internal/engine/recommendation_history_test.go`
- [x] T009 [US3] Assert the HTTP body keeps events and the fixed warning, and a lookup error omits the Go error, in `internal/webui/handlers_recommendations_test.go`

## Phase 4: Web Parity

- [x] T010 Document `GET /api/recommendations/history` in `docs/src/content/docs/reference/web-ui.md` and the contract in `specs/1921-recommendation-history-web/contracts/http-api.md`
- [x] T011 Record the Recommendations detail render in the spec's Web UI Fit section
- [ ] T012 File one UX-only `web-ui` issue for the detail-panel history table after the PR is open

## Phase 5: Verification

- [x] T013 `make validate`, `make test`, `make lint`, and the history-focused tests
- [x] T014 `make test-e2e-web` because the recommendations route surface changed
