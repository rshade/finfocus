---

description: "Task list for Pulumi ESC credentials for plugins"
---

# Tasks: Pulumi ESC Credentials for Plugins

**Input**: Design documents from `/specs/1730-pulumi-esc-plugin-credentials/`
**Prerequisites**: plan.md, spec.md, research.md (R1–R12), data-model.md,
contracts/ (esc-package.md, config.md, http-api.md), quickstart.md

**Tests**: Per Constitution Principle II, tests are MANDATORY and are written
BEFORE implementation. Each test task must fail before its implementation
task starts. `ResolveDeliveredEnv`, `appendDeliveredEnv`, and `esc.Source`
are plugin-communication critical paths, with a 95% coverage target. Other
code needs at least 80%.

**Completeness**: Per Constitution Principle VI, no stubs, placeholders, or
TODO comments.

**Documentation**: Per Constitution Principle IV, docs change in the same PR
as the code.

**Conventions used by every task**:

- Use testify only (`require` / `assert`).
- Call `t.Parallel()` unless the test uses `t.Setenv`, `t.Chdir`, process
  globals, or builds a root command. Such tests are marked
  `//nolint:paralleltest // <reason>`.
- Tests that write and execute a script plugin are `//go:build unix` and are
  not parallel.
- Never print or assert on a secret value in a log message. Assert that it is
  absent.
- Every CLI test sets `t.Setenv("FINFOCUS_HOME", t.TempDir())`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: The user story the task belongs to (US1–US5)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: The dependency, the package skeleton docs, and the shared ESC
fake that every later test uses.

- [X] T001 Add `github.com/pulumi/esc-sdk/sdk@v0.14.0` to the root `go.mod`.
  - Run `go mod tidy`.
  - Confirm that the only new modules are `esc-sdk/sdk` and
    `gopkg.in/ghodss/yaml.v1` (research R1) and that the nested plugin
    modules under `plugins/` are unchanged.
  - Run `make validate` and `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`.
- [X] T002 [P] Create `internal/esc/doc.go` with the package godoc.
  - It must state the purpose (opening Pulumi ESC environments for plugin
    credentials).
  - It must state the import rules from `contracts/esc-package.md`: imports
    only `internal/logging` and `internal/pulumi`, and never `cli`, `engine`,
    `pluginhost`, `registry`, `config`, or `webui`.
  - It must state the rule that no ESC value is ever logged, returned in an
    error, or persisted.
- [X] T003 [P] Create the reusable fake ESC API in
  `internal/esc/esctest/server.go`. It is an `httptest.Server` that serves
  the real esc-sdk paths under `/api/esc`:
  - `POST /environments/{org}/{project}/{env}/open` and
    `/versions/{version}/open`
  - `GET …/open/{id}`, returning `Environment` JSON with
    `properties.environmentVariables` entries that carry `value`, `secret`,
    and `trace`
  - `HEAD /environments/{org}/{project}/{env}`, which sets an `ETag`
  - `GET …/versions/tags/{tag}`, which returns `{"revision": N}`

  Per environment it must be configurable with:
  - the variables and their secret flags
  - rotation, so selected values change on every open
  - the ETag
  - tag → revision mappings
  - the response status (401, 403, 404, or 500 with and without a `message`)
  - a response delay
  - a malformed trace that makes the SDK panic

  It must count opens per environment and require
  `Authorization: token <t>`. Add `internal/esc/esctest/server_test.go`,
  which proves each configuration option with a raw `net/http` client.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The `internal/esc` package and the stack-context helpers in
`internal/pulumi`. Every user story depends on them.

**⚠️ CRITICAL**: No user-story work can begin until this phase is complete.

### Tests (write first, must fail)

- [X] T004 [P] `ParseRef` and `Ref.String` table tests in `internal/esc/ref_test.go`.
  - **Segment forms**: one segment → `default/name`, two → `project/name`,
    three → `org/project/name`.
  - **Versions**: `@3`, `@stable`, `@latest`.
  - **Rejected input**: an empty segment, four segments, characters outside
    `^[A-Za-z0-9][A-Za-z0-9._-]*$`, an org shorter than 3 or longer than 40
    characters, and an `@` with no version.
  - **Canonical form**: `String()` omits an empty org and version.
- [X] T005 [P] Failure classification tests in `internal/esc/failure_test.go`.
  - Classify each input to its category:
    - `*esc.GenericOpenAPIError` with a 401 / 403 / 404 / 500 model → its category;
    - an error with no `message` field (status unreadable) → `unavailable`;
    - `context.DeadlineExceeded` → `timed_out`;
    - `*url.Error` → `unavailable`;
    - a recovered panic → `unavailable`.
  - Assert that `Failure.Error()` equals the fixed text from
    `contracts/esc-package.md`, including the `not_found` form "not found or
    no access", and contains neither the SDK error text nor a fixture body
    string.
- [X] T006 [P] Value extraction tests in `internal/esc/values_test.go`.
  - String, `float64` (formatted with `strconv`), and bool values become
    strings.
  - Object, list, and missing entries are dropped, and a debug log line
    carries only the name.
  - Each entry keeps its secret flag.
  - A missing `environmentVariables` property yields an empty result.
