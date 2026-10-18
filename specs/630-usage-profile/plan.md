# Implementation Plan: Usage Profile

**Branch**: `630-usage-profile` | **Date**: 2026-10-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/630-usage-profile/spec.md`

## Summary

Teach FinFocus to resolve one usage profile per run and send it on
`GetProjectedCost` and `GetRecommendations`. The profile comes from
`--profile`, then `FINFOCUS_PROFILE`, then `cost.profile`, then no
preference. Measured-cost RPCs stay unchanged. FinFocus does not adjust
prices itself. A profile other than no preference is part of the projected
and recommendation cache keys, and projected batching is skipped for that
run because `BatchCost` has no profile field.

## Technical Context

**Language/Version**: Go 1.27.1
**Primary Dependencies**: finfocus-spec v0.7.6 (`UsageProfile`, `pluginsdk.ParseUsageProfile`), Cobra, ax-go
**Storage**: Existing BoltDB projected and recommendation cache. The profile is a key suffix. No new store.
**Testing**: testify, `go test`, `make test`, `make test-race`, `make lint`, `make validate`, `make docs-lint`
**Target Platform**: Linux, macOS, Windows CLI
**Project Type**: Single Go CLI and plugin host
**Performance Goals**: No extra plugin round trip when the profile is unset. A set profile uses the existing per-resource projected path.
**Constraints**: Do not change plugin prices in core. Do not send the profile on actual-cost RPCs, including the hourly-rate lookup actual cost uses when a plugin has no bill. Do not send it from `cost history`, which reuses projected pricing to record a baseline. Empty cache suffix when the profile is unset, so existing keys still hit.
**Scale/Scope**: One string on the engine, two RPC fields that already exist, one config field, one flag.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Core only forwards the hint. Hour counts and instance class stay in plugins.
- [x] **Test-Driven Development**: Engine, config, and CLI tests land with the behavior. No TUI layout change, so no new golden view. MCP tool schema changes are pinned in the existing golden.
- [x] **Cross-Platform Compatibility**: Flag, env, and config file. No OS-specific path.
- [x] **Documentation Integrity**: Command docs, config reference, and CLAUDE.md describe the flag, the variable, and `cost.profile`.
- [x] **Protocol Stability**: No proto change. finfocus-spec v0.7.6 already has the fields.
- [x] **Implementation Completeness**: Full resolution, validation, cache separation, and output. No stub.
- [x] **Persistence Model**: `cost.profile` is optional configuration. Commands work with the field absent.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint`, and `make docs-lint` before the PR.
- [x] **Multi-Repo Coordination**: Plugin behavior is already specified in finfocus-spec `042-usage-profile-context`. This change does not bump the spec module.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/630-usage-profile/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/usage-profile.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/config/budget.go          # cost.profile and validation
internal/config/profile.go         # label parsing shared with the CLI
internal/engine/engine.go          # field, cache suffix, RPC, skip batch
internal/engine/profile.go         # WithUsageProfile
internal/proto/adapter.go          # copy the enum onto the spec request
internal/cli/root.go               # persistent --profile on cost
internal/cli/usage_profile.go      # flag, env, config resolution
internal/cli/cost_projected.go     # apply and print
internal/cli/cost_recommendations.go
internal/cli/cost_forecast.go
internal/cli/cost_estimate.go
internal/cli/cost_cluster.go
internal/cli/overview_pipeline.go
internal/cli/web_data.go
internal/cli/analyzer_serve.go
internal/engine/diff_render.go     # JSON profile field
docs/src/content/docs/reference/config-reference.md
docs/src/content/docs/commands/overview.md
docs/src/content/docs/commands/forecast.md
```

**Structure Decision**: Keep the hint on the engine that already builds the
two RPCs. The CLI resolves the string once and calls `WithUsageProfile`.
Callers that must not send a hint, including actual cost and history
collection, leave the engine at no preference.

## Complexity Tracking

None.
