# Contract: `internal/stackload`

**Feature**: `1733-shared-stack-loader` | **Model**: [../data-model.md](../data-model.md)

The package is internal. This contract fixes the names and behavior the tasks,
the tests, and the web contract rely on.

## Package rules

- Imports `internal/pulumi`, `internal/ingest`, and `internal/engine`. It must
  not import `internal/cli`, `internal/webui`, `internal/registry`, or
  `internal/pluginhost`.
- No history writes, no dismissal reads, no plugin calls, no ESC handling.
  Callers keep those (research R13).
- Subprocess output is parsed here. Raw state-export error text never leaves
  the package: a state failure crosses the boundary as a `Status.Reason`
  category. A failed preview is fatal and keeps Pulumi's error (`Unwrap`), as
  in v0.5.2, for the terminal only; the web shows a fixed message.
- The passphrase is passed to the subprocess environment only.

## Functions

| Signature | Behavior |
| --- | --- |
| `Load(ctx context.Context, src Source) (*Snapshot, error)` | Runs the state export and the preview concurrently, or reads the explicit files. Cancels the sibling on a fatal error. A state failure with a loaded plan is recorded in `StateStatus` and is not an error. |
| `LoadState(ctx context.Context, src Source) (*Snapshot, error)` | State only. Used by the terminal screen's state-first flow and by `--pulumi-state` on its own. |
| `LoadPlan(ctx context.Context, src Source) (*Snapshot, error)` | Plan only. Used by plan-file-only runs. |
| `(*Snapshot).WithPlan(plan *ingest.PulumiPlan) *Snapshot` | A new snapshot with the plan attached, for the on-demand preview. The receiver is not modified. |
| `(*Snapshot).Descriptors() []engine.ResourceDescriptor` | Plan descriptors, then unchanged state resources. See the data model. |
| `(*Snapshot).DeployedDescriptors() []engine.ResourceDescriptor` | State descriptors exactly as `ingest.MapStateResources` maps them. |
| `(*Snapshot).ActualDescriptors() []engine.ResourceDescriptor` | The actual-cost inventory: `DeployedDescriptors()` when state was read, otherwise `Descriptors()`, which keeps the v0.5.2 inventory of a saved preview. |
| `(*Snapshot).Rows(ctx context.Context) ([]engine.OverviewRow, error)` | `engine.MergeResourcesForOverview` over `State` and `Steps`; state-only rows when no plan was loaded. |
| `(*Snapshot).PendingChanges(ctx context.Context) (bool, int)` | `engine.DetectPendingChanges` over `Steps`. |

All returned slices are copies; callers may modify them.

## Errors

| Situation | Result |
| --- | --- |
| Neither state nor plan can be loaded | An error naming the input, as the command returns today. |
| State export fails, plan loaded | `nil` error; `StateStatus{Loaded: false, Reason: <category>}`. |
| Explicit `--pulumi-state` file cannot be read, plan requested | An error naming the file; the preview is cancelled. |
| Plan fails, state loaded | The plan error, as today. The caller decides whether state-only is acceptable (the terminal screen does; `cost projected` does not). |
| Passphrase missing for an encrypted stack | `Reason: passphrase_required`; the existing message that names the passphrase is produced by the caller. |
| Project state names a different stack or project than a saved preview's URNs | `nil` error; the preview alone with `Reason: not_requested`, as outside a project. An explicit state and preview file pair is merged as given. |
| Caller context canceled while either input loads | The context error, never a state category. |

## Engine additions

| Symbol | Contract |
| --- | --- |
| `engine.DescriptorOrigin`, `engine.OriginPlan`, `engine.OriginState` | Origin of a descriptor. `ResourceDescriptor.Origin` is `json:"-"`; the zero value is `OriginPlan`. |
| `engine.DiffSummary.StackTotal`, `.StackResources`, `.StackComplete`, `.StackIncompleteReason`, `.OmittedUnchanged` | Defined in the data model. Existing summary fields keep their v0.5.2 values. |
| `engine.DiffResult.StackUnchanged` | The `OriginState` entries, kept out of `Entries`. |
| `(*engine.Engine).GetProjectedCostDiff` | Prices every descriptor once. Splits plan-origin entries into `Entries` and the existing summary, and state-origin entries into `StackUnchanged`. A result with no state-origin descriptors is identical to v0.5.2. |
| `ingest.PulumiPlan.ChangeSummary` | `map[string]int` from `changeSummary`. Absent in older previews; then `OmittedUnchanged` is zero. |

## Compatibility checks the tests must pin

1. For a preview with no state, `Descriptors()` equals `ingest.MapResources`
   output element for element.
2. `DeployedDescriptors()` equals `ingest.MapStateResources` output.
3. For a resource present in both, the descriptor comes from the plan.
4. `GetProjectedCostDiff` on plan-origin descriptors only equals the v0.5.2
   result (golden).
5. A state descriptor and a `--show-sames` plan descriptor for the same URN
   agree except for `Operation`, `Origin`, and the injected `pulumi:`
   properties (parity test, research risk table).