- [X] T007 [P] Auth discovery tests in `internal/esc/auth_test.go`. Use
  `Options.Getenv` and temp directories so the tests stay parallel.
  - **Backend URL order**: `PULUMI_BACKEND_URL` → `Pulumi.yaml` `backend.url`
    → `credentials.json` `current` → `https://api.pulumi.com`.
  - **Self-managed backends**: the `file`, `s3`, `gs`, and `azblob` schemes
    → `unsupported_backend`.
  - **Non-https URLs**: `http` to a non-loopback host → `unsupported_backend`;
    `http://127.0.0.1` is allowed.
  - **Token order**: `PULUMI_ACCESS_TOKEN` → `accounts[url].accessToken` →
    `accessTokens[url]`.
  - **Missing or unusable credentials**: an encrypted envelope (a JSON object
    without `current`, `accounts`, or `accessTokens`), a missing file, or an
    empty token → `not_logged_in`.
  - **Credentials location**: `PULUMI_CREDENTIALS_PATH`, then `PULUMI_HOME`,
    then the home directory.
  - **Org order** (research R4):
    - A stack import uses the org segment of a qualified stack name, else the
      default-org chain.
    - A configured reference without an org always uses the default-org
      chain: `PULUMI_DEFAULT_ORGANIZATION` → `config.json`
      `backends[url].defaultOrg` → `accounts[url].username` → `not_logged_in`.
    - Case: a qualified stack in org A with default org B. A configured
      `proj/env` opens in B, and a stack import opens in A.
  - **Token trust (FR-020)**: `Pulumi.yaml` `backend.url: https://evil.example`
    with `PULUMI_ACCESS_TOKEN` set → `unsupported_backend`, and no request
    leaves the process. Also: a project URL with a token saved for exactly
    that URL in `credentials.json` → allowed.
  - **Read-only**: the test asserts that no file under the fake Pulumi home
    is created or modified.
- [X] T008 [P] Stack-context tests:
  - In `internal/pulumi/workspace_test.go`: `SelectedStack` reads
    `<PULUMI_HOME>/workspaces/<name>-<sha1(abs Pulumi.yaml)>-workspace.json`
    in the `{"stacks":{url:...}}` and legacy `{"stack":...}` formats. A
    missing or garbled file returns an error, not a panic.
  - In `internal/pulumi/stack_settings_test.go`: `StackEnvironment` parsing.
    A list of strings and a map with only `imports` → `Imports`. A map with
    `values`, an import written as `{name: {merge: false}}`, or any other
    key → `Unsupported`. A missing key → `Absent`.
  - In `internal/pulumi/stack_context_test.go`: `LoadStackContext`. The
    `--stack` value wins over the workspace file, and a short name resolves
    `Pulumi.<short>.yaml`. Install a runner that fails the test if called, to
    prove the Pulumi CLI is never invoked.
- [X] T009 [P] Client tests against `esctest` in `internal/esc/client_test.go`.
  - Opens go through a hand-built `esc.Configuration`, with `Servers` set to
    the `esctest` URL plus `/api/esc` and a no-redirect `HTTPClient`.
  - **Ports**: a `127.0.0.1` port is preserved. This is a regression test for
    the `NewCustomBackendConfiguration` port loss.
  - **Revision keys**:
    - no version → the `HEAD` ETag;
    - `@7` → `7`, with no extra call;
    - `@stable` → the tag's revision, through `GetEnvironmentRevisionTag`;
    - moving the tag in the fake changes the key while the ETag does not.
  - **Errors**: the malformed-trace fixture returns `unavailable` without a
    panic. A delay longer than the context deadline returns `timed_out`.
  - **Auth context**: it is derived from the caller's context, so cancelling
    the caller aborts the request.
- [X] T010 `Source` tests in `internal/esc/source_test.go` (depends on T003).
  - Eight concurrent `Delivery` calls for plugins that share one reference
    produce exactly one open in `esctest`.
  - Two references merge in order, and the later one wins per name (FR-004).
  - With no selection, `esctest` sees zero requests and the warn sink stays
    empty.
  - The wait budget (data-model Source "Budget"):
    - Two slow references with `Budget: 300ms`, opened concurrently, finish
      in under 400 ms total (SC-004).
    - `Delivery` for reference A, then a pause of 2× the budget with no ESC
      call in flight, then `Delivery` for reference B: B opens successfully.
      This is the case of the scorer opened after recommendations.
    - Once the budget is used up, a new call fails at once as `timed_out`.
  - One warning per `(reference, category)`, even when many plugins request
    the failing reference.
  - A failing reference does not suppress a healthy one.
  - `UseStack=false` ignores `StackContext` imports.
  - An `Unsupported` stack environment yields one `unsupported_definition`
    warning.
  - `Delivery` returns every resolved name except reserved ones, each with
    the reference that won it. It does not filter by allowlist, which
    `pluginhost` does (T018).

### Implementation

- [X] T011 [P] Implement `Ref`, `ParseRef`, and `String` in `internal/esc/ref.go`
  per data-model.md (makes T004 pass).
- [X] T012 [P] Implement `Category`, `Failure`, and `classify(ctx, err)` in
  `internal/esc/failure.go` per research R5.
  - Read the status from `GenericOpenAPIError.Model()` as an `esc.Error` or
    `*esc.Error` code.
  - Never call `Body()`, and never wrap the SDK error (makes T005 pass).
