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
make test-kubernetes   # Also test-jev, test-prometheus (nested plugin modules)
make install-kubernetes  # Also install-jev, install-prometheus, install-recorder
```

```bash
go test -run TestName ./internal/cli/
go test -race -shuffle=on -count=3 ./internal/<pkg>/...   # Parallel-safety check
./bin/finfocus plugin list --output json
```

- `make lint` checks golangci-lint's version against `mise.toml` and runs
  `$(HOME)/go/bin/golangci-lint`. When that binary lags the pin, run
  `make lint GOLANGCI_LINT="$(mise which golangci-lint)"`.
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
| `internal/proto/` | Adapter: descriptors, pre-flight validation |
| `internal/router/` | Plugin routing with priority and fallback |
| `internal/analyzer/` | Pulumi Analyzer gRPC server |
| `internal/tui/` | Bubble Tea v2 / Lipgloss v2 (`charm.land` imports) |
| `internal/config/` | Two-tier config |
| `plugins/{kubernetes,jev,prometheus}/` | Nested modules; never import core |

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
  merge). Legacy `config.yaml` is migrated to Hujson on first read.

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
  `t.Run`/`for` line for some subtests).
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
- Remote-write fixtures need `storage.tsdb.out_of_order_time_window` in the
  Prometheus config and a window relative to now. A fixed date is rejected as
  out of bounds once a newer sample exists, and later falls out of the 15-day
  retention.
- `make test-e2e-kind` pins `E2E_AWS_PUBLIC_VERSION` (Renovate bumps it).

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
  `inputs` and `type` there or plugins get `InvalidArgument`.
- `parent` is the stack, not a pricing reference. Cross-resource refs follow
  `propertyDependencies` only and become `ref.<property>.{urn,type,region,sku}`
  tags (`specs/618-cross-resource-refs/`). The unknown sentinel
  `04da6b54-80e4-46f7-96ec-b56ff0331ba9` is never sent as a tag, SKU, or
  region. A referenced SKU is never copied into the child's `Sku`.
- **No credential or Pulumi secret reaches a plugin.** One rule each:
  `isCredentialKey`/`skipDottedSegment` for names and `history.IsPulumiSecret`
  for values. They apply to collapsed tags, dotted keys
  (`specs/619-dotted-tag-keys/`), `ResourceDescriptor.attributes`
  (`specs/621-k8s-workload-projected-cost/`), actual-cost tags, and
  `EstimateCost` attributes. Do not add a second rule.
- Anything new sent to a plugin must be in the projected cache key; that is
  why keys end in `/tags-`, `/refs-`, `/attrs-` digests and feature suffixes
  (`/pricing-spec`, `/growth-v1`).
- `__`-prefixed inputs (`__defaults`, `__provider`) are Pulumi internals; filter
  them from anything shown to users.
- Analyzer import: `pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"`
  (not `.../proto/go/pulumirpc`).

## Plugins

- **Recorder** (reference/contract plugin): `FINFOCUS_RECORDER_OUTPUT_DIR`
  (default `./recorded_data`), `FINFOCUS_RECORDER_MOCK_RESPONSE`.
- **Jev scorer** (`plugins/jev/`): off unless `TYPESAFE_API_KEY` is set; the key
  never appears in logs, errors, tests, or files. The recommendation id never
  leaves the plugin (answers map back by position). `priority` is always one
  record per request (batching drops rank quality). Scores rank only; never
  gate an irreversible action on one.
- **Agent skills**: finfocus-specific skills live in `agent-skills/` here;
  generic cost-workflow skills live in `rshade/agent-skills`.

## Package-Specific Gotchas

### Plugin Host (`internal/pluginhost/`)

- Ports: allocate, hold, release, bind, retried by `StartWithRetry`. A port
  stays in `pendingPorts` until its plugin binds. A skipped pending port's
  listener is closed at once: the readiness probe is a bare TCP dial, so a
  held listener would answer for another plugin.
- Always `cmd.Wait()` after `cmd.Process.Kill()`.
- Core never sets `PORT`; plugins use `--port` or `FINFOCUS_PLUGIN_PORT`.

### CLI (`internal/cli/`)

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

### Registry (`internal/registry/`)

- `Open` launches up to 8 plugins concurrently; clients keep
  `ListLatestPlugins` order, sorted by name (router priority ties rely on it).
- With `region` in `plugin.metadata.json`, discovery looks for
  `finfocus-plugin-<name>-<region>` first.
- Only a confirmed checksum mismatch is fatal; `--skip-checksum` exists.
- Monorepo plugins (`tag_prefix: "kubernetes-"`): "latest" is the highest
  prefixed stable semver, never `/releases/latest`; installs use the canonical
  version; the entry needs `asset_hints.asset_prefix`.
- A latest release with no assets yet (release-please publishes before the
  archives upload) is replaced by the newest earlier release with assets. An
  explicit version is never swapped; it returns `*NoAssetsError`
  (`ErrReleaseHasNoAssets`), which the CLI's version fallback still handles.
- `DefaultPluginDir()` is `config.ResolveConfigDir()/plugins`.

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
- Projected diff (`cost projected`): replacement steps sharing a URN are
  collapsed into one `update` (`collapseReplacements`), or the old resource
  counts twice. Both sides use the same basis and the same plan-wide `ref.*`
  resolution. Unpriced `pulumi:` types are left out.
- Cost history (`internal/history`, `cost history *`): files are
  `<project>@<stack>.history.db`, separate from the resource-observation
  `history.db`. `collect` fails a checkpoint rather than store an inaccurate
  total (#549). `internal/history` must not import `engine` or `ingest`.
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
  is fail-open, like an RPC error.
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

- 626-cost-forecast: `cost forecast` projects plugin GrowthType with finfocus-spec `pricing.ApplyGrowth`
- 625-budget-alert-notifications: Slack and generic HTTPS webhook budget alert destinations (stdlib `net/http`)
- 623-prometheus-usage-source: Prometheus usage source plugin and historical `cost cluster` windows

## Active Technologies

- Go 1.27.1 (`go.mod`), finfocus-spec v0.7.5 (`pluginsdk`), Cobra, ax-go, gRPC, zerolog, BoltDB (`go.etcd.io/bbolt`), Bubble Tea v2, testify; `prometheus/client_golang` and client-go in plugin modules only
