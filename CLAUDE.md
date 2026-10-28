# CLAUDE.md

Guidance for Claude Code in this repository. It lists commands and the
non-obvious rules that have caused real bugs. Feature behavior lives in
`specs/NNN-*/` and the code; this file points there instead of restating it.

## Critical Instructions

- **Do not run `git commit`** unless the user asks for a commit, directly or by
  invoking a workflow whose purpose is a commit (for example `/pick-issue`).
- **Always run `make lint` and `make test`** before claiming success.
- **Do not modify `.golangci.yml`** without explicit approval.
- The constitution (`.specify/memory/constitution.md`) wins over any runtime
  mode instruction (learning, explanatory). Principle VI forbids TODOs and
  stubs. Record a conflict with `/speckit.revisit`.

## Project Overview

FinFocus Core is a CLI and plugin host that calculates cloud costs from Pulumi
(and Terraform state) definitions: projected estimates, actual history, and
Kubernetes allocation, all through gRPC plugins. Sibling repos:
`finfocus-spec` (protos, `pluginsdk`) and the `finfocus-plugin-*` plugins
(aws-public, azure-public, ...). Cross-repo changes follow the constitution's
protocol. Plugin domain knowledge stays in the plugin; core and spec get
generic channels only.

## Specs

Specs, plans, and tasks live in `specs/NNN-*/`, produced by **Spec Kit**
(`.specify/` and the `/speckit-*` skills). It is the only pipeline: output
from other planning tools must become a Spec Kit feature folder before
implementation.

### Web parity (constitution Principle VII)

A user-facing CLI feature is not complete until `finfocus --web` reaches it.
Every spec carries a `Web UI Fit` section. A feature with no web surface
records `Not applicable` and a reason there and is exempt from the rest.
Otherwise the `internal/webui` hook calls the same Go method as the CLI and
the contract names it, and completion files one UX-only `web-ui` issue.
Engine, API, or data-shape work never goes in that issue. Layout:
`docs/src/content/docs/architecture/web-ui.md`.

- To check a fit, run the server from a populated project (`initial-30` or
  `../finfocus-demo`) and render it with playwright-mcp. Both use an empty
  passphrase:
  `PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port <port>`.
  The MCP can write screenshots only under the repo (`.playwright-mcp/`),
  not the scratchpad.
  Stop the server with an anchored `pkill -f '^<path>/finfocus --web'`.

## Commands

```bash
make tools             # Install the toolchain pinned in mise.toml
make build             # bin/finfocus
make test              # Unit tests: internal/, pkg/, and the plugin modules
make test-race
make test-integration  # Slower; see Integration Tests below
make test-e2e          # Needs AWS credentials and Pulumi
make test-e2e-kind     # kind cluster: run-rate, then historical (Prometheus)
make lint              # golangci-lint, markdownlint, actionlint
make validate          # go mod tidy -diff, go vet
make docs-lint
make docs-serve        # Astro site at http://localhost:4321/finfocus/
make check-plugin-boundaries
make test-kubernetes   # Also test-scorer, test-prometheus (nested plugin modules)
make install-kubernetes  # Also install-scorer, install-prometheus, install-recorder
```

```bash
go test -run TestName ./internal/cli/
go test -race -shuffle=on -count=3 ./internal/<pkg>/...   # Parallel-safety check
./bin/finfocus plugin list --output json
```

- `make lint` checks golangci-lint's version against `mise.toml` and runs
  `mise which golangci-lint`. Install it with `mise install` (`make tools`).
  Do not point `GOLANGCI_LINT` at `$(HOME)/go/bin/golangci-lint`: that copy
  can lag the pin and reject the current config.
- `pkill golangci-lint || true` clears a parallel-runner conflict. Lint can run
  longer than five minutes.
- After a rebase, run `go vet ./...` before trusting `make test`. A widened
  interface (for example `engine.ResourcePricer`) breaks test doubles that
  another merged PR added, with no git conflict.

## Architecture

| Package | Role |
| --- | --- |
| `internal/cli/` | Cobra commands, routed through `ax.Execute` (ax-go) |
| `internal/engine/` | Cost orchestration, cluster allocation, rendering |
| `internal/pluginhost/` | gRPC plugin launch, connect, cleanup |
| `internal/registry/` | Discovery in `<home>/plugins/<name>/<version>/`, install |
| `internal/ingest/` | Pulumi plan and state parsing |
| `internal/stackload/` | Shared Pulumi state and preview snapshots, inventory, and safe input status |
| `internal/proto/` | Adapter: descriptors, pre-flight validation |
| `internal/router/` | Plugin routing with priority and fallback |
| `internal/analyzer/` | Pulumi Analyzer gRPC server |
| `internal/tui/` | Bubble Tea v2 / Lipgloss v2 (`charm.land` imports) |
| `internal/config/` | Two-tier config |
| `plugins/{kubernetes,scorer,prometheus}/` | Nested modules; never import core |

Entry point `cmd/finfocus/main.go`. Exit codes: 0, 1 (ax `ExitInternal`), and
the budget code from `cost --exit-code` (default 1), preserved through
`ax.Execute` by `cli.toAxExitError`. User-input errors (flag combinations,
malformed input files, bad dates) exit 2 through `toValidationError`
(`ax.ExitValidation`). Codes 3 and 4 (`ExitNetwork`, `ExitAuth`) are not used yet.

### Configuration Resolution

- Project settings (config, dismissals): `--project-dir`, then
  `FINFOCUS_PROJECT_DIR`, then the nearest `Pulumi.yaml` (`$PROJECT/.finfocus/`),
  then `~/.finfocus/`.
- Global resources (plugins, cache, logs): `FINFOCUS_HOME`, then
  `PULUMI_HOME/finfocus`, then `~/.finfocus/`. With none of those, it is
  `./.finfocus` (`config.UsesWorkingDirFallback`).
- Project `config.hujson` overrides global at the top-level key (shallow
  merge). The global loader migrates a legacy `config.yaml` to Hujson on first
  read; a legacy project `config.yaml` is read as an overlay and left as it is.

## Key Patterns

- `RunE`, not `Run`. `cmd.Printf`, not `fmt.Printf`. Defer cleanup right after
  acquiring a resource. Dates accept `2006-01-02` and RFC3339.
- Pre-flight validation (`internal/proto/`, `pluginsdk` validators) returns a
  `$0` placeholder with a `VALIDATION:` note (plugin errors use `ERROR:`) and
  still calls the plugin for valid resources. A resource with `ref.*` tags and
  no SKU uses `ValidateProjectedCostRequestLenient`.
- Logging: `--debug` > `FINFOCUS_LOG_LEVEL`/`FINFOCUS_LOG_FORMAT` > config >
  info/console. `FINFOCUS_TRACE_ID` injects a trace id.
