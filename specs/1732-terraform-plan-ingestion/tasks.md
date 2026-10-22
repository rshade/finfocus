# Tasks: Terraform Plan Ingestion

**Input**: Design documents from `/specs/1732-terraform-plan-ingestion/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Per Constitution Principle II, write the test in each story before
the implementation that makes it pass. Colocate Go tests with the package.
Do not add files under `test/unit/`. A test that writes a binary and executes
it is not parallel (`//nolint:paralleltest` with that reason).

**Completeness**: Per Constitution Principle VI, no stubs and no TODO comments.

**Documentation**: Per Constitution Principle IV, doc updates are in the
polish phase and must match the flags that landed.

**Organization**: Tasks are grouped by user story. The parser contract is
foundational because every story consumes a validated `ParseResponse`.

**Web Parity**: Required by the owner's follow-up. Existing web handlers
use the shared Terraform pipeline; browser validation and a UX-only
`web-ui` follow-up are tracked below.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an
  incomplete task)
- **[Story]**: User story only (`US1` through `US7`)

## Phase 1: Setup

**Purpose**: Confirm the parser contract is already in the module core uses.

- [X] T001 Confirm `go.mod` requires `github.com/rshade/finfocus-spec` v0.7.9 and that `pluginsdk.CostParseProvider`, `pluginsdk.ValidateParseRequest`, and `pluginsdk.ValidateParseResponse` are the symbols to call. Do not change the version.
- [X] T002 Add `internal/terraform/doc.go` stating that the package runs `terraform` or `tofu` and computes history identity, and that JSON parsing stays in `internal/ingest`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Detection and a `CostParseProvider` that returns a response the SDK validator accepts.

**Critical**: No user story starts until this phase is complete.

### Tests (write first)

- [X] T003 Produce `internal/ingest/testdata/terraform/plan/show.json` by running `tofu show -json` or `tofu show -json` on a plan file, and record that command in `internal/ingest/testdata/terraform/plan/README.md`. The generation step may run init. Product code must not. Do not hand-write attribute names. The existing `test/fixtures/plans/*-plan.json` excerpts are not this fixture. The configuration must plan with no cloud account: use the `aws` provider with `skip_credentials_validation`, `skip_requesting_account_id`, `skip_metadata_api_check`, and fake keys, and record those settings in the README. It must contain create, update, delete, replace, no-op, a data source, a module resource, a sensitive attribute in `before` and `after`, an unknown value inside a list, a null, and one resource each from `random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`, and `cloudflare`. Produce a second fixture with only `resource_drift`.
- [X] T004 [P] Write failing nearer-marker tests in `internal/cli/project_source_test.go`: same directory with `Pulumi.yaml` and a `.tf` file selects Pulumi; `.tf` in the working directory and `Pulumi.yaml` only in a parent selects Terraform; a parent Pulumi project with no `.tf` in the working directory selects Pulumi; neither marker returns an error whose text contains `Pulumi` and `Terraform`; `.tf.json` counts; a `.tf` file only in a subdirectory does not.
- [X] T005 Write failing parser tests in `internal/ingest/terraform_plan_test.go` against the fixture from T003. Build a `ParseRequest` with format `terraform-plan`, input mode content, and one file `plan.json`. Call `Parse`, then require `pluginsdk.ValidateParseResponse` to accept the response. Assert `source_id`, Terraform `resource_type` (not a Pulumi token), camel-cased attributes, `unknown_properties` absent from attributes and without list indexes, a null kept as present, `resource_skipped` INFO for a data source and for each skipped-provider resource, the `cloudflare` resource kept with provider `custom`, the sensitive leaf absent from attributes and old values and absent from `unknown_properties`, the drift-only fixture parsing to an empty successful plan, input over 32 MiB rejected by request validation, and the companion `map[source_id]operation` for create, update, delete, replace, and `same`. A response that fails the validator fails the test.

### Implementation

