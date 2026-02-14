# Tasks: Split Project-Local and User-Global Configuration

**Input**: Design documents from `/specs/591-config-split/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY
and must be written BEFORE implementation. All code changes must maintain minimum 80%
test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks
MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly
forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation
(CLAUDE.md, docs/) MUST be updated concurrently with implementation and verified in CI
to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation
and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Verification)

**Purpose**: Verify existing code compiles and all tests pass before making changes

- [X] T001 Verify baseline by running `make test` and `make lint` to confirm no
  pre-existing failures

---

## Phase 2: Foundational (Core Building Blocks)

**Purpose**: Create the shared infrastructure (new files) that ALL user stories depend
on. These are new standalone modules with no callers yet, so they can be built and
tested in isolation.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T002 [P] Implement `ResolveProjectDir(flagValue, startDir string) string` with
  tests in `internal/config/project.go` and `internal/config/project_test.go`. The
  function checks (1) flagValue, (2) `FINFOCUS_PROJECT_DIR` env var, (3)
  `pulumi.FindProject(startDir)` walk-up. Returns the path to `$PROJECT/.finfocus/`
  or empty string. Also add a package-level `resolvedProjectDir` variable with
  getter/setter for use by `NewDismissalStore`. Tests must cover: flag override, env
  var override, Pulumi.yaml walk-up, no project fallback, deep nesting (20+ levels),
  filesystem root boundary, absolute path resolution, invalid flagValue path,
  permission errors, nested Pulumi projects (Pulumi.yaml at both `/a/` and `/a/b/`,
  verify walk-up from `/a/b/c/` finds `/a/b/`), and a benchmark assertion that
  discovery completes in under 100ms for a 50-level-deep directory tree (SC-004).
- [X] T003[P] Implement `ShallowMergeYAML(target *Config, overlayPath string) error`
  with tests in `internal/config/merge.go` and `internal/config/merge_test.go`. The
  function reads overlay YAML into `map[string]interface{}`, then for each top-level
  key present (`output`, `plugins`, `logging`, `analyzer`, `plugin_host`, `cost`,
  `routing`), marshals that section to YAML and unmarshals onto the corresponding
  field of `target`. Keys absent in overlay are unchanged. Tests must cover: single
  key override, multiple key override, absent keys preserved, empty overlay file, all
  7 top-level keys, corrupted YAML returns error, missing file returns error, partial
  config (only `cost.budgets`), and zero-value fields in overlay replace non-zero
  global defaults.
- [X] T003b Implement `NewWithProjectDir(projectDir string) *Config` in
  `internal/config/project.go` with tests in `internal/config/project_test.go`.
  Depends on T003 (uses `ShallowMergeYAML`). The function calls `New()` to load
  global config, then if `projectDir` is non-empty and
  `projectDir/config.yaml` exists, applies `ShallowMergeYAML(cfg, overlayPath)`.
  Returns the merged config. Tests must cover: empty projectDir returns same as
  `New()`, project config overrides single section, project config preserves
  unspecified sections, missing project config.yaml uses global defaults, corrupted
  project config logs warning and uses global defaults.
- [X] T004 [P] Implement `EnsureGitignore(dir string) (bool, error)` with tests in
  `internal/config/gitignore.go` and `internal/config/gitignore_test.go`. The
  function checks if `dir/.gitignore` exists; if so returns `(false, nil)` without
  overwriting (FR-007). If not, writes the standard content: `dismissed.json`,
  `dismissed.json.lock`, `dismissed.json.tmp`, `*.log` with header comment. Tests
  must cover: creates new .gitignore, does not overwrite existing, creates parent
  directory if needed, file permission errors, and content verification.

**Checkpoint**: All foundational functions exist with 80%+ test coverage. No CLI
integration yet.

---

## Phase 3: User Story 3 - Project Directory Discovery (Priority: P1)

**Goal**: The CLI automatically detects Pulumi projects by walking up from CWD and
supports `--project-dir` flag and `FINFOCUS_PROJECT_DIR` env var for overrides.

**Independent Test**: Create a nested directory structure with `Pulumi.yaml` at the
root and verify FinFocus finds the project root from a deeply nested subdirectory.

**Why first**: US3 is the foundation that enables US1 (config merge) and US2
(project-scoped dismissals). Both depend on the resolved project directory.

### Tests for User Story 3 (TDD)

- [X] T005 [P] [US3] Write tests for `--project-dir` persistent flag and
  `FINFOCUS_PROJECT_DIR` env var integration in `internal/cli/root_test.go`. Tests
  must verify: flag is available on all subcommands, flag value takes precedence
  over env var, env var takes precedence over auto-detection, both empty falls back
  to walk-up.

### Implementation for User Story 3

- [X] T006 [US3] Add `--project-dir` persistent flag to root command in
  `internal/cli/root.go`. Declare a package-level `projectDirFlag string` variable
  and register it with `cmd.PersistentFlags().StringVar(&projectDirFlag,
  "project-dir", "", "explicit Pulumi project directory for config resolution")`.
- [X] T007 [US3] Wire project directory resolution into `PersistentPreRunE` in
  `internal/cli/root.go`. After logging setup, call
  `config.ResolveProjectDir(projectDirFlag, cwd)` where `cwd` is from `os.Getwd()`.
  Store the result via `config.SetResolvedProjectDir(resolvedDir)` so that
  `NewDismissalStore` and config initialization can use it. NOTE: This task only
  resolves and stores the project dir. The actual call to
  `InitGlobalConfigWithProject` is wired in T010 (US1) to keep the merge logic
  separate from the discovery logic.
- [X] T008 [US3] Modify `InitGlobalConfig` in `internal/config/integration.go` to
  accept the resolved project dir. Change `InitGlobalConfig()` to
  `InitGlobalConfigWithProject(projectDir string)`. If `projectDir` is non-empty,
  use `NewWithProjectDir(projectDir)` instead of `New()`. Keep a backward-compatible
  `InitGlobalConfig()` that calls `InitGlobalConfigWithProject("")`. Update all
  existing callers.

**Checkpoint**: Running `finfocus --project-dir /some/path cost projected --help`
resolves the project directory and stores it via `SetResolvedProjectDir`. Without the
flag, auto-detection via `pulumi.FindProject()` is used.
`InitGlobalConfigWithProject` exists but is not yet called from `PersistentPreRunE`
(that wiring happens in T010/US1). Global config loads via the existing `New()` path.

---

## Phase 4: User Story 1 - Per-Project Budget Configuration (Priority: P1)

**Goal**: Users can place a `.finfocus/config.yaml` in their Pulumi project root with
project-specific budgets/settings, and the tool merges it over global defaults.

**Independent Test**: Create two Pulumi projects with different
`.finfocus/config.yaml` budget values. Run FinFocus from each and verify the correct
budget is loaded while global defaults (e.g., output format) are inherited.

### Tests for User Story 1 (TDD)

- [X] T009 [P] [US1] Write integration tests for config merge in
  `internal/config/integration_test.go`. Tests must verify: project config overrides
  global budget value, project config inherits output format from global, no project
  dir produces identical config to `New()`, two projects load independent configs.

### Implementation for User Story 1

- [X] T010 [US1] Connect `PersistentPreRunE` to call
  `config.InitGlobalConfigWithProject(resolvedProjectDir)` in
  `internal/cli/root.go`. This ensures the global config singleton contains the
  merged project+global configuration for all subsequent commands. Verify that
  `config.GetGlobalConfig()` returns the merged config in cost commands.
- [X] T011 [US1] Add backward compatibility tests in
  `internal/config/project_test.go` verifying that `NewWithProjectDir("")` produces
  identical output to `New()` across all config fields. Test with
  `FINFOCUS_HOME` set, with `PULUMI_HOME` set, and with neither set.

**Checkpoint**: `finfocus cost projected` from inside a Pulumi project with
`.finfocus/config.yaml` containing `cost.budgets.global.amount: 5000` uses $5,000
budget. Same command from outside the project uses global default.

---

## Phase 5: User Story 2 - Per-Project Recommendation Dismissals (Priority: P1)

**Goal**: Recommendation dismissals are stored in the project-local
`.finfocus/dismissed.json` when inside a Pulumi project, preventing cross-project
dismissal leakage.

**Independent Test**: Dismiss a recommendation in Project A, then verify it is still
visible in Project B.

### Tests for User Story 2 (TDD)

- [X] T012 [P] [US2] Write unit tests for project-aware `NewDismissalStore` in
  `internal/config/dismissed_test.go`. Tests must verify: empty filePath with project
  dir set resolves to `$PROJECT/.finfocus/dismissed.json`, empty filePath without
  project dir resolves to `ResolveConfigDir()/dismissed.json` (not hardcoded
  `os.UserHomeDir()`), explicit filePath is used as-is, `FINFOCUS_HOME` is respected
  when no project context.
- [X] T013 [P] [US2] Write integration test for cross-project dismissal isolation in
  `test/integration/cli/recommendations_project_dismiss_test.go`. Create two temp
  Pulumi projects. Dismiss a recommendation in Project A. Verify it remains visible
  in Project B. Verify dismissal persists in Project A.

### Implementation for User Story 2

- [X] T014 [US2] Fix `NewDismissalStore` in `internal/config/dismissed.go`. When
  `filePath` is empty: (1) check `GetResolvedProjectDir()` — if non-empty, use
  `filepath.Join(projectDir, "dismissed.json")`; (2) otherwise fall back to
  `filepath.Join(ResolveConfigDir(), "dismissed.json")`. Remove the hardcoded
  `os.UserHomeDir()` call entirely.
- [X] T015 [US2] Verify all `loadDismissalStore()` call sites in
  `internal/cli/cost_recommendations_dismiss.go`,
  `internal/cli/cost_recommendations.go`,
  `internal/cli/cost_recommendations_history.go`, and
  `internal/cli/cost_recommendations_undismiss.go` work correctly with the new
  project-aware `NewDismissalStore`. No code changes expected — the fix in T014
  is transparent to callers. Add a test in `internal/cli/cost_recommendations_test.go`
  confirming that `loadDismissalStore()` returns a store with a path under the
  project dir when `SetResolvedProjectDir` has been called.

**Checkpoint**: `finfocus cost recommendations dismiss rec-123` in Project A stores
dismissal in `ProjectA/.finfocus/dismissed.json`. Running
`finfocus cost recommendations` in Project B still shows rec-123.

---

## Phase 6: User Story 4 - Configuration Initialization with .gitignore (Priority: P2)

**Goal**: `finfocus config init` creates a project-local `.finfocus/` directory with
`.gitignore` and default `config.yaml` when run inside a Pulumi project.

**Independent Test**: Run `config init` inside a Pulumi project directory and verify
the created `.finfocus/` directory contains both `config.yaml` and `.gitignore`.

### Tests for User Story 4 (TDD)

- [X] T016 [P] [US4] Write tests for project-aware config init in
  `internal/cli/config_init_test.go`. Tests must verify: inside Pulumi project
  creates `.finfocus/` at project root with `.gitignore` and `config.yaml`, existing
  `.gitignore` is preserved, `--global` flag forces global init even inside project,
  outside Pulumi project behaves as before (global init), `--force` overwrites
  `config.yaml` but not `.gitignore`.

### Implementation for User Story 4

- [X] T017 [US4] Enhance `NewConfigInitCmd` in `internal/cli/config_init.go`. Add
  `--global` flag. In `RunE`: (1) resolve project dir using
  `config.ResolveProjectDir(projectDirFlag, cwd)`; (2) if project dir found and
  `--global` not set, create config at `$PROJECT/.finfocus/config.yaml` and call
  `config.EnsureGitignore($PROJECT/.finfocus/)`; (3) if no project dir or
  `--global` set, keep current behavior; (4) update output messages to show the
  actual path used.
- [X] T018 [US4] Add `--global` flag documentation to the command's `Long`
  description and `Example` field in `internal/cli/config_init.go`.

**Checkpoint**: `finfocus config init` in a Pulumi project creates
`$PROJECT/.finfocus/config.yaml` + `$PROJECT/.finfocus/.gitignore`.
`finfocus config init --global` creates `~/.finfocus/config.yaml`.

---

## Phase 7: User Story 5 - Global Resources Remain Shared (Priority: P2)

**Goal**: Plugins, cache, and logs always resolve from the global `~/.finfocus/`
directory regardless of project context. The config split does not affect global
resource resolution.

**Independent Test**: Install a plugin globally and verify it is discovered when
running FinFocus from any Pulumi project.

### Tests for User Story 5 (TDD)

- [X] T019 [P] [US5] Write tests verifying global resource isolation in
  `internal/config/project_test.go`. Tests must verify: `Config.PluginDir` always
  uses `ResolveConfigDir()`, not project dir; `Config.SpecDir` always uses
  `ResolveConfigDir()`; `Config.Cost.Cache.Directory` always uses
  `ResolveConfigDir()`; `Config.Logging.File` always uses `ResolveConfigDir()`.
  Test with project dir set and verify these fields are unchanged. Also test the
  dual-path scenario: `FINFOCUS_HOME=/custom` set AND project dir active — verify
  `PluginDir` uses `/custom/plugins` while config merge uses the project overlay.

### Implementation for User Story 5

- [X] T020 [US5] Verify that `NewWithProjectDir` in `internal/config/project.go`
  preserves global resource paths after merge. The `ShallowMergeYAML` function must
  NOT merge `PluginDir` or `SpecDir` (they are `yaml:"-"` tagged, so they are
  excluded from YAML parsing). Add explicit assertions in `project_test.go` that
  after merging a project config, `PluginDir` and `SpecDir` still point to the
  global directory.

**Checkpoint**: With a project config that sets `cost.budgets`, verify that plugin
discovery, cache, and log paths still use the global `~/.finfocus/` directory.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, validation, and final quality checks

- [X] T021 [P] Update `CLAUDE.md` with new config resolution documentation including:
  project directory discovery, `--project-dir` flag, `FINFOCUS_PROJECT_DIR` env var,
  shallow merge behavior, project-local `.finfocus/` directory layout, and config
  resolution precedence order.
- [X] T022 [P] Update `internal/cli/CLAUDE.md` with `--project-dir` flag
  documentation, updated command hierarchy showing `config init` changes, and
  `loadDismissalStore` behavior change.
- [X] T023 Run `make lint` and fix any linting issues in new and modified files.
- [X] T024 Run `make test` and `make test-race` and verify all existing and new tests
  pass with zero failures. The race detector run is required by Constitution
  Principle II Quality Gates.
- [X] T025 Verify 80%+ test coverage on new code in `internal/config/project.go`,
  `internal/config/merge.go`, `internal/config/gitignore.go`, and
  `internal/config/dismissed.go` by running
  `go test -coverprofile=coverage.out ./internal/config/... && go tool cover -func=coverage.out`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — verify baseline first
- **Foundational (Phase 2)**: Depends on Phase 1 — creates new files, BLOCKS all
  user stories
- **US3 (Phase 3)**: Depends on Phase 2 — integrates ResolveProjectDir into CLI
- **US1 (Phase 4)**: Depends on Phase 3 — needs project dir resolved for config merge
- **US2 (Phase 5)**: Depends on Phase 3 — needs project dir resolved for dismissals
- **US4 (Phase 6)**: Depends on Phase 2 — uses EnsureGitignore and ResolveProjectDir
- **US5 (Phase 7)**: Depends on Phase 4 — verifies merge doesn't break global paths
- **Polish (Phase 8)**: Depends on all previous phases

### User Story Dependencies

```text
Phase 1: Setup
    │
    ▼
