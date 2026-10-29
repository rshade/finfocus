# Implementation Plan: Recommendation Lifecycle History on the Web

**Branch**: `issue-1858` | **Date**: 2026-10-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/1921-recommendation-history-web/spec.md`

## Summary

`GET /api/recommendations/history` returns the lifecycle events already
produced by `(*engine.Engine).GetRecommendationHistory` for an id that the
session has already listed. The dashboard engine supplies the plugin
clients. Plugin failures become fixed warnings without changing the merge.
The Recommendations page does not render the table in this change.

## Technical Context

**Language/Version**: Go 1.27.3
**Primary Dependencies**: net/http, existing engine and config dismissal store
**Storage**: Existing dismissal file, read only
**Testing**: testify, `go test`, `make test`, `make lint`, `make validate`, `make test-e2e-web`
**Target Platform**: Linux, macOS, Windows (the same CLI and web server)
**Project Type**: Single Go module with `internal/webui` transport
**Performance Goals**: One history lookup per request, bounded by the existing per-plugin timeout
**Constraints**: No second plugin process set. No new credential rule. No change to CLI history output. `internal/webui` does not import `cli`.
**Scale/Scope**: One endpoint, one callback, warnings added beside the existing plugin loop

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: History stays a plugin RPC. Core only merges events it already merges.
- [x] **Test-Driven Development**: Handler tests, engine warning tests, and a CLI wiring parity test are part of the work.
- [x] **Cross-Platform Compatibility**: No new OS calls.
- [x] **Documentation Integrity**: `docs/src/content/docs/reference/web-ui.md` gains the endpoint. CLI history output is unchanged.
- [x] **Protocol Stability**: No proto change.
- [x] **Implementation Completeness**: No stubs. The UX table is a follow-up issue, as the spec's Web UI Fit requires.
- [x] **Persistence Model**: Reads the existing dismissal file. No new store.
- [x] **Web Parity**: Web UI Fit extends the Recommendations detail panel. The contract names `GetRecommendationHistory`, `queryEngine`, and `loadDismissalStore`.
- [x] **Quality Gates**: `make test`, `make lint`, `make validate`, and `make test-e2e-web`.
- [x] **Multi-Repo Coordination**: Uses the existing `GetRecommendationHistory` RPC. No finfocus-spec change.

**Violations Requiring Justification**: None.

Post-design re-check: the same gates hold. Warnings do not change
`mergeRecommendationHistory` or the CLI document.

## Project Structure

### Documentation (this feature)

```text
specs/1921-recommendation-history-web/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/http-api.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code

```text
internal/engine/recommendation_history.go
internal/cli/cost_recommendations_history.go
internal/cli/web_data.go
internal/cli/web.go
internal/webui/session.go
internal/webui/handlers_recommendations.go
internal/webui/server.go
docs/src/content/docs/reference/web-ui.md
```

## Post-design Constitution Check

The handler serializes `GetRecommendationHistory`. Rendering stays in the
UX follow-up. No new persistence and no protocol change.
