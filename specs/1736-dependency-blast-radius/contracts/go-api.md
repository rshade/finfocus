# Contract: Go API shared by the terminal and web

This is the only computation of the blast radius. The terminal view and the
web handler both call it (FR-008, FR-009); the web computes nothing itself.

## `internal/engine`

```go
// ComputeBlastRadius returns the dependents of the resource at urn, directly or
// through other resources, using one basis (projected monthly cost).
// rows supplies type, status and cost; graph supplies the edges.
func ComputeBlastRadius(graph DependencyGraph, rows []OverviewRow, urn string) BlastRadius
```

Behavior:

- Pure and deterministic: no I/O, no plugin calls (FR-011), same output for
  the same input, safe to call from several goroutines on read-only inputs.
- `graph.Available == false` returns `BlastRadius{URN: urn, Available: false}`.
- An unknown `urn` returns an empty, available result; callers decide whether
  that is a 404 (the web handler already returns 404 for an unknown row).
- Never returns an error and never panics on a cycle or a dangling reference;
  the latter increments `LeftOut.Unresolved`.
- Field semantics and invariants: `data-model.md`.

## `internal/stackload`

```go
// Graph returns the dependency graph for this snapshot, including the preview
// overlay when one is loaded. The returned value must be treated as read-only.
func (s *Snapshot) Graph() engine.DependencyGraph
```

- Built at load from every deployment resource (custom and component).
- Edges are every link kind spec FR-005 lists (research R12); the signature
  and the HTTP payload are unchanged by ISS-001.
- Rebuilt by `WithPlan`, so an on-demand web preview publishes a new graph
  through the existing `setPipelineSource` path.
- Terraform and OpenTofu sources return `Available: false`.

## Delivery to the two renderers

| Renderer | How it gets the graph | Where it calls `ComputeBlastRadius` |
| --- | --- | --- |
| Terminal overview | A new Bubble Tea message carrying the graph, sent when the pipeline publishes rows and again after the in-TUI preview (`p`) | `renderDetailViewForDay` (`internal/tui/overview_view.go`) |
| Web dashboard | A new `Session.SetDependencyGraph(engine.DependencyGraph)` called wherever `setPipelineSource` publishes the snapshot (`internal/cli/web_data.go`) | `handleOverviewResource` (`internal/webui/handlers_overview.go`) |

`internal/webui` imports `engine` only; it must not import `cli`, `registry`
or `pluginhost`.
