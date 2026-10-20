# Implementation Plan: Pulumi ESC Credentials for Plugins

**Branch**: `1730-pulumi-esc-plugin-credentials` | **Date**: 2026-10-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/1730-pulumi-esc-plugin-credentials/spec.md`

## Summary

Plugins receive credentials only through their process environment, which is
built by `pluginhost.ResolveDeliveredEnv` from four sources: the shell, the
project file, the global file, and the descriptor default. This feature adds
Pulumi ESC as a fifth, highest-precedence source.

- **Allowlist**: ESC values are limited per plugin to the variables it
  advertises or that `esc.plugins.<name>.variables` allows.
- **Where environments come from**: the stack's `environment:` imports,
  overridden by a new top-level `esc` configuration section.
- **Lazy resolution**: a new `internal/esc` package wraps esc-sdk v0.14.0 in
  a per-command `Source` that resolves on first use.
- **No CLI**: the stack, backend, organization, and token are found from
  Pulumi's own files without running the Pulumi CLI.
- **Fallback**: every failure falls back to today's sources with one
  categorized, redacted warning.
- **Process environment**: the delivered environment gains an override set,
  so an ESC value can replace a shell value.
- **Cache key**: the key hashes the ESC reference, a revision key (the ETag
  for latest, the number for a pinned revision, the resolved revision for a
  tag), and the delivered variable names, never ESC values. Rotating OIDC credentials, whose
  access key ID is not marked secret, therefore do not invalidate the cache,
  and an edit to the environment does.
- **Spike**: a spike against real Pulumi Cloud (research R12) verified the
  discovery chain, the SDK open, the failure categories, and the 5-second
  deadline. It also corrected the first cache-key design.
- **Dashboard**: the settings view reports `effectiveSource: "esc"` and the
  reference.

## Technical Context

**Language/Version**: Go 1.27.2 (`go.mod`)
**Primary Dependencies**:

- New: `github.com/pulumi/esc-sdk/sdk` v0.14.0 (adds `gopkg.in/ghodss/yaml.v1`)
- Existing: Cobra, ax-go, zerolog, testify, `gopkg.in/yaml.v3` for stack-file
  parsing (already used by `internal/pulumi`)

**Storage**: None new. ESC values are in memory only and never in the Bolt
cache, logs, or files (FR-013).
**Testing**:

- testify unit tests in `internal/esc`, `internal/pulumi`,
  `internal/pluginhost`, `internal/config`, and `internal/cli`.
- An `httptest` ESC server reached through the real `PULUMI_BACKEND_URL`
  variable. No finfocus test seam is added.
- An integration test with a script plugin that records its environment.
- `-race -shuffle=on -count=3` on the changed packages.

**Target Platform**: Linux, macOS, and Windows CLI
**Project Type**: Single CLI and plugin host
**Performance Goals**:

- At most one open per ESC reference per command, plus one `HEAD` for the
  ETag.
- Zero network calls when no selection applies.

**Constraints**:

- A shared 5-second budget per command (SC-004).
- No Pulumi CLI process.
- No change to the plugin protocol or to any plugin.
- Overview plugin launch keeps overlapping stack loading (#692).

**Scale/Scope**: About 8 production files touched and 1 new package. Docs:
one new guide, the configuration reference, and `CLAUDE.md`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with the FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Core gains a generic credential channel
  for plugin processes. It has no provider knowledge: which variables a plugin
  gets comes from that plugin's own descriptors or from user configuration,
  never from a table in core. ESC is part of Pulumi, the input domain core
  already integrates with. It is not a cost data source.
- [x] **Test-Driven Development**: Tests are written first for:
  - `ParseRef`;
  - stack, workspace, and credentials file parsing;
  - `Source` memoization, budget, and failure categories;
  - the precedence table;
  - the override in `appendDeliveredEnv`;
  - digest stability under rotation;
  - config validation;
  - the web settings view.

  `ResolveDeliveredEnv` and `appendDeliveredEnv` are plugin-communication
  critical paths and target 95% coverage. No TUI view changes.
- [x] **Cross-Platform Compatibility**: The changes use the standard library,
  plus an HTTP SDK with no cgo or keychain. The workspace-file hash uses the
  absolute `Pulumi.yaml` path as Pulumi computes it. A mismatch on any
  platform (for example drive-letter case on Windows) only means "no selected
  stack", which is the graceful path. Tests that run script plugins stay
  Unix-only as today (`settings_inject_test.go`).
- [x] **Documentation Integrity**: The plan covers:
  - a new guide, `docs/src/content/docs/guides/esc-credentials.md`;
  - the `esc` keys in the configuration reference;
  - the plugin-settings precedence text, which changes to say that ESC wins;
  - godoc on every exported symbol of `internal/esc`;
  - a package README (it has more than 3 exported symbols);
  - `CLAUDE.md` gotchas.
- [x] **Protocol Stability**: No proto or SDK change. Plugins read ESC values
  exactly as they read shell values today (FR-019).
- [x] **Implementation Completeness**: Inline stack definitions are a defined
  `unsupported_definition` fallback with a warning, not a stub. A possible
  `/yaml/open` follow-up would be a tracked issue, not a TODO.
- [x] **Persistence Model**: No new store. The only change to the existing
  cost cache is its key suffix, and deleting the cache stays safe.
- [x] **Web Parity**: The spec has a `Web UI Fit` section (it extends Plugins
  → Settings, with a Playwright render on 2026-10-19).
  `contracts/http-api.md` names `cli.pluginSettingsView` →
  `cli.pluginSettingValues` → `pluginhost.ResolveDeliveredEnv` /
  `esc.(*Source).Delivery`, the same resolution `pluginhost.NewClient` uses.
  A UX-only `web-ui` follow-up issue is filed at completion.
- [x] **Quality Gates**: `make validate`, `make test`, `make test-race`,
  `make lint`, `make test-integration`, and `govulncheck`. The probe reported
  no vulnerabilities for esc-sdk v0.14.0.
- [x] **Multi-Repo Coordination**: None required. No `finfocus-spec` change
  and no plugin change. #1539 (per-request credentials) is complementary and
  untouched.

**Violations Requiring Justification**: None.

**Post-design re-check (after Phase 1)**: All gates still hold.

- Moving `StackSettingsNameCandidates` and `ReadStackSettingsFile` from
  `internal/cli` to `internal/pulumi` is a refactor with no behavior change.
- The `ResolveDeliveredEnv` signature change is internal (`internal/`) and
  has no external consumers.
- Precedence where ESC beats the shell is a deliberate, documented behavior
  change. It is limited to variables ESC supplies (FR-006 and FR-007), so
  users without ESC see byte-identical plugin environments (SC-002).

## Project Structure

### Documentation (this feature)

```text
specs/1730-pulumi-esc-plugin-credentials/
├── plan.md
├── research.md          # R1–R11 decisions, R12 spike results
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── esc-package.md   # internal/esc API, pluginhost + pulumi seams, attachment points
│   ├── config.md        # esc section, keys, validation messages
│   └── http-api.md      # Principle VII hook: GET/POST plugin settings
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit.tasks
```

### Source Code (repository root)

```text
internal/esc/                      # NEW package
├── README.md
├── ref.go                         # Ref, ParseRef
├── auth.go                        # backend URL, token, org (R4); never writes Pulumi files
├── client.go                      # esc-sdk wrapper: hand-built Configuration, recover(), ETag HEAD
├── failure.go                     # Category, Failure, classification (R5)
├── source.go                      # Source, Options, Delivery, singleflight + memo, budget, warnings
├── values.go                      # environmentVariables extraction and stringify (R6)
└── *_test.go                      # httptest ESC server fixtures