- [X] T013 [P] Implement `environmentVariables` extraction in
  `internal/esc/values.go`. It returns `map[string]entry{value string; secret bool}`
  from `*esc.Environment`'s `Properties["environmentVariables"].Value.(map[string]esc.Value)`
  (makes T006 pass).
- [X] T014 [P] Implement backend, token, and org discovery in
  `internal/esc/auth.go` per research R4.
  - Read only. Never call `MkdirAll` or write.
  - Return a `Failure` with no reference for a failure that happens before
    any reference is involved (makes T007 pass).
- [X] T015 Move `stackSettingsNameCandidates` and `readStackSettingsFile` from
  `internal/cli/overview.go` to `internal/pulumi/stack_settings.go` as the
  exported `StackSettingsNameCandidates` and `ReadStackSettingsFile`.
  - Update every caller in `internal/cli/overview.go` and its tests.
  - Add the `StackEnvironment` type and parser to the same file.
  - Add `SelectedStack` in `internal/pulumi/workspace.go`.
  - Add `StackContext` and `LoadStackContext` in
    `internal/pulumi/stack_context.go`.
  - Add a `backend.url` field to the project struct in
    `internal/pulumi/pulumi.go` (makes T008 pass, and existing overview tests
    stay green).
- [X] T016 Implement the SDK wrapper in `internal/esc/client.go` (depends on T012, T013).
  - Build the `esc.Configuration` by hand, with a no-redirect client.
  - Build the auth context from the caller's context.
  - Implement `open(ctx, Ref) (vars, revisionKey, error)`, which wraps
    `OpenAndReadEnvironment` / `OpenAndReadEnvironmentAtVersion` in
    `recover()`.
  - Compute the revision key per research R7: the ETag through
    `EscAPI.GetEnvironmentETag`, the number for `@N`, or `@tag` through
    `EscAPI.GetEnvironmentRevisionTag`.
  - This makes T009 pass.
- [X] T017 Implement `Options`, `NewSource`, `WithSource`, `SourceFromContext`,
  and `(*Source).Delivery` in `internal/esc/source.go` (depends on T011–T016).
  `(*Source).Variables` is added in T049.
  - Memoize and singleflight per reference.
  - Charge the wait budget per the data model: time counts only while at
    least one ESC call is in flight, overlapping calls count once, and each
    call's deadline is the remaining budget.
  - Resolve the selection in this order: per plugin, then global, then the
    stack's imports (when `UseStack` is true).
  - De-duplicate warnings per `(Ref, Category)`, then log at Warn
    (`component=esc`, `operation=resolve`, `ref`, `category`) and call
    `Options.Warn`.
  - This makes T010 pass.

**Checkpoint**: `go test -race ./internal/esc/... ./internal/pulumi/...` passes.

---

## Phase 3: User Story 1 - Plugins use the stack's ESC environment (Priority: P1) 🎯 MVP

**Goal**: With nothing exported, a plugin receives the variables it advertises
from the ESC environments the stack imports. ESC wins over the shell, and
nothing reaches a plugin that did not ask for it.

**Independent Test**: The `esctest` server serves a stack import that
defines `FINFOCUS_RECORDER_OUTPUT_DIR`. With the shell pointing that variable
at a different directory, the recorder writes its recordings into the
ESC-supplied directory. A variable the recorder does not advertise does not
reach it.

### Tests for User Story 1 (write first, must fail)

- [X] T018 [P] [US1] Precedence and digest table tests in
  `internal/pluginhost/settings_env_test.go`, extending
  `TestResolveDeliveredEnv_Precedence`.
  - An ESC value for a described key beats the parent, project, global, and
    default sources, and the name appears in `Delivered.Overrides`.
  - An ESC name that is not allowed is ignored.
  - Names ESC does not supply keep today's order (FR-007).
  - Reserved names (`pluginsdk.EnvPort`, `EnvLogFile`, `EnvTraceID`) are
    never taken from ESC.
  - With an empty `esc.Delivery`, both `Pairs` and `Digest` are
    byte-identical to the pre-change function for every existing case
    (SC-002). Capture the expected digests as constants before changing the
    code.
  - With ESC, the digest is unchanged when values change at the same
    revision key, and changes when the revision key or the name set changes
    (FR-014, SC-007).
  - The debug log lists `esc_env` names and `esc_refs`, and contains no
    fixture value (FR-016).
  - `Delivered.Origins` holds the winning reference for each delivered ESC
    name, and only for allowed names.
- [X] T019 [P] [US1] Override test in `internal/pluginhost/settings_inject_test.go`
  (`//go:build unix`, not parallel). It follows the pattern of
  `TestStartPlugin_ParentEnvWinsAndPortStays`.
  - With `t.Setenv("X_TOKEN","shell")` and a `Delivered` whose `Overrides`
    contains `X_TOKEN=esc`, the script plugin's dumped environment has
    exactly one `X_TOKEN`, and its value is `esc`.
  - `FINFOCUS_PLUGIN_PORT` still holds the port the host chose, even when
    ESC tried to override it.
- [X] T020 [P] [US1] Restart test in `internal/pluginhost/settings_restart_test.go`
  (not parallel).
  - Use a `recordingLauncher` and a ctx that carries an `esc.Source` built
    over `esctest`.
  - A name that is allowed only by the descriptor `GetPluginInfo` returns
    causes exactly one restart, and the second launch carries the ESC pair.
  - `esctest` records exactly one open.
