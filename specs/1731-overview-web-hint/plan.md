# Implementation Plan: Overview Web Hint

**Branch**: `1731-overview-web-hint` | **Date**: 2026-10-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1731-overview-web-hint/spec.md`

## Summary

Add a one-line footer hint and a `w` key to the interactive overview list view.
`w` sets a request on the model and quits; after the existing teardown
(`finishInteractiveOverview`, which closes the overview's plugins),
`runInteractiveOverviewWithInit` calls the existing `runWebSession` with a
`webFlagValues` copied from the overview's parameters. No new gate: the TUI is
only built when `DetectResolvedOutputMode` says interactive, so plain, piped,
agent, and MCP runs never see the hint or key. Bare `finfocus` outside a
project keeps printing help.

## Technical Context

**Language/Version**: Go 1.27.2 (`go.mod`)
**Primary Dependencies**: Bubble Tea v2, Lipgloss v2 (`charm.land`), Cobra,
testify. No new dependency
**Storage**: N/A
**Testing**: testify; golden render tests in `internal/tui`
(`UPDATE_GOLDEN=1` to regenerate); `internal/cli` tests with a function-variable
seam for the web start
**Target Platform**: Linux, macOS, Windows (amd64/arm64 as in the constitution)
**Project Type**: single (CLI)
**Performance Goals**: web link printed within 1 s of the time `finfocus --web`
takes after the overview has closed (SC-002)
**Constraints**: hint never wraps; no input read added to any non-interactive
path; no change to `runWebSession` behavior
**Scale/Scope**: touches `internal/tui/overview_model.go`,
`overview_view.go`, `internal/cli/overview.go`, tests, goldens, one docs page

## Constitution Check

- [x] **Plugin-First Architecture**: Orchestration and UI only; no provider
  logic.
- [x] **Test-Driven Development**: Tests first for the key handler, hint
  widths, hand-off, and the plain-path no-change case. TUI change carries
  golden snapshots and a rendered inspection (quickstart).
- [x] **Cross-Platform Compatibility**: No platform-specific code. The `w`
  hand-off calls the existing `runWebSession`, which already isolates the
  Windows SIGTERM difference.
- [x] **Documentation Integrity**: `commands/overview.md` key table updated in
  the same PR; Godoc on the new accessor.
- [x] **Protocol Stability**: No proto or plugin-facing change.
- [x] **Implementation Completeness**: Full implementation, no stubs or TODOs.
- [x] **Persistence Model**: No persistent state.
- [x] **Web Parity**: Spec records `Not applicable` with a reason (the
  feature is the terminal's pointer to the existing web UI). Exempt from the
  hook, render check, and follow-up issue.
- [x] **Quality Gates**: `make lint`, `make test`, golden update, markdownlint.
- [x] **Multi-Repo Coordination**: Core only.

**Violations Requiring Justification**: None.

Post-design re-check: no change. The design adds one model field, one view
helper, and a call to an existing function.

## Project Structure

### Documentation (this feature)

```text
specs/1731-overview-web-hint/
├── plan.md
├── spec.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── tui-contract.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit.tasks
```

### Source Code (repository root)

```text
internal/tui/
├── overview_model.go        # webRequested field, WebRequested(), `w` in handleListKeypress
├── overview_view.go         # hint line under the status bar (width-aware)
├── overview_model_test.go   # key handling: list, filter, passphrase, detail
├── overview_view_test.go    # hint at long / short / omitted widths
├── overview_golden_test.go  # list view golden with the hint
└── testdata/                # regenerated overview goldens

internal/cli/
├── overview.go              # hand off to runWebSession after teardown
└── overview_web_handoff_test.go   # hand-off builds webFlagValues; plain path unchanged

docs/src/content/docs/commands/overview.md   # key table and hint
```

**Structure Decision**: Single project. All changes sit in the two existing
packages that own the overview TUI and its command; no new package.

## Complexity Tracking

No violations.

## Spec adjustment from research

R5 narrows "while overview data is still loading" to "while rows are still
loading in the list view", because the key exists only once the list view is
shown. Applied to spec.md (Story 2 scenario 3, FR-005).
