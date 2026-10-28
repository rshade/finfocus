---
description: "Task list for the analyzer language plugin (#1895)"
---

# Tasks: Analyzer Language Plugin

**Input**: Design documents from `specs/1734-analyzer-language-plugin/`
**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md),
[research.md](research.md), [data-model.md](data-model.md),
[contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Per Constitution Principle II, tests are written before the code
they cover and must fail first. Coverage is measured in T011 and T033: at
least 80% for each changed package, and 95% for the critical paths the
constitution names: the CLI entry points `cli.RunAnalyzerLanguageHost` and
`startModeFor`, and the plugin communication `LanguageHost.RunPlugin` and
`SetupPolicyPack`.

**Completeness**: Per Principle VI, no stubs, placeholders, or TODOs. The
`LanguageRuntime` RPCs left to `UnimplementedLanguageRuntimeServer` are the
deliberate set in research R2, not stubs.

**Web parity**: The spec records Web UI Fit as `Not applicable` (analyzer mode
has no dashboard surface), so there is no Web Parity phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an
  unfinished task)
- **[Story]**: US1 to US4 from spec.md

## Conventions for every task

- Testify only (`require` to stop, `assert` to continue).
- `t.Parallel()` everywhere except tests that call `t.Setenv`, change process
  state, or execute a binary they wrote; mark those
  `//nolint:paralleltest // <reason>`.
- Any test that touches `FINFOCUS_HOME` sets it to `t.TempDir()`.
- The language host writes nothing but its port to standard output.

---

## Phase 1: Setup

**Purpose**: Record the failing baseline the change must turn green.

- [X] T001 Reproduce the break check from quickstart.md step 1: unpack Pulumi
  3.271.0 into `bin/pulumi-3.271.0/` (git-ignored), then run
  `FINFOCUS_TEST_PULUMI=<path> go test -count=1 -run TestAnalyzerPolicyPack ./test/integration/ -v`
  against it (expect FAIL with `no language plugin 'pulumi-language-finfocus' found`)
  and against `$HOME/.pulumi/bin/pulumi` 3.268.0 (expect PASS). Keep both
  outputs for the PR description. If `/tmp` is short on space, set `TMPDIR` to
  a directory under `bin/`.

---

## Phase 2: Foundational (Policy Pack Entries)

**Purpose**: Both pack entries and their platform names, which US1 needs to be
started at all and US2 builds on (data-model.md "Policy pack directory",
research R8).

**⚠️ CRITICAL**: No user story work begins until this phase is complete.

- [X] T002 Write failing tests in `internal/analyzer/policypack_test.go`:
  (a) table test of a pure `policyPackEntryName(name, goos string) string`
  for `linux`, `darwin`, and `windows` (Windows appends `.exe`, others do
  not), for both `policyPackBinaryName` and `LanguagePluginBinaryName`;
  (b) `PolicyPackEntryPath(dir, name)` joins `dir` with the current
  platform's name and is absolute for an absolute `dir`; (c)
  `SetupPolicyPack` creates `PulumiPolicy.yaml` plus both entries; on Unix
  each entry is a symlink whose target is the given exec path; (d) a second
  call is idempotent and replaces a dangling symlink at either entry without
  following it; (e) a pre-existing directory holding only the legacy entry
  gains the language plugin entry. Use `t.Setenv("FINFOCUS_HOME", t.TempDir())`
  and mark those tests non-parallel. Replace the existing
  `policyPackBinaryPath_returns_correct_name` subtest with (b). The nightly
  `windows-latest` job runs this file natively.
