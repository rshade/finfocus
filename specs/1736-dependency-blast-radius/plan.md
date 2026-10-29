# Implementation Plan: Dependency Blast Radius

**Branch**: `1736-dependency-blast-radius` | **Date**: 2026-10-28 |
**Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `specs/1736-dependency-blast-radius/spec.md`

## Summary

For a selected resource in the overview detail view and the web Overview
detail panel, list everything that depends on it (a tree from direct
dependents outward) with the monthly cost they carry. The data is the links
Pulumi records on each resource (`dependencies`, `propertyDependencies`,
`provider`, `deletedWith`, `replaceWith`; research R12), which core parses and
then drops before the overview. The plan keeps those edges in the shared stack
snapshot, computes the radius in one pure engine function, and has the
terminal view and the web handler both call it. No plugin, protocol,
`finfocus-spec` or setting change; this replaces the unshipped
`CostAllocationLineage` plan in issue #366.

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`)  
**Primary Dependencies**: existing only: Bubble Tea v2 / Lipgloss v2
(`charm.land` imports), testify, the embedded web UI under
`internal/webui/static`  
**Storage**: N/A. Nothing is persisted; the graph lives in the in-memory
`stackload.Snapshot`.  
**Testing**: testify (`require`/`assert`); table-driven engine tests; TUI
golden files (`internal/tui/testdata/detail_*.golden`); web handler tests in
`internal/webui`; `-race -shuffle=on -count=3` for parallel safety  
**Target Platform**: Linux, macOS, Windows (terminal and browser)  
**Project Type**: single Go module, plus the embedded web assets  
**Performance Goals**: opening a detail view on a 500-resource stack is
immediate (SC-004): one O(V+E) traversal, no plugin calls (FR-011)  
**Constraints**: no new setting, route or persisted state; non-interactive
output unchanged (FR-013); `internal/webui` must not import `cli`, `registry`
or `pluginhost`; the payload carries no properties or tags  
**Scale/Scope**: stacks up to a few thousand resources; one new engine file,
one converter in `stackload`, one TUI section, one web field and panel block

No `NEEDS CLARIFICATION` items remain. One fact needs a real state file to
confirm (research R1, quickstart step 2); the design degrades to a simpler
rule if it is wrong.

## Constitution Check

Checked against `.specify/memory/constitution.md` v1.8.1.

- [x] **Plugin-First Architecture**: Orchestration logic only. No provider
  logic and no plugin domain knowledge; cost values come from the plugins as
  they do today.
- [x] **Test-Driven Development**: Engine function and converter tests first
  (table-driven: diamond, cycle, dangling reference, component member,
  deleting, creating, unpriced, mixed currency). TUI changes get golden files
  and a rendered-output read, not `assert.Contains` alone.
- [x] **Cross-Platform Compatibility**: Pure Go and ASCII-safe tree glyphs via
  the existing accessibility plain mode; no OS-specific code.
- [x] **Documentation Integrity**: Update
  `docs/src/content/docs/commands/overview.md`, the web UI guide
  (`docs/src/content/docs/guides/web-ui.md`), Godoc on every new exported
  symbol, and the stale `ROADMAP.md` entry that cites `CostAllocationLineage`.
  `make docs-lint` and markdownlint run on every touched page.
- [x] **Protocol Stability**: No protobuf or `finfocus-spec` change.
- [x] **Implementation Completeness**: No stubs or TODOs; every task in
  `tasks.md` ships implemented.
- [x] **Persistence Model**: Not engaged; nothing new is stored.
- [x] **Web Parity**: The spec carries a Web UI Fit section (extends the
  Overview detail panel). The contract names the Go methods:
  `engine.ComputeBlastRadius`, called by `handleOverviewResource`, and
  `Snapshot.Graph`. The web adds no engine logic. The UX refinement of
  the detail-panel block is part of this feature (spec Web UI Fit, tasks
  T041 to T050). The user decided on 2026-10-29 not to file the
  `web-ui` follow-up issue Principle VII.3 asks for, since the UX work ships
  in this branch; the constitution is unchanged.
- [x] **Quality Gates**: `make lint`, `make test`, and a check against a real
  stack in `../finfocus-demo` before claiming success.
- [x] **Multi-Repo Coordination**: None. The issue's `finfocus-spec`
  dependency is dropped because the symbol never existed.

**Violations Requiring Justification**: none.

**Post-design re-check**: unchanged. The design adds one engine function, one
snapshot field and one existing-route field; no gate moved.

**Re-check after ISS-001 (2026-10-28)**: unchanged. The widened edge set
reads two more `ingest` fields and the existing `provider` field, stays
inside `internal/ingest` and `internal/stackload`, and adds no plugin call,
setting, route, payload field or persisted state. Plugin-First holds: the
link kinds are Pulumi's, not a provider's, and no provider name is matched.
Web Parity holds without web changes, because the handler and the TUI call
the same `ComputeBlastRadius` over the same `Snapshot.Graph`.

## Project Structure

### Documentation (this feature)

```text
specs/1736-dependency-blast-radius/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── go-api.md
│   └── http-api.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit.tasks - not created by this command
```

### Source Code (repository root)

```text
internal/
├── engine/
│   ├── blast_radius.go          # DependencyGraph, BlastRadius, ComputeBlastRadius (new)
│   └── blast_radius_test.go     # (new)
├── ingest/
│   ├── state.go                 # StackExportResource.DeletedWith, ReplaceWith (ISS-001)
│   └── pulumi_plan.go           # PulumiState.DeletedWith, ReplaceWith (ISS-001)
├── stackload/
│   ├── graph.go                 # ingest -> engine.DependencyGraph, preview overlay (new); all link kinds (ISS-001)
│   ├── graph_test.go            # (new)
│   ├── snapshot.go              # Graph field and accessor; WithPlan rebuilds it
│   └── load.go                  # keep all deployment resources for the graph
├── tui/
│   ├── overview_view.go         # BLAST RADIUS section in renderDetailViewForDay
│   ├── overview_messages.go     # graph-delivery message
│   ├── overview_model.go        # holds the graph, handles the message
│   └── testdata/detail_*.golden # new and updated goldens
├── webui/
│   ├── handlers_overview.go     # ResourceDetail.BlastRadius
│   ├── session.go               # SetDependencyGraph
│   ├── static/views/overview.js # detail-panel block and UX refinement
│   ├── static/styles.css        # .blast-* rules: scroll, sticky headings, narrow layout
│   └── frontendtest/overview.test.mjs # renderer, interaction and copy tests
└── cli/
    ├── overview_pipeline.go     # send the graph to the TUI
    └── web_data.go              # pass the graph to the web session
