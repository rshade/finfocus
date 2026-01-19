# Tasks: Update documentation for E2E testing and plugin ecosystem

**Input**: Design documents from `/specs/118-e2e-plugin-docs/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, quickstart.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).
*Note*: For documentation-only features, TDD applies to verification steps (e.g., linting, checking links).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV, documentation (README, docs/) MUST be updated concurrently with implementation to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Create documentation directory structure (`docs/testing`, `docs/architecture`, `docs/guides`, `docs/reference`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

*No blocking foundational tasks for this documentation feature.*

---

## Phase 3: User Story 1 - E2E Testing Setup and Execution (Priority: P1) 🎯 MVP

**Goal**: Developers and CI engineers have a clear guide to set up and run E2E tests.

**Independent Test**: Verify `docs/testing/e2e-guide.md` exists, renders correctly, and instructions are accurate by running `make test-e2e`.

### Implementation for User Story 1

- [x] T002 [P] [US1] Create `docs/testing/e2e-guide.md` with Prerequisites and Quick Start sections
- [x] T003 [P] [US1] Add "Running Tests" and "Test Scenarios" sections to `docs/testing/e2e-guide.md`
- [x] T004 [P] [US1] Update `README.md` to include an "E2E Testing" summary and link to `docs/testing/e2e-guide.md`

---

## Phase 4: User Story 2 - Understanding Plugin Ecosystem Architecture (Priority: P1)

**Goal**: Users understand the interaction between Core, Public Plugin, and CostExplorer Plugin.

**Independent Test**: Verify `docs/architecture/plugin-ecosystem.md` clearly explains component roles and `docs/reference/plugin-compatibility.md` accurately lists feature support.

### Implementation for User Story 2

- [x] T005 [P] [US2] Create `docs/architecture/plugin-ecosystem.md` with System Overview and Component Roles
- [x] T006 [P] [US2] Add Mermaid architecture diagram to `docs/architecture/plugin-ecosystem.md` showing Core -> Plugin Host -> Plugins flow
- [x] T007 [P] [US2] Create `docs/reference/plugin-compatibility.md` with feature support matrix
- [x] T008 [P] [US2] Update `README.md` with high-level architecture diagram and link to ecosystem docs
- [x] T014 [P] [US2] Create `docs/guides/cost-calculation.md` explaining Projected vs Actual cost workflows
- [x] T015 [P] [US2] Update `plugins/aws-public/README.md` to clarify its role as a fallback/public-pricing plugin

---

## Phase 5: User Story 3 - Troubleshooting & Common Issues (Priority: P2)

**Goal**: Users can resolve common installation and runtime errors without filing issues.

**Independent Test**: Verify `docs/guides/troubleshooting.md` covers at least the 4 specified failure scenarios.

### Implementation for User Story 3

- [x] T009 [P] [US3] Create `docs/guides/troubleshooting.md` with sections for "Installation Failures" and "AWS Credential Problems"
- [x] T010 [P] [US3] Add "Cost Calculation Errors" and "E2E Test Timeouts" sections to `docs/guides/troubleshooting.md`

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finalize links, formatting, and linting.

- [x] T011 Update `docs/index.md` to include links to all new guides (`testing/`, `architecture/`, `guides/`, `reference/`)
- [x] T012 Run `markdownlint-cli2 "**/*.md"` to verify formatting compliance
- [x] T013 Verify all relative links in new documents work correctly

## Dependencies

- **US1 (E2E)**: Independent.
- **US2 (Architecture)**: Independent.
- **US3 (Troubleshooting)**: Independent.
- **Polish**: Depends on all US tasks.

## Parallel Execution Examples

- Developer A can work on **US1** (E2E Guide).
- Developer B can work on **US2** (Architecture).
- Developer C can work on **US3** (Troubleshooting).

## Implementation Strategy

1.  **MVP**: Complete **US1** to unblock E2E testing usage.
2.  **Architecture**: Complete **US2** to clarify the system model.
3.  **Support**: Complete **US3** to reduce support burden.
