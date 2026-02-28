# Tasks: Config Routes CLI Commands

**Input**: Design documents from `/specs/604-config-routes-cli/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create source files, define shared output types, register parent command

- [x] T001 Create `internal/cli/config_routes.go` with package declaration, imports, and `NewConfigRoutesCmd()` parent command (Use: "routes", Short: "Plugin routing commands", no RunE - delegates to subcommands)
- [x] T002 Define output struct types in `internal/cli/config_routes.go`: `RoutesListOutput` (Mode, ConfigPath, Source, Rules), `RouteRuleOutput` (Plugin, Priority, Features, Patterns, Fallback), `RoutesTestOutput` (ResourceType, Region, Provider, Mode, Matches, Features), `RouteMatchOutput` (Rank, Plugin, Priority, MatchReason, Source, Fallback) with JSON tags per `specs/604-config-routes-cli/data-model.md`
- [x] T003 Register `NewConfigRoutesCmd()` in `newConfigCmd()` in `internal/cli/root.go` by adding it to the existing `cmd.AddCommand()` call on line 308-311

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared helper for config loading and source detection used by both list and test commands

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Implement `loadRoutingContext()` helper in `internal/cli/config_routes.go` that: (1) calls `config.New()` to load effective config, (2) determines config source ("project" or "global") by checking `config.GetResolvedProjectDir()`, (3) returns `(cfg *config.Config, source string, err error)` for reuse by list and test commands
- [x] T005 Create `internal/cli/config_routes_test.go` with package declaration (`package cli_test`), test imports (testing, testify require/assert, bytes, encoding/json, cobra), and a `newTestConfigRoutesCmd()` test helper that creates a minimal cobra command tree (`config routes list/test`) for executing in tests

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - View Routing Configuration (Priority: P1) MVP

**Goal**: Users can view the effective routing configuration in a formatted table by running `finfocus config routes list`, replacing manual YAML file reading

**Independent Test**: Run `finfocus config routes list` with a config containing routing rules and verify the table shows PRIORITY, PLUGIN, FEATURES, PATTERNS, FALLBACK columns sorted by priority descending. Run with no routing config and verify "automatic mode" message. Run with `--output json` and verify valid JSON output.

### Tests for User Story 1 (MANDATORY - TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T006 [US1] Write `TestConfigRoutesListTable` in `internal/cli/config_routes_test.go`: test with a config containing 3 plugins (aws-public priority 10 with ProjectedCosts + glob pattern, aws-ce priority 5 with ActualCosts/Recommendations + glob pattern + fallback, recorder priority 1 with no features/patterns + fallback). Assert table output contains correct headers (PRIORITY, PLUGIN, FEATURES, PATTERNS, FALLBACK), correct row data sorted by priority descending, and source path line
- [x] T007 [US1] Write `TestConfigRoutesListAutomatic` in `internal/cli/config_routes_test.go`: test with nil routing config. Assert output contains "automatic" or "No routing configuration" message indicating provider-based routing is active
- [x] T008 [US1] Write `TestConfigRoutesListJSON` in `internal/cli/config_routes_test.go`: test with configured routing, assert output unmarshals to valid `RoutesListOutput` struct with mode="configured", non-empty rules array matching config, and config_path set
- [x] T009 [US1] Write `TestConfigRoutesListAutomaticJSON` in `internal/cli/config_routes_test.go`: test with nil routing config and `--output json`, assert output unmarshals to `RoutesListOutput` with mode="automatic" and empty rules array
- [x] T010 [US1] Write `TestConfigRoutesListEmptyPlugins` in `internal/cli/config_routes_test.go`: test with routing config containing zero plugins. Assert table output shows headers but no data rows, with a note about no plugins configured
- [x] T010a [US1] Write `TestConfigRoutesListProjectLocal` in `internal/cli/config_routes_test.go`: test with a project-local config override (set `config.GetResolvedProjectDir()` to a temp dir containing a config with routing rules). Assert output shows source as "project" and displays the project-local config path, verifying the two-tier config resolution behavior described in spec edge case 4

### Implementation for User Story 1

- [x] T011 [US1] Implement `NewConfigRoutesListCmd()` in `internal/cli/config_routes.go`: cobra command with Use "list", Short description, Long description with examples (per FR-013), `--output` string flag defaulting to `outputFormatTable`, RunE that loads config, checks `cfg.Routing == nil` for automatic mode, and dispatches to table or JSON renderer
- [x] T012 [US1] Implement `renderRoutesListTable()` in `internal/cli/config_routes.go`: uses `tabwriter.NewWriter()` with `tabPadding` (=2), prints title "PLUGIN ROUTING RULES", header row (PRIORITY, PLUGIN, FEATURES, PATTERNS, FALLBACK), separator row, then iterates `cfg.Routing.Plugins` sorted by priority descending. Features display as comma-separated or "(all)" when empty. Patterns display as "type:pattern" or "(all)" when empty. Fallback displays "yes"/"no" (nil defaults to true per `FallbackEnabled()`). Footer shows `Source: {configPath} ({source})`
- [x] T013 [US1] Implement `renderRoutesListJSON()` in `internal/cli/config_routes.go`: builds `RoutesListOutput` struct with mode, config_path, source, and rules array. Each rule maps from `PluginRouting` to `RouteRuleOutput`. Uses `json.MarshalIndent()` with 2-space indent. Writes to `cmd.OutOrStdout()`

**Checkpoint**: At this point, `config routes list` should be fully functional with both table and JSON output

---

## Phase 4: User Story 2 - Test Plugin Selection for a Resource Type (Priority: P2)

**Goal**: Users can simulate plugin selection for a specific resource type by running `finfocus config routes test aws:ec2:Instance [region]`, seeing the full match chain and per-feature assignments

**Independent Test**: Run `finfocus config routes test aws:ec2:Instance` with routing config and verify output shows ranked plugin matches with priorities and reasons, plus per-feature assignments (ProjectedCosts, ActualCosts, Recommendations, Carbon, DryRun, Budgets). Run with no routing config and verify automatic mode message.

### Tests for User Story 2 (MANDATORY - TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T014 [US2] Write `TestConfigRoutesTestTable` in `internal/cli/config_routes_test.go`: test with 3-plugin config (aws-public priority 10 ProjectedCosts glob:aws:ec2:*, aws-ce priority 5 ActualCosts/Recommendations glob:aws:*, recorder priority 1 fallback). Invoke with resource type "aws:ec2:Instance". Assert output contains match chain table (#, PLUGIN, PRIORITY, MATCH REASON, SOURCE), feature availability section showing all 6 features with correct plugin assignments, and provider "aws" in header
- [x] T015 [US2] Write `TestConfigRoutesTestWithRegion` in `internal/cli/config_routes_test.go`: test with resource type "aws:ec2:Instance" and region "us-east-1". Assert region appears in output header. Assert match chain reflects region-aware selection
- [x] T016 [US2] Write `TestConfigRoutesTestAutomatic` in `internal/cli/config_routes_test.go`: test with nil routing config. Assert output contains automatic mode message explaining all provider-matching plugins would be queried
- [x] T017 [US2] Write `TestConfigRoutesTestJSON` in `internal/cli/config_routes_test.go`: test with configured routing and `--output json`. Assert output unmarshals to valid `RoutesTestOutput` with resource_type, provider, mode="configured", non-empty matches array with correct ranks, and features map with all 6 feature keys
- [x] T018 [US2] Write `TestConfigRoutesTestNoMatches` in `internal/cli/config_routes_test.go`: test with routing config that has patterns not matching the input type (e.g., test "gcp:compute:Instance" against aws-only patterns). Assert output shows "No plugins match" or empty match chain
- [x] T019 [US2] Write `TestConfigRoutesTestMissingArg` in `internal/cli/config_routes_test.go`: test invoking `config routes test` with no arguments. Assert command returns an error about missing resource-type argument

### Implementation for User Story 2

- [x] T020 [US2] Implement `NewConfigRoutesTestCmd()` in `internal/cli/config_routes.go`: cobra command with Use "test", Args requiring at least 1 arg (resource type) with optional second arg (region), Short/Long descriptions with examples, `--output` flag, RunE that loads config, extracts provider via `router.ExtractProviderFromType()`, checks automatic mode, and dispatches to table or JSON renderer
- [x] T021 [US2] Implement `buildSyntheticClients()` in `internal/cli/config_routes.go`: takes `*config.RoutingConfig`, creates `[]*pluginhost.Client` with only `Name` and `Metadata` populated (empty SupportedProviders for pattern-matched plugins). Returns the synthetic client slice for router construction
- [x] T022 [US2] Implement `simulatePluginSelection()` in `internal/cli/config_routes.go`: creates `router.NewRouter()` with `router.WithConfig()` and `router.WithClients()`, constructs a synthetic `engine.ResourceDescriptor{Type: resourceType, Provider: provider, Properties: map[string]interface{}{"region": region}}` (region key must match `ExtractResourceRegion()` lookup order in `internal/router/region.go:19-24`), iterates `router.ValidFeatureNames()` calling `SelectPlugins()` for each, collects unique matches (for match chain) and per-feature best match (highest priority per feature). Returns aggregated results
- [x] T023 [US2] Implement `renderRoutesTestTable()` in `internal/cli/config_routes.go`: prints header with resource type, region (if provided), and provider. Prints match chain table (#, PLUGIN, PRIORITY, MATCH REASON, SOURCE) using tabwriter. Prints "Feature availability:" section with each feature name and assigned plugin (name + priority). Handles "No plugins match" case
- [x] T024 [US2] Implement `renderRoutesTestJSON()` in `internal/cli/config_routes.go`: builds `RoutesTestOutput` struct, populates matches array (ranked) and features map. Uses `json.MarshalIndent()`. Handles automatic mode and no-match cases

**Checkpoint**: At this point, both `config routes list` and `config routes test` should be fully functional

---

## Phase 5: User Story 3 - Machine-Readable Output for Scripting (Priority: P3)

**Goal**: JSON output from both commands is valid, parseable by standard tools (jq), and suitable for CI/CD pipeline integration

**Independent Test**: Run both commands with `--output json`, pipe through `jq .`, and verify clean parsing. Validate JSON schema matches contracts/cli-commands.md specification.

### Tests for User Story 3 (MANDATORY - TDD Required)

- [x] T025 [US3] Write `TestConfigRoutesListJSONContract` in `internal/cli/config_routes_test.go`: validate JSON output matches the contract schema from `contracts/cli-commands.md`. Assert all required fields present (mode, config_path, source, rules), rules array elements have all required fields (plugin, priority, features, patterns, fallback), and empty arrays are `[]` not null
- [x] T026 [US3] Write `TestConfigRoutesTestJSONContract` in `internal/cli/config_routes_test.go`: validate JSON output matches the contract schema. Assert all required fields present (resource_type, provider, mode, matches, features), matches array elements have all required fields (rank, plugin, priority, match_reason, source, fallback), features map has all 6 feature keys, and empty arrays are `[]` not null
- [x] T027 [US3] Write `TestConfigRoutesOutputFormatValidation` in `internal/cli/config_routes_test.go`: test both list and test commands with invalid `--output` value (e.g., "xml"). Assert error message indicates supported formats (table, json)

**Checkpoint**: JSON output is validated against contracts for CI/CD consumption

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Quality gates, edge case coverage, and validation

- [x] T028 Run `make lint` and fix any linting issues in `internal/cli/config_routes.go` and `internal/cli/config_routes_test.go`
- [x] T029 Run `make test` and verify all new tests pass with 80%+ coverage for `internal/cli/config_routes.go`
- [x] T030 Run quickstart.md validation: manually verify the example commands and expected outputs from `specs/604-config-routes-cli/quickstart.md` match actual implementation behavior
- [x] T031 Update project documentation with new `config routes` commands: add `config routes list` and `config routes test` to the CLI reference in `docs/` (if CLI reference page exists) and ensure `CLAUDE.md` CLI section reflects the new subcommands. Per Constitution Principle IV (Documentation Integrity), documentation MUST be updated concurrently with implementation

---

## Dependencies and Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 (Phase 3) can proceed independently after Phase 2
  - US2 (Phase 4) can proceed independently after Phase 2 (does not depend on US1)
  - US3 (Phase 5) depends on both US1 and US2 being complete (validates their JSON output)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - No dependencies on US1 (independent command)
- **User Story 3 (P3)**: Depends on US1 and US2 (validates JSON output from both commands)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Output struct types (Phase 1) before renderers
- Table output before JSON output (table is default)
- Core implementation before edge cases
- Story complete before moving to next priority

### Parallel Opportunities

- T001 and T005 can run in parallel (different files: config_routes.go, config_routes_test.go)
- T006-T010 (US1 tests) can be written in parallel with T014-T019 (US2 tests) since both are test-only additions to the same test file but different test functions
- US1 implementation (T011-T013) and US2 implementation (T020-T024) can proceed in parallel after Phase 2 since they are independent subcommands with different RunE functions

---

## Parallel Example: User Stories 1 and 2

```text
# After Phase 2 completes, both stories can start in parallel:

# Developer A: User Story 1
Task T006: Write TestConfigRoutesListTable in internal/cli/config_routes_test.go
Task T007: Write TestConfigRoutesListAutomatic in internal/cli/config_routes_test.go
Task T011: Implement NewConfigRoutesListCmd() in internal/cli/config_routes.go
Task T012: Implement renderRoutesListTable() in internal/cli/config_routes.go
Task T013: Implement renderRoutesListJSON() in internal/cli/config_routes.go

# Developer B: User Story 2
Task T014: Write TestConfigRoutesTestTable in internal/cli/config_routes_test.go
Task T015: Write TestConfigRoutesTestWithRegion in internal/cli/config_routes_test.go
Task T020: Implement NewConfigRoutesTestCmd() in internal/cli/config_routes.go
Task T021: Implement buildSyntheticClients() in internal/cli/config_routes.go
Task T022: Implement simulatePluginSelection() in internal/cli/config_routes.go
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T005)
3. Complete Phase 3: User Story 1 (T006-T013)
4. **STOP and VALIDATE**: Run `finfocus config routes list` with sample config
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational -> Foundation ready
2. Add User Story 1 -> Test independently -> Deploy/Demo (MVP!)
3. Add User Story 2 -> Test independently -> Deploy/Demo
4. Add User Story 3 -> JSON contract validation -> Deploy/Demo
5. Each story adds value without breaking previous stories

### Single Developer Strategy (Recommended)

Since all code resides in a single file pair:

1. Complete T001-T005 (Setup + Foundational)
2. Write US1 tests (T006-T010), then implement US1 (T011-T013)
3. Write US2 tests (T014-T019), then implement US2 (T020-T024)
4. Write US3 contract tests (T025-T027)
5. Run polish tasks (T028-T030)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Stop at any checkpoint to validate story independently
- All new code in single file pair: `internal/cli/config_routes.go` + `internal/cli/config_routes_test.go`
- One-line change to `internal/cli/root.go` for command registration
- Reuse existing constants: `outputFormatTable`, `outputFormatJSON`, `tabPadding`
- Reuse existing patterns: synthetic `pluginhost.Client` construction from `config_validate.go`
- Router API: `router.NewRouter(WithConfig(), WithClients())`, `SelectPlugins()`, `ValidFeatureNames()`
- Provider extraction: `router.ExtractProviderFromType()`
- Config source: `config.GetResolvedProjectDir()`, `cfg.ConfigPath()`