- [X] T021 [P] [US1] Engine cache-key test in `internal/engine/engine_penv_test.go`.
  - Two clients whose `DeliveredDigest` comes from ESC deliveries that differ
    only in rotating values produce the same `/penv-` suffix.
  - A changed revision key produces a different suffix.
- [X] T022 [P] [US1] CLI attachment tests in `internal/cli/esc_source_test.go`
  (not parallel: it builds root commands).
  - `PersistentPreRunE` puts an `esc.Source` on the command context.
  - The stack comes from `--stack` when the command defines it, else from
    the workspace file.
  - The fake Pulumi runner is never called.
  - Two dispatched `mcp-server` tool calls receive two distinct `Source`
    values (FR-012).
  - The overview pipeline's plugin-open context carries the `Source` while
    stack loading still runs concurrently. Assert that phase events stay 1..6.
- [X] T023 [P] [US1] End-to-end integration test in
  `test/integration/esc_plugin_env_test.go`. Skip when
  `bin/finfocus-plugin-recorder` is missing, as `recorder_test.go` does.
  - Set `FINFOCUS_HOME` to a temp dir.
  - Point `PULUMI_BACKEND_URL` at `esctest` and set `PULUMI_ACCESS_TOKEN`.
  - Write a temp `Pulumi.yaml`, a `Pulumi.dev.yaml` with `environment: [proj/env]`,
    and a workspace file under a temp `PULUMI_HOME`.
  - Set `t.Setenv("FINFOCUS_RECORDER_OUTPUT_DIR", shellDir)`, while ESC
    supplies `escDir`.
  - Launch through `pluginhost.NewClient` with
    `pluginhost.WithInstallName(ctx, "recorder")` and the `Source`, then call
    `GetProjectedCost`.
  - Assert that the recordings exist in `escDir` and that `shellDir` is
    empty.
  - Assert that an ESC-only `UNRELATED_SECRET` never appears in the recorded
    files or logs.

### Implementation for User Story 1

- [X] T024 [US1] Change `ResolveDeliveredEnv` in `internal/pluginhost/settings_env.go`
  to return `Delivered{Pairs, Overrides, Digest, Origins}` and to take an
  `esc.Delivery` argument.
  - Apply the precedence and the allowed set (descriptor `Env` names) per
    `contracts/esc-package.md`.
  - Hash ESC names with `name\0esc\0<ref>\0<revision-key>\0` and every other
    source exactly as before.
  - Log `esc_env` and `esc_refs`.
  - `resolveInjected` obtains the `Delivery` from `esc.SourceFromContext`, or
    an empty `Delivery` when no `Source` is present.
  - Update every caller and the existing tests (makes T018 pass).
- [X] T025 [US1] Change `WithDeliveredEnv` and `DeliveredEnvFromContext` in
  `internal/pluginhost/settings_env.go`, and `NewClient` /
  `applyAdvertisedSettings` in `internal/pluginhost/host.go`, to carry
  `Delivered`.
  - The restart comparison (`sameStringSet`) covers both `Pairs` and the
    `Overrides` keys.
  - `Client.DeliveredDigest` is taken from `Delivered.Digest`.
  - This makes T020 pass.
- [X] T026 [US1] Change `appendDeliveredEnv` in `internal/pluginhost/process.go`.
  - For each name in `Overrides`, remove the inherited `KEY=` entry from the
    child environ before appending.
  - Keep the reserved-name guard.
  - Add a comment saying the stdio launcher (`stdio.go`) does not apply
    delivered env and has no production caller (research R8). This makes T019
    pass.
- [X] T027 [US1] Attach the `esc.Source` in `PersistentPreRunE` in `internal/cli/root.go`.
  - Build `pulumi.LoadStackContext(FindProject(cwd), stackFlag)`, where
    `stackFlag` is read with `cmd.Flags().Lookup("stack")` when the command
    defines it.
  - A `LoadStackContext` error is logged at debug level and gives a zero
    context.
  - Set `Budget: 5*time.Second`, `Getenv: os.Getenv`, and
    `Warn: func(s string){ cmd.PrintErrln("Warning: " + s) }`.
  - Global and per-plugin options stay nil until US3.
- [X] T028 [US1] Make the overview path use the attached source.
  - In `internal/cli/overview_pipeline.go` (`Run` / `startPluginOpen`),
    confirm that the parent ctx passed to `startPluginOpen` is the command
    context that carries the `Source`. Thread it explicitly if the pipeline
    rebuilds a context.
  - This makes the overview part of T022 pass. Warning deferral in TUI mode
    is US2 (T034, then T035).
- [X] T029 [US1] In `internal/cli/web_data.go`:
  - `reloadPlugins` and `closeReloaded` attach a new `esc.Source` with the
    same options before calling `openPlugins` (FR-012).
  - Store the session's current `Source` on `webData` as `escSource`, behind
    a mutex.
  - The initial pipeline run records its `Source` the same way.
- [X] T030 [US1] In `internal/cli/analyzer_serve.go`, attach a `Source` with
  `UseStack: false` before `reg.Open` (research R10).
  - Add a test case to `internal/cli/esc_source_test.go` asserting that
    analyzer serve ignores stack imports.
