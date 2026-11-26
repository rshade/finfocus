# Research: Plugin Name Conformance

All paths are relative to the repository root, on `main` at `7935bd35`.

## R1. Where the install name lives

**Decision**: Add `InstallName string` to `pluginhost.Client`. `launchClient`
sets it from `launchInputs.installName` when the context carries one
(`WithInstallName`, already set by `registry.openPlugin`). The pool's
`holderClient` copies the struct, so held clients carry it with no extra code.

**Rationale**: The install name already reaches `NewClient` through the context
(`internal/pluginhost/settings_env.go`), where it is used only to deliver
settings. Storing it on the client is the change #2278 and #2206 both propose.
A pooled process is keyed by binary path and environment, and a binary path
belongs to exactly one install directory, so every holder of a pooled process
shares the same install name.

**Alternatives considered**: Rewriting `Client.Name` to the install name
(changes the Adapter column and breakdown keys, which FR-008 keeps); a side map
in the registry from client to install name (every consumer would need the
registry).

## R2. One matching rule

**Decision**: Two functions in `internal/pluginhost/identity.go`:

- `(*Client).MatchesName(name string) bool`: true when `name` is non-empty and
  equals `InstallName`, one of `Aliases` (R14), or `Name`.
- `ResolveName(clients []*Client, name string) (client *Client, via MatchKind, ambiguous bool)`:
  the first client whose `InstallName` equals `name`; otherwise the first with
  `name` in `Aliases`; otherwise the first whose `Name` equals it. `via` says
  which (`install`, `alias`, `reported`) so validation can warn on aliases.
  `ambiguous` is true when a different client also matched at a lower
  precedence.

Every comparison of a user-supplied plugin name with a client goes through
these: the router, `ValidateRoutingConfig`, the engine's `--adapter` filters
(`engine.go:1374`, `:1739`, `actual_batch.go:90`), and
`estimate_selection.go:51`.