- [X] T003 Implement in `internal/analyzer/policypack.go`: exported constant
  `LanguagePluginBinaryName = "pulumi-language-finfocus"`, an unexported list
  of the two entry names (legacy first), `policyPackEntryName(name, goos)`,
  exported `PolicyPackEntryPath(dir, name string) string` (uses
  `runtime.GOOS`), and `SetupPolicyPack` creating every entry in the list
  through `linkOrCopy` at `PolicyPackEntryPath`. Remove `policyPackBinaryPath`,
  which this replaces. Keep `SetupPolicyPack`'s signature and its returned
  method (the language plugin entry's). Run
  `go test -race ./internal/analyzer/ -run 'PolicyPack'` until T002 passes.

**Checkpoint**: `finfocus analyzer install` produces the full layout on every
platform.

---

## Phase 3: User Story 1 - Cost diagnostics in `pulumi preview` on current Pulumi (Priority: P1) 🎯 MVP

**Goal**: `pulumi preview --policy-pack <pack dir>` works on Pulumi 3.268.0 and
3.270.0+ through the `pulumi-language-finfocus` entry (FR-001 to FR-003,
FR-007, FR-008, FR-009).

**Independent Test**: The integration test from T001 passes on Pulumi 3.271.0
and 3.268.0 (quickstart.md step 2).

### Tests for User Story 1 (TDD) ⚠️

- [X] T004 [P] [US1] Write a failing table test in `cmd/finfocus/main_test.go`
  for a pure `startModeFor(exeName string) startMode` covering
  `pulumi-language-finfocus` and its `.exe` form (language host),
  `pulumi-analyzer-policy-finfocus` and `pulumi-analyzer-finfocus` with and
  without `.exe` (analyzer serve), a path prefix such as
  `/home/u/.finfocus/analyzer/pulumi-language-finfocus`, and `finfocus` /
  `finfocus.exe` (CLI). Contract: data-model.md "Start-up mode".
- [X] T005 [P] [US1] Write failing tests in
  `internal/analyzer/languagehost_test.go` (package `analyzer`) against a
  `bufconn` gRPC server, following `internal/pluginhost/client_test.go`:
  `GetPluginInfo` returns `version.GetVersion()`; `Handshake`,
  `GetRequiredPlugins`, `GetRequiredPackages`, `GetProgramDependencies`
  return empty; `InstallDependencies` succeeds with no output; `About`
  returns the executable and version; `Run` returns `codes.Unimplemented`;
  `RunPlugin` returns `codes.InvalidArgument` for no args, nil `info`,
  relative `info.program_directory`, and `kind` other than `analyzer`.
  Child-process cases use a helper child: a `TestMain` that, when
  `FINFOCUS_LANGHOST_TEST_CHILD` is set, acts per that value and exits
  (no binary is written, so these can run in parallel), with
  `LanguageHostOptions.Executable` set to `os.Args[0]` of the test binary.
  Cover: the child sees `os.Args[0]` equal to
  `LanguageHostOptions.AnalyzerName`, arguments `[<engine addr>, "."]`, its
  working directory equal to `info.program_directory`, and an entry from
  `req.env`; stdout bytes arrive in order with the first line unchanged;
  stderr arrives as stderr messages; exit code 3 arrives as
  `RunPluginResponse.exitcode == 3`; a nonexistent executable makes
  `RunPlugin` return an error; canceling the stream makes a child that waits
  for SIGINT exit cleanly within `WaitDelay` (Unix only, `//go:build unix`
  subtest in `languagehost_unix_test.go`).
- [X] T006 [P] [US1] Write a failing test in
  `internal/cli/analyzer_language_test.go`: run `RunAnalyzerLanguageHost`
  with a cancelable command context and a buffer as `cmd.OutOrStdout()`;
  read exactly one line, check it is a port number and the buffer holds
  nothing else, dial it, call `GetPluginInfo`, cancel the context, and
  require the function to return nil within 4.5 seconds (the shutdown cap is
  4 seconds; Pulumi force-kills at 5). Add a case with an open `RunPlugin`
  stream whose helper child ignores SIGINT: the function still returns
  within 4.5 seconds.

### Implementation for User Story 1

- [X] T007 [US1] Implement `internal/analyzer/languagehost.go` per
  contracts/language-host.md: `LanguageHostOptions{Executable, AnalyzerName
  string; WaitDelay time.Duration}`, `NewLanguageHost` (defaults:
  `os.Executable()`, `policyPackEntryName(policyPackBinaryName, runtime.GOOS)`,
  3 seconds), `LanguageHost` embedding
  `pulumirpc.UnimplementedLanguageRuntimeServer`, the RPCs in the contract's
  table, and `RunPlugin` building `exec.Cmd{Path: Executable, Args:
  [AnalyzerName, args[0], "."]}` via `exec.CommandContext`, `Dir =
  info.ProgramDirectory`, `Env = os.Environ()` plus `req.Env`, stdout/stderr
  through an unexported `io.Writer` that sends `RunPluginResponse` messages
  (copy the slice before `Send`), `WaitDelay`, and the exit-code mapping in
  data-model.md. Import `pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"`;
  do not import `sdk/.../rpcutil` (research R10). Exported identifiers get
  doc comments.
- [X] T008 [P] [US1] Add `internal/analyzer/languagehost_unix.go`
  (`//go:build unix`, `cmd.Cancel` sends `os.Interrupt`) and
  `internal/analyzer/languagehost_windows.go` (`//go:build windows`,
  `cmd.Cancel` calls `Process.Kill`), each with a comment saying why
  (research R5). `languagehost.go` calls the shared unexported setter.
- [X] T009 [US1] Implement `internal/cli/analyzer_language.go`:
  `RunAnalyzerLanguageHost(cmd *cobra.Command) error` sets up the stderr
  logger the same way `RunAnalyzerServe` does, wraps `cmd.Context()` with
  `signal.NotifyContext(..., os.Interrupt, syscall.SIGTERM)`, listens on
  `127.0.0.1:0`, registers `analyzer.NewLanguageHost`, writes only the port
  and a newline to `cmd.OutOrStdout()`, serves, and on context end calls
  `GracefulStop`, falling back to `Stop` once a shutdown cap of
  `WaitDelay` + 1 second (4 seconds by default) has passed, so the host exits
  before Pulumi's 5-second force-kill (contracts/language-host.md "Process").
  Run T006 until it passes.
- [X] T010 [US1] In `cmd/finfocus/main.go`, add `startMode`/`startModeFor`
  and dispatch the language host name to `cli.RunAnalyzerLanguageHost` before
  the analyzer names, with a `cobra.Command` whose output is `os.Stdout` and
  context is `context.Background()`, exactly as the analyzer branch builds
  its command. Update `run()`'s doc comment to list the three names. Run T004
  until it passes.
- [X] T011 [US1] Run `go test -race -shuffle=on -count=3 ./internal/analyzer/... ./internal/cli/ -run 'LanguageHost|PolicyPack|AnalyzerLanguage'`,
  then the integration test on both Pulumi versions (quickstart.md step 2):
  both PASS, and on 3.271.0 the preview output contains
  `using pulumi-language-finfocus` and `analyzer server listening`. Measure
  coverage: `go test -coverprofile=bin/cover-us1.out ./internal/analyzer/ ./internal/cli/ ./cmd/finfocus/`
  then `go tool cover -func=bin/cover-us1.out`; `RunPlugin`,
  `RunAnalyzerLanguageHost`, `startModeFor`, and `SetupPolicyPack` each show
  at least 95%, and `languagehost.go` and `analyzer_language.go` as a whole at
  least 80%. Add tests until they do.

**Checkpoint**: US1 is the MVP. Analyzer mode works again on current Pulumi.

---

## Phase 4: User Story 2 - Fix an existing installation with one command (Priority: P1)

**Goal**: `finfocus analyzer install` upgrades any existing pack, including
"already installed" runs, and reports the language plugin entry (FR-004,
FR-005, FR-006).

**Independent Test**: A pack directory with only the legacy entry, after
`finfocus analyzer install`, has both entries, and the integration test passes
on Pulumi 3.271.0.

### Tests for User Story 2 (TDD) ⚠️

- [X] T012 [P] [US2] Write failing tests in `internal/analyzer/install_test.go`
  (non-parallel; `t.Setenv("FINFOCUS_HOME", t.TempDir())`): a fresh
  `Install` sets `InstallResult.PolicyPackLanguagePlugin` to the absolute
  language plugin entry path; an `Install` that finds the analyzer already
  current still adds the language plugin entry to a legacy-only pack
  directory; `Install` with `Force` recreates both entries pointing at the
  current executable.
- [X] T013 [P] [US2] Write failing tests in
  `internal/cli/analyzer_install_test.go`: human output contains the line
  `Language plugin: <pack dir>/pulumi-language-finfocus` indented two spaces
  (with `.exe` on Windows) and every existing line of contracts/cli-analyzer-install.md
  unchanged; `--format json` output carries `policy_pack_language_plugin`
  and every existing field.

### Implementation for User Story 2

- [X] T014 [US2] In `internal/analyzer/install.go`, add
  `PolicyPackLanguagePlugin string \`json:"policy_pack_language_plugin,omitempty"\``
  to `InstallResult` with a doc comment, set it in
  `setupPolicyPackForInstall` from `PolicyPackEntryPath`, and replace
  `maybeSyncPolicyPack`/`syncPolicyPackBinary` with a `SetupPolicyPack` call,
  which already recreates every entry (research R8). Delete the removed
  functions and their tests' references. Run T012 until it passes.
- [X] T015 [US2] In `internal/cli/analyzer_install.go`, print a
  `Language plugin: %s` line, indented two spaces like its neighbors, after
  `Policy pack method:` when
  `PolicyPackLanguagePlugin` is set. Run T013 until it passes.

**Checkpoint**: Existing users recover with `finfocus analyzer install`.

---

## Phase 5: User Story 3 - Hear about the next Pulumi break first (Priority: P2)

**Goal**: An automated check covers Pulumi 3.268.0 and the newest release on
every PR and nightly, including the legacy entry and plugin cleanup (FR-010,
FR-011, SC-004, SC-005).

**Independent Test**: With the fix reverted the CI job fails on `latest`;
restored, both matrix entries pass.

### Tests for User Story 3 (TDD) ⚠️

- [X] T016 [US3] Restructure
  `test/integration/analyzer_policy_pack_test.go` into subtests that share
  one built binary and environment builder: `language_plugin` (the current
  assertions, plus: after the preview, read
  `<project>/.finfocus/last-cost-summary.json` and require `stack` = `dev` and
  `project` = `policy-pack-smoke`, which proves `ConfigureStack` reached the
  analyzer on this path (US1 AS4) and that the summary lands under the
  project's `.finfocus/` and not under `FINFOCUS_HOME` (FR-009, FR-014); if the analyzer writes no summary for a program
  with no resources, give this subtest's `Pulumi.yaml` one resource and
  assert again); `default_acquisition` (the same preview without
  `PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION`: it passes, and the output
  contains neither `pulumi-finfocus` nor `could not find latest version`,
  which checks the spec's edge case that Pulumi never tries a download once
  the language plugin is present; needs network, which CI has);
  `legacy_entry_only` (delete the language plugin entry after
  `analyzer install`, run the preview, expect success and
  `using pulumi-analyzer-policy-finfocus`; skip with a reason when
  `pulumi version` is 3.270.0 or later, comparing with the semver package
  already in `go.mod`); `plugin_cleanup` (install the router-like fixture
  from `test/integration/testdata/orphanplugin` into `$FINFOCUS_HOME/plugins`
  the way `newOrphanFixture` in `web_sigterm_orphan_test.go` does, run the
  preview, then require every PID the fixture recorded to be gone within 10
  seconds using `processAlive`). If the analyzer does not start the fixture
  for an empty program, add one resource of a type the fixture's `Supports`
  accepts to that subtest's `Pulumi.yaml`. Keep the opt-in on
  `FINFOCUS_TEST_PULUMI` and the `//go:build unix` tag.

### Implementation for User Story 3

- [ ] T017 [P] [US3] In `test/e2e/analyzer_e2e_test.go`,
  `ensureAnalyzerConfig` copies the binary to both
  `pulumi-analyzer-policy-finfocus<ext>` and `pulumi-language-finfocus<ext>`
  and updates its doc comment (the e2e module cannot import
  `internal/analyzer`, so the names stay literal here, with a comment that
  names `analyzer.LanguagePluginBinaryName` as their source). Verify with
  `gh workflow run nightly.yml --ref <branch> -f run_analyzer=true` (it needs
  the repository's AWS credentials, so it cannot run locally) and confirm the
  `Analyzer E2E Tests` job passes; it failed on 2026-10-22 with this bug.
- [X] T018 [P] [US3] Add a job `analyzer-policy-pack` to
  `.github/workflows/ci.yml`: matrix `os: [ubuntu-latest, macos-latest]` ×
  `pulumi: ["3.268.0", "latest"]` with `fail-fast: false` and
  `runs-on: ${{ matrix.os }}` (macOS runs the same `//go:build unix` test, so
  SC-001 is checked on both automatically), `actions/checkout@v7`,
  `actions/setup-go@v7` with the workflow's `GO_VERSION`, `pulumi/actions@v7`
  with `pulumi-version: ${{ matrix.pulumi }}`, then
  `FINFOCUS_TEST_PULUMI="$(command -v pulumi)" go test -v -count=1 -run TestAnalyzerPolicyPack ./test/integration/`
  with a 15-minute timeout. Match the existing jobs' `permissions` and
  pinning style.
- [X] T019 [P] [US3] Add the same check with `pulumi-version: latest` to
  `.github/workflows/nightly.yml`, scheduled like the existing integration
  job, so Pulumi releases between PRs are caught.
- [ ] T020 [US3] Run `make lint` (includes actionlint) for T018 and T019, and
  run T016's subtests on both Pulumi versions: `language_plugin`,
  `default_acquisition`, and `plugin_cleanup` pass on both,
  `legacy_entry_only` passes on 3.268.0 and is skipped on 3.271.0. Push the
  branch and confirm all four `analyzer-policy-pack` jobs pass. The red side
  of the break check (SC-004) is T001's recorded run on `main`; do not repeat
  it here.

**Checkpoint**: The next Pulumi break turns CI red.

---

## Phase 6: User Story 4 - Docs that match the tools (Priority: P2)

**Goal**: Every analyzer doc describes the two-entry layout, and the FAQ
answers the exact error (FR-012).

**Independent Test**: Following `getting-started/analyzer-setup.md` on Pulumi
3.271.0 reaches a working preview; searching the docs for
`pulumi-language-finfocus` finds the FAQ entry.

- [X] T021 [P] [US4] Update
  `docs/src/content/docs/getting-started/analyzer-setup.md`: the pack holds
  `PulumiPolicy.yaml`, `pulumi-analyzer-policy-finfocus`, and
  `pulumi-language-finfocus`; Pulumi 3.270.0+ uses the language plugin;
  existing installs re-run `finfocus analyzer install`; add the error message
  to Troubleshooting.
- [X] T022 [P] [US4] Update `docs/src/content/docs/analyzer-integration.md`
  to the new layout and setup commands.
- [X] T023 [P] [US4] Update `docs/src/content/docs/architecture/analyzer.md`:
  replace "Pulumi finds `pulumi-analyzer-policy-finfocus` by searching PATH"
  with the language host flow (Pulumi starts `pulumi-language-finfocus`,
  `RunPlugin` starts the analyzer in the policy pack directory, the port is
  read from its output), and note the legacy entry for Pulumi before 3.270.0.
- [X] T024 [P] [US4] Update `docs/src/content/docs/deployment/deployment.md`
  CI and container setup steps to create both entries (or run
  `finfocus analyzer install`).
- [X] T025 [P] [US4] Update the policy pack references in
  `docs/src/content/docs/guides/routing.md`.
- [X] T026 [P] [US4] Add an FAQ entry to
  `docs/src/content/docs/support/faq.md` quoting
  `no language plugin 'pulumi-language-finfocus' found in the workspace or on your $PATH`
  and the 404 line about `pulumi-finfocus`, with the cause (Pulumi 3.270.0)
  and the fix (re-run `finfocus analyzer install`, keep the pack directory on
  `PATH`).
- [X] T027 [P] [US4] Write a failing
  `internal/cli/finfocus_analyzer_setup_skill_test.go`, modeled on
  `finfocus_diagnose_skill_test.go`: the archive
  `agent-skills/finfocus-analyzer-setup/finfocus-analyzer-setup.skill` holds
  exactly `finfocus-analyzer-setup/SKILL.md` and
  `finfocus-analyzer-setup/references/troubleshooting.md`, each byte-equal to
  the file on disk, and `SKILL.md` names `pulumi-language-finfocus`. (Today's
  archive also contains a stale nested copy of itself and has no test.) Then
  update `agent-skills/finfocus-analyzer-setup/SKILL.md` and
  `references/troubleshooting.md`, check `agent-skills/finfocus-install/SKILL.md`
  and update it (and its archive) if it names the pack entries, and rebuild
  the archive from `agent-skills/` with only those two paths
  (`zip -X <skill>/<skill>.skill <skill>/SKILL.md <skill>/references/*` into a
  fresh file). Run `go test ./internal/cli/ -run 'Skill'` until it passes.
- [X] T028 [US4] Run `make docs-lint` and `markdownlint-cli2` on the files
  changed in T021-T027 (`docs/src/content/docs/**`, `agent-skills/**`); fix
  all findings.

**Checkpoint**: Docs, FAQ, and agent skills match the shipped layout.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T029 Add an "Analyzer language host" bullet group under
  Package-Specific Gotchas in `CLAUDE.md` (the `os.Executable()` symlink
  trap and the explicit `Args[0]`, `programDirectory` as working directory
  with `pwd` exported as `FINFOCUS_PROJECT_DIR`, SIGINT on cancel and Kill on Windows, the
  `.exe` entry names, `FINFOCUS_TEST_PULUMI`), and a Recent Changes line for
  `1734-analyzer-language-plugin`. Edit by hand; do not run
  `update-agent-context.sh`, which rewrites the curated sections.
- [ ] T030 Draft the FR-013 issue for `rshade/finfocus-action`
  (`src/analyze.ts` `setupAnalyzerMode` writes only the legacy entry; it should
  run `finfocus analyzer install` or create both entries), researched and
  written as a detailed prompt-style issue, and file it once the owner
  approves the text.
- [X] T031 Resolve the analyzer's project directory from
  `RunPluginRequest.pwd` (FR-014) in this change, not in an issue:
  `projectDirEnv` in `internal/analyzer/languagehost.go` exports it as
  `FINFOCUS_PROJECT_DIR`, which `config.ResolveProjectDir` already reads, so
  `analyzer serve` is unchanged and the `cost.history.directory` safety check
  applies as before. `TestLanguageHost_RunPluginProjectDir` covers an
  absolute directory, empty, relative, missing, and file `pwd`, and an
  explicit setting winning. `language_plugin` now expects the summary under
  the project's `.finfocus/` and none under `FINFOCUS_HOME`. A demo preview
  on Pulumi 3.272.0 confirmed Pulumi sends the project as `pwd`.
- [ ] T032 Run quickstart.md step 4 against `../finfocus-demo` with
  `pulumi preview` only (no `pulumi up`), compare the cost diagnostics with
  a Pulumi 3.268.0 run, and check `ps` for leftover `finfocus-plugin-*`,
  `pulumi-analyzer-policy-finfocus`, or `pulumi-language-finfocus`
  processes (SC-005, SC-006). On Windows (a Windows machine, or a
  `windows-latest` runner via a `workflow_dispatch` job), build the release
  binary, run `finfocus analyzer install`, put the pack directory on `PATH`
  with the printed PowerShell line, and run `pulumi preview --policy-pack`
  against a project with no resources on the newest Pulumi. Record the
  Pulumi version, the `dir` listing of the pack (both `.exe` entries), and
  the preview output in the PR description (SC-001). If no Windows run is
  possible, say so in the PR; do not mark SC-001 met.
- [ ] T033 Final gates: `gofmt -l`, `make lint` (allow over 5 minutes),
  `make test`, `make validate`, `go test -race ./test/integration/ -run TestAnalyzerPolicyPack`
  on both Pulumi versions, a cross-compile of `./cmd/finfocus` for
  `GOOS=windows` and `GOOS=darwin`, and the coverage gate:
  `go test -coverprofile=bin/cover.out ./internal/analyzer/ ./internal/cli/ ./cmd/finfocus/`
  with `go tool cover -func=bin/cover.out` showing at least 95% for
  `RunPlugin`, `RunAnalyzerLanguageHost`, `startModeFor`, and
  `SetupPolicyPack`, and the `internal/analyzer` package total not below its
  value on `main`. Put the coverage numbers in the PR description. Remove
  `bin/pulumi-3.271.0/` and `bin/cover*.out` afterwards.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)**: none.
- **Foundational (T002-T003)**: after T001; blocks every story.
- **US1 (T004-T011)**: after Foundational. The MVP.
- **US2 (T012-T015)**: after Foundational; independent of US1's code, but
  its independent test's preview needs US1.
- **US3 (T016-T020)**: after US1 (the subtests need a working language
  host); T017-T019 can start once US1 lands.
- **US4 (T021-T028)**: after US1 and US2 settle the layout and the install
  output; all doc tasks are parallel.
- **Polish (T029-T033)**: after all stories.

### Within Each Story

- Tests first and failing, then the implementation that makes them pass.
- T007 before T009 (the CLI entry registers `NewLanguageHost`); T009 before
  T010 (main calls the CLI entry).
- T014 before T015.

### Parallel Opportunities

- T004, T005, T006 (three test files).
- T008 beside T007 once the setter's name is fixed.
- T012 and T013.
- T017, T018, T019.
- T021 to T027 (separate docs).

---

## Parallel Example: User Story 1

```text
Task: "T004 startModeFor table test in cmd/finfocus/main_test.go"
Task: "T005 LanguageHost bufconn and helper-child tests in internal/analyzer/languagehost_test.go"
Task: "T006 RunAnalyzerLanguageHost port and shutdown test in internal/cli/analyzer_language_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. T001 baseline, T002-T003 pack entries.
2. T004-T011: language host, entry point, dispatch.
3. Stop and validate: the integration test passes on Pulumi 3.271.0 and
   3.268.0. That alone fixes #1895 for anyone who re-runs
   `finfocus analyzer install`.

### Incremental Delivery

1. MVP (US1), then US2 (install output and upgrade path), in the same PR,
   since both are P1 and US2 is small.
2. US3 (CI coverage) and US4 (docs) complete the PR; the docs must ship with
   the code (Principle IV).
3. Polish: CLAUDE.md, the finfocus-action issue draft, demo check, final gates.

## Notes

- Commit only when asked; never on `main` (it is protected).
- Expected plugin failures in a passing test (`connection refused`, `EOF`)
  are logged with `t.Logf`, not failures.
- `CHANGELOG.md` belongs to release-please; the `fix(analyzer):` commit is
  the release note.
- After the release: remove the Pulumi 3.268.0 pin in `finfocus-demo`
  (SC-003); that change lives in that repository.