- [X] T031 [US1] Run `go test -race -shuffle=on -count=3 ./internal/esc/... ./internal/pluginhost/... ./internal/cli/... ./internal/engine/...`
  and `go test ./test/integration/ -run ESC` after `make build-recorder`.
  - Fix any failure at its cause; never weaken an assertion.

**Checkpoint**: The MVP works. Quickstart steps 1 and 2 pass against the real
binary.

---

## Phase 4: User Story 2 - Existing setups keep working when ESC is unavailable (Priority: P1)

**Goal**: No ESC means no change. Every ESC failure falls back with exactly
one categorized warning, and the command succeeds.

**Independent Test**: Run `cost projected` against `esctest` in each failure
mode. Each run exits 0, prints one `Warning: ESC …` line on stderr, and the
plugin still gets its shell value.

### Tests for User Story 2 (write first, must fail)

- [X] T032 [P] [US2] CLI fallback tests in `internal/cli/esc_fallback_test.go`
  (not parallel). For each case, run `cost projected --pulumi-json <fixture>`
  inside a temp project:
  - `404` → `not found or no access`;
  - `403` → `access_denied`;
  - a missing token with `PULUMI_CREDENTIALS_PATH` pointing at an empty dir
    → `not_logged_in` and the `PULUMI_ACCESS_TOKEN` hint;
  - `PULUMI_BACKEND_URL=file://…` → `unsupported_backend`;
  - an inline `environment:` with `values:` → `unsupported_definition`.

  In every case: exit code 0, exactly one `Warning: ESC` line on stderr,
  stdout unchanged from a no-ESC run, and no fixture value anywhere in
  stdout, stderr, or the log file.
- [X] T033 [P] [US2] No-regression test in `internal/cli/esc_fallback_test.go`.
  - With no `environment:` key and no `esc` config, `esctest` receives zero
    requests.
  - The recorder's delivered pairs and `DeliveredDigest` equal those of a run
    with the ESC code path disabled (no `Source` on the ctx) (SC-002).
- [X] T034 [P] [US2] Overview TUI warning-deferral test in
  `internal/cli/overview_esc_warning_test.go`.
  - While the program runs, nothing is written to stderr.
  - After the model quits, the collected warning is printed once.

### Implementation for User Story 2

- [X] T035 [US2] Finish the user-facing warnings.
  - In `Failure.Error` in `internal/esc/failure.go`, implement the exact
    texts from `contracts/esc-package.md`, including the `not_logged_in` hint
    and the `not_found` "not found or no access" form.
  - In the warning de-duplication in `internal/esc/source.go`, key by
    `(Ref, Category)`. The `Source` is shared across the restart in
    `applyAdvertisedSettings`, so a restarted plugin does not warn twice.
  - In TUI mode (`internal/cli/overview.go`), replace `Options.Warn` with a
    collector, and print the collected warnings to stderr after the Bubble
    Tea program exits.
  - This makes T032–T034 pass without changing the tests.
- [X] T036 [US2] Update `docs/src/content/docs/guides/troubleshooting.md` with
  an "ESC environment skipped" section. It lists each category, its cause,
  and the fix (log in, set `PULUMI_ACCESS_TOKEN`, check the reference or
  access, self-managed backend, inline definition, network).

**Checkpoint**: Quickstart steps 3 and 4 pass. Users without ESC are
provably unaffected.

---

## Phase 5: User Story 3 - Choose the environment explicitly (Priority: P2)

**Goal**: `esc.environments` and `esc.plugins.<name>.environments` override
the stack imports, `[]` disables ESC, and a malformed reference is a
validation error.

**Independent Test**: Set `esc.plugins.recorder.environments` to an
environment that differs from the stack import. The recorder gets the
configured environment's value, while another plugin keeps the stack's.
`config set esc.environments ''` produces zero ESC requests.

### Tests for User Story 3 (write first, must fail)

- [X] T037 [P] [US3] Config model tests in `internal/config/esc_config_test.go`.
  - `ESCConfig` round-trips through Hujson.
  - A nil `Environments` (not set) differs from an empty slice (disabled).
  - A project `esc` section replaces the global one as a whole.
  - `Get` / `Set` / `List` handle `esc.environments` and
    `esc.plugins.<name>.environments`.
  - `config set` with comma-separated input works, and `''` writes `[]`
    (`contracts/config.md`).
  - `MaskedForDisplay` leaves `esc` values visible.
- [X] T038 [P] [US3] Validation tests in `internal/config/validate_test.go`.
  - Each malformed-reference message from `contracts/config.md` is an error
    at its key path.
  - An unknown plugin name under `esc.plugins` is a warning.
  - The wrong JSON type is a type error.
  - `ValidateConfigSource` makes no network call.
- [X] T039 [P] [US3] Selection tests in `internal/esc/source_test.go`.
  - The per-plugin setting beats the global one, and the global beats the
    stack.
  - An empty per-plugin list disables ESC for that plugin only.
  - An empty global list disables ESC for every plugin without a per-plugin
    list.