**Rationale**: CLAUDE.md asks for one rule per concern ("Do not add a second
rule"). `pluginhost` is already imported by every consumer, and `Client` is the
type both names live on.

**Alternatives considered**: A helper in `internal/router` (the engine and
registry would import the router for a string comparison); stripping a
`finfocus-plugin-` prefix (a name table; fails for any other spelling).

## R3. Router keyed by client, not by name

**Decision**: `NewRouter` resolves each routing entry to a client once with
`ResolveName` and keeps `configFor map[*pluginhost.Client]*config.PluginRouting`
plus `unmatched []string` (entries that resolved to no client).

- The automatic pass (`router.go:328`) reads `configFor[client]`.
- `findClient(name)` becomes `ResolveName(r.clients, name)`.
- `hasMatch` compares client pointers, not names.
- `ShouldFallback(pluginName)` keeps its signature (engine interface
  `engine.go:128`); it resolves the name to a client, then reads
  `configFor`. The engine passes the reported name, which resolves to the same
  client as an entry written with the install name.
- New `(*DefaultRouter).UnmatchedEntries() []string`.
  `createRouterForEngine` (`internal/cli/common_execution.go`) logs one warning
  per entry when real clients were supplied: the entry name and that its
  feature and pattern restrictions are not applied (FR-005).

**Rationale**: Resolving once makes every later lookup agree and removes the
name comparison from the hot path. Logging in the CLI builder, not in
`NewRouter`, keeps `config routes test` (synthetic clients) quiet.

**Alternatives considered**: Indexing `pluginConfig` under both names (two
entries can then claim one client with no tie-break).

## R4. `config validate`

**Decision**: `ValidateRoutingConfig` (`internal/router/validation.go`)
replaces the `pluginNames` set with `ResolveName`:

- not found: error as today. When the name is `finfocus-plugin-<x>` and `<x>`
  resolves, the message adds `did you mean "<x>"?` (FR-006);
- ambiguous: a warning naming both plugins (FR-003).

The `finfocus-plugin-` prefix is the binary naming convention core already uses
to discover plugins (`internal/cli/plugin_inspect.go:131`,
`internal/registry`), so it is referenced from one exported constant
(R8), not repeated.

## R5. `--adapter`

**Decision**:

- `registry.Open(ctx, onlyName)`: when `onlyName` equals an install name or a
  catalog alias of one (R14), open only that plugin. When it equals neither,
  open all plugins, log at debug that the adapter will be matched against
  reported names, keep only the clients that report it, and close the rest.
  Open therefore still returns only the named plugin: commands such as
  `cost projected` have no engine-side adapter filter and rely on it
  (found by T012; an unknown `--adapter` would otherwise price with every
  plugin).
- Engine filters use `MatchesName`.
- The actual-cost cache filter (`engine.go:4531`) keeps the raw `--adapter`
  string. The two spellings produce separate cache entries with the same
  content; this is harmless and avoids a lookup in the key builder.

**Rationale**: The registry cannot know reported names without launching.
Opening every plugin only for a non-install spelling preserves today's cost
for the documented spelling. When the engine filter then keeps no client,
`openPlugins` (internal/cli/common_execution.go) logs a warning naming the
`--adapter` value (FR-024), so a typo no longer returns an empty result
silently. #2206 measured that the reported spelling opens
zero plugins today, so this fixes it.

**Alternatives considered**: Launching every plugin to read names before
filtering (slower for the common case).

The web dashboard reaches plugins through the same `openPlugins`
(`internal/cli/web_data.go:320`), so `--web --adapter` is covered.

## R6. `config routes test` without launching plugins

**Decision**: Read installed plugins with `registry.ListLatestPlugins()` (no
launch) and their catalog aliases (R14). An entry is installed when it equals
an install name or one of its aliases. Synthetic clients are built only for
installed entries, with `InstallName` and `Aliases` set. An alias match is
simulated and produces the deprecation warning from R14. Any other entry
produces a warning (table and JSON `warnings` array) and is left out of the
simulation, with the `did you mean` hint from R4 when it applies. The command
help states that reported names are confirmed by `config validate`, which
launches plugins.

**Rationale**: The catalog alias is the declared, launch-free answer for the
known mismatches (aws-public today, cloudzero before v0.1.1), so SC-001 holds
for routes test. A plugin whose reported name is neither its install name nor
an alias gets a warning in routes test but works at run time; the warning
points to `config validate`.

**Alternatives considered**: Launching plugins in routes test (contradicts its
documented "does not contact plugins"); treating the binary file name as a
reported-name stand-in (a guess, superseded by declared aliases).

## R7. Mismatch warning once per process

**Decision**: `registry.openPlugin`, after a successful open, compares
`client.InstallName` with `client.Name`. On a difference it logs one warning
(fields `install_name`, `reported_name`, message telling the user both names
are accepted and the plugin should report its install name). A package-level
`sync.Map` keyed by `install|reported` suppresses repeats for the process, so
multi-stack web sessions and dashboard reloads warn once.
`ResetNameMismatchWarningsForTest` clears it; tests that use it do not run in
parallel (CLAUDE.md, Parallel Tests).

**Alternatives considered**: Warning in `pluginhost` (also fires for
conformance runs and other non-registry opens, which have no install name);
warning every open (repeats per stack and reload).

## R8. Canonical name rule and binary prefix in a leaf package

**Decision**: New leaf package `internal/pluginname` (standard library only):

- `BinaryPrefix = "finfocus-plugin-"`
- `Valid(name string) bool`: moved from `cli.IsValidPluginName` (2-50
  characters, lowercase letters, digits, hyphens, no leading or trailing
  hyphen). `cli.IsValidPluginName` and `plugin init` call it.
- `Canonical(name string) error`: `Valid`, plus no `BinaryPrefix`, plus no
  doubled hyphen. Used by the conformance check.
- `FromBinary(path, region string) (string, bool)`: strips `.exe`,
  `BinaryPrefix`, and `-<region>`.

**Rationale**: `internal/conformance` cannot import `internal/cli` (the CLI
imports it). The rule already exists for `plugin init`; moving it keeps one
definition. The prefix literal currently appears in several places
(`plugin_inspect.go`, the registry, templates); new code uses the constant.

## R9. A warning status in the conformance suite

**Decision**: Add `StatusWarn Status = "warn"` and `Summary.Warnings`.

- `calculateSummary` counts it (today unknown statuses count as errors).
- Table: icon `⚠`, the message printed under the test like a failure, and a
  `Warnings: N` line only when N > 0, so existing output is unchanged for
  plugins with no warnings.
- JSON: `status: "warn"`, `summary.warnings`.
- JUnit: a passing `<testcase>` with `<system-out>WARNING: ...</system-out>`;
  JUnit has no warning state, and a failure would break CI that consumes it.
- `checkResults` (exit code) ignores warnings (FR-013).
- Certification (`certification.go:36`): warnings do not block `Certified`;
  the markdown report adds a `Warnings` line.
- The runner's per-status log switch (`runner.go:140`) logs warn at warn level.

**Alternatives considered**: A `Warning` string on a passing result (the
summary cannot count it, and readers of `status` miss it); reusing `skip`
(certification treats skips as not certified).

## R10. Expected name and the three checks

**Decision**: `internal/conformance/identity.go`:

- `ResolveExpectedName(binPath, explicit string) PluginIdentity`, in order:
  explicit `--expected-name`; install layout (the binary's parent directory
  parses as a semver version with an optional `v`, the same parser the
  registry uses, and the grandparent directory is the name); `pluginname.FromBinary`
  with the region from `plugin.metadata.json` next to the binary
  (`registry.ReadPluginMetadata`). Otherwise `Source` is empty and `Reason`
  says why.
- The suite resolves it once in `Run` and puts it on `TestContext.Identity`.
- Test IDs, all `CategoryProtocol`, `RequiredMethods: Name`:
  - `Name_MatchesInstallName`: warn `install name not checked: <Reason>` when
    unresolved; warn naming expected, actual, and where the expectation came
    from.
  - `Name_MatchesPluginInfo`: calls `GetPluginInfo`; warn on `Unimplemented`
    or another failure; warn when its name differs from `Name`.
  - None of the three skips (implementation finding): `Certify` counts every
    skip as unverified, so a skip would let an advisory check block
    `plugin certify`.
  - `Name_IsCanonicalForm`: warn with `pluginname.Canonical`'s reason.
- `plugin conformance` gains `--expected-name`; `plugin certify` resolves the
  name from the path only (it has no probe or filter flags either).

The suite already launches the plugin without an install name, so settings
delivery is unchanged.

**Out of scope, noted**: The report's plugin version is always `unknown`
(`suite.go`); a separate issue.

## R11. Plugin-dev guidance

**Decision**:

- New `## Naming conformance` section in `agent-skills/finfocus-plugin-dev/SKILL.md`
  (after "Testing loop"): the rule; one exported `PluginName` constant used by
  `NewBasePlugin`, `PluginInfo`/`NewPluginInfo`, the logger, every result
  `Source`, and cache paths; the three check IDs and `--expected-name`; grep
  recipes for stray literals. Detail in a new `references/naming.md`.
- `plugin init`'s Go template (`internal/cli/plugin_init.go`) generates
  `const PluginName = "{{NAME}}"` and uses it in `NewBasePlugin` and
  `PluginInfo`, so new plugins start conforming.
- New `internal/cli/finfocus_plugin_dev_skill_test.go`: every check ID the
  section cites exists in `conformance.NewSuite(...).GetTestCases()`, the flag
  exists on `NewPluginConformanceCmd`, and `finfocus-plugin-dev.skill` matches
  `SKILL.md` plus `references/`, following `plugin_manage_skill_test.go`.

**Alternatives considered**: A separate skill (user chose a section).

## R12. Cross-repo phases

**Phase 4 (plugins)**, each its own issue and PR; per standing guidance, ask
before opening plugin-repo PRs:

| Repo | Change | Tracking |
| --- | --- | --- |
| finfocus-plugin-aws-public | `Name()` in `internal/plugin/plugin.go` and `internal/router/router.go` return the exported `PluginName` (`aws-public`); release notes say the old name keeps working through the catalog alias (R14) and recommend `aws-public` | rshade/finfocus-plugin-aws-public#502 |
| finfocus-plugin-aws-ce | `cmd/plugin/main.go` logger and `Source` in `internal/pricing/calculator.go` use `pluginName` | to file |
| finfocus-plugin-vantage | `Name()` returns a `PluginName` constant | to file |
| opencost, flexera, cloudzero | Optional: rename to exported `PluginName` | to file, low priority |

**Phase 5 (finfocus-spec)**: `pluginsdk.WithExpectedName(name)` as a
`ConformanceOption`; the basic level reports `Name() != PluginInfo().Name`
when the plugin provides plugin info, and `Name() != expected` when supplied.
Additive, so a patch or minor release. Core then bumps the dependency with the
plugin-upgrade hop and `references/to-vX.Y.Z.md`
(`internal/pluginupgrade/hops.go` tests require both), landing the bump as
its own `deps:` commit first (CLAUDE.md, Releases).

## R13. Web parity

Not applicable, confirmed: `--web --adapter` goes through
`internal/cli/web_data.go:320` → `openPlugins` → `registry.Open` and the
engine filters fixed in R5; dashboard pricing uses the same router built by
`createRouterForEngine`. No `internal/webui` change and no follow-up issue.

## R14. Former names as catalog aliases

**Decision**: Add `Aliases []string` (`json:"aliases,omitempty"`) to
`registry.RegistryEntry`, next to `Deprecated` and `Replacement`. Seed
`internal/registry/registry.json` with `aws-public: ["finfocus-plugin-aws-public"]`
and `cloudzero: ["finfocus-plugin-cloudzero"]`.

- `pluginhost.Client` gains `Aliases []string`. `registry.openPlugin` fills it
  from `GetPlugin(plugin.Name)` (which reads the `FINFOCUS_CATALOG_FILE`
  overlay too); a plugin with no catalog entry gets none. `pluginhost` never
  reads the catalog itself.
- `ValidateRegistryEntry` rejects an alias that fails `pluginname.Valid`; a
  catalog-wide check (in `GetAllPluginEntries` validation and a test over the
  embedded catalog) rejects an alias equal to any entry name or another alias.
- `ValidateRoutingConfig`: a name resolved `via` alias is valid with a
  warning. When the alias is also the client's reported name (not renamed
  yet): `"<alias>" will be retired; use "<install>"`; otherwise
  `"<alias>" is a former name of "<install>"; use "<install>"`. Routes test
  has no reported names, so it always uses the retiring wording. Cost runs do
  not warn (FR-022).
- `plugin list` and the dashboard catalog do not show aliases.
- Custom plugin registries (#2272, merged while this was in progress) may not
  offer a plugin under a built-in name; `builtinNames` (custom_service.go)
  now reserves every built-in alias as well, so `finfocus-plugin-aws-public`
  cannot be taken by a custom entry and read two ways.

**Rationale**: The user asked that the old aws-public name keep working after
the plugin's rename. The catalog is core-owned data that already carries
per-plugin lifecycle fields (`deprecated`, `replacement`), is readable without
launching, and applies to every installed version. Aliases are data, not code,
so no plugin name appears in Go source.

**Alternatives considered**:

- Plugin-advertised aliases in `GetPluginInfo` metadata: needs a launch (routes
  test and the `--adapter` filter cannot use it) and depends on every plugin
  version keeping the old names.
- Aliases in the user's routing config: puts the burden on the user the rename
  broke.
- A hardcoded `finfocus-plugin-` prefix strip: a name table in code, and wrong
  for any other former name.
