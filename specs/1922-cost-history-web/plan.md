# Implementation Plan: Cost History on the Web

**Branch**: `1922-cost-history-web` | **Date**: 2026-10-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/1922-cost-history-web/spec.md`

## Summary

Three read-only endpoints, `GET /api/history/view`, `/api/history/diff`, and
`/api/history/resource`, return the stored cost history for the session
stack. They call the same Go functions as `cost history view`,
`cost history diff`, and the TUI detail loaders. The view and diff logic is
extracted from the Cobra commands into cmd-free functions first, and the CLI
output stays byte-identical. The JSON document types move into
`internal/history` so `internal/webui` can serialize them through
`RedactJSON` with data-keyed breakdown maps. Drawing the data is a UX-only
follow-up.

## Technical Context

**Language/Version**: Go 1.27.3
**Primary Dependencies**: net/http, BoltDB (`go.etcd.io/bbolt`) through `internal/history`, existing `viewmodel.FormatMoney`
**Storage**: Existing per-stack `<project>@<stack>.history.db`, read only
**Testing**: testify, `go test`, `make test`, `make lint`, `make validate`, `make test-e2e-web`
**Target Platform**: Linux, macOS, Windows (the same CLI and web server)
**Project Type**: Single Go module with `internal/webui` transport
**Performance Goals**: One database open per request, closed before the response is written
**Constraints**: `internal/webui` does not import `cli`; `internal/history` does not import `engine` or `ingest`; no write to the history store; CLI output unchanged
**Scale/Scope**: Three endpoints, three callbacks, one extraction in `internal/cli`, document types moved to `internal/history`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: No plugin call. Reads a local store core already owns.
- [x] **Test-Driven Development**: Tests for the extracted functions, handler tests with a temp history directory, and CLI-vs-web parity tests come first.
- [x] **Cross-Platform Compatibility**: No new OS calls; paths go through the existing resolver.
- [x] **Documentation Integrity**: `docs/src/content/docs/reference/web-ui.md` gains the endpoints. CLI docs are unchanged because CLI behavior is unchanged.
- [x] **Protocol Stability**: No proto change.
- [x] **Implementation Completeness**: No stubs. The chart and panel section are the follow-up issue the Web UI Fit requires.
- [x] **Persistence Model**: Reads the existing cost-history store read-only. A missing store answers "no history"; a damaged one is a reported failure, and the dashboard keeps running.
- [x] **Web Parity**: Web UI Fit extends the Overview and Cost detail panels and the Cost view. The contract names `readHistoryView`, `readHistoryDiff`, `loadDetailHistory`, and the history functions under them. Collect, prune, and export are recorded Not applicable with a reason.
- [x] **Quality Gates**: `make test`, `make lint`, `make validate`, `make test-race`, `make test-e2e-web`.
- [x] **Multi-Repo Coordination**: None. Core only.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/1922-cost-history-web/
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
internal/history/cost_types.go          # datakey tags
internal/history/document.go            # ViewDocument, DiffDocument, NewDiffDocument (moved)
internal/history/errors.go              # ErrNoCostHistory, ErrNoSnapshots, QueryError
internal/history/diff.go                # ResolveCostSnapshot returns ErrNoSnapshots
internal/cli/cost_history_query.go      # cmd-free view/diff reads and input checks
internal/cli/cost_history_view.go       # runView, viewBounds, applyViewCurrency call the shared code
internal/cli/cost_history_diff.go       # runDiff, readDiffFlags call the shared code
internal/cli/cost_history_export.go     # ErrNoCostHistory wrap; read-only opener
internal/cli/web_history.go             # (*webData).historyView/historyDiff/historyResource
internal/cli/web.go                     # SessionOptions wiring
internal/webui/session.go               # HistoryView, HistoryDiff, HistoryResource callbacks
internal/webui/handlers_history.go      # endpoints, status mapping, display strings
internal/webui/server.go                # registerHistory
docs/src/content/docs/reference/web-ui.md
```

**Structure Decision**: Follow the forecast and recommendation-history hooks:
domain in `internal/history`, wiring in `internal/cli/web_*.go`, transport in
`internal/webui/handlers_*.go`.

## Post-design Constitution Check

The handlers serialize the shared documents and add display strings. The web
opener is read-only. No new store and no protocol change. The gates hold.