internal/pulumi/
├── pulumi.go                      # pulumiProjectFile gains backend.url
├── stack_settings.go              # moved candidates/reader + StackEnvironment parsing
├── workspace.go                   # SelectedStack (workspace file, both formats)
└── stack_context.go               # LoadStackContext

internal/pluginhost/
├── settings_env.go                # ResolveDeliveredEnv(… fromESC) → Delivered; digest R7
├── host.go                        # WithDeliveredEnv carries Delivered
└── process.go                     # appendDeliveredEnv honors Overrides

internal/config/
├── config.go / merge.go           # ESC field, knownTopLevelKeys, unmarshalSection, Get/Set/List
├── descriptor.go                  # esc.environments + dynamic esc.plugins.<name>.{environments,variables}
└── validate.go                    # FR-015 messages

internal/cli/
├── root.go                        # attach esc.Source in PersistentPreRunE
├── overview.go                    # call moved pulumi stack-file helpers
├── web_data.go                    # fresh Source per reloadPlugins; webData.escSource
├── plugin_settings_web.go         # esc effectiveSource + escEnvironment; required satisfied by ESC
└── analyzer_serve.go              # Source with UseStack=false

internal/webui/
├── plugins_api.go                 # PluginSettingValue.ESCEnvironment
└── static/views/plugins.js        # one help sentence for esc rows

test/integration/
└── esc_plugin_env_test.go         # recorder-style script plugin + httptest ESC

docs/src/content/docs/
├── guides/esc-credentials.md      # NEW
├── reference/config-reference.md  # esc keys
└── reference/environment-variables.md  # PULUMI_ACCESS_TOKEN / PULUMI_BACKEND_URL as read by finfocus
```

**Structure Decision**: This is a single CLI project. All new logic lives in
one leaf package, `internal/esc`, behind a context-carried `Source`. That
keeps `pluginhost` the single place where precedence is decided and lets the
CLI and the web view share one resolution. Stack-file knowledge consolidates
in `internal/pulumi`.

## Risks

| Risk | Mitigation |
| --- | --- |
| esc-sdk is pre-1.0, and it panics on some traces | Pin v0.14.0; `recover()` around every call; Renovate tracks updates; tests use a recorded response shape |
| Workspace-file format changes in a future Pulumi | Two formats are supported. Any failure means "no selected stack" (graceful), and `--stack` still works |
| A plaintext token file becomes rare (keychain default) | A clear `not_logged_in` warning that names `PULUMI_ACCESS_TOKEN`. Documented in the guide |
| A value changes in an *imported* environment, and an old answer is reused until the TTL expires | Documented residual (R7). A user can clear the cache |
| Pulumi Cloud returns `404` for no-access as well as for missing | Warning says "not found or no access" (R12) |
| A cloned repository's `Pulumi.yaml` `backend.url` could receive the person's `PULUMI_ACCESS_TOKEN` | FR-020 trust rule (R4): an environment-variable token never goes to a project-sourced URL |
| A plugin opened late in a command (the scorer) hits an already-spent deadline | The budget charges in-flight ESC wait only, not wall-clock time (R10) |
| ESC beats the shell, which surprises someone who exports an override | Documented. `--debug` shows `esc_refs` per plugin, and the web view shows the ESC source |

## Complexity Tracking

No constitution violations.