- `ax.Execute` mounts `--format json|human`, `--dry-run`, `--yes`, and
  `--idempotency-key` on every command, plus `__schema` and `mcp-server`
  (`--mcp` is the stdio alias). A command must not redeclare `--format`;
  Cobra rejects it.

## Testing

- **Testify only** (`require` to stop, `assert` to continue); see `AGENTS.md`.
  Never write `if got != want { t.Errorf(...) }` or raw `t.Fatalf`.
- **TUI changes**: render the view and read it, then run
  `go test -run TestGolden ./internal/tui/...` (`UPDATE_GOLDEN=1` to
  regenerate). `assert.Contains` alone is not enough.
- **Expected plugin failures**: log expected errors with `t.Logf`; fail only
  when the required error is absent. `connection refused`, `context deadline
  exceeded`, or `EOF` in logs of a passing test are normal.
- Coverage targets (80%, 95% on critical paths) are in the constitution.

### Parallel Tests

Call `t.Parallel()` at the top level and in each `t.Run`, except:

- Tests that touch process-wide state: `t.Setenv`, `t.Chdir`, `os.Setenv`,
  package-level variables, `config.ResetGlobalConfigForTest`/`SetGlobalConfig`,
  or anything that builds a root command (`NewRootCmd*` sets the resolved
  project dir), including helpers such as `stubHome` and `WithEnv`.
