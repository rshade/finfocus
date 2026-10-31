# Implementation Plan: Daily Actual-Cost Series

**Branch**: `issue-1862` | **Date**: 2026-10-31 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/1924-daily-actuals/spec.md`

## Summary

Add `finfocus cost history collect-daily`. It prices actual cost for the
stack, groups the plugin's timestamped buckets into one point per resource
per UTC day per currency, and writes those points to a new Bolt file beside
the projected timeline. The even split in `DailyCosts` is not stored. The
timeline schema stays version 1. The command is flag-only, MCP-excluded, and
has no web handler.

## Technical Context

**Language/Version**: Go 1.27.3
**Primary Dependencies**: Cobra, ax-go, bbolt, testify. No finfocus-spec bump.
**Storage**: Optional Bolt file `<project>@<stack>.daily.db` in the existing
history directory. Safe to delete; the next collect rebuilds the window it
prices. Commands that do not read it are unchanged.
**Testing**: `go test` with testify in `internal/history` and `internal/cli`
**Target Platform**: Linux, macOS, Windows (symlink cases skip where the OS
cannot create them)
**Project Type**: Single Go CLI module
**Performance Goals**: One actual-cost pass, the same order of cost as
`finfocus cost actual` for the same stack and window
**Constraints**: `internal/history` must not import `engine` or `ingest`.
No new plugin RPC. Credential names stay on `config.IsCredentialName`.
**Scale/Scope**: One file per stack. A 30-day collect of a few hundred
resources is one Bolt transaction.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Core stores amounts plugins already
  return. No provider pricing logic.
- [x] **Test-Driven Development**: Store, mapper, command, path gate, and
  MCP tests are part of the feature. No TUI, so no golden view.
- [x] **Cross-Platform Compatibility**: Path and Bolt code uses `path/filepath`
  and `os`. Symlink tests skip on Windows when links cannot be created.
- [x] **Documentation Integrity**: User guide, CI guide, CLI reference,
  `CONTEXT.md`, and `CLAUDE.md` update with the command.
- [x] **Protocol Stability**: No proto or finfocus-spec change.
- [x] **Implementation Completeness**: No stubs. Read and prune exist so the
  file is a real store for #1863.
- [x] **Persistence Model**: The file is optional. Other commands succeed
  without it. A corrupt or wrong-kind file fails that open and does not
  rewrite the timeline. Documented in `CONTEXT.md`.
- [x] **Web Parity**: Spec Web UI Fit is Not applicable. No hook, because a
  handler that started collection would be a new web surface the issue
  forbids. No follow-up issue.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint`, and
  `make test-race` before the pull request.
- [x] **Multi-Repo Coordination**: None.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/1924-daily-actuals/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli.md
│   └── store.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/history/
├── daily.go              # DailyPoint, file names, PointsForSeries
├── daily_db.go           # DailyDB open, replace, list, prune
├── daily_test.go
├── daily_db_test.go
└── cost_db.go            # refuse a daily file; schema version stays 1

internal/cli/
├── cost_history_collect_daily.go
├── cost_history_collect_daily_test.go
├── cost_history.go       # register the command
├── history_record.go     # jail also covers *.daily.db
├── history_dir_test.go
├── mcp.go
└── output_validation_test.go

docs/src/content/docs/guides/user-guide.md
docs/src/content/docs/guides/ci-cd-cost-tracking.md
docs/src/content/docs/reference/cli-commands.md
CONTEXT.md
CLAUDE.md
```

## Post-Design Constitution Check

The design adds one optional Bolt file and no web route. Gates above still
hold. `costSchemaVersion` is not bumped. Plugin requests stay the actual-cost
request `cost actual` already sends.