- [X] T006 [P] Implement current-directory Terraform root detection in `internal/terraform/root.go` (regular files ending in `.tf` or `.tf.json` only).
- [X] T007 Implement the shared nearer-marker detector in `internal/cli/project_source.go`, using `internal/pulumi.FindProject` and `internal/terraform` root detection.
- [X] T008 Implement `pluginsdk.CostParseProvider.Parse` for format `terraform-plan` in `internal/ingest/terraform_plan.go`. Advertise limits format `terraform-plan`, input mode `content` only, and 32 MiB. Validate the request with `pluginsdk.ValidateParseRequest` before parsing and return a response that passes `pluginsdk.ValidateParseResponse`. Keep `map[source_id]operation` and before-values beside the response, not inside it. Reuse snake_case to camelCase and the `terraform:` metadata keys from `internal/ingest/terraform_map.go`. Remove leaves marked in `before_sensitive` and `after_sensitive` from attributes and old values. Take the provider from `resourcetype.ExtractProvider`: keep `aws`, `azure`, `gcp`, `kubernetes`; skip `random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http` with an INFO `resource_skipped`; send every other provider as `custom`. Do not put `terraform:unknownProperties` on the parse response.

**Checkpoint**: Detection and `Parse` pass their tests. No command runs a plan yet.

---

## Phase 3: User Story 1 - Project a Terraform directory with no flags (Priority: P1)

**Goal**: `finfocus cost projected` in a Terraform directory plans and prints a projected cost, including the zero-cost line when nothing changes.

**Independent Test**: In a temp directory with a `.tf` file and no Pulumi project, a fake `terraform` on PATH is invoked with the contract arguments and the command exits 0. A no-change plan prints `No changes. Projected monthly cost is zero.`

### Tests for User Story 1

- [X] T009 [US1] Write failing runner tests in `internal/terraform/runner_test.go` using a fake binary. Assert `plan -input=false -lock=false -no-color -out=<temp>/plan.bin`, then `show -json -no-color` on that file, temp directory mode `0700`, plan file mode `0600`, the plan path is outside the project, the file is removed on success, and `show` stdout is not logged. Mark the test `//nolint:paralleltest` because it writes a binary and executes it. In `internal/cli/cost_projected_terraform_test.go`, assert a no-change plan prints `No changes. Projected monthly cost is zero.` and exits 0.

### Implementation for User Story 1

- [X] T010 [US1] Implement the success path of the plan runner in `internal/terraform/runner.go` (`os.MkdirTemp`, modes, `CommandContext`, both commands, removal on success). Do not classify stderr yet beyond returning the raw failure.
- [X] T011 [US1] In `internal/cli/cost_projected.go`, when `loadProjectedResources` has no input file and `internal/cli/project_source.go` returns Terraform, call the runner, parse with T008, adapt the response, and print `No changes. Projected monthly cost is zero.` when every operation is `same` or the response has no resources.

**Checkpoint**: `cost projected` with no flags projects a Terraform directory. Pulumi auto-detect still calls the existing preview path.

---

## Phase 4: User Story 2 - Hear what to fix when a plan cannot run (Priority: P1)

**Goal**: A missing binary, a directory that is not initialized, a credentials or backend failure, and any other plan failure each exit 1 with one hint and no cost total. Cancellation removes the plan file.

**Independent Test**: Fake binaries in `internal/terraform/runner_test.go` emit the stderr phrases from `research.md`. Each case matches one hint and leaves no plan file in the project.

### Tests for User Story 2

- [X] T012 [US2] Extend `internal/terraform/runner_test.go` so these cases fail first: tofu missing from PATH; stderr asking for init; each credentials phrase in `research.md` (AWS, Azure, GCP, and backend), each from a recorded stderr sample; unrecognized stderr including the last 20 lines; `terraform version -json` below 1.1.0 and `tofu` below 1.6.0; removal of the temp directory on failure; cancellation kills the Unix process group. Add a `//go:build windows` test that cancellation stops the child process, or record in `internal/terraform/doc.go` that it is untested there. Keep `//nolint:paralleltest`.

