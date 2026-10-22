# Implementation Plan: Shared Stack Loader

**Branch**: `1733-shared-stack-loader` | **Date**: 2026-10-22 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/1733-shared-stack-loader/spec.md`

## Summary

One step, `internal/stackload`, loads a Pulumi stack for every consumer: the
projected family (`cost projected`, `forecast`, `estimate`, `recommendations`),
`overview` (plain, terminal, and `--web`), `cost actual`, and the web
dashboard. It runs `pulumi stack export` and `pulumi preview --json`
concurrently and returns a `Snapshot` from which each command takes its view:
overview rows (unchanged merge), descriptors for the live inventory (plan
first, then unchanged resources from state), or the deployed subset for
actual costs.

`cost projected` keeps its existing total over the resources the preview lists
(the plan total, still the budget input) and adds a deduplicated stack total.
A state that cannot be read degrades to the preview with a stated reason; a
saved preview reports how many unchanged resources it omitted. Research
(`research.md`) established each choice from the code and from measurements on
a real stack.

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`)
**Primary Dependencies**: Existing only: Cobra and ax-go (command wiring),
`internal/pulumi` (subprocess), `internal/ingest`, `internal/engine`,
testify. No new module.
**Storage**: N/A. No new store. History writes stay with the callers
(Constitution, Persistence Model).
**Testing**: testify (`require`, `assert`), `axtest.Run` for end-to-end CLI
tests, fixtures under `internal/cli/testdata/` and `test/fixtures/`, the web
frontend tests (`make test-frontend`), a parity test between plan-derived and
state-derived descriptors, and golden output for `cost projected`.
**Target Platform**: Linux, macOS, Windows (amd64, arm64 where already
built). Pure Go plus the `pulumi` subprocess the product already requires.
**Project Type**: Single Go module with nested plugin modules; this feature
touches the root module only.
**Performance Goals**: Within 3 seconds of v0.5.2 for `cost projected` on a
65-resource stack (SC-005). Measured baseline on `initial-30` `dev`: preview
11.5 to 13.6 s, stack export about 0.8 s, both in parallel 10.2 s.
**Constraints**: No credential or Pulumi secret reaches a plugin, a log, or an
error; error text from the backend never leaves `stackload` (a category does).
Plan total, budgets, and exit codes identical to v0.5.2 for every input.
**Scale/Scope**: Stacks of a few hundred resources. About ten commands and
three loader entry points change; no new command, flag, or endpoint.

## Constitution Check

*GATE: passed before Phase 0; re-checked after Phase 1 design below.*

- [x] **Plugin-First Architecture**: Orchestration only. No provider logic is
  added. Descriptors for unchanged resources go to the same plugins through the
  same calls.
- [x] **Test-Driven Development**: Tests come first in every slice (see
  Implementation Phasing). Coverage targets: 95% for `stackload` and the diff
  changes (critical path), 80% elsewhere. No `internal/tui` rendering changes,
  so no new golden files; the existing TUI goldens must stay green.
- [x] **Cross-Platform Compatibility**: No platform-specific code. Subprocess
  handling reuses `internal/pulumi`.
- [x] **Documentation Integrity**: Docs to update in the same PR are listed
  under Documentation. `stackload` is a new package with exported symbols and
  gets godoc and a package comment.
- [x] **Protocol Stability**: No protobuf or `finfocus-spec` change.
- [x] **Implementation Completeness**: Every slice ships complete; no stubs or
  TODOs. Open decisions are recorded in `research.md`, not left in code.
- [x] **Persistence Model**: No new store. Deleting every cache and history
  file still lets every command complete. The Pulumi stack state read here is
  external data, not local persistence.
- [x] **Web Parity**: The spec carries a Web UI Fit section, and
  `contracts/http-api.md` names the Go methods behind each affected endpoint
  (`stackload.Load`, `(*Snapshot).Descriptors`, `(*Snapshot).DeployedDescriptors`,
  `(*Snapshot).Rows`, `(*webData).estimateResources`,
  `(*webData).recommendations`, `(*webData).actualCosts`). The Playwright render
  check is a task, not yet done (below).
- [x] **Quality Gates**: `make lint`, `make test`, `make test-frontend`,
  `make docs-lint`, race tests on the new package and the diff.
- [x] **Multi-Repo Coordination**: None. No spec, plugin, or action change. The
  finfocus-action reads the existing JSON keys, which keep their meaning.

**Violations Requiring Justification**: None.

**Re-check after design**: Same result. The one new structural element, a new
package, is justified under Complexity Tracking.

**Pending**: Render evidence for the Web UI Fit section (spec) is not captured.
`contracts/http-api.md` makes it a required task before the design is final.

## Project Structure

### Documentation (this feature)

