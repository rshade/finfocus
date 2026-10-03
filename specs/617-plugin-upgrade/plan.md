# Implementation Plan: Plugin upgrade command and agent skill

**Branch**: `617-plugin-upgrade` | **Date**: 2026-10-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/617-plugin-upgrade/spec.md`

## Summary

Add `finfocus plugin upgrade` and the `finfocus-plugin-upgrade` agent skill.
A new package, `internal/pluginupgrade`, holds the hop table, detection,
planning, and the mechanical edits. The CLI command wraps it in
`ax.Perform`, so the global `--dry-run` prints the plan. The skill calls the
command with `--output json` and works through each hop's guide. Research
and the hop table are in [research.md](research.md).

## Technical Context

**Language/Version**: Go 1.27.1 (see `go.mod`)
**Primary Dependencies**: Cobra, ax-go (`ax.Perform`, dry run),
`golang.org/x/mod/modfile` (new direct dependency, from the Go project),
`github.com/Masterminds/semver/v3` (existing), `go/parser` (stdlib)
**Storage**: None. The command edits files in the plugin's directory only.
**Testing**: testify; fixture plugin trees under
`internal/pluginupgrade/testdata/`, copied to `t.TempDir()` per test
**Target Platform**: Linux, macOS, Windows. Git is invoked through
`exec.Command("git", …)` for the clean-tree check only
**Project Type**: CLI
**Constraints**: No network access; edits preserve formatting outside the
replaced spans

## Constitution Check

| Principle | Status |
| --- | --- |
| I. Plugin-First | Pass. Developer tooling for plugin authors; no provider logic in core |
| II. TDD | Pass. Detection, planning, apply, and the drift test are written first |
| III. Cross-Platform | Pass. `filepath` throughout; git via `exec`, no shell |
| IV. Documentation Integrity | Pass. CLI reference, MCP exclusion table, skills README, and the drift test that ties guides to code |
| V. Protocol Stability | Pass. No protocol change |
| VI. Completeness | Pass. No stubs; the command does all deterministic edits it plans |

## Design

### `internal/pluginupgrade`

- `hops.go`: `Hop{To, MinGo, Summary, Automatic, Manual, Guide}` and the
  ordered table from research.md. `Guide` is `to-vX.Y.Z.md`.
- `detect.go`: `Detect(dir) (*Project, error)`. Parses `go.mod` with
  `modfile`; finds the finfocus-spec require, `replace` directives for it,
  and the `go` directive. Walks `*.go` files (skipping `vendor`, `testdata`,
  dot directories, and nested modules) with `go/parser` to find top-level
  `SpecVersion` string constants or variables.
- `plan.go`: `NewPlan(project, target) (*Plan, error)`. Selects hops with
  `project < hop.To <= target`, the highest `MinGo` among them, and warnings
  (SpecVersion mismatch or missing `v` prefix, replace directive). Rejects a
  target above core's or below the project's; rejects projects below v0.5.0.
- `apply.go`: `Apply(project, plan) ([]string, error)`. Byte-span edits of
  the `SpecVersion` literals recorded by `Detect`; `modfile` for `go.mod`
  (`AddRequire`, `AddGoStmt`). Verifies every file is unchanged since
  `Detect` before writing, writes `go.mod` last, and returns changed files,
  relative and sorted.
- `git.go`: `CheckClean(dir) error`, using `git status --porcelain`.

### `internal/cli/plugin_upgrade.go`

Flags: `--dir` (default `.`), `--to`, `--allow-dirty`, `--output
table|json` through `resolveOutputFormat`. Validates `--output` first.
Default target is `pluginsdk.SpecVersion`. The rehearse path prints the
plan. The commit path runs the clean check, applies, and prints the plan,
changed files, and next steps (`go mod tidy`, `go build ./...`,
`go test ./...`). Added to `mcpExcludedCommands`.

### `agent-skills/finfocus-plugin-upgrade/`

`SKILL.md` with the workflow (plan, approve, branch, apply, fix by guide,
verify), `references/to-v*.md` for every hop, and the packaged `.skill`
archive in the same layout as the existing skills.

## Project Structure

```text
specs/617-plugin-upgrade/   spec.md research.md plan.md tasks.md
internal/pluginupgrade/     hops.go detect.go plan.go apply.go git.go (+ tests, testdata/)
internal/cli/               plugin_upgrade.go (+ test), root.go, mcp.go
agent-skills/finfocus-plugin-upgrade/
docs/src/content/docs/reference/cli-commands.md
docs/src/content/docs/guides/mcp.md
```

## Complexity Tracking

None.
