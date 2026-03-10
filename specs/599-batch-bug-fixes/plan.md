# Implementation Plan: Batch Bug Fixes — Analyzer, Registry, Config, and CI

**Branch**: `599-batch-bug-fixes` | **Date**: 2026-02-21 | **Spec**: [spec.md](spec.md)

## Summary

Fix 11 confirmed bugs spanning the Pulumi analyzer integration, plugin registry,
configuration system, recorder plugin, and CI pipeline. Each fix is independently
implementable and testable. The largest fixes are the AnalyzeStack zero-cost bug
(cache lifecycle correction) and the analyzer log redirect (file-based logging when
`FINFOCUS_ANALYZER_MODE=true`). All other fixes are small, targeted one-file changes.

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: `github.com/rshade/finfocus-spec` (pluginsdk, proto types),
`github.com/rs/zerolog` (logging), `github.com/spf13/cobra` (CLI), `google.golang.org/grpc`
**Storage**: N/A (no new persistent storage; BoltDB cache is untouched)
**Testing**: `go test`, `testify/assert`, `testify/require`; `make test` for full suite
**Target Platform**: Linux, macOS, Windows (all fixes are platform-agnostic except the
registry symlink fix, which is gated on OS symlink support)
**Project Type**: Single Go module (CLI + library)
**Performance Goals**: No performance regression; fixes must not add observable latency
**Constraints**: No `.golangci.yml` changes; no proto/spec changes (all fixes are in core);
`make lint` and `make test` must pass before any task is marked complete
**Scale/Scope**: 11 bug fixes across 8 packages; no new packages needed

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: All fixes are in the orchestration layer (core,
  registry, config, CLI). No direct provider integrations introduced. The recorder
  plugin fix is within the reference plugin, not a new provider integration.