- [X] T040 [P] [US3] CLI tests in `internal/cli/config_validate_test.go` and
  `internal/cli/esc_source_test.go`.
  - `config validate` exits 2 and names `esc.environments[0]` for `a//b`.
  - `cost projected`, `overview --plain`, and `--web` with the same config
    each exit 2 before any plugin launches. The root `PersistentPreRunE`
    parse covers every command.
  - `config set esc.environments 'billing/aws'` succeeds while the bad value
    is present, so it can be repaired. `config validate` still reports the
    error.
  - The root `Source` uses `Global` and `PerPlugin` from the merged config,
    which is `InitializedGlobalConfig` and never `config.New`, so the project
    tier is not lost.

### Implementation for User Story 3

- [X] T041 [US3] Add `ESCConfig` / `ESCPluginConfig` (with `Environments`) and
  `Config.ESC` in `internal/config/config.go`.
  - Also add the key to the `UnmarshalJSON` delete list and the
    `Get`/`Set`/`List` cases.
  - In `internal/config/merge.go`, add the `esc` key constant, the
    `knownTopLevelKeys` entry, and the `unmarshalSection` case.
  - This makes T037 pass.
- [X] T042 [US3] In `internal/config/descriptor.go`, add the `KnownSettings`
  entry for `esc.environments` (`KindStringList`). Also add a dynamic
  `escPluginSetting` lookup for `esc.plugins.<name>.environments`, modeled on
  `pluginSetting`, so that `PatchFile` and the dashboard Config page accept
  the key.
- [X] T043 [US3] Validation rules in `internal/config/validate.go`.
  - Check every entry with `esc.ParseRef`; this is a new import of
    `internal/esc` from `internal/config`, allowed by the import rules.
  - Warn about unknown plugin names.
  - Use the exact messages in `contracts/config.md`. This makes T038 and
    T040 pass.
- [X] T044 [US3] In `internal/cli/root.go`, map the merged `Config.ESC` into
  `esc.Options.Global` and `PerPlugin` (`Environments`).
  - Before building the `Source`, parse every configured reference with
    `esc.ParseRef`.
  - Return the first error through `toValidationError` (exit 2), except when
    the command is in the `config` subtree, so it can still repair or report
    the value.
  - This makes T039 and T040 pass.
- [X] T045 [US3] Document the `esc` section in
  `docs/src/content/docs/reference/config-reference.md`: its shape, every key,
  the meaning of nil versus `[]`, the project-replaces-global rule, `config set`
  syntax, and the validation messages.

**Checkpoint**: Explicit selection works without any stack import.

---

## Phase 6: User Story 4 - Allow additional variables for a plugin (Priority: P2)

**Goal**: `esc.plugins.<name>.variables` adds names a plugin may receive from
ESC, for that plugin only.

**Independent Test**: Allow the AWS credential variables for one plugin. It
receives them from ESC, and the scorer, sharing the same environment,
receives none of them.

### Tests for User Story 4 (write first, must fail)

- [X] T046 [P] [US4] Allowlist tests in `internal/pluginhost/settings_env_test.go`.
  - The allowed set = descriptor `Env` names ∪ the configured `variables`.
  - A configured variable with no descriptor is delivered under its own name.
  - Plugin B, which has no list, receives none of plugin A's listed names
    from the same environment (SC-006).
- [X] T047 [P] [US4] Config tests.
  - In `internal/config/esc_config_test.go`: `esc.plugins.<name>.variables`
    round-trips and `config set` accepts it.
  - In `internal/config/validate_test.go`: an invalid name such as `1BAD`,
    and the reserved `FINFOCUS_PLUGIN_PORT`, the log-file variable, and the
    trace-ID variable, are errors with the `contracts/config.md` messages.

### Implementation for User Story 4

- [X] T048 [US4] Add `Variables []string` to `ESCPluginConfig` in
  `internal/config/config.go`. In `internal/config/descriptor.go`, add the
  dynamic key `esc.plugins.<name>.variables`. In `internal/config/validate.go`,
  add the name and reserved-name rules (makes T047 pass).
- [X] T049 [US4] Pass `PluginOptions.Variables` from `internal/cli/root.go`
  into `esc.Options`. Add the configured names to the allowed set where
  `resolveInjected` in `internal/pluginhost/settings_env.go` builds it, by
  exposing them from the `Source` as `(*Source).Variables(plugin)` (makes
  T046 pass).

**Checkpoint**: Plugins that rely on SDK-default variables can use ESC
without leaking those variables to other plugins.

---

## Phase 7: User Story 5 - See where a plugin setting comes from (Priority: P3)

**Goal**: The dashboard's plugin settings report `effectiveSource: "esc"`
and the reference, never the value.

**Independent Test**: With ESC supplying a recorder setting,
`GET /api/plugins/recorder/settings` returns `effectiveSource: "esc"` and
`escEnvironment` for that row, with no `value`.

### Tests for User Story 5 (write first, must fail)

- [X] T050 [P] [US5] Settings-view tests in `internal/cli/plugin_settings_web_test.go`.
  - `pluginSettingValues` with a `Delivered.Origins` entry yields
    `EffectiveSource: "esc"` and `ESCEnvironment: "<ref>"`.
  - When the shell also has the variable, `envSet` stays true.
  - When ESC failed, no row reports `esc`.
  - `requirePluginSettings` treats an ESC-supplied required setting as
    satisfied.
  - **Parity**: given the same `esc.Source` and inputs, `pluginSettingValues`
    reports exactly the origins in the `Delivered.Origins` that
    `pluginhost.ResolveDeliveredEnv` returns. T020 asserts that `NewClient`
    launches with that same `Delivered`. Together they prove the web view and
    the plugin launch use one resolution.