docs/src/content/docs/commands/overview.md
docs/src/content/docs/guides/web-ui.md
ROADMAP.md                       # drop the CostAllocationLineage dependency note
```

**Structure Decision**: Keep the single Go module. The computation sits in
`internal/engine` beside `ComputeOverviewResult`, the graph is built in
`internal/stackload` (which already converts `ingest` types for `engine`),
and both renderers read the result, so the terminal and web cannot disagree
(FR-009).

## Complexity Tracking

No violations to justify.

## Discovered Constraints

### ISS-001: Pulumi link kinds beyond `dependencies` (resolved 2026-10-28)

Found after implementation during a web walkthrough of `initial-30`;
decision in spec Clarifications (session 2026-10-28), evidence and
alternatives in research R12. The graph followed
`dependencies` and `propertyDependencies` only, so a resource reached through
a provider (every Kubernetes resource on an in-stack cluster) was missing.

Design:

- `internal/ingest`: add `DeletedWith string` and `ReplaceWith []string` to
  `StackExportResource` and `PulumiState`, with the `apitype.ResourceV3` JSON
  names. `Provider` is already parsed on both.
- `internal/stackload/graph.go`: replace `mergeDependencies` with one helper
  that takes every link kind and returns the sorted, de-duplicated
  `DependsOn`. `BuildDependencyGraph` passes the state resource's links;
  `stepDependencies` passes the step state's, or the step's own `Provider`,
  `Dependencies` and `PropertyDependencies` when the state is nil. A provider
  reference becomes its URN by cutting at the last `::`; a reference without
  one adds nothing.
- `isStepComponent`: the `pulumi:` fallback excludes `pulumi:providers:`.
- Unchanged: `engine.ComputeBlastRadius`, the TUI section, the web handler and
  panel, the HTTP contract, and the `Available` rule (any non-empty
  `DependsOn`).

Tests first (`internal/stackload/graph_test.go`): a provider reference is an
edge on the provider URN; a provider-only state is available; a malformed
reference adds nothing; `deletedWith` and each `replaceWith` entry are edges;
a preview step's provider reference with the unknown-id sentinel still yields
the URN; a provider created in a preview without `custom` is not a component.
One engine test pins the cluster shape end to end (cluster, provider built
from it, workloads on the provider: the cluster's radius lists all of them).
Docs: the "What counts as a dependent" list in
`docs/src/content/docs/commands/overview.md`, and the blast-radius bullet in
`CLAUDE.md`. Real-stack check: quickstart step 5.