### Implementation for User Story 2

- [X] T013 [US2] Implement hint kinds and the phrase order in `internal/terraform/hint.go`, and the version check, stderr classification, and Unix process-group kill in `internal/terraform/runner.go`. Hint text is the table in `contracts/cli.md`. Map these failures to exit 1, not `ax.ExitValidation`.

**Checkpoint**: The four hints and cancellation pass without a partial cost result.

---

## Phase 5: User Story 3 - Leave existing Pulumi projects alone (Priority: P1)

**Goal**: A Pulumi project keeps today's preview, export, and `--terraform-state` behavior. ESC loading and analyzer install still search only for a Pulumi project.

**Independent Test**: `cost projected` in a Pulumi fixture does not start `terraform` or `tofu`. `--terraform-state` prices state and does not plan. `esc_source.go` still calls `FindProject`.

### Tests for User Story 3

- [X] T014 [P] [US3] Add Pulumi non-regression tests in `internal/cli/cost_projected_test.go`: a directory with `Pulumi.yaml` does not invoke the Terraform runner; `--terraform-state` still uses the state loader; combining `--terraform-state` with `--pulumi-json` still exits 2.
- [X] T015 [P] [US3] Add a test in `internal/cli/esc_source_test.go` that ESC resolution still uses `internal/pulumi.FindProject` and does not call the Terraform root detector.

### Implementation for User Story 3

- [X] T016 [US3] Keep the Pulumi arm of `loadProjectedResources` in `internal/cli/cost_projected.go` and `resolveResourcesFromPulumi` in `internal/cli/common_execution.go` on the existing preview path when the detector returns Pulumi. Leave `internal/cli/esc_source.go` and `internal/cli/analyzer_install.go` on `FindProject`.

**Checkpoint**: Pulumi fixtures behave as they did before this feature. `--include-workspace` is not wired yet. When it is, US7 must ignore it for Pulumi.

---

## Phase 6: User Story 4 - Price a plan file the person already has (Priority: P2)

**Goal**: `--terraform-plan` prices a `show -json` file, starts no binary, and is mutually exclusive with the other infrastructure inputs.

**Independent Test**: Pass a fixture to `--terraform-plan` and assert no `terraform` process. Pair it with `--pulumi-json`, `--pulumi-state`, or `--terraform-state` and assert exit 2.

### Tests for User Story 4

- [X] T017 [US4] Add failing flag tests in `internal/cli/cost_projected_terraform_test.go`: `--terraform-plan` parses the file and does not call the runner; a missing file and a file whose `format_version` major is not 1 exit 2 through `toValidationError`; the flag combined with `--pulumi-json`, `--pulumi-state`, or `--terraform-state` exits 2 and does not run a binary; a file over 32 MiB exits 2 and names the limit.

### Implementation for User Story 4

- [X] T018 [US4] Add `--terraform-plan` and the shared mutual-exclusion check in `internal/cli/infrastructure_inputs.go`, and call it from `internal/cli/cost_projected.go` before `loadProjectedResources`. A user-supplied bad format is exit 2. The same bytes from the runner use the plan-failed hint. Rename `planPath` (the `--pulumi-json` field) to `pulumiJSONPath` in `internal/cli/cost_projected.go`, `cost_recommendations.go`, and `cost_estimate.go` as part of this task, so it cannot be read as the new flag. The shared check's error names all four inputs (`--terraform-plan`, `--pulumi-json`, `--pulumi-state`, `--terraform-state`) on commands that have them.

**Checkpoint**: A saved plan file is priced with no Terraform process.

---

## Phase 7: User Story 5 - See plan actions and unknown values (Priority: P2)

**Goal**: The cost diff sees create, update, delete, replace, and unchanged. Plugins can tell unknown from absent. The projected cache digest changes when the unknown set changes.