- [X] T051 [P] [US5] Handler test in `internal/webui/handlers_plugins_test.go`.
  - The JSON from `handlePluginSettingsGet` includes `escEnvironment` only
    for `esc` rows.
  - A secret row never has a `value` property whatever its source.
  - The body passes `RedactJSON` unchanged.
- [X] T052 [P] [US5] A frontend test in `internal/webui/frontendtest/`,
  following the existing plugins-view tests. `fieldHelp` renders the ESC
  sentence for a row whose `effectiveSource` is `esc`.

### Implementation for User Story 5

- [X] T053 [US5] Add ``ESCEnvironment string `json:"escEnvironment,omitempty"` ``
  to `PluginSettingValue` in `internal/webui/plugins_api.go`, and document the
  `esc` value of `EffectiveSource`.
- [X] T054 [US5] Use the shared resolution in `internal/cli/plugin_settings_web.go`.
  - `loadPluginSettingState` calls `pluginhost.ResolveDeliveredEnv` with the
    `Delivery` from the session `Source` (`webData.escSource`, passed in
    through `wirePlugins` in `internal/cli/web_plugins.go`).
  - Map `Delivered.Origins` into `pluginSettingValues`.
  - Extend `effectiveSource` with `esc` as the highest precedence.
  - Make `requirePluginSettings` accept an ESC-supplied value.
  - The web code computes no ESC fact itself (`contracts/http-api.md`).
  - This makes T050 and T051 pass.
- [X] T055 [US5] In `fieldHelp` in `internal/webui/static/views/plugins.js`,
  add one sentence for `row.effectiveSource === 'esc'`: "Supplied by Pulumi
  ESC environment `row.escEnvironment`, which overrides the shell and both
  files." It sits
  next to the existing `envSet` sentence (makes T052 pass).

**Checkpoint**: The settings view tells the truth about where a value comes
from.

---

## Phase 8: Web Parity (Constitution Principle VII)

**Purpose**: Verify the hook against the spec's Web UI Fit section, and hand
off UX polish.

- [X] T056 [P] Reconcile `specs/1730-pulumi-esc-plugin-credentials/contracts/http-api.md`
  with the implemented Go method names. They are
  `cli.pluginSettingsView` → `cli.loadPluginSettingState` →
  `cli.pluginSettingValues` → `pluginhost.ResolveDeliveredEnv` /
  `esc.(*Source).Delivery`, plus `cli.preparePluginSettings` →
  `cli.requirePluginSettings`. Correct any drift.
- [X] T057 Prerequisite: the `finfocus-test/recorder` ESC environment from
  quickstart §1, and `esc.plugins.recorder.environments` set in a temporary
  `FINFOCUS_HOME`. Then render Plugins → recorder → Settings with Playwright
  from `../finfocus-demo` (stack `dev`), running
  `PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port <port>`
  with ESC supplying one recorder setting.
  - Compare the result with the spec's Web UI Fit section (Status cell, help
    sentence, no value).
  - Record the date and outcome under "Render evidence" in `spec.md`.
  - Save screenshots only under `.playwright-mcp/`.
  - Stop the server with an anchored `pkill -f '^<repo>/bin/finfocus --web'`
    and check `ps` for orphaned plugins.
- [X] T058 [P] Update `docs/src/content/docs/reference/web-ui.md` for the
  `effectiveSource: "esc"` and `escEnvironment` fields of the plugin settings
  endpoint.
- [X] T059 Final task, done after T060–T067: file one GitHub issue labeled
  `web-ui`, covering UX only.
  - **Scope**: Status-cell design for ESC-supplied rows, copy, and
    accessibility of the ESC source text in Plugins → Settings.
  - **Exclusions**: it must state that no engine, API, redaction, or
    data-shape work is in scope, and link #1865 and this spec.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T060 [P] Write the user guide `docs/src/content/docs/guides/esc-credentials.md`:
  - setup (stack import or `esc` config)
  - precedence, with ESC over the shell called out as a behavior change
  - the allowlist and `variables`
  - fallback categories (link to the troubleshooting section from T036)
  - the Pulumi Cloud-only limitation
  - inline definitions not supported
  - `PULUMI_ACCESS_TOKEN` when the login is keychain-encrypted
  - the token trust rule (FR-020): self-hosted Pulumi Cloud users set
    `PULUMI_BACKEND_URL` or run `pulumi login`, because a `backend.url` in
    `Pulumi.yaml` alone never receives an environment-variable token
  - the imported-environment cache residual (research R7)
  - the analyzer behavior

  Link it from the guides index and the plugin-settings section of the
  user guide.
- [X] T061 [P] Update `docs/src/content/docs/reference/environment-variables.md`
  with the Pulumi variables finfocus now reads: `PULUMI_ACCESS_TOKEN`,
  `PULUMI_BACKEND_URL`, `PULUMI_HOME`, `PULUMI_CREDENTIALS_PATH`, and
  `PULUMI_DEFAULT_ORGANIZATION`, each with the reason.
