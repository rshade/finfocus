# Data Model: Shared Stack Loader

**Feature**: `1733-shared-stack-loader` | **Spec**: [spec.md](spec.md) |
**Research**: [research.md](research.md)

The model reuses the types the overview already depends on. New types are
marked **new**. Field names are the Go names the tasks will use.

## Source (new, `internal/stackload`)

What to load. Built by the command from its flags and by the web launcher.

| Field | Meaning |
| --- | --- |
| `ProjectDir`, `Stack` | A resolved Pulumi project directory and stack (resolution stays in `internal/cli`). Empty when only files are given. |
| `StatePath` | Explicit stack export file (`--pulumi-state`). |
| `PlanPath` | Explicit preview file (`--pulumi-json`). |
| `Passphrase` | Optional; injected into the subprocess environment only, never `os.Setenv`. |
| `SkipPreview` | True for the state-first terminal flow and `--pulumi-state` on its own. |

Rules:

- With `ProjectDir` set and no files, both the state export and the preview
  run, concurrently.
- With `PlanPath` and no readable project, only the plan is loaded.
- With `StatePath` and no `PlanPath`, only the state is loaded.

## Snapshot (new)

The result of one load.

| Field | Type | Meaning |
| --- | --- | --- |
| `StackName`, `ProjectDir` | string | Resolved identity, kept for on-demand preview and history. |
| `State` | `[]engine.StateResource` | Custom resources from the state, in state order. |
| `RawState` | `[]ingest.StackExportResource` | The parsed custom resources, for callers that map them to descriptors. |
| `Plan` | `*ingest.PulumiPlan` | The parsed preview, kept so callers can map it as today. |
| `Steps` | `[]engine.PlanStep` | The preview as overview steps. |
| `StateStatus` | `Status` | Whether the state was read, and why not. |
| `PlanStatus` | `Status` | Whether a preview was available. |
| `OmittedUnchanged` | int | `changeSummary.same` minus the `same` steps the preview lists. Zero when unknown. |
| `Mismatches` | `[]string` | URNs that do not agree between preview and state (see below). |

`Status`:

| Field | Values |
| --- | --- |
| `Loaded` | bool |
| `Reason` | one of `none`, `not_requested`, `backend_unreachable`, `passphrase_required`, `read_failed`. A category only; never raw error text. |

Validation rules:

- A snapshot with neither state nor plan loaded is an error, returned as
  today's error for that input.
- `Mismatches` holds the URN of each plan step whose operation is not `create`
  and whose URN is not in the state, and of each `create` whose URN is already
  in the state. It is empty when either side was not loaded.
- `OmittedUnchanged` is `max(0, changeSummary.same - listedSame)` and is
  meaningful only when the plan was loaded.

## Change kind

The single field every command reads. It is the existing plan operation
vocabulary, not a new enum.

| Kind | Where it comes from |
| --- | --- |
| `create` | Plan step `create`, or a descriptor with no operation (existing behavior). |
| `update` | Plan step `update`. |
| `replace` | Plan steps `replace`, `create-replacement`, `delete-replaced` (collapsed per URN by the diff, as today). |
| `delete` | Plan step `delete`. |
| `same` | Plan step `same`, or any state resource the plan does not list. |

State resources the plan does not list are `same`. State supplied with no
preview at all is `same` for every resource (today it is priced as `create`).

## Descriptor view (existing type, one new field)

`engine.ResourceDescriptor` is unchanged except for one field:

| Field | Type | Meaning |
| --- | --- | --- |
| `Origin` | `DescriptorOrigin` (**new**) | `plan` or `state`. Not serialized (`json:"-"`). Set by the loader; zero value is `plan`, so every existing construction keeps today's meaning. |

`Snapshot.Descriptors()`:

1. For each plan resource, in plan order: the descriptor `ingest.MapResources`
   produces today, `Origin = plan`.
2. Then, in state order, for each custom state resource whose URN is not among
   the plan's resources: the descriptor `ingest.MapStateResource` produces,
   with `Operation = "same"` and `Origin = state`.

`Snapshot.DeployedDescriptors()`: every custom state resource through
`ingest.MapStateResource`, unchanged from today, no `Operation`.

`Snapshot.Rows(ctx)`: `engine.MergeResourcesForOverview(ctx, State, Steps)`,
unchanged.

## DiffResult and DiffSummary (existing, extended)

`DiffSummary` gains stack-level fields. Existing fields keep their meaning:
they are computed over `Origin = plan` entries.

| Field | Meaning |
| --- | --- |
| `TotalBefore`, `TotalAfter`, `TotalDelta`, `Creates`, `Updates`, `Deletes`, `Unchanged` | Plan membership only, as in v0.5.2. `TotalAfter` is the plan total and the budget input. |
| `StackTotal` (**new**) | Sum of the after cost over every entry, each resource once. |
| `StackResources` (**new**) | Count of entries in the stack total. |
| `StackComplete` (**new**) | False when the state could not be read. |
| `StackIncompleteReason` (**new**) | The `Status.Reason` category when incomplete; empty otherwise. |
| `OmittedUnchanged` (**new**) | Copied from the snapshot when state was unavailable. |

`DiffResult` gains `StackUnchanged []DiffEntry` (**new**): the `Origin = state`
entries, kept out of `Entries`. `EmptyPlan` already exists (v0.5.2).

Invariants:

- `Entries` and `Summary.TotalAfter` equal what v0.5.2 produced for the same
  plan, for every input (SC-007).
- `StackTotal = TotalAfter + sum(StackUnchanged after cost)` when the state was
  read and no resource is in both lists.
- A resource listed by the preview as `same` and also present in state is in
  `Entries` once, as the plan lists it, and counts once in `StackTotal`.
- With `StackComplete = false`, `StackTotal = TotalAfter` and
  `StackUnchanged` is empty.
- With state and no preview, `Entries` holds the state resources as `same`,
  `TotalAfter` is their sum, and `StackTotal` equals it.

## CLI JSON additions (`cost projected --output json`)

Additive keys under the existing `finfocus` object. See
[contracts/cli-output.md](contracts/cli-output.md).

## State transitions

None persisted. One load produces one snapshot. The terminal screen's staged
flow loads state first and the plan later; it builds a new snapshot when the
plan arrives, and the earlier one is discarded.