- A test that writes a binary or script and then executes it. A concurrent
  fork inherits the write fd and the exec fails with `text file busy`
  (golang/go#22315).
- Mark them `//nolint:paralleltest // <reason>` above `func` (or on the
  `t.Run`/`for` line for some subtests) only when the state is reached
  through a helper. `paralleltest` already skips a test that calls
  `t.Setenv` or `t.Chdir` itself, and `nolintlint` then fails the marker as
  unused; give that test a plain comment instead.
- Do not share a mutable fixture across subtests; production code mutates
  protos in place. In a parent with parallel subtests use `t.Cleanup`, never
  `defer`.

### E2E

- `test/e2e/` is its own module and must run the real `finfocus` binary;
  never stub cost values. AWS E2E needs `PULUMI_CONFIG_PASSPHRASE` and
  `~/.pulumi/bin` on `PATH`.
- The run-rate half of `make test-e2e-kind` installs aws-public into the real
  `~/.finfocus`. `TestCostCluster_KindHistorical` uses a temp home: run
  `test/e2e/kind/setup.sh`, apply `test/e2e/kind/prometheus.yaml`, then
  `go test -tags e2e_kind -run '^TestCostCluster_KindHistorical$' ./...` from
  `test/e2e` (its own module; a bare run from the repo root skips it) with
  `FINFOCUS_BINARY` set.
- Real-stack checks use `../finfocus-demo` (stack `dev` in Pulumi Cloud, AWS
  creds from an ESC environment attached with `pulumi config env add`; it
  rewrites the tracked `Pulumi.dev.yaml`; never commit that change, since the
  demo's CI has its own credentials). `pulumi preview` needs
  no spend; web Cost reads deployed state plus history, while Estimate and
  Recommendations include preview-only creates through the shared inventory. Always `pulumi destroy` afterwards. Never
  `pkill -f <name>` or `pgrep -f <name>` from a shell whose own command line
  contains the name: it matches and kills the shell. Anchor the pattern.
- The aws-public router starts a per-region child
  (`finfocus-plugin-aws-public-<region>`) that can outlive the router when
  finfocus is SIGKILLed or the child leaves the plugin's process group. Check
  `ps` for orphans after local runs.
- Remote-write fixtures need `storage.tsdb.out_of_order_time_window` in the
  Prometheus config and a window relative to now. A fixed date is rejected as
  out of bounds once a newer sample exists, and later falls out of the 15-day
  retention.
- `make test-e2e-kind` pins `E2E_AWS_PUBLIC_VERSION` (Renovate bumps it).
- Web plugin e2e uses the production seams `FINFOCUS_CATALOG_FILE` and
  `FINFOCUS_GITHUB_API` (a stand-in catalog and GitHub API, not a supported
  custom registry). `webEnvironment` strips every `FINFOCUS_*` value before
  `fixtureEnv` is appended, so pass those two through `fixtureEnv`.
  `GITHUB_TOKEN` is not stripped. Do not mention the seams in user docs.

### Integration Tests (`test/integration/`)

- `helpers.CLIHelper.Execute` must go through `ax.Execute`; a bare
  `cli.NewRootCmd().Execute()` lacks `--format`, `--dry-run`, `--yes`, and
  `--idempotency-key`.
- Tests built with `helpers.NewCLIHelper(t)` and no `WithEnv` read the real
  `~/.finfocus/config.hujson`. Compare the failure *set* with a pre-change
  baseline, not the count.
- Setting `HOME` for a `go`/`make` child moves Go's caches when `GOPATH` is not
  exported (CI); cleanup then fails on read-only module files. Pass the real
  `GOMODCACHE`/`GOCACHE` (see `scaffold_build_test.go`). Reproduce with
  `env -u GOPATH go test ...`.
- `config.yaml` fixtures are intentional legacy input for auto-migration.

## Releases

- release-please manages `CHANGELOG.md` and versions; never edit them by hand.
  A `feat` below 1.0 bumps the patch.
- A new nested plugin goes into `release-please-config.json` with
  `initial-version` and its path in the root package's `exclude-paths`. Do
  not add it to `.release-please-manifest.json`: a hand-written `0.1.0` makes
  the first release `0.1.1`. The release PR adds the entry.
- `release-monorepo-plugin.yml` builds any `<name>-vX.Y.Z` tag from
  `plugins/<name>/cmd`; a new plugin needs no workflow change. Plugin tag
  prefixes must not start with `v`.
- Nested modules must pin the same `finfocus-spec` as the root `go.mod`; CI
  compares them.

## Pulumi Integration

- `pulumi preview --json` nests resources under `newState`; ingestion must read
  `inputs` and `type` there or plugins get `InvalidArgument`. A step has no
  top-level `type`: call `PulumiStep.ResourceType()`, never read `step.Type`
  (the overview showed no costs for preview-only resources until it did).
  Test fixtures must carry the type only under `newState`, as Pulumi emits it.
- A default `pulumi preview --json` omits unchanged resources but still
  reports their count in `changeSummary.same`; `--show-sames` lists them.
  `internal/stackload` merges state and preview for `overview`, `cost projected`,
  `estimate`, `recommendations`, and web callbacks. Estimate and recommendations
  cover the live inventory, including unchanged state and preview-only creates.
  Projected budgets and `summary.totalMonthly` retain the plan total;
  `finfocus.stack` reports the separate stack total and safe completeness status.
  `finfocus.emptyPlan` depends only on priceable plan resources, even with state.
  Forecast retains the plan-origin total; actual costs use deployed inventory
  plus existing history enrichment (`specs/1733-shared-stack-loader/`).
  A saved preview whose URNs name another stack or project than the
  surrounding project's state is not merged (`not_requested`). Plan resources
  keep plan-scoped `ref.*` tags (and cache keys); only the stack total reprices
  one whose refs resolve differently against the full inventory. Test fixtures
  must pair a preview and state from the same stack and project, or the loader
  now drops the state.
- `parent` is the stack, not a pricing reference. Cross-resource refs follow
  `propertyDependencies` only and become `ref.<property>.{urn,type,region,sku}`
  tags (`specs/618-cross-resource-refs/`). The unknown sentinel
  `04da6b54-80e4-46f7-96ec-b56ff0331ba9` is never sent as a tag, SKU, or
  region. A referenced SKU is never copied into the child's `Sku`.
- **No credential or Pulumi secret reaches a plugin.** One rule each:
  `config.IsCredentialName` (which `engine.isCredentialKey` calls) /
  `skipDottedSegment` for names and `history.IsPulumiSecret`
  for values. They apply to collapsed tags, dotted keys
  (`specs/619-dotted-tag-keys/`), `ResourceDescriptor.attributes`
  (`specs/621-k8s-workload-projected-cost/`), actual-cost tags, and
  `EstimateCost` attributes. Do not add a second rule.
- Anything new sent to a plugin must be in the projected cache key; that is
  why keys end in `/tags-`, `/refs-`, `/attrs-` digests and feature suffixes
  (`/pricing-spec`, `/growth-v1`, `/penv-`, `/profile-dev|prod|burst`). `/penv-` is the digest of the
  clients that engine retained at construction. A settings or validate probe
  must not update another engine's key. An empty digest omits the suffix.
  Tests of the process-wide helper must not run in parallel and must call
  `ResetPluginEnvDigestsForTest`.
- `__`-prefixed inputs (`__defaults`, `__provider`) are Pulumi internals; filter
  them from anything shown to users.
- Analyzer import: `pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"`
  (not `.../proto/go/pulumirpc`).
- `analyzer serve` does not open stack ESC imports. Pulumi already put those
  variables in the process environment, and the stack is unknown until
  `ConfigureStack`. Explicit `esc` configuration is still opened. The ESC
  client is the hand-built one in `internal/esc` (see Plugin Host).

## Plugins

- **Recorder** (reference/contract plugin): `FINFOCUS_RECORDER_OUTPUT_DIR`
  (default `./recorded_data`), `FINFOCUS_RECORDER_MOCK_RESPONSE`.
- **Recommendation scorer** (`plugins/scorer/`): opt-in; `SCORER_BACKEND=jev|clef`
  defaults to Jev. `SCORER_API_KEY` is the selected backend token; it
  never appears in logs, errors, tests, or files. The recommendation id never
  leaves the plugin (answers map back by position). `priority` is always one
  record per request. Scores rank only; never
  gate an irreversible action on one.
  Hosted Jev sends data to TypeSafe AI; hosted Clef sends it to Cloudflare.
  Keep identifier handling and `field_allowlist` intact. Clef requires a
  32-hex-character `SCORER_CLEF_ACCOUNT_ID` when a token is provided and only
  accepts `SCORER_MODEL=clef`; its optional `SCORER_BASE_URL` is an origin.
  Source install is `make install-scorer`; the first public scorer registry
  release is pending. The legacy `jev` registry entry is deprecated
  (`deprecated` and `replacement: scorer` in `internal/registry/registry.json`).
  It stays installable, `plugin list --available` and the dashboard catalog
  say to use scorer, and install prints `DeprecationNotice` without migrating
  settings. `SCORER_BACKEND=jev` is the scorer backend, not that catalog entry. `TYPESAFE_API_KEY` and `JEV_*` are not read by
  scorer. Live tests require `SCORER_LIVE_TEST=1`; agents must not run them.
- **Agent skills**: all finfocus skills live in `agent-skills/` here,
  including `cost-check` and `cost-drift`. `rshade/agent-skills` keeps stubs
  that point to them. `internal/cli/cost_skills_test.go` pins both to the
  live flags and JSON fields. After editing a skill, rebuild its `.skill` zip
  (`SKILL.md` + `references/`) or the archive test fails.

## Package-Specific Gotchas

### Config singleton (`internal/config/integration.go`)

- `GetGlobalConfig()` initializes the singleton on its first call, without the
  project overlay, and `InitGlobalConfigWithProject` then does nothing. Never
  call it (or `GetDefaultOutputFormat`, `GetOutputFormat`) while building a
  command or its flags: the project tier would be lost. `--output` defaults to
  `""` and `resolveOutputFormatWithConfig` fills it in at run time, after
  `PersistentPreRunE`. `TestNewRootCmdDoesNotInitializeConfig` guards this.
- `config get` reads `cost.budgets.global.amount`, `currency`, and `period`.
  Flat `cost.budgets.amount` still addresses that same global scope.

### Plugin Host (`internal/pluginhost/`)

- Ports: allocate, hold, release, bind, retried by `StartWithRetry`. A port
  stays in `pendingPorts` until its plugin binds. A skipped pending port's
  listener is closed at once: the readiness probe is a bare TCP dial, so a
  held listener would answer for another plugin.
- Always `cmd.Wait()` after `cmd.Process.Kill()`.
- Core never sets `PORT`; plugins use `--port` or `FINFOCUS_PLUGIN_PORT`.
- `NewClient` injects delivered settings before `Launcher.Start`. A descriptor
  key uses `descriptor.env`. A key with no descriptor uses
  `pluginhost.GenericPluginEnv` (`FINFOCUS_PLUGIN_<NAME>_<KEY>`). A parent
  value already set, including empty, is left as it is. Skip
  `FINFOCUS_PLUGIN_PORT`, the log-file variable, and the trace id. Do not
  inject from redacted `pluginValue` or `config.effective()`: the config
  loader shallow-replaces the whole `plugins` object.
- ESC is the highest-precedence plugin environment source, and only for names
  the plugin advertises or that `esc.plugins.<name>.variables` lists.
  `appendDeliveredEnv` removes an inherited `KEY=` entry only for names in
  `Delivered.Overrides`. The projected cache key hashes `name`, the ESC
  reference, and the revision key, never the value. The `esc` config section
  is top-level because `plugins.<name>` is injected as environment. The
  selected stack comes from the Pulumi workspace file unless `--stack` is set.
  `PULUMI_CREDENTIALS_PATH` is a directory holding `credentials.json`, as in
  the Pulumi SDK. Workspace stack selection uses the same active backend as ESC
  auth (`PULUMI_BACKEND_URL`, `backend.url`, `credentials.json` `current`) and
  never falls back to another backend's stack. A value is read at a pinned
  revision (tag to number, `latest` ETag checked before and after the open); an
  unknown revision is an ESC failure. The ESC SDK client is hand-built (`Servers` = `<backend>/api/esc`, no
  redirects, token on the context). Open calls use `recover()` because a
  non-string trace panics the SDK. Never read `GenericOpenAPIError.Body()`.
- After `GetPluginInfo` (5s, request `*proto.Empty`), `NewClient` restarts
  the process once when advertised settings change the injected pairs. That
  restart happens before the version check, so stderr from the new process
  can appear even when the client is then closed as incompatible. A second
  change does not restart again. `Client.Close` is the func field from
  `started.idempotentClose()`.
- Secrets never appear in logs, errors, or another plugin's environment.
  Credential names stay on `config.IsCredentialName`.
- Dashboard settings apply on the next `NewClient`. The first reload must not
  close the overview pipeline's original clients. Later reloads close only
  the set the previous reload opened. `closeReloaded` runs before
  `pipe.Cleanup`. Install and remove fail the operation when reopen fails.
  A settings save still returns 200 and logs the reopen error.
- A dashboard HTTP request context has no plugin log writer. Plugin stderr is
  then discarded when the process stderr is not a terminal. Short probes
  (settings read, validate) use a child of the session context that is
  canceled when the request ends. A reload that keeps clients uses the session
  context itself. `exec.CommandContext` kills the plugin when its launch
  context is canceled, so a request-scoped context must not own those clients.

### CLI (`internal/cli/`)

- Overview TUI quit waits at most `overviewShutdownGrace` (2s) for the
  enrichment goroutine, then `overviewCleanupHandoff` (100ms) for the plugin
  cleanup callback. The budget fetch's done channel is joined before that
  cleanup runs (`Run` joins it itself). A plugin call that ignores cancel is
  abandoned after the grace period so quit cannot hang. The cleanup is
  published when the engine is built (`newCleanupPublisher`), so the abandon
  path still closes plugins before the process exits; a detached goroutine
  (`runLateOverviewCleanup`) covers a cleanup that appears later.
- The overview hands off to `runWebSession` only after plugin cleanup, through
  the `startWebSession` seam. The hint and `w` live inside the interactive
  model, so non-interactive runs never see them.
- `finfocus --web` shuts down on SIGINT and SIGTERM (`webui.Server.ServeUntilInterrupt`,
  the single owner; `Run` is Listen plus that). That is the path that stops
  plugin children.
  `server_sigterm_test.go` is `//go:build unix` because Windows has no
  `syscall.Kill` or SIGTERM. The interrupt test signals `os.Interrupt`.
  On SIGTERM, core SIGKILLs each plugin's whole process group
  (`killProcessGroup`), so a grandchild that stays in the plugin's group dies
  too; `test/integration/web_sigterm_orphan_test.go` pins that with a
  router-like fixture. SIGKILL of finfocus itself only delivers `Pdeathsig` to
  the direct child, and a grandchild that leaves the group (`Setsid` or its own
  `Setpgid`) survives either way. Those cases belong in the plugin.
  `analyzer serve` already handles both signals.
  Under WSL (`WSL_DISTRO_NAME` or `WSL_INTEROP`), `runWebSession` waits
  `wslBrowserOpenDelay` (1.5s) before opening the browser. The Windows-side
  localhost relay refuses a freshly bound port for about a second, so an
  immediate open fails until the user reloads. `--no-browser` skips the wait.
- The web session holds one `*stackload.Snapshot` (`webData.snapshot`).
  `setPipelineSource` publishes it under the query gate after the pipeline
  loads and again after an on-demand preview, which replaces the pipeline's
  snapshot through `Snapshot.WithPlan`. Estimate and Recommendations read
  `Descriptors()`; Cost reads `ActualDescriptors()`. Web Overview appends
  plan-only creates through `engine.AppendPlanOnlyRows` (the same step
  `MergeResourcesForOverview` uses) and prices only those new rows with
  `EnrichOverviewRows`; it does not only patch rows that already exist.
- `cost estimate --modify` splits on the last colon before `=`, so a Pulumi
  URN selector keeps its colons. A selector matches the resource ID, the
  logical name (the last `::` segment), or the type, and every matching
  selector applies; on a conflicting property the more specific wins (ID over
  name over type). `selectorKeys` is the one definition of a match. A
  selector that matches
  nothing is a validation error (exit 2) that lists the plan's resources.
  `--resource-type` must be a full token (`package:module:Type`) whose
  package matches `--provider`; a short name such as `ec2:Instance` is
  rejected.
- `plugin list` shows the highest semver of each plugin. `--all` lists every
  installed version, marks the one core opens (`Active` / JSON `active`), and
  starts older binaries only with `--verbose`. A non-semver directory name is
  a warning and is omitted from the default listing. Rows sort by name
  ascending, then installed version descending as semver. A version that is
  not semver uses ascending string order, and path breaks remaining ties.
  Table and JSON use that same order. Invalid-semver warnings go to stderr
  and do not change the exit code.
- `analyzer serve` prints only the port on stdout (Pulumi handshake); all logs
  go to stderr.
- **Tests leak into the real `~/.finfocus`** unless they set
  `t.Setenv("FINFOCUS_HOME", t.TempDir())`; this has produced tests that pass
  only after a sibling seeded the store. A test that fails alone
  (`HOME=$(mktemp -d) go test -run Name`) is order-dependent, not flaky. The
  same leak makes `TestPluginListCmd_TableOutputUnchanged` fail when several
  plugin versions are installed locally.
- Reject an unknown `--output` before loading state; early returns on empty
  results otherwise exit 0 silently (see `config_routes.go`, `plugin_list.go`).
- `config validate` and every `cost` pre-run share `ValidateConfigSource`.
  The on-disk budget field is `cost.budgets.global.amount`; flat
  `cost.budgets.amount` is warned and ignored.
- Do not put `--plain` (or other accessibility flags) on the `cost` parent:
  `cost history view` has its own `--plain`. Resolve modes with
  `tui.DetectResolvedOutputMode`, never `DetectOutputModeFor`, which re-reads
  `NO_COLOR` and undoes an explicit `--color`.
- Budget CLI flags (`--exit-on-threshold`, `--exit-code`) are
  `BudgetFlagOverrides` on the command context, never written onto the global
  config; only a copy of the global scope receives them.
- Budget notifications (`specs/625-budget-alert-notifications/`): stderr only,
  never change stdout or the exit code. Only `${FINFOCUS_NOTIFY_*}` expands,
  and a project-sourced destination with `${` is skipped without a lookup.
  Sends are HTTPS only, follow no redirects, and time out after 10s. Failure
  reasons redact every variable value, URL, and secret-looking header, and
  never include a response body.
  `ThresholdStatus.Notifications` is `json:"-"`; masking is display-only
  because `Config.Save` marshals the struct. Tests swap the client with
  `cli.SetNotificationClientForTest`.
- The overview docs page is `docs/src/content/docs/commands/overview.md`.
- Overview plugin startup overlaps stack loading (`OverviewPipeline.Run`, #692).
  The launch context is a child of the caller context. `errgroup.WithContext`
  cancels on `Wait` and would kill plugins (`exec.CommandContext`) before
  enrichment, so this overlap does not use it. A plugin error cancels the
  load; a load error cancels the launch. Cleanup runs before `Run` returns
  when either side fails, and stays on the pipeline only when both succeed.
  Phase events stay 1..6; phase 4 is emitted when the open is joined.
  `GetCurrentStack` wraps `ctx.Err()` when `pulumi stack ls` fails after that
  context is canceled. The process error is `signal: killed` and does not
  wrap `context.Canceled`, so overview would otherwise report the kill
  instead of the plugin failure that canceled the load. A load error whose
  context is still active stays a load error.
- `internal/webui` must not import `cli`, `registry`, or `pluginhost`. Plugin
  page callbacks live in `internal/cli/web_plugins.go` and
  `plugin_settings_web.go`. Install progress is a per-operation SSE stream
  (`snapshot`, `progress`, `done`). Do not publish it on the overview bus:
  that bus runs `RedactJSON` and drops slow subscribers. Settings GET uses
  `writePlainJSON`. Secret values are absent from the body. Bool and enum
  plugin settings skip `IsCredentialName` unless the descriptor sets `secret`.
  Plugins → Settings paints an ESC row in `plugins.js` only. The Status cell
  reads `from ESC (<reference>)` from `effectiveSource` and `escEnvironment`,
  and the help reference is a `code` element. Do not put Markdown backticks in
  that sentence, and do not add a display string to the settings JSON.
  Data-keyed maps keep their keys through `RedactJSON`: use `engine.DataKeyed`
  or the struct tag `datakey:"true"` (trends, breakdowns, and the cost-summary
  groupings). `RedactJSON` redacts in one pass (`redaction_walk.go`): it walks
  the decoded JSON alongside the original Go value, and that value is the only
  thing that says a map is data-keyed. A data-keyed map's keys are never
  matched against `IsHiddenPropertyKey`; its values still pass
  `history.IsPulumiSecret` and recurse, and one that is itself a secret
  envelope is dropped. Property maps do not get the mark, so
  `IsHiddenPropertyKey` still drops credential-like names. A Go value with no
  matching source (a type with its own `MarshalJSON`, an unknown key) is
  treated as a property map. There is no restore step, so do not add one.
  Do not add a second credential-name list or a payload-specific redactor case.
  `GET /api/recommendations/item` reads the last successful recommendations
  query cached on the session. It does not call the plugin again, because a
  plugin may mint a new `Recommendation.ID` on every call. That ID stays the
  dismissal id. A later successful query replaces the cache, and an id that
  was not in it is 404. The web UI has no dismiss action, so a dismissal
  made elsewhere shows only on the next query: dismissed items are absent from
  a default query and present, flagged `Dismissed`, with `includeDismissed`.
  With no successful query yet, the endpoint runs the query once and caches
  it. With several tabs the last query wins.
  The overview EventSource checks `GET /api/overview/budget` (same-origin
  cookie, no bearer token) after a connection failure. A 401 stops retries
  and tells the user to open the link printed in the terminal. `close`
  aborts that check. Other failures, and a check that takes over 5s, keep the
  capped backoff. `OverviewStream` moves through explicit states
  (`connecting`, `open`, `authenticating`, `reconnecting`, `backoff`, `failed`,
  `closed`) only via `dispatch`.
  `GET /api/estimate/baseline` returns a provider failure as HTTP 200 with
  `status: "unavailable"`, `error`, `code: "estimate_unavailable"`, redacted
  `display.properties` with no deltas, `pricingModes` from
  `DiscoverPricingSpec`, and `resource`. Never a 5xx that carries data. The
  editor must treat the marker as a failure, not a success. Request and
  server errors, `400 invalid_pricing_mode`, and recalculate failures stay a
  plain `{error, code}` 4xx/5xx.
  Web overview learns state-only from `pipe.OnStateOnly`
  (`Session.SetStateOnly`). The three flags are one `webui.RunStatus` on the
  session (browser: `mergeRunStatus` in `app.js`); the snapshot and the
  `run_status` event carry `stateOnly`, `previewLoaded`, and `detectFailed` (a
  boolean; the error text stays in the terminal log and never reaches the
  browser). `POST /api/overview/preview`
  is 409 unless the run is state-only and a preview has not been applied. A
  successful preview clears that status. A preview whose added rows fail to
  price (`appendPreviewRows` returns `errPreviewPricing`; a row has `Error` or
  `ProjectedPricingFailed`) is an error, not loaded, so Run preview can retry;
  an unpriced type that only reports `NO_COST_DATA` is not a failure. Do not keep a second store beside
  those session fields.
  `--pulumi-state` without `--pulumi-json` is state-only too, like the TUI.
  After a preview adds rows, `appendPreviewRows` applies the dismissal delta to
  them, and `OverviewPipeline.RefreshFallbackBudget` re-evaluates a config
  budget against the new total (plugin budgets are left alone).
  `pluginMetadata`
  does not copy `Settings`, so the settings handler calls `GetPluginInfo`
  again.
- The dashboard Config page and `config set` patch `config.hujson` through
  `config.PatchFile`. Do not call `Config.Save` for that write: it rewrites the
  struct from `New()`, folds environment variables into the file, and drops
  comments. `baseVersion` is the SHA-256 hex of the file bytes, or `absent`
  when the file is missing. A legacy project `config.yaml` is hashed as raw
  YAML and converted inside the patch, after that hash check, not before it.
  Only the global loader rewrites a legacy file to `config.hujson`. `config set`
  runs after that migration and passes an empty `baseVersion`. A request the
  caller got wrong is a `config.InvalidChangeError` (400); any other write
  error is logged and returned as a generic 500, because it can carry paths.
  Both accept only keys `config.LookupSetting` knows: a new settable key needs
  a `config.KnownSettings` entry (and a `defaultValue` case), which also puts
  it on the Config page. That makes it a web surface for Principle VII.

### Registry (`internal/registry/`)

- `Open` launches up to 8 plugins concurrently; clients keep
  `ListLatestPlugins` order, sorted by name (router priority ties rely on it).
- With `region` in `plugin.metadata.json`, discovery looks for
  `finfocus-plugin-<name>-<region>` first.
- Only a confirmed checksum mismatch is fatal; `--skip-checksum` exists.
  The dashboard catalog path does not offer that flag. A missing
  `checksums.txt` warns and continues on the CLI and returns
  `ErrChecksumUnavailable` on the web path unless the person accepts it.
  The web planner reports a zero-asset release as a fallback so confirmation
  can name both versions. The confirmed install is an explicit `name@version`
  of the fallback, so `AcceptFallbackVersion` stays unset. The installer
  still does not swap an explicit version on its own.
- `FINFOCUS_CATALOG_FILE` overlays the embedded catalog for `GetPlugin` and
  `GetAllPluginEntries` (read each call, not cached). `ListPluginsFromRegistry`
  stays on the embedded catalog. `FINFOCUS_GITHUB_API` replaces the GitHub API
  base. Both are test seams. A custom registry remains #1766.
- Monorepo plugins (`tag_prefix: "kubernetes-"`): "latest" is the highest
  prefixed stable semver, never `/releases/latest`; installs use the canonical
  version; the entry needs `asset_hints.asset_prefix`.
- A latest release with no assets yet (release-please publishes before the
  archives upload) is replaced by the newest earlier release with assets. An
  explicit version is never swapped; it returns `*NoAssetsError`
  (`ErrReleaseHasNoAssets`), which the CLI's version fallback still handles.
- `DefaultPluginDir()` is `config.New().PluginDir` (`FINFOCUS_PLUGIN_DIR`,
  then `plugin_dir`, then the plugins directory under the resolved home).
  Install and discovery share it.

### Router (`internal/router/`)

- A `$0.00` result is valid and does not fall back; only nil/empty does.
- Higher priority number wins. No routing config means no router (all
  plugins are queried).
- Provider means the billing cloud. `resourcetype.NormalizeProvider` holds the
  only alias table (`aws-native`→`aws`, `azure-native`/`azurerm`→`azure`,
  `google-native`/`google`→`gcp`); never split a type token for a provider
  anywhere else. `resource_type` keeps the package, so `aws-native:*` patterns
  still match.

### Engine (`internal/engine/`)

- `hoursPerMonth = 730`. Pricing-spec `per_day` uses 30 days (720 h). Do not
  "fix" one against the other: both are AWS's own conventions.
- JSON/NDJSON error codes are `PLUGIN_ERROR`, `VALIDATION_ERROR`,
  `TIMEOUT_ERROR`, and `NO_COST_DATA` (`types.go`); `NO_COST_DATA` is not an
  error.
- A failed plugin call keeps the gRPC status as `<Code>: <message>`
  (`pluginRPCError`). Only an empty projected result is `no cost data
  available`. Core does not read `aws:region` from provider or stack config.
  A fixed-cost type with no SKU property, including
  `aws:ec2/natGateway:NatGateway`, gets one from `skus.ResolveSKU`. An empty
  SKU is rejected before pricing (`resource.sku is required`) and the row
  then shows no price.
- Overview actual enrichment copies `ResourceID` into `pulumi:cloudId`, and
  fills `pulumi:urn` and `pulumi:created` when those properties are empty,
  before `GetActualCost`. The row's own properties are not rewritten. Cost
  actual already receives those keys from `MapStateResource`. Without the
  cloud id, a plugin can estimate a monthly figure instead of the measured
  window total the Cost page shows.
  The request `From` is clamped to the row's `CreatedAt` when that falls
  inside the window (`actualRequestStart`), because plugins trust an explicit
  Start over `pulumi:created`; without it a resource made yesterday was
  charged for the whole month. `ActualCost.Period` keeps the display window.
  An imported row (`External`) is not clamped: its `CreatedAt` is the import
  time. A clamped row records `ActualCost.BilledDays`, and the delta
  extrapolation divides by that instead of the day of month.
- Projected diff (`cost projected`): replacement steps sharing a URN are
  collapsed into one `update` (`collapseReplacements`), or the old resource
  counts twice. Both sides use the same basis and the same plan-wide `ref.*`
  resolution. Unpriced `pulumi:` types are left out.
- Usage profile (`specs/630-usage-profile/`): `--profile`, then
  `FINFOCUS_PROFILE`, then `cost.profile` on the merged config
  (`InitializedGlobalConfig`). `config.New` has no project overlay, so
  estimate, overview, and the dashboard must not read the profile from it.
  An omitted `cost.profile` is no preference. A present empty value fails
  validation. The interactive recommendations view names the same profile.
  The environment variable is not
  copied into the config file. Opt-in engines are projected, recommendations,
  forecast, estimate, cluster run-rate, overview, the dashboard engine, and
  `analyzer serve`. `cost actual` and `cost history collect` stay unspecified.
  The hourly-rate lookup inside actual cost passes UNSPECIFIED even when the
  engine has a profile. A set profile skips projected `BatchCost` because
  `BatchCostRequest` has no field, and appends `/profile-dev|prod|burst`
  after the other cache suffixes. Unspecified adds nothing. Do not set the
  profile inside `newEngineWithCache`.
- `ResourceExhausted` is a rate limit, not a reason to switch plugins.
  Projected and actual cost retry that same plugin (`retryRateLimited`,
  three attempts) before the fallback log. A wait uses the caller context.
  `ErrorDetail.retry_after_seconds`, when set, is the wait; otherwise the
  wait is bounded jitter. Each attempt still gets its own
  `perResourceTimeout` where that timeout already applied. A batch that
  returns `ResourceExhausted`, or per-resource code 8, is retried as a
  batch. Those resources are not sent back through `GetProjectedCost` or
  `GetActualCost` on the rate-limited plugin. A deadline that fires during
  the wait stops the chain. `priceChunk` gives RPC-level and per-item rate
  limits one shared three-attempt budget, resends only the still-limited
  resources, and splits each retry by the smallest `MaxBatchSize` seen. A
  later chunk's ordinary failure keeps earlier rate-limit markers: only the
  unfinished rows get an error, because callers ignore results when
  `executeBatchForPlugin` returns one.
- Cost history (`internal/history`, `cost history *`): files are
  `<project>@<stack>.history.db`, separate from the resource-observation
  `history.db`. Both use one directory: `FINFOCUS_HISTORY_DIR`, then
  `cost.history.directory`, then `<ResolveConfigDir()>/history`.
  A project-tier `cost.history.directory` is ignored with a warning when it
  is absolute, has `..`, or resolves (symlinks followed on the deepest existing
  ancestor, against the working directory) outside the project root, or when
  an entry named `history.db*` or containing `.history.db` (including the
  `.compact-*` compaction temp) is a symlink or not a regular file
  (rechecked after `MkdirAll`; a swap after the check is not covered). The gate
  takes the project dir that built the config (`initHistoryFromConfig`'s
  `projectDir`; the analyzer passes its own, never the process-global one) and
  judges the overlay value from `config.ProjectHistoryDirectory`; global tier and the
  env var are trusted. The working-dir fallback is made absolute.
  `collect` fails a checkpoint rather than store an inaccurate
  total (#549). `internal/history` must not import `engine` or `ingest`.
  The overview and `cost actual` interactive detail views load one resource's
  series from that database only after Enter (`internal/tui/detail`). An empty
  stack or a missing database returns `detail.ErrHistorySkipped` and the
  section is omitted. Any other error stays
  inline; `r` retries that load and leaves the detail view open. Replies carry
  the model pointer and a generation so a late fetch cannot fill a newer view.
  History subcommands reuse the parent `--stack`. `cost history export` takes
  `csv`/`ndjson` through the root `--format` by rewriting it in `Args` before
  `ax.Execute` resolves the mode. `pulumi stack history` is read in pages of
  100 until a page adds no new version; a short page is not the end, and the
  CLI's default page is only 10 updates.
- `--explain`, `--pricing-spec-fallback`, and estimate-TUI discovery call
  `GetPricingSpec`; disabled runs never call it, and each call has a 5s
  deadline. A `$0` rate is a price; a plugin must answer `zero_cost` or an
  error for "no price". Tiers are graduated, and tiers that do not cover the
  whole quantity fall through to YAML.
- `cost forecast` (`specs/626-cost-forecast/`) does not change `cost projected`
  totals. A blank currency is USD.
- `checkPluginSupports` caches per client, provider, type, region, SKU,
  feature, and attributes digest. SKU must stay in the key, or one SKU-less
  answer poisons every SKU in that region. The SDK's "not implemented" reason
  is fail-open, like `Unimplemented`. `InvalidArgument` from `Supports` is a
  decline: it is cached as unsupported, the reason is shown, and no pricing
  RPC follows. `Unavailable`, `DeadlineExceeded`, and `Canceled` stay
  fail-open and are not cached. Do not add a provider-name table here.
- Plugin `expires_at` sets a per-entry TTL (capped at 7 days; a past time skips
  caching). A plugin whose price depends on its own environment must send
  `expires_at` = now, because the cache key cannot see it.
- Cache hits append "(cached)", with a leading space, to the adapter. A
  corrupt cache is deleted and recreated. `cost.cache.lru_enabled` wraps
  Bolt in `cache.TieredStore`.
- Budget health: OK < 80%, WARNING 80-89%, CRITICAL 90-100%, EXCEEDED > 100%;
  aggregation takes the worst.
- Recommendations: `Recommendation.ID` is the dismissal id. The cache key hashes
  inputs plus dismissed ids; `--include-dismissed` still sends excluded ids and
  adds `/include-dismissed`.
- `cost recommendations history` (`specs/1729-recommendation-history/`) merges
  plugin `GetRecommendationHistory` events with the local dismissal record.
  It queries every connected plugin, including when that id has no local
  record, and does not need a Pulumi plan. `Unimplemented`, an empty result,
  and any other plugin failure are logged and local history stays. Each plugin
  call uses `perResourceTimeout`. A plugin that hits that deadline, or that
  returns `Canceled` or `DeadlineExceeded` on its own, is skipped. Only the
  caller's context being canceled or past its deadline aborts the query. The
  read copies events before sorting and does
  not write `dismissed.json`. `actor` on `config.LifecycleEvent` is omitted
  when empty, so older files load with no migration. A missing timestamp sorts
  after dated events. A timestamp or expiry that is present but invalid drops
  that event only. Events match on action, reason, note, time, and expiry;
  a named actor replaces a blank one, and different actors are both kept.
- Scoring (`internal/scoring`, off by default) pseudonymizes identifiers by
  default, only warns on failure, and never dismisses or hides anything.
  Scorer-only plugins are dropped from cost features before `Supports`.

### Overview Field Semantics (`internal/engine/overview_*.go`)

| Field | Basis | Source |
| --- | --- | --- |
| `MTDCost` | Day 1 to today (spent so far) | Actual plugin |
| `MonthlyCost` | Full 730 h month | Projected plugin |
| `ExtrapolatedMonthly` | Calendar month from the MTD trend | `CalculateCostDrift` |
| `Delta` (drift) | `ExtrapolatedMonthly - MonthlyCost` | `CalculateCostDrift` |

- `MTDCost` and `MonthlyCost` are different units; never subtract them without
  extrapolating first.
- Population by status:

  | Status | Actual | Projected | Drift | PropertyDiffs |
  | --- | --- | --- | --- | --- |
  | Active | yes | yes | maybe | no |
  | Updating / Replacing | yes | yes | maybe | yes |
  | Creating | no | yes | no | no |
  | Deleting | yes | no | no | no |

- The Delta column always comes from `CalculateRowDelta()`, which is
  status-aware; never compute it inline. Update/replace is projected(after)
  minus the baseline projected(current) (`GetBaselineProjectedMonthlyCost`),
  the same basis. It falls back to extrapolated actual only without a
  baseline, and shows no delta without property diffs. Creating is
  `+projected`, deleting is `-extrapolatedActual`, active is the drift delta.
- `GetExtrapolatedActual()` and `ForceExtrapolateActual()` use 30 days
  (stable deltas); `CalculateCostDrift()` uses calendar days. Do not unify
  them.
- `CostDrift == nil` is normal: day 1-2, drift under 10%, new or deleted, or
  created under 3 days ago.
- TUI diff values are truncated to 40 characters (`truncateDiffValue`). State-only
  mode marks projected costs with `*`; `p` runs a preview and updates rows in
  place.

### Cluster Allocation (`internal/engine/cluster*.go`, `plugins/`)

Specs 613-623 hold the details. Invariants:

- Core never interprets Kubernetes. Nodes arrive from `GetStats` as ordinary
  descriptors and are priced by the engine; grouping is string-map aggregation.
- Run-rate prices with `GetProjectedCostWithErrors` and `Monthly`. A window
  (`--from`/`--to`) prices with `GetWindowCost` and `TotalCost` only, labels the
  period with `FormatWindow` (not `FormatPeriod`, which `cost actual` keeps),
  and requires a source in historical mode (`ErrStatsModeMismatch`).
- A `$0` or missing price is unpriced (aws-public returns `$0` for unknown
  types). All nodes unpriced is fatal. A stats warning starting `incomplete:`
  sets `Incomplete`.
- Conservation is enforced in core (`VerifyConservation`, shared SDK rule)
  even though the conformance suite checks it too.
- Nodes are keyed by cluster and node name; the priceable `id` stays the
  Kubernetes node name.
- The idle row stays at `$0`: `ValidateAllocateResponse` needs one per priced
  node. Fargate pods are their own priceables and never fold into idle.
- `plugins/kubernetes` and `plugins/prometheus` must not import core or each
  other. Prometheus node identity copies the descriptor rules from
  `plugins/kubernetes/usage/nodes.go`.
- Prometheus collection (`plugins/prometheus/`):
  - Take `max by (node)` of kube-state-metrics capacity before integrating.
    Replicas export the same node twice, and a sum doubles capacity.
  - cAdvisor scraped from the kubelet often has no `node` label. The node
    then comes from `kube_pod_info`, and matching must accept the empty label.
  - Clusters are discovered from `kube_node_labels` or
    `kube_node_status_allocatable` before any sum drops the label.
  - Retention shorter than the window is detected store-wide
    (`promql.StoreStart`), not per pod. A hole shorter than the 5m lookback is
    not detected.
  - The bearer token goes only to the configured scheme and host (a redirect
    elsewhere would receive it otherwise), and URLs in errors and logs drop
    userinfo and query.
- Pod label keys from kube-state-metrics are sanitized
  (`app.kubernetes.io/name` becomes `app_kubernetes_io_name`).

### MCP Server (`internal/cli/mcp.go`, `output_mode.go`)

- `mcp-server` and `--mcp` cancel their context on SIGINT and SIGTERM
  (`cancelOnShutdownSignal`, installed only on those two entry points; `main`
  still passes `context.Background()`). ax-go `mcp.Serve` then drains and
  returns nil, so the process exits 0 with deferred cleanup run. Do not move the
  handler into `main`: `analyzer serve`, `--web`, and the TUI commands own their
  signals. Do not patch ax-go from this repo. `mcp_server_signal_test.go`
  (`//go:build unix`) pins this.
- ax-go re-executes the shared root once per `tools/call`, so persistent
  pre/post run per call. Dispatched calls reuse the server's
  `loggingSession` and never close it.
- One mutex serializes every call; a blocking command stalls the server. That
  is why `analyzer serve` and `cost history collect` are excluded
  (`mcpExcludedCommands`, `mcp.Exclude`). Never use `Hidden` to exclude a
  group: ax-go prunes the whole subtree.
- `resolveOutputFormat`: explicit `--output`, then `--format`, then
  `AGENT_MODE`. It ignores TTY detection on purpose. New commands with an
  output flag must use it.
- Flag values reset per call only on a real server; assert cross-call
  behavior in the integration test.
- `internal/cli/testdata/mcp/tools.golden` pins the tool list. A new command
  must be exposed or excluded on purpose (`UPDATE_GOLDEN=1`).
- The MCP handshake rejects `dev`/`unknown`; build test binaries with
  `-ldflags -X .../pkg/version.version=...`.

### Plugin Upgrade (`internal/pluginupgrade/`, `agent-skills/finfocus-plugin-upgrade/`)

- Bumping finfocus-spec needs a hop: `hops.go` must reach
  `pluginsdk.SpecVersion` and have a `references/to-vX.Y.Z.md` guide (tests
  enforce both), even for an additive release. Add the guide's row to
  `SKILL.md` and rebuild the `.skill` zip (`SKILL.md` + `references/`).
- `SpecVersion` must be `v`-prefixed, or `GetPluginInfo` returns `Internal`.
- `plugin.manifest.json` is the flat `registry.Manifest`, not the pluginsdk
  manifest; `plugin validate` needs name and version to match the install dir.
- Skill install runs `npx skills@1.7.0 add` in an empty temp dir, never the
  plugin dir (its `.npmrc` and `node_modules` could run code), and writes
  through an `os.Root`. Every failure is a warning. Tests never run npx.

## Recent Changes

- 1735-pulumi-cloud-history: `cost.history.pulumi_cloud.enabled` defaults
  off (`false`); `auto` falls back and `true` requires a complete import.
  Read historical snapshots from `/export/{version}`, not `/updates/{version}`.
  The update list ends with `{"updates":null}`; a deployment with no `resources`
  list is an empty snapshot, not a failed read.
  Cache update metadata and extracted URN/cloud ID/type/provider only in
  `history.db`; never retain raw exports, properties, tags, or ciphertext.
  History merge takes minimum `FirstSeen` and maximum `LastSeen`. Clamp imports
  using the resolved retention value (environment, config, default), never a
  second config read. Overview and web never fetch Cloud history; the web Cost
  page reads stored identities and overview keeps its state/plan inventory.
  `KnownSettings` gates both `config set` and the Config page; the mode accepts
  booleans or strings but writes strings. Preserve ESC's credential rule:
  project `backend.url` gets only a saved token for that URL; an environment
  token override requires explicit `PULUMI_BACKEND_URL` or the current account.
  `internal/pulumicloud` wraps the official `pulumi-cloud-sdk` `apiclient`; its
  `Executor` owns the token, no-redirect, retry, ETag, and size-bound rules.
  Do not run `update-agent-context.sh`: it damages this file's curated content.

- 1733-shared-stack-loader: Shared Pulumi state and preview snapshots align CLI and web live inventory. Projected output adds a separate stack total and safe state-availability categories while preserving plan totals, budgets, exit codes, and forecast totals (`specs/1733-shared-stack-loader/`).

- 1732-terraform-plan-ingestion: Terraform plan mode parses `terraform show -json` through in-process `pluginsdk.CostParseProvider` (`ParseRequest` / `ParseResponse` plus `ValidateParseRequest` and `ValidateParseResponse`). Not served as `CostParseService`. `ParseResponse` has no operation; the host keeps `map[source_id]operation` beside it. `resource_type` stays the Terraform type and `ResolveResourceTypes` still runs after validation (`specs/1732-terraform-plan-ingestion/`). Terraform 1.16 records data sources on `prior_state`, not `resource_changes`; the committed plan fixture copies those emitted data resources into `resource_changes` with action `read`. `cost projected` plans a Terraform directory or reads `--terraform-plan`. Overview, estimate, recommendations, forecast, cost history, and `--web` also plan a detected Terraform directory. `cost actual` and `cost history collect` do not plan.
- 1730-pulumi-esc-plugin-credentials: Pulumi ESC `environmentVariables` as the highest-precedence plugin env source, per-plugin allowlist, fallback with categorized warnings (`specs/1730-pulumi-esc-plugin-credentials/`)
- 1729-recommendation-history: `cost recommendations history` merges plugin lifecycle events, including actor (`specs/1729-recommendation-history/`)
- 630-usage-profile: `--profile` / `FINFOCUS_PROFILE` / `cost.profile` sent on projected cost and recommendations (`specs/630-usage-profile/`)
- 629-web-plugin-page: Plugins page in `finfocus --web` (catalog, install, settings)
- 628-web-config-page: dashboard Config page and `config set` patch `config.hujson` in place
- 626-cost-forecast: `cost forecast` projects plugin GrowthType with finfocus-spec `pricing.ApplyGrowth`
- 625-budget-alert-notifications: Slack and generic HTTPS webhook budget alert destinations (stdlib `net/http`)
- 623-prometheus-usage-source: Prometheus usage source plugin and historical `cost cluster` windows

## Active Technologies

- Go 1.27.3 (`go.mod`), finfocus-spec v0.7.9 (`pluginsdk`, `WithSettings`), Cobra, ax-go, gRPC, zerolog, BoltDB (`go.etcd.io/bbolt`), Bubble Tea v2, testify; `prometheus/client_golang` and client-go in plugin modules only; `github.com/pulumi/esc-sdk/sdk` v0.14.0