- [x] **Test-Driven Development**: Each fix includes a unit test. The `AnalyzeStack`
  fix (#746) and registry fix (#750) include integration-level tests. 80% minimum
  coverage maintained.
- [x] **Cross-Platform Compatibility**: Registry symlink fix uses `os.Stat` (works on
  all platforms); the test for symlinks is gated on OS symlink support (skipped on
  Windows where `os.Symlink` may require elevated privileges). All other fixes are
  platform-agnostic.
- [x] **Documentation Integrity**: `docs/reference/environment-variables.md` and
  `docs/reference/config-reference.md` updated to reflect implemented behavior. The
  `plugins.dir` key is renamed to `plugin_dir:` at the top level.
- [x] **Protocol Stability**: No proto changes. All fixes are in core logic only.
- [x] **Implementation Completeness**: No TODOs, no stubs. Each fix is a complete
  implementation. Issue #754 is explicitly deferred (P4) and tracked in the issue
  tracker, not as a TODO in code.
- [x] **Quality Gates**: `make test` and `make lint` must pass after each fix.
- [x] **Multi-Repo Coordination**: No cross-repo changes required. The recorder plugin
  is in this repo (`plugins/recorder/`); the fix to `GetRecommendations` touches
  only the in-repo reference plugin.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/599-batch-bug-fixes/
├── plan.md              # This file
├── research.md          # Phase 0 — all 11 bug investigations
├── data-model.md        # Phase 1 — entities and lifecycle changes
├── quickstart.md        # Phase 1 — ordered developer guide
├── contracts/
│   └── overview.md      # Phase 1 — corrected behavioral contracts
└── tasks.md             # Phase 2 — /speckit.tasks output (not yet created)
```

### Source Code (modified files)

```text
.github/workflows/
└── docker.yml                          # #698: contents: write permission

internal/
├── analyzer/
│   ├── install.go                      # #749: double-v version directory fix
│   ├── install_test.go                 # #749: version normalization test
│   ├── server.go                       # #746: clearCostCache() lifecycle fix
│   └── server_test.go                  # #746: AnalyzeStack summary test
├── cli/
│   ├── analyzer_serve.go               # #751: filter disabled plugins
│   ├── analyzer_serve_test.go          # #751: Enabled=false test
│   ├── logging_setup.go                # #748: analyzer mode log redirect
│   └── logging_setup_test.go          # #748: analyzer mode log redirect test
├── config/
│   ├── config.go                       # #752 #753: FINFOCUS_PLUGIN_DIR + plugin_dir
│   └── config_test.go                  # #752 #753: env/config override tests
├── engine/
│   └── overview_enrich.go              # #723: debug logging for $0 investigation
├── proto/
│   └── adapter.go                      # #723: debug logging in resolveSKUAndRegion
└── registry/
    ├── registry.go                     # #750: os.Stat symlink fix
    └── registry_test.go                # #750: symlink discovery test

plugins/recorder/
├── plugin.go                           # #747: non-nil Summary in GetRecommendations
└── plugin_test.go                      # #747: Summary != nil assertion

docs/reference/
├── environment-variables.md            # #752: confirm FINFOCUS_PLUGIN_DIR documented
└── config-reference.md                 # #753: update plugins.dir → plugin_dir
```

## Implementation Sequence

Ordered by size (smallest first), with dependencies respected.

| Order | Issue | File(s) | Effort | Dependencies |
|-------|-------|---------|--------|--------------|
| 1 | #698 SBOM permission | `docker.yml` | 5 min | None |
| 2 | #749 Double-v directory | `install.go` | 30 min | None |
| 3 | #750 Registry symlinks | `registry.go` | 45 min | None |
| 4 | #747 Recorder nil summary | `recorder/plugin.go` | 30 min | None |
| 5 | #752 FINFOCUS_PLUGIN_DIR | `config.go` | 30 min | None |
| 6 | #753 plugin_dir config key | `config.go` | 45 min | #752 (same file) |
| 7 | #751 AnalyzerPlugin.Enabled | `analyzer_serve.go` | 45 min | None |
| 8 | #748 Analyzer log redirect | `common_execution.go` | 60 min | None |
| 9 | #746 AnalyzeStack zero cost | `server.go` | 90 min | None |
| 10 | #723 TUI intermittent $0 | `adapter.go`, `overview_enrich.go` | 120 min | None |
| 11 | #754 Force reinstall sync | Deferred | — | Policy pack feature |

## Phase 0: Research Findings

See [research.md](research.md) for full details. Summary of key decisions:

| Issue | Decision |
|-------|----------|
| #750 | `os.Stat(path).IsDir()` at both directory levels; error → skip |
| #749 | Prefix `"analyzer-finfocus-"`, normalize version to always have single `v` |
| #746 | Move `clearCostCache()` from `ConfigureStack` → end of `AnalyzeStack` |
| #748 | Check `FINFOCUS_ANALYZER_MODE`; redirect `Logging.Outputs` to file when true |
| #747 | Add non-nil `pbc.RecommendationsSummary{}` to all recorder response paths |
| #752 | Add `FINFOCUS_PLUGIN_DIR` to `applyEnvOverrides()` |
| #753 | Add `PluginDirOverride string \`yaml:"plugin_dir"\`` to `Config`; update docs |
| #751 | Filter `clients` slice after `reg.Open()` based on `cfg.Analyzer.Plugins[name].Enabled` |
| #698 | Change `contents: read` → `contents: write` in `docker.yml` |
| #723 | Add debug logging in `resolveSKUAndRegion` and `enrichProjectedCost`; investigate |
| #754 | Deferred (depends on policy pack setup feature) |

## Phase 1: Design

See [data-model.md](data-model.md) and [contracts/overview.md](contracts/overview.md).

### Key Design Decisions

**`plugins.dir` vs `plugin_dir`**: The existing `plugins:` YAML key maps to a
`map[string]PluginConfig` (keyed by plugin name). Adding a `dir:` field inside this
map would require custom `UnmarshalYAML` to distinguish structural keys from plugin
names. Instead, `plugin_dir:` is added as a top-level config key. Documentation is
updated accordingly. No breaking change to existing per-plugin config entries.

**`AnalyzerPlugin.Enabled` zero value**: The Go zero value for `bool` is `false`.
To distinguish "plugin not mentioned in config" (enable by default) from "plugin
explicitly set to `enabled: false`" (disable), the filter only excludes clients
that are **present in `cfg.Analyzer.Plugins` AND have `Enabled == false`**. Plugins
absent from the map default to enabled.

**Registry symlink — Windows**: `os.Symlink` may require elevated privileges on
Windows. The registry symlink test is gated with `t.Skipf` on `runtime.GOOS ==
"windows"` to avoid CI failures while preserving the fix behavior on all platforms.

**Analyzer log redirect — scope**: The redirect applies only when
`FINFOCUS_ANALYZER_MODE=true`. This env var is set exclusively by `RunAnalyzerServe()`
before any `PersistentPreRunE` logging initialization occurs, so the redirect activates
reliably for all Pulumi-launched invocations.

## Complexity Tracking

No constitution violations. No complexity justification required.