**Independent Test**: Adapt the T003 fixture and assert descriptor operations, omitted data sources, unknown paths, and that `terraform:unknownProperties` is in attributes and not in the string tag map.

### Tests for User Story 5

- [X] T019 [P] [US5] Extend `internal/ingest/terraform_plan_test.go` to fail until the adapter copies `source_id` to the descriptor ID, sets `Operation` from the companion map (`same` for no-op, `replace` for delete plus create), copies before-values to `OldProperties`, and leaves data sources and the skipped providers out of the descriptor slice while keeping other providers with provider `custom`. Assert the stderr note names each skipped provider and count and does not touch stdout or the exit code.
- [X] T020 [P] [US5] Add failing tests in `internal/engine/attributes_terraform_test.go` that `ConvertToProto` omits `terraform:unknownProperties` while `BuildAttributes` keeps that list, and that `attributesCacheSuffix` changes when only that list changes.

### Implementation for User Story 5

- [X] T021 [US5] Implement the post-validation adapter in `internal/ingest/terraform_plan.go` from `ParseResponse` plus the operation map and before-values onto `[]engine.ResourceDescriptor`. Copy `unknown_properties` into properties as `terraform:unknownProperties` only on that engine value.
- [X] T022 [US5] Skip the key `terraform:unknownProperties` in `ConvertToProto` in `internal/engine/engine.go`. Do not add a second credential-name list.

**Checkpoint**: A fixture with mixed actions produces the same diff operations Pulumi uses, including `replace` normalized as an update by `normalizeDiffOperation`.

---

## Phase 8: User Story 6 - Use the same detection for every projection command (Priority: P2)

**Goal**: Overview, bare `finfocus`, forecast, estimate, and recommendations plan through the same detector and runner. Dismiss and snooze never plan. Overview does not skip the plan by file times.

**Independent Test**: Each command, pointed at a Terraform directory with no input file, calls the runner once. Overview does not call the Pulumi change-detection prompt.

### Tests for User Story 6

- [X] T023 [P] [US6] Add failing tests in `internal/cli/overview_terraform_test.go` that a Terraform directory always runs the plan and does not call `promptForPreview`, that `overview --terraform-plan` reads the file, starts no binary, and exits 2 when combined with `--pulumi-json`, `--pulumi-state`, or `--terraform-state`, and that bare `finfocus` delegates to overview in `internal/cli/root_test.go`.
- [X] T024 [P] [US6] Add a failing test in `internal/cli/cost_forecast_test.go` that `--terraform-plan` and auto-detect reach `loadProjectedResources`.
- [X] T025 [P] [US6] Add failing tests in `internal/cli/cost_estimate_terraform_test.go` that estimate accepts `--terraform-plan` or a Terraform directory instead of requiring `--pulumi-json`, and that `--modify` still matches the resource ID.
- [X] T026 [P] [US6] Add failing tests in `internal/cli/cost_recommendations_terraform_test.go` that `cost recommendations` no longer requires `--pulumi-json` when the detector returns Terraform or `--terraform-plan` is set, and that `dismiss` and `snooze` in a Terraform directory never invoke the runner and behave as before, and that both reject `--terraform-plan` as an unknown flag.

### Implementation for User Story 6

- [X] T027 [US6] Call the shared detector and runner from `internal/cli/overview.go`, register `--terraform-plan` there with the shared exclusivity check from T018, and delegate bare `finfocus` to overview for a Terraform directory in `internal/cli/root.go`.
- [X] T028 [US6] Register `--terraform-plan` on `internal/cli/cost_forecast.go` and pass it through the existing `loadProjectedResources` call.
- [X] T029 [US6] Accept `--terraform-plan` or Terraform auto-detect in `internal/cli/cost_estimate.go` without requiring `--pulumi-json` in those cases.
- [X] T030 [US6] Replace `MarkFlagRequired("pulumi-json")` (`internal/cli/cost_recommendations.go:156`) with a check that accepts Terraform detection or `--terraform-plan` as the resource source, and register `--terraform-plan` there. Leave `internal/cli/cost_recommendations_dismiss.go` unchanged: its `--pulumi-json` is already optional and only decides whether plugins open.

