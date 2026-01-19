---
description: 'Task list for feature implementation'
---

# Tasks: Plugin Init Recorder Fixtures

**Input**: Design documents from `/specs/001-plugin-init-recorder/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are mandatory and must be written before implementation.

**Completeness**: Per Constitution Principle VI, stubs, placeholders, and TODO comments are forbidden.

**Documentation**: Per Constitution Principle IV, README.md and docs/ updates must ship with the feature.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add shared CLI flags and option wiring needed by all stories.

- [x] T001 Update plugin init options for recorder inputs in internal/cli/plugin_init.go
- [x] T002 Add CLI flags for fixture version and offline mode in internal/cli/plugin_init.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core helpers required before story work begins.

- [x] T003 [P] Add fixture source resolution helpers in internal/cli/plugin_init_fixtures.go
- [x] T004 [P] Add recorder workflow helpers in internal/cli/plugin_init_recording.go
- [x] T005 Add progress/error reporting helpers in internal/cli/plugin_init.go
- [x] T006 Add foundational unit test scaffolding in internal/cli/plugin_init_fixtures_test.go

**Checkpoint**: Foundation ready for user story implementation.

---

## Phase 3: User Story 1 - Generate Recorded Fixtures on Init (Priority: P1) 🎯 MVP

**Goal**: Produce recorded request fixtures during plugin initialization using canonical remote fixtures.

**Independent Test**: Running `finfocus plugin init` online writes recorded request files into the generated plugin testdata directory.

### Tests for User Story 1 (TDD)

- [x] T007 [P] [US1] Add online fixture fetch tests in internal/cli/plugin_init_fixtures_test.go
- [x] T008 [P] [US1] Add integration test for recorded requests in test/integration/plugin/init_recording_test.go

### Implementation for User Story 1

- [x] T009 [US1] Implement remote fixture download and validation in internal/cli/plugin_init_fixtures.go
- [x] T010 [US1] Implement recorder command execution for projected/actual/recommendations in internal/cli/plugin_init_recording.go
- [ ] T011 [US1] Integrate recording workflow into RunPluginInit in internal/cli/plugin_init.go
- [ ] T012 [US1] Add progress output and completion summary in internal/cli/plugin_init.go
- [ ] T013 [US1] Ensure recorded fixtures are copied into testdata in internal/cli/plugin_init_recording.go

**Checkpoint**: User Story 1 is fully functional and testable.

---

## Phase 4: User Story 2 - Offline Initialization Fallback (Priority: P2)

**Goal**: Allow plugin initialization to succeed without network access using local fixtures.

**Independent Test**: Running `finfocus plugin init --offline` produces recorded requests using local fixture files.

### Tests for User Story 2 (TDD)

- [x] T014 [P] [US2] Add offline fixture resolution tests in internal/cli/plugin_init_fixtures_test.go
- [x] T015 [P] [US2] Add integration test for offline mode in test/integration/plugin/init_offline_test.go

### Implementation for User Story 2

- [x] T016 [US2] Implement offline fixture lookup in internal/cli/plugin_init_fixtures.go
- [x] T017 [US2] Skip remote fetch when offline in internal/cli/plugin_init.go
- [x] T018 [US2] Add user-facing errors for missing offline fixtures in internal/cli/plugin_init.go

**Checkpoint**: User Story 2 is independently functional.

---

## Phase 5: User Story 3 - Recorder Supports Current Request Types (Priority: P3)

**Goal**: Recorder plugin captures all required request types with mock recommendations when enabled.

**Independent Test**: Recorder plugin records projected cost, actual cost, recommendations, and plugin info requests with deterministic responses.

### Tests for User Story 3 (TDD)

- [x] T019 [P] [US3] Add recorder RPC tests in plugins/recorder/plugin_test.go
- [x] T020 [P] [US3] Add mocker recommendation tests in plugins/recorder/mocker_test.go

### Implementation for User Story 3

- [x] T021 [US3] Implement recommendation mock generation in plugins/recorder/mocker.go
- [x] T022 [US3] Implement recommendations and plugin info handlers in plugins/recorder/plugin.go
- [x] T023 [US3] Verify recorder request recording in plugins/recorder/recorder_test.go

**Checkpoint**: User Story 3 is independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T024 [P] Update README and docs for plugin init recording in README.md
- [ ] T025 [P] Update plugin developer docs in docs/plugins/plugin-development.md
- [ ] T026 [P] Update plugin SDK guidance in docs/plugins/plugin-sdk.md
- [ ] T027 Run markdownlint for updated docs in Makefile
- [ ] T028 Run make lint and make test for full validation in Makefile

---

## Dependencies & Execution Order

### Phase Dependencies

- Setup (Phase 1) → Foundational (Phase 2) → User Stories (Phases 3-5) → Polish (Phase 6)

### User Story Dependencies

- US1 can begin after Foundational and is the MVP.
- US2 and US3 can proceed after Foundational, independent of each other.

### Parallel Opportunities

- T003 and T004 can run in parallel.
- Story-specific tests (T007/T008, T014/T015, T019/T020) can run in parallel.
- Documentation updates (T024-T026) can run in parallel.

---

## Parallel Example: User Story 1

```bash
Task: "Add online fixture fetch tests in internal/cli/plugin_init_fixtures_test.go"
Task: "Add integration test for recorded requests in test/integration/plugin/init_recording_test.go"
```

---

## Parallel Example: User Story 2

```bash
Task: "Add offline fixture resolution tests in internal/cli/plugin_init_fixtures_test.go"
Task: "Add integration test for offline mode in test/integration/plugin/init_offline_test.go"
```

---

## Parallel Example: User Story 3

```bash
Task: "Add recorder RPC tests in plugins/recorder/plugin_test.go"
Task: "Add mocker recommendation tests in plugins/recorder/mocker_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. Complete Phase 1 and Phase 2.
2. Deliver User Story 1 end-to-end, including tests.
3. Validate recorded fixtures and user-facing output.

### Incremental Delivery

1. Add User Story 2 for offline flows.
2. Add User Story 3 for recorder RPC completeness.
3. Finish documentation and validation checks.