- [X] T062 [P] Write `internal/esc/README.md` (package completeness, more than
  3 exported symbols): purpose, API, import rules, the redaction rule, and
  how to test with `esctest`.
- [X] T063 [P] Redaction sweep test in `test/integration/esc_redaction_test.go`.
  - Put the fixture secret `do-not-deliver-7f3a` in an `esctest` environment.
  - Run `cost projected --debug --format json` through `helpers.CLIHelper`
    with a log file.
  - Assert that the string is absent from stdout, stderr, the log file, the
    cache directory under `FINFOCUS_HOME`, and the
    `GET /api/plugins/recorder/settings` body (SC-005).
- [X] T064 Update the `CLAUDE.md` gotchas under "Plugin Host" and "Pulumi Integration":
  - ESC is the highest-precedence source, limited to allowed names.
  - `appendDeliveredEnv` overrides parent entries only for `Overrides`.
  - The cache key hashes the reference and revision key, never values.
  - The `esc` config is top-level, because `plugins.<name>` is injected.
  - The workspace file supplies the selected stack.
  - The SDK quirks: hand-built Configuration, `recover()`, and no `Body()`.

  Replace the "(planned)" Recent Changes line with the shipped description.
- [X] T065 Coverage check.
  - Run `go test -coverprofile` for `./internal/esc/...`,
    `./internal/pluginhost/...`, `./internal/config/...`, and the touched
    `./internal/cli/...` files.
  - Confirm at least 95% for `settings_env.go`, `process.go`
    (`appendDeliveredEnv`), and `internal/esc/source.go`, and at least 80%
    elsewhere.
  - Add tests for gaps with real behavior only.
- [X] T066 Run the full gates: `make validate`, `make test`, `make test-race`,
  `make test-integration`, `make lint` (extended timeout), `make docs-lint`,
  and `govulncheck`. Fix every finding at its cause, without changing
  `.golangci.yml`.
- [X] T067 Run `specs/1730-pulumi-esc-plugin-credentials/quickstart.md` steps
  1–6 against the real binary and `../finfocus-demo`.
  - Never commit the rewritten `Pulumi.dev.yaml`.
  - Restore it with `git -C ../finfocus-demo checkout Pulumi.dev.yaml`.
  - Record any deviation in `research.md` R12.

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: no dependencies.
- **Foundational (Phase 2)**: depends on T001 and T003. It blocks every user
  story.
- **US1 (Phase 3)**: depends on Phase 2. It is the MVP.
- **US2 (Phase 4)**: depends on US1, because fallback is observed through the
  delivery path US1 builds.
- **US3 (Phase 5)**: depends on Phase 2 and on T027 (the root attachment). It
  is independent of US2.
- **US4 (Phase 6)**: depends on US3's `ESCPluginConfig` (T041) and on US1's
  allowed-set code (T024).
- **US5 (Phase 7)**: depends on US1 (`Delivered.Origins` and
  `webData.escSource`). It is independent of US2–US4.
- **Web Parity (Phase 8)**: depends on US5. T059 is the last task of the
  feature.
- **Polish (Phase 9)**: depends on every story it documents. T066 and T067
  run last, before T059.

### Within each story

The tests are written first and fail. Implementation follows in the order
listed. The checkpoint passes before the next phase starts.

### Parallel opportunities

- **Phase 1**: T002 and T003.
- **Phase 2 tests**: T004–T009 (separate files). **Implementation**: T011–T014.
  T015 is independent of the `esc` files.
- **US1 tests**: T018–T023.
- **US3 tests**: T037–T040. **US4 tests**: T046 and T047. **US5 tests**:
  T050–T052.
- **Across stories**, after US1: US3 and US5 can proceed in parallel, and US2
  can proceed alongside them.
- **Polish**: T060–T063.

---

## Parallel Example: User Story 1

```bash
# Tests first, together (separate files):
Task: "T018 precedence and digest table tests in internal/pluginhost/settings_env_test.go"
Task: "T019 override test in internal/pluginhost/settings_inject_test.go"
Task: "T020 restart test in internal/pluginhost/settings_restart_test.go"
Task: "T021 cache-key test in internal/engine/engine_penv_test.go"
Task: "T022 CLI attachment tests in internal/cli/esc_source_test.go"
Task: "T023 integration test in test/integration/esc_plugin_env_test.go"

# Then the implementation, in order (shared files):
T024 → T025 → T026 → T027 → T028 → T029 → T030 → T031
```

---

## Implementation Strategy

### MVP first (User Story 1)

1. Phase 1, then Phase 2. The `internal/esc` package is complete and tested
   in isolation.
2. Phase 3 (US1). Stack imports reach the plugins, ESC wins, and nothing
   leaks.
3. **Stop and validate**: run quickstart steps 1 and 2 against
   `../finfocus-demo`.

### Incremental delivery

1. US1, then US2 (fallback and no-regression proof). This makes a shippable
   P1 pair.
2. US3, then US4 (explicit selection, then the allowlist).
3. US5 and the Web Parity phase (dashboard source display, render check).
4. Polish: guides, redaction sweep, coverage, gates, quickstart. File the
   `web-ui` issue last (T059).

---

## Notes

- Total: 67 tasks.
- The spike in research R12 already validated the library paths that
  Phase 2 implements. Re-use its findings, not its code.
- Commit only when the user asks.
