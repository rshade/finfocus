# Tasks: Batch Bug Fixes — Analyzer, Registry, Config, and CI Correctness

**Input**: Design documents from `specs/599-batch-bug-fixes/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY
and must be written BEFORE implementation. Write the test, confirm it fails, then fix.

**Completeness**: Per Constitution Principle VI, all tasks MUST be fully implemented.
No stub functions, no placeholders, no TODO comments.

**Documentation**: Docs updated concurrently with implementation (config-reference.md
for #753).

**Organization**: Tasks are grouped by user story. Each story is independently
implementable and testable. Tests go co-located with source as `*_test.go` files —
never in the deprecated `test/unit/` directory.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete dependencies)
- **[Story]**: Which user story this task belongs to
- Exact file paths required in every description

---

## Phase 1: Setup

**Purpose**: Verify clean baseline before any changes

- [X] T001 Verify baseline — run `make test` and `make lint` on the clean branch to
  confirm zero pre-existing failures before any code changes are made

---

## Phase 2: Foundational

**Purpose**: All 10 active fixes are independent — no shared blocking prerequisites.
Phase 2 is intentionally empty. User story phases can begin immediately after T001.

---

## Phase 3: User Story 1 — Analyzer Stack Summary Shows Correct Total Cost (Priority: P1) 🎯 MVP

**Goal**: Fix `AnalyzeStack` returning `$0.00 (0 resources analyzed)` by correcting
the `clearCostCache()` lifecycle. Fixes issue #746.

**Independent Test**: Run `pulumi preview --policy-pack ~/.finfocus/analyzer` on a stack
with ≥2 priced resources; verify `stack-cost-summary` advisory shows a non-zero total
equal to the sum of per-resource costs.

### Tests for User Story 1 (TDD — write first, confirm they FAIL before fixing) ⚠️

- [X] T002 [US1] Write table-driven unit test in `internal/analyzer/server_test.go`
  named `TestAnalyzeStack_CostAccumulation` that: (1) calls `ConfigureStack`, (2) calls
  `Analyze` N times with valid mock costs, (3) calls `AnalyzeStack`, (4) asserts
  `ResourceCount == N` and `TotalMonthlyCost > 0`; test must FAIL before the fix
- [X] T005 [P] [US1] In `internal/analyzer/summary_test.go` (create if absent), add a
  test case named `TestBuildCostSummary_IncludesNonErrorItems` that passes cost items
  with no `Error` and no error-prefixed `Notes` to `BuildCostSummary` and asserts they
  are included in `ResourceCount` and the total cost; this validates the secondary
  filter in `internal/analyzer/summary.go` does not exclude valid results; can run in
  parallel with T003/T004 since it touches a different file

### Implementation for User Story 1

- [X] T003 [US1] Remove the `s.clearCostCache()` call from `ConfigureStack` in
  `internal/analyzer/server.go` (approximately line 435)
- [X] T004 [US1] Add `s.clearCostCache()` at the END of `AnalyzeStack` in
  `internal/analyzer/server.go`, after the stack-cost-summary diagnostic is emitted and
  returned, so cache is cleared only after the summary is built

**Checkpoint**: `go test ./internal/analyzer/... -run TestAnalyzeStack` passes; `make lint` clean

---

## Phase 4: User Story 2 — Analyzer Install Creates Correct Version Directory (Priority: P1)

**Goal**: Fix the double-`v` directory name (`analyzer-finfocus-vv0.3.1`) by normalizing
the version string in `Install()`. Fixes issue #749.

**Independent Test**: Run `finfocus analyzer install` and confirm the created directory
under `~/.pulumi/plugins/` has exactly one `v` in its name.

### Tests for User Story 2 (TDD — write first, confirm they FAIL before fixing) ⚠️

- [X] T006 [US2] Write table-driven unit test in `internal/analyzer/install_test.go`
  named `TestInstall_VersionNormalization` with inputs `"v0.3.1"`, `"0.3.1-dirty"`,
  `"v1.0.0-rc1"` asserting the resulting directory name contains exactly one `v` prefix
  and no double-`v` sequence; test must FAIL before the fix

### Implementation for User Story 2

- [X] T007 [US2] Change `analyzerDirPrefix` constant from `"analyzer-finfocus-v"` to
  `"analyzer-finfocus-"` in `internal/analyzer/install.go` (approximately line 18)
- [X] T008 [US2] Add version normalization in `Install()` in
  `internal/analyzer/install.go` before constructing `versionedDir`: get version via
  `version.GetVersion()`; if it does not have a `"v"` prefix, prepend one; use the
  normalized version string (not raw `currentVersion`) everywhere a versioned directory
  path is constructed (approximately lines 177 and 211)

**Checkpoint**: `go test ./internal/analyzer/... -run TestInstall` passes; `make lint` clean

---

## Phase 5: User Story 3 — SBOM Attached to GitHub Releases Successfully (Priority: P1)

**Goal**: Fix the `Resource not accessible by integration` CI failure for the SBOM
workflow by granting `contents: write`. Fixes issue #698.

**Independent Test**: Push a version tag and confirm the SBOM `.spdx.json` appears as
a release asset with no permission error in the workflow log.

### Implementation for User Story 3

- [X] T009 [US3] Change `contents: read` to `contents: write` in the `permissions:`
  block of the `build-and-push` job in `.github/workflows/docker.yml`; no Go changes,
  no tests required

**Checkpoint**: Workflow YAML is syntactically valid (`gh workflow view docker.yml`); change
is the only edit in this file

---

## Phase 6: User Story 4 — No JSON Log Lines in `pulumi preview` Diagnostics (Priority: P2)

**Goal**: Redirect all finfocus log output to a file when `FINFOCUS_ANALYZER_MODE=true`,
preventing Pulumi from capturing JSON log lines as `Diagnostics:` entries. Fixes #748.

**Independent Test**: Run `pulumi preview --policy-pack ~/.finfocus/analyzer`; confirm
`Diagnostics:` contains no lines starting with `{"level":`.

### Tests for User Story 4 (TDD — write first, confirm they FAIL before fixing) ⚠️

- [X] T010 [US4] Write unit test in `internal/cli/logging_setup_test.go` named
  `TestSetupLogging_AnalyzerModeRedirect` that: (1) sets `FINFOCUS_ANALYZER_MODE=true`
  via `t.Setenv`, (2) calls `setupLogging()` with a test cobra command, (3) inspects
  the logging config applied (e.g., via the returned `LogPathResult` or by capturing
  the logger's output destination) and asserts no console/stderr output is active and
  the file path is non-empty; also add a case with `FINFOCUS_ANALYZER_MODE` unset
  asserting console output is unchanged; test must FAIL before the fix

### Implementation for User Story 4

- [X] T011 [US4] In `setupLogging()` in `internal/cli/logging_setup.go` (the function
  is defined there, not in `common_execution.go`), after building `loggingCfg` from
  `config.GetLoggingConfig()` and applying debug/env overrides, check
  `os.Getenv(constants.EnvAnalyzerMode) == "true"`; when true, set
  `loggingCfg.Outputs = []config.LogOutput{{Type: "file", Level: loggingCfg.Level, Path: loggingCfg.File}}`
  (using the existing `loggingCfg.File` path, or defaulting to
  `filepath.Join(config.ResolveConfigDir(), "logs", "analyzer.log")` if empty) so
  that `loggingCfg.ToLoggingConfig()` produces a file-only `logging.Config` with
  no stderr writes

**Checkpoint**: `go test ./internal/cli/... -run TestSetupLogging` passes; run
`pulumi preview --policy-pack ~/.finfocus/analyzer` locally and confirm clean Diagnostics

---

## Phase 7: User Story 5 — Recorder Plugin Does Not Flood Logs with Nil Summary Warnings (Priority: P2)

**Goal**: Add a non-nil `RecommendationsSummary` to all `GetRecommendations` return
paths in the recorder plugin. Fixes issue #747.

**Independent Test**: Run any cost command with the recorder plugin installed; confirm
no `"plugin returned response with nil summary"` WARN entries appear.

### Tests for User Story 5 (TDD — write first, confirm they FAIL before fixing) ⚠️

- [X] T012 [US5] Write unit test in `plugins/recorder/plugin_test.go` named
  `TestGetRecommendations_SummaryNotNil` that calls `GetRecommendations` with mock mode
  both enabled and disabled and asserts `resp.Summary != nil` in both cases; test must
  FAIL before the fix

### Implementation for User Story 5

- [X] T013 [US5] In `plugins/recorder/plugin.go`, add a non-nil
  `&pbc.RecommendationsSummary{TotalCount: 0, TotalMonthlySavings: 0, Currency: "USD"}`
  to all return paths of `GetRecommendations` (approximately lines 140–208): the mock
  mode path with `mocker.CreateRecommendationsResponse()`, the mock-enabled pagination
  path, and the non-mock path — ensure `Summary` is set on every returned
  `GetRecommendationsResponse`

**Checkpoint**: `go test ./plugins/recorder/... -run TestGetRecommendations` passes;
`make lint` clean

---

## Phase 8: User Story 6 — Registry Discovers Plugins via Directory-Level Symlinks (Priority: P3)

**Goal**: Replace `DirEntry.IsDir()` with `os.Stat().IsDir()` at both directory levels
in `ListPlugins()` so symlinks-to-directories are followed. Fixes issue #750.

**Independent Test**: Create a symlinked plugin directory structure and verify
`finfocus plugin list` returns the expected plugin name.

### Tests for User Story 6 (TDD — write first, confirm they FAIL before fixing) ⚠️

- [X] T014 [US6] Write test in `internal/registry/registry_test.go` named
  `TestListPlugins_SymlinkDiscovery` that: (1) creates a real plugin directory structure
  in `t.TempDir()` with name and version directories, (2) creates a symlink pointing to
  the plugin name directory, (3) creates a `Registry` with the temp root, (4) calls
  `ListPlugins()`, (5) asserts the plugin is discovered; add `t.Skipf` on
  `runtime.GOOS == "windows"` since `os.Symlink` may require elevation there; test must
  FAIL before the fix

### Implementation for User Story 6

- [X] T015 [US6] At the plugin-name level in `ListPlugins()` in
  `internal/registry/registry.go` (approximately line 52), replace `entry.IsDir()` with:
  `info, err := os.Stat(filepath.Join(r.root, entry.Name())); if err != nil || !info.IsDir() { continue }`
  to follow symlinks when checking directory status
- [X] T016 [US6] At the version level in `ListPlugins()` in
  `internal/registry/registry.go` (approximately line 63), replace `version.IsDir()` with:
  `info, err := os.Stat(filepath.Join(pluginPath, version.Name())); if err != nil || !info.IsDir() { continue }`
  to follow symlinks when checking version directory status

**Checkpoint**: `go test ./internal/registry/... -run TestListPlugins` passes; `make lint` clean

---

## Phase 9: User Story 7 — Documentation Matches Implementation for Config/Env Vars (Priority: P3)

**Goal**: Implement the three documented-but-unimplemented configuration knobs:
`FINFOCUS_PLUGIN_DIR` env var (#752), `plugin_dir:` YAML key (#753), and
`analyzer.plugins.<name>.enabled: false` (#751). Update docs to reflect actual key name
for #753.

**Independent Test**: Set each documented knob and verify behavior changes as documented.

### Tests for User Story 7 (TDD — write first, confirm they FAIL before fixing) ⚠️

- [X] T017 [US7] Write test in `internal/config/config_test.go` named
  `TestConfig_FinfocusPluginDirEnvOverride` that sets `FINFOCUS_PLUGIN_DIR=/custom/path`
  via `t.Setenv`, calls `New()`, and asserts `cfg.PluginDir == "/custom/path"`; test must
  FAIL before the fix
- [X] T018 [US7] Write test in `internal/config/config_test.go` named
  `TestConfig_PluginDirYAMLKey` that writes a config file with `plugin_dir: /yaml/path`,
  calls `New()` with that config, and asserts `cfg.PluginDir == "/yaml/path"`; test must
  FAIL before the fix
- [X] T019 [P] [US7] Write test in `internal/cli/analyzer_serve_test.go` named
  `TestSetupAnalyzerInfra_DisabledPlugin` that creates a `cfg.Analyzer.Plugins` map with
  a plugin entry having `Enabled: false`, passes a mock client list containing that
  plugin, calls the filtering logic, and asserts the disabled plugin is excluded from
  the returned client list; test must FAIL before the fix

### Implementation for User Story 7

- [X] T020 [US7] Add `FINFOCUS_PLUGIN_DIR` env var handling at the end of
  `applyEnvOverrides()` in `internal/config/config.go`:
  `if pluginDir := os.Getenv("FINFOCUS_PLUGIN_DIR"); pluginDir != "" { c.PluginDir = pluginDir }`
  (applied after `FINFOCUS_HOME` sets the base directory, so it overrides only `PluginDir`)
- [X] T021 [US7] Add a new top-level `PluginDirOverride` field to the `Config` struct
  in `internal/config/config.go` with YAML tag `plugin_dir,omitempty`; include a godoc
  comment consistent with adjacent fields (e.g., `// PluginDirOverride overrides the
  computed PluginDir when set via the plugin_dir: config key`); then in `New()`, after
  `cfg.Load()` returns, apply:
  `if cfg.PluginDirOverride != "" { cfg.PluginDir = cfg.PluginDirOverride }` (YAML value
  applied before env var override, which takes higher precedence)
- [X] T022 [P] [US7] Update `docs/reference/config-reference.md` to show `plugin_dir:`
  as a top-level key (not nested under `plugins: dir:`), reflecting the actual YAML
  schema; no change to `environment-variables.md` (FINFOCUS_PLUGIN_DIR is already
  correctly documented there)
- [X] T023 [P] [US7] In `setupAnalyzerInfra()` in `internal/cli/analyzer_serve.go`,
  after `reg.Open()` returns `clients`, add filtering logic:
  if `cfg.Analyzer.Plugins` is non-empty, iterate `clients` and exclude any client
  where `cfg.Analyzer.Plugins[client.Name]` exists AND `Enabled == false`; note that
  `client.Name` is a **string field** (not a method call) on `*pluginhost.Client`
  (verified: `type Client struct { Name string ... }` in `internal/pluginhost/host.go`);
  plugins absent from the map default to enabled (backward compatible)

**Checkpoint**: `go test ./internal/config/... ./internal/cli/...` passes; `make lint` clean

---

## Phase 10: User Story 8 — Intermittent TUI Zero-Cost Display Investigated and Resolved (Priority: P3)

**Goal**: Add targeted debug logging in `resolveSKUAndRegion` and `enrichProjectedCost`
to identify the root cause of intermittent `$0.00` projected costs in the TUI overview,
then implement the appropriate fix. Fixes issue #723.

**Independent Test**: Run `finfocus cost overview --debug` 10+ times; observe zero-cost
runs and correlate with debug log entries identifying the source of the failure.

### Tests for User Story 8 (TDD — write first, then validate against debug findings) ⚠️

- [X] T024 [P] [US8] Add DEBUG-level log in `resolveSKUAndRegion` in
  `internal/proto/adapter.go` when returning an empty SKU or empty region: log the
  provider, resource type, and available property keys to enable tracing which resource
  types trigger the fallback; this is additive (no test required, but validate via debug
  run)
- [X] T025 [P] [US8] Add DEBUG-level log in `enrichProjectedCost` in
  `internal/engine/overview_enrich.go` when `costResult.Monthly == 0`: log the resource
  type, SKU, region, and any error/notes from the result to identify which lookup
  returned zero

### Implementation for User Story 8

- [X] T026 [US8] Write regression test in `internal/proto/adapter_test.go` named
  `TestResolveSKUAndRegion_EmptyFallback` that passes a resource with properties that
  all fail extraction and asserts the function returns empty strings (not panics); this
  creates a baseline test covering the empty-SKU/region path found during investigation
- [X] T027 [US8] **Blocked by T024 and T025** (complete debug runs first): based on
  findings, implement the root cause fix — if SKU/region extraction is the cause, add
  missing property name mappings in `internal/proto/adapter.go`; if plugin startup
  timing is the cause, add a startup health check or retry in `internal/pluginhost/`;
  if the fix belongs in a plugin (not core), document the root cause with a
  reproducible test case and record the finding in GitHub issue #723; update or add
  tests in the appropriate `*_test.go` file co-located with the fixed source

**Checkpoint**: `go test ./internal/proto/... ./internal/engine/...` passes; 10
consecutive `finfocus cost overview` runs show non-zero costs for known-priced resources

---

## Phase 11: User Story 9 — Force Reinstall Keeps Policy Pack Binary in Sync (Priority: P4, DEFERRED)

**Goal**: When `finfocus analyzer install --force` runs, both the Pulumi plugin binary
and the policy pack binary should be updated. Fixes issue #754.

> **⚠️ DEFERRED**: This fix depends on the `--setup-policy-pack` feature (not yet
> implemented). Implement after that feature lands. No tasks to execute now.

---

## Phase 12: Polish & Cross-Cutting Concerns

**Purpose**: Final validation across all stories

- [X] T028 [P] Verify `docs/reference/environment-variables.md` correctly documents
  `FINFOCUS_PLUGIN_DIR` (no change expected; this is a verification-only step to confirm
  #752 implementation matches existing docs)
- [X] T029 Run `make lint` — zero lint errors required before claiming completion
- [X] T030 Run `make test` — full test suite must pass with no regressions; additionally
  run `go test -coverprofile=coverage.out ./internal/analyzer/... ./internal/config/...
  ./internal/cli/... ./internal/registry/... ./plugins/recorder/... ./internal/proto/...
  ./internal/engine/...` and confirm no package coverage regresses below its pre-change
  baseline (SC-009)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately with T001
- **Phase 2 (Foundational)**: Empty — no blocking prerequisites
- **Phases 3–10 (User Stories)**: All depend on T001 (baseline verified); all independent
  of each other — can run in parallel across developers
- **Phase 11 (Deferred)**: Blocked on policy pack setup feature; do not start
- **Phase 12 (Polish)**: Depends on all desired user story phases completing

### User Story Dependencies

| Story | Issues | Files | Depends On |
|-------|--------|-------|------------|
| US1 | #746 | `internal/analyzer/server.go`, `server_test.go`, `summary.go` | T001 only |
| US2 | #749 | `internal/analyzer/install.go`, `install_test.go` | T001 only |
| US3 | #698 | `.github/workflows/docker.yml` | T001 only |
| US4 | #748 | `internal/cli/common_execution.go`, `common_execution_test.go` | T001 only |
| US5 | #747 | `plugins/recorder/plugin.go`, `plugin_test.go` | T001 only |
| US6 | #750 | `internal/registry/registry.go`, `registry_test.go` | T001 only |
| US7 | #751–753 | `internal/config/config.go`, `config_test.go`, `internal/cli/analyzer_serve.go`, `analyzer_serve_test.go`, `docs/` | T001 only |
| US8 | #723 | `internal/proto/adapter.go`, `adapter_test.go`, `internal/engine/overview_enrich.go` | T001 only |
| US9 | #754 | Deferred | Policy pack feature |

### Within Each User Story

1. **Test first** (TDD): Write failing test → confirm it fails → implement fix
2. **Sequential within story**: Tasks in the same file are sequential
3. **Parallel within story**: Tasks marked [P] touch different files and can run
   simultaneously

---

## Parallel Execution Examples

### Example: Run US1, US3, US5 simultaneously (three independent developers)

```bash
# Developer A — US1 (#746)
go test ./internal/analyzer/... -run TestAnalyzeStack_CostAccumulation  # confirm fail
# Edit server.go, summary.go
go test ./internal/analyzer/... -run TestAnalyzeStack

# Developer B — US3 (#698, no tests needed)
# Edit .github/workflows/docker.yml
gh workflow view docker.yml

# Developer C — US5 (#747)
go test ./plugins/recorder/... -run TestGetRecommendations_SummaryNotNil  # confirm fail
# Edit plugin.go
go test ./plugins/recorder/... -run TestGetRecommendations
```

### Example: Within US7, parallelize across different files

```bash
# Stream 1 — config.go changes (T017, T018, T020, T021 sequential)
go test ./internal/config/... -run TestConfig_FinfocusPluginDirEnvOverride  # fail
go test ./internal/config/... -run TestConfig_PluginDirYAMLKey              # fail
# Edit config.go
go test ./internal/config/...

# Stream 2 — analyzer_serve.go changes (T019, T023 sequential, parallel with Stream 1)
go test ./internal/cli/... -run TestSetupAnalyzerInfra_DisabledPlugin  # fail
# Edit analyzer_serve.go
go test ./internal/cli/...

# Stream 3 — docs changes (T022 parallel with both streams)
# Edit docs/reference/config-reference.md
make docs-lint
```

---

## Implementation Strategy

### MVP First (User Stories 1–3 Only, all P1)

1. Complete T001 (baseline)
2. Complete Phase 3 (US1 — stack summary fix) → validate independently
3. Complete Phase 4 (US2 — double-v fix) → validate independently
4. Complete Phase 5 (US3 — CI permission fix) → validate on next tag push
5. **STOP and VALIDATE**: All three P1 fixes work independently
6. Merge or demo if ready

### Incremental Delivery

1. T001 → P1 stories (US1, US2, US3) → validate → merge
2. P2 stories (US4, US5) → validate → merge
3. P3 stories (US6, US7, US8) → validate → merge
4. Phase 12 (Polish) → final validation → merge

### Parallel Team Strategy (if multiple developers available)

With 3+ developers after T001 completes:

- Developer A: US1 + US2 (both in `internal/analyzer/`)
- Developer B: US3 + US5 (trivial CI fix + recorder plugin)
- Developer C: US4 (log redirect — `internal/cli/`)
- Developer D (when A/B/C finish): US6 + US7 (registry + config)
- Any developer: US8 (investigation — can be done in parallel)

---

## Summary

| Phase | Story | Issue(s) | Tasks | Priority |
|-------|-------|----------|-------|----------|
| 3 | US1 — AnalyzeStack zero cost | #746 | T002–T005 | P1 |
| 4 | US2 — Analyzer install double-v | #749 | T006–T008 | P1 |
| 5 | US3 — SBOM CI permission | #698 | T009 | P1 |
| 6 | US4 — Analyzer log redirect | #748 | T010–T011 | P2 |
| 7 | US5 — Recorder nil summary | #747 | T012–T013 | P2 |
| 8 | US6 — Registry symlinks | #750 | T014–T016 | P3 |
| 9 | US7 — Config/env docs | #751–753 | T017–T023 | P3 |
| 10 | US8 — TUI intermittent $0 | #723 | T024–T027 | P3 |
| 11 | US9 — Force reinstall sync | #754 | (deferred) | P4 |
| 12 | Polish | — | T028–T030 | — |

**Total actionable tasks**: 30 (T001–T030)
**Deferred tasks**: US9 (blocked on policy pack setup feature)
**Parallel opportunities**: US1–US8 all independent; within US7 three streams can run in parallel

## Notes

- [P] tasks touch different files with no dependencies on incomplete tasks
- [Story] labels map each task to a specific user story for traceability
- All test tasks must be written AND confirmed failing before implementation begins
- Tests live co-located with source (`*_test.go`) — the `test/unit/` directory is
  deprecated and must not be used
- `make lint` must pass after every task or logical group; never skip it
- `git add` and `git commit` are the user's responsibility — Claude does not commit
