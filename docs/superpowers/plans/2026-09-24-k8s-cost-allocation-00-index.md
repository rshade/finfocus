# Kubernetes Cost Allocation — Plan Index

<!-- markdownlint-configure-file { "MD010": { "code_blocks": false } } -->

**Spec:** `docs/superpowers/specs/2026-09-24-k8s-cost-allocation-design.md`

The spec spans four independently testable sub-projects, so it is split into
one plan per sub-project. Each plan stands alone; this index records order and
gates only.

| Order | Plan | Repo | Starts when | Produces |
| --- | --- | --- | --- | --- |
| 1 | [SP1 — finfocus-spec issues](2026-09-24-k8s-cost-allocation-sp1-spec-issues.md) | finfocus-spec (issues only) | Now | Two filed issues; later, a finfocus-spec release containing both RPCs |
| 1 | [SP3b — monorepo plugin releases](2026-09-24-k8s-cost-allocation-sp3b-registry-monorepo.md) | finfocus | Now (no dependencies) | Registry installs `kubernetes-vX.Y.Z` tagged plugins; release pipeline for `plugins/kubernetes` |
| 2 | [SP2 — kubernetes plugin](2026-09-24-k8s-cost-allocation-sp2-kubernetes-plugin.md) | finfocus (`plugins/kubernetes/`) | finfocus-spec release from SP1 | Plugin serving `GetStats` (run-rate) and `Allocate` |
| 3 | [SP3 — `cost cluster`](2026-09-24-k8s-cost-allocation-sp3-cost-cluster.md) | finfocus | SP1 release **and** PR #1510 merged; E2E task needs SP2 | Command, pipeline, rendering, MCP, kind E2E |

## Tracking

- finfocus-spec issues: [#505 GetStats](https://github.com/rshade/finfocus-spec/issues/505), [#506 Allocate](https://github.com/rshade/finfocus-spec/issues/506), [#507 Supports default registry](https://github.com/rshade/finfocus-spec/issues/507)

## Status (2026-09-28)

- **SP1**: complete. Tasks 1–3 filed #505/#506/#507 (closed by finfocus-spec v0.6.2); Task 4 bumped
  to v0.6.2; Task 5 makes the engine send provider/type/SKU/region in `Supports`, keyed per
  client+provider+type+region+SKU+feature, and fails open for plugins without `SupportsProvider`.
- **SP3b**: complete (uncommitted on `612-k8s-cost-allocation`).
- **SP2**: Tasks 1–7 complete (uncommitted); Task 8 parked until a `kubernetes-v0.1.0` release exists.
- **SP3**: unblocked by v0.6.2; waits on PR #1510 (open).

## Gates

- **finfocus-spec 051 + 052 → SP2/SP3**: besides the generated types, the plans call SDK helpers
  that 052 adds (`pluginsdk.DecodePolicy`, `ValidateAllocateRequest`, `ResolveCurrency`,
  `CheckConservation`); the bump in SP1 Task 4 must be to a release containing both features.
- **SP1 → SP2/SP3**: core and the plugin compile against generated types
  (`pbc.GetStatsRequest`, `pbc.NewUsageSourceServiceClient`, …) that exist only
  after finfocus-spec releases the two RPCs. A local `replace` to a finfocus-spec
  branch is allowed for development and must not be merged.
- **PR #1510 → SP3**: `resolveOutputFormat`, `machineOutputRequested`,
  `writeJSON`, `mcp.Exclude`, and `internal/cli/testdata/mcp/tools.golden` exist
  only on branch `611-mcp-fidelity`. SP3 starts from `main` after #1510 merges.
- **SP3b → first plugin release**: the `kubernetes` registry entry is added in
  SP2's final task, after SP3b is merged and a `kubernetes-v0.1.0` release exists.

## Spec amendments

Research during planning changed several spec details (SDK serving work, plugin
selection, `$0` = unpriced, plugin `Supports` opt-out, SP3b canonical versions,
and more). They are recorded in spec §8 "Plan-time amendments"; the plans follow
that section.

## Repository rules that override plan-template defaults

- **Never run `git commit`** (project CLAUDE.md). Every "commit" step in these
  plans is a **stage-and-hand-off** step: `git add` the listed files and propose
  the conventional-commit message; the user commits.
- Run `make lint` (may exceed 5 minutes) and `make test` before declaring any
  task complete.
- Do not modify `.golangci.yml` without explicit approval.
- Filing issues on finfocus-spec is outward-facing: confirm with the user before
  running `gh issue create`.