**Checkpoint**: The projection commands in `contracts/cli.md` share one detector. `cost actual` is still unchanged.

---

## Phase 9: User Story 7 - Keep history under a stable project identity (Priority: P3)

**Goal**: History files use the hash from `contracts/history-identity.md`. The workspace is included only with `--include-workspace` or `cost.terraform.include_workspace`. Collect does not invent applies. Actual cost does not plan.

**Independent Test**: Two temp git repos with the same `origin` in https and scp form and the same relative path resolve one `<hash>@unscoped` file. Opt-in changes the file. `cost history collect` and `cost actual` with no file exit 2 and do not plan.

### Tests for User Story 7

- [X] T031 [P] [US7] Write failing identity tests in `internal/terraform/identity_test.go`: https and `git@host:org/repo.git` share a hash; the label is the relative path; no `origin` uses the `tf-id-abspath-v1` hash and a `path:` label; the raw remote URL is not in the label; opt-in appends the workspace and rejects workspace name `unscoped`; opt-in off does not run `workspace show`.
- [X] T032 [P] [US7] Write failing CLI tests in `internal/cli/cost_history_terraform_test.go` and `internal/cli/cost_actual_terraform_test.go`: view, list, diff, export, and prune resolve the Terraform identity; collect prints the Pulumi-checkpoint sentence and exits 2; actual with no file prints the `--terraform-state` sentence and exits 2; `--include-workspace` on a Pulumi project does not change the Pulumi history key; a snapshot recorded from a Terraform projection is stored under the Terraform identity (FR-027).
- [X] T033 [P] [US7] Write failing config tests in `internal/config/terraform_workspace_test.go` that `cost.terraform.include_workspace` defaults false and is read from the merged config.

### Implementation for User Story 7

- [X] T034 [US7] Implement the hash, label, and workspace opt-in in `internal/terraform/identity.go` per `contracts/history-identity.md`.
- [X] T035 [US7] Add boolean `cost.terraform.include_workspace` to `internal/config/budget.go` without writing it from the flag.
- [X] T036 [US7] Add `--include-workspace` on the projection commands and on `cost history` in `internal/cli/infrastructure_inputs.go`. Resolve identity from `internal/cli/cost_history.go` and `internal/cli/history_record.go`. Write optional meta `project_label` in `internal/history/cost_db.go` without bumping `costSchemaVersion`. Implement the actual and collect refusals in `internal/cli/cost_actual.go` and `internal/cli/cost_history_collect.go`.

**Checkpoint**: History identity matches across clones. Actual cost and collect do not run a plan.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Docs, MCP schema, and the validation commands.

- [X] T037 [P] Document `--terraform-plan`, `--include-workspace`, `cost.terraform.include_workspace`, the four hints, and the unchanged `--terraform-state` behavior in `docs/src/content/docs/reference/cli-commands.md`, `docs/src/content/docs/reference/config-reference.md`, `docs/src/content/docs/commands/overview.md`, and `docs/src/content/docs/commands/forecast.md`. Also update the estimate, recommendations, and history command pages if they exist, and the Cobra `Long` and flag help text of every command in FR-020 and FR-027 (FR-029).
- [X] T038 Regenerate `internal/cli/testdata/mcp/tools.golden` so the new flags appear (`UPDATE_GOLDEN=1` on the MCP tool test only).
- [X] T039 Run `make test`, `make lint`, and `make validate`. Report coverage for `internal/terraform` and `internal/ingest/terraform_plan*.go` against the constitution's 95% critical-path target. Fix failures in the files this feature touched. Do not edit `.golangci.yml` or `CHANGELOG.md`.
- [X] T040 Walk `quickstart.md` for the cases that do not need cloud credentials (Pulumi non-regression, mutual exclusion, hints via the fake binary, history identity in temp git repos) and record any credentialed step that was not run.