```text
specs/1733-shared-stack-loader/
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   ├── loader-api.md    # stackload package contract
│   ├── cli-output.md    # JSON, table, stderr additions
│   └── http-api.md      # web hook map (Principle VII)
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 (/speckit.tasks, not created here)
```

### Source Code (repository root)

```text
internal/
├── stackload/                    # new
│   ├── source.go                 # Source, Status, categories
│   ├── load.go                   # Load, LoadState, LoadPlan (concurrent export + preview)
│   ├── snapshot.go               # Snapshot, WithPlan, Rows, PendingChanges
│   ├── descriptors.go            # Descriptors, DeployedDescriptors
│   ├── mismatch.go               # preview/state disagreement, omitted-unchanged count
│   └── *_test.go
├── ingest/
│   └── pulumi_plan.go            # ChangeSummary on PulumiPlan
├── engine/
│   ├── types.go                  # DescriptorOrigin on ResourceDescriptor
│   ├── diff.go                   # split plan and state entries, stack totals
│   ├── diff_types.go             # Stack* summary fields, StackUnchanged
│   └── diff_render.go            # stack key in JSON, unchanged section in table
├── cli/
│   ├── common_execution.go       # resolveResourcesFromPulumi -> stackload
│   ├── cost_projected.go         # loadProjectedResources, notices, emptyPlan by origin
│   ├── cost_forecast.go          # inventory from snapshot, plan-origin total
│   ├── cost_estimate.go          # live inventory
│   ├── cost_recommendations.go   # live inventory
│   ├── cost_actual.go            # DeployedDescriptors
│   ├── overview.go               # drop duplicate export/preview helpers
│   ├── overview_pipeline.go      # three loaders call stackload
│   ├── web_data.go               # one snapshot store; drop capture hooks
│   ├── web.go                    # SessionOptions wiring (names unchanged)
│   └── testdata/pulumi/          # fixtures: unchanged stack, mismatch, secrets
docs/src/content/docs/
├── reference/cli-commands.md
├── commands/overview.md
├── reference/web-ui.md
└── architecture/web-ui.md        # only if the capture description changes
CLAUDE.md                         # architecture table: internal/stackload
```

**Structure Decision**: One new package, `internal/stackload`, because `engine`
cannot import `ingest` and `webui` cannot import `cli` (research R1). Everything
else is a change to an existing file. The web handlers in `internal/webui` do
not change.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --- | --- | --- |
| New package `internal/stackload` (not a principle violation; a structural addition) | The loading step must be importable by `internal/cli` and called for the web without `webui` importing `cli`; it needs `ingest`, which imports `engine` | Placing it in `engine` creates an import cycle; placing it in `cli` is where three divergent copies already live and the web cannot reach it directly |

## Implementation Phasing

Each slice is shippable and keeps `make test` and `make lint` green. Detailed
tasks come from `/speckit.tasks`.

1. **Foundation.** `ingest.PulumiPlan.ChangeSummary`; `stackload` with
   `Load`, `LoadState`, `LoadPlan`, `Snapshot`, `Rows`, `PendingChanges`, the
   mismatch and omitted-unchanged logic, and the compatibility tests from
   `contracts/loader-api.md`. No command changes yet.
2. **Overview.** The plain, terminal, and web overview loaders call
   `stackload`; the duplicate helpers in `overview.go` go. Output is
   byte-identical; the overview tests, TUI goldens, and web tests pin it.
3. **Projected family.** `Descriptors()`, `DescriptorOrigin`, the diff split,
   stack totals, JSON and table additions, the notices, and `emptyPlan` by
   origin. `cost estimate` and `cost recommendations` adopt the live
   inventory; `cost forecast` takes its inventory from the snapshot and keeps
   its plan-origin total. Golden tests prove v0.5.2 output for plan-only input.
4. **Web data.** `webData` keeps the snapshot; `captureState`, `capturePlan`,
   and `captureTerraform` collapse; `estimateResources`, `recommendations`, and
   `actualCosts` read from it. The web Playwright render check runs here.
5. **Actual.** `cost actual` and `cost history collect` use
   `DeployedDescriptors()`; history enrichment is untouched; the SC-009 test
   pins the deleted-resource case.
6. **Close-out.** Docs, `CLAUDE.md` architecture table, the `web-ui` follow-up
   issue, the quickstart run on `initial-30` `dev`, and the timing measurement.

## Risks

See the table in `research.md`. The two that decide the order of work are
descriptor parity between plan and state (slice 1 tests it before anything
depends on it) and secrets in state inputs (slice 3 tests it before the
projected family sends state-derived descriptors to a plugin).

## Open Decisions

Recorded in `research.md` under Open decisions: whether the forecast should
include unchanged resources, and Terraform's JSON no-change output. Neither
blocks implementation.
