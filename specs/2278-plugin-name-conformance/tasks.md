---
description: "Task list for Plugin Name Conformance (#2278, #2206)"
---

# Tasks: Plugin Name Conformance

**Input**: Design documents from `specs/2278-plugin-name-conformance/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli-behavior.md, quickstart.md

**Tests**: Mandatory (Constitution Principle II). Each story writes its tests
first and confirms they fail before implementing. Testify only (`require` /
`assert`); `t.Parallel()` except where noted (CLAUDE.md, Parallel Tests).

**Completeness**: No stubs or TODOs (Principle VI). Cross-repo stories
(US6, US7) are tracked as issues and PRs in their own repositories.

**Web parity**: The spec records Web UI Fit as Not applicable (research R13),
so there is no Web Parity phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: US1-US8 from spec.md

---

## Phase 1: Setup

**Purpose**: Baseline before any change.

- [X] T001 Record a baseline in the worktree `../finfocus-2278`: run `go build ./...` and `go test ./internal/router/... ./internal/registry/... ./internal/conformance/... ./internal/cli/... ./internal/engine/... ./internal/pluginhost/...`, and keep the failing-test set (if any) for comparison in T065; do not commit the output

---

## Phase 2: Foundational (blocks every story)

**Purpose**: The shared name rule (research R2, R8) and the install name and aliases on the client (R1, R14).

- [X] T002 [P] Write table-driven tests for `Valid`, `Canonical`, and `FromBinary` in internal/pluginname/pluginname_test.go: `Valid` covers the cases now in `TestIsValidPluginName` (internal/cli/plugin_init_test.go:230); `Canonical` rejects `finfocus-plugin-aws-public`, `AWS-Public`, `aws--public`, `-aws`, accepts `aws-public`, `aws-ce`, `k8s`, and its error names the reason; `FromBinary` handles `finfocus-plugin-aws-public`, `finfocus-plugin-aws-public.exe`, `finfocus-plugin-aws-public-us-east-1` with region `us-east-1`, a region that is not a suffix, and a name without the prefix (returns false)
- [X] T003 [P] Write tests for `(*Client).MatchesName` and `ResolveName` in internal/pluginhost/identity_test.go: empty name never matches; install name, an alias, and reported name each match; a client with no `InstallName` or `Aliases` matches only its reported name; `ResolveName` precedence is install name, then alias, then reported name, returns the match kind (`install`, `alias`, `reported`), and reports `ambiguous` when a different client also matched at a lower precedence; nil and empty slices
- [X] T004 [P] Write a test in internal/pluginhost/host_test.go that a client launched with `WithInstallName(ctx, "aws-public")` has `InstallName == "aws-public"` and one launched without it has an empty `InstallName`; add the same assertion for a pooled holder in internal/registry/registry_pool_test.go (every holder of a shared process carries the install name)
- [X] T005 [P] Create the leaf package internal/pluginname/pluginname.go (standard library only) with `BinaryPrefix = "finfocus-plugin-"`, `Valid` (moved from `cli.IsValidPluginName`, same rule and godoc), `Canonical(name string) error`, and `FromBinary(path, region string) (string, bool)`, so T002 passes
- [X] T006 Make `cli.IsValidPluginName` in internal/cli/plugin_init.go delegate to `pluginname.Valid` (keep the exported function; its test still passes)
- [X] T007 Add `InstallName string` and `Aliases []string` with godoc to `Client` in internal/pluginhost/host.go; set `InstallName` in `launchClient` from `launchInputs.installName` when `in.named` (the registry sets `Aliases`, T032), so T004 passes
- [X] T008 Implement `MatchKind`, `MatchesName`, and `ResolveName` with godoc in internal/pluginhost/identity.go, so T003 passes

**Checkpoint**: `go test ./internal/pluginname/... ./internal/pluginhost/... ./internal/registry/...` passes.

---

## Phase 3: User Story 1 - Routing rules work with the name `plugin list` shows (P1) MVP

**Goal**: Routing entries match a plugin by install name or reported name in `config validate`, the run-time router, and `config routes test` (FR-002, FR-003, FR-004, FR-006).

**Independent Test**: The #2278 reproduction in quickstart.md section 1 gives the same result for `name: aws-public` and `name: finfocus-plugin-aws-public` in `config validate` and `cost actual`. `config routes test` matches the reported spelling once US8 adds the alias.

### Tests for User Story 1 (write first, confirm they fail)

- [X] T009 [P] [US1] Router tests in internal/router/router_test.go with a client `{Name: "finfocus-plugin-aws-public", InstallName: "aws-public"}` and a second client: for each of the two spellings, a feature rule restricts the plugin (automatic pass), a pattern rule selects it (`findClient` path), `ShouldFallback("finfocus-plugin-aws-public")` returns the entry's `fallback: false`, and `hasMatch` does not double-count a pattern-matched client; `UnmatchedEntries()` lists `aws-publik` and nothing else
- [X] T010 [P] [US1] Validation tests in internal/router/validation_test.go: both spellings valid; `aws-publik` is the existing `plugin not found` error; `finfocus-plugin-acme` against a client whose install name and reported name are both `acme` errors with `did you mean "acme"?`; a name that is one client's install name and another's reported name is valid with an ambiguity warning naming both
- [X] T011 [P] [US1] Routes-test CLI tests in internal/cli/config_routes_test.go with a temp `FINFOCUS_HOME` holding fake installs `plugins/aws-public/v0.2.5/finfocus-plugin-aws-public` and `plugins/cloudzero/v0.1.1/finfocus-plugin-cloudzero` (files only, never executed): `aws-public` simulates its features; `aws-publik` is absent from matches and appears in the JSON `warnings` array and the table `Warnings:` block with the `config validate` hint; `finfocus-plugin-acme`, with `acme` installed and no alias, warns with `did you mean "acme"?`; a config with no warnings produces JSON without a `warnings` key (not parallel: sets `FINFOCUS_HOME`)
- [X] T012 [P] [US1] End-to-end CLI test in test/integration/routing_install_name_test.go: build the recorder with `recorderBinary(t)` and install it under a temp `FINFOCUS_HOME` as `plugins/recorder-alt/v0.0.1/finfocus-plugin-recorder-alt` (extend `installRecorder` in test/integration/esc_redaction_test.go, or add a helper beside it, to take the install directory name), so its reported name `recorder` differs from its install name; through `helpers.NewCLIHelper(t)` with `WithEnv`, run `config validate` and `cost projected` on a `test:` resource with routing `name: recorder-alt` and then `name: recorder`, each with a feature rule that excludes ProjectedCosts; assert both spellings validate and both exclude the plugin (no recorder adapter in the output), and that `--adapter recorder-alt` and `--adapter recorder` both return the recorder row; not parallel (writes and executes a binary, sets `FINFOCUS_HOME`) — Done with a different check: with one plugin, an excluded feature falls back to every plugin (the engine asks all clients when the router matches none), so a lone recorder cannot show a feature rule. Feature, pattern, and fallback rules for every spelling are pinned in internal/router/router_identity_test.go; this test pins the real CLI wiring instead: `config validate` for both spellings and a typo, the run-time unmatched-entry warning, and `--adapter` for both spellings and a typo (which asks no plugin)

### Implementation for User Story 1

- [X] T013 [US1] In internal/router/router.go replace the name-keyed `pluginConfig` with `configFor map[*pluginhost.Client]*config.PluginRouting` and `unmatched []string`, built in `NewRouter` with `pluginhost.ResolveName`; switch the automatic pass (`router.go:328`), `findClient`, `hasMatch` (compare client pointers), and `ShouldFallback` (resolve the name, then read `configFor`) to it; add `UnmatchedEntries() []string` with godoc; keep pattern compilation keyed by entry name
- [X] T014 [US1] In internal/router/validation.go replace the `pluginNames` set with `pluginhost.ResolveName`; add the `did you mean "<x>"?` suffix using `pluginname.BinaryPrefix` (FR-006) and the ambiguity warning (FR-003)
- [X] T015 [US1] In internal/cli/config_routes.go read installed plugins with `registry.NewDefault().ListLatestPlugins()` (no launch); treat an entry as installed when it equals an install name; build synthetic clients only for installed entries, with `InstallName` set; add `Warnings []RouteWarningOutput` (`json:"warnings,omitempty"`) to the routes-test output and a table `Warnings:` block, with the `did you mean` hint from T014; update the command's `Long` help to say reported names are confirmed by `config validate`
- [X] T016 [US1] Add a case in internal/cli/config_validate_test.go that the ambiguity warning from `collectRoutingIssues` (internal/cli/config_validate.go) prints as a warning, not an error, and that its plugin-load fallback (clients with only `Name`) is unchanged

**Checkpoint**: T009-T012 pass.

---

## Phase 4: User Story 2 - `--adapter` selects a plugin by its install name (P1)

**Goal**: `--adapter` accepts either name for every command and `--web` (FR-002, research R5).

**Independent Test**: `cost actual --adapter aws-public` and `--adapter finfocus-plugin-aws-public` return the same rows.

### Tests for User Story 2 (write first, confirm they fail)

- [X] T017 [P] [US2] Registry tests in internal/registry/registry_open_test.go: `Open(ctx, "aws-public")` opens only that install; `Open(ctx, "some-reported-name")` (no install name or alias) opens every installed plugin; use the existing fake launcher in that file
- [X] T018 [P] [US2] Engine tests: in the projected and actual adapter-filter tests (find them with `grep -rn 'Adapter:' internal/engine/*_test.go`) add cases where `request.Adapter` is the install name of a client whose reported name differs, for `engine.go:1374`, `engine.go:1739`, and `actual_batch.go:90`; assert the result's `Adapter` is still the reported name (FR-008); in internal/engine/estimate_selection_test.go add the same for the pricing-mode plugin match (`estimate_selection.go:51`) — Tests in internal/engine/adapter_identity_test.go; the three filters are all actual-cost paths (`GetActualCostWithOptions`, `getActualCostForResource`, `actualBatchPlugins`)
- [X] T019 [P] [US2] CLI test in internal/cli/common_execution_test.go: with fake clients, `openPlugins` (or the helper it calls after opening) logs a warning with `adapter` when no client `MatchesName` the `--adapter` value, and logs nothing when one does (FR-024)

### Implementation for User Story 2

- [X] T020 [US2] In internal/registry/registry.go `Open`, filter by `onlyName` only when it equals an install name (US8 adds aliases, T033); otherwise open all plugins and log at debug (`component=registry`, `plugin_filter`) that the adapter will be matched against reported names
- [X] T021 [P] [US2] Replace `client.Name != request.Adapter` / `m.Client.Name == request.Adapter` with `MatchesName` in internal/engine/engine.go (two sites) and internal/engine/actual_batch.go; leave the cache filter at `engine.go:4531` on the raw string (research R5)
- [X] T022 [P] [US2] Use `MatchesName` for the pricing-mode plugin match in internal/engine/estimate_selection.go — Done as a check, not a change: `mode.Plugin` comes from the engine's own pricing-spec discovery (it is the client's reported name inside an opaque pricing-mode ID), never a name the user types, so the exact comparison stays
- [X] T023 [US2] In internal/cli/common_execution.go `openPlugins`, after a successful open with a non-empty adapter, warn (`component=cli`, `adapter`, message from contracts/cli-behavior.md) when no opened client `MatchesName` it
- [X] T024 [US2] Search for any other comparison of a user-supplied plugin name with `client.Name` (`grep -rn 'client.Name ==\|Client.Name ==\|\.Name != ' internal --include=*.go`, excluding tests and output-only uses such as `Adapter:` fields and log fields) and move each one to `MatchesName` or `ResolveName`; record the list checked in the PR description — Found and moved to `MatchesName`: `findScorerClient` (`scoring.plugin`, internal/cli/cost_recommendations_scoring.go) and `selectCapablePlugin` (`--usage-source`/`--allocator`, internal/cli/cost_cluster.go). The remaining `.Name ==` uses are internal bookkeeping (rate-limit maps, batch grouping) or compare config/catalog entries

**Checkpoint**: T017, T018, and T019 pass; quickstart section 1 `--adapter` rows match.

---

## Phase 5: User Story 8 - A plugin's former name keeps working after a rename (P2)

**Goal**: Catalog `aliases` keep `finfocus-plugin-aws-public` and `finfocus-plugin-cloudzero` working in routing and `--adapter` whatever the installed plugin reports (FR-019 to FR-023, research R14). Ships before any plugin rename (US6).

**Independent Test**: With a fake aws-public that reports `aws-public`, routing and `--adapter` spelled `finfocus-plugin-aws-public` behave like `aws-public`; `config validate` and `config routes test` show the former-name warning.

### Tests for User Story 8 (write first, confirm they fail)

- [X] T025 [P] [US8] Catalog tests in internal/registry/entry_test.go (or the existing registry JSON test, internal/registry/registry_json_test.go): `aliases` parses; `ValidateRegistryEntry` rejects an alias failing `pluginname.Valid`; a catalog where an alias equals another entry's name, or two entries share an alias, fails validation naming both; the embedded catalog lists `finfocus-plugin-aws-public` for aws-public and `finfocus-plugin-cloudzero` for cloudzero and passes validation; a `FINFOCUS_CATALOG_FILE` overlay's aliases are returned by `GetPlugin` (not parallel when it sets the variable)
- [X] T026 [P] [US8] Registry open tests in internal/registry/registry_open_test.go: an opened aws-public client carries `Aliases == ["finfocus-plugin-aws-public"]`; a plugin with no catalog entry carries none; `Open(ctx, "finfocus-plugin-aws-public")` opens only aws-public
- [X] T027 [P] [US8] Router and validation tests in internal/router/router_test.go and internal/router/validation_test.go with a client `{Name: "aws-public", InstallName: "aws-public", Aliases: ["finfocus-plugin-aws-public"]}` (the post-rename state): an entry spelled with the alias applies feature rules, pattern rules, and fallback; `ValidateRoutingConfig` returns valid with the warning `"finfocus-plugin-aws-public" is a former name of "aws-public"; use "aws-public"`; with the pre-rename client (`Name: "finfocus-plugin-aws-public"`) the warning is `"finfocus-plugin-aws-public" will be retired; use "aws-public"`; an alias never wins over another client's install name
- [X] T028 [P] [US8] Routes-test tests in internal/cli/config_routes_test.go: with the fake installs from T011, `finfocus-plugin-aws-public` is simulated as aws-public and the retiring-name warning appears in table and JSON
- [X] T029 [P] [US8] Engine test: `--adapter finfocus-plugin-aws-public` selects the post-rename client through `MatchesName` (extend a T018 case)

### Implementation for User Story 8

- [X] T030 [US8] Add `Aliases []string` (`json:"aliases,omitempty"`, godoc: former names that keep matching after a rename) to `RegistryEntry` in internal/registry/entry.go; validate each alias with `pluginname.Valid` in `ValidateRegistryEntry`; add catalog-wide uniqueness of names and aliases where the catalog (embedded and overlay) is loaded in internal/registry/embed.go
- [X] T031 [US8] Add `"aliases": ["finfocus-plugin-aws-public"]` to the aws-public entry and `"aliases": ["finfocus-plugin-cloudzero"]` to the cloudzero entry in internal/registry/registry.json
- [X] T032 [US8] In internal/registry/registry.go `openPlugin`, set `client.Aliases` from `GetPlugin(plugin.Name)` when the catalog has the entry (ignore not-found)
- [X] T033 [US8] Extend the `Open` filter in internal/registry/registry.go: `onlyName` that equals a catalog alias of an installed plugin opens only that plugin
- [X] T034 [US8] In internal/router/validation.go emit the alias warning when `ResolveName` matched `via` alias (FR-022): the retiring wording when the alias equals the client's reported name, otherwise the former-name wording; no run-time warning in internal/cli/common_execution.go for alias use
- [X] T035 [US8] In internal/cli/config_routes.go treat an entry equal to a catalog alias of an installed plugin as installed, set `Aliases` on its synthetic client, and add the retiring-name warning (routes test has no reported names)

**Checkpoint**: T025-T029 pass; the contract's "after Phase 4" statement holds with a fake renamed plugin.

---

## Phase 6: User Story 3 - Silent mismatches become visible (P2)

**Goal**: Warnings for unmatched routing entries at run time and for plugins whose names differ (FR-005, FR-007).

**Independent Test**: With aws-public v0.2.5 and `name: aws-publik`, a cost run logs both warnings, each once.

### Tests for User Story 3 (write first, confirm they fail)

- [X] T036 [P] [US3] Registry test in internal/registry/registry_test.go: opening a fake plugin whose reported name differs from its install directory logs one warn-level line with `install_name` and `reported_name`; opening it again in the same process logs nothing more; a matching plugin logs nothing; call `ResetNameMismatchWarningsForTest` and mark the test `//nolint:paralleltest // shared warning set`
- [X] T037 [P] [US3] CLI test in internal/cli/common_execution_test.go (or the existing router-creation test file) that `createRouterForEngine` with real clients and an entry `aws-publik` logs the warn line from contracts/cli-behavior.md, and that an entry resolved through an alias logs nothing

### Implementation for User Story 3

- [X] T038 [US3] In internal/registry/registry.go `openPlugin`, after a successful open, warn once per process when `client.InstallName != client.Name` (package-level `sync.Map` keyed `install|reported`; exported `ResetNameMismatchWarningsForTest` with godoc saying it is for tests)
- [X] T039 [US3] In internal/cli/common_execution.go `createRouterForEngine`, after `router.NewRouter` succeeds, log one warning per `UnmatchedEntries()` entry with `component=cli`, `operation=create_router`, `plugin`

**Checkpoint**: T036-T037 pass; quickstart section 1 shows both warnings.

---

## Phase 7: User Story 4 - Plugin authors see name drift in conformance runs (P2)

**Goal**: Three protocol name checks with a `warn` status that never changes the exit code (FR-010 to FR-013, research R9, R10).

**Independent Test**: quickstart.md section 2 against aws-public v0.2.5: two warnings, one pass, exit 0.

### Tests for User Story 4 (write first, confirm they fail)

- [X] T040 [P] [US4] Status and summary tests in internal/conformance/suite_test.go and internal/conformance/certification_test.go: `calculateSummary` counts `warn` in `Warnings`, not in `Errors`; certification stays `Certified` with only warnings and its markdown report lists `- Warnings: N`
- [X] T041 [P] [US4] Reporter tests in internal/conformance/reporter_test.go: table shows `⚠` and the message and prints `Warnings: N` only when N > 0 (output with zero warnings is unchanged); JSON has `"status": "warn"` and `summary.warnings`; JUnit emits a passing testcase with `<system-out>WARNING: ...</system-out>` and does not add to `failures`
- [X] T042 [P] [US4] Identity tests in internal/conformance/identity_test.go using temp directories: an explicit name wins (`Source == "flag"`); `<root>/aws-public/v0.2.5/finfocus-plugin-aws-public` gives `aws-public` from `install-layout`; `<root>/aws-public/0.2.5/...` (no `v`) also works; `finfocus-plugin-aws-public-us-east-1` next to a `plugin.metadata.json` with region `us-east-1`, outside a version directory, gives `aws-public` from `binary-name`; a binary named `my-plugin` in a plain directory is unresolved with a reason
- [X] T043 [P] [US4] Check tests in internal/conformance/protocol_test.go with fake clients: `Name_MatchesInstallName` pass, warn (message names expected, actual, source), skip (reason); `Name_MatchesPluginInfo` pass, warn, skip on `Unimplemented`; `Name_IsCanonicalForm` pass and warn with the reason from `pluginname.Canonical` — A check that cannot run (no expected name, GetPluginInfo unimplemented or failing, Name failing) warns with what is missing instead of skipping: certification treats every skip as unverified, so a skip would make a warn-only check block `plugin certify`. Spec FR-011, Story 4, and the contract updated to match
- [X] T044 [P] [US4] CLI tests in internal/cli/plugin_conformance_test.go: `--expected-name` is registered and reaches `SuiteConfig.ExpectedName`; `checkResults` returns nil for a report with only warnings

### Implementation for User Story 4

- [X] T045 [US4] In internal/conformance/types.go add `StatusWarn`, `Summary.Warnings` (`json:"warnings"`), `SuiteConfig.ExpectedName`, `PluginIdentity`, and `TestContext.Identity`, all with godoc
- [X] T046 [US4] Create internal/conformance/identity.go with `ResolveExpectedName(binPath, explicit string) PluginIdentity` per research R10, using the registry's semver parser for version directories, `registry.ReadPluginMetadata` for the region, and `pluginname.FromBinary`
- [X] T047 [US4] Add the three checks to internal/conformance/protocol.go and register them in `registerDefaultTests` in internal/conformance/suite.go (category protocol, `RequiredMethods: methodName`); in `Suite.Run` resolve the identity once and pass it to every `TestContext` (follow how `runner.probe` reaches tests in internal/conformance/runner.go)
- [X] T048 [US4] Update `calculateSummary` in internal/conformance/suite.go, `getStatusIcon` and the table, JSON, and JUnit writers in internal/conformance/reporter.go, the status log switch in internal/conformance/runner.go, and certification in internal/conformance/certification.go per research R9
- [X] T049 [US4] Add `--expected-name` to `NewPluginConformanceCmd` in internal/cli/plugin_conformance.go and pass it into `buildSuiteConfig`; leave `checkResults` ignoring warnings
- [X] T050 [US4] Run the suite against the installed aws-public v0.2.5 binary (quickstart.md section 2) and confirm two warnings, one pass, exit 0; paste the summary into the PR description — Result on 2026-11-20 against ~/.finfocus/plugins/aws-public/v0.2.5: `Name_MatchesInstallName` warn (`plugin reports "finfocus-plugin-aws-public"; its install name is "aws-public" (from install-layout)`), `Name_MatchesPluginInfo` pass, `Name_IsCanonicalForm` warn; summary `{total: 6, passed: 4, failed: 0, skipped: 0, errors: 0, warnings: 2}`; exit 0; no orphaned plugin processes

**Checkpoint**: T040-T044 pass; quickstart section 2 matches.

---

## Phase 8: User Story 5 - Plugin-dev guidance states the naming convention (P3)

**Goal**: A naming section in finfocus-plugin-dev, a conforming `plugin init` template, and a contract test (FR-014, FR-015, research R11).

**Independent Test**: `go test -run TestFinfocusPluginDevSkill ./internal/cli/` passes, and fails when a cited check ID is renamed.

### Tests for User Story 5 (write first, confirm they fail)

- [X] T051 [P] [US5] Create internal/cli/finfocus_plugin_dev_skill_test.go following internal/cli/plugin_manage_skill_test.go: every `Name_*` check ID cited in agent-skills/finfocus-plugin-dev/SKILL.md and references/naming.md exists in `conformance.NewSuite(...).GetTestCases()`; `--expected-name` is a flag on `NewPluginConformanceCmd()`; the `finfocus-plugin-dev.skill` archive holds exactly `SKILL.md` and `references/*` with matching bytes
- [X] T052 [P] [US5] Add a test in internal/cli/plugin_init_test.go that the generated Go calculator source declares `const PluginName = "<name>"` and uses `PluginName` (not a literal) in `NewBasePlugin` and the plugin info `Name`

### Implementation for User Story 5

- [X] T053 [US5] Add `## Naming conformance` to agent-skills/finfocus-plugin-dev/SKILL.md after `## Testing loop`: the rule (install name = binary suffix = `Name()` = plugin info name = `Source` = logger `plugin_name`), one exported `PluginName` constant, the three check IDs, `--expected-name`, that the checks warn and do not fail, that a rename needs a catalog alias for the old name, and a link to references/naming.md
- [X] T054 [US5] Create agent-skills/finfocus-plugin-dev/references/naming.md: where the constant must be used (`NewBasePlugin`, `PluginInfo`/`NewPluginInfo`, `NewPluginLogger`, every result `Source`, cache directories), grep recipes for stray literals (`grep -rn '"<name>"' --include=*.go . | grep -v _test`), how core treats a mismatch (install name, alias, or reported name accepted; one warning), how to rename safely (add the old name to the catalog entry's `aliases` in finfocus first), and how to fix each warning; link it from references/testing.md's Conformance section
- [X] T055 [US5] Update the Go calculator template in internal/cli/plugin_init.go to declare `const PluginName = "{{NAME}}"` and use it in `NewBasePlugin` and the plugin info `Name`; check the generated test template (`TestPluginName`, `resp.Name` assertions) still compiles against it
- [X] T056 [US5] Rebuild agent-skills/finfocus-plugin-dev/finfocus-plugin-dev.skill from `SKILL.md` and `references/` (same layout as the existing archive) and run markdownlint on the skill files

**Checkpoint**: T051-T052 pass.

---

## Phase 9: User Story 6 - Plugins report their install name (P3, cross-repo)

**Goal**: aws-public, aws-ce, and vantage define their name once and report the install name (FR-016). Each item waits for owner approval before any issue or PR is opened (standing guidance for plugin repos), and the aws-public rename waits for a core release that carries its alias (US8).

**Independent Test**: Each fixed plugin passes the three name checks with zero warnings.

- [X] T057 [US6] Ask the owner to approve filing issues for aws-ce and vantage and opening PRs in plugin repositories; record the answer in this task; once approved, add every cross-repo issue (including rshade/finfocus-plugin-aws-public#502 and the finfocus-spec issue for US7) to GitHub Project #3 (constitution, Multi-Repo Governance) — Approved 2026-11-20: file the issues and open PRs in aws-ce, vantage, and aws-public (aws-public's PR held until a core release carries the alias); file the three optional rename issues; implement the finfocus-spec change in a worktree with no PR
- [X] T058 [P] [US6] rshade/finfocus-plugin-aws-public#502, after a core release with the US8 alias: add an exported `PluginName = "aws-public"` and return it from `Name()` in internal/plugin/plugin.go:286 and internal/router/router.go:71 (main and per-region binaries); release notes say `finfocus-plugin-aws-public` keeps working in routing and `--adapter` through the core catalog alias (from that core version on) and recommend `aws-public`; verify with `finfocus plugin conformance` (zero name warnings) and the quickstart alias run — Already done upstream: rshade/finfocus-plugin-aws-public#503 (merged 2026-11-20) closed #502; `internal/pluginid.Name = "aws-public"` is used by both the main and per-region binaries. Not released yet: release-please PR #481 (0.3.0) must wait for a finfocus release that carries the catalog alias, or configs spelled `finfocus-plugin-aws-public` stop matching
- [X] T059 [P] [US6] finfocus-plugin-aws-ce: file an issue, then use `pluginName` for `pluginsdk.NewPluginLogger` in cmd/plugin/main.go:25 (export it if `main` cannot reach it) and for `Source` in internal/pricing/calculator.go:678; verify with conformance — rshade/finfocus-plugin-aws-ce#114, PR #115 (merged): exported `PluginName` now feeds the logger, the cache directory, and the `Source` field (now at calculator.go:712); `make test` and `make lint` pass; the three `Name_*` checks pass
- [X] T060 [P] [US6] finfocus-plugin-vantage: file an issue, then add an exported `PluginName` and return it from `Name()` in internal/plugin/plugin.go:57; verify with conformance — rshade/finfocus-plugin-vantage#159, PR #160 (merged): `PluginName = "vantage"` feeds `Name()`, `Source`, and a shared `Plugin.Info()`; the served `GetPluginInfo` name changes from `finfocus-plugin-vantage` to `vantage` (it disagreed with `Name()`; core matches on `Name()`, which is unchanged); `make test` and `make lint` pass; the three `Name_*` checks pass
- [X] T061 [P] [US6] finfocus-plugin-opencost, -flexera, -cloudzero: file one low-priority issue each to rename the private `pluginName` constant to the exported `PluginName` convention (no behavior change) — Filed rshade/finfocus-plugin-opencost#164 (logger and one `Source` repeat the literal), rshade/finfocus-plugin-cloudzero#32 (logger repeats it), rshade/finfocus-plugin-flexera#33 (convention only, no drift). PRs followed on 2026-11-21: opencost #165, cloudzero #35, flexera #34. Flexera's conformance run could not start the binary: its `main` has no `--port` flag and always listens on 127.0.0.1:50051, so core cannot launch it (pre-existing, unrelated to the rename). Adding the issues to Project #3 failed: the gh token lacks the `project` scope

---

## Phase 10: User Story 7 - Plugin unit tests catch drift without core (P3, cross-repo)

**Goal**: finfocus-spec in-process conformance accepts an expected name and checks `Name()` against plugin info (FR-017).

**Independent Test**: In finfocus-spec, a test plugin with mismatched names is reported by `RunBasicConformance`.

- [X] T062 [US7] finfocus-spec: write tests first in sdk/go/pluginsdk/conformance_test.go for a plugin whose `Name()` differs from its `PluginInfo().Name` and for `WithExpectedName("x")` against a plugin named `y`; then add `WithExpectedName(name string) ConformanceOption` and the checks to sdk/go/pluginsdk/conformance.go (and to sdk/go/testing/conformance.go if the basic level is defined there); update sdk/go/pluginsdk/README.md (conformance options and example) in the same PR (Principle IV); release as an additive version — Implemented in rshade/finfocus-spec#682 (branch `feat/conformance-expected-name`), opened 2026-11-21 at the owner's request. `pluginsdk.WithExpectedName` plus two Basic `rpc_correctness` checks, `RPCCorrectness_NameMatchesPluginInfo` and `RPCCorrectness_NameMatchesExpected`, wrapping `ErrPluginNameMismatch`; opt-in per the owner (2026-11-20): neither check runs unless the caller passes a non-empty `WithExpectedName`, so a spec bump changes no existing plugin's results or counts; with it, a mismatch is a failed check (the SDK suite has no warning status). READMEs and PLUGIN_DEVELOPER_GUIDE updated. `make test` passes; `make lint`'s Go step fails in the environment (pinned golangci-lint 2.13.2 cannot typecheck go1.27.4's stdlib, in an untouched file); golangci-lint 2.15.0 reports nothing in the changed files. Still open: the SDK release, then T063-T064
- [ ] T063 [US7] finfocus core follow-up branch: land the finfocus-spec bump alone as `deps: bump finfocus-spec to vX.Y.Z` (root and every nested plugin `go.mod`); add the hop in internal/pluginupgrade/hops.go and agent-skills/finfocus-plugin-upgrade/references/to-vX.Y.Z.md describing `WithExpectedName`; add its row to that skill's SKILL.md and rebuild its `.skill` archive
- [ ] T064 [US7] Update agent-skills/finfocus-plugin-dev/references/testing.md's Conformance section with `pluginsdk.WithExpectedName(PluginName)` and rebuild finfocus-plugin-dev.skill (re-run T051's test)

---

## Phase 11: Polish & Cross-Cutting Concerns

- [X] T065 [P] Update docs/src/content/docs/guides/routing.md (`name` field, around line 173): the install name from `finfocus plugin list` is recommended, the reported name in the Adapter column is also accepted, former names listed by the catalog keep working with a `config validate` warning, which says a name will be retired while the plugin still reports it (as aws-public does today); mention the unmatched-entry warning
- [X] T066 [P] Update docs/src/content/docs/plugins/plugin-development.md and docs/src/content/docs/plugins/certification.md: the three name checks, `--expected-name`, the `warn` status (exit code unchanged, certification not blocked), the `PluginName` convention, and that renaming a published plugin needs a catalog alias for the old name
- [X] T067 Run `make docs-cli` to regenerate docs/src/content/docs/reference/cli/ for the new flag and help text; confirm `TestCommittedReferenceIsCurrent` passes
- [X] T068 [P] Update CLAUDE.md (Router, Registry, and Plugin Host sections): a plugin has an install name, catalog aliases, and a reported name; every user-supplied plugin name goes through `pluginhost.MatchesName`/`ResolveName` (precedence install, alias, reported); routes test matches install names and aliases without launching; never put a plugin's old name in Go code, add it to `aliases`; conformance has a `warn` status that never changes the exit code
- [X] T069 Run `go vet ./...`, `make lint` (extended timeout), `make test`, and markdownlint on changed markdown; compare failures with the T001 baseline; run `go test -cover` on internal/pluginname, internal/pluginhost, internal/router, internal/registry, internal/conformance, and internal/cli, and record the figures in the PR description (80% overall, 95% for the router and plugin host, Principle II) — 2026-11-20: `go vet ./...` clean; `make test` passes (no failures; the T001 baseline was clean once run without a `FINFOCUS_HOME` override). `make lint` passes every step except `lint-vscode-extension`, which fails before linting on the local pnpm corepack shim (`Cannot find module .../corepack/.../pnpm.cjs`, a known local issue in CLAUDE.md; this branch does not touch the extension); run with `make -o lint-vscode-extension lint`. Coverage: pluginname 100%, pluginhost 91.2%, router 92.0%, registry 84.3%, conformance 72.9% (65.9% on main; the new name checks are 78-100% per function)
- [X] T070 Run quickstart.md sections 1-3 with the built binary in a scratch `FINFOCUS_HOME`, check for orphaned `finfocus-plugin-aws-public-*` processes afterwards (CLAUDE.md, E2E), and record the results in the PR description — Results 2026-11-20, scratch `FINFOCUS_HOME` with aws-public v0.2.5 and cloudzero v0.1.2, state with one t3.micro in us-west-2 created three days earlier, routing `{name, features: [ProjectedCosts]}` plus `{cloudzero, [ActualCosts]}`: `aws-public` and `finfocus-plugin-aws-public` both validate (the second with `"finfocus-plugin-aws-public" will be retired; use "aws-public"` in validate and routes test), both route ProjectedCosts to aws-public and ActualCosts to cloudzero, and `cost actual` answers from `estimate`, not aws-public; `aws-publik` fails validate (exit 2), routes test warns and simulates nothing for ProjectedCosts, `cost actual` falls back to aws-public, and the log has the unmatched-entry and name-mismatch warnings. `cost projected --adapter aws-public` and `--adapter finfocus-plugin-aws-public` both return aws-public at $7.592/month; `--adapter aws-publik` asks no plugin. Section 2 is T050; section 3 is T051. No orphaned plugin processes
- [ ] T071 After US6 lands, run `finfocus plugin conformance --category protocol` against the latest release of every plugin in internal/registry/registry.json (including jev and azure-public) and record each plugin's name-check results for SC-005; file an issue in any plugin that still warns
- [X] T072 Run a local review with the `local-cli-review` skill before opening the PR; verify findings before fixing — 2026-11-20 on the uncommitted worktree: codex ok (3 findings), grok timed out at 900 s, plus an own pass. All three codex findings confirmed and fixed with tests: (1) a routing entry written with a reported name now restricts every plugin reporting it, as before (`resolveEntries`); (2) `scoring.plugin`, `--usage-source`, and `--allocator` resolve with `ResolveName` so the install name wins; (3) the name checks share one deadline at 3/4 of the test timeout, so a stalled `GetPluginInfo` warns instead of timing out into an error. After the rebase onto a008af08, three more codex passes found four more, all confirmed and fixed with tests: (4) router matches stay unique by reported name, since recommendation targets, batches, and rate limits group by it; (5) `--usage-source`/`--allocator` resolve against every plugin before checking capability; (6) the engine resolves `--adapter` once per request (`adapterFilter`) so install-name precedence holds inside the engine too; (7) two routing entries for one plugin: only the later applies in both passes, and `config validate` warns. Final codex pass: no findings. Custom registries now reserve built-in aliases (`builtinNames`)

---

## Dependencies & Execution Order

### Phase dependencies

- Setup (T001) → Foundational (T002-T008) → every story.
- US1 and US2 share the foundation and touch different files; they can run in
  parallel.
- US8 depends on US1 (router and validation, T013-T015) and US2 (registry
  filter, T020).
- US3 depends on US1 (`UnmatchedEntries`, T013) for T039, and on T007 for T038.
- US4 depends only on Foundational (`pluginname`), independent of US1-US3 and US8.
- US5 depends on US4 (check IDs and flag it cites).
- US6 depends on a core release containing US8 (so the aws-public rename
  breaks no config) and on owner approval (T057).
- US7's finfocus-spec change depends on nothing in core; T063-T064 follow its
  release.
- Polish depends on the stories it documents; T071 also waits for US6.

### Within each story

Tests first and failing → types → logic → CLI wiring → manual check.

## Parallel Opportunities

- Foundational: T002, T003, T004 together; then T005 with T007 (different packages).
- US1 tests: T009-T012 together.
- US2: T017, T018, and T019 together; T021 with T022.
- US8 tests: T025-T029 together.
- US4 tests: T040-T044 together (separate files).
- US6: T058-T061 in parallel once approved (separate repositories).
- Polish: T065, T066, T068 together.

### Example: User Story 8 tests

```text
T025 internal/registry/entry_test.go (catalog aliases and validation)
T026 internal/registry/registry_open_test.go (client aliases, adapter filter)
T027 internal/router/router_test.go + validation_test.go (post-rename client)
T028 internal/cli/config_routes_test.go (alias simulated, warning)
T029 engine adapter test (alias through MatchesName)
```

## Implementation Strategy

### MVP

Phases 1-5 (Foundational, US1, US2, US8): fixes #2278 and #2206 for users with
no plugin release, and makes the later aws-public rename safe. Validate with
quickstart section 1, then stop and review.

### Incremental delivery

1. MVP (US1 + US2 + US8) → one core PR, `fix(router)`.
2. US3 warnings → can join the MVP PR or stack on it.
3. US4 conformance checks → stacked PR, `feat(conformance)`.
4. US5 guidance and template → stacked PR.
5. US6 plugin PRs after the core release with aliases ships (with approval).
6. US7 finfocus-spec release, then the `deps:` bump PR in core.

Stack the core PRs (each branched from the previous, base set to it) and
retarget a child to `main` before its base merges (CLAUDE.md, Stacked PRs).