---

## Dependencies & Execution Order

### Phase Dependencies

- Setup (Phase 1) has no dependencies.
- Foundational (Phase 2) depends on Setup and blocks every user story.
- User stories depend on Foundational. US2 depends on the runner file from US1. US4 depends on `loadProjectedResources` from US1. US5 depends on the parser from Phase 2 and the projected path from US1. US6 depends on US1 and US4. US3 and US7's identity tests can start after Phase 2.
- Polish depends on the stories you intend to ship.

### User Story Dependencies

- **US1 (P1)**: After Phase 2. No other story required.
- **US2 (P1)**: After US1's runner exists (T010).
- **US3 (P1)**: After Phase 2. Independently testable against the Pulumi path.
- **US4 (P2)**: After US1's loader (T011).
- **US5 (P2)**: After Phase 2's parser and US1's projected path.
- **US6 (P2)**: After US1 and US4, so every command can share `--terraform-plan`.
- **US7 (P3)**: Identity implementation after Phase 2. Wiring snapshots from a projection after US1. Actual and collect refusals do not need the runner's success path.

### Within Each User Story

- Tests fail before implementation.
- Parser and detector before the runner.
- Runner before command wiring.
- Validated `ParseResponse` before the engine adapter.

### Parallel Opportunities

- T004 can run beside T003.
- T006 can run beside T005 after T004 is written.
- T014 and T015 can run together.
- T019 and T020 can run together.
- T023, T024, T025, and T026 can run together.
- T031, T032, and T033 can run together.
- T037 can run beside T038 after the flags exist.

---

## Parallel Example: User Story 6

```bash
# Tests in different files, after US1 and US4:
# internal/cli/overview_terraform_test.go
# internal/cli/cost_forecast_test.go
# internal/cli/cost_estimate_terraform_test.go
# internal/cli/cost_recommendations_terraform_test.go
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. Finish Phase 1 and Phase 2.
2. Finish Phase 3 (US1).
3. Stop and run `go test ./internal/ingest/ ./internal/terraform/ ./internal/cli/ -count=1`.
4. `cost projected` in a Terraform directory is the demo.

### Incremental Delivery

1. US1: project with no flags.
2. US2: hints and cancellation.
3. US3: Pulumi non-regression.
4. US4: saved plan file.
5. US5: operations, unknowns, cache key.
6. US6: the rest of the projection commands.
7. US7: history identity, actual, and collect.
8. Polish: docs, MCP golden, `make test`, `make lint`, `make validate`.

### Parallel Team Strategy

After Phase 2:

- One person takes US1 then US2.
- Another takes US3.
- After US1, one person takes US4 and US5 while another starts US7's identity tests.
- US6 waits until `--terraform-plan` is on the shared loader.

---

## Notes

- [P] tasks touch different files and do not need an unfinished task.
- The parser is the first host use of `CostParseService`. Do not serve the RPC.
- `ParseResponse` has no operation. Do not encode the operation inside the proto.
- `resource_type` stays the Terraform type. `ResolveResourceTypes` runs after validation.
- Web UI Fit is Not applicable, so this list has no web tasks.

## Follow-up: OpenTofu execution and web parity

The owner's follow-up replaces the earlier web exemption and Terraform
binary preference. Compatible input names remain unchanged.

- [X] T041 Require OpenTofu only for planning and workspace lookup; reject missing or invalid versions before planning and add binary sentinel regressions.
- [X] T042 Regenerate genuine OpenTofu fixtures, record provenance, pin tooling, and check feature source/dependencies for Terraform core inclusion.
- [X] T043 Route web detection and query resources through the shared Terraform overview pipeline; add web and Pulumi regression tests.
- [X] T044 Validate a fresh OpenTofu plan and populated web dashboard in a browser, update the web contract, and file the required UX-only `web-ui` issue.
- [X] T045 Run repository tests, lint, validation, and coverage after the follow-up changes; update the validation record and implementation status.
