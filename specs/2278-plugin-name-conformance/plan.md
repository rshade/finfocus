# Implementation Plan: Plugin Name Conformance

**Branch**: `2278-plugin-name-conformance` | **Date**: 2026-11-19 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2278-plugin-name-conformance/spec.md`

## Summary

Core treats a plugin's install name and its reported `Name()` as two
unrelated strings, so routing rules and `--adapter` written with the documented
install name are rejected or silently ignored (#2278, #2206). The fix stores
the install name on `pluginhost.Client` and routes every user-supplied plugin
name through one matching rule that accepts the install name, a former name
listed as an `aliases` entry in the plugin catalog, or the reported name, with
warnings for entries that match nothing, for former names, and for plugins
whose names differ. The alias keeps `finfocus-plugin-aws-public` working after
aws-public itself is renamed. The conformance
suite gains three name checks with a new `warn` status that never changes the
exit code, the plugin-dev skill and `plugin init` template teach one
`PluginName` constant, and the cross-repo phases fix the drifting plugins and
add an in-process check to finfocus-spec.

## Technical Context

**Language/Version**: Go 1.27.4 (`go.mod`)
**Primary Dependencies**: Cobra, ax-go, gRPC, zerolog, finfocus-spec v0.7.12
(`pluginsdk`, `proto`), Masterminds semver (already used by the registry)
**Storage**: N/A (no persistent state)
**Testing**: testify (`require`/`assert`), table-driven unit tests, CLI
integration tests through `ax.Execute`, conformance suite tests with fake
plugins
**Target Platform**: Linux, macOS, Windows (binary name handling strips `.exe`)
**Project Type**: Single Go module (core) plus cross-repo follow-ups
**Performance Goals**: No added plugin launches for the documented spelling;
`--adapter` with a non-install spelling opens all plugins (as a run without
`--adapter` does)
**Constraints**: Adapter column and breakdown keys unchanged (FR-008); routes
test must not launch plugins; warnings must not change exit codes
**Scale/Scope**: ~10 packages touched in core; 3 required and 3 optional plugin
repos; 1 SDK change

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Orchestration only. No plugin name
  appears in Go code; former names are catalog data next to the existing
  `deprecated` and `replacement` fields, and the prefix the hint uses is
  core's own binary naming convention.
- [x] **Test-Driven Development**: Tests first for the matching rule, router
  resolution, validation messages, registry adapter filter, conformance checks,
  and skill contract. No TUI change.
- [x] **Cross-Platform Compatibility**: `pluginname.FromBinary` strips `.exe`;
  no platform-specific code.
- [x] **Documentation Integrity**: `guides/routing.md` (either name accepted),
  plugin conformance docs (new checks, `--expected-name`, warn status), the
  regenerated CLI reference (`make docs-cli`), the plugin-dev skill and its
  archive.
- [x] **Protocol Stability**: No proto change. Phase 5 is an additive SDK
  option.
- [x] **Implementation Completeness**: Each phase ships whole; cross-repo
  phases are tracked as issues, not TODOs.
- [x] **Persistence Model**: No persistent state.
- [x] **Web Parity**: Spec records Not applicable with a reason; research R13
  confirms `--web --adapter` and dashboard routing share the fixed paths.
- [x] **Quality Gates**: `make lint`, `make test`, `make docs-cli`,
  markdownlint before completion.
- [x] **Multi-Repo Coordination**: Phases 4 and 5 are documented with
  tracking issues; FR-018 keeps core independent of them. Phase 5's bump lands
  as its own `deps:` commit with a plugin-upgrade hop.

**Violations Requiring Justification**: None.

**Post-design re-check**: Still passes. The one process-wide state added
(R7's once-per-process warning set) is in-memory, has a test reset helper, and
its tests are marked non-parallel.

## Project Structure

### Documentation (this feature)

```text
specs/2278-plugin-name-conformance/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── cli-behavior.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit.tasks
```

### Source Code (repository root)

```text
internal/pluginname/            # NEW leaf: BinaryPrefix, Valid, Canonical, FromBinary
internal/pluginhost/
├── host.go                     # Client.InstallName (launchClient), Client.Aliases
└── identity.go                 # NEW: MatchesName, ResolveName
internal/registry/
├── registry.go                 # Open adapter filter (install name or alias); Client.Aliases; mismatch warning
├── entry.go                    # RegistryEntry.Aliases, alias validation
└── registry.json               # aliases for aws-public and cloudzero
internal/router/
├── router.go                   # configFor, unmatched, UnmatchedEntries, client-keyed lookups
└── validation.go               # ResolveName, did-you-mean, ambiguity warning
internal/engine/
├── engine.go                   # --adapter filters use MatchesName
├── actual_batch.go             # same
└── estimate_selection.go       # same
internal/conformance/
├── types.go                    # StatusWarn, Summary.Warnings, SuiteConfig.ExpectedName, TestContext.Identity
├── identity.go                 # NEW: ResolveExpectedName, PluginIdentity
├── protocol.go                 # three Name_* checks
├── suite.go                    # register checks, resolve identity, count warnings
├── reporter.go                 # warn in table, JSON, JUnit
├── runner.go                   # warn log level
└── certification.go            # warnings do not block; report line
internal/cli/
├── common_execution.go         # log UnmatchedEntries after createRouterForEngine
├── config_routes.go            # install-name and alias check, warnings in table and JSON
├── plugin_conformance.go       # --expected-name
├── plugin_init.go              # PluginName constant in template; IsValidPluginName -> pluginname.Valid
└── finfocus_plugin_dev_skill_test.go  # NEW contract + archive test
agent-skills/finfocus-plugin-dev/
├── SKILL.md                    # ## Naming conformance
├── references/naming.md        # NEW
└── finfocus-plugin-dev.skill   # rebuilt
docs/src/content/docs/
├── guides/routing.md           # either name accepted
└── reference/cli/              # regenerated by make docs-cli
```

**Structure Decision**: Existing single-module layout. The only new package is
the `internal/pluginname` leaf, needed because `internal/conformance` cannot
import `internal/cli` where the name rule lives today.

### Cross-repo work (not in this branch)

| Phase | Repo | Item |
| --- | --- | --- |
| 4 | finfocus-plugin-aws-public | #502: report `aws-public`; release notes say the old name keeps working through the core alias |
| 4 | finfocus-plugin-aws-ce | Issue to file: logger and `Source` use the name constant |
| 4 | finfocus-plugin-vantage | Issue to file: name constant |
| 4 | opencost, flexera, cloudzero | Optional convention rename |
| 5 | finfocus-spec | `WithExpectedName`, `Name()` vs plugin info check |
| 5 | finfocus (follow-up) | `deps:` bump, upgrade hop, `to-vX.Y.Z.md` |

Filing issues and opening PRs in plugin repositories waits for owner approval.

## Delivery order

1. `internal/pluginname` and `pluginhost` identity (foundation for both P1
   stories).
2. Router, validation, engine, registry adapter filter (Stories 1-2).
3. Catalog aliases and former-name warnings (Story 8), before any plugin
   rename ships.
4. Warnings: registry mismatch, unmatched entries, routes test (Story 3).
5. Conformance warn status and name checks (Story 4).
6. Skill section, `plugin init` template, contract test, docs (Story 5).
7. Cross-repo issues and PRs (Stories 6-7), after core with aliases is
   released.

## Complexity Tracking

None.