Phase 2: Foundational (T002, T003, T004 in parallel)
    │
    ├───────────────────┐
    ▼                   ▼
Phase 3: US3         Phase 6: US4
(T005→T006→T007→T008)  (T016→T017→T018)
    │
    ├──────────┐
    ▼          ▼
Phase 4: US1  Phase 5: US2
(T009→T010→T011)  (T012,T013→T014→T015)
    │          │
    ▼          │
Phase 7: US5◄──┘
(T019→T020)
    │
    ▼
Phase 8: Polish
(T021,T022→T023→T024→T025)
```

### Within Each User Story

1. Tests MUST be written and FAIL before implementation
2. Core logic before CLI integration
3. Unit tests before integration tests
4. Story complete before moving to next priority

### Parallel Opportunities

- **Phase 2**: T002, T003, T004 are independent new files — all parallel
- **Phase 3**: T005 parallel with Phase 6 tests (T016)
- **Phase 4 + Phase 5**: US1 and US2 can run in parallel after US3 completes
- **Phase 6**: Can run in parallel with Phase 3 (different files)
- **Phase 7**: T019 parallel (tests only)
- **Phase 8**: T021, T022 parallel (different doc files)

---

## Parallel Example: Phase 2 (Foundational)

```text
# All three foundational modules can be built simultaneously:
Agent 1: "Implement ResolveProjectDir in internal/config/project.go + project_test.go"
Agent 2: "Implement ShallowMergeYAML in internal/config/merge.go + merge_test.go"
Agent 3: "Implement EnsureGitignore in internal/config/gitignore.go + gitignore_test.go"
```

## Parallel Example: US1 + US2 (after US3)

```text
# Once project directory discovery is wired in (US3 complete):
Agent 1: "US1 - Wire config merge into global config init + test budget override"
Agent 2: "US2 - Fix NewDismissalStore + test cross-project isolation"
```

---

## Implementation Strategy

### MVP First (US3 + US1)

1. Complete Phase 1: Setup (verify baseline)
2. Complete Phase 2: Foundational (T002, T003, T004 in parallel)
3. Complete Phase 3: US3 — Project directory discovery works
4. Complete Phase 4: US1 — Project budgets work
5. **STOP and VALIDATE**: Test with two Pulumi projects having different budgets
6. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Core building blocks ready
2. Add US3 → Project detection works → Validate
3. Add US1 → Per-project budgets → Validate (MVP!)
4. Add US2 → Per-project dismissals → Validate
5. Add US4 → Clean init experience → Validate
6. Add US5 → Verify global resources → Validate
7. Polish → Documentation and coverage

### Key Risk: Backward Compatibility

The highest-risk change is modifying `InitGlobalConfig` (T008) and
`NewDismissalStore` (T014). Both are called from many places. Mitigation:

- Keep backward-compatible wrapper functions
- Run `make test` after each phase to catch regressions
- All existing tests must pass without modification (SC-006)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Run `make lint` and `make test` after each phase
- Stop at any checkpoint to validate story independently
- Total: 25 tasks across 8 phases
